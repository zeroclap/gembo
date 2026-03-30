package api

import (
	"net/http"
	"sort"

	"github.com/zeroclap/gembo/internal/domain"
)

// tickerRow groups tickers for a single symbol across all exchanges.
type tickerRow struct {
	Symbol  string          `json:"symbol"`
	Tickers []domain.Ticker `json:"tickers"`
}

// handleListAllTickers returns tickers across all exchanges, grouped by symbol.
func (h *Handler) handleListAllTickers(w http.ResponseWriter, r *http.Request) {
	exchangeIDs := h.registry.IDs()

	// symbol -> []Ticker
	grouped := make(map[string][]domain.Ticker)

	for _, eid := range exchangeIDs {
		tickers, err := h.store.ListTickers(r.Context(), eid)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		for _, t := range tickers {
			grouped[t.Symbol] = append(grouped[t.Symbol], t)
		}
	}

	rows := make([]tickerRow, 0, len(grouped))
	for sym, tickers := range grouped {
		rows = append(rows, tickerRow{Symbol: sym, Tickers: tickers})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Symbol < rows[j].Symbol
	})

	writeOK(w, rows)
}

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
