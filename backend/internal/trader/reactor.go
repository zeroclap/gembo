package trader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// runPositionReactor subscribes to exchange position state events published by
// the collector via Redis pubsub, updates the position in PG with the latest
// exchange state, and reacts to closures (auto-close remaining leg) and partial
// closes (rebalance the other leg).
func (t *Trader) runPositionReactor(ctx context.Context) error {
	// Build channel list: subscribe to position events from all registered exchanges.
	channels := make([]string, 0, len(t.registry.IDs()))
	for _, id := range t.registry.IDs() {
		channels = append(channels, rediskey.PositionStatePubSub(id))
	}

	if len(channels) == 0 {
		t.logger.Warn("reactor: no exchanges registered, position reactor idle")
		<-ctx.Done()
		return ctx.Err()
	}

	pubsub := t.redis.Subscribe(ctx, channels...)
	defer pubsub.Close()

	ch := pubsub.Channel()

	t.logger.Info("position reactor started", "channels", channels)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			var event domain.ExchangePositionEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				t.logger.Warn("reactor: unmarshal position event", "error", err)
				continue
			}

			if err := t.handlePositionEvent(ctx, &event); err != nil {
				t.logger.Error("reactor: handle position event",
					"exchange", event.ExchangeID,
					"symbol", event.Symbol,
					"side", event.Side,
					"error", err,
				)
			}
		}
	}
}

// handlePositionEvent processes a single position state update from an exchange.
// It finds the matching Gembo position, updates state, and reacts to closures.
//
// Reactions (rebalance / auto-close) run synchronously so that the DB is fully
// updated before the next event is processed. This prevents recursive cascading:
// our own rebalance order triggers a counter-exchange position event, but by the
// time the reactor processes it the FilledQty in PG already reflects the
// rebalance, so detectAction returns reactorActionNone.
// Events that arrive during the reaction simply buffer in the Redis pubsub
// channel and are drained afterward.
func (t *Trader) handlePositionEvent(ctx context.Context, event *domain.ExchangePositionEvent) error {
	// Find open positions that match this exchange + symbol.
	positions, err := t.store.ListPositions(ctx, domain.PositionFilter{
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
		Symbol: event.Symbol,
	})
	if err != nil {
		return fmt.Errorf("list positions: %w", err)
	}

	for i := range positions {
		p := &positions[i]

		// Determine which leg this event belongs to.
		leg := t.matchLeg(p, event)
		t.logger.Debug("reactor: position leg", "position_id", p.ID, "leg", leg)
		if leg == "" {
			continue
		}

		t.logger.Debug("reactor: position event matched",
			"position_id", p.ID,
			"exchange", event.ExchangeID,
			"symbol", event.Symbol,
			"leg", leg,
			"contracts", event.Contracts,
		)

		// Update the position state from the event.
		t.applyPositionEvent(p, event, leg)

		// Detect whether the event represents a closure or partial close.
		action := t.detectAction(p, event, leg)

		// Persist updated state.
		if err := t.store.UpdatePosition(ctx, p); err != nil {
			t.logger.Error("reactor: update position",
				"position_id", p.ID, "error", err)
			continue
		}

		// React synchronously — the reactor loop blocks until the reaction
		// completes and the DB is up to date. This guarantees the next event
		// reads the correct FilledQty, eliminating recursive cascades.
		switch action {
		case reactorActionAutoClose:
			t.logger.Warn("reactor: leg closed externally, auto-closing remaining leg",
				"position_id", p.ID,
				"closed_leg", leg,
				"symbol", p.Symbol,
			)
			t.autoCloseRemainingLeg(context.Background(), p, leg)

		case reactorActionRebalance:
			t.logger.Warn("reactor: leg partially closed, rebalancing other leg",
				"position_id", p.ID,
				"changed_leg", leg,
				"symbol", p.Symbol,
				"remaining_contracts", event.Contracts,
			)
			t.rebalanceOtherLeg(context.Background(), p, leg, event.Contracts)
		}
	}

	return nil
}

// reactorAction represents what the reactor should do after processing an event.
type reactorAction int

const (
	reactorActionNone      reactorAction = iota
	reactorActionAutoClose               // one leg fully closed -> close other
	reactorActionRebalance               // one leg partially reduced -> reduce other
)

