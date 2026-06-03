// Package marketdatacollector orchestrates the public-data pipeline (markets,
// tickers, funding rates) for a set of exchanges. No API credentials required.
package marketdatacollector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/internal/pkg/hook"
	"github.com/zeroclap/gembo/internal/store"
)

const (
	marketsPollInterval  = 60 * time.Second
	snapshotInterval     = 10 * time.Second
)

// Collector drives the public-data pipeline.
type Collector struct {
	registry     *base.Registry
	store        store.Store
	redis        *goredis.Client
	emitter      hook.Emitter
	logger       *slog.Logger
	marketFilter domain.MarketFilter
}

// New creates a Collector.
func New(
	registry *base.Registry,
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

// Run starts all public-data exchange pipelines and blocks until ctx is
// cancelled or an unrecoverable error occurs.
func (c *Collector) Run(ctx context.Context) error {
	c.logger.Info("market-data collector starting", "exchanges", len(c.registry.All()))
	g, ctx := errgroup.WithContext(ctx)

	for _, ex := range c.registry.All() {
		ex := ex
		g.Go(func() error { return c.runExchange(ctx, ex) })
	}

	err := g.Wait()
	c.logger.Info("market-data collector stopped", "error", err)
	return err
}

func (c *Collector) runExchange(ctx context.Context, ex base.Exchange) error {
	c.logger.Info("starting exchange (public)", "exchange", ex.ID())
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return ex.Run(ctx) })
	g.Go(func() error { return c.marketLoop(ctx, ex) })
	g.Go(func() error { return c.tickerLoop(ctx, ex) })
	g.Go(func() error { return c.fundingRateLoop(ctx, ex) })

	err := g.Wait()
	c.logger.Info("exchange stopped (public)", "exchange", ex.ID(), "error", err)
	return err
}

func (c *Collector) marketLoop(ctx context.Context, ex base.Exchange) error {
	ticker := time.NewTicker(marketsPollInterval)
	defer ticker.Stop()

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

func (c *Collector) fetchAndStoreMarkets(ctx context.Context, ex base.Exchange) {
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

func (c *Collector) tickerLoop(ctx context.Context, ex base.Exchange) error {
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

	return c.subscribeToTickers(ctx, ex, symbols)
}

func (c *Collector) subscribeToTickers(ctx context.Context, ex base.Exchange, symbols []string) error {
	ch, err := ex.Tickers(ctx, symbols)
	if err != nil {
		return fmt.Errorf("start tickers for %s: %w", ex.ID(), err)
	}

	// lastSnapshot tracks when each symbol last had a snapshot written.
	// Keyed by symbol only — this function is called once per exchange so the
	// map is naturally scoped to a single exchange.
	lastSnapshot := make(map[string]time.Time)

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

			if time.Since(lastSnapshot[t.Symbol]) >= snapshotInterval {
				c.appendTickerSnapshot(ctx, t)
				lastSnapshot[t.Symbol] = time.Now()
			}
		}
	}
}

// appendTickerSnapshot writes a TickerSnapshot for the given ticker, enriched
// with the latest funding rate from the store (read once per call; funding rate
// has no TTL so it is always current).
func (c *Collector) appendTickerSnapshot(ctx context.Context, t domain.Ticker) {
	snap := domain.TickerSnapshot{
		ExchangeID: t.ExchangeID,
		Symbol:     t.Symbol,
		TS:         t.UpdatedAt.Unix(),
		Price:      t.Price,
		Bid:        t.Bid,
		Ask:        t.Ask,
	}

	if fr, err := c.store.GetFundingRate(ctx, t.ExchangeID, t.Symbol); err == nil && fr != nil {
		snap.FundingRate = fr.Rate
		snap.MarkPrice = fr.MarkPrice
		snap.IndexPrice = fr.IndexPrice
		if fr.IntervalHours > 0 {
			// Normalize to 8h equivalent, consistent with the arbitrage calculator.
			snap.Rate8h = fr.Rate.
				Mul(decimal.NewFromInt(8)).
				Div(decimal.NewFromInt(int64(fr.IntervalHours)))
		}
	}

	if err := c.store.AppendTickerSnapshot(ctx, snap); err != nil {
		c.logger.Error("append ticker snapshot error",
			"exchange", t.ExchangeID,
			"symbol", t.Symbol,
			"error", err,
		)
	}
}

func (c *Collector) fundingRateLoop(ctx context.Context, ex base.Exchange) error {
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
