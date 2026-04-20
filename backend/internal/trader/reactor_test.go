package trader

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/store"
)

// ---------------------------------------------------------------------------
// Mock PositionStore
// ---------------------------------------------------------------------------

type mockPositionStore struct {
	mu        sync.Mutex
	positions map[string]*domain.Position
	orders    []domain.ExchangeOrder
	funding   map[string][]domain.FundingPayment
}

func newMockPositionStore() *mockPositionStore {
	return &mockPositionStore{
		positions: make(map[string]*domain.Position),
		orders:    nil,
		funding:   make(map[string][]domain.FundingPayment),
	}
}

func (m *mockPositionStore) CreatePosition(_ context.Context, p *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.UserID == "" {
		p.UserID = "test-user-id"
	}
	m.positions[p.ID] = p
	return nil
}

func (m *mockPositionStore) GetPosition(_ context.Context, userID, id string) (*domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.positions[id]
	if !ok {
		return nil, nil
	}
	if p.UserID != "" && p.UserID != userID {
		return nil, nil
	}
	return p, nil
}

func (m *mockPositionStore) ListPositions(_ context.Context, f domain.PositionFilter) ([]domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Position
	for _, p := range m.positions {
		if f.UserID != "" && p.UserID != f.UserID {
			continue
		}
		if f.Symbol != "" && p.Symbol != f.Symbol {
			continue
		}
		if len(f.Status) > 0 {
			match := false
			for _, s := range f.Status {
				if p.Status == s {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, *p)
	}
	return out, nil
}

func (m *mockPositionStore) UpdatePosition(_ context.Context, p *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.positions[p.ID] = p
	return nil
}

func (m *mockPositionStore) DeletePosition(_ context.Context, userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.positions[id]
	if ok && p.UserID != "" && p.UserID != userID {
		return nil
	}
	delete(m.positions, id)
	return nil
}

func (m *mockPositionStore) CreateOrder(_ context.Context, o *domain.ExchangeOrder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o.UserID == "" {
		o.UserID = "test-user-id"
	}
	m.orders = append(m.orders, *o)
	return nil
}

func (m *mockPositionStore) UpdateOrder(_ context.Context, _ *domain.ExchangeOrder) error {
	return nil
}

func (m *mockPositionStore) ListOrdersByPosition(_ context.Context, _, _ string) ([]domain.ExchangeOrder, error) {
	return nil, nil
}

func (m *mockPositionStore) CreateFundingPayment(_ context.Context, fp *domain.FundingPayment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if fp.UserID == "" {
		fp.UserID = "test-user-id"
	}
	m.funding[fp.PositionID] = append(m.funding[fp.PositionID], *fp)
	return nil
}

func (m *mockPositionStore) CreateFundingPaymentIfNotExists(_ context.Context, fp *domain.FundingPayment) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.funding[fp.PositionID] {
		if fp.ExternalID != "" &&
			existing.ExternalID == fp.ExternalID &&
			existing.UserID == fp.UserID &&
			existing.ExchangeID == fp.ExchangeID {
			return false, nil
		}
	}

	if fp.UserID == "" {
		fp.UserID = "test-user-id"
	}
	m.funding[fp.PositionID] = append(m.funding[fp.PositionID], *fp)
	return true, nil
}

func (m *mockPositionStore) ListFundingPayments(_ context.Context, _, positionID string) ([]domain.FundingPayment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.funding[positionID], nil
}

func (m *mockPositionStore) SumFundingPnL(_ context.Context, userID, positionID string) (decimal.Decimal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sum := decimal.Zero
	for _, fp := range m.funding[positionID] {
		if userID != "" && fp.UserID != userID {
			continue
		}
		sum = sum.Add(fp.Payment)
	}
	return sum, nil
}

func (m *mockPositionStore) LatestFundingPayment(_ context.Context, _ string, positionID, exchangeID string) (*domain.FundingPayment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var latest *domain.FundingPayment
	for i := range m.funding[positionID] {
		fp := m.funding[positionID][i]
		if exchangeID != "" && fp.ExchangeID != exchangeID {
			continue
		}
		if latest == nil || fp.FundedAt.After(latest.FundedAt) {
			copy := fp
			latest = &copy
		}
	}

	return latest, nil
}

var _ store.PositionStore = (*mockPositionStore)(nil)

// ---------------------------------------------------------------------------
// Mock UserStore
// ---------------------------------------------------------------------------

type mockUserStore struct{}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{}
}

func (m *mockUserStore) GetUserByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserStore) GetUserByID(_ context.Context, _ string) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserStore) CreateUser(_ context.Context, _ *domain.User) error {
	return nil
}

func (m *mockUserStore) ListExchangeCredentialsByUserID(_ context.Context, _ string) ([]domain.ExchangeCredential, error) {
	return nil, nil
}

func (m *mockUserStore) ListExchangeCredentials(_ context.Context) ([]domain.ExchangeCredential, error) {
	return nil, nil
}

func (m *mockUserStore) GetExchangeCredentialByID(_ context.Context, _, _ string) (*domain.ExchangeCredential, error) {
	return nil, nil
}

func (m *mockUserStore) CreateExchangeCredential(_ context.Context, _ *domain.ExchangeCredential) error {
	return nil
}

func (m *mockUserStore) UpdateExchangeCredential(_ context.Context, _ *domain.ExchangeCredential) (bool, error) {
	return false, nil
}

