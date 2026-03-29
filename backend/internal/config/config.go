// Package config loads configuration from a YAML file with environment variable overrides.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for gembo.
type Config struct {
	Server    ServerConfig  `yaml:"server"`
	Redis     RedisConfig   `yaml:"redis"`
	Exchanges []ExchangeCfg `yaml:"exchanges"`
	Hooks     HooksConfig   `yaml:"hooks"`
	Log       LogConfig     `yaml:"log"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// ExchangeCfg holds per-exchange settings.
type ExchangeCfg struct {
	ID                  string        `yaml:"id"`
	Enabled             bool          `yaml:"enabled"`
	MarketsPollInterval time.Duration `yaml:"markets_poll_interval"`
	FundingPollInterval time.Duration `yaml:"funding_poll_interval"`
}

// HooksConfig holds event hook settings.
type HooksConfig struct {
	BullMQ BullMQConfig `yaml:"bullmq"`
}

// BullMQConfig holds BullMQ publisher settings.
type BullMQConfig struct {
	Enabled     bool   `yaml:"enabled"`
	QueuePrefix string `yaml:"queue_prefix"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // json | text
}

// defaults returns a Config populated with sensible defaults.
func defaults() Config {
	return Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Redis: RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		},
		Hooks: HooksConfig{
			BullMQ: BullMQConfig{
				Enabled:     false,
				QueuePrefix: "gembo",
			},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load loads configuration from a YAML file and applies environment variable overrides.
// If path is empty or the file does not exist, only defaults and env vars are used.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		f, err := os.Open(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("open config %s: %w", path, err)
		}
		if err == nil {
			defer f.Close()
			if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
				return nil, fmt.Errorf("parse config %s: %w", path, err)
			}
		}
	}

	// Apply environment variable overrides.
	applyEnv(&cfg)

	return &cfg, nil
}

// applyEnv overlays environment variables onto cfg.
func applyEnv(cfg *Config) {
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if db, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = db
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := os.Getenv("BULLMQ_ENABLED"); v != "" {
		cfg.Hooks.BullMQ.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("BULLMQ_QUEUE_PREFIX"); v != "" {
		cfg.Hooks.BullMQ.QueuePrefix = v
	}
	// EXCHANGES=bybit,okx enables exchanges without a config file.
	// Each comma-separated ID is upserted into the exchanges list with enabled=true
	// and default poll intervals. IDs already present in the YAML list are updated
	// in place so explicit YAML settings are preserved.
	if v := os.Getenv("EXCHANGES"); v != "" {
		for _, id := range strings.Split(v, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			found := false
			for i := range cfg.Exchanges {
				if cfg.Exchanges[i].ID == id {
					cfg.Exchanges[i].Enabled = true
					found = true
					break
				}
			}
			if !found {
				cfg.Exchanges = append(cfg.Exchanges, ExchangeCfg{
					ID:                  id,
					Enabled:             true,
					MarketsPollInterval: 60 * time.Second,
					FundingPollInterval: 60 * time.Second,
				})
			}
		}
	}
}

// Addr returns the server listen address as "host:port".
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ExchangeByID returns the ExchangeCfg for the given ID, or a zero value if not found.
func (c *Config) ExchangeByID(id string) (ExchangeCfg, bool) {
	for _, e := range c.Exchanges {
		if e.ID == id {
			return e, true
		}
	}
	return ExchangeCfg{}, false
}
