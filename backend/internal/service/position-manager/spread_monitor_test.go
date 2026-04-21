package positionmanager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/store"
)

// ---------------------------------------------------------------------------
// Mock market store (store.Store) for spread monitor tests.
// Only GetTicker is used by evaluateSpreadPosition; other methods are stubs.
// ---------------------------------------------------------------------------

type mockMarketStore struct {
	mu      sync.Mutex
	tickers map[string]*domain.Ticker // keyed by "exchangeID:symbol"
}

func newMockMarketStore() *mockMarketStore {
	return &mockMarketStore{
		tickers: make(map[string]*domain.Ticker),
	}
}

func (m *mockMarketStore) setTicker(exchangeID, symbol string, bid, ask decimal.Decimal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := exchangeID + ":" + symbol
	m.tickers[key] = &domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     symbol,
		Bid:        bid,
		Ask:        ask,
	}
}

func (m *mockMarketStore) GetTicker(_ context.Context, exchangeID, symbol string) (*domain.Ticker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := exchangeID + ":" + symbol
	t, ok := m.tickers[key]
	if !ok {
		return nil, nil
	}
	return t, nil
}

// Stub implementations for the rest of store.Store interface.
func (m *mockMarketStore) SetMarket(context.Context, domain.Market) error { return nil }
func (m *mockMarketStore) GetMarket(context.Context, string, string) (*domain.Market, error) {
	return nil, nil
}
func (m *mockMarketStore) ListMarkets(context.Context, string) ([]domain.Market, error) {
	return nil, nil
}
func (m *mockMarketStore) SetTicker(context.Context, domain.Ticker) error { return nil }
func (m *mockMarketStore) ListTickers(context.Context, string) ([]domain.Ticker, error) {
	return nil, nil
}
func (m *mockMarketStore) SetFundingRate(context.Context, domain.FundingRate) error { return nil }
func (m *mockMarketStore) GetFundingRate(context.Context, string, string) (*domain.FundingRate, error) {
	return nil, nil
}
func (m *mockMarketStore) ListFundingRates(context.Context, string) ([]domain.FundingRate, error) {
	return nil, nil
}
func (m *mockMarketStore) SetBalance(context.Context, domain.Balance) error { return nil }
func (m *mockMarketStore) ListBalances(context.Context, string, string) ([]domain.Balance, error) {
	return nil, nil
}
func (m *mockMarketStore) SetFundingArbitrage(context.Context, domain.FundingArbitrage) error {
	return nil
}
func (m *mockMarketStore) GetFundingArbitrage(context.Context, string, string, string) (*domain.FundingArbitrage, error) {
	return nil, nil
}
func (m *mockMarketStore) ListFundingArbitrage(context.Context) ([]domain.FundingArbitrage, error) {
	return nil, nil
}
func (m *mockMarketStore) RemoveFundingArbitrage(context.Context, string, string, string) error {
	return nil
}
func (m *mockMarketStore) SetSpreadArbitrage(context.Context, domain.SpreadArbitrage) error {
	return nil
}
func (m *mockMarketStore) GetSpreadArbitrage(context.Context, string, string, string) (*domain.SpreadArbitrage, error) {
	return nil, nil
}
func (m *mockMarketStore) ListSpreadArbitrage(context.Context) ([]domain.SpreadArbitrage, error) {
	return nil, nil
}
func (m *mockMarketStore) RemoveSpreadArbitrage(context.Context, string, string, string) error {
	return nil
}
func (m *mockMarketStore) FlushAll(context.Context) (int64, error) { return 0, nil }

var _ store.Store = (*mockMarketStore)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newSpreadTestTrader(ps *mockPositionStore, ms store.Store) (*Trader, *SpreadMonitor) {
	tr := newTestTrader(ps, &mockTradingExchange{id: "binance"}, &mockTradingExchange{id: "bybit"})
	tr.marketStore = ms
	sm := newSpreadMonitor(tr, ms)
	tr.spreadMonitor = sm
	return tr, sm
}

