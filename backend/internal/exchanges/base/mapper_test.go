package base_test

import (
	"testing"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/exchanges/base"
)

func ptrFloat(f float64) *float64 { return &f }
func ptrString(s string) *string  { return &s }

func TestMapPosition(t *testing.T) {
	p := ccxt.Position{
		Symbol:           ptrString("BTC/USDT:USDT"),
		Side:             ptrString("long"),
		Contracts:        ptrFloat(5.0),
		EntryPrice:       ptrFloat(42000.0),
		MarkPrice:        ptrFloat(42500.0),
		LiquidationPrice: ptrFloat(35000.0),
		UnrealizedPnl:    ptrFloat(2500.0),
		Leverage:         ptrFloat(10.0),
		MarginMode:       ptrString("cross"),
		Notional:         ptrFloat(212500.0),
	}

	ep := base.MapPosition(p)

	if ep.Symbol != "BTC/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", ep.Symbol, "BTC/USDT:USDT")
	}
	if ep.Side != "long" {
		t.Errorf("Side = %q, want %q", ep.Side, "long")
	}
	if !ep.Contracts.Equal(decimal.NewFromFloat(5.0)) {
		t.Errorf("Contracts = %s, want 5", ep.Contracts)
	}
	if !ep.EntryPrice.Equal(decimal.NewFromFloat(42000.0)) {
		t.Errorf("EntryPrice = %s, want 42000", ep.EntryPrice)
	}
	if !ep.UnrealizedPnL.Equal(decimal.NewFromFloat(2500.0)) {
		t.Errorf("UnrealizedPnL = %s, want 2500", ep.UnrealizedPnL)
	}
	if ep.MarginMode != "cross" {
		t.Errorf("MarginMode = %q, want %q", ep.MarginMode, "cross")
	}
}

func TestMapBalances(t *testing.T) {
	b := ccxt.Balances{
		Balances: map[string]ccxt.Balance{
			"USDT": {
				Free:  ptrFloat(1000.0),
				Used:  ptrFloat(500.0),
				Total: ptrFloat(1500.0),
			},
		},
	}

	bals := base.MapBalances(b)

	if len(bals) != 1 {
		t.Fatalf("len(bals) = %d, want 1", len(bals))
	}
	bal := bals[0]
	if bal.Currency != "USDT" {
		t.Errorf("Currency = %q, want %q", bal.Currency, "USDT")
	}
	if !bal.Free.Equal(decimal.NewFromFloat(1000.0)) {
		t.Errorf("Free = %s, want 1000", bal.Free)
	}
	if !bal.Used.Equal(decimal.NewFromFloat(500.0)) {
		t.Errorf("Used = %s, want 500", bal.Used)
	}
	if !bal.Total.Equal(decimal.NewFromFloat(1500.0)) {
		t.Errorf("Total = %s, want 1500", bal.Total)
	}
}

func TestMapBalances_Empty(t *testing.T) {
	b := ccxt.Balances{}
	bals := base.MapBalances(b)
	if len(bals) != 0 {
		t.Errorf("len(bals) = %d, want 0 for empty balances", len(bals))
	}
}
