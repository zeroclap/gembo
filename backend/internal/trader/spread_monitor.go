package trader

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/store"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// SpreadMonitor subscribes to ticker PubSub events and monitors open spread
// arbitrage positions for auto-close triggers: target convergence, stop-loss
// divergence, and max hold timeout.
type SpreadMonitor struct {
	trader *Trader
	store  store.Store // Redis store for reading tickers
	logger *slog.Logger
}

// newSpreadMonitor creates a new SpreadMonitor. Only used internally by the Trader.
func newSpreadMonitor(t *Trader, st store.Store) *SpreadMonitor {
	return &SpreadMonitor{
		trader: t,
		store:  st,
		logger: t.logger.With("component", "spread-monitor"),
	}
}

// run subscribes to ticker PubSub events and checks open spread positions on
// each update. It also runs a periodic ticker to check for timeouts.
func (m *SpreadMonitor) run(ctx context.Context) error {
	// Subscribe to ticker update channels from all registered exchanges.
	channels := make([]string, 0, len(m.trader.registry.IDs()))
	for _, id := range m.trader.registry.IDs() {
		channels = append(channels, rediskey.TickerPubSub(id))
	}

	if len(channels) == 0 {
		m.logger.Warn("spread monitor: no exchanges, idle")
		<-ctx.Done()
		return ctx.Err()
	}

	pubsub := m.trader.redis.Subscribe(ctx, channels...)
	defer pubsub.Close()

	ch := pubsub.Channel()

	// Debounce ticker events to avoid excessive spread checks.
	const debounceInterval = 2 * time.Second
	ticker := time.NewTicker(debounceInterval)
	defer ticker.Stop()

	// Periodic timeout check: every 30 seconds.
	const timeoutCheckInterval = 30 * time.Second
	timeoutTicker := time.NewTicker(timeoutCheckInterval)
	defer timeoutTicker.Stop()

	pending := false

	m.logger.Info("spread monitor started", "channels", channels)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case _, ok := <-ch:
			if !ok {
				return nil
			}
			pending = true

		case <-ticker.C:
			if !pending {
				continue
			}
			pending = false
			m.checkSpreadPositions(ctx, "")

		case <-timeoutTicker.C:
			m.checkTimeouts(ctx, "")
		}
	}
}

// checkSpreadPositions loads all open spread positions and evaluates their
// current spread against target and stop-loss thresholds.
func (m *SpreadMonitor) checkSpreadPositions(ctx context.Context, userID string) {
	positions, err := m.trader.store.ListPositions(ctx, domain.PositionFilter{
		UserID: userID,
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
	})
	if err != nil {
		m.logger.Warn("spread monitor: list positions", "error", err)
		return
	}

	for i := range positions {
		p := &positions[i]
		if p.ArbitrageType != domain.ArbitrageTypeSpread {
			continue
		}

		m.evaluateSpreadPosition(ctx, p)
	}
}

// evaluateSpreadPosition checks if a spread position should be auto-closed
// based on the current spread vs target/stop-loss.
func (m *SpreadMonitor) evaluateSpreadPosition(ctx context.Context, p *domain.Position) {
	// Read current tickers for both exchanges.
	longTicker, err := m.store.GetTicker(ctx, p.LongExchangeID, p.Symbol)
	if err != nil || longTicker == nil {
		return
	}
	shortTicker, err := m.store.GetTicker(ctx, p.ShortExchangeID, p.Symbol)
	if err != nil || shortTicker == nil {
		return
	}

	// Compute current spread: the spread position is buy on long exchange,
	// sell on short exchange. The "current spread" for monitoring is
	// (short_bid - long_ask) / midpoint * 100.
	if longTicker.Ask.IsZero() || shortTicker.Bid.IsZero() {
		return
	}

	midPrice := longTicker.Ask.Add(shortTicker.Bid).Div(decimal.NewFromInt(2))
	if midPrice.IsZero() {
		return
	}

	currentSpread := shortTicker.Bid.Sub(longTicker.Ask).Div(midPrice).Mul(decimal.NewFromInt(100))

	logger := m.logger.With(
		"position_id", p.ID,
		"symbol", p.Symbol,
		"current_spread", currentSpread,
		"target_spread", p.TargetSpreadPct,
		"stop_loss_spread", p.StopLossSpreadPct,
	)

	// Check target convergence: close if spread <= target.
	if !p.TargetSpreadPct.IsZero() && currentSpread.LessThanOrEqual(p.TargetSpreadPct) {
		logger.Info("spread monitor: target reached, closing position")
		m.triggerClose(ctx, p, domain.CloseReasonTargetReached)
		return
	}

	// Check stop-loss divergence: close if spread >= stop-loss.
	if !p.StopLossSpreadPct.IsZero() && currentSpread.GreaterThanOrEqual(p.StopLossSpreadPct) {
		logger.Warn("spread monitor: stop-loss hit, closing position")
		m.triggerClose(ctx, p, domain.CloseReasonStopLoss)
		return
	}
}

