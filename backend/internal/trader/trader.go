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
//   - Monitor: periodically polls exchange position state (mark price, unrealized P&L)
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

	// Monitor loop: periodically updates open position state from exchanges.
	g.Go(func() error {
		return t.runMonitor(ctx)
	})

	// Funding tracker loop: periodically detects new funding payments.
	g.Go(func() error {
		return t.runFundingTracker(ctx)
	})

	t.logger.Info("trader started",
		"monitor_interval", t.cfg.MonitorInterval,
		"funding_check_interval", t.cfg.FundingCheckInterval,
		"leverage", t.cfg.Leverage,
		"margin_mode", t.cfg.MarginMode,
	)

	return g.Wait()
}

// monitorInterval returns the configured monitor interval with a sensible default.
func (t *Trader) monitorInterval() time.Duration {
	if t.cfg.MonitorInterval > 0 {
		return t.cfg.MonitorInterval
	}
	return 30 * time.Second
}

// fundingCheckInterval returns the configured funding check interval with a sensible default.
func (t *Trader) fundingCheckInterval() time.Duration {
	if t.cfg.FundingCheckInterval > 0 {
		return t.cfg.FundingCheckInterval
	}
	return 5 * time.Minute
}
