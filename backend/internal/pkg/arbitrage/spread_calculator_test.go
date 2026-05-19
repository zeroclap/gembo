package arbitrage

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/zeroclap/gembo/internal/domain"
)

// spreadMockStore extends mockStore with working spread arbitrage methods.
type spreadMockStore struct {
	mockStore
	spreads map[string]domain.Arbitrage // keyed by "symbol|sortedExA|sortedExB"
}

func newSpreadMockStore() *spreadMockStore {
	return &spreadMockStore{
		mockStore: *newMockStore(),
		spreads:   make(map[string]domain.Arbitrage),
	}
}

func (m *spreadMockStore) SetArbitrage(_ context.Context, a domain.Arbitrage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sortedKey(a.Symbol, a.ExchangeA, a.ExchangeB)
	m.spreads[key] = a
	return nil
}

func (m *spreadMockStore) GetArbitrage(_ context.Context, _ domain.ArbitrageType, symbol, exchangeA, exchangeB string) (*domain.Arbitrage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sortedKey(symbol, exchangeA, exchangeB)
	a, ok := m.spreads[key]
	if !ok {
		return nil, nil
	}
	return &a, nil
}

func (m *spreadMockStore) ListArbitrage(_ context.Context) ([]domain.Arbitrage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Arbitrage, 0, len(m.spreads))
	for _, a := range m.spreads {
		out = append(out, a)
	}
	return out, nil
}

func (m *spreadMockStore) RemoveArbitrage(_ context.Context, _ domain.ArbitrageType, symbol, exchangeA, exchangeB string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sortedKey(symbol, exchangeA, exchangeB)
	delete(m.spreads, key)
	return nil
}

func makeTickerWithPrices(exchange, symbol, bid, ask, volume string) domain.Ticker {
	return domain.Ticker{
		ExchangeID: exchange,
		Symbol:     symbol,
		Price:      d(bid), // approximate
		Bid:        d(bid),
		Ask:        d(ask),
		Volume24h:  d(volume),
	}
}

func TestSpreadRecompute_BasicTwoExchanges(t *testing.T) {
	st := newSpreadMockStore()
	calc := NewSpreadCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "BTC/USDT:USDT"
	exchanges := []string{"binance", "bybit"}

	// binance: ask=50000, bid=49990
	// bybit:   ask=50100, bid=50090
	// Spread direction: buy on binance (ask=50000), sell on bybit (bid=50090)
	// spreadAB = bybit.bid - binance.ask = 50090 - 50000 = 90
	// spreadBA = binance.bid - bybit.ask = 49990 - 50100 = -110 (negative)
	// Best direction: buy binance, sell bybit.
	st.mu.Lock()
	st.tickers["binance"] = []domain.Ticker{makeTickerWithPrices("binance", sym, "49990", "50000", "1000000")}
	st.tickers["bybit"] = []domain.Ticker{makeTickerWithPrices("bybit", sym, "50090", "50100", "800000")}
	st.markets["binance"] = []domain.Market{makeMarket("binance", sym)}
	st.markets["bybit"] = []domain.Market{makeMarket("bybit", sym)}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.spreads) != 1 {
		t.Fatalf("expected 1 spread arbitrage, got %d", len(st.spreads))
	}

	for _, arb := range st.spreads {
		if arb.Spread.BuyExchangeID != "binance" {
			t.Errorf("expected buy=binance, got %s", arb.Spread.BuyExchangeID)
		}
		if arb.Spread.SellExchangeID != "bybit" {
			t.Errorf("expected sell=bybit, got %s", arb.Spread.SellExchangeID)
		}
		// buyAsk=50000, sellBid=50090, mid=50045, spread=90/50045*100 ≈ 0.1798%
		assertDecimalEqual(t, "BuyAsk", d("50000"), arb.Spread.BuyAsk)
		assertDecimalEqual(t, "SellBid", d("50090"), arb.Spread.SellBid)
		assertDecimalEqual(t, "SpreadAbs", d("90"), arb.Spread.SpreadAbs)
		if !arb.Spread.SpreadPct.IsPositive() {
			t.Errorf("expected positive SpreadPct, got %s", arb.Spread.SpreadPct)
		}
		if !arb.Spread.MidPrice.IsPositive() {
			t.Errorf("expected positive MidPrice, got %s", arb.Spread.MidPrice)
		}
		// Fees: taker=0.00055 per side. Open = 0.055% + 0.055% = 0.11%.
		// Round trip = 0.22%.
		assertDecimalClose(t, "RoundTripFeesPct", d("0.22"), arb.Spread.Fees.RoundTripFeesPct, d("0.001"))
		// Net spread = spread% - round_trip_fees%
		expectedNet := arb.Spread.SpreadPct.Sub(arb.Spread.Fees.RoundTripFeesPct)
		assertDecimalClose(t, "NetSpreadPct", expectedNet, arb.Spread.NetSpreadPct, d("0.0001"))
	}
}

