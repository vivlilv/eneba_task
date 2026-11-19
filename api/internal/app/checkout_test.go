package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type checkoutResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	TotalCents int64  `json:"totalCents"`
	Items      []struct {
		OfferID        string  `json:"offerId"`
		ProductTitle   string  `json:"productTitle"`
		SellerName     string  `json:"sellerName"`
		UnitPriceCents int64   `json:"unitPriceCents"`
		DigitalKey     *string `json:"digitalKey"`
	} `json:"items"`
}

func TestCheckoutCreatesPendingOrderAndIdempotentRetryReturnsIt(t *testing.T) {
	fixture := newCartTestFixture(t)
	addCartItemForCheckout(t, fixture, "o-stock", 1999)

	firstRecorder, firstOrder := sendCheckout(t, fixture, "checkout-once", false)
	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf("first checkout status = %d, want %d; body: %s", firstRecorder.Code, http.StatusCreated, firstRecorder.Body)
	}
	assertPendingCheckout(t, firstOrder, 1999)
	assertOrderCountForBuyer(t, fixture, 1)
	assertCartItemCountForBuyer(t, fixture, 1)

	retryRecorder, retryOrder := sendCheckout(t, fixture, "checkout-once", false)
	if retryRecorder.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want %d; body: %s", retryRecorder.Code, http.StatusOK, retryRecorder.Body)
	}
	if retryOrder.ID != firstOrder.ID {
		t.Fatalf("retry Order ID = %q, want original %q", retryOrder.ID, firstOrder.ID)
	}
	assertOrderCountForBuyer(t, fixture, 1)
}

func TestCheckoutRequiresConfirmationForPriceChanges(t *testing.T) {
	fixture := newCartTestFixture(t)
	addCartItemForCheckout(t, fixture, "o-stock", 1999)
	if _, err := fixture.db.Exec(`UPDATE offers SET price_cents = 2499 WHERE id = 'o-stock'`); err != nil {
		t.Fatalf("change Offer price: %v", err)
	}

	recorder := sendCheckoutRequest(t, fixture, "checkout-price-change", false)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("unconfirmed checkout status = %d, want %d; body: %s", recorder.Code, http.StatusConflict, recorder.Body)
	}
	var apiError struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiError); err != nil {
		t.Fatalf("decode error response (%s): %v", recorder.Body, err)
	}
	if apiError.Code != "price_changed" {
		t.Fatalf("error code = %q, want price_changed", apiError.Code)
	}
	assertOrderCountForBuyer(t, fixture, 0)

	confirmedRecorder, confirmedOrder := sendCheckout(t, fixture, "checkout-price-change-confirmed", true)
	if confirmedRecorder.Code != http.StatusCreated {
		t.Fatalf("confirmed checkout status = %d, want %d; body: %s", confirmedRecorder.Code, http.StatusCreated, confirmedRecorder.Body)
	}
	assertPendingCheckout(t, confirmedOrder, 2499)
}

func TestCheckoutRejectsEmptyAndOutOfStockCart(t *testing.T) {
	emptyFixture := newCartTestFixture(t)
	emptyRecorder := sendCheckoutRequest(t, emptyFixture, "checkout-empty", false)
	assertCheckoutError(t, emptyRecorder, http.StatusConflict, "empty_cart")
	assertOrderCountForBuyer(t, emptyFixture, 0)

	stockFixture := newCartTestFixture(t)
	addCartItemForCheckout(t, stockFixture, "o-empty", 1500)
	stockRecorder := sendCheckoutRequest(t, stockFixture, "checkout-out-of-stock", false)
	assertCheckoutError(t, stockRecorder, http.StatusConflict, "out_of_stock")
	assertOrderCountForBuyer(t, stockFixture, 0)
	assertCartItemCountForBuyer(t, stockFixture, 1)
}

func TestCheckoutRequiresIdempotencyKey(t *testing.T) {
	fixture := newCartTestFixture(t)
	recorder := sendCartRequest(t, fixture, http.MethodPost, "/api/checkout", `{"confirmPriceChanges":false}`)
	assertCheckoutError(t, recorder, http.StatusBadRequest, "idempotency_key_required")
}