func (m *mockUserStore) DeleteExchangeCredential(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

var _ store.UserStore = (*mockUserStore)(nil)

// ---------------------------------------------------------------------------
// Mock TradingExchange
// ---------------------------------------------------------------------------

type mockTradingExchange struct {
	id              string
	createdOrders   []mockOrderCall
	fetchedPosition *exchange.ExchangePosition
	fundingHistory  []exchange.FundingHistoryEntry
	lastOrderAmount float64 // track last order amount for FetchOrder
	mu              sync.Mutex
}

type mockOrderCall struct {
	symbol string
	side   domain.OrderSide
	amount float64
}

func (m *mockTradingExchange) ID() string { return m.id }
func (m *mockTradingExchange) Run(_ context.Context) error {
	return nil
}
func (m *mockTradingExchange) Markets(_ context.Context) ([]domain.Market, error) {
	return nil, nil
}
func (m *mockTradingExchange) Tickers(_ context.Context, _ []string) (<-chan domain.Ticker, error) {
	return nil, nil
}
func (m *mockTradingExchange) FundingRates(_ context.Context) (<-chan domain.FundingRate, error) {
	return nil, nil
}
func (m *mockTradingExchange) CreateMarketOrder(_ context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdOrders = append(m.createdOrders, mockOrderCall{symbol: symbol, side: side, amount: amount})
	m.lastOrderAmount = amount
	return &domain.ExchangeOrder{
		ExchangeOrderID: "mock-order-123",
		Status:          "closed", // CCXT uses "closed" for filled
		FilledQty:       decimal.NewFromFloat(amount),
		AveragePrice:    decimal.NewFromFloat(50000),
		Cost:            decimal.NewFromFloat(amount * 50000),
		FeeCost:         decimal.NewFromFloat(0.5),
	}, nil
}
func (m *mockTradingExchange) CloseMarketOrder(_ context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdOrders = append(m.createdOrders, mockOrderCall{symbol: symbol, side: side, amount: amount})
	m.lastOrderAmount = amount
	return &domain.ExchangeOrder{
		ExchangeOrderID: "mock-order-123",
		Status:          "closed",
		FilledQty:       decimal.NewFromFloat(amount),
		AveragePrice:    decimal.NewFromFloat(50000),
		Cost:            decimal.NewFromFloat(amount * 50000),
		FeeCost:         decimal.NewFromFloat(0.5),
	}, nil
}
func (m *mockTradingExchange) FetchOrder(_ context.Context, _, _ string) (*domain.ExchangeOrder, error) {
	m.mu.Lock()
	amount := m.lastOrderAmount
	m.mu.Unlock()
	return &domain.ExchangeOrder{
		ExchangeOrderID: "mock-order-123",
		Status:          "closed",
		FilledQty:       decimal.NewFromFloat(amount),
		AveragePrice:    decimal.NewFromFloat(50000),
		Cost:            decimal.NewFromFloat(amount * 50000),
		FeeCost:         decimal.NewFromFloat(0.5),
	}, nil
}
func (m *mockTradingExchange) FetchPosition(_ context.Context, _ string) (*exchange.ExchangePosition, error) {
	return m.fetchedPosition, nil
}
func (m *mockTradingExchange) FetchBalance(_ context.Context) ([]exchange.Balance, error) {
	return nil, nil
}

func (m *mockTradingExchange) FetchFundingHistory(_ context.Context, symbol string, since time.Time, limit int64) ([]exchange.FundingHistoryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]exchange.FundingHistoryEntry, 0, len(m.fundingHistory))
	for _, row := range m.fundingHistory {
		if row.Symbol != "" && row.Symbol != symbol {
			continue
		}
		if row.Timestamp.Before(since) {
			continue
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	if limit > 0 && int64(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *mockTradingExchange) SetMarginMode(_ context.Context, _, _ string, _ int) error {
	return nil
}
func (m *mockTradingExchange) HasCredentials() bool { return true }

var _ exchange.TradingExchange = (*mockTradingExchange)(nil)
var _ exchange.FundingHistoryFetcher = (*mockTradingExchange)(nil)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestTrader(ps store.PositionStore, exchanges ...exchange.TradingExchange) *Trader {
	reg := exchange.NewTradingRegistry()
	for _, e := range exchanges {
		reg.Register(e)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rc := goredis.NewClient(&goredis.Options{Addr: "localhost:0"}) // stub, won't connect
	return &Trader{
		registry:  reg,
		store:     ps,
		userStore: newMockUserStore(),
		redis:     rc,
		logger:    logger,
		cfg:       config.TraderConfig{},
		clients:   make(map[string]exchange.TradingExchange),
	}
}

func dec(v string) decimal.Decimal {
	d, _ := decimal.NewFromString(v)
	return d
}

func newOpenPosition() *domain.Position {
	return &domain.Position{
		ID:                "pos-1",
		UserID:            "test-user-id",
		Symbol:            "BTC/USDT:USDT",
		Status:            domain.PositionStatusOpen,
		LongExchangeID:    "bybit",
		LongCredentialID:  "cred-long",
		ShortExchangeID:   "okx",
		ShortCredentialID: "cred-short",
		LongFilledQty:     dec("0.1"),
		ShortFilledQty:    dec("0.1"),
		LongEntryPrice:    dec("50000"),
		ShortEntryPrice:   dec("50100"),
	}
}

// ---------------------------------------------------------------------------
// Tests: matchLeg
// ---------------------------------------------------------------------------

func TestMatchLeg_LongMatch(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideLong,
	}

	leg := tr.matchLeg(p, event)
	if leg != "long" {
		t.Errorf("expected long, got %q", leg)
	}
}

func TestMatchLeg_ShortMatch(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		ExchangeID: "okx",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideShort,
	}

	leg := tr.matchLeg(p, event)
	if leg != "short" {
		t.Errorf("expected short, got %q", leg)
	}
}

func TestMatchLeg_ExchangeIDDeterminesLeg(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	// Exchange matches long — even if side says "short", the exchange ID
	// determines the leg in the arbitrage system (each exchange holds one side).
	event := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideShort,
	}

	leg := tr.matchLeg(p, event)
	if leg != "long" {
		t.Errorf("expected long (exchange ID determines leg), got %q", leg)
	}
}

func TestMatchLeg_UnknownExchange(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		ExchangeID: "binance",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideLong,
	}

	leg := tr.matchLeg(p, event)
	if leg != "" {
		t.Errorf("expected empty, got %q", leg)
	}
}

// ---------------------------------------------------------------------------
// Tests: applyPositionEvent
// ---------------------------------------------------------------------------

func TestApplyPositionEvent_Long(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		MarkPrice:        dec("51000"),
		UnrealizedPnL:    dec("100"),
		LiquidationPrice: dec("40000"),
		Leverage:         dec("10"),
		MarginMode:       "cross",
		Notional:         dec("5100"),
	}

	tr.applyPositionEvent(p, event, "long")

	if !p.LongMarkPrice.Equal(dec("51000")) {
		t.Errorf("expected LongMarkPrice 51000, got %s", p.LongMarkPrice)
	}
	if !p.LongUnrealizedPnL.Equal(dec("100")) {
		t.Errorf("expected LongUnrealizedPnL 100, got %s", p.LongUnrealizedPnL)
	}
	if !p.LongLiquidationPrice.Equal(dec("40000")) {
		t.Errorf("expected LongLiquidationPrice 40000, got %s", p.LongLiquidationPrice)
	}
	if !p.LongLeverage.Equal(dec("10")) {
		t.Errorf("expected LongLeverage 10, got %s", p.LongLeverage)
	}
	if p.LongMarginMode != "cross" {
		t.Errorf("expected LongMarginMode cross, got %s", p.LongMarginMode)
	}
	if !p.LongNotional.Equal(dec("5100")) {
		t.Errorf("expected LongNotional 5100, got %s", p.LongNotional)
	}
	// Short fields should remain untouched.
	if !p.ShortMarkPrice.IsZero() {
		t.Errorf("expected ShortMarkPrice zero, got %s", p.ShortMarkPrice)
	}
}

func TestApplyPositionEvent_Short(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		MarkPrice:        dec("50200"),
		UnrealizedPnL:    dec("-50"),
		LiquidationPrice: dec("65000"),
		Leverage:         dec("5"),
		MarginMode:       "isolated",
		Notional:         dec("5020"),
	}

	tr.applyPositionEvent(p, event, "short")

	if !p.ShortMarkPrice.Equal(dec("50200")) {
		t.Errorf("expected ShortMarkPrice 50200, got %s", p.ShortMarkPrice)
	}
	if !p.ShortUnrealizedPnL.Equal(dec("-50")) {
		t.Errorf("expected ShortUnrealizedPnL -50, got %s", p.ShortUnrealizedPnL)
	}
	if !p.ShortLiquidationPrice.Equal(dec("65000")) {
		t.Errorf("expected ShortLiquidationPrice 65000, got %s", p.ShortLiquidationPrice)
	}
	if !p.ShortLeverage.Equal(dec("5")) {
		t.Errorf("expected ShortLeverage 5, got %s", p.ShortLeverage)
	}
	if p.ShortMarginMode != "isolated" {
		t.Errorf("expected ShortMarginMode isolated, got %s", p.ShortMarginMode)
	}
	if !p.ShortNotional.Equal(dec("5020")) {
		t.Errorf("expected ShortNotional 5020, got %s", p.ShortNotional)
	}
}

