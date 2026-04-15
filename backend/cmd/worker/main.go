// Command worker subscribes to Redis Pub/Sub events (ticker and funding rate
// updates) and recomputes funding rate arbitrage opportunities.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/arbitrage"
	"github.com/zeroclap/gembo/internal/config"
	redistore "github.com/zeroclap/gembo/internal/store/redis"
)

func main() {
	_ = godotenv.Load()

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "configs/config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.Log)

	// Redis client for store reads/writes.
	rc := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rc.Close()

	st := redistore.New(rc)

	// Resolve enabled exchange IDs from config.
	exchangeIDs := enabledExchangeIDs(cfg)
	if len(exchangeIDs) == 0 {
		logger.Error("no exchanges enabled; check config")
		os.Exit(1)
	}

	calc := arbitrage.NewCalculator(st, logger)
	spreadCalc := arbitrage.NewSpreadCalculator(st, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	// Subscribe to ticker and funding rate update channels from all exchanges.
	// Uses PSubscribe with patterns: gembo:*:ticker:updates, gembo:*:funding:updates
	g.Go(func() error {
		return runSubscriber(ctx, rc, calc, spreadCalc, exchangeIDs, logger)
	})

	logger.Info("worker starting", "exchanges", exchangeIDs)
	if err := g.Wait(); err != nil && err != context.Canceled {
		logger.Error("worker exited with error", "error", err)
		os.Exit(1)
	}
	logger.Info("worker stopped")
}

// runSubscriber listens for Redis Pub/Sub events and triggers recomputation.
// It debounces rapid events to avoid excessive recalculations.
func runSubscriber(
	ctx context.Context,
	rc *goredis.Client,
	calc *arbitrage.Calculator,
	spreadCalc *arbitrage.SpreadCalculator,
	exchangeIDs []string,
	logger *slog.Logger,
) error {
	pubsub := rc.PSubscribe(ctx,
		"gembo:*:ticker:updates",
		"gembo:*:funding:updates",
	)
	defer pubsub.Close()

	ch := pubsub.Channel()

	// Debounce: wait for a quiet period before recomputing.
	const debounceInterval = 2 * time.Second
	timer := time.NewTimer(debounceInterval)
	timer.Stop()

	pending := false

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()

		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			logger.Debug("pubsub event", "channel", msg.Channel)
			if !pending {
				timer.Reset(debounceInterval)
				pending = true
			}

		case <-timer.C:
			pending = false
			start := time.Now()
			if err := calc.Recompute(ctx, exchangeIDs); err != nil {
				logger.Warn("funding recompute failed", "error", err)
			}
			if err := spreadCalc.Recompute(ctx, exchangeIDs); err != nil {
				logger.Warn("spread recompute failed", "error", err)
			}
			logger.Debug("recompute done", "duration", time.Since(start))
		}
	}
}

// enabledExchangeIDs extracts the IDs of all enabled exchanges from config.
func enabledExchangeIDs(cfg *config.Config) []string {
	var ids []string
	for _, ecfg := range cfg.Exchanges {
		if ecfg.Enabled {
			ids = append(ids, ecfg.ID)
		}
	}
	return ids
}

// newLogger constructs a slog.Logger from LogConfig.
func newLogger(cfg config.LogConfig) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}
