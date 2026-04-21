// Command position-manager is the position execution engine for Gembo. It
// subscribes to Redis Pub/Sub signals from the API, executes market orders on
// exchanges, monitors open positions, and tracks funding payments.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/internal/pkg/logger"
	"github.com/zeroclap/gembo/internal/pkg/security"
	positionmanager "github.com/zeroclap/gembo/internal/service/position-manager"
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

	if cfg.Database.URL == "" {
		log.Error("DATABASE_URL is required for the position-manager")
		os.Exit(1)
	}
	ctx := context.Background()
	ps, err := pgstore.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Error("postgres connect", "error", err)
		os.Exit(1)
	}
	if cfg.Security.CredentialsEncryptionKeyBase64 != "" {
		cipher, err := security.NewCredentialCipherFromBase64(cfg.Security.CredentialsEncryptionKeyBase64)
		if err != nil {
			log.Error("credentials encryption init", "error", err)
			os.Exit(1)
		}
		ps.WithCredentialCipher(cipher)
		log.Info("credentials encryption enabled")
	}
	defer ps.Close()

	if err := ps.RunMigrations(); err != nil {
		log.Error("postgres migrations", "error", err)
		os.Exit(1)
	}
	log.Info("postgres connected and migrated")

	// Registry is intentionally empty in DB-only credential mode.
	reg := base.NewPrivateExchangeRegistry()

	marketStore := redistore.New(rc)

	t := positionmanager.New(reg, ps, rc, log, cfg.PositionManager,
		positionmanager.WithMarketStore(marketStore),
		positionmanager.WithUserStore(ps),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("position-manager starting", "credential_mode", "db_only")
	if err := t.Run(ctx); err != nil && err != context.Canceled {
		log.Error("position-manager exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("position-manager stopped")
}
