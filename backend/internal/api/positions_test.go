package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/store"
)

// ---------------------------------------------------------------------------
// Mock PositionStore
// ---------------------------------------------------------------------------

type mockPositionStore struct {
	mu        sync.Mutex
	positions map[string]*domain.Position
	orders    map[string][]domain.ExchangeOrder
	funding   map[string][]domain.FundingPayment
}

func newMockPositionStore() *mockPositionStore {
	return &mockPositionStore{
		positions: make(map[string]*domain.Position),
		orders:    make(map[string][]domain.ExchangeOrder),
		funding:   make(map[string][]domain.FundingPayment),
	}
}

func (m *mockPositionStore) CreatePosition(_ context.Context, p *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p.ID = "pos-" + p.Symbol
	m.positions[p.ID] = p
	return nil
}

func (m *mockPositionStore) GetPosition(_ context.Context, id string) (*domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.positions[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockPositionStore) ListPositions(_ context.Context, f domain.PositionFilter) ([]domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Position
	for _, p := range m.positions {
		if f.Symbol != "" && p.Symbol != f.Symbol {
			continue
		}
		if len(f.Status) > 0 {
			match := false
			for _, s := range f.Status {
				if p.Status == s {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, *p)
	}
	return out, nil
}

func (m *mockPositionStore) UpdatePosition(_ context.Context, p *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.positions[p.ID] = p
	return nil
}

func (m *mockPositionStore) DeletePosition(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.positions, id)
	delete(m.orders, id)
	delete(m.funding, id)
	return nil
}

func (m *mockPositionStore) CreateOrder(_ context.Context, o *domain.ExchangeOrder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders[o.PositionID] = append(m.orders[o.PositionID], *o)
	return nil
}

func (m *mockPositionStore) UpdateOrder(_ context.Context, o *domain.ExchangeOrder) error {
	return nil
}

func (m *mockPositionStore) ListOrdersByPosition(_ context.Context, positionID string) ([]domain.ExchangeOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders[positionID], nil
}

func (m *mockPositionStore) CreateFundingPayment(_ context.Context, fp *domain.FundingPayment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.funding[fp.PositionID] = append(m.funding[fp.PositionID], *fp)
	return nil
}

func (m *mockPositionStore) ListFundingPayments(_ context.Context, positionID string) ([]domain.FundingPayment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.funding[positionID], nil
}

func (m *mockPositionStore) SumFundingPnL(_ context.Context, _ string) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (m *mockPositionStore) LatestFundingPayment(_ context.Context, _, _ string) (*domain.FundingPayment, error) {
	return nil, nil
}

// Verify interface compliance at compile time.
var _ store.PositionStore = (*mockPositionStore)(nil)

// ---------------------------------------------------------------------------
// Mock Store (for arbitrage snapshot)
// ---------------------------------------------------------------------------

type mockStore struct {
	arbitrages []domain.FundingArbitrage
}

func (m *mockStore) SetMarket(_ context.Context, _ domain.Market) error { return nil }
func (m *mockStore) GetMarket(_ context.Context, _, _ string) (*domain.Market, error) {
	return nil, nil
}
func (m *mockStore) ListMarkets(_ context.Context, _ string) ([]domain.Market, error) {
	return nil, nil
}
func (m *mockStore) SetTicker(_ context.Context, _ domain.Ticker) error { return nil }
func (m *mockStore) GetTicker(_ context.Context, _, _ string) (*domain.Ticker, error) {
	return nil, nil
}
func (m *mockStore) ListTickers(_ context.Context, _ string) ([]domain.Ticker, error) {
	return nil, nil
}
func (m *mockStore) SetFundingRate(_ context.Context, _ domain.FundingRate) error { return nil }
func (m *mockStore) GetFundingRate(_ context.Context, _, _ string) (*domain.FundingRate, error) {
	return nil, nil
}
func (m *mockStore) ListFundingRates(_ context.Context, _ string) ([]domain.FundingRate, error) {
	return nil, nil
}
func (m *mockStore) SetFundingArbitrage(_ context.Context, _ domain.FundingArbitrage) error {
	return nil
}
func (m *mockStore) ListFundingArbitrage(_ context.Context) ([]domain.FundingArbitrage, error) {
	return m.arbitrages, nil
}
func (m *mockStore) RemoveFundingArbitrage(_ context.Context, _, _, _ string) error { return nil }
func (m *mockStore) FlushAll(_ context.Context) (int64, error)                      { return 0, nil }

var _ store.Store = (*mockStore)(nil)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestHandler(ps *mockPositionStore, st *mockStore) *Handler {
	reg := exchange.NewRegistry()
	var opts []HandlerOption
	if ps != nil {
		opts = append(opts, WithPositionStore(ps))
	}
	return NewHandler(reg, st, opts...)
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) response {
	t.Helper()
	var resp response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreatePosition_ValidRequest(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	body := `{
		"symbol":"BTC/USDT:USDT",
		"long_exchange_id":"bybit",
		"short_exchange_id":"okx",
		"size_type":"usd",
		"requested_size":"1000"
	}`
	req := httptest.NewRequest("POST", "/v1/positions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the position was stored.
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if len(ps.positions) != 1 {
		t.Fatalf("expected 1 position in store, got %d", len(ps.positions))
	}
	for _, p := range ps.positions {
		if p.Symbol != "BTC/USDT:USDT" {
			t.Errorf("expected symbol BTC/USDT:USDT, got %s", p.Symbol)
		}
		if p.Status != domain.PositionStatusPending {
			t.Errorf("expected status pending, got %s", p.Status)
		}
		if p.LongExchangeID != "bybit" {
			t.Errorf("expected long exchange bybit, got %s", p.LongExchangeID)
		}
		if p.ShortExchangeID != "okx" {
			t.Errorf("expected short exchange okx, got %s", p.ShortExchangeID)
		}
		if !p.RequestedSize.Equal(decimal.NewFromInt(1000)) {
			t.Errorf("expected size 1000, got %s", p.RequestedSize)
		}
	}
}

func TestCreatePosition_MissingSymbol(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	body := `{"long_exchange_id":"bybit","short_exchange_id":"okx","size_type":"usd","requested_size":"1000"}`
	req := httptest.NewRequest("POST", "/v1/positions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreatePosition_InvalidSizeType(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	body := `{"symbol":"BTC/USDT:USDT","long_exchange_id":"bybit","short_exchange_id":"okx","size_type":"invalid","requested_size":"1000"}`
	req := httptest.NewRequest("POST", "/v1/positions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreatePosition_SameExchange(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	body := `{"symbol":"BTC/USDT:USDT","long_exchange_id":"bybit","short_exchange_id":"bybit","size_type":"usd","requested_size":"1000"}`
	req := httptest.NewRequest("POST", "/v1/positions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreatePosition_NegativeSize(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	body := `{"symbol":"BTC/USDT:USDT","long_exchange_id":"bybit","short_exchange_id":"okx","size_type":"usd","requested_size":"-100"}`
	req := httptest.NewRequest("POST", "/v1/positions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreatePosition_ZeroSize(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	body := `{"symbol":"BTC/USDT:USDT","long_exchange_id":"bybit","short_exchange_id":"okx","size_type":"base","requested_size":"0"}`
	req := httptest.NewRequest("POST", "/v1/positions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListPositions_Empty(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	req := httptest.NewRequest("GET", "/v1/positions", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestListPositions_WithFilter(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	// Seed some positions.
	ps.positions["p1"] = &domain.Position{
		ID:     "p1",
		Symbol: "BTC/USDT:USDT",
		Status: domain.PositionStatusOpen,
	}
	ps.positions["p2"] = &domain.Position{
		ID:     "p2",
		Symbol: "ETH/USDT:USDT",
		Status: domain.PositionStatusClosed,
	}

	req := httptest.NewRequest("GET", "/v1/positions?status=open", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetPosition_NotFound(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	req := httptest.NewRequest("GET", "/v1/positions/nonexistent", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetPosition_Found(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	ps.positions["p1"] = &domain.Position{
		ID:     "p1",
		Symbol: "BTC/USDT:USDT",
		Status: domain.PositionStatusOpen,
	}

	req := httptest.NewRequest("GET", "/v1/positions/p1", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestClosePosition_NotFound(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	req := httptest.NewRequest("POST", "/v1/positions/nope/close", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestClosePosition_WrongStatus(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	ps.positions["p1"] = &domain.Position{
		ID:     "p1",
		Symbol: "BTC/USDT:USDT",
		Status: domain.PositionStatusPending,
	}

	req := httptest.NewRequest("POST", "/v1/positions/p1/close", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestClosePosition_Success(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	ps.positions["p1"] = &domain.Position{
		ID:     "p1",
		Symbol: "BTC/USDT:USDT",
		Status: domain.PositionStatusOpen,
	}

	req := httptest.NewRequest("POST", "/v1/positions/p1/close", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify status changed.
	ps.mu.Lock()
	p := ps.positions["p1"]
	ps.mu.Unlock()
	if p.Status != domain.PositionStatusClosing {
		t.Errorf("expected status closing, got %s", p.Status)
	}
}

func TestListPositionOrders_Empty(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	req := httptest.NewRequest("GET", "/v1/positions/p1/orders", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestListPositionFunding_Empty(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	req := httptest.NewRequest("GET", "/v1/positions/p1/funding", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestPositionRoutes_Unavailable_WithoutStore(t *testing.T) {
	st := &mockStore{}
	h := newTestHandler(nil, st) // No position store

	req := httptest.NewRequest("GET", "/v1/positions", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	// Should 503 because position store is not configured.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when positionStore is nil, got %d", rec.Code)
	}
}

func TestCreatePosition_InvalidJSON(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	req := httptest.NewRequest("POST", "/v1/positions", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeletePosition_Success(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	ps.positions["p1"] = &domain.Position{
		ID:     "p1",
		Symbol: "BTC/USDT:USDT",
		Status: domain.PositionStatusPending,
	}

	req := httptest.NewRequest("DELETE", "/v1/positions/p1", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the position was removed.
	ps.mu.Lock()
	_, exists := ps.positions["p1"]
	ps.mu.Unlock()
	if exists {
		t.Fatal("expected position to be deleted from store")
	}
}

func TestDeletePosition_NotFound(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	req := httptest.NewRequest("DELETE", "/v1/positions/nonexistent", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeletePosition_NotPending(t *testing.T) {
	ps := newMockPositionStore()
	st := &mockStore{}
	h := newTestHandler(ps, st)

	ps.positions["p1"] = &domain.Position{
		ID:     "p1",
		Symbol: "BTC/USDT:USDT",
		Status: domain.PositionStatusOpen,
	}

	req := httptest.NewRequest("DELETE", "/v1/positions/p1", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	// Verify the position still exists.
	ps.mu.Lock()
	_, exists := ps.positions["p1"]
	ps.mu.Unlock()
	if !exists {
		t.Fatal("expected position to still exist in store")
	}
}
