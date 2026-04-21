package positionmanager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
)

// ---------------------------------------------------------------------------
// Mock TradingExchange — configurable FetchPosition
// ---------------------------------------------------------------------------

// mockReconcileExchange extends mockTradingExchange with a configurable
// FetchPosition response. This lets tests control what the exchange reports
// when reconciliation queries it via REST.
type mockReconcileExchange struct {
	mockTradingExchange
	fetchPos    *base.ExchangePosition
	fetchPosErr error
}

func (m *mockReconcileExchange) FetchPosition(_ context.Context, _ string) (*base.ExchangePosition, error) {
	if m.fetchPosErr != nil {
		return nil, m.fetchPosErr
	}
	return m.fetchPos, nil
}

var _ base.PrivateExchange = (*mockReconcileExchange)(nil)

// ---------------------------------------------------------------------------
// Tests: reconcileOpenPositions — Phase 1
// ---------------------------------------------------------------------------

func TestReconcile_NoOpenPositions(t *testing.T) {
	ps := newMockPositionStore()
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, &mockTradingExchange{id: "okx"})

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReconcile_UnchangedState(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideLong,
			Contracts: dec("0.1"), // matches LongFilledQty
			MarkPrice: dec("51000"),
			Leverage:  dec("10"),
		},
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideShort,
			Contracts: dec("0.1"), // matches ShortFilledQty
			MarkPrice: dec("50200"),
			Leverage:  dec("5"),
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should remain open with updated live state.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open, got %s", updated.Status)
	}
	if !updated.LongMarkPrice.Equal(dec("51000")) {
		t.Errorf("expected LongMarkPrice 51000, got %s", updated.LongMarkPrice)
	}
	if !updated.ShortMarkPrice.Equal(dec("50200")) {
		t.Errorf("expected ShortMarkPrice 50200, got %s", updated.ShortMarkPrice)
	}

	// No close orders should have been placed.
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	okxMock.mu.Lock()
	okxOrders := len(okxMock.createdOrders)
	okxMock.mu.Unlock()

	if bybitOrders != 0 || okxOrders != 0 {
		t.Errorf("expected no orders, got bybit=%d okx=%d", bybitOrders, okxOrders)
	}
}

func TestReconcile_OneLegFullyClosed(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	// Bybit reports zero contracts — long leg was closed while we were down.
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos:            nil, // nil = no position on exchange
	}
	// OKX still has the short open.
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideShort,
			Contracts: dec("0.1"),
			MarkPrice: dec("50200"),
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reactor should detect long closure and auto-close the short.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonAutoClose {
		t.Errorf("expected close reason auto_close, got %s", updated.CloseReason)
	}

	// OKX should have received a close order.
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 close order on OKX, got %d", len(okxMock.createdOrders))
	}
	if okxMock.createdOrders[0].side != domain.OrderSideBuy {
		t.Errorf("expected buy order to close short, got %s", okxMock.createdOrders[0].side)
	}
}

func TestReconcile_OneLegPartiallyClosed(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.LongFilledQty = dec("1.0")
	p.ShortFilledQty = dec("1.0")
	ps.positions[p.ID] = p

	// Bybit reports reduced contracts — user manually closed some while we were down.
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideLong,
			Contracts: dec("0.6"), // reduced from 1.0 to 0.6
			MarkPrice: dec("51000"),
		},
	}
	// OKX unchanged.
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideShort,
			Contracts: dec("1.0"),
			MarkPrice: dec("50200"),
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reactor should detect partial close on long and rebalance short.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusPartiallyOpen {
		t.Errorf("expected status partially_open, got %s", updated.Status)
	}
	if !updated.LongFilledQty.Equal(dec("0.6")) {
		t.Errorf("expected LongFilledQty 0.6, got %s", updated.LongFilledQty)
	}
	// ShortFilledQty = 1.0 - 0.4 (delta) = 0.6
	if !updated.ShortFilledQty.Equal(dec("0.6")) {
		t.Errorf("expected ShortFilledQty 0.6, got %s", updated.ShortFilledQty)
	}

	// OKX should have received a rebalance order.
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 rebalance order on OKX, got %d", len(okxMock.createdOrders))
	}
}

