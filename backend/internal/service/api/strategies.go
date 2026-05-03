package api

import (
	"encoding/json"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// requireStrategyStore returns true if the strategy store is available.
func (h *Handler) requireStrategyStore(w http.ResponseWriter) bool {
	if h.strategyStore == nil {
		writeErr(w, http.StatusServiceUnavailable, "strategies_unavailable",
			"strategy endpoints require a configured database")
		return false
	}
	return true
}

// createStrategyRequest is the JSON body for POST /v1/strategies.
type createStrategyRequest struct {
	Name          string            `json:"name"`
	Enabled       bool              `json:"enabled"`
	ArbitrageType string            `json:"arbitrage_type"` // "funding" or "spread"
	MinAPR        string            `json:"min_apr"`
	MinNetSpread  string            `json:"min_net_spread_pct"`
	SymbolWL      []string          `json:"symbol_whitelist"`
	ExchangeWL    []string          `json:"exchange_whitelist"`
	Credentials   map[string]string `json:"credentials"` // {exchange_id: credential_id}
	SizeUSD       string            `json:"size_usd"`
	Leverage      int               `json:"leverage"`
	MaxConcurrent int               `json:"max_concurrent_positions"`
	// Spread params
	TargetSpreadPct   string `json:"target_spread_pct"`
	StopLossSpreadPct string `json:"stop_loss_spread_pct"`
	MaxHoldDuration   int64  `json:"max_hold_duration"`
}

// updateStrategyRequest is the JSON body for PATCH /v1/strategies/{id}.
// All fields are optional — only non-zero values are applied.
type updateStrategyRequest struct {
	Name          *string           `json:"name"`
	Enabled       *bool             `json:"enabled"`
	ArbitrageType *string           `json:"arbitrage_type"`
	MinAPR        *string           `json:"min_apr"`
	MinNetSpread  *string           `json:"min_net_spread_pct"`
	SymbolWL      []string          `json:"symbol_whitelist"`
	ExchangeWL    []string          `json:"exchange_whitelist"`
	Credentials   map[string]string `json:"credentials"`
	SizeUSD       *string           `json:"size_usd"`
	Leverage      *int              `json:"leverage"`
	MaxConcurrent *int              `json:"max_concurrent_positions"`
	// Spread params
	TargetSpreadPct   *string `json:"target_spread_pct"`
	StopLossSpreadPct *string `json:"stop_loss_spread_pct"`
	MaxHoldDuration   *int64  `json:"max_hold_duration"`
}

func (h *Handler) handleListStrategies(w http.ResponseWriter, r *http.Request) {
	if !h.requireStrategyStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}
	strategies, err := h.strategyStore.ListStrategies(r.Context(), au.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to list strategies")
		return
	}
	writeOK(w, strategies)
}

func (h *Handler) handleGetStrategy(w http.ResponseWriter, r *http.Request) {
	if !h.requireStrategyStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	st, err := h.strategyStore.GetStrategy(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to get strategy")
		return
	}
	if st == nil {
		writeErr(w, http.StatusNotFound, "not_found", "strategy not found")
		return
	}
	writeOK(w, st)
}

func (h *Handler) handleCreateStrategy(w http.ResponseWriter, r *http.Request) {
	if !h.requireStrategyStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var req createStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}

	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "missing_field", "name is required")
		return
	}
	if req.ArbitrageType != string(domain.ArbitrageTypeFunding) && req.ArbitrageType != string(domain.ArbitrageTypeSpread) {
		writeErr(w, http.StatusBadRequest, "invalid_arbitrage_type", "arbitrage_type must be 'funding' or 'spread'")
		return
	}

	sizeUSD, err := decimal.NewFromString(req.SizeUSD)
	if err != nil || sizeUSD.LessThanOrEqual(decimal.Zero) {
		writeErr(w, http.StatusBadRequest, "invalid_size_usd", "size_usd must be a positive number")
		return
	}

	if req.Leverage < 0 {
		writeErr(w, http.StatusBadRequest, "invalid_leverage", "leverage must be >= 0")
		return
	}
	if req.Leverage == 0 {
		req.Leverage = 1
	}
	maxConcurrent := req.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	st := &domain.Strategy{
		UserID:                 au.ID,
		Name:                   req.Name,
		Enabled:                req.Enabled,
		ArbitrageType:          domain.ArbitrageType(req.ArbitrageType),
		SymbolWhitelist:        req.SymbolWL,
		ExchangeWhitelist:      req.ExchangeWL,
		Credentials:            req.Credentials,
		SizeUSD:                sizeUSD,
		Leverage:               req.Leverage,
		MaxConcurrentPositions: maxConcurrent,
		MaxHoldDuration:        req.MaxHoldDuration,
	}

	if req.MinAPR != "" {
		if st.MinAPR, err = decimal.NewFromString(req.MinAPR); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_min_apr", "min_apr must be a valid number")
			return
		}
	}
	if req.MinNetSpread != "" {
		if st.MinNetSpreadPct, err = decimal.NewFromString(req.MinNetSpread); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_min_net_spread_pct", "min_net_spread_pct must be a valid number")
			return
		}
	}
	if req.TargetSpreadPct != "" {
		if st.TargetSpreadPct, err = decimal.NewFromString(req.TargetSpreadPct); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_target_spread_pct", "target_spread_pct must be a valid number")
			return
		}
	}
	if req.StopLossSpreadPct != "" {
		if st.StopLossSpreadPct, err = decimal.NewFromString(req.StopLossSpreadPct); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_stop_loss_spread_pct", "stop_loss_spread_pct must be a valid number")
			return
		}
	}

	if st.Credentials == nil {
		st.Credentials = make(map[string]string)
	}
	if st.SymbolWhitelist == nil {
		st.SymbolWhitelist = []string{}
	}
	if st.ExchangeWhitelist == nil {
		st.ExchangeWhitelist = []string{}
	}

	if err := h.strategyStore.CreateStrategy(r.Context(), st); err != nil {
		writeErr(w, http.StatusInternalServerError, "create_failed", "failed to create strategy")
		return
	}
	writeJSON(w, http.StatusCreated, response{Data: st, TS: nowTS()})
}

