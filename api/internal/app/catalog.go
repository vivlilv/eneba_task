package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

type productCard struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Edition     string `json:"edition"`
	Platform    string `json:"platform"`
	Region      string `json:"region"`
	Category    string `json:"category"`
	LowestPrice int64  `json:"lowestPriceCents"`
	Currency    string `json:"currency"`
	Available   bool   `json:"available"`
	OfferCount  int    `json:"offerCount"`
}

type homepageBlock struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Kind     string        `json:"kind"`
	Eyebrow  string        `json:"eyebrow"`
	Body     string        `json:"body"`
	CTALabel string        `json:"ctaLabel"`
	CTAURL   string        `json:"ctaUrl"`
	Products []productCard `json:"products"`
}

func (a *App) handleHomepage(w http.ResponseWriter, r *http.Request) {
	blockRows, err := a.db.QueryContext(r.Context(), `
		SELECT id, title, kind, eyebrow, body, cta_label, cta_url
		FROM homepage_blocks ORDER BY position
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load homepage", "internal_error", nil)
		return
	}
	blocks := make([]homepageBlock, 0)
	blockIndex := make(map[string]int)
	for blockRows.Next() {
		var block homepageBlock
		if err := blockRows.Scan(&block.ID, &block.Title, &block.Kind, &block.Eyebrow, &block.Body, &block.CTALabel, &block.CTAURL); err != nil {
			blockRows.Close()
			writeError(w, http.StatusInternalServerError, "could not load homepage", "internal_error", nil)
			return
		}
		block.Products = make([]productCard, 0)
		blockIndex[block.ID] = len(blocks)
		blocks = append(blocks, block)
	}
	if err := blockRows.Err(); err != nil {
		blockRows.Close()
		writeError(w, http.StatusInternalServerError, "could not load homepage", "internal_error", nil)
		return
	}
	blockRows.Close()

	productRows, err := a.db.QueryContext(r.Context(), `
		SELECT hb.id, p.id, p.slug, p.title, p.edition, p.platform, p.region, p.category,
		       COALESCE(MIN(o.price_cents), 0), COALESCE(MIN(o.currency), 'EUR'),
		       COALESCE(BOOL_OR(EXISTS (
		           SELECT 1 FROM digital_keys dk
		           WHERE dk.offer_id = o.id
		             AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id)
		       )), FALSE),
		       COUNT(DISTINCT o.id)::int
		FROM homepage_blocks hb
		JOIN homepage_block_products hbp ON hbp.block_id = hb.id
		JOIN products p ON p.id = hbp.product_id
		LEFT JOIN offers o ON o.product_id = p.id AND o.active = TRUE
		GROUP BY hb.id, hb.position, hbp.position, p.id
		ORDER BY hb.position, hbp.position
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load homepage products", "internal_error", nil)
		return
	}
	defer productRows.Close()
	for productRows.Next() {
		var blockID string
		var product productCard
		if err := productRows.Scan(
			&blockID, &product.ID, &product.Slug, &product.Title, &product.Edition,
			&product.Platform, &product.Region, &product.Category,
			&product.LowestPrice, &product.Currency, &product.Available, &product.OfferCount,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "could not load homepage products", "internal_error", nil)
			return
		}
		product.Region = strings.TrimSpace(product.Region)
		product.Currency = strings.TrimSpace(product.Currency)
		if index, ok := blockIndex[blockID]; ok {
			blocks[index].Products = append(blocks[index].Products, product)
		}
	}
	if err := productRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load homepage products", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blocks": blocks})
}

type offerView struct {
	ID            string  `json:"id"`
	SellerName    string  `json:"sellerName"`
	SellerRating  float64 `json:"sellerRating"`
	PriceCents    int64   `json:"priceCents"`
	Currency      string  `json:"currency"`
	AvailableKeys int     `json:"availableKeys"`
	Available     bool    `json:"available"`
}

type productDetail struct {
	productCard
	Description string      `json:"description"`
	Offers      []offerView `json:"offers"`
}

func (a *App) handleProduct(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var product productDetail
	err := a.db.QueryRowContext(r.Context(), `
		SELECT p.id, p.slug, p.title, p.edition, p.platform, p.region, p.category, p.description,
		       COALESCE(MIN(o.price_cents), 0), COALESCE(MIN(o.currency), 'EUR'),
		       COALESCE(BOOL_OR(EXISTS (
		           SELECT 1 FROM digital_keys dk
		           WHERE dk.offer_id = o.id
		             AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id)
		       )), FALSE),
		       COUNT(DISTINCT o.id)::int
		FROM products p
		LEFT JOIN offers o ON o.product_id = p.id AND o.active = TRUE
		WHERE p.slug = $1
		GROUP BY p.id
	`, slug).Scan(
		&product.ID, &product.Slug, &product.Title, &product.Edition, &product.Platform,
		&product.Region, &product.Category, &product.Description,
		&product.LowestPrice, &product.Currency, &product.Available, &product.OfferCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "product not found", "not_found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load product", "internal_error", nil)
		return
	}
	product.Region = strings.TrimSpace(product.Region)
	product.Currency = strings.TrimSpace(product.Currency)

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT o.id, s.name, s.rating, o.price_cents, o.currency,
		       COUNT(dk.id) FILTER (WHERE oi.id IS NULL)::int
		FROM offers o
		JOIN sellers s ON s.id = o.seller_id
		LEFT JOIN digital_keys dk ON dk.offer_id = o.id
		LEFT JOIN order_items oi ON oi.digital_key_id = dk.id
		WHERE o.product_id = $1 AND o.active = TRUE
		GROUP BY o.id, s.name, s.rating, o.price_cents, o.currency
		ORDER BY o.price_cents, s.name
	`, product.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load offers", "internal_error", nil)
		return
	}
	defer rows.Close()
	product.Offers = make([]offerView, 0)
	for rows.Next() {
		var offer offerView
		if err := rows.Scan(&offer.ID, &offer.SellerName, &offer.SellerRating, &offer.PriceCents, &offer.Currency, &offer.AvailableKeys); err != nil {
			writeError(w, http.StatusInternalServerError, "could not load offers", "internal_error", nil)
			return
		}
		offer.Currency = strings.TrimSpace(offer.Currency)
		offer.Available = offer.AvailableKeys > 0
		product.Offers = append(product.Offers, offer)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load offers", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, product)
}
