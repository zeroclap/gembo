package ccxtutil

import (
	"math"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
)

const (
	// MaxBackoff is the ceiling for exponential backoff.
	MaxBackoff = 60 * time.Second
	// InitialBackoff is the starting backoff duration.
	InitialBackoff = 1 * time.Second
)

// IsRetryable returns true for transient CCXT errors that warrant a retry.
// Unknown (non-CCXT) errors are treated as retryable to be optimistic.
func IsRetryable(err error) bool {
	ccxtErr, ok := err.(*ccxt.Error)
	if !ok {
		return true // unknown errors — be optimistic
	}
	switch ccxtErr.Type {
	case "NetworkError", "RateLimitExceeded", "ExchangeNotAvailable",
		"DDoSProtection", "RequestTimeout", "OnMaintenance":
		return true
	}
	return false
}

// NextBackoff returns the next exponential backoff duration using the golden
// ratio (math.Phi ≈ 1.618) as the multiplier, capped at maxBackoff.
func NextBackoff(d, maxBackoff time.Duration) time.Duration {
	next := time.Duration(float64(d) * math.Phi)
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}
