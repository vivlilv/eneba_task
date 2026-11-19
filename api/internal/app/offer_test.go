package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type offerPreviewResponse struct {
	ID            string `json:"id"`
	ProductSlug   string `json:"productSlug"`
	ProductTitle  string `json:"productTitle"`
	Platform      string `json:"platform"`
	Region        string `json:"region"`
	SellerName    string `json:"sellerName"`
	PriceCents    int64  `json:"priceCents"`
	Currency      string `json:"currency"`
	AvailableKeys int    `json:"availableKeys"`
	Available     bool   `json:"available"`
}

func TestOfferPreviewReturnsActiveOfferAndUnassignedKeyCount(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, target)
	seedOfferPreviewFixtures(t, db)

	recorder, body := getOfferPreview(t, target, "o-active")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, body)
	}
	var offer offerPreviewResponse
	if err := json.Unmarshal(body, &offer); err != nil {
		t.Fatalf("decode response body (%s): %v", body, err)
	}
	if offer.ID != "o-active" || offer.ProductSlug != "offer-product" || offer.ProductTitle != "Offer Product" ||
		offer.Platform != "PC" || offer.Region != "Europe" || offer.SellerName != "Offer Seller" ||
		offer.PriceCents != 1999 || offer.Currency != "EUR" {
		t.Fatalf("offer fields = %+v", offer)
	}
	if offer.AvailableKeys != 1 || !offer.Available {
		t.Fatalf("availableKeys/available = %d/%v, want 1/true", offer.AvailableKeys, offer.Available)
	}
}

func TestOfferPreviewHidesInactiveAndMissingOffers(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, target)
	seedOfferPreviewFixtures(t, db)

	for _, offerID := range []string{"o-inactive", "missing-offer"} {
		recorder, body := getOfferPreview(t, target, offerID)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("offer %q status = %d, want %d; body: %s", offerID, recorder.Code, http.StatusNotFound, body)
		}
		var apiError struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(body, &apiError); err != nil {
			t.Fatalf("decode error response (%s): %v", body, err)
		}
		if apiError.Code != "not_found" {
			t.Fatalf("offer %q error code = %q, want not_found", offerID, apiError.Code)
		}
	}
}

func TestOfferPreviewDatabaseUnavailable(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://eneba:eneba_dev@127.0.0.1:1/eneba?sslmode=disable")
	if err != nil {
		t.Fatalf("open unavailable database: %v", err)
	}
	defer db.Close()

	target := New(db, false)
	recorder, body := getOfferPreview(t, target, "o-active")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusInternalServerError, body)
	}
}

func getOfferPreview(t *testing.T, target *App, offerID string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/offers/"+offerID, nil)
	target.Handler().ServeHTTP(recorder, request)
	return recorder, recorder.Body.Bytes()
}

func seedOfferPreviewFixtures(t *testing.T, db *sql.DB) {
	t.Helper()
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description)
		 VALUES ('p-offer', 'offer-product', 'Offer Product', 'Standard', 'PC', 'Europe', 'Puzzle', 'Offer fixture.')`,
		`INSERT INTO sellers (id, name, rating) VALUES
		 ('s-offer', 'Offer Seller', 4.80),
		 ('s-inactive', 'Inactive Seller', 4.50)`,
		`INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active) VALUES
		 ('o-active', 'p-offer', 's-offer', 1999, 'EUR', TRUE),
		 ('o-inactive', 'p-offer', 's-inactive', 999, 'EUR', FALSE)`,
		`INSERT INTO digital_keys (offer_id, code) VALUES
		 ('o-active', 'OFFER-FREE-KEY'),
		 ('o-active', 'OFFER-ASSIGNED-KEY')`,
		`INSERT INTO users (email, password_hash) VALUES ('offer-buyer@example.com', 'unused')`,
		`INSERT INTO orders (id, user_id, status, currency, total_cents, checkout_key)
		 VALUES ('33333333-3333-3333-3333-333333333333',
		         (SELECT id FROM users WHERE email = 'offer-buyer@example.com'),
		         'paid', 'EUR', 1999, 'offer-test-checkout')`,
		`INSERT INTO order_items (order_id, product_id, offer_id, product_title_snapshot,
		                         seller_name_snapshot, unit_price_cents, digital_key_id)
		 SELECT o.id, 'p-offer', 'o-active', 'Offer Product', 'Offer Seller', 1999, dk.id
		 FROM orders o JOIN digital_keys dk ON dk.code = 'OFFER-ASSIGNED-KEY'
		 WHERE o.checkout_key = 'offer-test-checkout'`,
	})
}
