package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type authResponse struct {
	User struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

func TestRegisterLoginLogoutAndCurrentBuyerFlow(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)

	registerRecorder := sendAuthRequest(t, target, http.MethodPost, "/api/auth/register",
		`{"email":" Buyer@Example.com ","password":"a-strong-password"}`, nil)
	if registerRecorder.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d; body: %s", registerRecorder.Code, http.StatusCreated, registerRecorder.Body)
	}
	var registered authResponse
	if err := json.Unmarshal(registerRecorder.Body.Bytes(), &registered); err != nil {
		t.Fatalf("decode registration response (%s): %v", registerRecorder.Body, err)
	}
	if registered.User.ID == 0 || registered.User.Email != "buyer@example.com" {
		t.Fatalf("registered user = %+v, want normalized email and ID", registered.User)
	}
	registerCookies := registerRecorder.Result().Cookies()
	if len(registerCookies) != 1 || registerCookies[0].Name != sessionCookieName || !registerCookies[0].HttpOnly {
		t.Fatalf("registration cookies = %+v, want one HttpOnly session cookie", registerCookies)
	}

	meRecorder := sendAuthRequest(t, target, http.MethodGet, "/api/me", "", registerCookies[0])
	assertCurrentBuyer(t, meRecorder, registered.User.ID, "buyer@example.com")

	logoutRecorder := sendAuthRequest(t, target, http.MethodPost, "/api/auth/logout", "", registerCookies[0])
	if logoutRecorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutRecorder.Code, http.StatusNoContent)
	}
	unauthenticatedRecorder := sendAuthRequest(t, target, http.MethodGet, "/api/me", "", registerCookies[0])
	if unauthenticatedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("/api/me after logout status = %d, want %d; body: %s", unauthenticatedRecorder.Code, http.StatusUnauthorized, unauthenticatedRecorder.Body)
	}

	loginRecorder := sendAuthRequest(t, target, http.MethodPost, "/api/auth/login",
		`{"email":"BUYER@example.com","password":"a-strong-password"}`, nil)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body: %s", loginRecorder.Code, http.StatusOK, loginRecorder.Body)
	}
	var loggedIn authResponse
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &loggedIn); err != nil {
		t.Fatalf("decode login response (%s): %v", loginRecorder.Body, err)
	}
	if loggedIn.User.ID != registered.User.ID || loggedIn.User.Email != "buyer@example.com" {
		t.Fatalf("logged-in user = %+v, want registered user %+v", loggedIn.User, registered.User)
	}
}

func TestRegistrationAndLoginRejectInvalidCredentials(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)

	invalidRegistration := sendAuthRequest(t, target, http.MethodPost, "/api/auth/register",
		`{"email":"not-an-email","password":"short"}`, nil)
	if invalidRegistration.Code != http.StatusBadRequest {
		t.Fatalf("invalid registration status = %d, want %d", invalidRegistration.Code, http.StatusBadRequest)
	}

	validRegistration := sendAuthRequest(t, target, http.MethodPost, "/api/auth/register",
		`{"email":"duplicate@example.com","password":"a-strong-password"}`, nil)
	if validRegistration.Code != http.StatusCreated {
		t.Fatalf("setup registration status = %d, want %d; body: %s", validRegistration.Code, http.StatusCreated, validRegistration.Body)
	}
	duplicateRegistration := sendAuthRequest(t, target, http.MethodPost, "/api/auth/register",
		`{"email":"duplicate@example.com","password":"a-strong-password"}`, nil)
	if duplicateRegistration.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d, want %d", duplicateRegistration.Code, http.StatusConflict)
	}
	wrongPassword := sendAuthRequest(t, target, http.MethodPost, "/api/auth/login",
		`{"email":"duplicate@example.com","password":"wrong-password"}`, nil)
	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-password login status = %d, want %d", wrongPassword.Code, http.StatusUnauthorized)
	}
}

func sendAuthRequest(t *testing.T, target *App, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	target.Handler().ServeHTTP(recorder, request)
	return recorder
}

func assertCurrentBuyer(t *testing.T, recorder *httptest.ResponseRecorder, wantID int64, wantEmail string) {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("/api/me status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	var response authResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode /api/me response (%s): %v", recorder.Body, err)
	}
	if response.User.ID != wantID || response.User.Email != wantEmail {
		t.Fatalf("/api/me user = %+v, want ID/email %d/%q", response.User, wantID, wantEmail)
	}
}
