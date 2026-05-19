package api

import (
	"net/http"
	"sort"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/pkg/arbitrage/recommend"
)

// handleListFundingArbitrage returns all pre-computed funding rate arbitrage
// opportunities, sorted by APR descending.
func (h *Handler) handleListFundingArbitrage(w http.ResponseWriter, r *http.Request) {
	all, err := h.store.ListArbitrage(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	arbs := make([]domain.Arbitrage, 0, len(all))
	for _, a := range all {
		if a.Type == domain.ArbitrageTypeFunding {
			arbs = append(arbs, a)
		}
	}

	// Sort by APR descending (highest opportunity first).
	sort.Slice(arbs, func(i, j int) bool {
		if arbs[i].Funding == nil || arbs[j].Funding == nil {
			return false
		}
		return arbs[i].Funding.APR.GreaterThan(arbs[j].Funding.APR)
	})

	// Flag recommended opportunities.
	recommend.Default().Annotate(arbs)

	writeOK(w, arbs)
}
