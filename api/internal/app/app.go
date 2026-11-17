package app

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const sessionCookieName = "eneba_session"

type App struct {
	db           *sql.DB
	cookieSecure bool
	mux          *http.ServeMux
}

type user struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type apiError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

func New(db *sql.DB, cookieSecure bool) *App {
	a := &App{db: db, cookieSecure: cookieSecure, mux: http.NewServeMux()}
	a.routes()
	return a
}

func (a *App) Handler() http.Handler {
	return a.mux
}

func (a *App) routes() {
	a.mux.HandleFunc("GET /api/health", a.handleHealth)
	a.mux.HandleFunc("GET /api/homepage", a.handleHomepage)
	a.mux.HandleFunc("GET /api/products/{slug}", a.handleProduct)
	a.mux.HandleFunc("GET /api/offers/{offerID}", a.handleOfferPreview)
	a.mux.HandleFunc("POST /api/auth/register", a.handleRegister)
	a.mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	a.mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	a.mux.HandleFunc("GET /api/me", a.handleMe)
	a.mux.HandleFunc("GET /api/cart", a.handleGetCart)
	a.mux.HandleFunc("POST /api/cart/items", a.handleAddCartItem)
	a.mux.HandleFunc("POST /api/cart/merge", a.handleMergeCart)
	a.mux.HandleFunc("POST /api/cart/confirm-prices", a.handleConfirmPrices)
	a.mux.HandleFunc("DELETE /api/cart/items/{offerID}", a.handleDeleteCartItem)
	a.mux.HandleFunc("POST /api/checkout", a.handleCheckout)
	a.mux.HandleFunc("POST /api/orders/{orderID}/pay", a.handlePay)
	a.mux.HandleFunc("GET /api/orders", a.handleListOrders)
	a.mux.HandleFunc("GET /api/orders/{orderID}", a.handleOrder)
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := a.db.PingContext(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable", "database_unavailable", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) requireUser(w http.ResponseWriter, r *http.Request) (user, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "sign in required", "unauthenticated", nil)
		return user{}, false
	}

	tokenHash := sha256.Sum256([]byte(cookie.Value))
	var current user
	err = a.db.QueryRowContext(r.Context(), `
		SELECT u.id, u.email
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, tokenHash[:]).Scan(&current.ID, &current.Email)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "sign in required", "unauthenticated", nil)
		return user{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not verify session", "internal_error", nil)
		return user{}, false
	}
	return current, true
}

func (a *App) createSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if _, err := a.db.ExecContext(r.Context(), `
		INSERT INTO sessions(token_hash, user_id, expires_at) VALUES ($1, $2, $3)
	`, tokenHash[:], userID, expiresAt); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *App) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		tokenHash := sha256.Sum256([]byte(cookie.Value))
		_, _ = a.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, tokenHash[:])
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message, code string, details any) {
	writeJSON(w, status, apiError{Error: message, Code: code, Details: details})
}