// matchLeg returns "long" or "short" if this event matches a leg of the position,
// or "" if no match.
//
// Matching is based solely on exchange ID. In the funding rate arbitrage system
// each exchange holds exactly one side — the exchange ID uniquely identifies the
// leg. This avoids issues with exchanges that report an empty Side ("") when a
// position is fully closed, where there is no longer a direction to report.
func (t *Trader) matchLeg(p *domain.Position, event *domain.ExchangePositionEvent) string {
	if p.LongExchangeID == event.ExchangeID {
		return "long"
	}
	if p.ShortExchangeID == event.ExchangeID {
		return "short"
	}
	return ""
}

// applyPositionEvent updates the position's live state fields from the event.
func (t *Trader) applyPositionEvent(p *domain.Position, event *domain.ExchangePositionEvent, leg string) {
	switch leg {
	case "long":
		p.LongMarkPrice = event.MarkPrice
		p.LongUnrealizedPnL = event.UnrealizedPnL
		p.LongLiquidationPrice = event.LiquidationPrice
		p.LongLeverage = event.Leverage
		p.LongMarginMode = event.MarginMode
		p.LongNotional = event.Notional
	case "short":
		p.ShortMarkPrice = event.MarkPrice
		p.ShortUnrealizedPnL = event.UnrealizedPnL
		p.ShortLiquidationPrice = event.LiquidationPrice
		p.ShortLeverage = event.Leverage
		p.ShortMarginMode = event.MarginMode
		p.ShortNotional = event.Notional
	}
}

// detectAction compares the event's contract count with the stored filled qty
// to detect closures or partial closes.
func (t *Trader) detectAction(p *domain.Position, event *domain.ExchangePositionEvent, leg string) reactorAction {
	var filledQty decimal.Decimal
	switch leg {
	case "long":
		filledQty = p.LongFilledQty
	case "short":
		filledQty = p.ShortFilledQty
	}

	t.logger.Debug("-- detectAction --",
		"position_id", p.ID,
		"leg", leg,
		"filled_qty", filledQty,
		"event_contracts", event.Contracts,
	)

	if filledQty.IsZero() {
		// No filled qty recorded yet — position may still be opening.
		return reactorActionNone
	}

	// Full closure: contracts dropped to zero.
	if event.Contracts.IsZero() {
		return reactorActionAutoClose
	}

	// Partial close: contracts reduced but not zero.
	// Use a small tolerance to avoid floating-point noise triggering rebalances.
	threshold := filledQty.Mul(decimal.NewFromFloat(0.01)) // 1% tolerance
	diff := filledQty.Sub(event.Contracts)
	if diff.GreaterThan(threshold) {
		return reactorActionRebalance
	}

	t.logger.Debug("--- No Action! ---")
	return reactorActionNone
}

