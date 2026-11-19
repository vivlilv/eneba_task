package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type orderReadItem struct {
	DigitalKey *string `json:"digitalKey"`
}

type orderReadOrder struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Items  []orderReadItem `json:"items"`
}

type ordersListResponse struct {
	Orders []orderReadOrder `json:"orders"`
}

func TestOrderReadExposesKeysOnlyAfterSuccessfulPayment(t *testing.T) {
	fixture := newPaymentFixture(t, "ORDER-READ-KEY")

	pendingRecorder, pending := getOrder(t, fixture.app, fixture.cookie, fixture.orderID)
	if pendingRecorder.Code != http.StatusOK {
		t.Fatalf("pending Order status = %d, want %d; body: %s", pendingRecorder.Code, http.StatusOK, pendingRecorder.Body)
	}
	if pending.Status != "pending_payment" || len(pending.Items) != 1 || pending.Items[0].DigitalKey != nil {
		t.Fatalf("pending Order response = %+v, want no key", pending)
	}
	pendingList := getOrders(t, fixture.app, fixture.cookie)
	if pendingList.Code != http.StatusOK {
		t.Fatalf("pending Order list status = %d, want %d", pendingList.Code, http.StatusOK)
	}
	if got := decodeOrdersList(t, pendingList.Body.Bytes()); got.Orders == nil || len(got.Orders) != 0 {
		t.Fatalf("pending Order list = %+v, want empty list", got.Orders)
	}

	paymentRecorder, payment := sendPayment(t, fixture, "order-read-payment", "success")
	if paymentRecorder.Code != http.StatusOK {
		t.Fatalf("payment status = %d, want %d; body: %s", paymentRecorder.Code, http.StatusOK, paymentRecorder.Body)
	}
	if payment.Order.Status != "paid" {
		t.Fatalf("payment Order status = %q, want paid", payment.Order.Status)
	}

	paidRecorder, paid := getOrder(t, fixture.app, fixture.cookie, fixture.orderID)
	if paidRecorder.Code != http.StatusOK {
		t.Fatalf("paid Order status = %d, want %d; body: %s", paidRecorder.Code, http.StatusOK, paidRecorder.Body)
	}
	if paid.Status != "paid" || len(paid.Items) != 1 || paid.Items[0].DigitalKey == nil || *paid.Items[0].DigitalKey != "ORDER-READ-KEY" {
		t.Fatalf("paid Order response = %+v, want assigned key", paid)
	}
	paidList := decodeOrdersList(t, getOrders(t, fixture.app, fixture.cookie).Body.Bytes())
	if len(paidList.Orders) != 1 || paidList.Orders[0].ID != fixture.orderID || paidList.Orders[0].Items[0].DigitalKey == nil {
		t.Fatalf("paid Order list = %+v, want the paid Order with its key", paidList.Orders)
	}
}

func TestOrderReadIsScopedToOwningBuyer(t *testing.T) {
	fixture := newPaymentFixture(t, "PRIVATE-ORDER-KEY")
	if recorder, _ := sendPayment(t, fixture, "owner-payment", "success"); recorder.Code != http.StatusOK {
		t.Fatalf("owner payment status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body)
	}

	var otherBuyerID int64
	if err := fixture.db.QueryRow(`INSERT INTO users(email, password_hash) VALUES ('other-buyer@example.com', 'unused') RETURNING id`).Scan(&otherBuyerID); err != nil {
		t.Fatalf("create other buyer: %v", err)
	}
	cookieRecorder := httptest.NewRecorder()
	if err := fixture.app.createSession(cookieRecorder, httptest.NewRequest(http.MethodPost, "/", nil), otherBuyerID); err != nil {
		t.Fatalf("create other-buyer session: %v", err)
	}
	cookies := cookieRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("created %d other-buyer cookies, want 1", len(cookies))
	}

	orderRecorder, _ := getOrder(t, fixture.app, cookies[0], fixture.orderID)
	if orderRecorder.Code != http.StatusNotFound {
		t.Fatalf("other buyer Order status = %d, want %d", orderRecorder.Code, http.StatusNotFound)
	}
	orders := decodeOrdersList(t, getOrders(t, fixture.app, cookies[0]).Body.Bytes())
	if orders.Orders == nil || len(orders.Orders) != 0 {
		t.Fatalf("other buyer Orders = %+v, want empty list", orders.Orders)
	}
}

func getOrder(t *testing.T, target *App, cookie *http.Cookie, orderID string) (*httptest.ResponseRecorder, orderReadOrder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/orders/"+orderID, nil)
	request.AddCookie(cookie)
	target.Handler().ServeHTTP(recorder, request)
	var response orderReadOrder
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode Order response (%s): %v", recorder.Body, err)
	}
	return recorder, response
}

func getOrders(t *testing.T, target *App, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	request.AddCookie(cookie)
	target.Handler().ServeHTTP(recorder, request)
	return recorder
}

func decodeOrdersList(t *testing.T, body []byte) ordersListResponse {
	t.Helper()
	var response ordersListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode Orders list (%s): %v", body, err)
	}
	return response
}
