package binance

import (
	"context"
	"io"
	"log/slog"
	"testing"

	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/exchanges/base"
)

var _ base.PrivateExchange = (*Client)(nil)

func TestHasCredentials(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("missing", func(t *testing.T) {
		c := New(logger)
		if c.HasCredentials() {
			t.Fatal("expected HasCredentials=false when key and secret are empty")
		}
	})

	t.Run("present", func(t *testing.T) {
		c := New(logger, WithAPIKey("k"), WithSecret("s"))
		if !c.HasCredentials() {
			t.Fatal("expected HasCredentials=true when key and secret are set")
		}
	})

	t.Run("partial", func(t *testing.T) {
		c := New(logger, WithAPIKey("k"))
		if c.HasCredentials() {
			t.Fatal("expected HasCredentials=false when secret is missing")
		}
	})
}

func TestPositionsRequiresCredentials(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := New(logger)

	_, err := c.Positions(context.Background())
	if err == nil {
		t.Fatal("expected error when credentials are missing")
	}
}

func TestIsPositionAlreadyClosed(t *testing.T) {
	err := &ccxt.Error{Type: "InvalidOrder"}
	if !isPositionAlreadyClosed(err) {
		t.Fatal("expected InvalidOrder to be treated as already-closed position")
	}

	err = &ccxt.Error{Type: "ExchangeError"}
	if isPositionAlreadyClosed(err) {
		t.Fatal("expected non-InvalidOrder to not be treated as already-closed position")
	}
}

func TestIsAuthOrPermissionError(t *testing.T) {
	t.Run("authentication error type", func(t *testing.T) {
		err := &ccxt.Error{Type: "AuthenticationError"}
		if !isAuthOrPermissionError(err) {
			t.Fatal("expected AuthenticationError to be treated as auth/permission error")
		}
	})

	t.Run("binance code -2015 in message", func(t *testing.T) {
		err := &ccxt.Error{Type: "ExchangeError", Message: "{\"code\":-2015,\"msg\":\"Invalid API-key, IP, or permissions for action\"}"}
		if !isAuthOrPermissionError(err) {
			t.Fatal("expected -2015 message to be treated as auth/permission error")
		}
	})

	t.Run("other exchange error", func(t *testing.T) {
		err := &ccxt.Error{Type: "ExchangeError", Message: "{\"code\":-1121,\"msg\":\"Invalid symbol\"}"}
		if isAuthOrPermissionError(err) {
			t.Fatal("expected unrelated exchange error to not be treated as auth/permission error")
		}
	})
}