func newSpreadPosition(id string, opts ...func(*domain.Position)) *domain.Position {
	now := time.Now().UTC()
	p := &domain.Position{
		ID:                id,
		UserID:            "test-user-id",
		Symbol:            "BTC/USDT:USDT",
		Status:            domain.PositionStatusOpen,
		ArbitrageType:     domain.ArbitrageTypeSpread,
		LongExchangeID:    "binance",
		ShortExchangeID:   "bybit",
		LongFilledQty:     dec("0.1"),
		ShortFilledQty:    dec("0.1"),
		EntrySpreadPct:    dec("2.0"),
		TargetSpreadPct:   dec("0.5"),
		StopLossSpreadPct: dec("3.5"),
		MaxHoldDuration:   3600, // 1 hour
		OpenedAt:          &now,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// ---------------------------------------------------------------------------
// Tests: evaluateSpreadPosition
// ---------------------------------------------------------------------------

func TestSpreadMonitor_TargetReached(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	p := newSpreadPosition("pos-spread-1")
	ps.positions[p.ID] = p

	// Set up tickers so current spread <= target (0.5%).
	// long (binance) ask=50000, short (bybit) bid=50100
	// spread = (50100 - 50000) / ((50000+50100)/2) * 100 ≈ 0.1998%
	ms.setTicker("binance", "BTC/USDT:USDT", dec("49990"), dec("50000"))
	ms.setTicker("bybit", "BTC/USDT:USDT", dec("50100"), dec("50110"))

	sm.evaluateSpreadPosition(context.Background(), p)

	// Position should be transitioned to closing with target_reached reason.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosing {
		t.Errorf("expected status closing, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonTargetReached {
		t.Errorf("expected close reason target_reached, got %s", updated.CloseReason)
	}
}

func TestSpreadMonitor_StopLossHit(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	p := newSpreadPosition("pos-spread-2")
	ps.positions[p.ID] = p

	// Set up tickers so current spread >= stop-loss (3.5%).
	// long (binance) ask=50000, short (bybit) bid=51800
	// spread = (51800 - 50000) / ((50000+51800)/2) * 100 ≈ 3.54%
	ms.setTicker("binance", "BTC/USDT:USDT", dec("49990"), dec("50000"))
	ms.setTicker("bybit", "BTC/USDT:USDT", dec("51800"), dec("51810"))

	sm.evaluateSpreadPosition(context.Background(), p)

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosing {
		t.Errorf("expected status closing, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonStopLoss {
		t.Errorf("expected close reason stop_loss, got %s", updated.CloseReason)
	}
}

func TestSpreadMonitor_NoTrigger(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	p := newSpreadPosition("pos-spread-3")
	ps.positions[p.ID] = p

	// Spread is between target and stop-loss: ~2.0%
	// long (binance) ask=50000, short (bybit) bid=51000
	// spread = (51000 - 50000) / 50500 * 100 ≈ 1.98%
	ms.setTicker("binance", "BTC/USDT:USDT", dec("49990"), dec("50000"))
	ms.setTicker("bybit", "BTC/USDT:USDT", dec("51000"), dec("51010"))

	sm.evaluateSpreadPosition(context.Background(), p)

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	// Position should remain open.
	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open, got %s", updated.Status)
	}
}

func TestSpreadMonitor_MissingTickers_NoAction(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	p := newSpreadPosition("pos-spread-4")
	ps.positions[p.ID] = p

	// No tickers set up — evaluateSpreadPosition should return early.
	sm.evaluateSpreadPosition(context.Background(), p)

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open when tickers missing, got %s", updated.Status)
	}
}

func TestSpreadMonitor_ZeroTargetAndStopLoss(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Position with zero target and zero stop-loss → no auto-close triggers.
	p := newSpreadPosition("pos-spread-5", func(p *domain.Position) {
		p.TargetSpreadPct = decimal.Zero
		p.StopLossSpreadPct = decimal.Zero
	})
	ps.positions[p.ID] = p

	// Set any spread.
	ms.setTicker("binance", "BTC/USDT:USDT", dec("49990"), dec("50000"))
	ms.setTicker("bybit", "BTC/USDT:USDT", dec("51000"), dec("51010"))

	sm.evaluateSpreadPosition(context.Background(), p)

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open with zero thresholds, got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: checkTimeouts
// ---------------------------------------------------------------------------

func TestSpreadMonitor_Timeout(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Opened 2 hours ago with max hold 1 hour → expired.
	openedAt := time.Now().UTC().Add(-2 * time.Hour)
	p := newSpreadPosition("pos-timeout-1", func(p *domain.Position) {
		p.OpenedAt = &openedAt
		p.MaxHoldDuration = 3600 // 1 hour
	})
	ps.positions[p.ID] = p

	sm.checkTimeouts(context.Background(), "")

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosing {
		t.Errorf("expected status closing, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonTimeout {
		t.Errorf("expected close reason timeout, got %s", updated.CloseReason)
	}
}

func TestSpreadMonitor_NoTimeout_WithinDuration(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Opened 10 minutes ago with max hold 1 hour → not expired.
	openedAt := time.Now().UTC().Add(-10 * time.Minute)
	p := newSpreadPosition("pos-timeout-2", func(p *domain.Position) {
		p.OpenedAt = &openedAt
		p.MaxHoldDuration = 3600
	})
	ps.positions[p.ID] = p

	sm.checkTimeouts(context.Background(), "")

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open, got %s", updated.Status)
	}
}

func TestSpreadMonitor_NoTimeout_ZeroMaxHold(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// MaxHoldDuration=0 means no timeout.
	openedAt := time.Now().UTC().Add(-100 * time.Hour)
	p := newSpreadPosition("pos-timeout-3", func(p *domain.Position) {
		p.OpenedAt = &openedAt
		p.MaxHoldDuration = 0
	})
	ps.positions[p.ID] = p

	sm.checkTimeouts(context.Background(), "")

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open with zero max hold, got %s", updated.Status)
	}
}

func TestSpreadMonitor_Timeout_IgnoresFundingPositions(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Funding position that has been open past its "max hold" should not be closed.
	openedAt := time.Now().UTC().Add(-2 * time.Hour)
	p := &domain.Position{
		ID:              "pos-funding-1",
		UserID:          "test-user-id",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpen,
		ArbitrageType:   domain.ArbitrageTypeFunding,
		LongExchangeID:  "binance",
		ShortExchangeID: "bybit",
		MaxHoldDuration: 3600,
		OpenedAt:        &openedAt,
	}
	ps.positions[p.ID] = p

	sm.checkTimeouts(context.Background(), "")

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("funding position should not be closed by spread timeout, got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: checkSpreadPositions (integration of evaluate + filter)
// ---------------------------------------------------------------------------

func TestSpreadMonitor_CheckSpreadPositions_FiltersNonSpread(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// One funding position, one spread position with target-reached condition.
	fundingPos := &domain.Position{
		ID:              "pos-funding",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpen,
		ArbitrageType:   domain.ArbitrageTypeFunding,
		LongExchangeID:  "binance",
		ShortExchangeID: "bybit",
	}
	ps.positions[fundingPos.ID] = fundingPos

	spreadPos := newSpreadPosition("pos-spread-check")
	ps.positions[spreadPos.ID] = spreadPos

	// Tickers for target convergence.
	ms.setTicker("binance", "BTC/USDT:USDT", dec("49990"), dec("50000"))
	ms.setTicker("bybit", "BTC/USDT:USDT", dec("50100"), dec("50110"))

	sm.checkSpreadPositions(context.Background(), "")

	ps.mu.Lock()
	fundingUpdated := ps.positions[fundingPos.ID]
	spreadUpdated := ps.positions[spreadPos.ID]
	ps.mu.Unlock()

	// Funding position should be untouched.
	if fundingUpdated.Status != domain.PositionStatusOpen {
		t.Errorf("funding position should remain open, got %s", fundingUpdated.Status)
	}

	// Spread position should be closing (target reached).
	if spreadUpdated.Status != domain.PositionStatusClosing {
		t.Errorf("spread position should be closing, got %s", spreadUpdated.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: reconcileSpreadPositions
// ---------------------------------------------------------------------------

func TestSpreadMonitor_Reconcile_TimeoutOnStartup(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Position that timed out while trader was offline.
	openedAt := time.Now().UTC().Add(-3 * time.Hour)
	p := newSpreadPosition("pos-reconcile-1", func(p *domain.Position) {
		p.OpenedAt = &openedAt
		p.MaxHoldDuration = 3600 // 1 hour
	})
	ps.positions[p.ID] = p

	err := sm.reconcileSpreadPositions(context.Background())
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosing {
		t.Errorf("expected closing after reconcile timeout, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonTimeout {
		t.Errorf("expected timeout reason, got %s", updated.CloseReason)
	}
}

func TestSpreadMonitor_Reconcile_TargetReached(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Position opened recently (not timed out) but spread converged.
	openedAt := time.Now().UTC().Add(-5 * time.Minute)
	p := newSpreadPosition("pos-reconcile-2", func(p *domain.Position) {
		p.OpenedAt = &openedAt
	})
	ps.positions[p.ID] = p

	// Tickers for target convergence.
	ms.setTicker("binance", "BTC/USDT:USDT", dec("49990"), dec("50000"))
	ms.setTicker("bybit", "BTC/USDT:USDT", dec("50100"), dec("50110"))

	err := sm.reconcileSpreadPositions(context.Background())
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosing {
		t.Errorf("expected closing after reconcile target reached, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonTargetReached {
		t.Errorf("expected target_reached reason, got %s", updated.CloseReason)
	}
}

func TestSpreadMonitor_Reconcile_NoAction(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Position opened recently, spread is within range.
	openedAt := time.Now().UTC().Add(-5 * time.Minute)
	p := newSpreadPosition("pos-reconcile-3", func(p *domain.Position) {
		p.OpenedAt = &openedAt
	})
	ps.positions[p.ID] = p

	// Spread between target and stop-loss.
	ms.setTicker("binance", "BTC/USDT:USDT", dec("49990"), dec("50000"))
	ms.setTicker("bybit", "BTC/USDT:USDT", dec("51000"), dec("51010"))

	err := sm.reconcileSpreadPositions(context.Background())
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected open after reconcile with no trigger, got %s", updated.Status)
	}
}

func TestSpreadMonitor_Reconcile_IgnoresClosedPositions(t *testing.T) {
	ps := newMockPositionStore()
	ms := newMockMarketStore()
	_, sm := newSpreadTestTrader(ps, ms)

	// Already closed position should not be touched.
	p := newSpreadPosition("pos-reconcile-4", func(p *domain.Position) {
		p.Status = domain.PositionStatusClosed
	})
	ps.positions[p.ID] = p

	err := sm.reconcileSpreadPositions(context.Background())
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("closed position should remain closed, got %s", updated.Status)
	}
}