// ---------------------------------------------------------------------------
// Tests: detectAction
// ---------------------------------------------------------------------------

func TestDetectAction_NoneWhenContractsMatchFilled(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		Contracts: dec("0.1"), // matches LongFilledQty
	}

	action := tr.detectAction(p, event, "long")
	if action != reactorActionNone {
		t.Errorf("expected reactorActionNone, got %d", action)
	}
}

func TestDetectAction_NoneWhenFilledQtyZero(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	p.LongFilledQty = decimal.Zero // not yet filled
	event := &domain.ExchangePositionEvent{
		Contracts: dec("0.1"),
	}

	action := tr.detectAction(p, event, "long")
	if action != reactorActionNone {
		t.Errorf("expected reactorActionNone (filled is zero), got %d", action)
	}
}

func TestDetectAction_AutoCloseWhenContractsZero(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		Contracts: decimal.Zero, // fully closed
	}

	action := tr.detectAction(p, event, "long")
	if action != reactorActionAutoClose {
		t.Errorf("expected reactorActionAutoClose, got %d", action)
	}
}

func TestDetectAction_AutoCloseOnShortLeg(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		Contracts: decimal.Zero,
	}

	action := tr.detectAction(p, event, "short")
	if action != reactorActionAutoClose {
		t.Errorf("expected reactorActionAutoClose for short, got %d", action)
	}
}

func TestDetectAction_RebalanceOnPartialClose(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	p.LongFilledQty = dec("1.0")
	event := &domain.ExchangePositionEvent{
		Contracts: dec("0.5"), // reduced from 1.0 to 0.5 (50% reduction, well above 1%)
	}

	action := tr.detectAction(p, event, "long")
	if action != reactorActionRebalance {
		t.Errorf("expected reactorActionRebalance, got %d", action)
	}
}

func TestDetectAction_NoneWithinTolerance(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	p.LongFilledQty = dec("1.0")
	// Contracts differ by 0.005 — within 1% tolerance of 1.0 (threshold = 0.01)
	event := &domain.ExchangePositionEvent{
		Contracts: dec("0.995"),
	}

	action := tr.detectAction(p, event, "long")
	if action != reactorActionNone {
		t.Errorf("expected reactorActionNone (within tolerance), got %d", action)
	}
}

// ---------------------------------------------------------------------------
// Tests: otherLeg
// ---------------------------------------------------------------------------

func TestOtherLeg(t *testing.T) {
	if otherLeg("long") != "short" {
		t.Errorf("otherLeg(long) should be short")
	}
	if otherLeg("short") != "long" {
		t.Errorf("otherLeg(short) should be long")
	}
}

// ---------------------------------------------------------------------------
// Tests: handlePositionEvent (integration with mocks)
// ---------------------------------------------------------------------------

func TestHandlePositionEvent_UpdatesState(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, &mockTradingExchange{id: "okx"})

	event := &domain.ExchangePositionEvent{
		ExchangeID:       "bybit",
		Symbol:           "BTC/USDT:USDT",
		Side:             domain.PositionSideLong,
		Contracts:        dec("0.1"),
		MarkPrice:        dec("51000"),
		UnrealizedPnL:    dec("100"),
		LiquidationPrice: dec("40000"),
		Leverage:         dec("10"),
		MarginMode:       "cross",
		Notional:         dec("5100"),
	}

	if err := tr.handlePositionEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if !updated.LongMarkPrice.Equal(dec("51000")) {
		t.Errorf("expected LongMarkPrice 51000, got %s", updated.LongMarkPrice)
	}
	if !updated.LongLiquidationPrice.Equal(dec("40000")) {
		t.Errorf("expected LongLiquidationPrice 40000, got %s", updated.LongLiquidationPrice)
	}
}

func TestHandlePositionEvent_NoMatchForDifferentSymbol(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, &mockTradingExchange{id: "okx"})

	event := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "ETH/USDT:USDT", // different symbol
		Side:       domain.PositionSideLong,
		Contracts:  dec("0.1"),
		MarkPrice:  dec("3000"),
	}

	if err := tr.handlePositionEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should be unchanged (mark price still zero).
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if !updated.LongMarkPrice.IsZero() {
		t.Errorf("expected LongMarkPrice to be unchanged (zero), got %s", updated.LongMarkPrice)
	}
}

func TestHandlePositionEvent_IgnoresClosedPositions(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.Status = domain.PositionStatusClosed // closed
	ps.positions[p.ID] = p

	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, &mockTradingExchange{id: "okx"})

	event := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideLong,
		Contracts:  decimal.Zero,
		MarkPrice:  dec("51000"),
	}

	if err := tr.handlePositionEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should remain closed (the filter excludes closed positions).
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status to remain closed, got %s", updated.Status)
	}
	if !updated.LongMarkPrice.IsZero() {
		t.Errorf("expected LongMarkPrice unchanged, got %s", updated.LongMarkPrice)
	}
}

// ---------------------------------------------------------------------------
// Tests: autoCloseRemainingLeg
// ---------------------------------------------------------------------------

func TestAutoCloseRemainingLeg_ClosesShort(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, okxMock)

	// Long leg was closed externally — close the short.
	tr.autoCloseRemainingLeg(context.Background(), p, "long")

	// Verify market order was placed.
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 order on okx, got %d", len(okxMock.createdOrders))
	}
	order := okxMock.createdOrders[0]
	if order.side != "buy" {
		t.Errorf("expected buy order to close short, got %s", order.side)
	}
	if order.symbol != "BTC/USDT:USDT" {
		t.Errorf("expected symbol BTC/USDT:USDT, got %s", order.symbol)
	}

	// Verify position was closed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonAutoClose {
		t.Errorf("expected close reason auto_close, got %s", updated.CloseReason)
	}
	if updated.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}

	// Verify order was recorded.
	ps.mu.Lock()
	orders := ps.orders
	ps.mu.Unlock()
	if len(orders) != 1 {
		t.Fatalf("expected 1 order record, got %d", len(orders))
	}
	if orders[0].Leg != "short" {
		t.Errorf("expected order leg short, got %s", orders[0].Leg)
	}
	if orders[0].Action != "close" {
		t.Errorf("expected action close, got %s", orders[0].Action)
	}
}

func TestAutoCloseRemainingLeg_ClosesLong(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	tr := newTestTrader(ps, bybitMock, &mockTradingExchange{id: "okx"})

	// Short leg was closed externally — close the long.
	tr.autoCloseRemainingLeg(context.Background(), p, "short")

	bybitMock.mu.Lock()
	defer bybitMock.mu.Unlock()
	if len(bybitMock.createdOrders) != 1 {
		t.Fatalf("expected 1 order on bybit, got %d", len(bybitMock.createdOrders))
	}
	order := bybitMock.createdOrders[0]
	if order.side != "sell" {
		t.Errorf("expected sell order to close long, got %s", order.side)
	}
}