func TestReconcile_BothLegsClosed(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	// Both exchanges report no position — both legs closed while we were down.
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos:            nil,
	}
	// Auto-close on the short will get ErrPositionAlreadyClosed since OKX
	// also reports zero. Use the already-closed mock for the auto-close.
	// Since the mockReconcileExchange inherits from mockTradingExchange,
	// CloseMarketOrder will try to close — but the position is already gone.
	// We need to handle this: use a mock that returns ErrPositionAlreadyClosed
	// on CloseMarketOrder.
	okxAlreadyClosed := &mockReconcileAlreadyClosedExchange{
		mockReconcileExchange: mockReconcileExchange{
			mockTradingExchange: mockTradingExchange{id: "okx"},
			fetchPos:            nil,
		},
	}

	tr := newTestTrader(ps, bybitMock, okxAlreadyClosed)

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should be closed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonAutoClose {
		t.Errorf("expected close reason auto_close, got %s", updated.CloseReason)
	}
}

func TestReconcile_FetchPositionError(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	// Bybit returns an error — leg is skipped.
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPosErr:         fmt.Errorf("connection timeout"),
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideShort,
			Contracts: dec("0.1"),
			MarkPrice: dec("50200"),
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should remain open — the failed leg was skipped.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status to remain open (error leg skipped), got %s", updated.Status)
	}
}

func TestReconcile_FetchPositionNil(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	// Bybit returns nil (no position = closed).
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos:            nil,
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideShort,
			Contracts: dec("0.1"),
			MarkPrice: dec("50200"),
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// nil is treated as zero contracts → triggers auto-close.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed (nil = zero contracts), got %s", updated.Status)
	}
}

func TestReconcile_ZeroContractsExplicit(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	// Bybit returns a position with zero contracts.
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideNone,
			Contracts: decimal.Zero,
		},
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideShort,
			Contracts: dec("0.1"),
			MarkPrice: dec("50200"),
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.reconcileOpenPositions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Zero contracts treated same as nil → auto-close.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: recoverStuckOpening — Phase 2
// ---------------------------------------------------------------------------

func TestRecoverOpening_BothLegsPresent(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-opening",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		CreatedAt:       time.Now().Add(-10 * time.Minute), // older than threshold
	}
	ps.positions[p.ID] = p

	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos: &base.ExchangePosition{
			Symbol:     "BTC/USDT:USDT",
			Side:       domain.PositionSideLong,
			Contracts:  dec("0.5"),
			EntryPrice: dec("50000"),
			MarkPrice:  dec("51000"),
			Leverage:   dec("10"),
			MarginMode: "cross",
		},
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:     "BTC/USDT:USDT",
			Side:       domain.PositionSideShort,
			Contracts:  dec("0.5"),
			EntryPrice: dec("50100"),
			MarkPrice:  dec("50200"),
			Leverage:   dec("5"),
			MarginMode: "isolated",
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckOpening(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open, got %s", updated.Status)
	}
	if updated.OpenedAt == nil {
		t.Error("expected OpenedAt to be set")
	}
	if !updated.LongFilledQty.Equal(dec("0.5")) {
		t.Errorf("expected LongFilledQty 0.5, got %s", updated.LongFilledQty)
	}
	if !updated.ShortFilledQty.Equal(dec("0.5")) {
		t.Errorf("expected ShortFilledQty 0.5, got %s", updated.ShortFilledQty)
	}
	if !updated.LongEntryPrice.Equal(dec("50000")) {
		t.Errorf("expected LongEntryPrice 50000, got %s", updated.LongEntryPrice)
	}
	if !updated.ShortEntryPrice.Equal(dec("50100")) {
		t.Errorf("expected ShortEntryPrice 50100, got %s", updated.ShortEntryPrice)
	}
	if updated.LongMarginMode != "cross" {
		t.Errorf("expected LongMarginMode cross, got %s", updated.LongMarginMode)
	}
	if updated.ShortMarginMode != "isolated" {
		t.Errorf("expected ShortMarginMode isolated, got %s", updated.ShortMarginMode)
	}
}