// autoCloseRemainingLeg closes the other leg of a position when one leg was
// closed externally (manually on exchange UI, or liquidated). It places a market
// order to close the remaining leg and updates the position to closed.
func (t *Trader) autoCloseRemainingLeg(ctx context.Context, p *domain.Position, closedLeg string) {
	logger := t.logger.With(
		slog.String("position_id", p.ID),
		slog.String("symbol", p.Symbol),
		slog.String("closed_leg", closedLeg),
	)

	// Determine the remaining leg.
	var (
		remainExchangeID string
		remainSide       domain.OrderSide // Buy to close short, Sell to close long
		remainQty        decimal.Decimal
	)

	switch closedLeg {
	case "long":
		// Long was closed externally. Close the short by buying.
		remainExchangeID = p.ShortExchangeID
		remainSide = domain.OrderSideBuy
		remainQty = p.ShortFilledQty
	case "short":
		// Short was closed externally. Close the long by selling.
		remainExchangeID = p.LongExchangeID
		remainSide = domain.OrderSideSell
		remainQty = p.LongFilledQty
	default:
		logger.Error("reactor: unknown closed leg")
		return
	}

	if remainQty.IsZero() {
		// Both legs are already closed — just update status.
		t.closePosition(ctx, p, domain.CloseReasonAutoClose, logger)
		return
	}

	exch, err := t.registry.Get(remainExchangeID)
	if err != nil {
		logger.Error("reactor: get remaining exchange", "error", err)
		t.closePosition(ctx, p, domain.CloseReasonAutoClose, logger)
		return
	}

	// Place market order to close remaining leg.
	amount := remainQty.InexactFloat64()
	logger.Info("reactor: placing auto-close order",
		"exchange", remainExchangeID,
		"side", remainSide,
		"amount", amount,
	)

	order, err := exch.CloseMarketOrder(ctx, p.Symbol, remainSide, amount)
	if err != nil {
		// If the reduce-only order was rejected because the position is already
		// closed on the exchange, this is expected (e.g. collector was down when
		// it happened). Log at info level and close cleanly without an error.
		if isPositionAlreadyClosed(err) {
			logger.Info("reactor: remaining leg already closed on exchange, nothing to reduce",
				"exchange", remainExchangeID,
			)
			t.closePosition(ctx, p, domain.CloseReasonAutoClose, logger)
			return
		}

		logger.Error("reactor: auto-close order failed", "error", err)
		// Still mark position as closed with error.
		p.ErrorMessage = fmt.Sprintf("auto-close %s leg failed: %s", closedLeg, err)
		t.closePosition(ctx, p, domain.CloseReasonAutoClose, logger)
		return
	}

	// Record the close order.
	orderRec := &domain.ExchangeOrder{
		PositionID:      p.ID,
		ExchangeID:      remainExchangeID,
		ExchangeOrderID: order.ExchangeOrderID,
		Symbol:          p.Symbol,
		Side:            remainSide,
		Leg:             otherLeg(closedLeg),
		Action:          "close",
		RequestedQty:    remainQty,
		FilledQty:       order.FilledQty,
		AveragePrice:    order.AveragePrice,
		Cost:            order.Cost,
		FeeCost:         order.FeeCost,
		Status:          order.Status,
		RawResponse:     order.RawResponse,
	}
	if err := t.store.CreateOrder(ctx, orderRec); err != nil {
		logger.Warn("reactor: create auto-close order record", "error", err)
	}

	// Update exit prices.
	switch closedLeg {
	case "long":
		p.ShortExitPrice = order.AveragePrice
		p.ShortCloseFees = order.FeeCost
	case "short":
		p.LongExitPrice = order.AveragePrice
		p.LongCloseFees = order.FeeCost
	}

	logger.Info("reactor: auto-close order placed",
		"order_id", order.ExchangeOrderID,
		"avg_price", order.AveragePrice,
		"filled_qty", order.FilledQty,
	)

	t.closePosition(ctx, p, domain.CloseReasonAutoClose, logger)
}