func TestAutoCloseRemainingLeg_BothLegsAlreadyClosed(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.ShortFilledQty = decimal.Zero // short already has zero qty
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, okxMock)

	// Long was closed externally, short is already zero — should just update status.
	tr.autoCloseRemainingLeg(context.Background(), p, "long")

	// No market order should be placed.
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 0 {
		t.Errorf("expected no orders when remaining qty is zero, got %d", len(okxMock.createdOrders))
	}

	// Position should still be closed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: rebalanceOtherLeg
// ---------------------------------------------------------------------------

func TestRebalanceOtherLeg_ReducesShort(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.LongFilledQty = dec("1.0")
	p.ShortFilledQty = dec("1.0")
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, okxMock)

	// Long was reduced to 0.6 — rebalance short by delta of 0.4.
	tr.rebalanceOtherLeg(context.Background(), p, "long", dec("0.6"))

	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 order on okx, got %d", len(okxMock.createdOrders))
	}
	order := okxMock.createdOrders[0]
	if order.side != "buy" {
		t.Errorf("expected buy order to reduce short, got %s", order.side)
	}
	// Delta = 1.0 - 0.6 = 0.4
	expectedAmount := 0.4
	if order.amount != expectedAmount {
		t.Errorf("expected amount %f, got %f", expectedAmount, order.amount)
	}

	// Verify position was updated.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if updated.Status != domain.PositionStatusPartiallyOpen {
		t.Errorf("expected status partially_open, got %s", updated.Status)
	}
	if !updated.LongFilledQty.Equal(dec("0.6")) {
		t.Errorf("expected LongFilledQty 0.6, got %s", updated.LongFilledQty)
	}
	// ShortFilledQty = 1.0 - 0.4 (filled) = 0.6
	if !updated.ShortFilledQty.Equal(dec("0.6")) {
		t.Errorf("expected ShortFilledQty 0.6, got %s", updated.ShortFilledQty)
	}
}

func TestRebalanceOtherLeg_ReducesLong(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.LongFilledQty = dec("2.0")
	p.ShortFilledQty = dec("2.0")
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	tr := newTestTrader(ps, bybitMock, &mockTradingExchange{id: "okx"})

	// Short was reduced to 1.5 — rebalance long by delta of 0.5.
	tr.rebalanceOtherLeg(context.Background(), p, "short", dec("1.5"))

	bybitMock.mu.Lock()
	defer bybitMock.mu.Unlock()
	if len(bybitMock.createdOrders) != 1 {
		t.Fatalf("expected 1 order on bybit, got %d", len(bybitMock.createdOrders))
	}
	order := bybitMock.createdOrders[0]
	if order.side != "sell" {
		t.Errorf("expected sell order to reduce long, got %s", order.side)
	}
}

func TestRebalanceOtherLeg_DeltaCappedAtOtherQty(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.LongFilledQty = dec("1.0")
	p.ShortFilledQty = dec("0.3") // short is smaller than the delta would be
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, okxMock)

	// Long was reduced from 1.0 to 0.2. Delta = 0.8, but short only has 0.3.
	tr.rebalanceOtherLeg(context.Background(), p, "long", dec("0.2"))

	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(okxMock.createdOrders))
	}
	// Should cap at 0.3 (the short's filled qty).
	expectedAmount := 0.3
	if okxMock.createdOrders[0].amount != expectedAmount {
		t.Errorf("expected amount %f (capped), got %f", expectedAmount, okxMock.createdOrders[0].amount)
	}
}

func TestRebalanceOtherLeg_NoDeltaNoOrder(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.LongFilledQty = dec("1.0")
	p.ShortFilledQty = dec("1.0")
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, okxMock)

	// Long remains at 1.0 — delta is 0, no rebalance needed.
	tr.rebalanceOtherLeg(context.Background(), p, "long", dec("1.0"))

	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 0 {
		t.Errorf("expected no orders when delta is zero, got %d", len(okxMock.createdOrders))
	}
}

// ---------------------------------------------------------------------------
// Mock TradingExchange — zero-fill variant
// ---------------------------------------------------------------------------

// mockZeroFillExchange simulates an exchange where CreateMarketOrder returns
// FilledQty=0, which happens in production when the exchange acknowledges the
// order but CCXT's Filled field is nil (floatToDecimal(nil) → decimal.Zero).
type mockZeroFillExchange struct {
	id            string
	createdOrders []mockOrderCall
	mu            sync.Mutex
}

func (m *mockZeroFillExchange) ID() string { return m.id }
func (m *mockZeroFillExchange) Run(_ context.Context) error {
	return nil
}
func (m *mockZeroFillExchange) Markets(_ context.Context) ([]domain.Market, error) {
	return nil, nil
}
func (m *mockZeroFillExchange) Tickers(_ context.Context, _ []string) (<-chan domain.Ticker, error) {
	return nil, nil
}
func (m *mockZeroFillExchange) FundingRates(_ context.Context) (<-chan domain.FundingRate, error) {
	return nil, nil
}
func (m *mockZeroFillExchange) CreateMarketOrder(_ context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdOrders = append(m.createdOrders, mockOrderCall{symbol: symbol, side: side, amount: amount})
	// Simulate exchange returning zero fill — order acknowledged but not yet confirmed.
	return &domain.ExchangeOrder{
		ExchangeOrderID: "mock-order-zerofill",
		Status:          "open",
		FilledQty:       decimal.Zero, // This is what floatToDecimal(nil) returns
		AveragePrice:    decimal.Zero,
		Cost:            decimal.Zero,
		FeeCost:         decimal.Zero,
	}, nil
}
func (m *mockZeroFillExchange) CloseMarketOrder(_ context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdOrders = append(m.createdOrders, mockOrderCall{symbol: symbol, side: side, amount: amount})
	return &domain.ExchangeOrder{
		ExchangeOrderID: "mock-order-zerofill",
		Status:          "open",
		FilledQty:       decimal.Zero,
		AveragePrice:    decimal.Zero,
		Cost:            decimal.Zero,
		FeeCost:         decimal.Zero,
	}, nil
}
func (m *mockZeroFillExchange) FetchOrder(_ context.Context, _, _ string) (*domain.ExchangeOrder, error) {
	return nil, nil
}
func (m *mockZeroFillExchange) FetchPosition(_ context.Context, _ string) (*exchange.ExchangePosition, error) {
	return nil, nil
}
func (m *mockZeroFillExchange) FetchBalance(_ context.Context) ([]exchange.Balance, error) {
	return nil, nil
}
func (m *mockZeroFillExchange) FetchFundingHistory(_ context.Context, _ string, _ time.Time, _ int64) ([]exchange.FundingHistoryEntry, error) {
	return nil, nil
}
func (m *mockZeroFillExchange) SetMarginMode(_ context.Context, _, _ string, _ int) error {
	return nil
}
func (m *mockZeroFillExchange) HasCredentials() bool { return true }

var _ exchange.TradingExchange = (*mockZeroFillExchange)(nil)
var _ exchange.FundingHistoryFetcher = (*mockZeroFillExchange)(nil)