func addCartItemForCheckout(t *testing.T, fixture cartTestFixture, offerID string, seenPrice int64) {
	t.Helper()
	if _, err := fixture.db.Exec(`INSERT INTO cart_items(user_id, offer_id, price_seen_cents) VALUES ($1, $2, $3)`, fixture.buyerID, offerID, seenPrice); err != nil {
		t.Fatalf("insert Cart item: %v", err)
	}
}

func sendCheckout(t *testing.T, fixture cartTestFixture, idempotencyKey string, confirmPrices bool) (*httptest.ResponseRecorder, checkoutResponse) {
	t.Helper()
	recorder := sendCheckoutRequest(t, fixture, idempotencyKey, confirmPrices)
	var response checkoutResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode checkout response (%s): %v", recorder.Body, err)
	}
	return recorder, response
}

func sendCheckoutRequest(t *testing.T, fixture cartTestFixture, idempotencyKey string, confirmPrices bool) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"confirmPriceChanges":%t}`, confirmPrices)
	request := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(body))
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	request.AddCookie(fixture.cookie)
	recorder := httptest.NewRecorder()
	fixture.app.Handler().ServeHTTP(recorder, request)
	return recorder
}

func assertPendingCheckout(t *testing.T, order checkoutResponse, wantTotal int64) {
	t.Helper()
	if order.ID == "" || order.Status != "pending_payment" || order.TotalCents != wantTotal {
		t.Fatalf("Order = ID %q status %q total %d, want nonempty/pending_payment/%d", order.ID, order.Status, order.TotalCents, wantTotal)
	}
	if len(order.Items) != 1 || order.Items[0].OfferID != "o-stock" || order.Items[0].UnitPriceCents != wantTotal ||
		order.Items[0].ProductTitle != "Cart Product" || order.Items[0].SellerName != "Stock Seller" || order.Items[0].DigitalKey != nil {
		t.Fatalf("Order items = %+v, want one snapshotted item with no assigned key", order.Items)
	}
}

func assertOrderCountForBuyer(t *testing.T, fixture cartTestFixture, want int) {
	t.Helper()
	var count int
	if err := fixture.db.QueryRow(`SELECT COUNT(*)::int FROM orders WHERE user_id = $1`, fixture.buyerID).Scan(&count); err != nil {
		t.Fatalf("count buyer Orders: %v", err)
	}
	if count != want {
		t.Fatalf("Order count = %d, want %d", count, want)
	}
}

func assertCheckoutError(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("checkout status = %d, want %d; body: %s", recorder.Code, wantStatus, recorder.Body)
	}
	var apiError struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiError); err != nil {
		t.Fatalf("decode checkout error (%s): %v", recorder.Body, err)
	}
	if apiError.Code != wantCode {
		t.Fatalf("checkout error code = %q, want %q", apiError.Code, wantCode)
	}
}

func TestSummarizeCheckoutLines(t *testing.T) {
	lines := []checkoutLine{
		{OfferID: "o-same-price", PriceCents: 1000, PriceSeenCents: 1000, Currency: "EUR", Active: true, AvailableKeys: 2},
		{OfferID: "o-price-change", PriceCents: 1500, PriceSeenCents: 1200, Currency: "EUR", Active: true, AvailableKeys: 1},
		{OfferID: "o-inactive", PriceCents: 700, PriceSeenCents: 700, Currency: "EUR", Active: false, AvailableKeys: 1},
		{OfferID: "o-out-of-stock", PriceCents: 800, PriceSeenCents: 800, Currency: "EUR", Active: true, AvailableKeys: 0},
	}

	got := summarizeCheckoutLines(lines)
	if got.totalCents != 4000 {
		t.Fatalf("totalCents = %d, want 4000", got.totalCents)
	}
	if len(got.priceChanges) != 1 || got.priceChanges[0] != (priceChange{
		OfferID: "o-price-change", PreviousCents: 1200, CurrentCents: 1500, Currency: "EUR",
	}) {
		t.Fatalf("priceChanges = %+v, want only o-price-change", got.priceChanges)
	}
	if len(got.unavailableOfferIDs) != 2 || got.unavailableOfferIDs[0] != "o-inactive" || got.unavailableOfferIDs[1] != "o-out-of-stock" {
		t.Fatalf("unavailableOfferIDs = %v, want inactive and out-of-stock Offers", got.unavailableOfferIDs)
	}
}
