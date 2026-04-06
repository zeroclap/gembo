// Package collector orchestrates the pipeline from exchanges to the store and event emitter.
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
	"github.com/zeroclap/gembo/internal/store"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

const marketsPollInterval = 60 * time.Second

// Collector drives the data pipeline for a set of exchanges.
type Collector struct {
	registry     *exchange.Registry
	store        store.Store
	redis        *goredis.Client
	emitter      hook.Emitter
	logger       *slog.Logger
	marketFilter domain.MarketFilter
}

// New creates a Collector.
func New(
	registry *exchange.Registry,
	st store.Store,
	rc *goredis.Client,
	emitter hook.Emitter,
	logger *slog.Logger,
	marketFilter domain.MarketFilter,
) *Collector {
	return &Collector{
		registry:     registry,
		store:        st,
		redis:        rc,
		emitter:      emitter,
		logger:       logger,
		marketFilter: marketFilter,
	}
}

// Run starts all exchange pipelines and blocks until ctx is cancelled or an
// unrecoverable error occurs.
func (c *Collector) Run(ctx context.Context) error {
	c.logger.Info("collector starting", "exchanges", len(c.registry.All()))
	g, ctx := errgroup.WithContext(ctx)

	for _, ex := range c.registry.All() {
		ex := ex // capture
		g.Go(func() error { return c.runExchange(ctx, ex) })
	}

	err := g.Wait()
	c.logger.Info("collector stopped", "error", err)
	return err
}

// runExchange starts all loops for a single exchange.
func (c *Collector) runExchange(ctx context.Context, ex exchange.Exchange) error {
	c.logger.Info("starting exchange", "exchange", ex.ID())
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return ex.Run(ctx) })
	g.Go(func() error { return c.marketLoop(ctx, ex) })
	g.Go(func() error { return c.tickerLoop(ctx, ex) })
	g.Go(func() error { return c.fundingRateLoop(ctx, ex) })

	// Position loop: only for exchanges that implement PositionWatcher (have credentials).
	if pw, ok := ex.(exchange.PositionWatcher); ok {
		g.Go(func() error { return c.positionLoop(ctx, ex.ID(), pw) })
	}

	err := g.Wait()
	c.logger.Info("exchange stopped", "exchange", ex.ID(), "error", err)
	return err
}

// marketLoop periodically fetches markets and stores them.
func (c *Collector) marketLoop(ctx context.Context, ex exchange.Exchange) error {
	ticker := time.NewTicker(marketsPollInterval)
	defer ticker.Stop()

	// Poll immediately.
	c.fetchAndStoreMarkets(ctx, ex)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("market loop shutting down", "exchange", ex.ID())
			return ctx.Err()
		case <-ticker.C:
			c.fetchAndStoreMarkets(ctx, ex)
		}
	}
}

func (c *Collector) fetchAndStoreMarkets(ctx context.Context, ex exchange.Exchange) {
	markets, err := ex.Markets(ctx)
	if err != nil {
		c.logger.Error("fetch markets error", "exchange", ex.ID(), "error", err)
		return
	}

	before := len(markets)
	markets = domain.FilterMarkets(markets, c.marketFilter)
	if !c.marketFilter.IsEmpty() {
		c.logger.Debug("market filter applied",
			"exchange", ex.ID(),
			"before", before,
			"after", len(markets),
		)
	}

	for _, m := range markets {
		if err := c.store.SetMarket(ctx, m); err != nil {
			c.logger.Error("store market error",
				"exchange", ex.ID(),
				"symbol", m.Symbol,
				"error", err,
			)
			continue
		}
		c.emitMarket(ctx, m)
	}

	c.logger.Info("markets updated", "exchange", ex.ID(), "count", len(markets))
}