// ---------------------------------------------------------------------------
// Tests: synchronous reactions prevent recursive cascading
// ---------------------------------------------------------------------------

// TestSynchronousReaction_NoCascadeOnRebalance reproduces the exact user scenario:
// 10/10 → user manually closes 1 on Bybit → reactor rebalances OKX (synchronously,
// DB updated to 9/9) → OKX event arrives with contracts=9 → detectAction sees
// filledQty=9 vs contracts=9 → no cascade.
func TestSynchronousReaction_NoCascadeOnRebalance(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-cascade",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpen,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		LongFilledQty:   dec("10"),
		ShortFilledQty:  dec("10"),
		LongEntryPrice:  dec("50000"),
		ShortEntryPrice: dec("50100"),
	}
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	bybitMock := &mockTradingExchange{id: "bybit"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	// Step 1: Bybit event — user manually closed 1 contract (10 → 9).
	// This triggers a synchronous rebalance that places an order on OKX and
	// updates the DB before handlePositionEvent returns.
	bybitEvent := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideLong,
		Contracts:  dec("9"),
		MarkPrice:  dec("51000"),
	}
	if err := tr.handlePositionEvent(context.Background(), bybitEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the rebalance order was placed on OKX.
	okxMock.mu.Lock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 rebalance order on OKX, got %d", len(okxMock.createdOrders))
	}
	okxMock.mu.Unlock()

	// Verify the DB was updated to 9/9 BEFORE we process the next event.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if !updated.LongFilledQty.Equal(dec("9")) {
		t.Errorf("expected LongFilledQty=9 after rebalance, got %s", updated.LongFilledQty)
	}
	if !updated.ShortFilledQty.Equal(dec("9")) {
		t.Errorf("expected ShortFilledQty=9 after rebalance, got %s", updated.ShortFilledQty)
	}

	// Step 2: OKX event arrives — the rebalance order reduced short from 10 → 9.
	// Because rebalance was synchronous, the DB already has ShortFilledQty=9.
	// detectAction will see filledQty=9 vs contracts=9 → diff=0 → no action.
	okxEvent := &domain.ExchangePositionEvent{
		ExchangeID: "okx",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideShort,
		Contracts:  dec("9"),
		MarkPrice:  dec("50200"),
	}
	if err := tr.handlePositionEvent(context.Background(), okxEvent); err != nil {
		t.Fatalf("unexpected error on OKX event: %v", err)
	}

	// Verify: no cascade — no orders placed on Bybit.
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	if bybitOrders != 0 {
		t.Errorf("expected 0 orders on Bybit (no cascade), got %d", bybitOrders)
	}

	// Verify: still only 1 order on OKX (the original rebalance).
	okxMock.mu.Lock()
	okxOrders := len(okxMock.createdOrders)
	okxMock.mu.Unlock()
	if okxOrders != 1 {
		t.Errorf("expected 1 total order on OKX, got %d", okxOrders)
	}

	// Verify: final state is 9/9.
	ps.mu.Lock()
	final := ps.positions[p.ID]
	ps.mu.Unlock()
	if !final.LongFilledQty.Equal(dec("9")) || !final.ShortFilledQty.Equal(dec("9")) {
		t.Errorf("expected final state 9/9, got %s/%s", final.LongFilledQty, final.ShortFilledQty)
	}
}

// TestSynchronousReaction_PriceUpdateAfterRebalance ensures that a price-only
// event (no contract change) from the counter-exchange after a rebalance is
// processed normally (state updates applied, no false rebalance triggered).
func TestSynchronousReaction_PriceUpdateAfterRebalance(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-price",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpen,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		LongFilledQty:   dec("10"),
		ShortFilledQty:  dec("10"),
		LongEntryPrice:  dec("50000"),
		ShortEntryPrice: dec("50100"),
	}
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	bybitMock := &mockTradingExchange{id: "bybit"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	// Step 1: Trigger rebalance (Bybit 10 → 9).
	bybitEvent := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideLong,
		Contracts:  dec("9"),
		MarkPrice:  dec("51000"),
	}
	if err := tr.handlePositionEvent(context.Background(), bybitEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step 2: OKX sends a price-only update (contracts still 9, just PnL changed).
	okxEvent := &domain.ExchangePositionEvent{
		ExchangeID:    "okx",
		Symbol:        "BTC/USDT:USDT",
		Side:          domain.PositionSideShort,
		Contracts:     dec("9"),
		MarkPrice:     dec("50300"),
		UnrealizedPnL: dec("-20"),
	}
	if err := tr.handlePositionEvent(context.Background(), okxEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No additional orders should be placed.
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	if bybitOrders != 0 {
		t.Errorf("expected 0 orders on Bybit, got %d", bybitOrders)
	}

	// The price update should have been applied to the DB.
	ps.mu.Lock()
	final := ps.positions[p.ID]
	ps.mu.Unlock()
	if !final.ShortMarkPrice.Equal(dec("50300")) {
		t.Errorf("expected ShortMarkPrice=50300, got %s", final.ShortMarkPrice)
	}
	if !final.ShortUnrealizedPnL.Equal(dec("-20")) {
		t.Errorf("expected ShortUnrealizedPnL=-20, got %s", final.ShortUnrealizedPnL)
	}
}

// TestSynchronousReaction_NoCascadeOnAutoClose verifies that when one leg is
// fully closed and the reactor auto-closes the other, the resulting counter-
// exchange event does not trigger another auto-close.
func TestSynchronousReaction_NoCascadeOnAutoClose(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-autoclose",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpen,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		LongFilledQty:   dec("5"),
		ShortFilledQty:  dec("5"),
		LongEntryPrice:  dec("50000"),
		ShortEntryPrice: dec("50100"),
	}
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	bybitMock := &mockTradingExchange{id: "bybit"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	// Bybit long fully closed (contracts = 0). Reactor auto-closes OKX short
	// synchronously and marks position as closed.
	bybitEvent := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideLong,
		Contracts:  decimal.Zero,
	}
	if err := tr.handlePositionEvent(context.Background(), bybitEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Position should be closed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected position closed, got %s", updated.Status)
	}

	// OKX event arrives showing contracts=0 (from the auto-close order).
	// Position is already closed, so ListPositions won't return it.
	okxEvent := &domain.ExchangePositionEvent{
		ExchangeID: "okx",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideShort,
		Contracts:  decimal.Zero,
	}
	if err := tr.handlePositionEvent(context.Background(), okxEvent); err != nil {
		t.Fatalf("unexpected error on OKX event: %v", err)
	}

	// No additional orders should have been placed on Bybit.
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	if bybitOrders != 0 {
		t.Errorf("expected 0 orders on Bybit (no cascade), got %d", bybitOrders)
	}

	// Exactly 1 auto-close order on OKX.
	okxMock.mu.Lock()
	okxOrders := len(okxMock.createdOrders)
	okxMock.mu.Unlock()
	if okxOrders != 1 {
		t.Errorf("expected 1 auto-close order on OKX, got %d", okxOrders)
	}
}

// ---------------------------------------------------------------------------
// Tests: zero-fill exchange does not cause cascading
// ---------------------------------------------------------------------------

