package app

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "invalid_request", nil)
		return
	}
	input.Email = normalizeEmail(input.Email)
	parsed, err := mail.ParseAddress(input.Email)
	if err != nil || parsed.Address != input.Email || len(input.Password) < 10 || len(input.Password) > 72 {
		writeError(w, http.StatusBadRequest, "enter a valid email and a password between 10 and 72 bytes", "invalid_credentials", nil)
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account", "internal_error", nil)
		return
	}
	var created user
	err = a.db.QueryRowContext(r.Context(), `
		INSERT INTO users(email, password_hash) VALUES ($1, $2)
		RETURNING id, email
	`, input.Email, string(passwordHash)).Scan(&created.ID, &created.Email)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "an account with that email already exists", "email_taken", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create account", "internal_error", nil)
		return
	}
	if err := a.createSession(w, r, created.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "account created but sign-in failed", "session_error", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": created})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "invalid_request", nil)
		return
	}
	input.Email = normalizeEmail(input.Email)
	var current user
	var passwordHash string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT id, email, password_hash FROM users WHERE email = $1
	`, input.Email).Scan(&current.ID, &current.Email, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil) {
		writeError(w, http.StatusUnauthorized, "email or password is incorrect", "invalid_credentials", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not sign in", "internal_error", nil)
		return
	}
	if err := a.createSession(w, r, current.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not sign in", "session_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": current})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.clearSession(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": current})
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
