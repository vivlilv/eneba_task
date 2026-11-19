package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type productDetailResponse struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Title            string `json:"title"`
	Edition          string `json:"edition"`
	Platform         string `json:"platform"`
	Region           string `json:"region"`
	Category         string `json:"category"`
	Description      string `json:"description"`
	LowestPriceCents int64  `json:"lowestPriceCents"`
	Currency         string `json:"currency"`
	Available        bool   `json:"available"`
	OfferCount       int    `json:"offerCount"`
	Offers           []struct {
		ID            string  `json:"id"`
		SellerName    string  `json:"sellerName"`
		SellerRating  float64 `json:"sellerRating"`
		PriceCents    int64   `json:"priceCents"`
		Currency      string  `json:"currency"`
		AvailableKeys int     `json:"availableKeys"`
		Available     bool    `json:"available"`
	} `json:"offers"`
}

func TestProductDetailReturnsProductAndActiveOffers(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, target)
	seedProductDetailFixtures(t, db)

	recorder, body := getProduct(t, target, "test-product")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, body)
	}

	var product productDetailResponse
	if err := json.Unmarshal(body, &product); err != nil {
		t.Fatalf("decode response body (%s): %v", body, err)
	}
	if product.ID != "p-main" || product.Slug != "test-product" || product.Title != "Test Product" ||
		product.Edition != "Deluxe" || product.Platform != "PC" || product.Region != "Europe" ||
		product.Category != "Adventure" || product.Description != "A fixture product." {
		t.Fatalf("product fields = %+v", product)
	}
	if product.LowestPriceCents != 1999 || product.Currency != "EUR" || !product.Available || product.OfferCount != 3 {
		t.Fatalf("product offer summary = price %d, currency %q, available %v, count %d",
			product.LowestPriceCents, product.Currency, product.Available, product.OfferCount)
	}
	if len(product.Offers) != 3 {
		t.Fatalf("len(offers) = %d, want 3 active offers; body: %s", len(product.Offers), body)
	}

	wantOfferIDs := []string{"o-low", "o-sold", "o-high"}
	for index, wantID := range wantOfferIDs {
		if product.Offers[index].ID != wantID {
			t.Fatalf("offer[%d].id = %q, want %q (offers should be ordered by price)",
				index, product.Offers[index].ID, wantID)
		}
	}
	if product.Offers[0].SellerName != "Alpha Seller" || product.Offers[0].SellerRating != 4.75 ||
		product.Offers[0].PriceCents != 1999 || product.Offers[0].Currency != "EUR" {
		t.Fatalf("cheapest offer = %+v", product.Offers[0])
	}
	if product.Offers[0].AvailableKeys != 0 || product.Offers[0].Available {
		t.Fatalf("o-low availability = %d/%v, want 0/false", product.Offers[0].AvailableKeys, product.Offers[0].Available)
	}
	if product.Offers[1].AvailableKeys != 0 || product.Offers[1].Available {
		t.Fatalf("o-sold availability = %d/%v, want 0/false after its key is assigned", product.Offers[1].AvailableKeys, product.Offers[1].Available)
	}
	if product.Offers[2].AvailableKeys != 1 || !product.Offers[2].Available {
		t.Fatalf("o-high availability = %d/%v, want 1/true", product.Offers[2].AvailableKeys, product.Offers[2].Available)
	}
}

func TestProductDetailWithoutOffersReturnsEmptyOfferListAndDefaults(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, target)
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description)
		 VALUES ('p-no-offers', 'no-offers', 'No Offers', 'Standard', 'PC', 'Global', 'Puzzle', 'No seller listings.')`,
	})

	recorder, body := getProduct(t, target, "no-offers")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, body)
	}

	var product productDetailResponse
	if err := json.Unmarshal(body, &product); err != nil {
		t.Fatalf("decode response body (%s): %v", body, err)
	}
	if product.LowestPriceCents != 0 || product.Currency != "EUR" || product.Available || product.OfferCount != 0 {
		t.Fatalf("no-offer summary = price %d, currency %q, available %v, count %d",
			product.LowestPriceCents, product.Currency, product.Available, product.OfferCount)
	}
	if product.Offers == nil || len(product.Offers) != 0 {
		t.Fatalf("offers = %#v, want an empty non-nil slice", product.Offers)
	}
}

func TestProductDetailNotFound(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)

	recorder, body := getProduct(t, target, "missing-product")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusNotFound, body)
	}
	var apiError struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(body, &apiError); err != nil {
		t.Fatalf("decode error body (%s): %v", body, err)
	}
	if apiError.Error != "product not found" || apiError.Code != "not_found" {
		t.Fatalf("error = %+v, want product not found/not_found", apiError)
	}
}

func TestProductDetailDatabaseUnavailable(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://eneba:eneba_dev@127.0.0.1:1/eneba?sslmode=disable")
	if err != nil {
		t.Fatalf("open unavailable database: %v", err)
	}
	defer db.Close()

	target := New(db, false)
	recorder, body := getProduct(t, target, "test-product")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusInternalServerError, body)
	}
	var apiError struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(body, &apiError); err != nil {
		t.Fatalf("decode error body (%s): %v", body, err)
	}
	if apiError.Error != "could not load product" || apiError.Code != "internal_error" {
		t.Fatalf("error = %+v, want could not load product/internal_error", apiError)
	}
}

func getProduct(t *testing.T, target *App, slug string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/products/"+slug, nil)
	target.Handler().ServeHTTP(recorder, request)
	return recorder, recorder.Body.Bytes()
}

func seedProductDetailFixtures(t *testing.T, db *sql.DB) {
	t.Helper()
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description)
		 VALUES ('p-main', 'test-product', 'Test Product', 'Deluxe', 'PC', ' Europe ', 'Adventure', 'A fixture product.')`,
		`INSERT INTO sellers (id, name, rating) VALUES
		 ('s-alpha', 'Alpha Seller', 4.75),
		 ('s-beta', 'Beta Seller', 4.50),
		 ('s-zeta', 'Zeta Seller', 4.90),
		 ('s-inactive', 'Inactive Seller', 4.00)`,
		`INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active) VALUES
		 ('o-high', 'p-main', 's-zeta', 2500, 'EUR', TRUE),
		 ('o-low', 'p-main', 's-alpha', 1999, 'EUR', TRUE),
		 ('o-sold', 'p-main', 's-beta', 2200, 'EUR', TRUE),
		 ('o-inactive', 'p-main', 's-inactive', 500, 'EUR', FALSE)`,
		`INSERT INTO digital_keys (offer_id, code) VALUES
		 ('o-high', 'PRODUCT-AVAILABLE-KEY'),
		 ('o-sold', 'PRODUCT-SOLD-KEY')`,
		`INSERT INTO users (email, password_hash) VALUES ('product-buyer@example.com', 'unused')`,
		`INSERT INTO orders (id, user_id, status, currency, total_cents, checkout_key)
		 VALUES ('22222222-2222-2222-2222-222222222222',
		         (SELECT id FROM users WHERE email = 'product-buyer@example.com'),
		         'paid', 'EUR', 2200, 'product-test-checkout')`,
		`INSERT INTO order_items (order_id, product_id, offer_id, product_title_snapshot,
		                         seller_name_snapshot, unit_price_cents, digital_key_id)
		 SELECT o.id, 'p-main', 'o-sold', 'Test Product', 'Beta Seller', 2200, dk.id
		 FROM orders o JOIN digital_keys dk ON dk.offer_id = 'o-sold'
		 WHERE o.checkout_key = 'product-test-checkout'`,
	})
}