// TestZeroFillExchange_NoCascadeOnRebalance proves the delta-based fix prevents
// cascading when CreateMarketOrder returns FilledQty=0 (exchange acknowledged
// but not yet confirmed).
//
// Without the fix (using order.FilledQty):
//
//	Bybit event 10→9: rebalance OKX, DB updates ShortFilledQty = 10 - 0 = 10 (WRONG!)
//	OKX event contracts=9: detectAction sees filledQty=10 vs 9 → rebalance again → cascade
//
// With the fix (using delta):
//
//	Bybit event 10→9: rebalance OKX, DB updates ShortFilledQty = 10 - 1 = 9 (correct)
//	OKX event contracts=9: detectAction sees filledQty=9 vs 9 → no action → no cascade
func TestZeroFillExchange_NoCascadeOnRebalance(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-zerofill",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpen,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		LongFilledQty:   dec("10"),
		ShortFilledQty:  dec("10"),
		LongEntryPrice:  dec("50000"),
		ShortEntryPrice: dec("50100"),
	}
	ps.positions[p.ID] = p

	// OKX returns FilledQty=0 on CreateMarketOrder (simulates the production bug).
	okxMock := &mockZeroFillExchange{id: "okx"}
	bybitMock := &mockTradingExchange{id: "bybit"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	// Step 1: Bybit event — user manually closed 1 contract (10 → 9).
	bybitEvent := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideLong,
		Contracts:  dec("9"),
		MarkPrice:  dec("51000"),
	}
	if err := tr.handlePositionEvent(context.Background(), bybitEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: rebalance order was placed on OKX.
	okxMock.mu.Lock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 rebalance order on OKX, got %d", len(okxMock.createdOrders))
	}
	okxMock.mu.Unlock()

	// CRITICAL: even though OKX returned FilledQty=0, the DB should show 9/9
	// because we use delta (=1) instead of order.FilledQty (=0).
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if !updated.LongFilledQty.Equal(dec("9")) {
		t.Errorf("expected LongFilledQty=9, got %s", updated.LongFilledQty)
	}
	if !updated.ShortFilledQty.Equal(dec("9")) {
		t.Errorf("expected ShortFilledQty=9 (delta-based), got %s — this means order.FilledQty was used instead of delta!", updated.ShortFilledQty)
	}

	// Step 2: OKX event arrives with contracts=9 (the actual fill from the rebalance).
	okxEvent := &domain.ExchangePositionEvent{
		ExchangeID: "okx",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideShort,
		Contracts:  dec("9"),
		MarkPrice:  dec("50200"),
	}
	if err := tr.handlePositionEvent(context.Background(), okxEvent); err != nil {
		t.Fatalf("unexpected error on OKX event: %v", err)
	}

	// Verify: NO cascade — no orders placed on Bybit.
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	if bybitOrders != 0 {
		t.Errorf("expected 0 orders on Bybit (no cascade), got %d", bybitOrders)
	}

	// Verify: still only 1 order on OKX.
	okxMock.mu.Lock()
	okxOrders := len(okxMock.createdOrders)
	okxMock.mu.Unlock()
	if okxOrders != 1 {
		t.Errorf("expected 1 total order on OKX, got %d", okxOrders)
	}

	// Verify: final state is 9/9.
	ps.mu.Lock()
	final := ps.positions[p.ID]
	ps.mu.Unlock()
	if !final.LongFilledQty.Equal(dec("9")) || !final.ShortFilledQty.Equal(dec("9")) {
		t.Errorf("expected final state 9/9, got %s/%s", final.LongFilledQty, final.ShortFilledQty)
	}
}

// TestZeroFillExchange_MultiStepNoCascade verifies that even with 3 sequential
// manual partial closes (each triggering a rebalance with zero-fill response),
// the system stabilizes at the correct state without cascading.
func TestZeroFillExchange_MultiStepNoCascade(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-multi-zerofill",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpen,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		LongFilledQty:   dec("10"),
		ShortFilledQty:  dec("10"),
		LongEntryPrice:  dec("50000"),
		ShortEntryPrice: dec("50100"),
	}
	ps.positions[p.ID] = p

	okxMock := &mockZeroFillExchange{id: "okx"}
	bybitMock := &mockTradingExchange{id: "bybit"} // Bybit has normal fills
	tr := newTestTrader(ps, bybitMock, okxMock)

	// Simulate 3 sequential manual closes on Bybit with OKX zero-fill responses.
	steps := []struct {
		bybitContracts string // what Bybit reports after user close
		okxContracts   string // what OKX reports after rebalance
	}{
		{"9", "9"}, // 10→9
		{"7", "7"}, // 9→7
		{"4", "4"}, // 7→4
	}

	for i, step := range steps {
		// Bybit event: user closed some contracts.
		bybitEvent := &domain.ExchangePositionEvent{
			ExchangeID: "bybit",
			Symbol:     "BTC/USDT:USDT",
			Side:       domain.PositionSideLong,
			Contracts:  dec(step.bybitContracts),
			MarkPrice:  dec("51000"),
		}
		if err := tr.handlePositionEvent(context.Background(), bybitEvent); err != nil {
			t.Fatalf("step %d bybit event: unexpected error: %v", i+1, err)
		}

		// OKX event: reports new contract count after rebalance.
		okxEvent := &domain.ExchangePositionEvent{
			ExchangeID: "okx",
			Symbol:     "BTC/USDT:USDT",
			Side:       domain.PositionSideShort,
			Contracts:  dec("9"),
			MarkPrice:  dec("50200"),
		}
		if err := tr.handlePositionEvent(context.Background(), okxEvent); err != nil {
			t.Fatalf("step %d okx event: unexpected error: %v", i+1, err)
		}
	}

	// Verify: exactly 3 rebalance orders on OKX (one per step, no cascades).
	okxMock.mu.Lock()
	okxOrders := len(okxMock.createdOrders)
	okxMock.mu.Unlock()
	if okxOrders != 3 {
		t.Errorf("expected 3 rebalance orders on OKX (one per step), got %d", okxOrders)
	}

	// Verify: NO orders on Bybit (no cascades from OKX events).
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	if bybitOrders != 0 {
		t.Errorf("expected 0 orders on Bybit (no cascade), got %d", bybitOrders)
	}

	// Verify: final state is 4/4.
	ps.mu.Lock()
	final := ps.positions[p.ID]
	ps.mu.Unlock()
	if !final.LongFilledQty.Equal(dec("4")) || !final.ShortFilledQty.Equal(dec("4")) {
		t.Errorf("expected final state 4/4, got %s/%s", final.LongFilledQty, final.ShortFilledQty)
	}
}

// ---------------------------------------------------------------------------
// Tests: isPositionAlreadyClosed
// ---------------------------------------------------------------------------

