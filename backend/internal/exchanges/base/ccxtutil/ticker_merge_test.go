package ccxtutil

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeroclap/gembo/internal/domain"
)

func TestApplyTickerUpdate_NewSymbol(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	ticker := domain.Ticker{
		ExchangeID: "test",
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42000),
		High24h:    decimal.NewFromFloat(43000),
		Low24h:     decimal.NewFromFloat(41000),
		Volume24h:  decimal.NewFromFloat(1000),
		Change24h:  decimal.NewFromFloat(2.5),
		UpdatedAt:  now,
	}

	merged := ApplyTickerUpdate(state, ticker)

	if merged.Symbol != "BTC/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", merged.Symbol, "BTC/USDT:USDT")
	}
	if !merged.Price.Equal(decimal.NewFromFloat(42000)) {
		t.Errorf("Price = %s, want 42000", merged.Price)
	}
	if !merged.Bid.IsZero() {
		t.Errorf("Bid = %s, want 0", merged.Bid)
	}
}

func TestApplyTickerUpdate_PreservesBidAsk(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	// Seed bid/ask first.
	ApplyBidAskUpdate(state, domain.Ticker{
		ExchangeID: "test",
		Symbol:     "BTC/USDT:USDT",
		Bid:        decimal.NewFromFloat(41999),
		Ask:        decimal.NewFromFloat(42001),
		UpdatedAt:  now,
	})

	// Ticker update with zero bid/ask should preserve.
	merged := ApplyTickerUpdate(state, domain.Ticker{
		ExchangeID: "test",
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42000),
		UpdatedAt:  now,
	})

	if !merged.Bid.Equal(decimal.NewFromFloat(41999)) {
		t.Errorf("Bid = %s, want 41999 (preserved)", merged.Bid)
	}
	if !merged.Ask.Equal(decimal.NewFromFloat(42001)) {
		t.Errorf("Ask = %s, want 42001 (preserved)", merged.Ask)
	}
}

func TestApplyBidAskUpdate_PreservesTickerFields(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	// Seed ticker first.
	ApplyTickerUpdate(state, domain.Ticker{
		ExchangeID: "test",
		Symbol:     "ETH/USDT:USDT",
		Price:      decimal.NewFromFloat(3500),
		High24h:    decimal.NewFromFloat(3600),
		Volume24h:  decimal.NewFromFloat(5000),
		UpdatedAt:  now,
	})

	// Bid/ask update should preserve price fields.
	merged := ApplyBidAskUpdate(state, domain.Ticker{
		ExchangeID: "test",
		Symbol:     "ETH/USDT:USDT",
		Bid:        decimal.NewFromFloat(3499),
		Ask:        decimal.NewFromFloat(3501),
		UpdatedAt:  now,
	})

	if !merged.Price.Equal(decimal.NewFromFloat(3500)) {
		t.Errorf("Price = %s, want 3500 (preserved)", merged.Price)
	}
	if !merged.Bid.Equal(decimal.NewFromFloat(3499)) {
		t.Errorf("Bid = %s, want 3499", merged.Bid)
	}
}
