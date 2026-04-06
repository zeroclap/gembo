package trader

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// stuckOpeningThreshold is the minimum age of an "opening" position before
// reconciliation considers it stuck. Positions newer than this are skipped
// because they might still be actively handled by the executor.
const stuckOpeningThreshold = 5 * time.Minute

// reconcileOnStartup fetches the current exchange state for all active
// positions and reconciles any discrepancies that occurred while the trader
// was down. It runs synchronously before the main loops start, ensuring the
// DB reflects reality before the WebSocket-driven reactor takes over.
//
// Three phases:
//  1. Open/partially_open positions: fetch exchange state via REST, detect
//     closures and partial closes, react through the existing reactor logic.
//  2. Stuck "opening" positions: check what was actually filled on exchanges,
//     complete or rollback accordingly.
//  3. Stuck "closing" positions: re-run the close flow (idempotent).
func (t *Trader) reconcileOnStartup(ctx context.Context) error {
	t.logger.Info("reconcile: starting position reconciliation on startup")

	if err := t.reconcileOpenPositions(ctx); err != nil {
		return fmt.Errorf("reconcile open positions: %w", err)
	}

	if err := t.recoverStuckOpening(ctx); err != nil {
		return fmt.Errorf("recover stuck opening: %w", err)
	}

	if err := t.recoverStuckClosing(ctx); err != nil {
		return fmt.Errorf("recover stuck closing: %w", err)
	}

	t.logger.Info("reconcile: position reconciliation complete")
	return nil
}

// ---------------------------------------------------------------------------
// Phase 1: reconcile open/partially_open positions
// ---------------------------------------------------------------------------

// reconcileOpenPositions loads all open/partially_open positions and checks
// each leg's actual state on the exchange via REST. Any drift (full close,
// partial close, state changes) is processed through the existing reactor
// logic to maintain consistency.
func (t *Trader) reconcileOpenPositions(ctx context.Context) error {
	positions, err := t.store.ListPositions(ctx, domain.PositionFilter{
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
	})
	if err != nil {
		return fmt.Errorf("list positions: %w", err)
	}

	if len(positions) == 0 {
		t.logger.Info("reconcile: no open positions to reconcile")
		return nil
	}

	t.logger.Info("reconcile: reconciling open positions", "count", len(positions))

	for i := range positions {
		p := &positions[i]
		t.reconcilePosition(ctx, p)
	}

	return nil
}

// reconcilePosition fetches the actual exchange state for both legs of a
// position and processes any detected drift through the reactor pipeline.
//
// Both legs are fetched BEFORE processing events to capture a consistent
// snapshot of exchange state. This prevents reconciliation actions (e.g.
// rebalance on leg A) from altering leg B's exchange state before we read it.
func (t *Trader) reconcilePosition(ctx context.Context, p *domain.Position) {
	logger := t.logger.With("position_id", p.ID, "symbol", p.Symbol)

	// Snapshot both legs via REST before taking any action.
	longEvent := t.fetchLegState(ctx, p.LongExchangeID, p.Symbol, logger)
	shortEvent := t.fetchLegState(ctx, p.ShortExchangeID, p.Symbol, logger)

	if longEvent != nil {
		logger.Info("reconcile: processing long leg",
			"exchange", longEvent.ExchangeID,
			"contracts", longEvent.Contracts,
			"side", longEvent.Side,
		)
		if err := t.handlePositionEvent(ctx, longEvent); err != nil {
			logger.Error("reconcile: handle long leg event", "error", err)
		}
	}

	if shortEvent != nil {
		logger.Info("reconcile: processing short leg",
			"exchange", shortEvent.ExchangeID,
			"contracts", shortEvent.Contracts,
			"side", shortEvent.Side,
		)
		if err := t.handlePositionEvent(ctx, shortEvent); err != nil {
			logger.Error("reconcile: handle short leg event", "error", err)
		}
	}
}

