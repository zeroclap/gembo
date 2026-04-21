package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// handleLogin authenticates a user by email/password and returns a JWT.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "missing_fields", "email and password are required")
		return
	}

	if h.userStore == nil {
		writeErr(w, http.StatusServiceUnavailable, "auth_unavailable", "authentication requires a database connection")
		return
	}

	user, err := h.userStore.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to look up user")
		return
	}
	if user == nil {
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(h.tokenTTL).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to sign token")
		return
	}

	writeOK(w, loginResponse{Token: signed})
}
