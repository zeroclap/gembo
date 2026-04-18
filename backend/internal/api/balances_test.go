package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/store"
)

type mockBalanceStore struct {
	mu       sync.Mutex
	balances map[string][]domain.Balance
}

func newMockBalanceStore() *mockBalanceStore {
	return &mockBalanceStore{balances: make(map[string][]domain.Balance)}
}

func (m *mockBalanceStore) SetMarket(context.Context, domain.Market) error { return nil }
func (m *mockBalanceStore) GetMarket(context.Context, string, string) (*domain.Market, error) {
	return nil, nil
}
func (m *mockBalanceStore) ListMarkets(context.Context, string) ([]domain.Market, error) {
	return nil, nil
}
func (m *mockBalanceStore) SetTicker(context.Context, domain.Ticker) error { return nil }
func (m *mockBalanceStore) GetTicker(context.Context, string, string) (*domain.Ticker, error) {
	return nil, nil
}
func (m *mockBalanceStore) ListTickers(context.Context, string) ([]domain.Ticker, error) {
	return nil, nil
}
func (m *mockBalanceStore) SetFundingRate(context.Context, domain.FundingRate) error { return nil }
func (m *mockBalanceStore) GetFundingRate(context.Context, string, string) (*domain.FundingRate, error) {
	return nil, nil
}
func (m *mockBalanceStore) ListFundingRates(context.Context, string) ([]domain.FundingRate, error) {
	return nil, nil
}
func (m *mockBalanceStore) SetBalance(_ context.Context, b domain.Balance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[b.ExchangeID] = append(m.balances[b.ExchangeID], b)
	return nil
}
func (m *mockBalanceStore) ListBalances(_ context.Context, exchangeID string) ([]domain.Balance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := m.balances[exchangeID]
	out := make([]domain.Balance, len(b))
	copy(out, b)
	return out, nil
}
func (m *mockBalanceStore) SetFundingArbitrage(context.Context, domain.FundingArbitrage) error {
	return nil
}
func (m *mockBalanceStore) GetFundingArbitrage(context.Context, string, string, string) (*domain.FundingArbitrage, error) {
	return nil, nil
}
func (m *mockBalanceStore) ListFundingArbitrage(context.Context) ([]domain.FundingArbitrage, error) {
	return nil, nil
}
func (m *mockBalanceStore) RemoveFundingArbitrage(context.Context, string, string, string) error {
	return nil
}
func (m *mockBalanceStore) SetSpreadArbitrage(context.Context, domain.SpreadArbitrage) error {
	return nil
}
func (m *mockBalanceStore) GetSpreadArbitrage(context.Context, string, string, string) (*domain.SpreadArbitrage, error) {
	return nil, nil
}
func (m *mockBalanceStore) ListSpreadArbitrage(context.Context) ([]domain.SpreadArbitrage, error) {
	return nil, nil
}
func (m *mockBalanceStore) RemoveSpreadArbitrage(context.Context, string, string, string) error {
	return nil
}
func (m *mockBalanceStore) FlushAll(context.Context) (int64, error) { return 0, nil }

var _ store.Store = (*mockBalanceStore)(nil)

type staticExchange struct{ id string }

func (s *staticExchange) ID() string { return s.id }
func (s *staticExchange) Run(context.Context) error {
	return nil
}
func (s *staticExchange) Markets(context.Context) ([]domain.Market, error) { return nil, nil }
func (s *staticExchange) Tickers(context.Context, []string) (<-chan domain.Ticker, error) {
	ch := make(chan domain.Ticker)
	close(ch)
	return ch, nil
}
func (s *staticExchange) FundingRates(context.Context) (<-chan domain.FundingRate, error) {
	ch := make(chan domain.FundingRate)
	close(ch)
	return ch, nil
}

func newBalanceTestHandler(st store.Store, exchangeIDs ...string) *Handler {
	reg := exchange.NewRegistry()
	for _, id := range exchangeIDs {
		reg.Register(&staticExchange{id: id})
	}
	return NewHandler(reg, st, WithJWTSecret(testJWTSecret), WithTokenTTL(time.Hour))
}

func TestListAllBalances(t *testing.T) {
	st := newMockBalanceStore()
	now := time.Now().UTC()
	_ = st.SetBalance(context.Background(), domain.Balance{
		ExchangeID: "bybit",
		Currency:   "USDT",
		Free:       decimal.RequireFromString("10"),
		Used:       decimal.RequireFromString("5"),
		Total:      decimal.RequireFromString("15"),
		UpdatedAt:  now,
	})
	_ = st.SetBalance(context.Background(), domain.Balance{
		ExchangeID: "okx",
		Currency:   "USDC",
		Free:       decimal.RequireFromString("20"),
		Used:       decimal.RequireFromString("0"),
		Total:      decimal.RequireFromString("20"),
		UpdatedAt:  now,
	})

	h := newBalanceTestHandler(st, "bybit", "okx")
	req := newAuthRequest(http.MethodGet, "/v1/balances", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var envelope struct {
		Data []domain.Balance `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(envelope.Data) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(envelope.Data))
	}
}

func TestListBalancesByExchange(t *testing.T) {
	st := newMockBalanceStore()
	now := time.Now().UTC()
	_ = st.SetBalance(context.Background(), domain.Balance{
		ExchangeID: "bybit",
		Currency:   "USDT",
		Free:       decimal.RequireFromString("7"),
		Used:       decimal.RequireFromString("3"),
		Total:      decimal.RequireFromString("10"),
		UpdatedAt:  now,
	})
	_ = st.SetBalance(context.Background(), domain.Balance{
		ExchangeID: "okx",
		Currency:   "USDT",
		Free:       decimal.RequireFromString("100"),
		Used:       decimal.Zero,
		Total:      decimal.RequireFromString("100"),
		UpdatedAt:  now,
	})

	h := newBalanceTestHandler(st, "bybit", "okx")
	req := newAuthRequest(http.MethodGet, "/v1/exchanges/bybit/balances", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var envelope struct {
		Data []domain.Balance `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(envelope.Data) != 1 {
		t.Fatalf("expected 1 balance for bybit, got %d", len(envelope.Data))
	}
	if envelope.Data[0].ExchangeID != "bybit" {
		t.Fatalf("expected bybit balance, got %s", envelope.Data[0].ExchangeID)
	}
}

func TestListBalancesByExchange_NotFound(t *testing.T) {
	h := newBalanceTestHandler(newMockBalanceStore(), "bybit")
	req := newAuthRequest(http.MethodGet, "/v1/exchanges/okx/balances", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
