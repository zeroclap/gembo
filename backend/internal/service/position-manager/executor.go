package positionmanager

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// runExecutor subscribes to Redis Pub/Sub for position open and close signals.
// When a position ID arrives on the pending channel, it executes the open flow.
// When a position ID arrives on the closing channel, it executes the close flow.
func (t *Trader) runExecutor(ctx context.Context) error {
	pubsub := t.redis.PSubscribe(ctx,
		rediskey.UserPositionPendingPattern(),
		rediskey.UserPositionClosingPattern(),
	)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			positionID := msg.Payload
			userID := rediskey.UserIDFromChannel(msg.Channel)
			if userID == "" {
				t.logger.Warn("received position signal with invalid channel", "channel", msg.Channel)
				continue
			}

			switch msg.Channel {
			case rediskey.UserPositionPending(userID):
				t.logger.Info("received open signal", "position_id", positionID)
				if err := t.executeOpen(ctx, userID, positionID); err != nil {
					t.logger.Error("execute open failed", "position_id", positionID, "error", err)
				}

			case rediskey.UserPositionClosing(userID):
				t.logger.Info("received close signal", "position_id", positionID)
				if err := t.executeClose(ctx, userID, positionID); err != nil {
					t.logger.Error("execute close failed", "position_id", positionID, "error", err)
				}
			}
		}
	}
}