func (h *Handler) handleUpdateStrategy(w http.ResponseWriter, r *http.Request) {
	if !h.requireStrategyStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	st, err := h.strategyStore.GetStrategy(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to fetch strategy")
		return
	}
	if st == nil {
		writeErr(w, http.StatusNotFound, "not_found", "strategy not found")
		return
	}

	var req updateStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}

	// Apply partial updates.
	if req.Name != nil {
		st.Name = *req.Name
	}
	if req.Enabled != nil {
		st.Enabled = *req.Enabled
	}
	if req.ArbitrageType != nil {
		if *req.ArbitrageType != string(domain.ArbitrageTypeFunding) && *req.ArbitrageType != string(domain.ArbitrageTypeSpread) {
			writeErr(w, http.StatusBadRequest, "invalid_arbitrage_type", "arbitrage_type must be 'funding' or 'spread'")
			return
		}
		st.ArbitrageType = domain.ArbitrageType(*req.ArbitrageType)
	}
	if req.MinAPR != nil {
		v, perr := decimal.NewFromString(*req.MinAPR)
		if perr != nil {
			writeErr(w, http.StatusBadRequest, "invalid_min_apr", "min_apr must be a valid number")
			return
		}
		st.MinAPR = v
	}
	if req.MinNetSpread != nil {
		v, perr := decimal.NewFromString(*req.MinNetSpread)
		if perr != nil {
			writeErr(w, http.StatusBadRequest, "invalid_min_net_spread_pct", "min_net_spread_pct must be a valid number")
			return
		}
		st.MinNetSpreadPct = v
	}
	if req.SymbolWL != nil {
		st.SymbolWhitelist = req.SymbolWL
	}
	if req.ExchangeWL != nil {
		st.ExchangeWhitelist = req.ExchangeWL
	}
	if req.Credentials != nil {
		st.Credentials = req.Credentials
	}
	if req.SizeUSD != nil {
		v, perr := decimal.NewFromString(*req.SizeUSD)
		if perr != nil || v.LessThanOrEqual(decimal.Zero) {
			writeErr(w, http.StatusBadRequest, "invalid_size_usd", "size_usd must be a positive number")
			return
		}
		st.SizeUSD = v
	}
	if req.Leverage != nil {
		if *req.Leverage < 1 {
			writeErr(w, http.StatusBadRequest, "invalid_leverage", "leverage must be >= 1")
			return
		}
		st.Leverage = *req.Leverage
	}
	if req.MaxConcurrent != nil {
		if *req.MaxConcurrent < 1 {
			writeErr(w, http.StatusBadRequest, "invalid_max_concurrent_positions", "max_concurrent_positions must be >= 1")
			return
		}
		st.MaxConcurrentPositions = *req.MaxConcurrent
	}
	if req.TargetSpreadPct != nil {
		v, perr := decimal.NewFromString(*req.TargetSpreadPct)
		if perr != nil {
			writeErr(w, http.StatusBadRequest, "invalid_target_spread_pct", "target_spread_pct must be a valid number")
			return
		}
		st.TargetSpreadPct = v
	}
	if req.StopLossSpreadPct != nil {
		v, perr := decimal.NewFromString(*req.StopLossSpreadPct)
		if perr != nil {
			writeErr(w, http.StatusBadRequest, "invalid_stop_loss_spread_pct", "stop_loss_spread_pct must be a valid number")
			return
		}
		st.StopLossSpreadPct = v
	}
	if req.MaxHoldDuration != nil {
		st.MaxHoldDuration = *req.MaxHoldDuration
	}

	found, err := h.strategyStore.UpdateStrategy(r.Context(), st)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "update_failed", "failed to update strategy")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "not_found", "strategy not found")
		return
	}
	writeOK(w, st)
}

func (h *Handler) handleDeleteStrategy(w http.ResponseWriter, r *http.Request) {
	if !h.requireStrategyStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	found, err := h.strategyStore.DeleteStrategy(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete_failed", "failed to delete strategy")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "not_found", "strategy not found")
		return
	}
	writeOK(w, map[string]string{"id": id})
}
