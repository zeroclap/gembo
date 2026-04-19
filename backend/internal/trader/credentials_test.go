package trader

import (
	"io"
	"log/slog"
	"testing"

	"github.com/zeroclap/gembo/internal/domain"
)

func TestNewTradingExchangeFromCredential_Binance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cred := &domain.ExchangeCredential{
		ID:         "cred-binance",
		ExchangeID: "binance",
		APIKey:     "key",
		APISecret:  "secret",
	}

	exch, err := newTradingExchangeFromCredential(logger, cred)
	if err != nil {
		t.Fatalf("newTradingExchangeFromCredential() error = %v", err)
	}
	if exch.ID() != "binance" {
		t.Fatalf("exchange ID = %q, want %q", exch.ID(), "binance")
	}
	if !exch.HasCredentials() {
		t.Fatal("expected Binance exchange to report configured credentials")
	}
}
