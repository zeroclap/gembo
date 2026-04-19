package trader

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	binanceex "github.com/zeroclap/gembo/internal/exchange/exchanges/binance"
	bybitex "github.com/zeroclap/gembo/internal/exchange/exchanges/bybit"
	okxex "github.com/zeroclap/gembo/internal/exchange/exchanges/okx"
)

func tradingClientKey(userID, credentialID string) string {
	return userID + ":" + credentialID
}

func (t *Trader) tradingExchangeForLeg(ctx context.Context, p *domain.Position, leg string) (exchange.TradingExchange, error) {
	if t.userStore == nil {
		return nil, fmt.Errorf("user store is required for credential-based trading")
	}

	var credentialID string
	var exchangeID string
	if leg == "long" {
		credentialID = p.LongCredentialID
		exchangeID = p.LongExchangeID
	} else {
		credentialID = p.ShortCredentialID
		exchangeID = p.ShortExchangeID
	}

	if existing, err := t.registry.Get(exchangeID); err == nil {
		return existing, nil
	}

	cacheKey := tradingClientKey(p.UserID, credentialID)
	t.clientMu.RLock()
	if client, ok := t.clients[cacheKey]; ok {
		t.clientMu.RUnlock()
		return client, nil
	}
	t.clientMu.RUnlock()

	cred, err := t.userStore.GetExchangeCredentialByID(ctx, p.UserID, credentialID)
	if err != nil {
		return nil, fmt.Errorf("resolve leg credential: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("credential %s not found for user %s", credentialID, p.UserID)
	}
	if cred.ExchangeID != exchangeID {
		return nil, fmt.Errorf("credential %s exchange %s does not match leg exchange %s", credentialID, cred.ExchangeID, exchangeID)
	}

	logger := t.logger.With(
		slog.String("user_id", p.UserID),
		slog.String("credential_id", cred.ID),
		slog.String("exchange", cred.ExchangeID),
	)

	client, err := newTradingExchangeFromCredential(logger, cred)
	if err != nil {
		return nil, err
	}

	t.clientMu.Lock()
	defer t.clientMu.Unlock()
	if existing, ok := t.clients[cacheKey]; ok {
		return existing, nil
	}
	t.clients[cacheKey] = client
	return client, nil
}

func newTradingExchangeFromCredential(logger *slog.Logger, cred *domain.ExchangeCredential) (exchange.TradingExchange, error) {
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
		return nil, fmt.Errorf("exchange %s is not supported for trading", cred.ExchangeID)
	}
}
