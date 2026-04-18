package api

import (
	"net/http"

	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/store"
)

// Handler wires all API routes onto an http.ServeMux.
type Handler struct {
	mux           *http.ServeMux
	registry      *exchange.Registry
	store         store.Store
	positionStore store.PositionStore // nil if PostgreSQL is not configured.
	redis         *goredis.Client     // for Pub/Sub signaling to the trader process.
}

// NewHandler creates a Handler and registers all routes.
func NewHandler(registry *exchange.Registry, st store.Store, opts ...HandlerOption) *Handler {
	h := &Handler{
		mux:      http.NewServeMux(),
		registry: registry,
		store:    st,
	}
	for _, o := range opts {
		o(h)
	}
	h.registerRoutes()
	return h
}

// HandlerOption is a functional option for configuring a Handler.
type HandlerOption func(*Handler)

// WithPositionStore sets the PositionStore for position management endpoints.
func WithPositionStore(ps store.PositionStore) HandlerOption {
	return func(h *Handler) { h.positionStore = ps }
}

// WithRedis sets the Redis client for Pub/Sub signaling.
func WithRedis(rc *goredis.Client) HandlerOption {
	return func(h *Handler) { h.redis = rc }
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// registerRoutes attaches all endpoint handlers.
func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("GET /v1/health", h.handleHealth)
	h.mux.HandleFunc("GET /v1/exchanges", h.handleExchanges)
	h.mux.HandleFunc("GET /v1/tickers", h.handleListAllTickers)
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/markets", h.handleListMarkets)
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/markets/{symbol}", h.handleGetMarket)
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/tickers", h.handleListTickers)
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/tickers/{symbol}", h.handleGetTicker)
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/funding-rates", h.handleListFundingRates)
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/funding-rates/{symbol}", h.handleGetFundingRate)
	h.mux.HandleFunc("GET /v1/arbitrage/funding", h.handleListFundingArbitrage)
	h.mux.HandleFunc("POST /v1/arbitrage/funding/simulate", h.handleSimulateFundingArbitrage)
	h.mux.HandleFunc("GET /v1/arbitrage/spread", h.handleListSpreadArbitrage)
	h.mux.HandleFunc("POST /v1/arbitrage/spread/simulate", h.handleSimulateSpreadArbitrage)

	// Positions
	h.mux.HandleFunc("POST /v1/positions", h.handleCreatePosition)
	h.mux.HandleFunc("GET /v1/positions", h.handleListPositions)
	h.mux.HandleFunc("GET /v1/positions/{id}", h.handleGetPosition)
	h.mux.HandleFunc("POST /v1/positions/{id}/close", h.handleClosePosition)
	h.mux.HandleFunc("DELETE /v1/positions/{id}", h.handleDeletePosition)
	h.mux.HandleFunc("GET /v1/positions/{id}/orders", h.handleListPositionOrders)
	h.mux.HandleFunc("GET /v1/positions/{id}/funding", h.handleListPositionFunding)

	// WebSocket
	h.mux.HandleFunc("/v1/arbitrage/ws", h.handleArbitrageWS)

	// Admin
	h.mux.HandleFunc("DELETE /v1/flush", h.handleFlushAll)
}