// executeOpen handles the full open flow for a position:
//  1. Fetch position from PG and validate status
//  2. Set margin mode (with leverage) on both exchanges
//  3. Place market orders on both legs concurrently
//  4. Poll orders until filled
//  5. Update position with fill details and transition to "open"
func (t *Trader) executeOpen(ctx context.Context, userID, positionID string) error {
	p, err := t.store.GetPosition(ctx, userID, positionID)
	if err != nil {
		return fmt.Errorf("get position: %w", err)
	}
	if p == nil {
		return fmt.Errorf("position %s not found", positionID)
	}
	if p.Status != domain.PositionStatusPending {
		t.logger.Warn("position not pending, skipping open",
			"position_id", positionID, "status", p.Status)
		return nil
	}

	// Transition to opening.
	p.Status = domain.PositionStatusOpening
	if err := t.store.UpdatePosition(ctx, p); err != nil {
		return fmt.Errorf("update position to opening: %w", err)
	}

	// Stepped execution: delegate to the stepped executor if configured.
	if isStepped(p) {
		return t.executeSteppedOpen(ctx, p)
	}

	longExch, err := t.tradingExchangeForLeg(ctx, p, "long")
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("long exchange: %w", err))
	}
	shortExch, err := t.tradingExchangeForLeg(ctx, p, "short")
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("short exchange: %w", err))
	}

	// Configure leverage and margin mode on both exchanges.
	// Use per-position leverage when set, falling back to global config.
	leverage := t.resolveLeverage(p)
	if err := t.configureExchangeWithLeverage(ctx, longExch, p.Symbol, leverage); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("configure long exchange: %w", err))
	}
	if err := t.configureExchangeWithLeverage(ctx, shortExch, p.Symbol, leverage); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("configure short exchange: %w", err))
	}

	longContractSize, shortContractSize, err := t.resolveLegContractSizes(ctx, p)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("resolve contract sizes: %w", err))
	}
	p.LongContractSize = longContractSize
	p.ShortContractSize = shortContractSize

	baseQty, err := t.resolveBaseQuantity(ctx, p, longExch, shortExch)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("resolve base quantity: %w", err))
	}
	p.RequestedBaseQty = baseQty

	longContracts := baseQtyToContracts(baseQty, longContractSize)
	shortContracts := baseQtyToContracts(baseQty, shortContractSize)
	if longContracts.LessThanOrEqual(decimal.Zero) || shortContracts.LessThanOrEqual(decimal.Zero) {
		return t.failPosition(ctx, p, fmt.Errorf("resolved non-positive contracts: long=%s short=%s", longContracts, shortContracts))
	}

	if err := t.store.UpdatePosition(ctx, p); err != nil {
		return fmt.Errorf("persist sizing fields: %w", err)
	}

	// Create order records in PG before submitting to exchanges.
	longOrder := &domain.ExchangeOrder{
		UserID:     p.UserID,
		PositionID: p.ID,
		ExchangeID: p.LongExchangeID,
		Symbol:     p.Symbol,
		Side:       domain.OrderSideBuy,
		Leg:        "long",
		Action:     "open",
		Status:     "pending",
	}
	shortOrder := &domain.ExchangeOrder{
		UserID:     p.UserID,
		PositionID: p.ID,
		ExchangeID: p.ShortExchangeID,
		Symbol:     p.Symbol,
		Side:       domain.OrderSideSell,
		Leg:        "short",
		Action:     "open",
		Status:     "pending",
	}

	if err := t.store.CreateOrder(ctx, longOrder); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("create long order record: %w", err))
	}
	if err := t.store.CreateOrder(ctx, shortOrder); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("create short order record: %w", err))
	}

	// Place both orders concurrently.
	// IMPORTANT: we do NOT use errgroup.WithContext here because one leg
	// failing must not cancel the other's context. If leg A fails instantly
	// (e.g. InvalidOrder) but leg B already submitted an order to the exchange,
	// cancelling B's context would prevent us from capturing its result — leaving
	// an orphaned position on the exchange with no rollback.
	type orderResult struct {
		order *domain.ExchangeOrder
		err   error
	}

	var longRes, shortRes orderResult

	var g errgroup.Group

	g.Go(func() error {
		filled, err := t.placeAndPoll(
			ctx,
			longExch,
			p.Symbol,
			domain.OrderSideBuy,
			longContracts,
			baseQty,
			longContractSize,
			longOrder,
		)
		longRes = orderResult{order: filled, err: err}
		// Always return nil — we handle errors ourselves after both goroutines finish.
		return nil
	})

	g.Go(func() error {
		filled, err := t.placeAndPoll(
			ctx,
			shortExch,
			p.Symbol,
			domain.OrderSideSell,
			shortContracts,
			baseQty,
			shortContractSize,
			shortOrder,
		)
		shortRes = orderResult{order: filled, err: err}
		return nil
	})

	_ = g.Wait() // both goroutines always return nil

	if longRes.err != nil || shortRes.err != nil {
		err := buildOpenError(longRes.err, shortRes.err)
		t.attemptRollback(ctx, p, longExch, shortExch, longRes.order, shortRes.order)
		return t.failPosition(ctx, p, err)
	}

	longResult, shortResult := longRes.order, shortRes.order

	// Both legs filled — update position with fill details.
	now := time.Now().UTC()
	p.Status = domain.PositionStatusOpen
	p.OpenedAt = &now
	p.LongEntryPrice = longResult.AveragePrice
	p.LongFilledQty = longResult.FilledQty
	p.LongFilledBaseQty = contractsToBaseQty(longResult.FilledQty, p.LongContractSize)
	p.LongEntryCost = longResult.Cost
	p.LongEntryFees = longResult.FeeCost
	p.ShortEntryPrice = shortResult.AveragePrice
	p.ShortFilledQty = shortResult.FilledQty
	p.ShortFilledBaseQty = contractsToBaseQty(shortResult.FilledQty, p.ShortContractSize)
	p.ShortEntryCost = shortResult.Cost
	p.ShortEntryFees = shortResult.FeeCost

	if err := t.store.UpdatePosition(ctx, p); err != nil {
		return fmt.Errorf("update position to open: %w", err)
	}

	t.logger.Info("position opened",
		"position_id", p.ID,
		"symbol", p.Symbol,
		"long_price", p.LongEntryPrice,
		"short_price", p.ShortEntryPrice,
		"long_qty", p.LongFilledQty,
		"short_qty", p.ShortFilledQty,
	)

	// Notify position update via Redis Pub/Sub.
	t.redis.Publish(ctx, rediskey.UserPositionUpdates(p.UserID), p.ID)

	return nil
}

