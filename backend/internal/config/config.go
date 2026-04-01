// Package config loads configuration from a YAML file with environment variable overrides.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zeroclap/gembo/internal/domain"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for gembo.
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Redis        RedisConfig        `yaml:"redis"`
	Database     DatabaseConfig     `yaml:"database"`
	Exchanges    []ExchangeCfg      `yaml:"exchanges"`
	MarketFilter MarketFilterConfig `yaml:"market_filter"`
	Hooks        HooksConfig        `yaml:"hooks"`
	Trader       TraderConfig       `yaml:"trader"`
	Log          LogConfig          `yaml:"log"`
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

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL string `yaml:"url"` // postgres://user:password@host:port/dbname?sslmode=disable
}

// ExchangeCfg holds per-exchange settings.
type ExchangeCfg struct {
	ID                  string        `yaml:"id"`
	Enabled             bool          `yaml:"enabled"`
	APIKey              string        `yaml:"api_key"`
	Secret              string        `yaml:"secret"`
	Passphrase          string        `yaml:"passphrase"`
	MarketsPollInterval time.Duration `yaml:"markets_poll_interval"`
	FundingPollInterval time.Duration `yaml:"funding_poll_interval"`
}

// TraderConfig holds settings for the trader binary.
type TraderConfig struct {
	Leverage             int           `yaml:"leverage"`
	MarginMode           string        `yaml:"margin_mode"`
	MonitorInterval      time.Duration `yaml:"monitor_interval"`
	FundingCheckInterval time.Duration `yaml:"funding_check_interval"`
}

// MarketFilterConfig controls which markets are included in the data pipeline.
// Empty slices mean "no filter" (include all). This is a global filter applied
// across all exchanges.
type MarketFilterConfig struct {
	QuoteAssets  []string `yaml:"quote_assets"`  // e.g. ["USDT"]
	SettleAssets []string `yaml:"settle_assets"` // e.g. ["USDT"]
	MarketTypes  []string `yaml:"market_types"`  // e.g. ["perpetual"]
}

// ToDomain converts the YAML/env config into a domain.MarketFilter.
func (c MarketFilterConfig) ToDomain() domain.MarketFilter {
	f := domain.MarketFilter{
		QuoteAssets:  c.QuoteAssets,
		SettleAssets: c.SettleAssets,
	}
	for _, t := range c.MarketTypes {
		f.MarketTypes = append(f.MarketTypes, domain.MarketType(t))
	}
	return f
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
		Database: DatabaseConfig{},
		Hooks: HooksConfig{
			BullMQ: BullMQConfig{
				Enabled:     false,
				QueuePrefix: "gembo",
			},
		},
		Trader: TraderConfig{
			Leverage:             1,
			MarginMode:           "cross",
			MonitorInterval:      30 * time.Second,
			FundingCheckInterval: 5 * time.Minute,
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

	// MARKET_QUOTE_ASSETS=USDT — comma-separated list of allowed quote assets.
	if v := os.Getenv("MARKET_QUOTE_ASSETS"); v != "" {
		cfg.MarketFilter.QuoteAssets = splitCSV(v)
	}
	// MARKET_SETTLE_ASSETS=USDT — comma-separated list of allowed settle assets.
	if v := os.Getenv("MARKET_SETTLE_ASSETS"); v != "" {
		cfg.MarketFilter.SettleAssets = splitCSV(v)
	}
	// MARKET_TYPES=perpetual — comma-separated list of allowed market types.
	if v := os.Getenv("MARKET_TYPES"); v != "" {
		cfg.MarketFilter.MarketTypes = splitCSV(v)
	}

	// Database override.
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}

	// Exchange credential overrides: <EXCHANGE_ID>_API_KEY, <EXCHANGE_ID>_SECRET, <EXCHANGE_ID>_PASSPHRASE.
	for i := range cfg.Exchanges {
		upper := strings.ToUpper(cfg.Exchanges[i].ID)
		if v := os.Getenv(upper + "_API_KEY"); v != "" {
			cfg.Exchanges[i].APIKey = v
		}
		if v := os.Getenv(upper + "_API_SECRET"); v != "" {
			cfg.Exchanges[i].Secret = v
		}
		if v := os.Getenv(upper + "_API_PASSPHRASE"); v != "" {
			cfg.Exchanges[i].Passphrase = v
		}
	}

	// Trader overrides.
	if v := os.Getenv("TRADER_LEVERAGE"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			cfg.Trader.Leverage = l
		}
	}
	if v := os.Getenv("TRADER_MARGIN_MODE"); v != "" {
		cfg.Trader.MarginMode = v
	}
	if v := os.Getenv("TRADER_MONITOR_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Trader.MonitorInterval = d
		}
	}
	if v := os.Getenv("TRADER_FUNDING_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Trader.FundingCheckInterval = d
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

// splitCSV splits a comma-separated string into trimmed, non-empty tokens.
func splitCSV(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
