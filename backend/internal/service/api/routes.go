package api

import (
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/internal/store"
)

// Handler wires all API routes onto an http.ServeMux.
type Handler struct {
	mux           *http.ServeMux
	registry      *base.Registry
	store         store.Store
	positionStore store.PositionStore // nil if PostgreSQL is not configured.
	userStore     store.UserStore     // nil if PostgreSQL is not configured.
	redis         *goredis.Client     // for Pub/Sub signaling to the trader process.
	jwtSecret     string
	tokenTTL      time.Duration
}

// NewHandler creates a Handler and registers all routes.
func NewHandler(registry *base.Registry, st store.Store, opts ...HandlerOption) *Handler {
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

// WithUserStore sets the UserStore for authentication.
func WithUserStore(us store.UserStore) HandlerOption {
	return func(h *Handler) { h.userStore = us }
}

// WithRedis sets the Redis client for Pub/Sub signaling.
func WithRedis(rc *goredis.Client) HandlerOption {
	return func(h *Handler) { h.redis = rc }
}

// WithJWTSecret sets the HMAC secret used to sign and verify JWT tokens.
func WithJWTSecret(secret string) HandlerOption {
	return func(h *Handler) { h.jwtSecret = secret }
}

// WithTokenTTL sets the duration a JWT token remains valid.
func WithTokenTTL(ttl time.Duration) HandlerOption {
	return func(h *Handler) { h.tokenTTL = ttl }
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// registerRoutes attaches all endpoint handlers.
func (h *Handler) registerRoutes() {
	// Public routes — no authentication required.
	h.mux.HandleFunc("GET /v1/health", h.handleHealth)
	h.mux.HandleFunc("POST /v1/auth/login", h.handleLogin)

	// Protected routes.
	auth := h.requireAuth
	h.mux.HandleFunc("GET /v1/exchanges", auth(h.handleExchanges))
	h.mux.HandleFunc("GET /v1/balances", auth(h.handleListAllBalances))
	h.mux.HandleFunc("GET /v1/tickers", auth(h.handleListAllTickers))
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/markets", auth(h.handleListMarkets))
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/markets/{symbol}", auth(h.handleGetMarket))
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/balances", auth(h.handleListBalances))
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/tickers", auth(h.handleListTickers))
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/tickers/{symbol}", auth(h.handleGetTicker))
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/funding-rates", auth(h.handleListFundingRates))
	h.mux.HandleFunc("GET /v1/exchanges/{exchange}/funding-rates/{symbol}", auth(h.handleGetFundingRate))
	h.mux.HandleFunc("GET /v1/arbitrage/funding", auth(h.handleListFundingArbitrage))
	h.mux.HandleFunc("POST /v1/arbitrage/funding/simulate", auth(h.handleSimulateFundingArbitrage))
	h.mux.HandleFunc("GET /v1/arbitrage/spread", auth(h.handleListSpreadArbitrage))
	h.mux.HandleFunc("POST /v1/arbitrage/spread/simulate", auth(h.handleSimulateSpreadArbitrage))
	h.mux.HandleFunc("GET /v1/arbitrage/chart", auth(h.handleGetArbitrageChart))
	h.mux.HandleFunc("GET /v1/account", auth(h.handleGetAccount))
	h.mux.HandleFunc("GET /v1/account/credentials", auth(h.handleListExchangeCredentials))
	h.mux.HandleFunc("POST /v1/account/credentials", auth(h.handleCreateExchangeCredential))
	h.mux.HandleFunc("PUT /v1/account/credentials/{id}", auth(h.handleUpdateExchangeCredential))
	h.mux.HandleFunc("DELETE /v1/account/credentials/{id}", auth(h.handleDeleteExchangeCredential))
	h.mux.HandleFunc("GET /v1/account/workspaces", auth(h.handleGetWorkspaces))
	h.mux.HandleFunc("PUT /v1/account/workspaces", auth(h.handleSaveWorkspaces))

	// Positions
	h.mux.HandleFunc("POST /v1/positions/preflight-open", auth(h.handlePreflightOpenPosition))
	h.mux.HandleFunc("POST /v1/positions", auth(h.handleCreatePosition))
	h.mux.HandleFunc("GET /v1/positions", auth(h.handleListPositions))
	h.mux.HandleFunc("GET /v1/positions/{id}", auth(h.handleGetPosition))
	h.mux.HandleFunc("POST /v1/positions/{id}/close", auth(h.handleClosePosition))
	h.mux.HandleFunc("POST /v1/positions/{id}/size", auth(h.handleAdjustPositionSize))
	h.mux.HandleFunc("POST /v1/positions/{id}/margin", auth(h.handleTransferMargin))
	h.mux.HandleFunc("DELETE /v1/positions/{id}", auth(h.handleDeletePosition))
	h.mux.HandleFunc("GET /v1/positions/{id}/orders", auth(h.handleListPositionOrders))
	h.mux.HandleFunc("GET /v1/positions/{id}/funding", auth(h.handleListPositionFunding))

	// WebSocket — JWT validated inside the handler via query param or header.
	h.mux.HandleFunc("/v1/arbitrage/ws", auth(h.handleArbitrageWS))

	// Admin
	h.mux.HandleFunc("DELETE /v1/flush", auth(h.handleFlushAll))
}
