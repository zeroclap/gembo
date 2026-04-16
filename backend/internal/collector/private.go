package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/hook"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// PrivateCollector drives the private-data pipeline (positions, orders,
// balances) for a set of exchanges. Requires API credentials on each exchange.
type PrivateCollector struct {
	registry *exchange.Registry
	redis    *goredis.Client
	emitter  hook.Emitter
	logger   *slog.Logger
}

// NewPrivate creates a PrivateCollector.
func NewPrivate(
	registry *exchange.Registry,
	rc *goredis.Client,
	emitter hook.Emitter,
	logger *slog.Logger,
) *PrivateCollector {
	return &PrivateCollector{
		registry: registry,
		redis:    rc,
		emitter:  emitter,
		logger:   logger,
	}
}

// Run starts all private-data exchange pipelines and blocks until ctx is
// cancelled or an unrecoverable error occurs.
func (c *PrivateCollector) Run(ctx context.Context) error {
	c.logger.Info("private collector starting", "exchanges", len(c.registry.All()))
	g, ctx := errgroup.WithContext(ctx)

	for _, ex := range c.registry.All() {
		ex := ex // capture
		g.Go(func() error { return c.runExchange(ctx, ex) })
	}

	err := g.Wait()
	c.logger.Info("private collector stopped", "error", err)
	return err
}

// runExchange starts all private-data loops for a single exchange.
func (c *PrivateCollector) runExchange(ctx context.Context, ex exchange.Exchange) error {
	c.logger.Info("starting exchange (private)", "exchange", ex.ID())
	g, ctx := errgroup.WithContext(ctx)

	// Keep the underlying WS connections alive.
	g.Go(func() error { return ex.Run(ctx) })

	// Position loop: only for exchanges that implement PositionWatcher.
	pw, ok := ex.(exchange.PositionWatcher)
	if !ok {
		c.logger.Warn("exchange does not support position watching, skipping",
			"exchange", ex.ID(),
		)
		// Still need to keep Run alive; wait for context cancellation.
		<-ctx.Done()
		return ctx.Err()
	}

	g.Go(func() error { return c.positionLoop(ctx, ex.ID(), pw) })

	err := g.Wait()
	c.logger.Info("exchange stopped (private)", "exchange", ex.ID(), "error", err)
	return err
}

// positionLoop consumes position updates from an exchange that implements
// PositionWatcher and publishes each position as an ExchangePositionEvent to
// Redis pubsub. The trader subscribes to these events.
func (c *PrivateCollector) positionLoop(ctx context.Context, exchangeID string, pw exchange.PositionWatcher) error {
	ch, err := pw.Positions(ctx)
	if err != nil {
		return fmt.Errorf("start position watcher for %s: %w", exchangeID, err)
	}

	c.logger.Info("position loop started", "exchange", exchangeID)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("position loop shutting down", "exchange", exchangeID)
			return ctx.Err()
		case positions, ok := <-ch:
			if !ok {
				c.logger.Info("position channel closed", "exchange", exchangeID)
				return nil
			}
			for _, pos := range positions {
				event := domain.ExchangePositionEvent{
					ExchangeID:       exchangeID,
					Symbol:           pos.Symbol,
					Side:             pos.Side,
					Contracts:        pos.Contracts,
					EntryPrice:       pos.EntryPrice,
					MarkPrice:        pos.MarkPrice,
					LiquidationPrice: pos.LiquidationPrice,
					UnrealizedPnL:    pos.UnrealizedPnL,
					Leverage:         pos.Leverage,
					MarginMode:       pos.MarginMode,
					Notional:         pos.Notional,
					Timestamp:        time.Now().UTC(),
				}
				payload, err := json.Marshal(event)
				if err != nil {
					c.logger.Error("marshal position event", "error", err)
					continue
				}
				if err := c.redis.Publish(ctx, rediskey.PositionStatePubSub(exchangeID), payload).Err(); err != nil {
					c.logger.Warn("publish position event",
						"exchange", exchangeID,
						"symbol", pos.Symbol,
						"error", err,
					)
				}
			}
		}
	}
}
