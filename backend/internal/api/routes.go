package api

import (
	"net/http"

	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/store"
)

// Handler wires all API routes onto an http.ServeMux.
type Handler struct {
	mux      *http.ServeMux
	registry *exchange.Registry
	store    store.Store
}

// NewHandler creates a Handler and registers all routes.
func NewHandler(registry *exchange.Registry, st store.Store) *Handler {
	h := &Handler{
		mux:      http.NewServeMux(),
		registry: registry,
		store:    st,
	}
	h.registerRoutes()
	return h
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
	h.mux.HandleFunc("GET /v1/{exchange}/markets", h.handleListMarkets)
	h.mux.HandleFunc("GET /v1/{exchange}/markets/{symbol}", h.handleGetMarket)
	h.mux.HandleFunc("GET /v1/{exchange}/tickers", h.handleListTickers)
	h.mux.HandleFunc("GET /v1/{exchange}/tickers/{symbol}", h.handleGetTicker)
	h.mux.HandleFunc("GET /v1/{exchange}/funding-rates", h.handleListFundingRates)
	h.mux.HandleFunc("GET /v1/{exchange}/funding-rates/{symbol}", h.handleGetFundingRate)

	// Admin
	h.mux.HandleFunc("DELETE /v1/flush", h.handleFlushAll)
}
