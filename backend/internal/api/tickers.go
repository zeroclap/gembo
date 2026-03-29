package api

import "net/http"

// handleListTickers returns a snapshot of all tickers for the given exchange.
func (h *Handler) handleListTickers(w http.ResponseWriter, r *http.Request) {
	exchangeID := r.PathValue("exchange")
	if _, err := h.registry.Get(exchangeID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"exchange "+exchangeID+" not found")
		return
	}

	tickers, err := h.store.ListTickers(r.Context(), exchangeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeOK(w, tickers)
}

// handleGetTicker returns a single ticker by exchange and symbol.
func (h *Handler) handleGetTicker(w http.ResponseWriter, r *http.Request) {
	exchangeID := r.PathValue("exchange")
	symbol := decodeSymbol(r.PathValue("symbol"))

	if _, err := h.registry.Get(exchangeID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"exchange "+exchangeID+" not found")
		return
	}

	t, err := h.store.GetTicker(r.Context(), exchangeID, symbol)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if t == nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"ticker for "+symbol+" not found")
		return
	}
	writeOK(w, t)
}
