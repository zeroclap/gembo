// Command arbitrage-worker subscribes to Redis Pub/Sub events (ticker and
// funding rate updates) and recomputes arbitrage opportunities.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/pkg/arbitrage"
	"github.com/zeroclap/gembo/internal/pkg/logger"
	arbitrageworker "github.com/zeroclap/gembo/internal/service/arbitrage-worker"
	pgstore "github.com/zeroclap/gembo/internal/store/postgres"
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

	exchangeIDs := enabledExchangeIDs(cfg)
	if len(exchangeIDs) == 0 {
		log.Error("no exchanges enabled; check config")
		os.Exit(1)
	}

	calc := arbitrage.NewCalculator(st, log)
	spreadCalc := arbitrage.NewSpreadCalculator(st, log)

	w := arbitrageworker.New(rc, calc, spreadCalc, exchangeIDs, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.Run(ctx) })

	// Alert worker — optional, requires a PostgreSQL database.
	if cfg.Database.URL != "" {
		ps, err := pgstore.New(ctx, cfg.Database.URL)
		if err != nil {
			log.Warn("alert worker: postgres connect failed, alerts disabled", "error", err)
		} else {
			defer ps.Close()
			alertWorker := arbitrageworker.NewAlertWorker(ps, st, rc, log)
			g.Go(func() error { return alertWorker.Run(ctx) })
			log.Info("alert worker enabled")
		}
	} else {
		log.Info("alert worker disabled (no DATABASE_URL configured)")
	}

	log.Info("arbitrage-worker starting", "exchanges", exchangeIDs)
	if err := g.Wait(); err != nil && err != context.Canceled {
		log.Error("arbitrage-worker exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("arbitrage-worker stopped")
}

// enabledExchangeIDs returns IDs of all enabled exchanges from config.
func enabledExchangeIDs(cfg *config.Config) []string {
	var ids []string
	for _, ecfg := range cfg.Exchanges {
		if ecfg.Enabled {
			ids = append(ids, ecfg.ID)
		}
	}
	return ids
}
