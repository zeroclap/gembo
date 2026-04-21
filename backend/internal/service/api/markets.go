package api

import (
	"net/http"
	"strings"
)

// handleHealth returns a simple 200 OK health check.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]string{"status": "ok"})
}

// handleExchanges lists all registered exchange IDs.
func (h *Handler) handleExchanges(w http.ResponseWriter, r *http.Request) {
	ids := h.registry.IDs()
	writeOK(w, ids)
}

// handleListMarkets lists all markets for the given exchange.
func (h *Handler) handleListMarkets(w http.ResponseWriter, r *http.Request) {
	exchangeID := r.PathValue("exchange")
	if _, err := h.registry.Get(exchangeID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"exchange "+exchangeID+" not found")
		return
	}

	markets, err := h.store.ListMarkets(r.Context(), exchangeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeOK(w, markets)
}

// handleGetMarket returns a single market by symbol.
func (h *Handler) handleGetMarket(w http.ResponseWriter, r *http.Request) {
	exchangeID := r.PathValue("exchange")
	symbol := decodeSymbol(r.PathValue("symbol"))

	if _, err := h.registry.Get(exchangeID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"exchange "+exchangeID+" not found")
		return
	}

	m, err := h.store.GetMarket(r.Context(), exchangeID, symbol)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if m == nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"symbol "+symbol+" not found")
		return
	}
	writeOK(w, m)
}

// handleListFundingRates lists all funding rates for the given exchange.
func (h *Handler) handleListFundingRates(w http.ResponseWriter, r *http.Request) {
	exchangeID := r.PathValue("exchange")
	if _, err := h.registry.Get(exchangeID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"exchange "+exchangeID+" not found")
		return
	}

	rates, err := h.store.ListFundingRates(r.Context(), exchangeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeOK(w, rates)
}

// handleGetFundingRate returns a single funding rate by symbol.
func (h *Handler) handleGetFundingRate(w http.ResponseWriter, r *http.Request) {
	exchangeID := r.PathValue("exchange")
	symbol := decodeSymbol(r.PathValue("symbol"))

	if _, err := h.registry.Get(exchangeID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"exchange "+exchangeID+" not found")
		return
	}

	fr, err := h.store.GetFundingRate(r.Context(), exchangeID, symbol)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if fr == nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"symbol "+symbol+" not found")
		return
	}
	writeOK(w, fr)
}

// decodeSymbol converts URL-safe symbol encoding back to the canonical form.
// Symbols are passed as URL path segments; "/" is encoded as "-" or ":" in
// the path by clients. We also handle %2F for clients that URL-encode it.
// Example: "BTC-USDT" or "BTCUSDT" → "BTC/USDT"
func decodeSymbol(s string) string {
	// Support both "BTC-USDT" and "BTC:USDT" variants that some clients send.
	// If the symbol already contains a "/" it is passed through as-is.
	if strings.Contains(s, "/") {
		return s
	}
	if strings.Contains(s, ":") {
		return strings.ReplaceAll(s, ":", "/")
	}
	return s
}