func TestIsPositionAlreadyClosed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "non-domain error",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "domain ErrPositionAlreadyClosed — direct",
			err:  exchange.ErrPositionAlreadyClosed,
			want: true,
		},
		{
			name: "domain ErrPositionAlreadyClosed — wrapped by bybit adapter",
			err:  fmt.Errorf("bybit create market order sell BTC/USDT:USDT: %w: current position is zero", exchange.ErrPositionAlreadyClosed),
			want: true,
		},
		{
			name: "domain ErrPositionAlreadyClosed — wrapped by okx adapter",
			err:  fmt.Errorf("okx create market order buy BTC/USDT:USDT: %w: sCode 51169", exchange.ErrPositionAlreadyClosed),
			want: true,
		},
		{
			name: "domain ErrPositionAlreadyClosed — deeply wrapped",
			err:  fmt.Errorf("create market order: %w", fmt.Errorf("bybit: %w", exchange.ErrPositionAlreadyClosed)),
			want: true,
		},
		{
			name: "generic exchange error — not position closed",
			err:  fmt.Errorf("bybit create market order: insufficient balance"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPositionAlreadyClosed(tt.err)
			if got != tt.want {
				t.Errorf("isPositionAlreadyClosed() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: matchLeg with empty side
// ---------------------------------------------------------------------------

func TestMatchLeg_EmptySideLongExchange(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		ExchangeID: "bybit", // long exchange
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideNone, // empty — position closed on exchange
	}

	leg := tr.matchLeg(p, event)
	if leg != "long" {
		t.Errorf("expected long (inferred from exchange ID), got %q", leg)
	}
}

func TestMatchLeg_EmptySideShortExchange(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		ExchangeID: "okx", // short exchange
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideNone,
	}

	leg := tr.matchLeg(p, event)
	if leg != "short" {
		t.Errorf("expected short (inferred from exchange ID), got %q", leg)
	}
}

func TestMatchLeg_EmptySideUnknownExchange(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := newOpenPosition()
	event := &domain.ExchangePositionEvent{
		ExchangeID: "binance", // not in the position
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideNone,
	}

	leg := tr.matchLeg(p, event)
	if leg != "" {
		t.Errorf("expected empty for unknown exchange, got %q", leg)
	}
}

// TestHandlePositionEvent_EmptySideAutoClose verifies that a Bybit event with
// side="" and contracts=0 (the format Bybit uses when a position is fully closed)
// is correctly matched to the long leg and triggers auto-close of the other leg.
func TestHandlePositionEvent_EmptySideAutoClose(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	ps.positions[p.ID] = p

	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, okxMock)

	// Bybit sends event with empty side and zero contracts — position fully closed.
	event := &domain.ExchangePositionEvent{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Side:       domain.PositionSideNone,
		Contracts:  decimal.Zero,
		MarkPrice:  dec("50000"),
	}

	if err := tr.handlePositionEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auto-close order was placed on OKX (the short leg).
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 auto-close order on OKX, got %d", len(okxMock.createdOrders))
	}
	order := okxMock.createdOrders[0]
	if order.side != "buy" {
		t.Errorf("expected buy order to close short, got %s", order.side)
	}

	// Verify position was closed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// Mock TradingExchange — already-closed variant
// ---------------------------------------------------------------------------

// mockAlreadyClosedExchange simulates an exchange where CreateMarketOrder
// returns a reduce-only rejection because the position is already at zero.
// The error wraps exchange.ErrPositionAlreadyClosed, matching the contract
// that real adapters (bybit, okx) provide.
type mockAlreadyClosedExchange struct {
	id  string
	err error
	mu  sync.Mutex
}

func (m *mockAlreadyClosedExchange) ID() string { return m.id }
func (m *mockAlreadyClosedExchange) Run(_ context.Context) error {
	return nil
}
func (m *mockAlreadyClosedExchange) Markets(_ context.Context) ([]domain.Market, error) {
	return nil, nil
}
func (m *mockAlreadyClosedExchange) Tickers(_ context.Context, _ []string) (<-chan domain.Ticker, error) {
	return nil, nil
}
func (m *mockAlreadyClosedExchange) FundingRates(_ context.Context) (<-chan domain.FundingRate, error) {
	return nil, nil
}
func (m *mockAlreadyClosedExchange) CreateMarketOrder(_ context.Context, _ string, _ domain.OrderSide, _ float64) (*domain.ExchangeOrder, error) {
	return nil, m.err
}
func (m *mockAlreadyClosedExchange) CloseMarketOrder(_ context.Context, _ string, _ domain.OrderSide, _ float64) (*domain.ExchangeOrder, error) {
	return nil, m.err
}
func (m *mockAlreadyClosedExchange) FetchOrder(_ context.Context, _, _ string) (*domain.ExchangeOrder, error) {
	return nil, nil
}
func (m *mockAlreadyClosedExchange) FetchPosition(_ context.Context, _ string) (*exchange.ExchangePosition, error) {
	return nil, nil
}
func (m *mockAlreadyClosedExchange) FetchBalance(_ context.Context) ([]exchange.Balance, error) {
	return nil, nil
}
func (m *mockAlreadyClosedExchange) FetchFundingHistory(_ context.Context, _ string, _ time.Time, _ int64) ([]exchange.FundingHistoryEntry, error) {
	return nil, nil
}
func (m *mockAlreadyClosedExchange) SetMarginMode(_ context.Context, _, _ string, _ int) error {
	return nil
}
func (m *mockAlreadyClosedExchange) HasCredentials() bool { return true }

var _ exchange.TradingExchange = (*mockAlreadyClosedExchange)(nil)
var _ exchange.FundingHistoryFetcher = (*mockAlreadyClosedExchange)(nil)

// ---------------------------------------------------------------------------
// Tests: executeClose with already-closed legs
// ---------------------------------------------------------------------------

func TestExecuteClose_BothLegsAlreadyClosed(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.Status = domain.PositionStatusClosing
	ps.positions[p.ID] = p

	// Both exchanges reject reduce-only orders because positions are gone.
	bybitMock := &mockAlreadyClosedExchange{
		id:  "bybit",
		err: fmt.Errorf("bybit create market order sell BTC/USDT:USDT: %w: current position is zero", exchange.ErrPositionAlreadyClosed),
	}
	okxMock := &mockAlreadyClosedExchange{
		id:  "okx",
		err: fmt.Errorf("okx create market order buy BTC/USDT:USDT: %w: sCode 51169", exchange.ErrPositionAlreadyClosed),
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeClose(context.Background(), p.UserID, p.ID)
	if err != nil {
		t.Fatalf("expected no error (both legs already closed), got: %v", err)
	}

	// Position should be closed, not failed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
	if updated.CloseReason != domain.CloseReasonNormal {
		t.Errorf("expected close reason normal, got %s", updated.CloseReason)
	}
	if updated.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}
}

func TestExecuteClose_OneLegAlreadyClosed(t *testing.T) {
	ps := newMockPositionStore()
	p := newOpenPosition()
	p.Status = domain.PositionStatusClosing
	ps.positions[p.ID] = p

	// OKX rejects (already closed), Bybit succeeds.
	bybitMock := &mockTradingExchange{id: "bybit"}
	okxMock := &mockAlreadyClosedExchange{
		id:  "okx",
		err: fmt.Errorf("okx create market order buy BTC/USDT:USDT: %w: sCode 51169", exchange.ErrPositionAlreadyClosed),
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeClose(context.Background(), p.UserID, p.ID)
	if err != nil {
		t.Fatalf("expected no error (OKX already closed, Bybit succeeded), got: %v", err)
	}

	// Position should be closed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()
	if updated.Status != domain.PositionStatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}

	// Bybit should have placed one close order.
	bybitMock.mu.Lock()
	defer bybitMock.mu.Unlock()
	if len(bybitMock.createdOrders) != 1 {
		t.Errorf("expected 1 close order on Bybit, got %d", len(bybitMock.createdOrders))
	}
}

// ---------------------------------------------------------------------------
// Mock TradingExchange — fails on CreateMarketOrder
// ---------------------------------------------------------------------------

// mockFailingCreateExchange simulates an exchange where CreateMarketOrder fails
// immediately (e.g. InvalidOrder, amount below minimum precision). All other
// methods behave like a normal mock exchange.
type mockFailingCreateExchange struct {
	mockTradingExchange
	createErr error
}

func (m *mockFailingCreateExchange) CreateMarketOrder(_ context.Context, _ string, _ domain.OrderSide, _ float64) (*domain.ExchangeOrder, error) {
	return nil, m.createErr
}

var _ exchange.TradingExchange = (*mockFailingCreateExchange)(nil)

// ---------------------------------------------------------------------------
// Tests: executeOpen rollback scenarios
// ---------------------------------------------------------------------------

// TestExecuteOpen_LongFailsShortSucceeds_RollsBackShort verifies that when the
// long leg fails immediately (e.g. InvalidOrder) but the short leg succeeds,
// the short leg is rolled back. This is the exact scenario from the production
// logs: bybit rejects with "amount must be greater than minimum amount precision
// of 10" while OKX's sell order fills.
func TestExecuteOpen_LongFailsShortSucceeds_RollsBackShort(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-open-rollback",
		Symbol:          "SOPH/USDT:USDT",
		Status:          domain.PositionStatusPending,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("5"),
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	bybitMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		createErr:           fmt.Errorf("bybit create market order buy SOPH/USDT:USDT: InvalidOrder: amount must be greater than minimum amount precision of 10"),
	}
	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeOpen(context.Background(), p.UserID, p.ID)
	if err == nil {
		t.Fatal("expected error from executeOpen, got nil")
	}

	// Position should be failed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// OKX should have 2 orders: the initial sell (open) + the rollback buy (close).
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 2 {
		t.Fatalf("expected 2 orders on OKX (open + rollback), got %d", len(okxMock.createdOrders))
	}
	if okxMock.createdOrders[0].side != domain.OrderSideSell {
		t.Errorf("expected first OKX order to be sell (open short), got %s", okxMock.createdOrders[0].side)
	}
	if okxMock.createdOrders[1].side != domain.OrderSideBuy {
		t.Errorf("expected second OKX order to be buy (rollback short), got %s", okxMock.createdOrders[1].side)
	}
}

