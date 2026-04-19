// Command trader is the position execution engine for Gembo. It subscribes to
// Redis Pub/Sub signals from the API, executes market orders on exchanges,
// monitors open positions, and tracks funding payments.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/security"
	pgstore "github.com/zeroclap/gembo/internal/store/postgres"
	redistore "github.com/zeroclap/gembo/internal/store/redis"
	"github.com/zeroclap/gembo/internal/trader"
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

	// Redis client for Pub/Sub and data reads.
	rc := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rc.Close()

	// PostgreSQL — required for the trader.
	if cfg.Database.URL == "" {
		logger.Error("DATABASE_URL is required for the trader")
		os.Exit(1)
	}
	ctx := context.Background()
	ps, err := pgstore.New(ctx, cfg.Database.URL)
	if err != nil {
		logger.Error("postgres connect", "error", err)
		os.Exit(1)
	}
	if cfg.Security.CredentialsEncryptionKeyBase64 != "" {
		credentialCipher, err := security.NewCredentialCipherFromBase64(cfg.Security.CredentialsEncryptionKeyBase64)
		if err != nil {
			logger.Error("credentials encryption init", "error", err)
			os.Exit(1)
		}
		ps.WithCredentialCipher(credentialCipher)
		logger.Info("credentials encryption enabled")
	}
	defer ps.Close()

	if err := ps.RunMigrations(); err != nil {
		logger.Error("postgres migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("postgres connected and migrated")

	// Trading exchange registry is intentionally empty in DB-only credential mode.
	reg := exchange.NewTradingRegistry()

	// Redis store for market data reads (used by spread monitor).
	marketStore := redistore.New(rc)

	t := trader.New(reg, ps, rc, logger, cfg.Trader,
		trader.WithMarketStore(marketStore),
		trader.WithUserStore(ps),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("trader starting", "credential_mode", "db_only")
	if err := t.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("trader exited with error", "error", err)
		os.Exit(1)
	}
	logger.Info("trader stopped")
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
