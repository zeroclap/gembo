package ccxtutil

import (
	"errors"
	"testing"

	ccxt "github.com/ccxt/ccxt/go/v4"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"non-ccxt error is retryable", errors.New("connection reset"), true},
		{"NetworkError", &ccxt.Error{Type: "NetworkError"}, true},
		{"RateLimitExceeded", &ccxt.Error{Type: "RateLimitExceeded"}, true},
		{"ExchangeNotAvailable", &ccxt.Error{Type: "ExchangeNotAvailable"}, true},
		{"DDoSProtection", &ccxt.Error{Type: "DDoSProtection"}, true},
		{"RequestTimeout", &ccxt.Error{Type: "RequestTimeout"}, true},
		{"OnMaintenance", &ccxt.Error{Type: "OnMaintenance"}, true},
		{"InvalidOrder not retryable", &ccxt.Error{Type: "InvalidOrder"}, false},
		{"ExchangeError not retryable", &ccxt.Error{Type: "ExchangeError"}, false},
		{"AuthenticationError not retryable", &ccxt.Error{Type: "AuthenticationError"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}
