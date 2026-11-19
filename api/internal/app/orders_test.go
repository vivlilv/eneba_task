package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type paymentResponse struct {
	PaymentStatus string `json:"paymentStatus"`
	Order         struct {
		Status string `json:"status"`
		Items  []struct {
			DigitalKey *string `json:"digitalKey"`
		} `json:"items"`
	} `json:"order"`
}

type paymentFixture struct {
	app     *App
	db      *sql.DB
	cookie  *http.Cookie
	orderID string
	buyerID int64
}

func TestPaySuccessAssignsKeyAndIdempotentRetryReturnsSameOrder(t *testing.T) {
	fixture := newPaymentFixture(t, "PAY-KEY-001", "PAY-KEY-002")

	firstRecorder, firstResponse := sendPayment(t, fixture, "attempt-success", "success")
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first payment status = %d, want %d; body: %s", firstRecorder.Code, http.StatusOK, firstRecorder.Body)
	}
	assertSuccessfulPayment(t, firstResponse, "PAY-KEY-001")
	assertOrderState(t, fixture, "paid", 1)
	assertCartItemCount(t, fixture, 0)
	assertPaymentAttempt(t, fixture, "attempt-success", "succeeded")

	// Replaying the same idempotency key returns the original Payment result.
	secondRecorder, secondResponse := sendPayment(t, fixture, "attempt-success", "success")
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want %d; body: %s", secondRecorder.Code, http.StatusOK, secondRecorder.Body)
	}
	assertSuccessfulPayment(t, secondResponse, "PAY-KEY-001")
	assertOrderState(t, fixture, "paid", 1)
}

func TestPayDeclinedLeavesOrderAndKeysUnchanged(t *testing.T) {
	fixture := newPaymentFixture(t, "PAY-KEY-DECLINED")

	recorder, response := sendPayment(t, fixture, "attempt-declined", "declined")
	if recorder.Code != http.StatusOK {
		t.Fatalf("payment status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if response.PaymentStatus != "declined" || response.Order.Status != "pending_payment" {
		t.Fatalf("payment/order status = %q/%q, want declined/pending_payment", response.PaymentStatus, response.Order.Status)
	}
	if len(response.Order.Items) != 1 {
		t.Fatalf("declined Order has %d items, want 1", len(response.Order.Items))
	}
	if response.Order.Items[0].DigitalKey != nil {
		t.Fatalf("declined Order has digital key %q, want no key", *response.Order.Items[0].DigitalKey)
	}
	assertOrderState(t, fixture, "pending_payment", 0)
	assertCartItemCount(t, fixture, 1)
	assertPaymentAttempt(t, fixture, "attempt-declined", "declined")
}

func TestPayOutOfStockDoesNotRecordPaymentOrAssignKeys(t *testing.T) {
	fixture := newPaymentFixture(t)

	recorder := sendPaymentRequest(t, fixture, "attempt-out-of-stock", "success")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("payment status = %d, want %d; body: %s", recorder.Code, http.StatusConflict, recorder.Body)
	}
	var response struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response (%s): %v", recorder.Body, err)
	}
	if response.Code != "out_of_stock" {
		t.Fatalf("error code = %q, want out_of_stock", response.Code)
	}
	assertOrderState(t, fixture, "pending_payment", 0)
	assertCartItemCount(t, fixture, 1)
	assertNoPaymentAttempt(t, fixture, "attempt-out-of-stock")
}

func newPaymentFixture(t *testing.T, keyCodes ...string) paymentFixture {
	t.Helper()
	app := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, app)
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description)
		 VALUES ('p-payment', 'payment-product', 'Payment Product', 'Standard', 'PC', 'Europe', 'Action', 'Payment fixture.')`,
		`INSERT INTO sellers (id, name, rating) VALUES ('s-payment', 'Payment Seller', 4.90)`,
		`INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active)
		 VALUES ('o-payment', 'p-payment', 's-payment', 1999, 'EUR', TRUE)`,
	})
	for _, code := range keyCodes {
		if _, err := db.Exec(`INSERT INTO digital_keys(offer_id, code) VALUES ('o-payment', $1)`, code); err != nil {
			t.Fatalf("insert digital key %q: %v", code, err)
		}
	}

	var buyerID int64
	if err := db.QueryRow(`INSERT INTO users(email, password_hash) VALUES ('payment-buyer@example.com', 'unused') RETURNING id`).Scan(&buyerID); err != nil {
		t.Fatalf("create buyer: %v", err)
	}
	var orderID string
	if err := db.QueryRow(`
		INSERT INTO orders(user_id, status, currency, total_cents, checkout_key)
		VALUES ($1, 'pending_payment', 'EUR', 1999, 'payment-checkout')
		RETURNING id::text
	`, buyerID).Scan(&orderID); err != nil {
		t.Fatalf("create Order: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO order_items(order_id, product_id, offer_id, product_title_snapshot,
		                        seller_name_snapshot, unit_price_cents)
		VALUES ($1, 'p-payment', 'o-payment', 'Payment Product', 'Payment Seller', 1999)
	`, orderID); err != nil {
		t.Fatalf("create Order item: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO cart_items(user_id, offer_id, price_seen_cents) VALUES ($1, 'o-payment', 1999)`, buyerID); err != nil {
		t.Fatalf("create Cart item: %v", err)
	}

	cookieRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	if err := app.createSession(cookieRecorder, request, buyerID); err != nil {
		t.Fatalf("create buyer session: %v", err)
	}
	cookies := cookieRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("created %d session cookies, want 1", len(cookies))
	}
	return paymentFixture{app: app, db: db, cookie: cookies[0], orderID: orderID, buyerID: buyerID}
}

