package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func qt(bid, ask string, src time.Time) Ticker {
	return Ticker{
		Bid:        decimal.RequireFromString(bid),
		Ask:        decimal.RequireFromString(ask),
		SourceTime: src,
	}
}

func TestTickerValidate(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	p := DefaultQualityPolicy()

	tests := []struct {
		name string
		t    Ticker
		want error
	}{
		{"valid", qt("100", "101", now.Add(-time.Second)), nil},
		{"zero bid", qt("0", "101", now), ErrNonPositiveBid},
		{"negative bid", qt("-1", "101", now), ErrNonPositiveBid},
		{"zero ask", qt("100", "0", now), ErrNonPositiveAsk},
		{"crossed", qt("102", "101", now), ErrCrossedQuote},
		{"locked", qt("101", "101", now), ErrCrossedQuote},
		{"missing source", qt("100", "101", time.Time{}), ErrMissingSource},
		{"future within skew", qt("100", "101", now.Add(p.MaxFutureSkew)), nil},
		{"future beyond skew", qt("100", "101", now.Add(p.MaxFutureSkew+time.Millisecond)), ErrSourceInFuture},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.t.Validate(now, p); !errors.Is(got, tc.want) {
				t.Fatalf("Validate = %v, want %v", got, tc.want)
			}
		})
	}
}

// A feed that stops updating must be reported stale once the threshold passes.
func TestTickerStaleFeed(t *testing.T) {
	p := DefaultQualityPolicy()
	last := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tk := qt("100", "101", last)

	if tk.IsStale(last.Add(p.MaxTickerAge), p) {
		t.Fatal("ticker at exactly the max age must not be stale")
	}
	if !tk.IsStale(last.Add(p.MaxTickerAge+time.Millisecond), p) {
		t.Fatal("ticker past the max age must be stale")
	}
	if tk.Usable(last.Add(p.MaxTickerAge+time.Second), p) {
		t.Fatal("stale ticker must not be usable")
	}
	if !tk.Usable(last.Add(time.Second), p) {
		t.Fatal("fresh valid ticker must be usable")
	}
	if got := tk.SourceAge(last.Add(3 * time.Second)); got != 3*time.Second {
		t.Fatalf("SourceAge = %v, want 3s", got)
	}
}

func TestTickerMissingSourceNotUsable(t *testing.T) {
	now := time.Now()
	tk := qt("100", "101", time.Time{})
	if tk.IsStale(now, DefaultQualityPolicy()) {
		t.Fatal("missing source time is invalid, not stale")
	}
	if tk.Usable(now, DefaultQualityPolicy()) {
		t.Fatal("ticker without source time must not be usable")
	}
}

func TestFundingRateStale(t *testing.T) {
	p := DefaultQualityPolicy()
	fetched := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	f := FundingRate{FetchedAt: fetched}

	if f.IsStale(fetched.Add(p.MaxFundingAge), p) {
		t.Fatal("funding at exactly the max age must not be stale")
	}
	if !f.IsStale(fetched.Add(p.MaxFundingAge+time.Millisecond), p) {
		t.Fatal("funding past the max age must be stale")
	}
	if !(FundingRate{}).IsStale(fetched, p) {
		t.Fatal("funding without fetch time must be stale")
	}
}
