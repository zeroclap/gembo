package exchange

import (
	"context"
	"testing"

	"github.com/zeroclap/gembo/internal/domain"
)

// mockTradingExchange implements TradingExchange for testing.
type mockTradingExchange struct {
	id       string
	hasCreds bool
}

func (m *mockTradingExchange) ID() string { return m.id }
func (m *mockTradingExchange) Run(_ context.Context) error {
	return nil
}
func (m *mockTradingExchange) Markets(_ context.Context) ([]domain.Market, error) {
	return nil, nil
}
func (m *mockTradingExchange) Tickers(_ context.Context, _ []string) (<-chan domain.Ticker, error) {
	return nil, nil
}
func (m *mockTradingExchange) FundingRates(_ context.Context) (<-chan domain.FundingRate, error) {
	return nil, nil
}
func (m *mockTradingExchange) CreateMarketOrder(_ context.Context, _, _ string, _ float64) (*domain.ExchangeOrder, error) {
	return nil, nil
}
func (m *mockTradingExchange) FetchOrder(_ context.Context, _, _ string) (*domain.ExchangeOrder, error) {
	return nil, nil
}
func (m *mockTradingExchange) FetchPosition(_ context.Context, _ string) (*ExchangePosition, error) {
	return nil, nil
}
func (m *mockTradingExchange) FetchBalance(_ context.Context) ([]Balance, error) {
	return nil, nil
}
func (m *mockTradingExchange) SetMarginMode(_ context.Context, _, _ string, _ int) error {
	return nil
}
func (m *mockTradingExchange) HasCredentials() bool { return m.hasCreds }

var _ TradingExchange = (*mockTradingExchange)(nil)

func TestTradingRegistry_RegisterAndGet(t *testing.T) {
	reg := NewTradingRegistry()
	mock := &mockTradingExchange{id: "bybit", hasCreds: true}

	reg.Register(mock)

	got, err := reg.Get("bybit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID() != "bybit" {
		t.Errorf("expected ID bybit, got %s", got.ID())
	}
}

func TestTradingRegistry_GetNotFound(t *testing.T) {
	reg := NewTradingRegistry()

	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing exchange, got nil")
	}
}

func TestTradingRegistry_RegisterDuplicate(t *testing.T) {
	reg := NewTradingRegistry()
	mock := &mockTradingExchange{id: "bybit", hasCreds: true}
	reg.Register(mock)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate registration")
		}
	}()

	reg.Register(mock)
}

func TestTradingRegistry_All(t *testing.T) {
	reg := NewTradingRegistry()
	reg.Register(&mockTradingExchange{id: "bybit", hasCreds: true})
	reg.Register(&mockTradingExchange{id: "okx", hasCreds: true})

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 exchanges, got %d", len(all))
	}
}

func TestTradingRegistry_IDs(t *testing.T) {
	reg := NewTradingRegistry()
	reg.Register(&mockTradingExchange{id: "bybit", hasCreds: true})
	reg.Register(&mockTradingExchange{id: "okx", hasCreds: true})

	ids := reg.IDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet["bybit"] || !idSet["okx"] {
		t.Errorf("expected bybit and okx in IDs, got %v", ids)
	}
}

func TestTradingRegistry_Empty(t *testing.T) {
	reg := NewTradingRegistry()

	all := reg.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 exchanges, got %d", len(all))
	}

	ids := reg.IDs()
	if len(ids) != 0 {
		t.Fatalf("expected 0 IDs, got %d", len(ids))
	}
}
