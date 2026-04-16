package api

import (
	"net/http"
	"sort"

	"github.com/zeroclap/gembo/internal/arbitrage/recommend"
)

// handleListFundingArbitrage returns all pre-computed funding rate arbitrage
// opportunities, sorted by APR descending.
func (h *Handler) handleListFundingArbitrage(w http.ResponseWriter, r *http.Request) {
	arbs, err := h.store.ListFundingArbitrage(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	// Sort by APR descending (highest opportunity first).
	sort.Slice(arbs, func(i, j int) bool {
		return arbs[i].APR.GreaterThan(arbs[j].APR)
	})

	// Flag recommended opportunities.
	recommend.Default().Annotate(arbs)

	writeOK(w, arbs)
}