// tickerLoop consumes the ticker channel from an exchange and stores each update.
// It fetches markets and chunks them into batches of 100 symbols for subscription.
func (c *Collector) tickerLoop(ctx context.Context, ex exchange.Exchange) error {
	markets, err := ex.Markets(ctx)
	if err != nil {
		return fmt.Errorf("fetch markets for tickers %s: %w", ex.ID(), err)
	}

	markets = domain.FilterMarkets(markets, c.marketFilter)

	symbols := make([]string, 0, len(markets))
	for _, m := range markets {
		symbols = append(symbols, m.Symbol)
	}

	if len(symbols) == 0 {
		c.logger.Warn("no symbols available for ticker subscription", "exchange", ex.ID())
		return nil
	}

	// Chunk symbols into batches of 100
	const batchSize = 100
	g, gctx := errgroup.WithContext(ctx)

	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]

		g.Go(func() error {
			return c.subscribeToTickers(gctx, ex, batch)
		})
	}

	// Wait for all subscriptions with timeout on shutdown
	done := make(chan error, 1)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context cancelled, give goroutines 5 seconds to clean up
		c.logger.Info("ticker loop shutting down, waiting for subscriptions to close",
			"exchange", ex.ID(),
			"batches", (len(symbols)+batchSize-1)/batchSize,
		)

		shutdownTimer := time.NewTimer(5 * time.Second)
		defer shutdownTimer.Stop()

		select {
		case err := <-done:
			c.logger.Info("ticker loop shutdown complete", "exchange", ex.ID())
			return err
		case <-shutdownTimer.C:
			c.logger.Warn("ticker loop shutdown timeout, some subscriptions may not have closed cleanly",
				"exchange", ex.ID(),
			)
			return ctx.Err()
		}
	}
}

func (c *Collector) subscribeToTickers(ctx context.Context, ex exchange.Exchange, symbols []string) error {
	ch, err := ex.Tickers(ctx, symbols)
	if err != nil {
		return fmt.Errorf("start tickers for %s: %w", ex.ID(), err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t, ok := <-ch:
			if !ok {
				return nil
			}
			if err := c.store.SetTicker(ctx, t); err != nil {
				c.logger.Error("store ticker error",
					"exchange", ex.ID(),
					"symbol", t.Symbol,
					"error", err,
				)
				continue
			}
			c.emitTicker(ctx, t)
		}
	}
}

// fundingRateLoop consumes the funding rate channel from an exchange.
func (c *Collector) fundingRateLoop(ctx context.Context, ex exchange.Exchange) error {
	ch, err := ex.FundingRates(ctx)
	if err != nil {
		return fmt.Errorf("start funding rates for %s: %w", ex.ID(), err)
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("funding rate loop shutting down", "exchange", ex.ID())
			return ctx.Err()
		case fr, ok := <-ch:
			if !ok {
				c.logger.Info("funding rate channel closed", "exchange", ex.ID())
				return nil
			}
			if err := c.store.SetFundingRate(ctx, fr); err != nil {
				c.logger.Error("store funding rate error",
					"exchange", ex.ID(),
					"symbol", fr.Symbol,
					"error", err,
				)
				continue
			}
			c.emitFundingRate(ctx, fr)
		}
	}
}

// positionLoop consumes position updates from an exchange that implements
// PositionWatcher and publishes each position as an ExchangePositionEvent to
// Redis pubsub. The trader subscribes to these events.
func (c *Collector) positionLoop(ctx context.Context, exchangeID string, pw exchange.PositionWatcher) error {
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

// --- event emission helpers ---

func (c *Collector) emitMarket(ctx context.Context, m domain.Market) {
	payload, err := json.Marshal(m)
	if err != nil {
		c.logger.Error("marshal market for event", "error", err)
		return
	}
	ev := hook.Event{
		Type:     hook.EventMarketUpdated,
		Exchange: m.ExchangeID,
		Symbol:   m.Symbol,
		Payload:  payload,
	}
	if err := c.emitter.Emit(ctx, ev); err != nil {
		c.logger.Warn("emit market event error", "exchange", m.ExchangeID, "symbol", m.Symbol, "error", err)
	}
}

func (c *Collector) emitTicker(ctx context.Context, t domain.Ticker) {
	payload, err := json.Marshal(t)
	if err != nil {
		c.logger.Error("marshal ticker for event", "error", err)
		return
	}
	ev := hook.Event{
		Type:     hook.EventTickerUpdated,
		Exchange: t.ExchangeID,
		Symbol:   t.Symbol,
		Payload:  payload,
	}
	if err := c.emitter.Emit(ctx, ev); err != nil {
		c.logger.Warn("emit ticker event error", "exchange", t.ExchangeID, "symbol", t.Symbol, "error", err)
	}
}

func (c *Collector) emitFundingRate(ctx context.Context, fr domain.FundingRate) {
	payload, err := json.Marshal(fr)
	if err != nil {
		c.logger.Error("marshal funding rate for event", "error", err)
		return
	}
	ev := hook.Event{
		Type:     hook.EventFundingUpdated,
		Exchange: fr.ExchangeID,
		Symbol:   fr.Symbol,
		Payload:  payload,
	}
	if err := c.emitter.Emit(ctx, ev); err != nil {
		c.logger.Warn("emit funding rate event error", "exchange", fr.ExchangeID, "symbol", fr.Symbol, "error", err)
	}
}
