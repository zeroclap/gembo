// Package trader implements the position execution engine. It subscribes to
// Redis Pub/Sub for position open/close requests, executes market orders on
// both exchanges concurrently, monitors open positions, and tracks funding
// payments.
package trader

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/store"
)

// Trader is the position execution engine. It runs three concurrent loops:
//   - Executor: subscribes to Redis Pub/Sub for pending/closing position signals
//   - Position Reactor: subscribes to exchange position state events from the collector
//   - FundingTracker: periodically checks for new funding payments
type Trader struct {
	registry *exchange.TradingRegistry
	store    store.PositionStore
	redis    *goredis.Client
	logger   *slog.Logger
	cfg      config.TraderConfig
}

// New creates a new Trader.
func New(
	registry *exchange.TradingRegistry,
	st store.PositionStore,
	rc *goredis.Client,
	logger *slog.Logger,
	cfg config.TraderConfig,
) *Trader {
	return &Trader{
		registry: registry,
		store:    st,
		redis:    rc,
		logger:   logger.With("component", "trader"),
		cfg:      cfg,
	}
}

// Run starts all trader loops and blocks until ctx is cancelled.
func (t *Trader) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	// Executor loop: listens for position open/close signals via Redis Pub/Sub.
	g.Go(func() error {
		return t.runExecutor(ctx)
	})

	// Position Reactor: subscribes to exchange position state events from the
	// collector via Redis pubsub, updates PG, detects closures, auto-closes.
	g.Go(func() error {
		return t.runPositionReactor(ctx)
	})

	// Funding tracker loop: periodically detects new funding payments.
	g.Go(func() error {
		return t.runFundingTracker(ctx)
	})

	t.logger.Info("trader started",
		"funding_check_interval", t.cfg.FundingCheckInterval,
		"leverage", t.cfg.Leverage,
		"margin_mode", t.cfg.MarginMode,
	)

	return g.Wait()
}

// fundingCheckInterval returns the configured funding check interval with a sensible default.
func (t *Trader) fundingCheckInterval() time.Duration {
	if t.cfg.FundingCheckInterval > 0 {
		return t.cfg.FundingCheckInterval
	}
	return 5 * time.Minute
}