func sendPayment(t *testing.T, fixture paymentFixture, attemptKey, outcome string) (*httptest.ResponseRecorder, paymentResponse) {
	t.Helper()
	recorder := sendPaymentRequest(t, fixture, attemptKey, outcome)
	var response paymentResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode payment response (%s): %v", recorder.Body, err)
	}
	return recorder, response
}

func sendPaymentRequest(t *testing.T, fixture paymentFixture, attemptKey, outcome string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/orders/"+fixture.orderID+"/pay", strings.NewReader(`{"outcome":"`+outcome+`"}`))
	request.Header.Set("Idempotency-Key", attemptKey)
	request.AddCookie(fixture.cookie)
	recorder := httptest.NewRecorder()
	fixture.app.Handler().ServeHTTP(recorder, request)
	return recorder
}

func assertSuccessfulPayment(t *testing.T, response paymentResponse, wantKey string) {
	t.Helper()
	if response.PaymentStatus != "succeeded" || response.Order.Status != "paid" {
		t.Fatalf("payment/order status = %q/%q, want succeeded/paid", response.PaymentStatus, response.Order.Status)
	}
	if len(response.Order.Items) != 1 || response.Order.Items[0].DigitalKey == nil || *response.Order.Items[0].DigitalKey != wantKey {
		t.Fatalf("Order items = %+v, want one item with key %q", response.Order.Items, wantKey)
	}
}

func assertOrderState(t *testing.T, fixture paymentFixture, wantStatus string, wantAssignedKeys int) {
	t.Helper()
	var status string
	var assignedKeys int
	err := fixture.db.QueryRow(`
		SELECT o.status, COUNT(oi.digital_key_id)::int
		FROM orders o JOIN order_items oi ON oi.order_id = o.id
		WHERE o.id::text = $1
		GROUP BY o.status
	`, fixture.orderID).Scan(&status, &assignedKeys)
	if err != nil {
		t.Fatalf("load Order state: %v", err)
	}
	if status != wantStatus || assignedKeys != wantAssignedKeys {
		t.Fatalf("Order status/assigned keys = %q/%d, want %q/%d", status, assignedKeys, wantStatus, wantAssignedKeys)
	}
}

func assertCartItemCount(t *testing.T, fixture paymentFixture, want int) {
	t.Helper()
	var count int
	if err := fixture.db.QueryRow(`SELECT COUNT(*)::int FROM cart_items WHERE user_id = $1`, fixture.buyerID).Scan(&count); err != nil {
		t.Fatalf("count Cart items: %v", err)
	}
	if count != want {
		t.Fatalf("Cart item count = %d, want %d", count, want)
	}
}

func assertPaymentAttempt(t *testing.T, fixture paymentFixture, attemptKey, wantStatus string) {
	t.Helper()
	var status string
	if err := fixture.db.QueryRow(`
		SELECT status FROM payment_attempts WHERE order_id::text = $1 AND idempotency_key = $2
	`, fixture.orderID, attemptKey).Scan(&status); err != nil {
		t.Fatalf("load Payment attempt: %v", err)
	}
	if status != wantStatus {
		t.Fatalf("Payment attempt status = %q, want %q", status, wantStatus)
	}
}

func assertNoPaymentAttempt(t *testing.T, fixture paymentFixture, attemptKey string) {
	t.Helper()
	var count int
	if err := fixture.db.QueryRow(`
		SELECT COUNT(*)::int FROM payment_attempts WHERE order_id::text = $1 AND idempotency_key = $2
	`, fixture.orderID, attemptKey).Scan(&count); err != nil {
		t.Fatalf("count Payment attempts: %v", err)
	}
	if count != 0 {
		t.Fatalf("Payment attempt count = %d, want 0", count)
	}
}
