package arbitrage

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// mockStore is a minimal in-memory implementation of store.Store for testing
// the arbitrage calculator. Only the methods used by Calculator are implemented;
// the rest panic if called.
type mockStore struct {
	mu sync.Mutex

	fundingRates map[string][]domain.FundingRate // keyed by exchangeID
	tickers      map[string][]domain.Ticker      // keyed by exchangeID
	markets      map[string][]domain.Market      // keyed by exchangeID

	// arbitrage stores keyed by "symbol|sortedExA|sortedExB"
	arbitrages map[string]domain.FundingArbitrage
}

func newMockStore() *mockStore {
	return &mockStore{
		fundingRates: make(map[string][]domain.FundingRate),
		tickers:      make(map[string][]domain.Ticker),
		markets:      make(map[string][]domain.Market),
		arbitrages:   make(map[string]domain.FundingArbitrage),
	}
}

func (m *mockStore) ListFundingRates(_ context.Context, exchangeID string) ([]domain.FundingRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fundingRates[exchangeID], nil
}

func (m *mockStore) ListTickers(_ context.Context, exchangeID string) ([]domain.Ticker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tickers[exchangeID], nil
}

func (m *mockStore) ListMarkets(_ context.Context, exchangeID string) ([]domain.Market, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.markets[exchangeID], nil
}

func sortedKey(symbol, a, b string) string {
	if a <= b {
		return symbol + "|" + a + "|" + b
	}
	return symbol + "|" + b + "|" + a
}

func (m *mockStore) SetFundingArbitrage(_ context.Context, a domain.FundingArbitrage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sortedKey(a.Symbol, a.Long.ExchangeID, a.Short.ExchangeID)
	m.arbitrages[key] = a
	return nil
}

func (m *mockStore) ListFundingArbitrage(_ context.Context) ([]domain.FundingArbitrage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.FundingArbitrage, 0, len(m.arbitrages))
	for _, a := range m.arbitrages {
		out = append(out, a)
	}
	return out, nil
}

func (m *mockStore) GetFundingArbitrage(_ context.Context, symbol, exchangeA, exchangeB string) (*domain.FundingArbitrage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sortedKey(symbol, exchangeA, exchangeB)
	a, ok := m.arbitrages[key]
	if !ok {
		return nil, nil
	}
	return &a, nil
}

func (m *mockStore) RemoveFundingArbitrage(_ context.Context, symbol, exchangeA, exchangeB string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sortedKey(symbol, exchangeA, exchangeB)
	delete(m.arbitrages, key)
	return nil
}

func (m *mockStore) SetMarket(context.Context, domain.Market) error { panic("not implemented") }
func (m *mockStore) GetMarket(context.Context, string, string) (*domain.Market, error) {
	panic("not implemented")
}
func (m *mockStore) SetTicker(context.Context, domain.Ticker) error { panic("not implemented") }
func (m *mockStore) GetTicker(context.Context, string, string) (*domain.Ticker, error) {
	panic("not implemented")
}
func (m *mockStore) SetFundingRate(context.Context, domain.FundingRate) error {
	panic("not implemented")
}
func (m *mockStore) GetFundingRate(context.Context, string, string) (*domain.FundingRate, error) {
	panic("not implemented")
}
func (m *mockStore) SetSpreadArbitrage(context.Context, domain.SpreadArbitrage) error {
	panic("not implemented")
}
func (m *mockStore) GetSpreadArbitrage(context.Context, string, string, string) (*domain.SpreadArbitrage, error) {
	panic("not implemented")
}
func (m *mockStore) ListSpreadArbitrage(context.Context) ([]domain.SpreadArbitrage, error) {
	panic("not implemented")
}
func (m *mockStore) RemoveSpreadArbitrage(context.Context, string, string, string) error {
	panic("not implemented")
}
func (m *mockStore) FlushAll(context.Context) (int64, error) { panic("not implemented") }

// --- helpers ---

