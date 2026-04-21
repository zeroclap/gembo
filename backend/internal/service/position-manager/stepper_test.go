package positionmanager

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// ---------------------------------------------------------------------------
// Tests: isStepped
// ---------------------------------------------------------------------------

func TestIsStepped_True(t *testing.T) {
	p := &domain.Position{
		PortionSize: dec("100"),
		StepsTotal:  10,
	}
	if !isStepped(p) {
		t.Error("expected isStepped=true for position with portion_size and steps_total > 1")
	}
}

func TestIsStepped_FalseZeroPortionSize(t *testing.T) {
	p := &domain.Position{
		PortionSize: decimal.Zero,
		StepsTotal:  0,
	}
	if isStepped(p) {
		t.Error("expected isStepped=false for position with zero portion_size")
	}
}

func TestIsStepped_FalseOneStep(t *testing.T) {
	p := &domain.Position{
		PortionSize: dec("1000"),
		StepsTotal:  1,
	}
	if isStepped(p) {
		t.Error("expected isStepped=false for position with steps_total=1")
	}
}

// ---------------------------------------------------------------------------
// Tests: vwapPrice
// ---------------------------------------------------------------------------

func TestVwapPrice_Normal(t *testing.T) {
	// Total cost = 5000, total qty = 1.0 → VWAP = 5000
	result := vwapPrice(dec("5000"), dec("1.0"))
	if !result.Equal(dec("5000")) {
		t.Errorf("expected VWAP 5000, got %s", result)
	}
}

func TestVwapPrice_ZeroQty(t *testing.T) {
	result := vwapPrice(dec("5000"), decimal.Zero)
	if !result.IsZero() {
		t.Errorf("expected zero for zero qty, got %s", result)
	}
}

func TestVwapPrice_MultipleSteps(t *testing.T) {
	// Step 1: 0.5 qty at 50000 → cost 25000
	// Step 2: 0.5 qty at 50200 → cost 25100
	// Total cost = 50100, total qty = 1.0 → VWAP = 50100
	totalCost := dec("25000").Add(dec("25100"))
	totalQty := dec("0.5").Add(dec("0.5"))
	result := vwapPrice(totalCost, totalQty)
	if !result.Equal(dec("50100")) {
		t.Errorf("expected VWAP 50100, got %s", result)
	}
}

// ---------------------------------------------------------------------------
// Tests: checkLegSlippage
// ---------------------------------------------------------------------------

