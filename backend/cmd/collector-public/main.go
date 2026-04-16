// Command collector-public starts the public-data collection pipeline for all
// configured exchanges. It fetches markets, tickers, and funding rates without
// requiring any API credentials.
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
	binanceex "github.com/zeroclap/gembo/internal/exchange/exchanges/binance"
	bybitex "github.com/zeroclap/gembo/internal/exchange/exchanges/bybit"
	gateex "github.com/zeroclap/gembo/internal/exchange/exchanges/gate"
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

	st := redistore.New(rc)

	// Event emitter.
	var emitter hook.Emitter
	if cfg.Hooks.BullMQ.Enabled {
		emitter = bullmq.New(rc, cfg.Hooks.BullMQ.QueuePrefix)
		logger.Info("bullmq emitter enabled", "prefix", cfg.Hooks.BullMQ.QueuePrefix)
	} else {
		emitter = hook.NoopEmitter{}
	}

	// Exchange registry — register all enabled exchanges WITHOUT credentials.
	// Public collector never needs API keys.
	reg := exchange.NewRegistry()
	registerPublicExchanges(reg, cfg, logger)

	if len(reg.All()) == 0 {
		logger.Error("no exchanges enabled; check config")
		os.Exit(1)
	}

	c := collector.NewPublic(reg, st, rc, emitter, logger, cfg.MarketFilter.ToDomain())

	mf := cfg.MarketFilter
	if len(mf.QuoteAssets) > 0 || len(mf.SettleAssets) > 0 || len(mf.MarketTypes) > 0 {
		logger.Info("market filter active",
			"quote_assets", mf.QuoteAssets,
			"settle_assets", mf.SettleAssets,
			"market_types", mf.MarketTypes,
		)
	}

	// Start pprof server for memory/goroutine profiling.
	pprofAddr := os.Getenv("PPROF_ADDR")
	if pprofAddr == "" {
		pprofAddr = ":6060"
	}
	go func() {
		logger.Info("pprof server starting", "addr", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			logger.Error("pprof server error", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("public collector starting", "exchanges", reg.IDs())
	if err := c.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("public collector exited with error", "error", err)
		os.Exit(1)
	}
	logger.Info("public collector stopped")
}

// registerPublicExchanges registers all enabled exchanges WITHOUT credentials.
// Only poll intervals are configured — no API keys, secrets, or passphrases.
func registerPublicExchanges(reg *exchange.Registry, cfg *config.Config, logger *slog.Logger) {
	for _, ecfg := range cfg.Exchanges {
		if !ecfg.Enabled {
			continue
		}
		switch ecfg.ID {
		case "bybit":
			var opts []bybitex.Option
			if ecfg.FundingPollInterval > 0 {
				opts = append(opts, bybitex.WithFundingPollInterval(ecfg.FundingPollInterval))
			}
			if ecfg.MarketsPollInterval > 0 {
				opts = append(opts, bybitex.WithMarketsPollInterval(ecfg.MarketsPollInterval))
			}
			// No credentials — public data only.
			reg.Register(bybitex.New(logger, opts...))
		case "okx":
			var opts []okxex.Option
			if ecfg.FundingPollInterval > 0 {
				opts = append(opts, okxex.WithFundingPollInterval(ecfg.FundingPollInterval))
			}
			if ecfg.MarketsPollInterval > 0 {
				opts = append(opts, okxex.WithMarketsPollInterval(ecfg.MarketsPollInterval))
			}
			// No credentials — public data only.
			reg.Register(okxex.New(logger, opts...))
		case "gate":
			var opts []gateex.Option
			if ecfg.FundingPollInterval > 0 {
				opts = append(opts, gateex.WithFundingPollInterval(ecfg.FundingPollInterval))
			}
			if ecfg.MarketsPollInterval > 0 {
				opts = append(opts, gateex.WithMarketsPollInterval(ecfg.MarketsPollInterval))
			}
			reg.Register(gateex.New(logger, opts...))
		case "binance":
			var opts []binanceex.Option
			if ecfg.FundingPollInterval > 0 {
				opts = append(opts, binanceex.WithFundingPollInterval(ecfg.FundingPollInterval))
			}
			if ecfg.MarketsPollInterval > 0 {
				opts = append(opts, binanceex.WithMarketsPollInterval(ecfg.MarketsPollInterval))
			}
			reg.Register(binanceex.New(logger, opts...))
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
