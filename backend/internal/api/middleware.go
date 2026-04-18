package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type authContextKey string

const authUserContextKey authContextKey = "auth_user"

type authUser struct {
	ID    string
	Email string
}

func userFromContext(ctx context.Context) (authUser, bool) {
	u, ok := ctx.Value(authUserContextKey).(authUser)
	return u, ok
}

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

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid token claims")
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid token subject")
			return
		}

		email, _ := claims["email"].(string)
		ctx := context.WithValue(r.Context(), authUserContextKey, authUser{ID: userID, Email: email})

		next(w, r.WithContext(ctx))
	}
}