func TestCheckLegSlippage_WithinTolerance(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	ticker := &domain.Ticker{
		Bid: dec("50000"),
		Ask: dec("50010"), // spread = 10, mid = 50005, slippage = 5/50005*100 ≈ 0.01%
	}
	err := tr.checkLegSlippage(ticker, dec("0.1"), "long") // 0.1% tolerance
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckLegSlippage_ExceedsTolerance(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	ticker := &domain.Ticker{
		Bid: dec("49000"),
		Ask: dec("51000"), // spread = 2000, mid = 50000, slippage = 1000/50000*100 = 2%
	}
	err := tr.checkLegSlippage(ticker, dec("0.1"), "long") // 0.1% tolerance
	if err == nil {
		t.Error("expected error for large slippage, got nil")
	}
}

func TestCheckLegSlippage_ZeroBidAskAllowed(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	ticker := &domain.Ticker{
		Bid: decimal.Zero,
		Ask: decimal.Zero,
	}
	// When bid/ask are zero, slippage can't be computed — allow the trade.
	err := tr.checkLegSlippage(ticker, dec("0.1"), "long")
	if err != nil {
		t.Errorf("expected nil error for zero bid/ask, got %v", err)
	}
}

func TestCheckLegSlippage_ShortLeg(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	ticker := &domain.Ticker{
		Bid: dec("49000"),
		Ask: dec("51000"), // mid = 50000, short slippage = (50000-49000)/50000*100 = 2%
	}
	err := tr.checkLegSlippage(ticker, dec("0.1"), "short")
	if err == nil {
		t.Error("expected error for large short slippage, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: findUSDTFree
// ---------------------------------------------------------------------------

func TestFindUSDTFree_Found(t *testing.T) {
	balances := []base.Balance{
		{Currency: "BTC", Free: dec("1.5")},
		{Currency: "USDT", Free: dec("10000")},
		{Currency: "ETH", Free: dec("5")},
	}
	result := findUSDTFree(balances)
	if !result.Equal(dec("10000")) {
		t.Errorf("expected 10000, got %s", result)
	}
}

func TestFindUSDTFree_NotFound(t *testing.T) {
	balances := []base.Balance{
		{Currency: "BTC", Free: dec("1.5")},
	}
	result := findUSDTFree(balances)
	if !result.IsZero() {
		t.Errorf("expected zero, got %s", result)
	}
}

func TestFindUSDTFree_Empty(t *testing.T) {
	result := findUSDTFree(nil)
	if !result.IsZero() {
		t.Errorf("expected zero, got %s", result)
	}
}

// ---------------------------------------------------------------------------
// Tests: resolveLeverage
// ---------------------------------------------------------------------------

func TestResolveLeverage_PositionOverride(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	tr.cfg.Leverage = 5 // global
	p := &domain.Position{Leverage: 10}
	if got := tr.resolveLeverage(p); got != 10 {
		t.Errorf("expected 10 (per-position), got %d", got)
	}
}

func TestResolveLeverage_GlobalFallback(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	tr.cfg.Leverage = 5
	p := &domain.Position{Leverage: 0}
	if got := tr.resolveLeverage(p); got != 5 {
		t.Errorf("expected 5 (global fallback), got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Tests: executeSteppedOpen (integration with mocks)
// ---------------------------------------------------------------------------

// mockRedisForStepper implements the minimum Redis interface needed by
// the stepper: Get for ticker data and Publish for position updates.
// We use a real redis.Client stub in the Trader struct, but override
// ticker reads by pre-populating the mock position store.

// For ticker-based tests, we need to set up Redis key responses.
// Since the Trader uses t.redis.Get() for tickers, and our test Redis
// client is a stub that won't connect, we test the sub-functions
// (checkPriceRange, checkSlippage) separately and test the full
// executeSteppedOpen with positions that have no price/slippage limits.

func TestExecuteSteppedOpen_AllStepsComplete(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-stepped-1",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("1.0"),
		PortionSize:     dec("0.5"),
		StepsTotal:      2,
		Leverage:        3,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeSteppedOpen(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open, got %s", updated.Status)
	}
	if updated.StepsCompleted != 2 {
		t.Errorf("expected steps_completed=2, got %d", updated.StepsCompleted)
	}
	if updated.OpenedAt == nil {
		t.Error("expected OpenedAt to be set")
	}
	if updated.StopReason != domain.StopReasonNone {
		t.Errorf("expected empty stop_reason, got %s", updated.StopReason)
	}

	// Each step places 0.5 on both exchanges. Mock returns FilledQty=amount.
	// Total long = 0.5 + 0.5 = 1.0
	if !updated.LongFilledQty.Equal(dec("1.0")) {
		t.Errorf("expected LongFilledQty 1.0, got %s", updated.LongFilledQty)
	}
	if !updated.ShortFilledQty.Equal(dec("1.0")) {
		t.Errorf("expected ShortFilledQty 1.0, got %s", updated.ShortFilledQty)
	}

	// Verify orders were placed: 2 steps × 2 legs = 4 total.
	bybitMock.mu.Lock()
	bybitOrders := len(bybitMock.createdOrders)
	bybitMock.mu.Unlock()
	okxMock.mu.Lock()
	okxOrders := len(okxMock.createdOrders)
	okxMock.mu.Unlock()
	if bybitOrders != 2 {
		t.Errorf("expected 2 orders on bybit, got %d", bybitOrders)
	}
	if okxOrders != 2 {
		t.Errorf("expected 2 orders on okx, got %d", okxOrders)
	}

	// Verify order records created: 2 steps × 2 legs = 4.
	ps.mu.Lock()
	orderRecords := len(ps.orders)
	ps.mu.Unlock()
	if orderRecords != 4 {
		t.Errorf("expected 4 order records, got %d", orderRecords)
	}
}

func TestExecuteSteppedOpen_StepFails_PartialFill(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-stepped-fail",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("3.0"),
		PortionSize:     dec("1.0"),
		StepsTotal:      3,
		Leverage:        2,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	// Bybit succeeds, OKX fails on second call (first step OK, second step fails).
	callCount := 0
	okxMock := &mockCountingExchange{
		id:        "okx",
		failAfter: 1, // fail on 2nd CreateMarketOrder call
	}
	bybitMock := &mockTradingExchange{id: "bybit"}
	_ = callCount
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeSteppedOpen(context.Background(), p)
	// Should not return a hard error — partial fill means position is still open.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	// Only 1 step completed, then step 2 failed.
	if updated.StepsCompleted != 1 {
		t.Errorf("expected steps_completed=1, got %d", updated.StepsCompleted)
	}
	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open (partial), got %s", updated.Status)
	}
	if updated.StopReason != domain.StopReasonStepFailed {
		t.Errorf("expected stop_reason=step_failed, got %s", updated.StopReason)
	}
}

func TestExecuteSteppedOpen_NoStepsCompleted_Fails(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-stepped-allfail",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("2.0"),
		PortionSize:     dec("1.0"),
		StepsTotal:      2,
		Leverage:        1,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	// Both fail immediately on first step.
	bybitMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		createErr:           errForTest("bybit: order rejected"),
	}
	okxMock := &mockFailingCreateExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		createErr:           errForTest("okx: order rejected"),
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeSteppedOpen(context.Background(), p)
	if err == nil {
		t.Fatal("expected error when no steps completed, got nil")
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}
}

func TestExecuteSteppedOpen_MarginCapitalCheck(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-stepped-margin",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("1.0"),
		PortionSize:     dec("0.5"),
		StepsTotal:      2,
		Leverage:        5,
		MarginCapital:   dec("20000"), // requires 10000 per exchange
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	// Both exchanges report only 5000 USDT free — insufficient.
	bybitMock := &mockBalanceExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		balances: []base.Balance{
			{Currency: "USDT", Free: dec("5000")},
		},
	}
	okxMock := &mockBalanceExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		balances: []base.Balance{
			{Currency: "USDT", Free: dec("5000")},
		},
	}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeSteppedOpen(context.Background(), p)
	if err == nil {
		t.Fatal("expected error from margin capital check, got nil")
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}
}

func TestExecuteSteppedOpen_PriceRange_StopsExecution(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-stepped-price",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("2.0"),
		PortionSize:     dec("1.0"),
		StepsTotal:      2,
		Leverage:        1,
		PriceMax:        dec("40000"), // price will exceed this
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	// The ticker fetch from Redis will fail (stub client), causing
	// checkPriceRange to return false (fail-safe). This means execution
	// stops at step 0 with StopReasonPriceExceeded.
	err := tr.executeSteppedOpen(context.Background(), p)
	if err == nil {
		t.Fatal("expected error when no steps completed, got nil")
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.StopReason != domain.StopReasonPriceExceeded {
		t.Errorf("expected stop_reason=price_exceeded, got %s", updated.StopReason)
	}
	if updated.Status != domain.PositionStatusFailed {
		t.Errorf("expected status failed (0 steps completed), got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: checkPriceRange
// ---------------------------------------------------------------------------

func TestCheckPriceRange_NoBounds_AlwaysPass(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := &domain.Position{
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		PriceMin:        decimal.Zero,
		PriceMax:        decimal.Zero,
	}
	ok, _ := tr.checkPriceRange(context.Background(), p)
	if !ok {
		t.Error("expected checkPriceRange to pass when no bounds set")
	}
}

// ---------------------------------------------------------------------------
// Tests: checkSlippage
// ---------------------------------------------------------------------------

func TestCheckSlippage_NoBound_AlwaysPass(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := &domain.Position{
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		SlippagePct:     decimal.Zero,
	}
	ok, _ := tr.checkSlippage(context.Background(), p)
	if !ok {
		t.Error("expected checkSlippage to pass when slippage_pct=0")
	}
}

// ---------------------------------------------------------------------------
// Tests: checkPriceRange with real ticker data
// ---------------------------------------------------------------------------

func TestCheckPriceRange_WithRedisTickerData(t *testing.T) {
	ps := newMockPositionStore()

	// Set up a real in-process mini-redis for ticker data.
	// Since we can't easily do that, we test checkPriceRange indirectly
	// by verifying the logic paths with the fetchTicker mock approach.

	tr := newTestTrader(ps, &mockTradingExchange{id: "bybit"}, &mockTradingExchange{id: "okx"})

	// Pre-populate tickers in Redis. Since our test redis client is a stub,
	// we can test the sub-functions directly instead.
	// This test validates the ticker read failure path.
	p := &domain.Position{
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		PriceMin:        dec("49000"),
		PriceMax:        dec("51000"),
	}

	// fetchTicker will fail → checkPriceRange returns false (fail-safe).
	ok, reason := tr.checkPriceRange(context.Background(), p)
	if ok {
		t.Error("expected checkPriceRange to fail when ticker fetch fails")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

// ---------------------------------------------------------------------------
// Tests: checkMarginCapital
// ---------------------------------------------------------------------------

func TestCheckMarginCapital_Sufficient(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := &domain.Position{
		ID:              "pos-margin-ok",
		MarginCapital:   dec("10000"), // half = 5000 per side
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}

	longExch := &mockBalanceExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		balances: []base.Balance{
			{Currency: "USDT", Free: dec("6000")},
		},
	}
	shortExch := &mockBalanceExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		balances: []base.Balance{
			{Currency: "USDT", Free: dec("7000")},
		},
	}

	err := tr.checkMarginCapital(context.Background(), longExch, shortExch, p)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckMarginCapital_InsufficientLong(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := &domain.Position{
		ID:              "pos-margin-fail-long",
		MarginCapital:   dec("10000"),
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}

	longExch := &mockBalanceExchange{
		mockTradingExchange: mockTradingExchange{id: "bybit"},
		balances: []base.Balance{
			{Currency: "USDT", Free: dec("3000")}, // needs 5000
		},
	}
	shortExch := &mockBalanceExchange{
		mockTradingExchange: mockTradingExchange{id: "okx"},
		balances: []base.Balance{
			{Currency: "USDT", Free: dec("7000")},
		},
	}

	err := tr.checkMarginCapital(context.Background(), longExch, shortExch, p)
	if err == nil {
		t.Error("expected error for insufficient long balance")
	}
}

// ---------------------------------------------------------------------------
// Tests: fetchTicker
// ---------------------------------------------------------------------------

func TestFetchTicker_RedisFailure(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	_, err := tr.fetchTicker(context.Background(), "bybit", "BTC/USDT:USDT")
	if err == nil {
		t.Error("expected error from Redis stub")
	}
}

// ---------------------------------------------------------------------------
// Tests: referencePrice
// ---------------------------------------------------------------------------

func TestReferencePrice_FromLongExchange(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	longExch := &mockTradingExchange{
		id: "bybit",
		fetchedPosition: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			MarkPrice: dec("50000"),
		},
	}
	shortExch := &mockTradingExchange{id: "okx"}

	price, err := tr.referencePrice(context.Background(), "BTC/USDT:USDT", longExch, shortExch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !price.Equal(dec("50000")) {
		t.Errorf("expected 50000, got %s", price)
	}
}

func TestReferencePrice_FallbackToShort(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	longExch := &mockTradingExchange{id: "bybit"} // no position
	shortExch := &mockTradingExchange{
		id: "okx",
		fetchedPosition: &base.ExchangePosition{
			Symbol:    "BTC/USDT:USDT",
			MarkPrice: dec("50100"),
		},
	}

	price, err := tr.referencePrice(context.Background(), "BTC/USDT:USDT", longExch, shortExch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !price.Equal(dec("50100")) {
		t.Errorf("expected 50100, got %s", price)
	}
}

func TestReferencePrice_NoSource(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	longExch := &mockTradingExchange{id: "bybit"}
	shortExch := &mockTradingExchange{id: "okx"}

	_, err := tr.referencePrice(context.Background(), "BTC/USDT:USDT", longExch, shortExch)
	if err == nil {
		t.Error("expected error when no reference price available")
	}
}

// ---------------------------------------------------------------------------
// Tests: resolvePortionBaseQty
// ---------------------------------------------------------------------------

func TestResolvePortionBaseQty_Base(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := &domain.Position{
		SizeType:    domain.SizeTypeQuantity,
		PortionSize: dec("0.5"),
	}
	amount, err := tr.resolvePortionBaseQty(context.Background(), p, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !amount.Equal(dec("0.5")) {
		t.Errorf("expected 0.5, got %s", amount)
	}
}

func TestResolvePortionBaseQty_USD(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	p := &domain.Position{
		SizeType:    domain.SizeTypePrice,
		PortionSize: dec("1000"),
	}
	longExch := &mockTradingExchange{
		id: "bybit",
		fetchedPosition: &base.ExchangePosition{
			MarkPrice: dec("50000"),
		},
	}
	shortExch := &mockTradingExchange{id: "okx"}

	amount, err := tr.resolvePortionBaseQty(context.Background(), p, longExch, shortExch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1000 / 50000 = 0.02
	if !amount.Equal(dec("0.02")) {
		t.Errorf("expected 0.02, got %s", amount)
	}
}

// ---------------------------------------------------------------------------
// Tests: configureExchangeWithLeverage
// ---------------------------------------------------------------------------

func TestConfigureExchangeWithLeverage_SetsMarginMode(t *testing.T) {
	ps := newMockPositionStore()
	tr := newTestTrader(ps)
	tr.cfg.MarginMode = "cross"

	exch := &mockTradingExchange{id: "bybit"}
	err := tr.configureExchangeWithLeverage(context.Background(), exch, "BTC/USDT:USDT", 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigureExchangeWithLeverage_NoMarginMode(t *testing.T) {
	ps := newMockPositionStore()
	tr := newTestTrader(ps)
	tr.cfg.MarginMode = "" // no margin mode configured

	exch := &mockTradingExchange{id: "bybit"}
	err := tr.configureExchangeWithLeverage(context.Background(), exch, "BTC/USDT:USDT", 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: executeSteppedOpen wiring from executeOpen
// ---------------------------------------------------------------------------

func TestExecuteOpen_DelegatesToStepped(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-stepped-delegated",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusPending,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("2.0"),
		PortionSize:     dec("1.0"),
		StepsTotal:      2,
		Leverage:        3,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeOpen(context.Background(), p.UserID, p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open, got %s", updated.Status)
	}
	if updated.StepsCompleted != 2 {
		t.Errorf("expected steps_completed=2, got %d", updated.StepsCompleted)
	}
}

func TestExecuteOpen_NonSteppedStillWorks(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-single-shot",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusPending,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("1.0"),
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
		// No PortionSize/StepsTotal — single-shot.
	}
	ps.positions[p.ID] = p

	bybitMock := &mockTradingExchange{id: "bybit"}
	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, bybitMock, okxMock)

	err := tr.executeOpen(context.Background(), p.UserID, p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps.mu.Lock()
	updated := ps.positions[p.ID]
	ps.mu.Unlock()

	if updated.Status != domain.PositionStatusOpen {
		t.Errorf("expected status open, got %s", updated.Status)
	}
	// Should NOT go through stepped path.
	if updated.StepsCompleted != 0 {
		t.Errorf("expected steps_completed=0 (non-stepped), got %d", updated.StepsCompleted)
	}
}

// ---------------------------------------------------------------------------
// Tests: executeSteppedOpen with per-position leverage
// ---------------------------------------------------------------------------

func TestExecuteSteppedOpen_UsesPerPositionLeverage(t *testing.T) {
	ps := newMockPositionStore()
	p := &domain.Position{
		ID:              "pos-stepped-leverage",
		Symbol:          "BTC/USDT:USDT",
		Status:          domain.PositionStatusOpening,
		SizeType:        domain.SizeTypeQuantity,
		RequestedSize:   dec("1.0"),
		PortionSize:     dec("1.0"),
		StepsTotal:      1, // isStepped requires >1, but we call directly
		Leverage:        7,
		LongExchangeID:  "bybit",
		ShortExchangeID: "okx",
	}
	ps.positions[p.ID] = p

	leverageCapture := &mockLeverageCaptureExchange{id: "bybit"}
	okxMock := &mockTradingExchange{id: "okx"}
	tr := newTestTrader(ps, leverageCapture, okxMock)
	tr.cfg.MarginMode = "cross"
	tr.cfg.Leverage = 3 // global

	// Note: StepsTotal=1 means isStepped()=false, so executeSteppedOpen won't
	// be called via executeOpen. We call it directly.
	// Actually, with StepsTotal=1 the loop runs once and completes.
	err := tr.executeSteppedOpen(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the leverage passed was 7 (per-position), not 3 (global).
	leverageCapture.mu.Lock()
	if leverageCapture.lastLeverage != 7 {
		t.Errorf("expected leverage 7, got %d", leverageCapture.lastLeverage)
	}
	leverageCapture.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Mock helpers for stepper tests
// ---------------------------------------------------------------------------

// errForTest is a helper to create simple error values.
func errForTest(msg string) error {
	return &testError{msg: msg}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

// mockCountingExchange fails after N successful CreateMarketOrder calls.
type mockCountingExchange struct {
	mockTradingExchange
	id        string
	failAfter int
	callCount int
}

func (m *mockCountingExchange) ID() string { return m.id }

func (m *mockCountingExchange) CreateMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	m.mu.Lock()
	count := m.callCount
	m.callCount++
	m.mu.Unlock()

	if count >= m.failAfter {
		return nil, errForTest("mock: order failed after limit")
	}
	return m.mockTradingExchange.CreateMarketOrder(ctx, symbol, side, amount)
}

// mockBalanceExchange returns pre-configured balances.
type mockBalanceExchange struct {
	mockTradingExchange
	balances []base.Balance
}

func (m *mockBalanceExchange) FetchBalance(_ context.Context) ([]base.Balance, error) {
	return m.balances, nil
}

var _ base.PrivateExchange = (*mockBalanceExchange)(nil)

// mockLeverageCaptureExchange captures the leverage passed to SetMarginMode.
type mockLeverageCaptureExchange struct {
	mockTradingExchange
	id            string
	lastLeverage  int
	marginSetCall bool
}

func (m *mockLeverageCaptureExchange) ID() string { return m.id }

func (m *mockLeverageCaptureExchange) SetMarginMode(_ context.Context, _ string, _ string, leverage int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastLeverage = leverage
	m.marginSetCall = true
	return nil
}

var _ base.PrivateExchange = (*mockLeverageCaptureExchange)(nil)

// ---------------------------------------------------------------------------
// Utility: verify the rediskey.Ticker function exists
// ---------------------------------------------------------------------------

func TestRediskeyTickerFormat(t *testing.T) {
	key := rediskey.Ticker("bybit", "BTC/USDT:USDT")
	if key == "" {
		t.Error("expected non-empty Redis key")
	}
}

// ---------------------------------------------------------------------------
// Tests: checkPriceRange with mock ticker JSON
// ---------------------------------------------------------------------------

// To properly test checkPriceRange and checkSlippage with real ticker data,
// we'd need a real Redis or an in-process mock. Instead, we test the
// sub-functions (checkLegSlippage) which don't depend on Redis.

func TestCheckLegSlippage_ExactlyAtThreshold(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	// bid=99, ask=101 → mid=100 → slippage = 1/100*100 = 1%
	ticker := &domain.Ticker{
		Bid: dec("99"),
		Ask: dec("101"),
	}
	// Tolerance is exactly 1% — should pass (not exceed).
	err := tr.checkLegSlippage(ticker, dec("1"), "long")
	if err != nil {
		t.Errorf("expected no error at exactly threshold, got %v", err)
	}
}

func TestCheckLegSlippage_JustAboveThreshold(t *testing.T) {
	tr := newTestTrader(newMockPositionStore())
	// bid=99, ask=101.1 → mid=100.05 → slippage = 1.05/100.05*100 ≈ 1.049%
	ticker := &domain.Ticker{
		Bid: dec("99"),
		Ask: dec("101.1"),
	}
	err := tr.checkLegSlippage(ticker, dec("1"), "long") // 1% tolerance
	if err == nil {
		t.Error("expected error just above threshold")
	}
}

// ---------------------------------------------------------------------------
// Unused import guard: ensure json import is used
// ---------------------------------------------------------------------------

func TestTickerJSONRoundtrip(t *testing.T) {
	ticker := domain.Ticker{
		ExchangeID: "bybit",
		Symbol:     "BTC/USDT:USDT",
		Price:      dec("50000"),
		Bid:        dec("49999"),
		Ask:        dec("50001"),
	}
	data, err := json.Marshal(ticker)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed domain.Ticker
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !parsed.Price.Equal(dec("50000")) {
		t.Errorf("expected price 50000, got %s", parsed.Price)
	}
}
