package app

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

type checkoutRequest struct {
	ConfirmPriceChanges bool `json:"confirmPriceChanges"`
}

type priceChange struct {
	OfferID       string `json:"offerId"`
	PreviousCents int64  `json:"previousPriceCents"`
	CurrentCents  int64  `json:"currentPriceCents"`
	Currency      string `json:"currency"`
}

type checkoutLine struct {
	OfferID        string
	ProductID      string
	ProductTitle   string
	SellerName     string
	PriceCents     int64
	PriceSeenCents int64
	Currency       string
	AvailableKeys  int
	Active         bool
}

type checkoutSummary struct {
	totalCents          int64
	priceChanges        []priceChange
	unavailableOfferIDs []string
}

type orderItemView struct {
	ID             int64   `json:"id"`
	ProductSlug    string  `json:"productSlug"`
	ProductTitle   string  `json:"productTitle"`
	OfferID        string  `json:"offerId"`
	SellerName     string  `json:"sellerName"`
	UnitPriceCents int64   `json:"unitPriceCents"`
	Currency       string  `json:"currency"`
	DigitalKey     *string `json:"digitalKey,omitempty"`
}

type orderView struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Currency   string          `json:"currency"`
	TotalCents int64           `json:"totalCents"`
	CreatedAt  time.Time       `json:"createdAt"`
	Items      []orderItemView `json:"items"`
}

func (a *App) handleCheckout(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	checkoutKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if checkoutKey == "" || len(checkoutKey) > 120 {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required", "idempotency_key_required", nil)
		return
	}
	var input checkoutRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "invalid_request", nil)
		return
	}
	if existing, err := a.findOrderByCheckoutKey(r, current.ID, checkoutKey); err == nil {
		writeJSON(w, http.StatusOK, existing)
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "could not load existing checkout", "internal_error", nil)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start checkout", "internal_error", nil)
		return
	}
	defer tx.Rollback()

	lines, err := loadCheckoutLines(r, tx, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not validate cart", "internal_error", nil)
		return
	}
	if len(lines) == 0 {
		writeError(w, http.StatusConflict, "your cart is empty", "empty_cart", nil)
		return
	}

	summary := summarizeCheckoutLines(lines)
	if len(summary.unavailableOfferIDs) > 0 {
		writeError(w, http.StatusConflict, "one or more offers are no longer available; no items were purchased", "out_of_stock", map[string]any{"offerIds": summary.unavailableOfferIDs})
		return
	}
	if len(summary.priceChanges) > 0 && !input.ConfirmPriceChanges {
		writeError(w, http.StatusConflict, "one or more offer prices changed; review and confirm the new total", "price_changed", map[string]any{"changes": summary.priceChanges, "newTotalCents": summary.totalCents, "currency": "EUR"})
		return
	}

	orderID, err := insertPendingOrder(r, tx, current.ID, summary.totalCents, checkoutKey)
	if err != nil {
		if isUniqueViolation(err) {
			_ = tx.Rollback()
			if existing, findErr := a.findOrderByCheckoutKey(r, current.ID, checkoutKey); findErr == nil {
				writeJSON(w, http.StatusOK, existing)
				return
			}
		}
		writeError(w, http.StatusInternalServerError, "could not create order", "internal_error", nil)
		return
	}
	if err := insertOrderItems(r, tx, orderID, lines); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create order items", "internal_error", nil)
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not finish checkout", "internal_error", nil)
		return
	}
	created, err := a.loadOrder(r, current.ID, orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "order created but could not be loaded", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func insertPendingOrder(r *http.Request, tx *sql.Tx, userID int64, totalCents int64, checkoutKey string) (string, error) {
	var orderID string
	err := tx.QueryRowContext(r.Context(), `
		INSERT INTO orders(user_id, status, currency, total_cents, checkout_key)
		VALUES ($1, 'pending_payment', 'EUR', $2, $3)
		RETURNING id::text
	`, userID, totalCents, checkoutKey).Scan(&orderID)
	return orderID, err
}

func insertOrderItems(r *http.Request, tx *sql.Tx, orderID string, lines []checkoutLine) error {
	for _, line := range lines {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO order_items(order_id, product_id, offer_id, product_title_snapshot,
			                        seller_name_snapshot, unit_price_cents)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, orderID, line.ProductID, line.OfferID, line.ProductTitle, line.SellerName, line.PriceCents); err != nil {
			return err
		}
	}
	return nil
}