// rebalanceOtherLeg reduces the other leg to match a partial close on one leg.
func (t *Trader) rebalanceOtherLeg(ctx context.Context, p *domain.Position, changedLeg string, remainingContracts decimal.Decimal) {
	logger := t.logger.With(
		slog.String("position_id", p.ID),
		slog.String("symbol", p.Symbol),
		slog.String("changed_leg", changedLeg),
	)

	// Determine how much to reduce the other leg.
	var (
		otherExchangeID string
		otherSide       domain.OrderSide // Buy to reduce short, Sell to reduce long
		otherFilledQty  decimal.Decimal
	)

	switch changedLeg {
	case "long":
		// Long was partially reduced. Reduce short to match.
		otherExchangeID = p.ShortExchangeID
		otherSide = domain.OrderSideBuy
		otherFilledQty = p.ShortFilledQty
	case "short":
		// Short was partially reduced. Reduce long to match.
		otherExchangeID = p.LongExchangeID
		otherSide = domain.OrderSideSell
		otherFilledQty = p.LongFilledQty
	default:
		logger.Error("reactor: unknown changed leg")
		return
	}

	// Calculate the reduction amount.
	// changed leg: was filledQty, now remainingContracts
	// reduce other leg by the same delta.
	var originalQty decimal.Decimal
	switch changedLeg {
	case "long":
		originalQty = p.LongFilledQty
	case "short":
		originalQty = p.ShortFilledQty
	}

	delta := originalQty.Sub(remainingContracts)
	if delta.LessThanOrEqual(decimal.Zero) {
		logger.Debug("reactor: no reduction needed, delta <= 0")
		return
	}

	// Ensure we don't try to reduce more than what's open on the other side.
	if delta.GreaterThan(otherFilledQty) {
		delta = otherFilledQty
	}

	exch, err := t.registry.Get(otherExchangeID)
	if err != nil {
		logger.Error("reactor: get other exchange for rebalance", "error", err)
		return
	}

	amount := delta.InexactFloat64()
	logger.Info("reactor: placing rebalance order",
		"exchange", otherExchangeID,
		"side", otherSide,
		"amount", amount,
		"delta", delta,
	)

	order, err := exch.CloseMarketOrder(ctx, p.Symbol, otherSide, amount)
	if err != nil {
		// If the position is already closed on this exchange, the reduce-only
		// order is correctly rejected. Log at info and skip.
		if isPositionAlreadyClosed(err) {
			logger.Info("reactor: other leg already closed on exchange, skipping rebalance",
				"exchange", otherExchangeID,
			)
			return
		}

		logger.Error("reactor: rebalance order failed", "error", err)
		return
	}

	// Record the rebalance order.
	orderRec := &domain.ExchangeOrder{
		PositionID:      p.ID,
		ExchangeID:      otherExchangeID,
		ExchangeOrderID: order.ExchangeOrderID,
		Symbol:          p.Symbol,
		Side:            otherSide,
		Leg:             otherLeg(changedLeg),
		Action:          "close",
		RequestedQty:    delta,
		FilledQty:       order.FilledQty,
		AveragePrice:    order.AveragePrice,
		Cost:            order.Cost,
		FeeCost:         order.FeeCost,
		Status:          order.Status,
		RawResponse:     order.RawResponse,
	}
	if err := t.store.CreateOrder(ctx, orderRec); err != nil {
		logger.Warn("reactor: create rebalance order record", "error", err)
	}

	// Update filled quantities in the position.
	// The changed leg's qty comes from the event; the other leg is reduced by
	// delta (the intended reduction) rather than order.FilledQty.
	//
	// Why delta instead of order.FilledQty: CreateMarketOrder may return
	// FilledQty=0 when the exchange acknowledges the order but hasn't confirmed
	// the fill yet (CCXT's Filled field is nil → floatToDecimal returns zero).
	// Using order.FilledQty=0 would leave the DB thinking the other leg is
	// unchanged, so the next position event from the counter-exchange still
	// shows a diff → triggering another rebalance → infinite cascade.
	//
	// Using delta is safe even if the order partially fills: the next position
	// event will report the actual contract count and detectAction will
	// reconcile accordingly.
	switch changedLeg {
	case "long":
		p.LongFilledQty = remainingContracts
		p.ShortFilledQty = p.ShortFilledQty.Sub(delta)
	case "short":
		p.ShortFilledQty = remainingContracts
		p.LongFilledQty = p.LongFilledQty.Sub(delta)
	}

	p.Status = domain.PositionStatusPartiallyOpen
	if err := t.store.UpdatePosition(ctx, p); err != nil {
		logger.Error("reactor: update position after rebalance", "error", err)
		return
	}

	logger.Info("reactor: rebalance complete",
		"order_id", order.ExchangeOrderID,
		"filled_qty", order.FilledQty,
		"long_qty", p.LongFilledQty,
		"short_qty", p.ShortFilledQty,
	)

	t.redis.Publish(ctx, rediskey.PositionUpdates(), p.ID)
}

// closePosition marks a position as closed with the given reason and persists it.
func (t *Trader) closePosition(ctx context.Context, p *domain.Position, reason domain.CloseReason, logger *slog.Logger) {
	now := time.Now().UTC()
	p.Status = domain.PositionStatusClosed
	p.ClosedAt = &now
	p.CloseReason = reason

	// Compute final funding P&L.
	fundingPnL, err := t.store.SumFundingPnL(ctx, p.ID)
	if err != nil {
		logger.Warn("reactor: sum funding pnl on close", "error", err)
	} else {
		p.TotalFundingPnL = fundingPnL
	}

	if err := t.store.UpdatePosition(ctx, p); err != nil {
		logger.Error("reactor: update position to closed", "error", err)
		return
	}

	logger.Info("reactor: position closed",
		"close_reason", reason,
		"total_funding_pnl", p.TotalFundingPnL,
	)

	t.redis.Publish(ctx, rediskey.PositionUpdates(), p.ID)
}

// otherLeg returns the opposite leg name.
func otherLeg(leg string) string {
	if leg == "long" {
		return "short"
	}
	return "long"
}

// isPositionAlreadyClosed checks whether a CreateMarketOrder error indicates the
// position is already closed on the exchange (i.e. contracts are zero). This
// happens when a reduce-only order is rejected because there is nothing to
// reduce — an expected condition when the collector was down while a position
// was closed externally.
//
// Each exchange adapter detects the exchange-specific error variant (e.g.
// Bybit InvalidOrder, OKX ExchangeError 51169) and wraps it with
// exchange.ErrPositionAlreadyClosed, so the trader never needs to import CCXT.
func isPositionAlreadyClosed(err error) bool {
	return errors.Is(err, exchange.ErrPositionAlreadyClosed)
}