// executeClose handles the full close flow for a position:
//  1. Fetch position and validate status
//  2. Place close orders on both legs concurrently
//  3. Update position with exit details and transition to "closed"
func (t *Trader) executeClose(ctx context.Context, userID, positionID string) error {
	p, err := t.store.GetPosition(ctx, userID, positionID)
	if err != nil {
		return fmt.Errorf("get position: %w", err)
	}
	if p == nil {
		return fmt.Errorf("position %s not found", positionID)
	}
	if p.Status != domain.PositionStatusClosing {
		t.logger.Warn("position not closing, skipping",
			"position_id", positionID, "status", p.Status)
		return nil
	}

	longExch, err := t.tradingExchangeForLeg(ctx, p, "long")
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("long exchange: %w", err))
	}
	shortExch, err := t.tradingExchangeForLeg(ctx, p, "short")
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("short exchange: %w", err))
	}

	// Create close order records.
	longCloseOrder := &domain.ExchangeOrder{
		UserID:     p.UserID,
		PositionID: p.ID,
		ExchangeID: p.LongExchangeID,
		Symbol:     p.Symbol,
		Side:       domain.OrderSideSell, // close long by selling
		Leg:        "long",
		Action:     "close",
		Status:     "pending",
	}
	shortCloseOrder := &domain.ExchangeOrder{
		UserID:     p.UserID,
		PositionID: p.ID,
		ExchangeID: p.ShortExchangeID,
		Symbol:     p.Symbol,
		Side:       domain.OrderSideBuy, // close short by buying
		Leg:        "short",
		Action:     "close",
		Status:     "pending",
	}

	if err := t.store.CreateOrder(ctx, longCloseOrder); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("create long close order record: %w", err))
	}
	if err := t.store.CreateOrder(ctx, shortCloseOrder); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("create short close order record: %w", err))
	}

	// Place close orders concurrently.
	g, gCtx := errgroup.WithContext(ctx)
	var longResult, shortResult *domain.ExchangeOrder

	g.Go(func() error {
		longBaseQty := p.LongFilledBaseQty
		if longBaseQty.LessThanOrEqual(decimal.Zero) {
			longBaseQty = contractsToBaseQty(p.LongFilledQty, p.LongContractSize)
		}
		filled, err := t.closeAndPoll(
			gCtx,
			longExch,
			p.Symbol,
			domain.OrderSideSell,
			p.LongFilledQty,
			longBaseQty,
			p.LongContractSize,
			longCloseOrder,
		)
		if err != nil {
			// Position was already closed on the exchange (e.g. manually closed
			// while the app was down). Treat as a successful no-op close.
			if isPositionAlreadyClosed(err) {
				t.logger.Info("long leg already closed on exchange",
					"position_id", p.ID, "exchange", p.LongExchangeID)
				longCloseOrder.Status = "closed"
				_ = t.store.UpdateOrder(ctx, longCloseOrder)
				longResult = &domain.ExchangeOrder{}
				return nil
			}
			return fmt.Errorf("close long leg: %w", err)
		}
		longResult = filled
		return nil
	})

	g.Go(func() error {
		shortBaseQty := p.ShortFilledBaseQty
		if shortBaseQty.LessThanOrEqual(decimal.Zero) {
			shortBaseQty = contractsToBaseQty(p.ShortFilledQty, p.ShortContractSize)
		}
		filled, err := t.closeAndPoll(
			gCtx,
			shortExch,
			p.Symbol,
			domain.OrderSideBuy,
			p.ShortFilledQty,
			shortBaseQty,
			p.ShortContractSize,
			shortCloseOrder,
		)
		if err != nil {
			if isPositionAlreadyClosed(err) {
				t.logger.Info("short leg already closed on exchange",
					"position_id", p.ID, "exchange", p.ShortExchangeID)
				shortCloseOrder.Status = "closed"
				_ = t.store.UpdateOrder(ctx, shortCloseOrder)
				shortResult = &domain.ExchangeOrder{}
				return nil
			}
			return fmt.Errorf("close short leg: %w", err)
		}
		shortResult = filled
		return nil
	})

	if err := g.Wait(); err != nil {
		return t.failPosition(ctx, p, err)
	}

	// Update position with exit details.
	now := time.Now().UTC()
	p.Status = domain.PositionStatusClosed
	p.ClosedAt = &now
	p.CloseReason = domain.CloseReasonNormal
	p.LongExitPrice = longResult.AveragePrice
	p.LongCloseFees = longResult.FeeCost
	p.ShortExitPrice = shortResult.AveragePrice
	p.ShortCloseFees = shortResult.FeeCost

	// Compute final funding P&L.
	fundingPnL, err := t.store.SumFundingPnL(ctx, p.UserID, p.ID)
	if err != nil {
		t.logger.Warn("sum funding pnl on close", "error", err)
	} else {
		p.TotalFundingPnL = fundingPnL
	}

	if err := t.store.UpdatePosition(ctx, p); err != nil {
		return fmt.Errorf("update position to closed: %w", err)
	}

	t.logger.Info("position closed",
		"position_id", p.ID,
		"symbol", p.Symbol,
		"long_exit_price", p.LongExitPrice,
		"short_exit_price", p.ShortExitPrice,
		"total_funding_pnl", p.TotalFundingPnL,
	)

	t.redis.Publish(ctx, rediskey.UserPositionUpdates(p.UserID), p.ID)

	return nil
}

