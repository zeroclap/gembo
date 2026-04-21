// Command account-data-collector starts the private account-data collection
// pipeline for all configured exchanges that have stored API credentials. It
// watches positions via WebSocket and publishes state updates to Redis.
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
	"github.com/zeroclap/gembo/internal/pkg/hook"
	"github.com/zeroclap/gembo/internal/pkg/hook/bullmq"
	"github.com/zeroclap/gembo/internal/pkg/logger"
	"github.com/zeroclap/gembo/internal/pkg/security"
	accountdatacollector "github.com/zeroclap/gembo/internal/service/account-data-collector"
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

	var emitter hook.Emitter
	if cfg.Hooks.BullMQ.Enabled {
		emitter = bullmq.New(rc, cfg.Hooks.BullMQ.QueuePrefix)
		log.Info("bullmq emitter enabled", "prefix", cfg.Hooks.BullMQ.QueuePrefix)
	} else {
		emitter = hook.NoopEmitter{}
	}

	if cfg.Database.URL == "" {
		log.Error("DATABASE_URL is required for the account-data-collector")
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

	st := redistore.New(rc)
	c := accountdatacollector.New(ps, st, rc, emitter, log)

	pprofAddr := os.Getenv("PPROF_ADDR")
	if pprofAddr == "" {
		pprofAddr = ":6061"
	}
	go func() {
		log.Info("pprof server starting", "addr", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			log.Error("pprof server error", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("account-data-collector starting", "credential_mode", "db_only")
	if err := c.Run(ctx); err != nil && err != context.Canceled {
		log.Error("account-data-collector exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("account-data-collector stopped")
}
