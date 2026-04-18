// Command create-user creates a new application user in the database.
//
// Usage:
//
//	create-user -email admin@gembo.local -password admin123
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/domain"
	pgstore "github.com/zeroclap/gembo/internal/store/postgres"
)

func main() {
	email := flag.String("email", "", "user email (required)")
	password := flag.String("password", "", "user password (required)")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "usage: create-user -email <email> -password <password>")
		os.Exit(1)
	}

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

	if cfg.Database.URL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	ps, err := pgstore.New(ctx, cfg.Database.URL)
	if err != nil {
		slog.Error("postgres connect", "error", err)
		os.Exit(1)
	}
	defer ps.Close()

	if err := ps.RunMigrations(); err != nil {
		slog.Error("postgres migrations", "error", err)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("hash password", "error", err)
		os.Exit(1)
	}

	u := &domain.User{
		Email:        *email,
		PasswordHash: string(hash),
	}
	if err := ps.CreateUser(ctx, u); err != nil {
		slog.Error("create user", "error", err)
		os.Exit(1)
	}

	slog.Info("user created", "email", u.Email, "id", u.ID)
}