func TestSpreadRecompute_DirectionSwap(t *testing.T) {
	// Round 1: binance cheaper -> buy binance, sell bybit.
	// Round 2: flip prices -> buy bybit, sell binance.
	// After round 2, should still be exactly 1 entry.
	st := newSpreadMockStore()
	calc := NewSpreadCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "ETH/USDT:USDT"
	exchanges := []string{"binance", "bybit"}

	// Round 1: binance ask=3000 < bybit ask=3100.
	st.mu.Lock()
	st.tickers["binance"] = []domain.Ticker{makeTickerWithPrices("binance", sym, "2990", "3000", "500000")}
	st.tickers["bybit"] = []domain.Ticker{makeTickerWithPrices("bybit", sym, "3090", "3100", "400000")}
	st.markets["binance"] = []domain.Market{makeMarket("binance", sym)}
	st.markets["bybit"] = []domain.Market{makeMarket("bybit", sym)}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("round 1: %v", err)
	}

	st.mu.Lock()
	if len(st.spreads) != 1 {
		t.Fatalf("round 1: expected 1 spread, got %d", len(st.spreads))
	}
	for _, arb := range st.spreads {
		if arb.Spread.BuyExchangeID != "binance" {
			t.Errorf("round 1: expected buy=binance, got %s", arb.Spread.BuyExchangeID)
		}
	}
	st.mu.Unlock()

	// Round 2: flip — bybit is now cheaper.
	st.mu.Lock()
	st.tickers["binance"] = []domain.Ticker{makeTickerWithPrices("binance", sym, "3190", "3200", "500000")}
	st.tickers["bybit"] = []domain.Ticker{makeTickerWithPrices("bybit", sym, "2990", "3000", "400000")}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("round 2: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.spreads) != 1 {
		t.Fatalf("round 2: expected 1 spread, got %d", len(st.spreads))
	}
	for _, arb := range st.spreads {
		if arb.Spread.BuyExchangeID != "bybit" {
			t.Errorf("round 2: expected buy=bybit, got %s", arb.Spread.BuyExchangeID)
		}
		if arb.Spread.SellExchangeID != "binance" {
			t.Errorf("round 2: expected sell=binance, got %s", arb.Spread.SellExchangeID)
		}
	}
}

func TestSpreadRecompute_ThreeExchanges(t *testing.T) {
	st := newSpreadMockStore()
	calc := NewSpreadCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "BTC/USDT:USDT"
	exchanges := []string{"binance", "bybit", "okx"}

	st.mu.Lock()
	st.tickers["binance"] = []domain.Ticker{makeTickerWithPrices("binance", sym, "49990", "50000", "1000000")}
	st.tickers["bybit"] = []domain.Ticker{makeTickerWithPrices("bybit", sym, "50090", "50100", "800000")}
	st.tickers["okx"] = []domain.Ticker{makeTickerWithPrices("okx", sym, "50200", "50210", "600000")}
	for _, ex := range exchanges {
		st.markets[ex] = []domain.Market{makeMarket(ex, sym)}
	}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// 3 choose 2 = 3 pairs.
	if len(st.spreads) != 3 {
		t.Fatalf("expected 3 spread pairs, got %d", len(st.spreads))
	}
}

func TestSpreadRecompute_SymbolOnSingleExchange_Skipped(t *testing.T) {
	st := newSpreadMockStore()
	calc := NewSpreadCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "SOL/USDT:USDT"
	exchanges := []string{"binance", "bybit"}

	// Only binance has this symbol.
	st.mu.Lock()
	st.tickers["binance"] = []domain.Ticker{makeTickerWithPrices("binance", sym, "100", "101", "100000")}
	st.markets["binance"] = []domain.Market{makeMarket("binance", sym)}
	st.tickers["bybit"] = []domain.Ticker{} // bybit has no tickers
	st.markets["bybit"] = []domain.Market{}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.spreads) != 0 {
		t.Fatalf("expected 0 spreads for single-exchange symbol, got %d", len(st.spreads))
	}
}

func TestSpreadRecompute_ZeroBidAsk_Skipped(t *testing.T) {
	st := newSpreadMockStore()
	calc := NewSpreadCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "BTC/USDT:USDT"
	exchanges := []string{"binance", "bybit"}

	// Binance has zero ask.
	st.mu.Lock()
	st.tickers["binance"] = []domain.Ticker{makeTickerWithPrices("binance", sym, "50000", "0", "1000000")}
	st.tickers["bybit"] = []domain.Ticker{makeTickerWithPrices("bybit", sym, "50090", "50100", "800000")}
	st.markets["binance"] = []domain.Market{makeMarket("binance", sym)}
	st.markets["bybit"] = []domain.Market{makeMarket("bybit", sym)}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.spreads) != 0 {
		t.Fatalf("expected 0 spreads with zero bid/ask, got %d", len(st.spreads))
	}
}

