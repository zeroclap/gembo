package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "test-secret"

// testToken returns a signed JWT token suitable for use in tests.
func testToken() string {
	claims := jwt.MapClaims{
		"sub":   "test-user-id",
		"email": "test@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(testJWTSecret))
	if err != nil {
		panic("testToken: " + err.Error())
	}
	return signed
}

// newAuthRequest is a drop-in replacement for httptest.NewRequest that
// automatically attaches a valid Bearer token so protected routes pass auth.
func newAuthRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer "+testToken())
	return req
}