// configureExchange sets margin mode (with optional leverage) on an exchange for a symbol.
// Deprecated: use configureExchangeWithLeverage for per-position leverage support.
func (t *Trader) configureExchange(ctx context.Context, exch base.PrivateExchange, symbol string) error {
	return t.configureExchangeWithLeverage(ctx, exch, symbol, t.cfg.Leverage)
}

// resolveLeverage returns the per-position leverage if set, otherwise falls back
// to the global TraderConfig.Leverage.
func (t *Trader) resolveLeverage(p *domain.Position) int {
	if p.Leverage > 0 {
		return p.Leverage
	}
	return t.cfg.Leverage
}

// placeAndPoll places a market order and polls until it reaches a terminal state.
// It updates the order record in PG as the order progresses.
func (t *Trader) placeAndPoll(
	ctx context.Context,
	exch base.PrivateExchange,
	symbol string,
	side domain.OrderSide,
	contracts decimal.Decimal,
	requestedBaseQty decimal.Decimal,
	contractSize decimal.Decimal,
	orderRec *domain.ExchangeOrder,
) (*domain.ExchangeOrder, error) {
	amount := contracts.InexactFloat64()
	logger := t.logger.With(
		"exchange", exch.ID(),
		"symbol", symbol,
		"side", side,
		"amount", amount,
	)

	logger.Info("placing market order")
	result, err := exch.CreateMarketOrder(ctx, symbol, side, amount)
	if err != nil {
		orderRec.Status = "failed"
		_ = t.store.UpdateOrder(ctx, orderRec)
		return nil, fmt.Errorf("create market order: %w", err)
	}

	return t.pollOrder(ctx, exch, symbol, result, orderRec, logger, contracts, requestedBaseQty, contractSize)
}

// closeAndPoll places a reduce-only market order and polls until it reaches a
// terminal state. It updates the order record in PG as the order progresses.
func (t *Trader) closeAndPoll(
	ctx context.Context,
	exch base.PrivateExchange,
	symbol string,
	side domain.OrderSide,
	contracts decimal.Decimal,
	requestedBaseQty decimal.Decimal,
	contractSize decimal.Decimal,
	orderRec *domain.ExchangeOrder,
) (*domain.ExchangeOrder, error) {
	amount := contracts.InexactFloat64()
	logger := t.logger.With(
		"exchange", exch.ID(),
		"symbol", symbol,
		"side", side,
		"amount", amount,
	)

	logger.Info("placing close market order")
	result, err := exch.CloseMarketOrder(ctx, symbol, side, amount)
	if err != nil {
		orderRec.Status = "failed"
		_ = t.store.UpdateOrder(ctx, orderRec)
		return nil, fmt.Errorf("close market order: %w", err)
	}

	return t.pollOrder(ctx, exch, symbol, result, orderRec, logger, contracts, requestedBaseQty, contractSize)
}

