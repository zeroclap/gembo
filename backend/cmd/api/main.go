// Command api starts the HTTP REST API server for gembo.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/exchanges"
	"github.com/zeroclap/gembo/internal/pkg/logger"
	"github.com/zeroclap/gembo/internal/pkg/security"
	serviceapi "github.com/zeroclap/gembo/internal/service/api"
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

	// Exchange registry — only used for validation/routing in the API.
	reg := exchanges.BuildPublicRegistry(cfg, log)

	var handlerOpts []serviceapi.HandlerOption
	handlerOpts = append(handlerOpts, serviceapi.WithRedis(rc))
	handlerOpts = append(handlerOpts, serviceapi.WithJWTSecret(cfg.Auth.JWTSecret))
	handlerOpts = append(handlerOpts, serviceapi.WithTokenTTL(cfg.Auth.TokenTTL))

	if cfg.Database.URL != "" {
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
		handlerOpts = append(handlerOpts, serviceapi.WithPositionStore(ps))
		handlerOpts = append(handlerOpts, serviceapi.WithUserStore(ps))
		handlerOpts = append(handlerOpts, serviceapi.WithNotificationStore(ps))
		handlerOpts = append(handlerOpts, serviceapi.WithStrategyStore(ps))
	}

	handler := serviceapi.NewHandler(reg, st, handlerOpts...)

	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("api server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		log.Info("shutting down api server")
		return srv.Shutdown(shutCtx)
	})

	if err := g.Wait(); err != nil && err != context.Canceled {
		log.Error("api server exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("api server stopped")
}
