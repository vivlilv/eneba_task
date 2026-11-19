package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type cartResponse struct {
	Items []struct {
		OfferID        string `json:"offerId"`
		PriceCents     int64  `json:"priceCents"`
		PriceSeenCents int64  `json:"priceSeenCents"`
		AvailableKeys  int    `json:"availableKeys"`
		PriceChanged   bool   `json:"priceChanged"`
	} `json:"items"`
	TotalCents      int64 `json:"totalCents"`
	HasPriceChanges bool  `json:"hasPriceChanges"`
}

type cartTestFixture struct {
	app     *App
	db      *sql.DB
	buyerID int64
	cookie  *http.Cookie
}

func TestCartAddReadConfirmAndDeleteFlow(t *testing.T) {
	fixture := newCartTestFixture(t)

	added := sendCartRequest(t, fixture, http.MethodPost, "/api/cart/items", `{"offerId":" o-stock ","priceSeenCents":1999}`)
	if added.Code != http.StatusOK {
		t.Fatalf("add status = %d, want %d; body: %s", added.Code, http.StatusOK, added.Body)
	}
	cart := decodeCartResponse(t, added.Body.Bytes())
	if len(cart.Items) != 1 || cart.Items[0].OfferID != "o-stock" || cart.Items[0].PriceCents != 1999 ||
		cart.Items[0].PriceSeenCents != 1999 || cart.Items[0].AvailableKeys != 1 || cart.Items[0].PriceChanged ||
		cart.TotalCents != 1999 || cart.HasPriceChanges {
		t.Fatalf("cart after add = %+v", cart)
	}

	if _, err := fixture.db.Exec(`UPDATE offers SET price_cents = 2499 WHERE id = 'o-stock'`); err != nil {
		t.Fatalf("change Offer price: %v", err)
	}
	read := sendCartRequest(t, fixture, http.MethodGet, "/api/cart", "")
	cart = decodeCartResponse(t, read.Body.Bytes())
	if len(cart.Items) != 1 || cart.Items[0].PriceCents != 2499 || cart.Items[0].PriceSeenCents != 1999 ||
		!cart.Items[0].PriceChanged || cart.TotalCents != 2499 || !cart.HasPriceChanges {
		t.Fatalf("cart after price change = %+v", cart)
	}

	confirmed := sendCartRequest(t, fixture, http.MethodPost, "/api/cart/confirm-prices", `{}`)
	cart = decodeCartResponse(t, confirmed.Body.Bytes())
	if cart.Items[0].PriceSeenCents != 2499 || cart.Items[0].PriceChanged || cart.HasPriceChanges {
		t.Fatalf("cart after price confirmation = %+v", cart)
	}

	deleted := sendCartRequest(t, fixture, http.MethodDelete, "/api/cart/items/o-stock", "")
	cart = decodeCartResponse(t, deleted.Body.Bytes())
	if cart.Items == nil || len(cart.Items) != 0 || cart.TotalCents != 0 {
		t.Fatalf("cart after delete = %+v, want empty cart", cart)
	}
}

func TestCartAddRejectsChangedInactiveAndOutOfStockOffers(t *testing.T) {
	fixture := newCartTestFixture(t)
	tests := []struct {
		name       string
		offerID    string
		seenPrice  int64
		wantStatus int
		wantCode   string
	}{
		{name: "price changed", offerID: "o-stock", seenPrice: 1000, wantStatus: http.StatusConflict, wantCode: "price_changed"},
		{name: "out of stock", offerID: "o-empty", seenPrice: 1500, wantStatus: http.StatusConflict, wantCode: "out_of_stock"},
		{name: "inactive", offerID: "o-inactive", seenPrice: 500, wantStatus: http.StatusNotFound, wantCode: "offer_unavailable"},
		{name: "missing", offerID: "missing-offer", seenPrice: 1000, wantStatus: http.StatusNotFound, wantCode: "offer_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"offerId":%q,"priceSeenCents":%d}`, test.offerID, test.seenPrice)
			recorder := sendCartRequest(t, fixture, http.MethodPost, "/api/cart/items", body)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", recorder.Code, test.wantStatus, recorder.Body)
			}
			var apiError struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &apiError); err != nil {
				t.Fatalf("decode error response (%s): %v", recorder.Body, err)
			}
			if apiError.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", apiError.Code, test.wantCode)
			}
		})
	}
	assertCartItemCountForBuyer(t, fixture, 0)
}

