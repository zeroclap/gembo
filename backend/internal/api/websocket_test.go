package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// wsTestStore extends mockStore to return test data for WS tests.
type wsTestStore struct {
	mockStore
	fundingArb *domain.FundingArbitrage
}

func (s *wsTestStore) GetFundingArbitrage(_ context.Context, _, _, _ string) (*domain.FundingArbitrage, error) {
	return s.fundingArb, nil
}

// mockExchange implements exchange.Exchange for test registry.
type mockExchange struct{ id string }

func (m *mockExchange) ID() string                                       { return m.id }
func (m *mockExchange) Run(context.Context) error                        { return nil }
func (m *mockExchange) Markets(context.Context) ([]domain.Market, error) { return nil, nil }
func (m *mockExchange) Tickers(context.Context, []string) (<-chan domain.Ticker, error) {
	return nil, nil
}
func (m *mockExchange) FundingRates(context.Context) (<-chan domain.FundingRate, error) {
	return nil, nil
}

func skipWithoutRedis(t *testing.T) *goredis.Client {
	t.Helper()
	rc := goredis.NewClient(&goredis.Options{Addr: "localhost:6379", DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := rc.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping: redis unavailable: %v", err)
	}
	return rc
}

func TestArbitrageWS_SubscribeAndReceiveSnapshot(t *testing.T) {
	rc := skipWithoutRedis(t)
	defer rc.Close()

	st := &wsTestStore{
		fundingArb: &domain.FundingArbitrage{
			Symbol: "BTC/USDT:USDT",
			Long:   domain.ArbitrageLeg{ExchangeID: "binance", Rate8h: decimal.NewFromFloat(0.001)},
			Short:  domain.ArbitrageLeg{ExchangeID: "bybit", Rate8h: decimal.NewFromFloat(0.002)},
			APR:    decimal.NewFromFloat(12.5),
		},
	}

	reg := exchange.NewRegistry()
	reg.Register(&mockExchange{id: "binance"})
	reg.Register(&mockExchange{id: "bybit"})

	h := NewHandler(reg, st, WithRedis(rc))
	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/arbitrage/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	// Subscribe.
	if err := ws.WriteJSON(map[string]interface{}{
		"type": "subscribe",
		"data": map[string]string{
			"symbol":         "BTC/USDT:USDT",
			"long_exchange":  "binance",
			"short_exchange": "bybit",
		},
	}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	// Read snapshot — single arbitrage message.
	ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	var envelope struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg, &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if envelope.Type != "arbitrage" {
		t.Fatalf("expected type 'arbitrage', got %q", envelope.Type)
	}

	var update wsArbitrageUpdate
	if err := json.Unmarshal(envelope.Data, &update); err != nil {
		t.Fatalf("unmarshal arbitrage: %v", err)
	}
	if update.Funding == nil {
		t.Fatal("expected non-nil funding arbitrage in snapshot")
	}
}

func TestArbitrageWS_PubSubForward(t *testing.T) {
	rc := skipWithoutRedis(t)
	defer rc.Close()

	st := &wsTestStore{}

	reg := exchange.NewRegistry()
	reg.Register(&mockExchange{id: "binance"})
	reg.Register(&mockExchange{id: "bybit"})

	h := NewHandler(reg, st, WithRedis(rc))
	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/arbitrage/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	// Subscribe.
	if err := ws.WriteJSON(map[string]interface{}{
		"type": "subscribe",
		"data": map[string]string{
			"symbol":         "BTC/USDT:USDT",
			"long_exchange":  "binance",
			"short_exchange": "bybit",
		},
	}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	// Drain initial snapshot (1 message).
	ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := ws.ReadMessage(); err != nil {
		t.Fatalf("drain snapshot: %v", err)
	}

	// Publish funding arbitrage updates via Redis Pub/Sub until received.
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long:   domain.ArbitrageLeg{ExchangeID: "binance", Rate8h: decimal.NewFromFloat(0.001)},
		Short:  domain.ArbitrageLeg{ExchangeID: "bybit", Rate8h: decimal.NewFromFloat(0.003)},
		APR:    decimal.NewFromFloat(15.0),
	}
	arbJSON, _ := json.Marshal(arb)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				rc.Publish(context.Background(), rediskey.FundingArbitragePubSub(), arbJSON)
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	// Read messages until we get the forwarded arbitrage update.
	ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var found bool
	for !found {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var envelope struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(msg, &envelope); err != nil {
			continue
		}
		if envelope.Type != "arbitrage" {
			continue
		}
		var update wsArbitrageUpdate
		if err := json.Unmarshal(envelope.Data, &update); err != nil {
			continue
		}
		if update.Funding != nil {
			found = true
		}
	}
	close(done)
	if !found {
		t.Fatal("never received funding arbitrage update via Pub/Sub")
	}
}
