package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type seededProductOffers struct {
	sellerCount  int
	offerCount   int
	minimumPrice int64
	maximumPrice int64
	minimumKeys  int
	maximumKeys  int
}

func TestSeededHomepageReplacesFeaturedRowsWithCheapGames(t *testing.T) {
	app := newTestAppWithSeededScratchDatabase(t)
	recorder, body := getHomepage(t, app)
	if recorder.Code != http.StatusOK {
		t.Fatalf("homepage status = %d, want %d; body: %s", recorder.Code, http.StatusOK, body)
	}

	var response homepageResponseJSON
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode homepage response: %v", err)
	}
	if len(response.Blocks) != 3 {
		t.Fatalf("homepage blocks = %d, want hero, cheap games, and more to discover", len(response.Blocks))
	}
	if response.Blocks[0].ID != "block-hero" || response.Blocks[0].CTAURL != "#block-cheap-games" {
		t.Errorf("hero block = %+v, want CTA to cheap games", response.Blocks[0])
	}
	cheapGames := response.Blocks[1]
	if cheapGames.ID != "block-cheap-games" || cheapGames.Title != "Cheap games" || cheapGames.Kind != "cheap_games" {
		t.Fatalf("second homepage block = %+v, want cheap games", cheapGames)
	}
	if len(cheapGames.Products) != 2 || cheapGames.Products[0].Slug != "hollow-knight-pc" || cheapGames.Products[1].Slug != "terraria-pc" {
		t.Fatalf("cheap games = %+v, want Hollow Knight and Terraria", cheapGames.Products)
	}
	if response.Blocks[2].ID != "block-more-to-discover" {
		t.Errorf("third homepage block id = %q, want block-more-to-discover", response.Blocks[2].ID)
	}
}

func TestSeededCatalogOfferCountsInventoryAndPriceSpread(t *testing.T) {
	app := newTestAppWithSeededScratchDatabase(t)
	db := appDatabase(t, app)

	rows, err := db.Query(`
		WITH per_offer AS (
			SELECT p.slug, o.id, o.seller_id, o.price_cents,
			       COUNT(dk.id) FILTER (WHERE oi.id IS NULL)::int AS available_keys
			FROM products p
			JOIN offers o ON o.product_id = p.id AND o.active = TRUE
			LEFT JOIN digital_keys dk ON dk.offer_id = o.id
			LEFT JOIN order_items oi ON oi.digital_key_id = dk.id
			WHERE p.listed = TRUE
			GROUP BY p.slug, o.id
		)
		SELECT slug, COUNT(*), COUNT(DISTINCT seller_id), MIN(price_cents), MAX(price_cents),
		       MIN(available_keys), MAX(available_keys)
		FROM per_offer
		GROUP BY slug
	`)
	if err != nil {
		t.Fatalf("query seeded offer summary: %v", err)
	}
	defer rows.Close()

	products := make(map[string]seededProductOffers)
	for rows.Next() {
		var slug string
		var summary seededProductOffers
		if err := rows.Scan(
			&slug, &summary.offerCount, &summary.sellerCount,
			&summary.minimumPrice, &summary.maximumPrice,
			&summary.minimumKeys, &summary.maximumKeys,
		); err != nil {
			t.Fatalf("scan seeded offer summary: %v", err)
		}
		products[slug] = summary
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read seeded offer summaries: %v", err)
	}

	wantSlugs := []string{
		"hollow-knight-pc",
		"hollow-knight-silksong-pc",
		"minecraft-pc",
		"terraria-pc",
		"grand-theft-auto-vi-ps5",
		"sekiro-shadows-die-twice-pc",
	}
	if len(products) != len(wantSlugs) {
		t.Fatalf("listed products with offers = %d, want %d: %+v", len(products), len(wantSlugs), products)
	}

	var retiredProductCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM products p
		WHERE p.slug IN ('demo-adventure-pc-eu', 'demo-racer-console-eu')
		  AND p.listed = FALSE
		  AND NOT EXISTS (SELECT 1 FROM offers o WHERE o.product_id = p.id AND o.active = TRUE)
	`).Scan(&retiredProductCount); err != nil {
		t.Fatalf("count retired demo products: %v", err)
	}
	if retiredProductCount != 2 {
		t.Fatalf("retired demo products = %d, want 2", retiredProductCount)
	}

	for _, slug := range wantSlugs {
		summary, exists := products[slug]
		if !exists {
			t.Errorf("listed product %q has no active offers", slug)
			continue
		}
		if summary.offerCount != 4 || summary.sellerCount != 4 {
			t.Errorf("%s has %d offers from %d sellers, want 4 each", slug, summary.offerCount, summary.sellerCount)
		}
		if summary.minimumKeys < 2 || summary.maximumKeys > 10 {
			t.Errorf("%s available keys range from %d to %d, want 2–10", slug, summary.minimumKeys, summary.maximumKeys)
		}
		spreadPercent := float64(summary.maximumPrice-summary.minimumPrice) / float64(summary.minimumPrice) * 100
		if spreadPercent < 10 || spreadPercent > 20 {
			t.Errorf("%s offer price spread is %.2f%%, want 10–20%%", slug, spreadPercent)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/products/demo-adventure-pc-eu", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Errorf("retired product endpoint status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
