// Command strategy-runner evaluates automated arbitrage strategies against live
// opportunities and opens positions on behalf of users when configured criteria are met.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/pkg/logger"
	"github.com/zeroclap/gembo/internal/pkg/security"
	strategyrunner "github.com/zeroclap/gembo/internal/service/strategy-runner"
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

	if cfg.Database.URL == "" {
		log.Error("DATABASE_URL is required for the strategy-runner")
		os.Exit(1)
	}

	ctx := context.Background()
	ps, err := pgstore.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Error("postgres connect", "error", err)
		os.Exit(1)
	}
	defer ps.Close()

	if cfg.Security.CredentialsEncryptionKeyBase64 != "" {
		cipher, err := security.NewCredentialCipherFromBase64(cfg.Security.CredentialsEncryptionKeyBase64)
		if err != nil {
			log.Error("credentials encryption init", "error", err)
			os.Exit(1)
		}
		ps.WithCredentialCipher(cipher)
		log.Info("credentials encryption enabled")
	}

	if err := ps.RunMigrations(); err != nil {
		log.Error("postgres migrations", "error", err)
		os.Exit(1)
	}
	log.Info("postgres connected and migrated")

	rc := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rc.Close()

	ms := redistore.New(rc)

	runner := strategyrunner.New(ps, ps, ms, rc, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("strategy-runner starting")
	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		log.Error("strategy-runner exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("strategy-runner stopped")
}
