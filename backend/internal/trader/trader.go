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
//   - SpreadMonitor: subscribes to ticker events, monitors spread position auto-close triggers
type Trader struct {
	registry      *exchange.TradingRegistry
	store         store.PositionStore
	marketStore   store.Store // Redis store for ticker data (used by SpreadMonitor).
	redis         *goredis.Client
	logger        *slog.Logger
	cfg           config.TraderConfig
	spreadMonitor *SpreadMonitor
}

// Option is a functional option for configuring a Trader.
type Option func(*Trader)

// WithMarketStore sets the market data store (Redis) for the spread monitor.
func WithMarketStore(st store.Store) Option {
	return func(t *Trader) { t.marketStore = st }
}

// New creates a new Trader.
func New(
	registry *exchange.TradingRegistry,
	st store.PositionStore,
	rc *goredis.Client,
	logger *slog.Logger,
	cfg config.TraderConfig,
	opts ...Option,
) *Trader {
	t := &Trader{
		registry: registry,
		store:    st,
		redis:    rc,
		logger:   logger.With("component", "trader"),
		cfg:      cfg,
	}
	for _, o := range opts {
		o(t)
	}
	// Initialize spread monitor if market store is available.
	if t.marketStore != nil {
		t.spreadMonitor = newSpreadMonitor(t, t.marketStore)
	}
	return t
}

// Run starts all trader loops and blocks until ctx is cancelled.
// On startup, it reconciles all active positions against exchange state to
// detect any changes that occurred while the trader was down. Reconciliation
// failures are logged but do not prevent the trader from starting.
func (t *Trader) Run(ctx context.Context) error {
	// Reconcile positions before entering the main loops. This catches
	// closures, partial closes, and stuck transitions that happened while
	// the trader was offline (Redis pubsub events are fire-and-forget).
	if err := t.reconcileOnStartup(ctx); err != nil {
		t.logger.Error("position reconciliation failed, continuing", "error", err)
	}

	// Reconcile spread positions (check for timeouts and trigger closures).
	if t.spreadMonitor != nil {
		if err := t.spreadMonitor.reconcileSpreadPositions(ctx); err != nil {
			t.logger.Error("spread position reconciliation failed, continuing", "error", err)
		}
	}

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

	// Spread monitor: subscribes to ticker events, checks open spread
	// positions for auto-close triggers (target, stop-loss, timeout).
	if t.spreadMonitor != nil {
		g.Go(func() error {
			return t.spreadMonitor.run(ctx)
		})
	}

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