func TestRecoverOpening_OnlyLongLeg(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-opening-partial",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		CreatedAt:       time.Now().Add(-10 * time.Minute),
	}
	ps.positions[p.ID] = p

	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideLong,
			Contracts: dec("0.5"),
			MarkPrice: dec("51000"),
		},
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos:            nil, // short never filled
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckOpening(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// Long leg should have been rolled back.
	bybitMock.mu.Lock()
	defer bybitMock.mu.Unlock()
	if len(bybitMock.createdOrders) != 1 {
		t.Fatalf("expected 1 rollback order on bybit, got %d", len(bybitMock.createdOrders))
	}
	if bybitMock.createdOrders[0].side != domain.OrderSideSell {
		t.Errorf("expected sell order to close long, got %s", bybitMock.createdOrders[0].side)
	}
}

func TestRecoverOpening_OnlyShortLeg(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-opening-short-only",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		CreatedAt:       time.Now().Add(-10 * time.Minute),
	}
	ps.positions[p.ID] = p

	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos:            nil, // long never filled
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			Side:      domain.PositionSideShort,
			Contracts: dec("0.5"),
			MarkPrice: dec("50200"),
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckOpening(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// Short leg should have been rolled back.
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 rollback order on okx, got %d", len(okxMock.createdOrders))
	}
	if okxMock.createdOrders[0].side != domain.OrderSideBuy {
		t.Errorf("expected buy order to close short, got %s", okxMock.createdOrders[0].side)
	}
}

func TestRecoverOpening_NoLegs(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-opening-empty",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		CreatedAt:       time.Now().Add(-10 * time.Minute),
	}
	ps.positions[p.ID] = p

	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos:            nil,
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos:            nil,
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckOpening(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// No rollback orders needed.
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	okxMock.mu.Lock()
	okxOrders := len(okxMock.createdOrders)
	okxMock.mu.Unlock()

	if bybitOrders != 0 || okxOrders != 0 {
		t.Errorf("expected no orders, got bybit=%d okx=%d", bybitOrders, okxOrders)
	}
}

func TestRecoverOpening_RecentSkipped(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-opening-recent",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		CreatedAt:       time.Now().Add(-1 * time.Minute), // very recent
	}
	ps.positions[p.ID] = p

	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos:            nil,
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos:            nil,
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckOpening(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should remain in opening — too recent to recover.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpening {
		t.Errorf("expected status to remain opening (recent), got %s", updated.Status)
	}
}

