// Package ccxtutil provides shared utilities for converting CCXT types to domain
// models. All exchange adapters delegate to these functions instead of maintaining
// their own copies.
package ccxtutil

import (
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// FloatToDecimal safely converts a *float64 to decimal.Decimal.
// Returns decimal.Zero when f is nil.
func FloatToDecimal(f *float64) decimal.Decimal {
	if f == nil {
		return decimal.Zero
	}
	return decimal.NewFromFloat(*f)
}

// DerefString safely dereferences a *string, returning "" when nil.
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DerefBool safely dereferences a *bool, returning false when nil.
func DerefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// TimestampToTime converts a CCXT *float64 millisecond timestamp to time.Time.
// Returns zero time when ts is nil or zero.
func TimestampToTime(ts *float64) time.Time {
	if ts == nil || *ts == 0 {
		return time.Time{}
	}
	sec := int64(*ts) / 1000
	ns := (int64(*ts) % 1000) * int64(time.Millisecond)
	return time.Unix(sec, ns).UTC()
}

// IntervalFromString parses CCXT funding interval strings like "4h" or "8h"
// into an integer number of hours.
func IntervalFromString(s *string) int {
	if s == nil {
		return 0
	}
	switch *s {
	case "1h":
		return 1
	case "2h":
		return 2
	case "4h":
		return 4
	case "8h":
		return 8
	case "12h":
		return 12
	case "24h":
		return 24
	}
	return 0
}

// NormalizePositionSide converts a raw CCXT position side string to a
// domain.PositionSide. Unknown values are mapped to PositionSideNone and
// logged as a warning so new exchange quirks are visible without panicking.
func NormalizePositionSide(raw string) domain.PositionSide {
	switch strings.ToLower(raw) {
	case "long":
		return domain.PositionSideLong
	case "short":
		return domain.PositionSideShort
	case "", "none", "net", "flat":
		return domain.PositionSideNone
	default:
		slog.Warn("unknown position side from exchange, treating as none",
			"raw_side", raw)
		return domain.PositionSideNone
	}
}

// NormalizeOrderSide converts a raw CCXT order side string to a
// domain.OrderSide. Returns the zero value and logs a warning for
// unrecognised values.
func NormalizeOrderSide(raw string) domain.OrderSide {
	switch strings.ToLower(raw) {
	case "buy":
		return domain.OrderSideBuy
	case "sell":
		return domain.OrderSideSell
	default:
		slog.Warn("unknown order side from exchange",
			"raw_side", raw)
		return ""
	}
}
