package api

import (
	"encoding/json"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/arbitrage"
	"github.com/zeroclap/gembo/internal/domain"
)

// simulateRequest is the JSON body for the simulate endpoint.
type simulateRequest struct {
	Symbol          string `json:"symbol"`
	LongExchangeID  string `json:"long_exchange_id"`
	ShortExchangeID string `json:"short_exchange_id"`
	PositionSize    string `json:"position_size"`
	Quantity        string `json:"quantity"`
	SizeType        string `json:"size_type"` // "usd" (default) or "qty"
	SlippagePct     string `json:"slippage_pct"`
}

// handleSimulateFundingArbitrage runs a P&L simulation on a specific
// funding arbitrage opportunity using the caller-provided position size
// and slippage.
func (h *Handler) handleSimulateFundingArbitrage(w http.ResponseWriter, r *http.Request) {
	var req simulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Symbol == "" || req.LongExchangeID == "" || req.ShortExchangeID == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "symbol, long_exchange_id, and short_exchange_id are required")
		return
	}

	sizeType := domain.SizeType(req.SizeType)
	if sizeType == "" {
		sizeType = domain.SizeTypeUSD // default
	}
	if sizeType != domain.SizeTypeUSD && sizeType != domain.SizeTypeQty {
		writeErr(w, http.StatusBadRequest, "bad_request", "size_type must be \"usd\" or \"qty\"")
		return
	}

	slippagePct, err := decimal.NewFromString(req.SlippagePct)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "slippage_pct must be a valid number")
		return
	}

	simReq := domain.SimulationRequest{
		Symbol:          req.Symbol,
		LongExchangeID:  req.LongExchangeID,
		ShortExchangeID: req.ShortExchangeID,
		SizeType:        sizeType,
		SlippagePct:     slippagePct,
	}

	switch sizeType {
	case domain.SizeTypeQty:
		qty, qerr := decimal.NewFromString(req.Quantity)
		if qerr != nil || !qty.IsPositive() {
			writeErr(w, http.StatusBadRequest, "bad_request", "quantity must be a positive number when size_type is \"qty\"")
			return
		}
		simReq.Quantity = qty
	default:
		positionSize, perr := decimal.NewFromString(req.PositionSize)
		if perr != nil || !positionSize.IsPositive() {
			writeErr(w, http.StatusBadRequest, "bad_request", "position_size must be a positive number")
			return
		}
		simReq.PositionSize = positionSize
	}

	// Look up the arbitrage opportunity.
	arb, err := h.store.GetFundingArbitrage(r.Context(), req.Symbol, req.LongExchangeID, req.ShortExchangeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if arb == nil {
		writeErr(w, http.StatusNotFound, "not_found", "arbitrage opportunity not found")
		return
	}

	result := arbitrage.Simulate(*arb, simReq)

	writeOK(w, result)
}
