package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development; tighten for production.
	},
}

// wsMessage is the envelope for all WebSocket messages (both directions).
type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// wsSubscribePayload is sent by the client to subscribe to a symbol pair.
type wsSubscribePayload struct {
	Symbol    string `json:"symbol"`
	ExchangeA string `json:"exchange_a"`
	ExchangeB string `json:"exchange_b"`
}

// handleArbitrageWS upgrades to WebSocket and streams real-time arbitrage
// updates for a subscribed symbol pair. On subscribe, an initial snapshot is
// sent, then updates are pushed as soon as they arrive via Redis Pub/Sub.
func (h *Handler) handleArbitrageWS(w http.ResponseWriter, r *http.Request) {
	if h.redis == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable", "websocket requires redis")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// wsMu serializes writes to the WebSocket connection.
	var wsMu sync.Mutex

	// Current subscription — guarded by subMu.
	var subMu sync.RWMutex
	var sub wsSubscribePayload

	getSub := func() wsSubscribePayload {
		subMu.RLock()
		defer subMu.RUnlock()
		return sub
	}

	// subReady is closed once the first subscribe message arrives.
	subReady := make(chan struct{})
	subReadyOnce := sync.Once{}

	// Read loop — receives subscribe messages from the client.
	go func() {
		defer cancel()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var envelope wsMessage
			if err := json.Unmarshal(msg, &envelope); err != nil {
				continue
			}
			if envelope.Type == "subscribe" {
				var payload wsSubscribePayload
				if err := json.Unmarshal(envelope.Data, &payload); err != nil {
					continue
				}
				if payload.Symbol == "" || payload.ExchangeA == "" || payload.ExchangeB == "" {
					continue
				}
				subMu.Lock()
				sub = payload
				subMu.Unlock()
				subReadyOnce.Do(func() { close(subReady) })

				// Send snapshot for new subscription.
				s := getSub()
				wsMu.Lock()
				h.sendArbitrageSnapshot(ctx, conn, s)
				wsMu.Unlock()
			}
		}
	}()

	// Wait for the first subscription before starting Pub/Sub.
	select {
	case <-subReady:
	case <-ctx.Done():
		return
	}

	// Subscribe to both arbitrage updates and ticker snapshot updates.
	pubsub := h.redis.Subscribe(ctx, rediskey.ArbitragePubSub(), rediskey.TickerSnapshotPubSub())
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-ch:
			if !ok {
				return
			}
			s := getSub()
			switch msg.Channel {
			case rediskey.ArbitragePubSub():
				h.handlePubSubMessage(conn, &wsMu, s, msg.Payload)
			case rediskey.TickerSnapshotPubSub():
				h.handleSnapshotPubSub(conn, &wsMu, s, msg.Payload)
			}
		}
	}
}

// handlePubSubMessage filters and forwards a Redis Pub/Sub message to the
// WebSocket client if it matches the current subscription.
func (h *Handler) handlePubSubMessage(conn *websocket.Conn, wsMu *sync.Mutex, sub wsSubscribePayload, payload string) {
	var a domain.Arbitrage
	if err := json.Unmarshal([]byte(payload), &a); err != nil {
		return
	}
	if a.Symbol != sub.Symbol {
		return
	}
	if !matchesExchangePair(a.ExchangeA, a.ExchangeB, sub.ExchangeA, sub.ExchangeB) {
		return
	}
	wsMu.Lock()
	writeWSMessage(conn, "arbitrage", a)
	wsMu.Unlock()
}

// handleSnapshotPubSub forwards a ticker snapshot to the client if the snapshot
// belongs to either leg of the subscribed pair.
func (h *Handler) handleSnapshotPubSub(conn *websocket.Conn, wsMu *sync.Mutex, sub wsSubscribePayload, payload string) {
	var snap domain.TickerSnapshot
	if err := json.Unmarshal([]byte(payload), &snap); err != nil {
		return
	}
	if snap.Symbol != sub.Symbol {
		return
	}
	if snap.ExchangeID != sub.ExchangeA && snap.ExchangeID != sub.ExchangeB {
		return
	}
	wsMu.Lock()
	writeWSMessage(conn, "ticker_snapshot", snap)
	wsMu.Unlock()
}

// matchesExchangePair returns true if the two exchange IDs match the subscription
// pair in either direction.
func matchesExchangePair(exA, exB, subA, subB string) bool {
	return (exA == subA && exB == subB) || (exA == subB && exB == subA)
}

// sendArbitrageSnapshot reads current data from the store and pushes full
// snapshots to the WebSocket client for all strategy types, then sends ticker
// snapshot history for the subscribed pair.
// Caller must hold wsMu.
func (h *Handler) sendArbitrageSnapshot(ctx context.Context, conn *websocket.Conn, sub wsSubscribePayload) {
	for _, arbType := range []domain.ArbitrageType{domain.ArbitrageTypeFunding, domain.ArbitrageTypeSpread} {
		arb, _ := h.store.GetArbitrage(ctx, arbType, sub.Symbol, sub.ExchangeA, sub.ExchangeB)
		if arb != nil {
			writeWSMessage(conn, "arbitrage", arb)
		}
	}
	h.sendTickerSnapshotHistory(ctx, conn, sub)
}

// sendTickerSnapshotHistory fetches the last 1h of ticker snapshots for both
// legs, aggregates them into OHLC bars for each resolution, and sends a single
// ticker_snapshot_history message. Caller must hold wsMu.
func (h *Handler) sendTickerSnapshotHistory(ctx context.Context, conn *websocket.Conn, sub wsSubscribePayload) {
	history := buildSnapshotHistory(ctx, h, sub.ExchangeA, sub.ExchangeB, sub.Symbol)
	if history == nil {
		return
	}
	writeWSMessage(conn, "ticker_snapshot_history", history)
}

// writeWSMessage marshals and sends a typed WebSocket message.
func writeWSMessage(conn *websocket.Conn, msgType string, data interface{}) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := wsMessage{Type: msgType, Data: raw}
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		slog.Debug("websocket write", "error", err)
	}
}
