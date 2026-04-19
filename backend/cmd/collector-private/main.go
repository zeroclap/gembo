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
	"github.com/zeroclap/gembo/internal/hook"
	"github.com/zeroclap/gembo/internal/hook/bullmq"
	"github.com/zeroclap/gembo/internal/security"
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

	if cfg.Database.URL == "" {
		logger.Error("DATABASE_URL is required for the private collector")
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

	st := redistore.New(rc)
	c := collector.NewPrivate(ps, st, rc, emitter, logger)

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

	logger.Info("private collector starting", "credential_mode", "db_only")
	if err := c.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("private collector exited with error", "error", err)
		os.Exit(1)
	}
	logger.Info("private collector stopped")
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