func TestSpreadRecompute_MissingMarketFees_Skipped(t *testing.T) {
	st := newSpreadMockStore()
	calc := NewSpreadCalculator(st, slog.Default())
	ctx := context.Background()
	sym := "BTC/USDT:USDT"
	exchanges := []string{"binance", "bybit"}

	st.mu.Lock()
	st.tickers["binance"] = []domain.Ticker{makeTickerWithPrices("binance", sym, "49990", "50000", "1000000")}
	st.tickers["bybit"] = []domain.Ticker{makeTickerWithPrices("bybit", sym, "50090", "50100", "800000")}
	st.markets["binance"] = []domain.Market{makeMarket("binance", sym)}
	// bybit has no markets -> fees unavailable.
	st.markets["bybit"] = []domain.Market{}
	st.mu.Unlock()

	if err := calc.Recompute(ctx, exchanges); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.spreads) != 0 {
		t.Fatalf("expected 0 spreads when market fees missing, got %d", len(st.spreads))
	}
}

func TestComputeSpreadPair_NegativeSpread(t *testing.T) {
	// When both directions produce negative spreads (ask > bid on the other exchange),
	// computeSpreadPair should still return a result — SpreadPct will be negative.
	calc := NewSpreadCalculator(nil, slog.Default())
	sym := "BTC/USDT:USDT"
	now := time.Now().UTC()

	// binance: ask=50100, bid=49900
	// bybit:   ask=50100, bid=49900
	// spreadAB = bybit.bid - binance.ask = 49900 - 50100 = -200
	// spreadBA = binance.bid - bybit.ask = 49900 - 50100 = -200
	// Both negative — the "better" one is AB (equal, >=).
	dataA := &exchangeData{
		tickers: map[string]domain.Ticker{
			sym: makeTickerWithPrices("binance", sym, "49900", "50100", "1000000"),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("binance", sym),
		},
	}
	dataB := &exchangeData{
		tickers: map[string]domain.Ticker{
			sym: makeTickerWithPrices("bybit", sym, "49900", "50100", "800000"),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("bybit", sym),
		},
	}

	arb := calc.computeSpreadPair(sym, "binance", "bybit", dataA, dataB, now)
	if arb == nil {
		t.Fatal("computeSpreadPair returned nil for negative spread")
	}

	// SpreadPct should be negative (no profitable spread).
	if arb.Spread.SpreadPct.IsPositive() {
		t.Errorf("expected non-positive SpreadPct, got %s", arb.Spread.SpreadPct)
	}
}

func TestComputeSpreadPair_FeeCalculation(t *testing.T) {
	calc := NewSpreadCalculator(nil, slog.Default())
	sym := "BTC/USDT:USDT"
	now := time.Now().UTC()

	// binance taker=0.00055, bybit taker=0.00055
	// Expected: buy_taker=0.055%, sell_taker=0.055%, open=0.11%, close=0.11%, round_trip=0.22%
	dataA := &exchangeData{
		tickers: map[string]domain.Ticker{
			sym: makeTickerWithPrices("binance", sym, "49990", "50000", "1000000"),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("binance", sym),
		},
	}
	dataB := &exchangeData{
		tickers: map[string]domain.Ticker{
			sym: makeTickerWithPrices("bybit", sym, "50090", "50100", "800000"),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("bybit", sym),
		},
	}

	arb := calc.computeSpreadPair(sym, "binance", "bybit", dataA, dataB, now)
	if arb == nil {
		t.Fatal("computeSpreadPair returned nil")
	}

	assertDecimalClose(t, "BuyTakerFeePct", d("0.055"), arb.Spread.Fees.BuyTakerFeePct, d("0.001"))
	assertDecimalClose(t, "SellTakerFeePct", d("0.055"), arb.Spread.Fees.SellTakerFeePct, d("0.001"))
	assertDecimalClose(t, "OpenFeesPct", d("0.11"), arb.Spread.Fees.OpenFeesPct, d("0.001"))
	assertDecimalClose(t, "CloseFeesPct", d("0.11"), arb.Spread.Fees.CloseFeesPct, d("0.001"))
	assertDecimalClose(t, "RoundTripFeesPct", d("0.22"), arb.Spread.Fees.RoundTripFeesPct, d("0.001"))
}

func TestComputeSpreadPair_Volume(t *testing.T) {
	calc := NewSpreadCalculator(nil, slog.Default())
	sym := "ETH/USDT:USDT"
	now := time.Now().UTC()

	dataA := &exchangeData{
		tickers: map[string]domain.Ticker{
			sym: makeTickerWithPrices("binance", sym, "2990", "3000", "500000"),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("binance", sym),
		},
	}
	dataB := &exchangeData{
		tickers: map[string]domain.Ticker{
			sym: makeTickerWithPrices("bybit", sym, "3090", "3100", "400000"),
		},
		markets: map[string]domain.Market{
			sym: makeMarket("bybit", sym),
		},
	}

	arb := calc.computeSpreadPair(sym, "binance", "bybit", dataA, dataB, now)
	if arb == nil {
		t.Fatal("computeSpreadPair returned nil")
	}

	// Buy side = binance (lower ask), sell side = bybit (higher bid).
	assertDecimalEqual(t, "BuyVolume24h", d("500000"), arb.Spread.BuyVolume24h)
	assertDecimalEqual(t, "SellVolume24h", d("400000"), arb.Spread.SellVolume24h)
}