func TestRecoverOpening_FetchError(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-opening-err",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		CreatedAt:       time.Now().Add(-10 * time.Minute),
	}
	ps.positions[p.ID] = p

	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPosErr:         fmt.Errorf("connection refused"),
	}
	okxMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		fetchPos:            nil,
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckOpening(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should remain in opening — transient error, don't fail it.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpening {
		t.Errorf("expected status to remain opening (fetch error), got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: recoverStuckClosing — Phase 3
// ---------------------------------------------------------------------------

func TestRecoverClosing_RetryClose(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.Status = domain.PositionStatusClosing
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckClosing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should be closed after retry.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonNormal {
		t.Errorf("expected close reason normal, got %s", updated.CloseReason)
	}
}

func TestRecoverClosing_AlreadyClosed(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.Status = domain.PositionStatusClosing
	ps.positions[p.ID] = p

	// Both exchanges reject reduce-only orders (already closed).
	bybitMock := &mockAlreadyClosedExchange{
		id:  "bybit",
		err: fmt.Errorf("bybit: %w", base.ErrPositionAlreadyClosed),
	}
	okxMock := &mockAlreadyClosedExchange{
		id:  "okx",
		err: fmt.Errorf("okx: %w", base.ErrPositionAlreadyClosed),
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.recoverStuckClosing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should be closed successfully.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
}

func TestRecoverClosing_NoStuckPositions(t *testing.T) {
	ps := newMockPositionStore()
	// No positions in closing state.
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, &mockTradingExchange{id: "okx"})

	err := tr.recoverStuckClosing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: fetchLegState
// ---------------------------------------------------------------------------

func TestFetchLegState_NilPosition(t *testing.T) {
	ps := newMockPositionStore()
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos:            nil,
	}
	tr := newTestTrader(ps, bybitMock)
	p := &domain.Position{UserID: "test-user-id", LongExchangeID: "bybit", LongCredentialID: "cred-long", ShortExchangeID: "okx", ShortCredentialID: "cred-short"}

	event := tr.fetchLegState(context.Background(), p, "long", "bybit", "BTC/USDT:USDT", tr.logger)
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if !event.Contracts.IsZero() {
		t.Errorf("expected zero contracts for nil position, got %s", event.Contracts)
	}
	if event.Side != domain.PositionSideNone {
		t.Errorf("expected side none, got %s", event.Side)
	}
}

func TestFetchLegState_ValidPosition(t *testing.T) {
	ps := newMockPositionStore()
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPos: &base.ExchangePosition{
			Symbol:           "BTC/USDT:USDT",
			Side:             domain.PositionSideLong,
			Contracts:        dec("1.5"),
			MarkPrice:        dec("51000"),
			LiquidationPrice: dec("40000"),
			UnrealizedPnL:    dec("150"),
			Leverage:         dec("10"),
			MarginMode:       "cross",
			Notional:         dec("76500"),
		},
	}
	tr := newTestTrader(ps, bybitMock)
	p := &domain.Position{UserID: "test-user-id", LongExchangeID: "bybit", LongCredentialID: "cred-long", ShortExchangeID: "okx", ShortCredentialID: "cred-short"}

	event := tr.fetchLegState(context.Background(), p, "long", "bybit", "BTC/USDT:USDT", tr.logger)
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if !event.Contracts.Equal(dec("1.5")) {
		t.Errorf("expected contracts 1.5, got %s", event.Contracts)
	}
	if event.Side != domain.PositionSideLong {
		t.Errorf("expected side long, got %s", event.Side)
	}
	if !event.MarkPrice.Equal(dec("51000")) {
		t.Errorf("expected mark price 51000, got %s", event.MarkPrice)
	}
	if !event.LiquidationPrice.Equal(dec("40000")) {
		t.Errorf("expected liquidation price 40000, got %s", event.LiquidationPrice)
	}
	if !event.Notional.Equal(dec("76500")) {
		t.Errorf("expected notional 76500, got %s", event.Notional)
	}
}

func TestFetchLegState_FetchError(t *testing.T) {
	ps := newMockPositionStore()
	bybitMock := &mockReconcileExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		fetchPosErr:         fmt.Errorf("rate limit exceeded"),
	}
	tr := newTestTrader(ps, bybitMock)
	p := &domain.Position{UserID: "test-user-id", LongExchangeID: "bybit", LongCredentialID: "cred-long", ShortExchangeID: "okx", ShortCredentialID: "cred-short"}

	event := tr.fetchLegState(context.Background(), p, "long", "bybit", "BTC/USDT:USDT", tr.logger)
	if event != nil {
		t.Errorf("expected nil event on fetch error, got %+v", event)
	}
}

func TestFetchLegState_UnknownExchange(t *testing.T) {
	ps := newMockPositionStore()
	tr := newTestTrader(ps) // no exchanges registered
	p := &domain.Position{UserID: "test-user-id", LongExchangeID: "unknown", LongCredentialID: "cred-long", ShortExchangeID: "okx", ShortCredentialID: "cred-short"}

	event := tr.fetchLegState(context.Background(), p, "long", "unknown", "BTC/USDT:USDT", tr.logger)
	if event != nil {
		t.Errorf("expected nil event for unknown exchange, got %+v", event)
	}
}