// checkTimeouts checks open spread positions for max hold duration expiry.
func (m *SpreadMonitor) checkTimeouts(ctx context.Context, userID string) {
	positions, err := m.trader.store.ListPositions(ctx, domain.PositionFilter{
		UserID: userID,
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
	})
	if err != nil {
		m.logger.Warn("spread monitor: list positions for timeout check", "error", err)
		return
	}

	now := time.Now().UTC()
	for i := range positions {
		p := &positions[i]
		if p.ArbitrageType != domain.ArbitrageTypeSpread {
			continue
		}
		if p.MaxHoldDuration <= 0 || p.OpenedAt == nil {
			continue
		}

		expiry := p.OpenedAt.Add(time.Duration(p.MaxHoldDuration) * time.Second)
		if now.After(expiry) {
			m.logger.Warn("spread monitor: timeout exceeded, closing position",
				"position_id", p.ID,
				"symbol", p.Symbol,
				"opened_at", p.OpenedAt,
				"max_hold_duration", p.MaxHoldDuration,
			)
			m.triggerClose(ctx, p, domain.CloseReasonTimeout)
		}
	}
}

// triggerClose initiates position closure by transitioning to closing status
// and signaling the executor via Redis PubSub.
func (m *SpreadMonitor) triggerClose(ctx context.Context, p *domain.Position, reason domain.CloseReason) {
	p.Status = domain.PositionStatusClosing
	p.CloseReason = reason

	if err := m.trader.store.UpdatePosition(ctx, p); err != nil {
		m.logger.Error("spread monitor: update position to closing",
			"position_id", p.ID, "error", err)
		return
	}

	// Signal the executor to close the position.
	m.trader.redis.Publish(ctx, rediskey.UserPositionClosing(p.UserID), p.ID)

	// Also publish a snapshot of the close trigger for debugging.
	payload, _ := json.Marshal(map[string]interface{}{
		"position_id":  p.ID,
		"close_reason": reason,
		"symbol":       p.Symbol,
	})
	m.trader.redis.Publish(ctx, rediskey.UserPositionUpdates(p.UserID), string(payload))

	m.logger.Info("spread monitor: close triggered",
		"position_id", p.ID,
		"close_reason", reason,
	)
}

// reconcileSpreadPositions checks open spread positions during startup.
// Called during the trader's reconciliation phase.
func (m *SpreadMonitor) reconcileSpreadPositions(ctx context.Context) error {
	positions, err := m.trader.store.ListPositions(ctx, domain.PositionFilter{
		UserID: "",
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
	})
	if err != nil {
		return fmt.Errorf("spread reconcile list positions: %w", err)
	}

	for i := range positions {
		p := &positions[i]
		if p.ArbitrageType != domain.ArbitrageTypeSpread {
			continue
		}

		// Check for timeout first (highest priority during reconciliation).
		if p.MaxHoldDuration > 0 && p.OpenedAt != nil {
			expiry := p.OpenedAt.Add(time.Duration(p.MaxHoldDuration) * time.Second)
			if time.Now().UTC().After(expiry) {
				m.logger.Warn("spread reconcile: timeout expired while offline",
					"position_id", p.ID,
					"symbol", p.Symbol,
				)
				m.triggerClose(ctx, p, domain.CloseReasonTimeout)
				continue
			}
		}

		// Check current spread.
		m.evaluateSpreadPosition(ctx, p)
	}

	return nil
}
