package api

import (
	"net/http"
	"sort"

	"github.com/zeroclap/gembo/internal/domain"
)

// handleListAllBalances returns balances across all exchanges.
func (h *Handler) handleListAllBalances(w http.ResponseWriter, r *http.Request) {
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	exchangeIDs := h.registry.IDs()
	balances := make([]domain.Balance, 0)

	for _, exchangeID := range exchangeIDs {
		exchangeBalances, err := h.store.ListBalances(r.Context(), au.ID, exchangeID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		balances = append(balances, exchangeBalances...)
	}

	sort.Slice(balances, func(i, j int) bool {
		if balances[i].ExchangeID != balances[j].ExchangeID {
			return balances[i].ExchangeID < balances[j].ExchangeID
		}
		return balances[i].Currency < balances[j].Currency
	})

	writeOK(w, balances)
}

// handleListBalances returns all balances for a specific exchange.
func (h *Handler) handleListBalances(w http.ResponseWriter, r *http.Request) {
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	exchangeID := r.PathValue("exchange")
	if _, err := h.registry.Get(exchangeID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"exchange "+exchangeID+" not found")
		return
	}

	balances, err := h.store.ListBalances(r.Context(), au.ID, exchangeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	sort.Slice(balances, func(i, j int) bool {
		return balances[i].Currency < balances[j].Currency
	})

	writeOK(w, balances)
}