// fetchLegState calls FetchPosition via REST for a single exchange+symbol and
// converts the result into an ExchangePositionEvent. Returns nil if the fetch
// fails (the WebSocket reactor will catch up once it starts).
func (t *Trader) fetchLegState(
	ctx context.Context,
	exchangeID, symbol string,
	logger *slog.Logger,
) *domain.ExchangePositionEvent {
	exch, err := t.registry.Get(exchangeID)
	if err != nil {
		logger.Warn("reconcile: exchange not in trading registry",
			"exchange", exchangeID, "error", err)
		return nil
	}

	pos, err := exch.FetchPosition(ctx, symbol)
	if err != nil {
		logger.Warn("reconcile: fetch position failed, skipping leg",
			"exchange", exchangeID, "error", err)
		return nil
	}

	event := &domain.ExchangePositionEvent{
		ExchangeID: exchangeID,
		Symbol:     symbol,
		Timestamp:  time.Now().UTC(),
	}

	if pos == nil || pos.Contracts.IsZero() {
		// No position on exchange — treat as fully closed.
		event.Contracts = decimal.Zero
		event.Side = domain.PositionSideNone
	} else {
		event.Side = pos.Side
		event.Contracts = pos.Contracts
		event.EntryPrice = pos.EntryPrice
		event.MarkPrice = pos.MarkPrice
		event.LiquidationPrice = pos.LiquidationPrice
		event.UnrealizedPnL = pos.UnrealizedPnL
		event.Leverage = pos.Leverage
		event.MarginMode = pos.MarginMode
		event.Notional = pos.Notional
	}

	return event
}

// ---------------------------------------------------------------------------
// Phase 2: recover stuck "opening" positions
// ---------------------------------------------------------------------------

// recoverStuckOpening finds positions stuck in "opening" status (the app died
// during executeOpen) and resolves them by checking the actual exchange state.
//
// Outcomes:
//   - Both legs present on exchange → complete to "open"
//   - Only one leg present → rollback that leg, mark "failed"
//   - No legs present → mark "failed"
//
// Positions newer than stuckOpeningThreshold are skipped — they may still be
// actively handled by the executor.
func (t *Trader) recoverStuckOpening(ctx context.Context) error {
	positions, err := t.store.ListPositions(ctx, domain.PositionFilter{
		Status: []domain.PositionStatus{domain.PositionStatusOpening},
	})
	if err != nil {
		return fmt.Errorf("list opening positions: %w", err)
	}

	if len(positions) == 0 {
		return nil
	}

	t.logger.Info("reconcile: checking stuck opening positions", "count", len(positions))

	for i := range positions {
		p := &positions[i]

		if time.Since(p.CreatedAt) < stuckOpeningThreshold {
			t.logger.Debug("reconcile: opening position too recent, skipping",
				"position_id", p.ID, "age", time.Since(p.CreatedAt))
			continue
		}

		t.recoverOpeningPosition(ctx, p)
	}

	return nil
}

// recoverOpeningPosition inspects the exchange state for a stuck "opening"
// position and either completes it or rolls it back.
func (t *Trader) recoverOpeningPosition(ctx context.Context, p *domain.Position) {
	logger := t.logger.With("position_id", p.ID, "symbol", p.Symbol)
	logger.Warn("reconcile: recovering stuck opening position")

	longExch, longErr := t.registry.Get(p.LongExchangeID)
	shortExch, shortErr := t.registry.Get(p.ShortExchangeID)

	if longErr != nil || shortErr != nil {
		logger.Error("reconcile: exchange not available for recovery",
			"long_err", longErr, "short_err", shortErr)
		t.failPosition(ctx, p, fmt.Errorf("reconcile opening: exchange unavailable"))
		return
	}

	longPos, longFetchErr := longExch.FetchPosition(ctx, p.Symbol)
	shortPos, shortFetchErr := shortExch.FetchPosition(ctx, p.Symbol)

	if longFetchErr != nil || shortFetchErr != nil {
		// Transient fetch error — don't fail the position, try again next restart.
		logger.Error("reconcile: fetch position failed during opening recovery",
			"long_err", longFetchErr, "short_err", shortFetchErr)
		return
	}

	longHas := longPos != nil && longPos.Contracts.GreaterThan(decimal.Zero)
	shortHas := shortPos != nil && shortPos.Contracts.GreaterThan(decimal.Zero)

	switch {
	case longHas && shortHas:
		// Both legs filled — complete the opening.
		logger.Info("reconcile: both legs present on exchanges, completing opening",
			"long_contracts", longPos.Contracts,
			"short_contracts", shortPos.Contracts,
		)
		t.completeStuckOpening(ctx, p, longPos, shortPos, logger)

	case longHas:
		// Only long leg filled — rollback.
		logger.Warn("reconcile: only long leg present, rolling back",
			"long_contracts", longPos.Contracts)
		t.rollbackLeg(ctx, longExch, p.Symbol, domain.OrderSideSell, longPos.Contracts, logger)
		t.failPosition(ctx, p, fmt.Errorf("reconcile opening: only long leg filled, rolled back"))

	case shortHas:
		// Only short leg filled — rollback.
		logger.Warn("reconcile: only short leg present, rolling back",
			"short_contracts", shortPos.Contracts)
		t.rollbackLeg(ctx, shortExch, p.Symbol, domain.OrderSideBuy, shortPos.Contracts, logger)
		t.failPosition(ctx, p, fmt.Errorf("reconcile opening: only short leg filled, rolled back"))

	default:
		// Neither leg has a position — mark as failed.
		logger.Info("reconcile: no legs found on exchanges, marking as failed")
		t.failPosition(ctx, p, fmt.Errorf("reconcile opening: no legs found after timeout"))
	}
}

