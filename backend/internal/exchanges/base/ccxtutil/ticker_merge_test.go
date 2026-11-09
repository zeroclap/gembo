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

// A merged ticker must carry the latest source and receive time, otherwise a
// live feed would be flagged stale after its first update.
func TestMerge_AdvancesSourceAndReceiveTime(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	sym := "BTC/USDT:USDT"

	ApplyBidAskUpdate(state, domain.Ticker{
		Symbol: sym, Bid: decimal.NewFromInt(99), Ask: decimal.NewFromInt(101),
		SourceTime: t0, ReceivedAt: t0.Add(50 * time.Millisecond), UpdatedAt: t0.Add(50 * time.Millisecond),
	})

	// A price-only update must not touch the bid/ask source time but does
	// advance the receive time.
	merged := ApplyTickerUpdate(state, domain.Ticker{
		Symbol: sym, Price: decimal.NewFromInt(100),
		SourceTime: t0.Add(time.Second), ReceivedAt: t0.Add(time.Second), UpdatedAt: t0.Add(time.Second),
	})
	if !merged.SourceTime.Equal(t0) {
		t.Errorf("SourceTime after price-only update = %v, want %v", merged.SourceTime, t0)
	}
	if !merged.ReceivedAt.Equal(t0.Add(time.Second)) || !merged.UpdatedAt.Equal(t0.Add(time.Second)) {
		t.Errorf("ReceivedAt/UpdatedAt = %v/%v, want %v", merged.ReceivedAt, merged.UpdatedAt, t0.Add(time.Second))
	}

	// A ticker update carrying bid/ask replaces the source time.
	merged = ApplyTickerUpdate(state, domain.Ticker{
		Symbol: sym, Price: decimal.NewFromInt(100),
		Bid: decimal.NewFromInt(100), Ask: decimal.NewFromInt(102),
		SourceTime: t0.Add(2 * time.Second), ReceivedAt: t0.Add(2 * time.Second),
	})
	if !merged.SourceTime.Equal(t0.Add(2 * time.Second)) {
		t.Errorf("SourceTime after bid/ask-bearing ticker update = %v", merged.SourceTime)
	}

	// A bid/ask update advances source and receive time.
	merged = ApplyBidAskUpdate(state, domain.Ticker{
		Symbol: sym, Bid: decimal.NewFromInt(101), Ask: decimal.NewFromInt(103),
		SourceTime: t0.Add(3 * time.Second), ReceivedAt: t0.Add(3 * time.Second),
	})
	if !merged.SourceTime.Equal(t0.Add(3*time.Second)) || !merged.ReceivedAt.Equal(t0.Add(3*time.Second)) {
		t.Errorf("SourceTime/ReceivedAt = %v/%v after bid/ask update", merged.SourceTime, merged.ReceivedAt)
	}

	// An exchange that omits the timestamp must not inherit an older one: the
	// quote is then reported as missing its source time, not as fresh.
	merged = ApplyBidAskUpdate(state, domain.Ticker{
		Symbol: sym, Bid: decimal.NewFromInt(102), Ask: decimal.NewFromInt(104),
		ReceivedAt: t0.Add(4 * time.Second),
	})
	if !merged.SourceTime.IsZero() {
		t.Errorf("SourceTime = %v, want zero when the update has no timestamp", merged.SourceTime)
	}
}
