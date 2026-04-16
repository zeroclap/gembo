package collector

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/hook"
	"github.com/zeroclap/gembo/internal/store"
)

// PublicCollector drives the public-data pipeline (markets, tickers, funding
// rates) for a set of exchanges. It does not require API credentials.
type PublicCollector struct {
	registry     *exchange.Registry
	store        store.Store
	redis        *goredis.Client
	emitter      hook.Emitter
	logger       *slog.Logger
	marketFilter domain.MarketFilter
}

// NewPublic creates a PublicCollector.
func NewPublic(
	registry *exchange.Registry,
	st store.Store,
	rc *goredis.Client,
	emitter hook.Emitter,
	logger *slog.Logger,
	marketFilter domain.MarketFilter,
) *PublicCollector {
	return &PublicCollector{
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
func (c *PublicCollector) Run(ctx context.Context) error {
	c.logger.Info("public collector starting", "exchanges", len(c.registry.All()))
	g, ctx := errgroup.WithContext(ctx)

	for _, ex := range c.registry.All() {
		ex := ex // capture
		g.Go(func() error { return c.runExchange(ctx, ex) })
	}

	err := g.Wait()
	c.logger.Info("public collector stopped", "error", err)
	return err
}

// runExchange starts all public-data loops for a single exchange.
func (c *PublicCollector) runExchange(ctx context.Context, ex exchange.Exchange) error {
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

// marketLoop periodically fetches markets and stores them.
func (c *PublicCollector) marketLoop(ctx context.Context, ex exchange.Exchange) error {
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

func (c *PublicCollector) fetchAndStoreMarkets(ctx context.Context, ex exchange.Exchange) {
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
func (c *PublicCollector) tickerLoop(ctx context.Context, ex exchange.Exchange) error {
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

func (c *PublicCollector) subscribeToTickers(ctx context.Context, ex exchange.Exchange, symbols []string) error {
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
func (c *PublicCollector) fundingRateLoop(ctx context.Context, ex exchange.Exchange) error {
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