// TestExecuteOpen_ShortFailsLongSucceeds_RollsBackLong verifies the mirror case:
// the short leg fails but the long leg succeeds, so the long is rolled back.
func TestExecuteOpen_ShortFailsLongSucceeds_RollsBackLong(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-open-rollback-2",
		Symbol:          "SOPH/USDT:USDT",
		Status:          domain.PositionStatusPending,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("5"),
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	okxMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		createErr:           fmt.Errorf("okx create market order sell: InvalidOrder: insufficient margin"),
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeOpen(context.Background(), p.UserID, p.ID)
	if err == nil {
		t.Fatal("expected error from executeOpen, got nil")
	}

	// Position should be failed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// Bybit should have 2 orders: the initial buy (open) + the rollback sell (close).
	bybitMock.mu.Lock()
	defer bybitMock.mu.Unlock()
	if len(bybitMock.createdOrders) != 2 {
		t.Fatalf("expected 2 orders on Bybit (open + rollback), got %d", len(bybitMock.createdOrders))
	}
	if bybitMock.createdOrders[0].side != domain.OrderSideBuy {
		t.Errorf("expected first Bybit order to be buy (open long), got %s", bybitMock.createdOrders[0].side)
	}
	if bybitMock.createdOrders[1].side != domain.OrderSideSell {
		t.Errorf("expected second Bybit order to be sell (rollback long), got %s", bybitMock.createdOrders[1].side)
	}
}

// TestExecuteOpen_BothFail_NoRollback verifies that when both legs fail,
// no rollback orders are placed (nothing to roll back).
func TestExecuteOpen_BothFail_NoRollback(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-open-both-fail",
		Symbol:          "SOPH/USDT:USDT",
		Status:          domain.PositionStatusPending,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("5"),
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	bybitMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		createErr:           fmt.Errorf("bybit: InvalidOrder"),
	}
	okxMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		createErr:           fmt.Errorf("okx: InvalidOrder"),
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeOpen(context.Background(), p.UserID, p.ID)
	if err == nil {
		t.Fatal("expected error from executeOpen, got nil")
	}

	// Position should be failed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// Neither exchange should have any close/rollback orders.
	// bybitMock inherits from mockTradingExchange so CreateMarketOrder is overridden,
	// but CloseMarketOrder still works. Check no CloseMarketOrder was called.
	bybitMock.mu.Lock()
	defer bybitMock.mu.Unlock()
	if len(bybitMock.createdOrders) != 0 {
		t.Errorf("expected 0 orders on Bybit, got %d", len(bybitMock.createdOrders))
	}

	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 0 {
		t.Errorf("expected 0 orders on OKX, got %d", len(okxMock.createdOrders))
	}
}

// TestExecuteOpen_OrphanedLegDetectedViaFetchPosition verifies that when
// a leg's CreateMarketOrder returns an error but the exchange actually has an
// open position (order was submitted but polling failed), the rollback fallback
// detects it via FetchPosition and closes it.
func TestExecuteOpen_OrphanedLegDetectedViaFetchPosition(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-open-orphan",
		Symbol:          "SOPH/USDT:USDT",
		Status:          domain.PositionStatusPending,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("5"),
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	// Bybit fails on create — order never submitted.
	bybitMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		createErr:           fmt.Errorf("bybit: InvalidOrder"),
	}
	// OKX also "fails" (e.g. timeout after order was submitted to exchange).
	// But FetchPosition reveals an open short position.
	okxMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{
			id: "okx",
			fetchedPosition: &exchange.ExchangePosition{
				Symbol:    "SOPH/USDT:USDT",
				Side:      domain.PositionSideShort,
				Contracts: dec("5"),
			},
		},
		createErr: fmt.Errorf("okx: timeout waiting for order response"),
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeOpen(context.Background(), p.UserID, p.ID)
	if err == nil {
		t.Fatal("expected error from executeOpen, got nil")
	}

	// Position should be failed.
	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// OKX should have 1 CloseMarketOrder (rollback of orphaned short).
	// CreateMarketOrder failed so createdOrders only has the close call.
	okxMock.mu.Lock()
	defer okxMock.mu.Unlock()
	if len(okxMock.createdOrders) != 1 {
		t.Fatalf("expected 1 rollback order on OKX, got %d", len(okxMock.createdOrders))
	}
	if okxMock.createdOrders[0].side != domain.OrderSideBuy {
		t.Errorf("expected OKX rollback to be buy (close short), got %s", okxMock.createdOrders[0].side)
	}
}