func d(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

func makeFundingRate(exchange, symbol string, rate string, intervalHours int) domain.FundingRate {
	return domain.FundingRate{
		ExchangeID:      exchange,
		Symbol:          symbol,
		Rate:            d(rate),
		IntervalHours:   intervalHours,
		NextFundingTime: time.Now().Add(time.Hour),
		MarkPrice:       d("50000"),
		IndexPrice:      d("50000"),
	}
}

func makeTicker(exchange, symbol string) domain.Ticker {
	return domain.Ticker{
		ExchangeID: exchange,
		Symbol:     symbol,
		Price:      d("50000"),
		Bid:        d("49999"),
		Ask:        d("50001"),
		Volume24h:  d("1000000"),
	}
}

func makeMarket(exchange, symbol string) domain.Market {
	return domain.Market{
		ExchangeID: exchange,
		Symbol:     symbol,
		MarketType: domain.MarketTypePerpetual,
		Active:     true,
		TakerFee:   d("0.00055"),
		MakerFee:   d("0.0002"),
	}
}

// --- tests ---

func TestRecompute_DirectionSwap_OverwritesSameKey(t *testing.T) {
	// Scenario: BTC/USDT:USDT on binance and bybit.
	// Round 1: binance has lower rate -> binance=long, bybit=short.
	// Round 2: rates flip -> bybit=long, binance=short.
	// After round 2, there must be exactly ONE arbitrage entry, not two.

	st := newMockStore()
	calc := NewCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "BTC/USDT:USDT"
	exchanges := []string{"binance", "bybit"}

	// Round 1: binance rate=0.0001 (lower), bybit rate=0.0005 (higher).
	// Expected: long=binance, short=bybit.
	st.mu.Lock()
	st.fundingRates["binance"] = []domain.FundingRate{makeFundingRate("binance", sym, "0.0001", 8)}
	st.fundingRates["bybit"] = []domain.FundingRate{makeFundingRate("bybit", sym, "0.0005", 8)}
	st.tickers["binance"] = []domain.Ticker{makeTicker("binance", sym)}
	st.tickers["bybit"] = []domain.Ticker{makeTicker("bybit", sym)}
	st.markets["binance"] = []domain.Market{makeMarket("binance", sym)}
	st.markets["bybit"] = []domain.Market{makeMarket("bybit", sym)}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("round 1 recompute: %v", err)
	}

	st.mu.Lock()
	if len(st.arbitrages) != 1 {
		t.Fatalf("round 1: expected 1 arbitrage, got %d", len(st.arbitrages))
	}
	for _, arb := range st.arbitrages {
		if arb.Long.ExchangeID != "binance" {
			t.Errorf("round 1: expected long=binance, got %s", arb.Long.ExchangeID)
		}
		if arb.Short.ExchangeID != "bybit" {
			t.Errorf("round 1: expected short=bybit, got %s", arb.Short.ExchangeID)
		}
	}
	st.mu.Unlock()

	// Round 2: flip the rates. bybit rate=0.0001 (lower), binance rate=0.0005 (higher).
	// Expected: long=bybit, short=binance.
	st.mu.Lock()
	st.fundingRates["binance"] = []domain.FundingRate{makeFundingRate("binance", sym, "0.0005", 8)}
	st.fundingRates["bybit"] = []domain.FundingRate{makeFundingRate("bybit", sym, "0.0001", 8)}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("round 2 recompute: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// Key assertion: exactly ONE arbitrage entry, not two.
	if len(st.arbitrages) != 1 {
		t.Fatalf("round 2: expected 1 arbitrage, got %d", len(st.arbitrages))
	}
	for _, arb := range st.arbitrages {
		if arb.Long.ExchangeID != "bybit" {
			t.Errorf("round 2: expected long=bybit, got %s", arb.Long.ExchangeID)
		}
		if arb.Short.ExchangeID != "binance" {
			t.Errorf("round 2: expected short=binance, got %s", arb.Short.ExchangeID)
		}
	}
}

func TestRecompute_ThreeExchanges_DirectionSwap(t *testing.T) {
	// Three exchanges: binance, bybit, okx.
	// This produces 3 pairs. When one pair flips direction, the other two
	// should be unaffected. Total should remain 3.

	st := newMockStore()
	calc := NewCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "ETH/USDT:USDT"
	exchanges := []string{"binance", "bybit", "okx"}

	// Round 1 rates: binance=0.0001, bybit=0.0003, okx=0.0005.
	st.mu.Lock()
	for _, ex := range exchanges {
		st.tickers[ex] = []domain.Ticker{makeTicker(ex, sym)}
		st.markets[ex] = []domain.Market{makeMarket(ex, sym)}
	}
	st.fundingRates["binance"] = []domain.FundingRate{makeFundingRate("binance", sym, "0.0001", 8)}
	st.fundingRates["bybit"] = []domain.FundingRate{makeFundingRate("bybit", sym, "0.0003", 8)}
	st.fundingRates["okx"] = []domain.FundingRate{makeFundingRate("okx", sym, "0.0005", 8)}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("round 1 recompute: %v", err)
	}

	st.mu.Lock()
	if len(st.arbitrages) != 3 {
		t.Fatalf("round 1: expected 3 arbitrages, got %d", len(st.arbitrages))
	}
	st.mu.Unlock()

	// Round 2: flip binance and bybit rates.
	// binance=0.0003, bybit=0.0001. okx stays at 0.0005.
	st.mu.Lock()
	st.fundingRates["binance"] = []domain.FundingRate{makeFundingRate("binance", sym, "0.0003", 8)}
	st.fundingRates["bybit"] = []domain.FundingRate{makeFundingRate("bybit", sym, "0.0001", 8)}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("round 2 recompute: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// Still exactly 3 arbitrages — no duplicates from direction change.
	if len(st.arbitrages) != 3 {
		t.Fatalf("round 2: expected 3 arbitrages, got %d", len(st.arbitrages))
	}

	// Verify the binance-bybit pair flipped direction.
	key := sortedKey(sym, "binance", "bybit")
	arb, ok := st.arbitrages[key]
	if !ok {
		t.Fatal("round 2: binance-bybit arbitrage not found")
	}
	if arb.Long.ExchangeID != "bybit" {
		t.Errorf("round 2: binance-bybit pair expected long=bybit, got %s", arb.Long.ExchangeID)
	}
	if arb.Short.ExchangeID != "binance" {
		t.Errorf("round 2: binance-bybit pair expected short=binance, got %s", arb.Short.ExchangeID)
	}
}

func TestComputePair_LongShortAssignment(t *testing.T) {
	// Verify that long is always assigned to the exchange with the lower 8h rate.
	calc := NewCalculator(nil, slog.Default())
	sym := "BTC/USDT:USDT"
	now := time.Now().UTC()

	dataA := &exchangeData{
		funding: map[string]domain.FundingRate{
			sym: makeFundingRate("binance", sym, "0.0005", 8),
		},
		tickers: map[string]domain.Ticker{
			sym: makeTicker("binance", sym),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("binance", sym),
		},
	}
	dataB := &exchangeData{
		funding: map[string]domain.FundingRate{
			sym: makeFundingRate("bybit", sym, "0.0001", 8),
		},
		tickers: map[string]domain.Ticker{
			sym: makeTicker("bybit", sym),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("bybit", sym),
		},
	}

	arb := calc.computePair(sym, "binance", "bybit", dataA, dataB, now)
	if arb == nil {
		t.Fatal("computePair returned nil")
	}

	// bybit has lower rate (0.0001) so it should be long.
	if arb.Long.ExchangeID != "bybit" {
		t.Errorf("expected long=bybit, got %s", arb.Long.ExchangeID)
	}
	if arb.Short.ExchangeID != "binance" {
		t.Errorf("expected short=binance, got %s", arb.Short.ExchangeID)
	}

	// NetRate8h should be positive (short_rate - long_rate).
	if !arb.NetRate8h.IsPositive() {
		t.Errorf("expected positive NetRate8h, got %s", arb.NetRate8h)
	}
}