// completeStuckOpening transitions a stuck "opening" position to "open" using
// the actual exchange state. Entry prices and quantities come from the exchange
// since the original order records may be incomplete.
func (t *Trader) completeStuckOpening(
	ctx context.Context,
	p *domain.Position,
	longPos, shortPos *exchange.ExchangePosition,
	logger *slog.Logger,
) {
	now := time.Now().UTC()
	p.Status = domain.PositionStatusOpen
	p.OpenedAt = &now
	p.LongFilledQty = longPos.Contracts
	p.LongEntryPrice = longPos.EntryPrice
	p.LongMarkPrice = longPos.MarkPrice
	p.LongLiquidationPrice = longPos.LiquidationPrice
	p.LongLeverage = longPos.Leverage
	p.LongMarginMode = longPos.MarginMode
	p.LongNotional = longPos.Notional
	p.ShortFilledQty = shortPos.Contracts
	p.ShortEntryPrice = shortPos.EntryPrice
	p.ShortMarkPrice = shortPos.MarkPrice
	p.ShortLiquidationPrice = shortPos.LiquidationPrice
	p.ShortLeverage = shortPos.Leverage
	p.ShortMarginMode = shortPos.MarginMode
	p.ShortNotional = shortPos.Notional

	if err := t.store.UpdatePosition(ctx, p); err != nil {
		logger.Error("reconcile: update position to open", "error", err)
		return
	}

	logger.Info("reconcile: stuck opening position completed",
		"long_qty", p.LongFilledQty,
		"short_qty", p.ShortFilledQty,
	)

	t.redis.Publish(ctx, rediskey.PositionUpdates(), p.ID)
}

// rollbackLeg places a reduce-only close order to unwind a single leg that
// was filled during a stuck opening. Best-effort — logs errors but doesn't
// propagate them (the position will be marked as failed regardless).
func (t *Trader) rollbackLeg(
	ctx context.Context,
	exch exchange.TradingExchange,
	symbol string,
	side domain.OrderSide,
	contracts decimal.Decimal,
	logger *slog.Logger,
) {
	qty := contracts.InexactFloat64()
	if qty <= 0 {
		return
	}

	logger.Info("reconcile: rolling back leg",
		"exchange", exch.ID(),
		"side", side,
		"qty", qty,
	)

	if _, err := exch.CloseMarketOrder(ctx, symbol, side, qty); err != nil {
		if !isPositionAlreadyClosed(err) {
			logger.Error("reconcile: rollback order failed",
				"exchange", exch.ID(), "error", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Phase 3: recover stuck "closing" positions
// ---------------------------------------------------------------------------

// recoverStuckClosing finds positions stuck in "closing" status and re-runs
// the close flow. executeClose is idempotent — it handles legs that are
// already closed on the exchange via ErrPositionAlreadyClosed.
func (t *Trader) recoverStuckClosing(ctx context.Context) error {
	positions, err := t.store.ListPositions(ctx, domain.PositionFilter{
		Status: []domain.PositionStatus{domain.PositionStatusClosing},
	})
	if err != nil {
		return fmt.Errorf("list closing positions: %w", err)
	}

	if len(positions) == 0 {
		return nil
	}

	t.logger.Info("reconcile: retrying stuck closing positions", "count", len(positions))

	for i := range positions {
		p := &positions[i]
		logger := t.logger.With("position_id", p.ID, "symbol", p.Symbol)
		logger.Warn("reconcile: retrying stuck close")

		if err := t.executeClose(ctx, p.ID); err != nil {
			logger.Error("reconcile: retry close failed", "error", err)
		}
	}

	return nil
}
