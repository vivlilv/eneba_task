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

	changes := make([]priceChange, 0)
	unavailable := make([]string, 0)
	var total int64
	for _, line := range lines {
		if !line.Active || line.AvailableKeys == 0 {
			unavailable = append(unavailable, line.OfferID)
		}
		if line.PriceSeenCents != line.PriceCents {
			changes = append(changes, priceChange{
				OfferID: line.OfferID, PreviousCents: line.PriceSeenCents,
				CurrentCents: line.PriceCents, Currency: line.Currency,
			})
		}
		total += line.PriceCents
	}
	if len(unavailable) > 0 {
		writeError(w, http.StatusConflict, "one or more offers are no longer available; no items were purchased", "out_of_stock", map[string]any{"offerIds": unavailable})
		return
	}
	if len(changes) > 0 && !input.ConfirmPriceChanges {
		writeError(w, http.StatusConflict, "one or more offer prices changed; review and confirm the new total", "price_changed", map[string]any{"changes": changes, "newTotalCents": total, "currency": "EUR"})
		return
	}

	var orderID string
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO orders(user_id, status, currency, total_cents, checkout_key)
		VALUES ($1, 'pending_payment', 'EUR', $2, $3)
		RETURNING id::text
	`, current.ID, total, checkoutKey).Scan(&orderID)
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
	for _, line := range lines {
		_, err := tx.ExecContext(r.Context(), `
			INSERT INTO order_items(order_id, product_id, offer_id, product_title_snapshot,
			                        seller_name_snapshot, unit_price_cents)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, orderID, line.ProductID, line.OfferID, line.ProductTitle, line.SellerName, line.PriceCents)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create order items", "internal_error", nil)
			return
		}
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
		result, loadErr := a.loadOrder(r, current.ID, orderID)
		if loadErr != nil {
			writeError(w, http.StatusInternalServerError, "could not load order", "internal_error", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"paymentStatus": previousOutcome, "order": result})
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
		result, loadErr := a.loadOrder(r, current.ID, orderID)
		if loadErr != nil {
			writeError(w, http.StatusInternalServerError, "could not load order", "internal_error", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"paymentStatus": "succeeded", "order": result})
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
		result, loadErr := a.loadOrder(r, current.ID, orderID)
		if loadErr != nil {
			writeError(w, http.StatusInternalServerError, "could not load order", "internal_error", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"paymentStatus": "declined", "order": result})
		return
	}

	rows, err := tx.QueryContext(r.Context(), `
		SELECT id, offer_id FROM order_items WHERE order_id::text = $1 ORDER BY offer_id FOR UPDATE
	`, orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load order items", "internal_error", nil)
		return
	}
	type itemOffer struct {
		itemID  int64
		offerID string
	}
	items := make([]itemOffer, 0)
	for rows.Next() {
		var item itemOffer
		if err := rows.Scan(&item.itemID, &item.offerID); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "could not load order items", "internal_error", nil)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeError(w, http.StatusInternalServerError, "could not load order items", "internal_error", nil)
		return
	}
	rows.Close()
	sort.Slice(items, func(i, j int) bool { return items[i].offerID < items[j].offerID })
	allocated := make(map[int64]int64, len(items))
	for _, item := range items {
		var keyID int64
		err := tx.QueryRowContext(r.Context(), `
			SELECT dk.id
			FROM digital_keys dk
			WHERE dk.offer_id = $1
			  AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id)
			ORDER BY dk.id
			FOR UPDATE OF dk SKIP LOCKED
			LIMIT 1
		`, item.offerID).Scan(&keyID)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "an offer has run out of keys; no items were purchased", "out_of_stock", map[string]string{"offerId": item.offerID})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not allocate key", "internal_error", nil)
			return
		}
		allocated[item.itemID] = keyID
	}
	for itemID, keyID := range allocated {
		if _, err := tx.ExecContext(r.Context(), `UPDATE order_items SET digital_key_id = $1 WHERE id = $2`, keyID, itemID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not assign keys", "internal_error", nil)
			return
		}
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
	result, err := a.loadOrder(r, current.ID, orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "payment completed but Order could not be loaded", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"paymentStatus": "succeeded", "order": result})
}

func (a *App) handleListOrders(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id::text FROM orders WHERE user_id = $1 AND status = 'paid' ORDER BY created_at DESC LIMIT 50
	`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load Orders", "internal_error", nil)
		return
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "could not load Orders", "internal_error", nil)
			return
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeError(w, http.StatusInternalServerError, "could not load Orders", "internal_error", nil)
		return
	}
	rows.Close()
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
