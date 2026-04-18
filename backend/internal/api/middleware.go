package api

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// requireAuth is an HTTP middleware that validates a Bearer JWT token.
// It returns 401 Unauthorized when the token is missing or invalid.
func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "authorization header must use Bearer scheme")
			return
		}

		tokenStr := authHeader[len(prefix):]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(h.jwtSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		next(w, r)
	}
}
