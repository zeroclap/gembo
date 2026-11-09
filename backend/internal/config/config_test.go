package config

import (
	"testing"
	"time"

	"github.com/zeroclap/gembo/internal/domain"
)

// setenv sets environment variables for the duration of a test and restores
// them via t.Cleanup so parallel tests are not affected.
func setenv(t *testing.T, kvs map[string]string) {
	t.Helper()
	for k, v := range kvs {
		t.Setenv(k, v)
	}
}

// TestExchangesEnvFiltersYAML verifies that EXCHANGES acts as an authoritative
// filter: exchanges present in YAML but absent from the env var are disabled.
func TestExchangesEnvFiltersYAML(t *testing.T) {
	// Simulate a YAML that has bybit + okx both enabled.
	base := func() Config {
		return Config{
			Exchanges: []ExchangeCfg{
				{ID: "bybit", Enabled: true},
				{ID: "okx", Enabled: true},
			},
		}
	}

	t.Run("subset disables unlisted", func(t *testing.T) {
		setenv(t, map[string]string{"EXCHANGES": "bybit"})
		cfg := base()
		applyEnv(&cfg)

		for _, ex := range cfg.Exchanges {
			switch ex.ID {
			case "bybit":
				if !ex.Enabled {
					t.Errorf("bybit should be enabled")
				}
			case "okx":
				if ex.Enabled {
					t.Errorf("okx should be disabled when not in EXCHANGES")
				}
			}
		}
	})

	t.Run("new id added with defaults", func(t *testing.T) {
		setenv(t, map[string]string{"EXCHANGES": "bybit,binance"})
		cfg := base()
		applyEnv(&cfg)

		found := false
		for _, ex := range cfg.Exchanges {
			if ex.ID == "binance" {
				found = true
				if !ex.Enabled {
					t.Errorf("binance should be enabled")
				}
			}
			if ex.ID == "okx" && ex.Enabled {
				t.Errorf("okx should be disabled when not in EXCHANGES")
			}
		}
		if !found {
			t.Errorf("binance not added to exchange list")
		}
	})

	t.Run("all listed exchanges enabled", func(t *testing.T) {
		setenv(t, map[string]string{"EXCHANGES": "bybit,okx"})
		cfg := base()
		applyEnv(&cfg)

		for _, ex := range cfg.Exchanges {
			if !ex.Enabled {
				t.Errorf("exchange %q should be enabled", ex.ID)
			}
		}
	})

	t.Run("unset EXCHANGES leaves YAML untouched", func(t *testing.T) {
		// Do NOT set EXCHANGES — ensure env from other tests doesn't bleed in.
		t.Setenv("EXCHANGES", "")
		cfg := base()
		applyEnv(&cfg)

		for _, ex := range cfg.Exchanges {
			if !ex.Enabled {
				t.Errorf("exchange %q should remain enabled when EXCHANGES is unset", ex.ID)
			}
		}
	})
}

func TestQuoteQualityConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := defaults()
		if got := cfg.QuoteQuality.ToDomain(); got != domain.DefaultQualityPolicy() {
			t.Errorf("policy = %+v, want defaults", got)
		}
	})

	t.Run("env override", func(t *testing.T) {
		setenv(t, map[string]string{
			"QUOTE_MAX_TICKER_AGE":  "3s",
			"QUOTE_MAX_FUNDING_AGE": "90s",
			"QUOTE_MAX_FUTURE_SKEW": "bogus", // ignored
		})
		cfg := defaults()
		applyEnv(&cfg)
		got := cfg.QuoteQuality.ToDomain()
		if got.MaxTickerAge != 3*time.Second || got.MaxFundingAge != 90*time.Second {
			t.Errorf("policy = %+v", got)
		}
		if got.MaxFutureSkew != domain.DefaultMaxFutureSkew {
			t.Errorf("MaxFutureSkew = %v, want default", got.MaxFutureSkew)
		}
	})

	t.Run("non-positive falls back to defaults", func(t *testing.T) {
		if got := (QuoteQualityConfig{}).ToDomain(); got != domain.DefaultQualityPolicy() {
			t.Errorf("policy = %+v, want defaults", got)
		}
	})
}
