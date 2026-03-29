// Command api starts the HTTP REST API server for gembo.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/api"
	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/exchange"
	bybitex "github.com/zeroclap/gembo/internal/exchange/exchanges/bybit"
	okxex "github.com/zeroclap/gembo/internal/exchange/exchanges/okx"
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

	// Exchange registry — only used for validation/routing in the API.
	reg := exchange.NewRegistry()
	registerExchanges(reg, cfg, logger)

	handler := api.NewHandler(reg, st)

	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	// Start the HTTP server.
	g.Go(func() error {
		logger.Info("api server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// Shut down the HTTP server when the context is cancelled.
	g.Go(func() error {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("shutting down api server")
		return srv.Shutdown(shutCtx)
	})

	if err := g.Wait(); err != nil && err != context.Canceled {
		logger.Error("api server exited with error", "error", err)
		os.Exit(1)
	}
	logger.Info("api server stopped")
}

// registerExchanges registers all enabled exchanges from config into the registry.
func registerExchanges(reg *exchange.Registry, cfg *config.Config, logger *slog.Logger) {
	for _, ecfg := range cfg.Exchanges {
		if !ecfg.Enabled {
			continue
		}
		switch ecfg.ID {
		case "bybit":
			reg.Register(bybitex.New(logger))
		case "okx":
			reg.Register(okxex.New(logger))
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
