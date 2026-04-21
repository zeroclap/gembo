// Command market-data-collector starts the public-data collection pipeline for
// all configured exchanges. It fetches markets, tickers, and funding rates
// without requiring any API credentials.
package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/exchanges"
	"github.com/zeroclap/gembo/internal/pkg/hook"
	"github.com/zeroclap/gembo/internal/pkg/hook/bullmq"
	"github.com/zeroclap/gembo/internal/pkg/logger"
	marketdatacollector "github.com/zeroclap/gembo/internal/service/market-data-collector"
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
		os.Stderr.WriteString("load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.Log)

	rc := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rc.Close()

	st := redistore.New(rc)

	var emitter hook.Emitter
	if cfg.Hooks.BullMQ.Enabled {
		emitter = bullmq.New(rc, cfg.Hooks.BullMQ.QueuePrefix)
		log.Info("bullmq emitter enabled", "prefix", cfg.Hooks.BullMQ.QueuePrefix)
	} else {
		emitter = hook.NoopEmitter{}
	}

	reg := exchanges.BuildPublicRegistry(cfg, log)
	if len(reg.All()) == 0 {
		log.Error("no exchanges enabled; check config")
		os.Exit(1)
	}

	mf := cfg.MarketFilter
	if len(mf.QuoteAssets) > 0 || len(mf.SettleAssets) > 0 || len(mf.MarketTypes) > 0 {
		log.Info("market filter active",
			"quote_assets", mf.QuoteAssets,
			"settle_assets", mf.SettleAssets,
			"market_types", mf.MarketTypes,
		)
	}

	c := marketdatacollector.New(reg, st, rc, emitter, log, mf.ToDomain())

	pprofAddr := os.Getenv("PPROF_ADDR")
	if pprofAddr == "" {
		pprofAddr = ":6060"
	}
	go func() {
		log.Info("pprof server starting", "addr", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			log.Error("pprof server error", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("market-data-collector starting", "exchanges", reg.IDs())
	if err := c.Run(ctx); err != nil && err != context.Canceled {
		log.Error("market-data-collector exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("market-data-collector stopped")
}