// ---------------------------------------------------------------------------
// Tests: full reconcileOnStartup integration
// ---------------------------------------------------------------------------

func TestReconcileOnStartup_MixedStates(t *testing.T) {
	ps := newMockPositionStore()

	// Open position with one leg closed.
	openPos := newOpenPosition()
	ps.positions[openPos.ID] = openPos

	// Stuck opening position.
	openingPos := &domain.Position{
		ID:              "pos-stuck-opening",
		Symbol:          "ETH/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		CreatedAt:       time.Now().Add(-10 * time.Minute),
	}
	ps.positions[openingPos.ID] = openingPos

	// Stuck closing position.
	closingPos := &domain.Position{
		ID:              "pos-stuck-closing",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusClosing,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		LongFilledQty:   dec("0.2"),
		ShortFilledQty:  dec("0.2"),
		LongEntryPrice:  dec("50000"),
		ShortEntryPrice: dec("50100"),
	}
	ps.positions[closingPos.ID] = closingPos

	// Bybit mock: BTC open at 0.1, ETH not present.
	// For BTC fetch: returns nil (closed on exchange).
	// For ETH fetch: returns nil (never opened).
	bybitMock := &mockReconcileMultiSymbol{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		positions:           map[string]*base.ExchangePosition{},
	}

	// OKX mock: BTC open at 0.1, ETH not present.
	okxMock := &mockReconcileMultiSymbol{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		positions: map[string]*base.ExchangePosition{
			"BTC/USDT:USDT": {
				Symbol:    "BTC/USDT:USDT",
				Side:      domain.PositionSideShort,
				Contracts: dec("0.1"),
				MarkPrice: dec("50200"),
			},
		},
	}

	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.reconcileOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Open position: bybit shows 0, should auto-close.
	open := ps.positions[openPos.ID]
	if open.Status != domain.PositionStatusClosed {
		t.Errorf("open position: expected closed, got %s", open.Status)
	}

	// Opening position: neither leg present, should be failed.
	opening := ps.positions[openingPos.ID]
	if opening.Status != domain.PositionStatusFailed {
		t.Errorf("opening position: expected failed, got %s", opening.Status)
	}

	// Closing position: re-executed close.
	closing := ps.positions[closingPos.ID]
	if closing.Status != domain.PositionStatusClosed {
		t.Errorf("closing position: expected closed, got %s", closing.Status)
	}
}

// ---------------------------------------------------------------------------
// Helper mocks for reconcile tests
// ---------------------------------------------------------------------------

// mockReconcileAlreadyClosedExchange wraps mockReconcileExchange but returns
// ErrPositionAlreadyClosed on CloseMarketOrder, simulating an exchange where
// the position was already closed.
type mockReconcileAlreadyClosedExchange struct {
	mockReconcileExchange
}

func (m *mockReconcileAlreadyClosedExchange) CloseMarketOrder(_ context.Context, _ string, _ domain.OrderSide, _ float64) (*domain.ExchangeOrder, error) {
	return nil, fmt.Errorf("reduce-only rejected: %w", base.ErrPositionAlreadyClosed)
}

var _ base.PrivateExchange = (*mockReconcileAlreadyClosedExchange)(nil)

// mockReconcileMultiSymbol supports per-symbol FetchPosition responses.
type mockReconcileMultiSymbol struct {
	mockTradingExchange
	positions map[string]*base.ExchangePosition
}

func (m *mockReconcileMultiSymbol) FetchPosition(_ context.Context, symbol string) (*base.ExchangePosition, error) {
	pos, ok := m.positions[symbol]
	if !ok {
		return nil, nil
	}
	return pos, nil
}

var _ base.PrivateExchange = (*mockReconcileMultiSymbol)(nil)