// pollOrder polls an exchange order until it reaches a terminal state.
func (t *Trader) pollOrder(
	ctx context.Context,
	exch base.PrivateExchange,
	symbol string,
	result *domain.ExchangeOrder,
	orderRec *domain.ExchangeOrder,
	logger *slog.Logger,
	contracts decimal.Decimal,
	requestedBaseQty decimal.Decimal,
	contractSize decimal.Decimal,
) (*domain.ExchangeOrder, error) {
	// Update order record with exchange ID.
	orderRec.ExchangeOrderID = result.ExchangeOrderID
	orderRec.Status = "submitted"
	orderRec.RequestedQty = contracts
	orderRec.RequestedBaseQty = requestedBaseQty
	orderRec.ContractSize = contractSize
	_ = t.store.UpdateOrder(ctx, orderRec)

	// Poll order status until terminal.
	const (
		pollInterval = 500 * time.Millisecond
		maxPolls     = 60 // 30 seconds max
	)

	for i := range maxPolls {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		fetched, err := exch.FetchOrder(ctx, result.ExchangeOrderID, symbol)
		if err != nil {
			logger.Warn("poll order failed, retrying", "attempt", i+1, "error", err)
			continue
		}

		logger.Debug("order status", "status", fetched.Status, "filled", fetched.FilledQty)

		// Update order record.
		orderRec.FilledQty = fetched.FilledQty
		orderRec.FilledBaseQty = contractsToBaseQty(fetched.FilledQty, contractSize)
		orderRec.AveragePrice = fetched.AveragePrice
		orderRec.Cost = fetched.Cost
		orderRec.FeeCost = fetched.FeeCost
		orderRec.Status = fetched.Status
		orderRec.RawResponse = fetched.RawResponse
		_ = t.store.UpdateOrder(ctx, orderRec)

		switch fetched.Status {
		case "closed": // CCXT uses "closed" for filled
			logger.Info("order filled",
				"filled_qty", fetched.FilledQty,
				"avg_price", fetched.AveragePrice,
				"fee", fetched.FeeCost,
			)
			return fetched, nil
		case "canceled", "cancelled", "expired", "rejected":
			return nil, fmt.Errorf("order %s on %s: %s", fetched.ExchangeOrderID, exch.ID(), fetched.Status)
		}
		// "open" — keep polling.
	}

	return nil, fmt.Errorf("order %s on %s did not fill within timeout", result.ExchangeOrderID, exch.ID())
}

// attemptRollback tries to close any leg that was successfully opened when the
// other leg failed. This is a best-effort operation.
//
// When one leg fails before even placing an order, the other leg's goroutine may
// have already submitted and filled its order. We always attempt to roll back any
// leg that has a filled result. Additionally, if a result is nil (no fill info
// captured — e.g. the goroutine was interrupted or the order was placed but polling
// didn't complete), we fall back to FetchPosition on the exchange to detect orphaned
// positions that need to be rolled back.
func (t *Trader) attemptRollback(
	ctx context.Context,
	p *domain.Position,
	longExch, shortExch base.PrivateExchange,
	longResult, shortResult *domain.ExchangeOrder,
) {
	longFilled := longResult != nil && longResult.Status == "closed"
	shortFilled := shortResult != nil && shortResult.Status == "closed"

	// If long leg filled but short didn't, close the long.
	if longFilled && !shortFilled {
		t.logger.Warn("rolling back long leg", "position_id", p.ID)
		t.rollbackOrder(ctx, longExch, p.Symbol, domain.OrderSideSell, longResult.FilledQty.InexactFloat64(), p.ID)
		return
	}

	// If short leg filled but long didn't, close the short.
	if shortFilled && !longFilled {
		t.logger.Warn("rolling back short leg", "position_id", p.ID)
		t.rollbackOrder(ctx, shortExch, p.Symbol, domain.OrderSideBuy, shortResult.FilledQty.InexactFloat64(), p.ID)
		return
	}

	// Neither result captured a fill. This can happen when one leg failed before
	// placing an order (so result is nil) while the other leg placed an order but
	// returned an error during polling (result is also nil, but order may be filled
	// on the exchange). Fall back to FetchPosition to detect orphaned legs.
	if !longFilled && !shortFilled {
		t.rollbackOrphanedLegs(ctx, p, longExch, shortExch)
	}
}

