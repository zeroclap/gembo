package collector

import (
	"io"
	"log/slog"
	"testing"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
)

func TestNewPrivateExchangeFromCredential_Binance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cred := domain.ExchangeCredential{
		ID:         "cred-binance",
		ExchangeID: "binance",
		APIKey:     "key",
		APISecret:  "secret",
	}

	exch, err := newPrivateExchangeFromCredential(logger, cred)
	if err != nil {
		t.Fatalf("newPrivateExchangeFromCredential() error = %v", err)
	}
	if exch.ID() != "binance" {
		t.Fatalf("exchange ID = %q, want %q", exch.ID(), "binance")
	}

	tx, ok := exch.(exchange.TradingExchange)
	if !ok {
		t.Fatal("expected Binance private exchange to implement TradingExchange")
	}
	if !tx.HasCredentials() {
		t.Fatal("expected Binance private exchange to report configured credentials")
	}
}
