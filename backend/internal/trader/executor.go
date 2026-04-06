package trader

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// runExecutor subscribes to Redis Pub/Sub for position open and close signals.
// When a position ID arrives on the pending channel, it executes the open flow.
// When a position ID arrives on the closing channel, it executes the close flow.
func (t *Trader) runExecutor(ctx context.Context) error {
	pubsub := t.redis.Subscribe(ctx,
		rediskey.PositionPending(),
		rediskey.PositionClosing(),
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

			switch msg.Channel {
			case rediskey.PositionPending():
				t.logger.Info("received open signal", "position_id", positionID)
				if err := t.executeOpen(ctx, positionID); err != nil {
					t.logger.Error("execute open failed", "position_id", positionID, "error", err)
				}

			case rediskey.PositionClosing():
				t.logger.Info("received close signal", "position_id", positionID)
				if err := t.executeClose(ctx, positionID); err != nil {
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
func (t *Trader) executeOpen(ctx context.Context, positionID string) error {
	p, err := t.store.GetPosition(ctx, positionID)
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

	longExch, err := t.registry.Get(p.LongExchangeID)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("long exchange: %w", err))
	}
	shortExch, err := t.registry.Get(p.ShortExchangeID)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("short exchange: %w", err))
	}

	// Configure leverage and margin mode on both exchanges.
	if err := t.configureExchange(ctx, longExch, p.Symbol); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("configure long exchange: %w", err))
	}
	if err := t.configureExchange(ctx, shortExch, p.Symbol); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("configure short exchange: %w", err))
	}

	// Determine order amount in base currency.
	// For now, if SizeType is USD, we need to convert using a reference price.
	// We'll use the mark price from the most recent ticker data.
	amount, err := t.resolveAmount(ctx, p, longExch, shortExch)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("resolve amount: %w", err))
	}

	// Create order records in PG before submitting to exchanges.
	longOrder := &domain.ExchangeOrder{
		PositionID: p.ID,
		ExchangeID: p.LongExchangeID,
		Symbol:     p.Symbol,
		Side:       domain.OrderSideBuy,
		Leg:        "long",
		Action:     "open",
		Status:     "pending",
	}
	shortOrder := &domain.ExchangeOrder{
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
	type orderResult struct {
		order *domain.ExchangeOrder
		err   error
	}

	g, gCtx := errgroup.WithContext(ctx)
	var longResult, shortResult *domain.ExchangeOrder

	g.Go(func() error {
		filled, err := t.placeAndPoll(gCtx, longExch, p.Symbol, domain.OrderSideBuy, amount, longOrder)
		if err != nil {
			return fmt.Errorf("long leg: %w", err)
		}
		longResult = filled
		return nil
	})

	g.Go(func() error {
		filled, err := t.placeAndPoll(gCtx, shortExch, p.Symbol, domain.OrderSideSell, amount, shortOrder)
		if err != nil {
			return fmt.Errorf("short leg: %w", err)
		}
		shortResult = filled
		return nil
	})

	if err := g.Wait(); err != nil {
		// Partial failure — at least one leg failed.
		t.logger.Error("order execution failed, attempting rollback", "error", err)
		t.attemptRollback(ctx, p, longExch, shortExch, longResult, shortResult)
		return t.failPosition(ctx, p, err)
	}

	// Both legs filled — update position with fill details.
	now := time.Now().UTC()
	p.Status = domain.PositionStatusOpen
	p.OpenedAt = &now
	p.LongEntryPrice = longResult.AveragePrice
	p.LongFilledQty = longResult.FilledQty
	p.LongEntryCost = longResult.Cost
	p.LongEntryFees = longResult.FeeCost
	p.ShortEntryPrice = shortResult.AveragePrice
	p.ShortFilledQty = shortResult.FilledQty
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
	t.redis.Publish(ctx, rediskey.PositionUpdates(), p.ID)

	return nil
}

// executeClose handles the full close flow for a position:
//  1. Fetch position and validate status
//  2. Place close orders on both legs concurrently
//  3. Update position with exit details and transition to "closed"
func (t *Trader) executeClose(ctx context.Context, positionID string) error {
	p, err := t.store.GetPosition(ctx, positionID)
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

	longExch, err := t.registry.Get(p.LongExchangeID)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("long exchange: %w", err))
	}
	shortExch, err := t.registry.Get(p.ShortExchangeID)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("short exchange: %w", err))
	}

	// Create close order records.
	longCloseOrder := &domain.ExchangeOrder{
		PositionID: p.ID,
		ExchangeID: p.LongExchangeID,
		Symbol:     p.Symbol,
		Side:       domain.OrderSideSell, // close long by selling
		Leg:        "long",
		Action:     "close",
		Status:     "pending",
	}
	shortCloseOrder := &domain.ExchangeOrder{
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
		qty := p.LongFilledQty.InexactFloat64()
		filled, err := t.closeAndPoll(gCtx, longExch, p.Symbol, domain.OrderSideSell, qty, longCloseOrder)
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
		qty := p.ShortFilledQty.InexactFloat64()
		filled, err := t.closeAndPoll(gCtx, shortExch, p.Symbol, domain.OrderSideBuy, qty, shortCloseOrder)
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
	fundingPnL, err := t.store.SumFundingPnL(ctx, p.ID)
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

	t.redis.Publish(ctx, rediskey.PositionUpdates(), p.ID)

	return nil
}