func TestCartMergeRollsBackUnavailableItemAndDeduplicatesValidItems(t *testing.T) {
	fixture := newCartTestFixture(t)

	unavailable := sendCartRequest(t, fixture, http.MethodPost, "/api/cart/merge",
		`{"items":[{"offerId":"o-stock","priceSeenCents":1999},{"offerId":"o-inactive","priceSeenCents":500}]}`)
	if unavailable.Code != http.StatusConflict {
		t.Fatalf("unavailable merge status = %d, want %d; body: %s", unavailable.Code, http.StatusConflict, unavailable.Body)
	}
	assertCartItemCountForBuyer(t, fixture, 0)

	merged := sendCartRequest(t, fixture, http.MethodPost, "/api/cart/merge",
		`{"items":[{"offerId":"o-stock","priceSeenCents":1999},{"offerId":" o-stock ","priceSeenCents":1000}]}`)
	if merged.Code != http.StatusOK {
		t.Fatalf("valid merge status = %d, want %d; body: %s", merged.Code, http.StatusOK, merged.Body)
	}
	cart := decodeCartResponse(t, merged.Body.Bytes())
	if len(cart.Items) != 1 || cart.Items[0].OfferID != "o-stock" || cart.Items[0].PriceSeenCents != 1999 || cart.TotalCents != 1999 {
		t.Fatalf("cart after merge = %+v, want one deduplicated item using first selected price", cart)
	}
}

func TestCartRequiresAuthentication(t *testing.T) {
	fixture := newCartTestFixture(t)
	recorder := httptest.NewRecorder()
	fixture.app.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/cart", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusUnauthorized, recorder.Body)
	}
}

func newCartTestFixture(t *testing.T) cartTestFixture {
	t.Helper()
	app := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, app)
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description) VALUES
		 ('p-cart', 'cart-product', 'Cart Product', 'Standard', 'PC', 'Europe', 'Action', 'Cart fixture.'),
		 ('p-empty', 'empty-product', 'Empty Product', 'Standard', 'PC', 'Europe', 'Action', 'Empty fixture.'),
		 ('p-inactive', 'inactive-product', 'Inactive Product', 'Standard', 'PC', 'Europe', 'Action', 'Inactive fixture.')`,
		`INSERT INTO sellers (id, name, rating) VALUES
		 ('s-stock', 'Stock Seller', 4.80),
		 ('s-empty', 'Empty Seller', 4.60),
		 ('s-inactive', 'Inactive Seller', 4.40)`,
		`INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active) VALUES
		 ('o-stock', 'p-cart', 's-stock', 1999, 'EUR', TRUE),
		 ('o-empty', 'p-empty', 's-empty', 1500, 'EUR', TRUE),
		 ('o-inactive', 'p-inactive', 's-inactive', 500, 'EUR', FALSE)`,
		`INSERT INTO digital_keys (offer_id, code) VALUES
		 ('o-stock', 'CART-STOCK-KEY'),
		 ('o-inactive', 'CART-INACTIVE-KEY')`,
	})
	var buyerID int64
	if err := db.QueryRow(`INSERT INTO users(email, password_hash) VALUES ('cart-buyer@example.com', 'unused') RETURNING id`).Scan(&buyerID); err != nil {
		t.Fatalf("create buyer: %v", err)
	}
	cookieRecorder := httptest.NewRecorder()
	if err := app.createSession(cookieRecorder, httptest.NewRequest(http.MethodPost, "/", nil), buyerID); err != nil {
		t.Fatalf("create buyer session: %v", err)
	}
	cookies := cookieRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("created %d session cookies, want 1", len(cookies))
	}
	return cartTestFixture{app: app, db: db, buyerID: buyerID, cookie: cookies[0]}
}

func sendCartRequest(t *testing.T, fixture cartTestFixture, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(fixture.cookie)
	fixture.app.Handler().ServeHTTP(recorder, request)
	return recorder
}

func decodeCartResponse(t *testing.T, body []byte) cartResponse {
	t.Helper()
	var response cartResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode cart response (%s): %v", body, err)
	}
	return response
}

func assertCartItemCountForBuyer(t *testing.T, fixture cartTestFixture, want int) {
	t.Helper()
	var count int
	if err := fixture.db.QueryRow(`SELECT COUNT(*)::int FROM cart_items WHERE user_id = $1`, fixture.buyerID).Scan(&count); err != nil {
		t.Fatalf("count buyer Cart items: %v", err)
	}
	if count != want {
		t.Fatalf("Cart item count = %d, want %d", count, want)
	}
}
