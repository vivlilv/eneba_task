package app

import (
	"database/sql"
	"net/http"
	"strings"
)

type guestCartItem struct {
	OfferID        string `json:"offerId"`
	PriceSeenCents int64  `json:"priceSeenCents"`
}

type cartItemView struct {
	OfferID        string `json:"offerId"`
	ProductSlug    string `json:"productSlug"`
	ProductTitle   string `json:"productTitle"`
	Platform       string `json:"platform"`
	Region         string `json:"region"`
	SellerName     string `json:"sellerName"`
	PriceCents     int64  `json:"priceCents"`
	PriceSeenCents int64  `json:"priceSeenCents"`
	Currency       string `json:"currency"`
	AvailableKeys  int    `json:"availableKeys"`
	PriceChanged   bool   `json:"priceChanged"`
}

type cartView struct {
	Items           []cartItemView `json:"items"`
	TotalCents      int64          `json:"totalCents"`
	HasPriceChanges bool           `json:"hasPriceChanges"`
}

func (a *App) handleGetCart(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	cart, err := a.loadCart(r, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load cart", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (a *App) handleAddCartItem(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var input guestCartItem
	if err := decodeJSON(w, r, &input); err != nil || strings.TrimSpace(input.OfferID) == "" || input.PriceSeenCents < 0 {
		writeError(w, http.StatusBadRequest, "offerId and priceSeenCents are required", "invalid_request", nil)
		return
	}
	input.OfferID = strings.TrimSpace(input.OfferID)
	var currentPrice int64
	var active bool
	var available int
	err := a.db.QueryRowContext(r.Context(), `
		SELECT o.price_cents, o.active,
		       (SELECT COUNT(*)::int FROM digital_keys dk
		        WHERE dk.offer_id = o.id
		          AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id))
		FROM offers o WHERE o.id = $1
	`, input.OfferID).Scan(&currentPrice, &active, &available)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "offer not found", "offer_unavailable", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not add offer", "internal_error", nil)
		return
	}
	if !active {
		writeError(w, http.StatusNotFound, "offer not found", "offer_unavailable", nil)
		return
	}
	if currentPrice != input.PriceSeenCents {
		writeError(w, http.StatusConflict, "the offer price has changed; review the new price before adding it", "price_changed", map[string]int64{"currentPriceCents": currentPrice})
		return
	}
	if available == 0 {
		writeError(w, http.StatusConflict, "this offer is out of stock", "out_of_stock", nil)
		return
	}
	_, err = a.db.ExecContext(r.Context(), `
		INSERT INTO cart_items(user_id, offer_id, price_seen_cents)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, offer_id) DO NOTHING
	`, current.ID, input.OfferID, input.PriceSeenCents)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not add offer", "internal_error", nil)
		return
	}
	cart, err := a.loadCart(r, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load cart", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (a *App) handleMergeCart(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Items []guestCartItem `json:"items"`
	}
	if err := decodeJSON(w, r, &input); err != nil || len(input.Items) > 50 {
		writeError(w, http.StatusBadRequest, "invalid cart", "invalid_request", nil)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not merge cart", "internal_error", nil)
		return
	}
	defer tx.Rollback()
	for _, item := range normalizedOfferIDs(input.Items) {
		if item.PriceSeenCents < 0 {
			writeError(w, http.StatusBadRequest, "invalid cart price", "invalid_request", nil)
			return
		}
		var active bool
		err := tx.QueryRowContext(r.Context(), `SELECT active FROM offers WHERE id = $1`, item.OfferID).Scan(&active)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusConflict, "an item in the guest cart is no longer available", "offer_unavailable", map[string]string{"offerId": item.OfferID})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not merge cart", "internal_error", nil)
			return
		}
		if !active {
			writeError(w, http.StatusConflict, "an item in the guest cart is no longer available", "offer_unavailable", map[string]string{"offerId": item.OfferID})
			return
		}
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO cart_items(user_id, offer_id, price_seen_cents)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, offer_id) DO NOTHING
		`, current.ID, item.OfferID, item.PriceSeenCents)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not merge cart", "internal_error", nil)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not merge cart", "internal_error", nil)
		return
	}
	cart, err := a.loadCart(r, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load cart", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (a *App) handleConfirmPrices(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	_, err := a.db.ExecContext(r.Context(), `
		UPDATE cart_items ci
		SET price_seen_cents = o.price_cents
		FROM offers o
		WHERE ci.user_id = $1 AND ci.offer_id = o.id AND o.active = TRUE
	`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not confirm prices", "internal_error", nil)
		return
	}
	cart, err := a.loadCart(r, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load cart", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (a *App) handleDeleteCartItem(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	offerID := strings.TrimSpace(r.PathValue("offerID"))
	_, err := a.db.ExecContext(r.Context(), `DELETE FROM cart_items WHERE user_id = $1 AND offer_id = $2`, current.ID, offerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not remove offer", "internal_error", nil)
		return
	}
	cart, err := a.loadCart(r, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load cart", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (a *App) loadCart(r *http.Request, userID int64) (cartView, error) {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT o.id, p.slug, p.title, p.platform, p.region, s.name,
		       o.price_cents, ci.price_seen_cents, o.currency,
		       COUNT(dk.id) FILTER (WHERE oi.id IS NULL)::int
	FROM cart_items ci
	JOIN offers o ON o.id = ci.offer_id AND o.active = TRUE
	JOIN products p ON p.id = o.product_id
	JOIN sellers s ON s.id = o.seller_id
	LEFT JOIN digital_keys dk ON dk.offer_id = o.id
	LEFT JOIN order_items oi ON oi.digital_key_id = dk.id
	WHERE ci.user_id = $1
	GROUP BY ci.added_at, o.id, p.slug, p.title, p.platform, p.region, s.name,
	         o.price_cents, ci.price_seen_cents, o.currency
	ORDER BY ci.added_at
	`, userID)
	if err != nil {
		return cartView{}, err
	}
	defer rows.Close()
	cart := cartView{Items: make([]cartItemView, 0)}
	for rows.Next() {
		var item cartItemView
		if err := rows.Scan(
			&item.OfferID, &item.ProductSlug, &item.ProductTitle, &item.Platform, &item.Region,
			&item.SellerName, &item.PriceCents, &item.PriceSeenCents, &item.Currency, &item.AvailableKeys,
		); err != nil {
			return cartView{}, err
		}
		item.Currency = strings.TrimSpace(item.Currency)
		item.PriceChanged = item.PriceCents != item.PriceSeenCents
		cart.HasPriceChanges = cart.HasPriceChanges || item.PriceChanged
		cart.TotalCents += item.PriceCents
		cart.Items = append(cart.Items, item)
	}
	return cart, rows.Err()
}