// rollbackOrder places a reduce-only close order to unwind a single leg.
func (t *Trader) rollbackOrder(ctx context.Context, exch base.PrivateExchange, symbol string, side domain.OrderSide, qty float64, positionID string) {
	if qty <= 0 {
		return
	}
	if _, err := exch.CloseMarketOrder(ctx, symbol, side, qty); err != nil {
		if !isPositionAlreadyClosed(err) {
			t.logger.Error("rollback order failed", "position_id", positionID, "exchange", exch.ID(), "error", err)
		}
	} else {
		t.logger.Info("rollback order succeeded", "position_id", positionID, "exchange", exch.ID())
	}
}

// rollbackOrphanedLegs is the fallback when neither goroutine captured a fill
// result. It fetches the actual exchange state via REST and closes any leg
// that has an open position.
func (t *Trader) rollbackOrphanedLegs(
	ctx context.Context,
	p *domain.Position,
	longExch, shortExch base.PrivateExchange,
) {
	t.logger.Warn("no fill results captured, checking exchanges for orphaned positions",
		"position_id", p.ID)

	longPos, longErr := longExch.FetchPosition(ctx, p.Symbol)
	if longErr != nil {
		t.logger.Error("rollback: fetch long position failed",
			"position_id", p.ID, "exchange", p.LongExchangeID, "error", longErr)
	}

	shortPos, shortErr := shortExch.FetchPosition(ctx, p.Symbol)
	if shortErr != nil {
		t.logger.Error("rollback: fetch short position failed",
			"position_id", p.ID, "exchange", p.ShortExchangeID, "error", shortErr)
	}

	longHas := longPos != nil && longPos.Contracts.GreaterThan(decimal.Zero)
	shortHas := shortPos != nil && shortPos.Contracts.GreaterThan(decimal.Zero)

	if longHas && !shortHas {
		t.logger.Warn("rollback: orphaned long leg found, closing",
			"position_id", p.ID, "contracts", longPos.Contracts)
		t.rollbackOrder(ctx, longExch, p.Symbol, domain.OrderSideSell, longPos.Contracts.InexactFloat64(), p.ID)
	}

	if shortHas && !longHas {
		t.logger.Warn("rollback: orphaned short leg found, closing",
			"position_id", p.ID, "contracts", shortPos.Contracts)
		t.rollbackOrder(ctx, shortExch, p.Symbol, domain.OrderSideBuy, shortPos.Contracts.InexactFloat64(), p.ID)
	}

	if longHas && shortHas {
		// Both legs are open — this shouldn't happen in a partial failure,
		// but log it for visibility. Don't roll back both — let reconciliation
		// handle it on next startup.
		t.logger.Error("rollback: both legs have open positions, skipping rollback",
			"position_id", p.ID,
			"long_contracts", longPos.Contracts,
			"short_contracts", shortPos.Contracts,
		)
	}
}

// buildOpenError combines long and short leg errors into a single error.
func buildOpenError(longErr, shortErr error) error {
	switch {
	case longErr != nil && shortErr != nil:
		return fmt.Errorf("both legs failed: long: %w; short: %v", longErr, shortErr)
	case longErr != nil:
		return fmt.Errorf("long leg: %w", longErr)
	default:
		return fmt.Errorf("short leg: %w", shortErr)
	}
}

// failPosition marks a position as failed with the given error message.
func (t *Trader) failPosition(ctx context.Context, p *domain.Position, cause error) error {
	p.Status = domain.PositionStatusFailed
	p.ErrorMessage = cause.Error()
	if err := t.store.UpdatePosition(ctx, p); err != nil {
		t.logger.Error("failed to update position to failed state",
			"position_id", p.ID, "error", err)
	}
	t.redis.Publish(ctx, rediskey.UserPositionUpdates(p.UserID), p.ID)
	return cause
}
