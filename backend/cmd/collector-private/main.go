// Command collector-private starts the private account-data collection pipeline
// for all configured exchanges that have API credentials. It watches positions
// via WebSocket and publishes state updates to Redis pubsub for the trader to
// consume.
package main

import (
	"context"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/collector"
	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/exchange"
	bybitex "github.com/zeroclap/gembo/internal/exchange/exchanges/bybit"
	okxex "github.com/zeroclap/gembo/internal/exchange/exchanges/okx"
	"github.com/zeroclap/gembo/internal/hook"
	"github.com/zeroclap/gembo/internal/hook/bullmq"
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

	// Redis client.
	rc := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rc.Close()

	// Event emitter.
	var emitter hook.Emitter
	if cfg.Hooks.BullMQ.Enabled {
		emitter = bullmq.New(rc, cfg.Hooks.BullMQ.QueuePrefix)
		logger.Info("bullmq emitter enabled", "prefix", cfg.Hooks.BullMQ.QueuePrefix)
	} else {
		emitter = hook.NoopEmitter{}
	}

	// Exchange registry — register only exchanges that have credentials.
	reg := exchange.NewRegistry()
	registerPrivateExchanges(reg, cfg, logger)

	if len(reg.All()) == 0 {
		logger.Error("no exchanges with credentials enabled; check config")
		os.Exit(1)
	}

	st := redistore.New(rc)
	c := collector.NewPrivate(reg, st, rc, emitter, logger)

	// Start pprof server for memory/goroutine profiling.
	pprofAddr := os.Getenv("PPROF_ADDR")
	if pprofAddr == "" {
		pprofAddr = ":6061"
	}
	go func() {
		logger.Info("pprof server starting", "addr", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			logger.Error("pprof server error", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("private collector starting", "exchanges", reg.IDs())
	if err := c.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("private collector exited with error", "error", err)
		os.Exit(1)
	}
	logger.Info("private collector stopped")
}

// registerPrivateExchanges registers only enabled exchanges that have API
// credentials configured. Exchanges without credentials are skipped because
// the private collector requires authentication for all its data streams.
func registerPrivateExchanges(reg *exchange.Registry, cfg *config.Config, logger *slog.Logger) {
	for _, ecfg := range cfg.Exchanges {
		if !ecfg.Enabled {
			continue
		}
		if ecfg.APIKey == "" || ecfg.Secret == "" {
			logger.Info("skipping exchange without credentials", "exchange", ecfg.ID)
			continue
		}
		switch ecfg.ID {
		case "bybit":
			var opts []bybitex.Option
			opts = append(opts,
				bybitex.WithAPIKey(ecfg.APIKey),
				bybitex.WithSecret(ecfg.Secret),
			)
			if ecfg.FundingPollInterval > 0 {
				opts = append(opts, bybitex.WithFundingPollInterval(ecfg.FundingPollInterval))
			}
			if ecfg.MarketsPollInterval > 0 {
				opts = append(opts, bybitex.WithMarketsPollInterval(ecfg.MarketsPollInterval))
			}
			reg.Register(bybitex.New(logger, opts...))
		case "okx":
			var opts []okxex.Option
			opts = append(opts,
				okxex.WithAPIKey(ecfg.APIKey),
				okxex.WithSecret(ecfg.Secret),
			)
			if ecfg.Passphrase != "" {
				opts = append(opts, okxex.WithPassphrase(ecfg.Passphrase))
			}
			if ecfg.FundingPollInterval > 0 {
				opts = append(opts, okxex.WithFundingPollInterval(ecfg.FundingPollInterval))
			}
			if ecfg.MarketsPollInterval > 0 {
				opts = append(opts, okxex.WithMarketsPollInterval(ecfg.MarketsPollInterval))
			}
			reg.Register(okxex.New(logger, opts...))
		case "gate", "binance":
			// Gate and Binance don't implement PositionWatcher yet.
			// They will be skipped at runtime by the PrivateCollector, but
			// we still log it here for clarity.
			logger.Info("exchange does not support private data collection yet",
				"exchange", ecfg.ID,
			)
		default:
			logger.Warn("unknown exchange in config", "id", ecfg.ID)
		}
	}
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
