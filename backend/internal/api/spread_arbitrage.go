package api

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/arbitrage"
	"github.com/zeroclap/gembo/internal/domain"
)

// handleListSpreadArbitrage returns all pre-computed spread arbitrage
// opportunities, sorted by net spread percentage descending.
func (h *Handler) handleListSpreadArbitrage(w http.ResponseWriter, r *http.Request) {
	arbs, err := h.store.ListSpreadArbitrage(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	sort.Slice(arbs, func(i, j int) bool {
		return arbs[i].NetSpreadPct.GreaterThan(arbs[j].NetSpreadPct)
	})

	writeOK(w, arbs)
}

// spreadSimulateRequest is the JSON body for POST /v1/arbitrage/spread/simulate.
type spreadSimulateRequest struct {
	Symbol          string `json:"symbol"`
	BuyExchangeID   string `json:"buy_exchange_id"`
	SellExchangeID  string `json:"sell_exchange_id"`
	PositionSize    string `json:"position_size"`
	TargetSpreadPct string `json:"target_spread_pct"`
	SlippagePct     string `json:"slippage_pct"`
}

// handleSimulateSpreadArbitrage runs a P&L simulation on a specific spread
// arbitrage opportunity.
func (h *Handler) handleSimulateSpreadArbitrage(w http.ResponseWriter, r *http.Request) {
	var req spreadSimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Symbol == "" || req.BuyExchangeID == "" || req.SellExchangeID == "" {
		writeErr(w, http.StatusBadRequest, "bad_request",
			"symbol, buy_exchange_id, and sell_exchange_id are required")
		return
	}

	positionSize, err := decimal.NewFromString(req.PositionSize)
	if err != nil || !positionSize.IsPositive() {
		writeErr(w, http.StatusBadRequest, "bad_request",
			"position_size must be a positive number")
		return
	}

	targetSpreadPct, err := decimal.NewFromString(req.TargetSpreadPct)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request",
			"target_spread_pct must be a valid number")
		return
	}

	slippagePct, err := decimal.NewFromString(req.SlippagePct)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request",
			"slippage_pct must be a valid number")
		return
	}

	// Look up the spread arbitrage opportunity.
	arb, err := h.store.GetSpreadArbitrage(r.Context(),
		req.Symbol, req.BuyExchangeID, req.SellExchangeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if arb == nil {
		writeErr(w, http.StatusNotFound, "not_found",
			"spread arbitrage opportunity not found")
		return
	}

	simReq := domain.SpreadSimulationRequest{
		Symbol:          req.Symbol,
		BuyExchangeID:   req.BuyExchangeID,
		SellExchangeID:  req.SellExchangeID,
		PositionSize:    positionSize,
		TargetSpreadPct: targetSpreadPct,
		SlippagePct:     slippagePct,
	}

	result := arbitrage.SimulateSpread(*arb, simReq)
	writeOK(w, result)
}