// summarizeCheckoutLines computes the current total and identifies price or availability changes.
func summarizeCheckoutLines(lines []checkoutLine) checkoutSummary {
	summary := checkoutSummary{
		priceChanges:        make([]priceChange, 0),
		unavailableOfferIDs: make([]string, 0),
	}
	for _, line := range lines {
		if !line.Active || line.AvailableKeys == 0 {
			summary.unavailableOfferIDs = append(summary.unavailableOfferIDs, line.OfferID)
		}
		if line.PriceSeenCents != line.PriceCents {
			summary.priceChanges = append(summary.priceChanges, priceChange{
				OfferID: line.OfferID, PreviousCents: line.PriceSeenCents,
				CurrentCents: line.PriceCents, Currency: line.Currency,
			})
		}
		summary.totalCents += line.PriceCents
	}
	return summary
}

func loadCheckoutLines(r *http.Request, tx *sql.Tx, userID int64) ([]checkoutLine, error) {
	rows, err := tx.QueryContext(r.Context(), `
		SELECT ci.offer_id, p.id, p.title, s.name, o.price_cents, ci.price_seen_cents,
		       o.currency, o.active,
		       (SELECT COUNT(*)::int FROM digital_keys dk
		        WHERE dk.offer_id = o.id
		          AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id))
		FROM cart_items ci
		JOIN offers o ON o.id = ci.offer_id
		JOIN products p ON p.id = o.product_id
		JOIN sellers s ON s.id = o.seller_id
		WHERE ci.user_id = $1
		ORDER BY ci.offer_id
		FOR UPDATE OF ci, o
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := make([]checkoutLine, 0)
	for rows.Next() {
		var line checkoutLine
		if err := rows.Scan(
			&line.OfferID, &line.ProductID, &line.ProductTitle, &line.SellerName,
			&line.PriceCents, &line.PriceSeenCents, &line.Currency, &line.Active, &line.AvailableKeys,
		); err != nil {
			return nil, err
		}
		line.Currency = strings.TrimSpace(line.Currency)
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (a *App) handlePay(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	orderID := strings.TrimSpace(r.PathValue("orderID"))
	attemptKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if attemptKey == "" || len(attemptKey) > 120 {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required", "idempotency_key_required", nil)
		return
	}
	var input struct {
		Outcome string `json:"outcome"`
	}
	if err := decodeJSON(w, r, &input); err != nil || (input.Outcome != "success" && input.Outcome != "declined") {
		writeError(w, http.StatusBadRequest, "outcome must be success or declined", "invalid_request", nil)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start payment", "internal_error", nil)
		return
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(r.Context(), `
		SELECT status FROM orders WHERE id::text = $1 AND user_id = $2 FOR UPDATE
	`, orderID, current.ID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "order not found", "not_found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load order", "internal_error", nil)
		return
	}

	var previousOutcome string
	err = tx.QueryRowContext(r.Context(), `
		SELECT status FROM payment_attempts WHERE order_id::text = $1 AND idempotency_key = $2
	`, orderID, attemptKey).Scan(&previousOutcome)
	if err == nil {
		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not confirm payment", "internal_error", nil)
			return
		}
		a.writePaymentResponse(w, r, current.ID, orderID, previousOutcome, "could not load order")
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "could not load payment attempt", "internal_error", nil)
		return
	}
	if status == "paid" {
		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not confirm payment", "internal_error", nil)
			return
		}
		a.writePaymentResponse(w, r, current.ID, orderID, "succeeded", "could not load order")
		return
	}
	if input.Outcome == "declined" {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO payment_attempts(order_id, idempotency_key, status)
			VALUES ($1, $2, 'declined')
		`, orderID, attemptKey); err != nil {
			writeError(w, http.StatusInternalServerError, "could not record payment attempt", "internal_error", nil)
			return
		}
		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not record payment attempt", "internal_error", nil)
			return
		}
		a.writePaymentResponse(w, r, current.ID, orderID, "declined", "could not load order")
		return
	}

	items, err := loadOrderItemsForPayment(r, tx, orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load order items", "internal_error", nil)
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].offerID < items[j].offerID })

	allocatedKeyIDs := make(map[int64]int64, len(items))
	for _, item := range items {
		keyID, err := findAvailableKeyForOffer(r, tx, item.offerID)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "an offer has run out of keys; no items were purchased", "out_of_stock", map[string]string{"offerId": item.offerID})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not allocate key", "internal_error", nil)
			return
		}
		allocatedKeyIDs[item.itemID] = keyID
	}
	if err := assignDigitalKeys(r, tx, allocatedKeyIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "could not assign keys", "internal_error", nil)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO payment_attempts(order_id, idempotency_key, status)
		VALUES ($1, $2, 'succeeded')
	`, orderID, attemptKey); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record payment", "internal_error", nil)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE orders SET status = 'paid' WHERE id::text = $1`, orderID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not complete Order", "internal_error", nil)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM cart_items
		WHERE user_id = $1 AND offer_id IN (SELECT offer_id FROM order_items WHERE order_id::text = $2)
	`, current.ID, orderID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not clear purchased cart items", "internal_error", nil)
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not finish payment", "internal_error", nil)
		return
	}
	a.writePaymentResponse(w, r, current.ID, orderID, "succeeded", "payment completed but Order could not be loaded")
}

func (a *App) writePaymentResponse(w http.ResponseWriter, r *http.Request, userID int64, orderID, paymentStatus, loadErrorMessage string) {
	order, err := a.loadOrder(r, userID, orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, loadErrorMessage, "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"paymentStatus": paymentStatus, "order": order})
}

type paymentOrderItem struct {
	itemID  int64
	offerID string
}

func loadOrderItemsForPayment(r *http.Request, tx *sql.Tx, orderID string) ([]paymentOrderItem, error) {
	rows, err := tx.QueryContext(r.Context(), `
		SELECT id, offer_id FROM order_items WHERE order_id::text = $1 ORDER BY offer_id FOR UPDATE
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]paymentOrderItem, 0)
	for rows.Next() {
		var item paymentOrderItem
		if err := rows.Scan(&item.itemID, &item.offerID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func findAvailableKeyForOffer(r *http.Request, tx *sql.Tx, offerID string) (int64, error) {
	var keyID int64
	err := tx.QueryRowContext(r.Context(), `
		SELECT dk.id
		FROM digital_keys dk
		WHERE dk.offer_id = $1
		  AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id)
		ORDER BY dk.id
		FOR UPDATE OF dk SKIP LOCKED
		LIMIT 1
	`, offerID).Scan(&keyID)
	return keyID, err
}

func assignDigitalKeys(r *http.Request, tx *sql.Tx, allocatedKeyIDs map[int64]int64) error {
	for itemID, keyID := range allocatedKeyIDs {
		if _, err := tx.ExecContext(r.Context(), `UPDATE order_items SET digital_key_id = $1 WHERE id = $2`, keyID, itemID); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) handleListOrders(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	ids, err := a.loadPaidOrderIDs(r, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load Orders", "internal_error", nil)
		return
	}
	orders := make([]orderView, 0, len(ids))
	for _, id := range ids {
		order, err := a.loadOrder(r, current.ID, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load Order details", "internal_error", nil)
			return
		}
		orders = append(orders, order)
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (a *App) loadPaidOrderIDs(r *http.Request, userID int64) ([]string, error) {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id::text FROM orders WHERE user_id = $1 AND status = 'paid' ORDER BY created_at DESC LIMIT 50
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (a *App) handleOrder(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	order, err := a.loadOrder(r, current.ID, strings.TrimSpace(r.PathValue("orderID")))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "order not found", "not_found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load Order", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (a *App) findOrderByCheckoutKey(r *http.Request, userID int64, checkoutKey string) (orderView, error) {
	var orderID string
	err := a.db.QueryRowContext(r.Context(), `SELECT id::text FROM orders WHERE user_id = $1 AND checkout_key = $2`, userID, checkoutKey).Scan(&orderID)
	if err != nil {
		return orderView{}, err
	}
	return a.loadOrder(r, userID, orderID)
}

func (a *App) loadOrder(r *http.Request, userID int64, orderID string) (orderView, error) {
	var order orderView
	err := a.db.QueryRowContext(r.Context(), `
		SELECT id::text, status, currency, total_cents, created_at
		FROM orders WHERE id::text = $1 AND user_id = $2
	`, orderID, userID).Scan(&order.ID, &order.Status, &order.Currency, &order.TotalCents, &order.CreatedAt)
	if err != nil {
		return orderView{}, err
	}
	order.Currency = strings.TrimSpace(order.Currency)
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT oi.id, p.slug, oi.product_title_snapshot, oi.offer_id,
		       oi.seller_name_snapshot, oi.unit_price_cents, o.currency, dk.code
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		JOIN offers o ON o.id = oi.offer_id
		LEFT JOIN digital_keys dk ON dk.id = oi.digital_key_id
		WHERE oi.order_id::text = $1
		ORDER BY oi.id
	`, orderID)
	if err != nil {
		return orderView{}, err
	}
	defer rows.Close()
	order.Items = make([]orderItemView, 0)
	for rows.Next() {
		var item orderItemView
		var key sql.NullString
		if err := rows.Scan(&item.ID, &item.ProductSlug, &item.ProductTitle, &item.OfferID,
			&item.SellerName, &item.UnitPriceCents, &item.Currency, &key); err != nil {
			return orderView{}, err
		}
		item.Currency = strings.TrimSpace(item.Currency)
		if order.Status == "paid" && key.Valid {
			value := key.String
			item.DigitalKey = &value
		}
		order.Items = append(order.Items, item)
	}
	return order, rows.Err()
}
