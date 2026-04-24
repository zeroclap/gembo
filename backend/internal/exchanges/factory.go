// Package exchanges provides the top-level wiring for exchange adapters.
// It imports both the base interfaces and all concrete adapter packages,
// making it the sole place where exchange construction is centralised.
package exchanges

import (
	"fmt"
	"log/slog"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	binanceex "github.com/zeroclap/gembo/internal/exchanges/binance"
	bybitex "github.com/zeroclap/gembo/internal/exchanges/bybit"
	gateex "github.com/zeroclap/gembo/internal/exchanges/gate"
	htxex "github.com/zeroclap/gembo/internal/exchanges/htx"
	okxex "github.com/zeroclap/gembo/internal/exchanges/okx"
)

// BuildPublicRegistry constructs a base.Registry populated with all enabled
// exchanges from cfg, configured for public data collection only (no credentials).
func BuildPublicRegistry(cfg *config.Config, logger *slog.Logger) *base.Registry {
	reg := base.NewRegistry()
	for _, ecfg := range cfg.Exchanges {
		if !ecfg.Enabled {
			continue
		}
		switch ecfg.ID {
		case "bybit":
			opts := bybitPublicOpts(ecfg)
			reg.Register(bybitex.New(logger, opts...))
		case "okx":
			opts := okxPublicOpts(ecfg)
			reg.Register(okxex.New(logger, opts...))
		case "gate":
			opts := gatePublicOpts(ecfg)
			reg.Register(gateex.New(logger, opts...))
		case "htx":
			opts := htxPublicOpts(ecfg)
			reg.Register(htxex.New(logger, opts...))
		case "binance":
			opts := binancePublicOpts(ecfg)
			reg.Register(binanceex.New(logger, opts...))
		default:
			logger.Warn("unknown exchange in config", "id", ecfg.ID)
		}
	}
	return reg
}

// NewPrivateExchangeFromCredential constructs a PrivateExchange adapter from a
// stored credential. Returns an error for unsupported or misconfigured exchanges.
func NewPrivateExchangeFromCredential(logger *slog.Logger, cred domain.ExchangeCredential) (base.PrivateExchange, error) {
	switch cred.ExchangeID {
	case "binance":
		return binanceex.New(logger,
			binanceex.WithAPIKey(cred.APIKey),
			binanceex.WithSecret(cred.APISecret),
		), nil
	case "bybit":
		return bybitex.New(logger,
			bybitex.WithAPIKey(cred.APIKey),
			bybitex.WithSecret(cred.APISecret),
		), nil
	case "okx":
		if cred.Passphrase == "" {
			return nil, fmt.Errorf("okx credential %s requires passphrase", cred.ID)
		}
		return okxex.New(logger,
			okxex.WithAPIKey(cred.APIKey),
			okxex.WithSecret(cred.APISecret),
			okxex.WithPassphrase(cred.Passphrase),
		), nil
	default:
		return nil, fmt.Errorf("exchange %s is not supported for private collection", cred.ExchangeID)
	}
}

// — helpers — keep the switch arms readable.

func bybitPublicOpts(e config.ExchangeCfg) []bybitex.Option {
	var opts []bybitex.Option
	if e.FundingPollInterval > 0 {
		opts = append(opts, bybitex.WithFundingPollInterval(e.FundingPollInterval))
	}
	if e.MarketsPollInterval > 0 {
		opts = append(opts, bybitex.WithMarketsPollInterval(e.MarketsPollInterval))
	}
	return opts
}

func okxPublicOpts(e config.ExchangeCfg) []okxex.Option {
	var opts []okxex.Option
	if e.FundingPollInterval > 0 {
		opts = append(opts, okxex.WithFundingPollInterval(e.FundingPollInterval))
	}
	if e.MarketsPollInterval > 0 {
		opts = append(opts, okxex.WithMarketsPollInterval(e.MarketsPollInterval))
	}
	return opts
}

func gatePublicOpts(e config.ExchangeCfg) []gateex.Option {
	var opts []gateex.Option
	if e.FundingPollInterval > 0 {
		opts = append(opts, gateex.WithFundingPollInterval(e.FundingPollInterval))
	}
	if e.MarketsPollInterval > 0 {
		opts = append(opts, gateex.WithMarketsPollInterval(e.MarketsPollInterval))
	}
	return opts
}

func htxPublicOpts(e config.ExchangeCfg) []htxex.Option {
	var opts []htxex.Option
	if e.FundingPollInterval > 0 {
		opts = append(opts, htxex.WithFundingPollInterval(e.FundingPollInterval))
	}
	if e.MarketsPollInterval > 0 {
		opts = append(opts, htxex.WithMarketsPollInterval(e.MarketsPollInterval))
	}
	return opts
}

func binancePublicOpts(e config.ExchangeCfg) []binanceex.Option {
	var opts []binanceex.Option
	if e.FundingPollInterval > 0 {
		opts = append(opts, binanceex.WithFundingPollInterval(e.FundingPollInterval))
	}
	if e.MarketsPollInterval > 0 {
		opts = append(opts, binanceex.WithMarketsPollInterval(e.MarketsPollInterval))
	}
	return opts
}
