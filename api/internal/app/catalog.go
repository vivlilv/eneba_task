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

// handleHomepage assembles the homepage: every block in display order, each
// carrying the product cards assigned to it. Blocks and their products are
// loaded in two passes; each pass keeps its own failure message.
func (a *App) handleHomepage(w http.ResponseWriter, r *http.Request) {
	blocks, positionByBlockID, err := a.loadHomepageBlocks(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load homepage", "internal_error", nil)
		return
	}
	if err := a.attachBlockProducts(r, blocks, positionByBlockID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load homepage products", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blocks": blocks})
}

// loadHomepageBlocks returns every homepage block in display order (block
// position), plus a map from block ID to slice position so product rows
// scanned later can be routed to the block they belong to. Each block starts
// with a non-nil empty Products slice so JSON encodes [] rather than null.
func (a *App) loadHomepageBlocks(r *http.Request) ([]homepageBlock, map[string]int, error) {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, title, kind, eyebrow, body, cta_label, cta_url
		FROM homepage_blocks ORDER BY position
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	blocks := make([]homepageBlock, 0)
	positionByBlockID := make(map[string]int)
	for rows.Next() {
		var block homepageBlock
		if err := rows.Scan(&block.ID, &block.Title, &block.Kind, &block.Eyebrow, &block.Body, &block.CTALabel, &block.CTAURL); err != nil {
			return nil, nil, err
		}
		block.Products = make([]productCard, 0)
		positionByBlockID[block.ID] = len(blocks)
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return blocks, positionByBlockID, nil
}

// attachBlockProducts loads all block products in a single query (ordered by
// block position, then in-block position) and appends each scanned product
// card to its block. Rows whose block ID is unknown are skipped.
func (a *App) attachBlockProducts(r *http.Request, blocks []homepageBlock, positionByBlockID map[string]int) error {
	rows, err := a.db.QueryContext(r.Context(), `
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
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var blockID string
		var product productCard
		if err := rows.Scan(
			&blockID, &product.ID, &product.Slug, &product.Title, &product.Edition,
			&product.Platform, &product.Region, &product.Category,
			&product.LowestPrice, &product.Currency, &product.Available, &product.OfferCount,
		); err != nil {
			return err
		}
		product.Region = strings.TrimSpace(product.Region)
		product.Currency = strings.TrimSpace(product.Currency)
		if position, ok := positionByBlockID[blockID]; ok {
			blocks[position].Products = append(blocks[position].Products, product)
		}
	}
	return rows.Err()
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
	product, err := a.loadProductSummary(r, r.PathValue("slug"))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "product not found", "not_found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load product", "internal_error", nil)
		return
	}

	product.Offers, err = a.loadOffersForProduct(r, product.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load offers", "internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (a *App) loadProductSummary(r *http.Request, slug string) (productDetail, error) {
	var product productDetail
	//SQL fetches one product by slug and computes summary values from its active offers: lowest listed price, a currency,
	//whether any offer has an unsold digital key, and how many active offers exist
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
		return productDetail{}, err
	}
	product.Region = strings.TrimSpace(product.Region)
	product.Currency = strings.TrimSpace(product.Currency)
	return product, nil
}

func (a *App) loadOffersForProduct(r *http.Request, productID string) ([]offerView, error) {
	//shows active offers for product and available keys from each seller
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
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	offers := make([]offerView, 0)
	for rows.Next() {
		var offer offerView
		if err := rows.Scan(&offer.ID, &offer.SellerName, &offer.SellerRating, &offer.PriceCents, &offer.Currency, &offer.AvailableKeys); err != nil {
			return nil, err
		}
		offer.Currency = strings.TrimSpace(offer.Currency)
		offer.Available = offer.AvailableKeys > 0
		offers = append(offers, offer)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return offers, nil
}