// configureExchange sets margin mode (with optional leverage) on an exchange for a symbol.
func (t *Trader) configureExchange(ctx context.Context, exch exchange.TradingExchange, symbol string) error {
	if t.cfg.MarginMode != "" {
		if err := exch.SetMarginMode(ctx, symbol, t.cfg.MarginMode, t.cfg.Leverage); err != nil {
			return fmt.Errorf("set margin mode: %w", err)
		}
	}
	return nil
}

// resolveAmount determines the order amount in base currency.
// For SizeTypeBase, it returns the requested size directly.
// For SizeTypeUSD, it fetches positions/tickers to determine a reference price.
func (t *Trader) resolveAmount(ctx context.Context, p *domain.Position, longExch, shortExch exchange.TradingExchange) (float64, error) {
	if p.SizeType == domain.SizeTypeBase {
		return p.RequestedSize.InexactFloat64(), nil
	}

	// SizeTypeUSD — need a reference price to convert to base.
	// Try fetching current position/mark price from the long exchange.
	// If not available, use short exchange.
	var refPrice decimal.Decimal

	longPos, err := longExch.FetchPosition(ctx, p.Symbol)
	if err == nil && longPos != nil && !longPos.MarkPrice.IsZero() {
		refPrice = longPos.MarkPrice
	}

	if refPrice.IsZero() {
		shortPos, err := shortExch.FetchPosition(ctx, p.Symbol)
		if err == nil && shortPos != nil && !shortPos.MarkPrice.IsZero() {
			refPrice = shortPos.MarkPrice
		}
	}

	if refPrice.IsZero() {
		return 0, fmt.Errorf("unable to determine reference price for %s", p.Symbol)
	}

	// amount = requested_usd / price
	amount := p.RequestedSize.Div(refPrice)
	return amount.InexactFloat64(), nil
}

// placeAndPoll places a market order and polls until it reaches a terminal state.
// It updates the order record in PG as the order progresses.
func (t *Trader) placeAndPoll(
	ctx context.Context,
	exch exchange.TradingExchange,
	symbol string,
	side domain.OrderSide,
	amount float64,
	orderRec *domain.ExchangeOrder,
) (*domain.ExchangeOrder, error) {
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

	return t.pollOrder(ctx, exch, symbol, result, orderRec, logger, amount)
}

// closeAndPoll places a reduce-only market order and polls until it reaches a
// terminal state. It updates the order record in PG as the order progresses.
func (t *Trader) closeAndPoll(
	ctx context.Context,
	exch exchange.TradingExchange,
	symbol string,
	side domain.OrderSide,
	amount float64,
	orderRec *domain.ExchangeOrder,
) (*domain.ExchangeOrder, error) {
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

	return t.pollOrder(ctx, exch, symbol, result, orderRec, logger, amount)
}

// pollOrder polls an exchange order until it reaches a terminal state.
func (t *Trader) pollOrder(
	ctx context.Context,
	exch exchange.TradingExchange,
	symbol string,
	result *domain.ExchangeOrder,
	orderRec *domain.ExchangeOrder,
	logger *slog.Logger,
	amount float64,
) (*domain.ExchangeOrder, error) {
	// Update order record with exchange ID.
	orderRec.ExchangeOrderID = result.ExchangeOrderID
	orderRec.Status = "submitted"
	orderRec.RequestedQty = decimal.NewFromFloat(amount)
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
func (t *Trader) attemptRollback(
	ctx context.Context,
	p *domain.Position,
	longExch, shortExch exchange.TradingExchange,
	longResult, shortResult *domain.ExchangeOrder,
) {
	// If long leg filled but short didn't, close the long.
	if longResult != nil && longResult.Status == "closed" && (shortResult == nil || shortResult.Status != "closed") {
		t.logger.Warn("rolling back long leg", "position_id", p.ID)
		qty := longResult.FilledQty.InexactFloat64()
		if qty > 0 {
			if _, err := longExch.CloseMarketOrder(ctx, p.Symbol, domain.OrderSideSell, qty); err != nil {
				t.logger.Error("rollback long leg failed", "position_id", p.ID, "error", err)
			} else {
				t.logger.Info("rollback long leg succeeded", "position_id", p.ID)
			}
		}
	}

	// If short leg filled but long didn't, close the short.
	if shortResult != nil && shortResult.Status == "closed" && (longResult == nil || longResult.Status != "closed") {
		t.logger.Warn("rolling back short leg", "position_id", p.ID)
		qty := shortResult.FilledQty.InexactFloat64()
		if qty > 0 {
			if _, err := shortExch.CloseMarketOrder(ctx, p.Symbol, domain.OrderSideBuy, qty); err != nil {
				t.logger.Error("rollback short leg failed", "position_id", p.ID, "error", err)
			} else {
				t.logger.Info("rollback short leg succeeded", "position_id", p.ID)
			}
		}
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
	t.redis.Publish(ctx, rediskey.PositionUpdates(), p.ID)
	return cause
}
