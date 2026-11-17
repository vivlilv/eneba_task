package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

type offerPreview struct {
	ID             string `json:"id"`
	ProductSlug    string `json:"productSlug"`
	ProductTitle   string `json:"productTitle"`
	Platform       string `json:"platform"`
	Region         string `json:"region"`
	SellerName     string `json:"sellerName"`
	PriceCents     int64  `json:"priceCents"`
	PriceSeenCents int64  `json:"priceSeenCents,omitempty"`
	Currency       string `json:"currency"`
	AvailableKeys  int    `json:"availableKeys"`
	Available      bool   `json:"available"`
}

func (a *App) handleOfferPreview(w http.ResponseWriter, r *http.Request) {
	var offer offerPreview
	err := a.db.QueryRowContext(r.Context(), `
		SELECT o.id, p.slug, p.title, p.platform, p.region, s.name, o.price_cents, o.currency,
		       (SELECT COUNT(*)::int FROM digital_keys dk
		        WHERE dk.offer_id = o.id
		          AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id))
		FROM offers o
		JOIN products p ON p.id = o.product_id
		JOIN sellers s ON s.id = o.seller_id
		WHERE o.id = $1 AND o.active = TRUE
	`, strings.TrimSpace(r.PathValue("offerID"))).Scan(
		&offer.ID, &offer.ProductSlug, &offer.ProductTitle, &offer.Platform, &offer.Region,
		&offer.SellerName, &offer.PriceCents, &offer.Currency, &offer.AvailableKeys,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "offer not found", "not_found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load offer", "internal_error", nil)
		return
	}
	offer.Currency = strings.TrimSpace(offer.Currency)
	offer.Available = offer.AvailableKeys > 0
	writeJSON(w, http.StatusOK, offer)
}
