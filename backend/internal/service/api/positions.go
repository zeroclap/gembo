package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// requirePositionStore returns true if the position store is available.
// If not, it writes a 503 error and returns false.
func (h *Handler) requirePositionStore(w http.ResponseWriter) bool {
	if h.positionStore == nil {
		writeErr(w, http.StatusServiceUnavailable, "positions_unavailable",
			"position management requires a configured database")
		return false
	}
	return true
}

// createPositionRequest is the JSON body for POST /v1/positions.
type createPositionRequest struct {
	Symbol            string `json:"symbol"`
	LongExchangeID    string `json:"long_exchange_id"`
	LongCredentialID  string `json:"long_credential_id"`
	ShortExchangeID   string `json:"short_exchange_id"`
	ShortCredentialID string `json:"short_credential_id"`
	SizeType          string `json:"size_type"`      // "price" or "quantity"
	RequestedSize     string `json:"requested_size"` // total position size
	PortionSize       string `json:"portion_size"`   // size per step (optional; 0 or empty = single-shot)
	PriceMin          string `json:"price_min"`      // min acceptable price (optional)
	PriceMax          string `json:"price_max"`      // max acceptable price (optional)
	SlippagePct       string `json:"slippage_pct"`   // max slippage % per portion (optional)
	Leverage          int    `json:"leverage"`       // per-position leverage (1, 2, 3, etc.)
	MarginCapital     string `json:"margin_capital"` // declared available collateral in USD (optional)

	// Spread arbitrage parameters (optional; spread mode is inferred when target_spread_pct is set).
	TargetSpreadPct   string `json:"target_spread_pct"`    // target convergence spread %
	StopLossSpreadPct string `json:"stop_loss_spread_pct"` // stop-loss spread %
	MaxHoldDuration   int64  `json:"max_hold_duration"`    // max hold time in seconds; 0 = no limit
}

// handleCreatePosition creates a new position and signals the trader.
func (h *Handler) handleCreatePosition(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}
	if !h.requireUserStore(w) {
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var req createPositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}

	// Validate required fields.
	if req.Symbol == "" || req.LongExchangeID == "" || req.ShortExchangeID == "" || req.LongCredentialID == "" || req.ShortCredentialID == "" {
		writeErr(w, http.StatusBadRequest, "missing_field", "symbol, long_exchange_id, short_exchange_id, long_credential_id, and short_credential_id are required")
		return
	}
	if req.SizeType != string(domain.SizeTypePrice) && req.SizeType != string(domain.SizeTypeQuantity) {
		writeErr(w, http.StatusBadRequest, "invalid_size_type", "size_type must be 'price' or 'quantity'")
		return
	}
	size, err := decimal.NewFromString(req.RequestedSize)
	if err != nil || size.LessThanOrEqual(decimal.Zero) {
		writeErr(w, http.StatusBadRequest, "invalid_size", "requested_size must be a positive number")
		return
	}
	if req.LongExchangeID == req.ShortExchangeID {
		writeErr(w, http.StatusBadRequest, "same_exchange", "long and short exchanges must differ")
		return
	}
	if req.LongCredentialID == req.ShortCredentialID {
		writeErr(w, http.StatusBadRequest, "same_credential", "long and short credentials must differ")
		return
	}

	longCredential, err := h.userStore.GetExchangeCredentialByID(r.Context(), au.ID, req.LongCredentialID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to resolve long credential")
		return
	}
	if longCredential == nil {
		writeErr(w, http.StatusBadRequest, "invalid_credential", "long_credential_id was not found for this user")
		return
	}
	if longCredential.ExchangeID != req.LongExchangeID {
		writeErr(w, http.StatusBadRequest, "invalid_credential_exchange", "long_credential_id exchange does not match long_exchange_id")
		return
	}

	shortCredential, err := h.userStore.GetExchangeCredentialByID(r.Context(), au.ID, req.ShortCredentialID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to resolve short credential")
		return
	}
	if shortCredential == nil {
		writeErr(w, http.StatusBadRequest, "invalid_credential", "short_credential_id was not found for this user")
		return
	}
	if shortCredential.ExchangeID != req.ShortExchangeID {
		writeErr(w, http.StatusBadRequest, "invalid_credential_exchange", "short_credential_id exchange does not match short_exchange_id")
		return
	}

	// Validate leverage. Zero means "use global config fallback".
	if req.Leverage < 0 {
		writeErr(w, http.StatusBadRequest, "invalid_leverage", "leverage must be >= 0 (0 = use global default)")
		return
	}

	// Parse optional stepped execution parameters.
	var portionSize, priceMin, priceMax, slippagePct, marginCapital decimal.Decimal

	if req.PortionSize != "" {
		portionSize, err = decimal.NewFromString(req.PortionSize)
		if err != nil || portionSize.LessThan(decimal.Zero) {
			writeErr(w, http.StatusBadRequest, "invalid_portion_size", "portion_size must be a non-negative number")
			return
		}
		if portionSize.GreaterThan(decimal.Zero) && portionSize.GreaterThan(size) {
			writeErr(w, http.StatusBadRequest, "invalid_portion_size", "portion_size must not exceed requested_size")
			return
		}
	}

	if req.PriceMin != "" {
		priceMin, err = decimal.NewFromString(req.PriceMin)
		if err != nil || priceMin.LessThan(decimal.Zero) {
			writeErr(w, http.StatusBadRequest, "invalid_price_min", "price_min must be a non-negative number")
			return
		}
	}
	if req.PriceMax != "" {
		priceMax, err = decimal.NewFromString(req.PriceMax)
		if err != nil || priceMax.LessThanOrEqual(decimal.Zero) {
			writeErr(w, http.StatusBadRequest, "invalid_price_max", "price_max must be a positive number")
			return
		}
	}
	if !priceMin.IsZero() && !priceMax.IsZero() && priceMin.GreaterThanOrEqual(priceMax) {
		writeErr(w, http.StatusBadRequest, "invalid_price_range", "price_min must be less than price_max")
		return
	}

	if req.SlippagePct != "" {
		slippagePct, err = decimal.NewFromString(req.SlippagePct)
		if err != nil || slippagePct.LessThan(decimal.Zero) {
			writeErr(w, http.StatusBadRequest, "invalid_slippage_pct", "slippage_pct must be a non-negative number")
			return
		}
	}

	if req.MarginCapital != "" {
		marginCapital, err = decimal.NewFromString(req.MarginCapital)
		if err != nil || marginCapital.LessThan(decimal.Zero) {
			writeErr(w, http.StatusBadRequest, "invalid_margin_capital", "margin_capital must be a non-negative number")
			return
		}
	}

	// Compute steps total.
	var stepsTotal int
	if portionSize.GreaterThan(decimal.Zero) {
		// ceil(requested_size / portion_size)
		stepsTotal = int(math.Ceil(size.Div(portionSize).InexactFloat64()))
	}

	// Snapshot the current arbitrage opportunity for this symbol pair.
	snapshot, err := h.snapshotArbitrage(r, req.Symbol, req.LongExchangeID, req.ShortExchangeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "snapshot_failed", err.Error())
		return
	}

	p := &domain.Position{
		UserID:            au.ID,
		Symbol:            req.Symbol,
		Status:            domain.PositionStatusPending,
		SizeType:          domain.SizeType(req.SizeType),
		RequestedSize:     size,
		ArbitrageType:     domain.ArbitrageTypeFunding,
		PortionSize:       portionSize,
		PriceMin:          priceMin,
		PriceMax:          priceMax,
		SlippagePct:       slippagePct,
		Leverage:          req.Leverage,
		MarginCapital:     marginCapital,
		StepsTotal:        stepsTotal,
		LongExchangeID:    req.LongExchangeID,
		LongCredentialID:  req.LongCredentialID,
		ShortExchangeID:   req.ShortExchangeID,
		ShortCredentialID: req.ShortCredentialID,
		Snapshot:          snapshot,
	}

	// Handle spread arbitrage type — inferred when target_spread_pct or stop_loss_spread_pct is set.
	if req.TargetSpreadPct != "" || req.StopLossSpreadPct != "" {
		p.ArbitrageType = domain.ArbitrageTypeSpread

		if req.TargetSpreadPct != "" {
			targetSpread, terr := decimal.NewFromString(req.TargetSpreadPct)
			if terr != nil {
				writeErr(w, http.StatusBadRequest, "invalid_target_spread_pct",
					"target_spread_pct must be a valid number")
				return
			}
			p.TargetSpreadPct = targetSpread
		}
		if req.StopLossSpreadPct != "" {
			slSpread, serr := decimal.NewFromString(req.StopLossSpreadPct)
			if serr != nil {
				writeErr(w, http.StatusBadRequest, "invalid_stop_loss_spread_pct",
					"stop_loss_spread_pct must be a valid number")
				return
			}
			p.StopLossSpreadPct = slSpread
		}
		p.MaxHoldDuration = req.MaxHoldDuration

		// Snapshot the current spread opportunity instead of funding.
		spreadSnap, serr := h.snapshotSpreadArbitrage(r, req.Symbol, req.LongExchangeID, req.ShortExchangeID)
		if serr == nil {
			p.Snapshot = spreadSnap
		}
	}

	if err := h.positionStore.CreatePosition(r.Context(), p); err != nil {
		writeErr(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}

	if h.store != nil {
		if lm, lerr := h.store.GetMarket(r.Context(), p.LongExchangeID, p.Symbol); lerr == nil && lm != nil {
			p.LongContractSize = lm.ContractSize
		}
		if sm, serr := h.store.GetMarket(r.Context(), p.ShortExchangeID, p.Symbol); serr == nil && sm != nil {
			p.ShortContractSize = sm.ContractSize
		}
		_ = h.positionStore.UpdatePosition(r.Context(), p)
	}

	// Signal the trader via Redis Pub/Sub.
	if h.redis != nil {
		h.redis.Publish(r.Context(), rediskey.UserPositionPending(au.ID), p.ID)
	}

	writeJSON(w, http.StatusCreated, response{
		Data:  p,
		Error: nil,
		TS:    nowTS(),
	})
}

// handleListPositions returns positions matching optional status and symbol filters.
func (h *Handler) handleListPositions(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	f := domain.PositionFilter{
		UserID: au.ID,
		Symbol: r.URL.Query().Get("symbol"),
	}
	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		for _, s := range strings.Split(statusParam, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				f.Status = append(f.Status, domain.PositionStatus(s))
			}
		}
	}

	positions, err := h.positionStore.ListPositions(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeOK(w, positions)
}

// handleGetPosition returns a single position by ID, enriched with live data.
func (h *Handler) handleGetPosition(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "position id is required")
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	p, err := h.positionStore.GetPosition(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if p == nil {
		writeErr(w, http.StatusNotFound, "not_found", "position not found")
		return
	}

	writeOK(w, p)
}

// handleClosePosition marks a position for closing and signals the trader.
func (h *Handler) handleClosePosition(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "position id is required")
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	p, err := h.positionStore.GetPosition(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if p == nil {
		writeErr(w, http.StatusNotFound, "not_found", "position not found")
		return
	}

	// Only open or partially_open positions can be closed.
	if p.Status != domain.PositionStatusOpen && p.Status != domain.PositionStatusPartiallyOpen {
		writeErr(w, http.StatusConflict, "invalid_status",
			"position must be 'open' or 'partially_open' to close, current: "+string(p.Status))
		return
	}

	p.Status = domain.PositionStatusClosing
	if err := h.positionStore.UpdatePosition(r.Context(), p); err != nil {
		writeErr(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	// Signal the trader via Redis Pub/Sub.
	if h.redis != nil {
		h.redis.Publish(r.Context(), rediskey.UserPositionClosing(au.ID), p.ID)
	}

	writeOK(w, p)
}

// handleDeletePosition deletes a pending position.
func (h *Handler) handleDeletePosition(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "position id is required")
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	p, err := h.positionStore.GetPosition(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if p == nil {
		writeErr(w, http.StatusNotFound, "not_found", "position not found")
		return
	}

	if p.Status != domain.PositionStatusPending {
		writeErr(w, http.StatusConflict, "invalid_status",
			"only pending positions can be deleted, current: "+string(p.Status))
		return
	}

	if err := h.positionStore.DeletePosition(r.Context(), au.ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListPositionOrders returns all orders for a position.
func (h *Handler) handleListPositionOrders(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "position id is required")
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	orders, err := h.positionStore.ListOrdersByPosition(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeOK(w, orders)
}

// handleListPositionFunding returns all funding payments for a position.
func (h *Handler) handleListPositionFunding(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "position id is required")
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	payments, err := h.positionStore.ListFundingPayments(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeOK(w, payments)
}

// transferMarginRequest is the JSON body for POST /v1/positions/{id}/margin.
type transferMarginRequest struct {
	Leg    string `json:"leg"`    // "long" or "short"
	Amount string `json:"amount"` // decimal string; positive = add, negative = remove
}

type adjustPositionSizeRequest struct {
	Action       string `json:"action"`         // "increase" or "reduce"
	DeltaBaseQty string `json:"delta_base_qty"` // positive decimal in base asset units
}

// handleAdjustPositionSize increases or reduces an open position by base quantity.
func (h *Handler) handleAdjustPositionSize(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}
	if !h.requireUserStore(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "position id is required")
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	p, err := h.positionStore.GetPosition(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if p == nil {
		writeErr(w, http.StatusNotFound, "not_found", "position not found")
		return
	}

	if p.Status != domain.PositionStatusOpen && p.Status != domain.PositionStatusPartiallyOpen {
		writeErr(w, http.StatusConflict, "invalid_status",
			"position must be 'open' or 'partially_open' to adjust size, current: "+string(p.Status))
		return
	}

	var req adjustPositionSizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}
	if req.Action != "increase" && req.Action != "reduce" {
		writeErr(w, http.StatusBadRequest, "invalid_action", "action must be 'increase' or 'reduce'")
		return
	}

	deltaBaseQty, err := decimal.NewFromString(req.DeltaBaseQty)
	if err != nil || deltaBaseQty.LessThanOrEqual(decimal.Zero) {
		writeErr(w, http.StatusBadRequest, "invalid_delta_base_qty", "delta_base_qty must be a positive decimal number")
		return
	}

	if p.LongContractSize.LessThanOrEqual(decimal.Zero) || p.ShortContractSize.LessThanOrEqual(decimal.Zero) {
		writeErr(w, http.StatusConflict, "invalid_contract_size", "position contract size is not available for one or both legs")
		return
	}

	if req.Action == "reduce" {
		maxReducible := p.LongFilledBaseQty
		if p.ShortFilledBaseQty.LessThan(maxReducible) {
			maxReducible = p.ShortFilledBaseQty
		}
		if deltaBaseQty.GreaterThan(maxReducible) {
			writeErr(w, http.StatusBadRequest, "reduce_exceeds_position", "delta_base_qty exceeds current open base quantity")
			return
		}
	}

	longCred, err := h.userStore.GetExchangeCredentialByID(r.Context(), au.ID, p.LongCredentialID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to resolve long credential")
		return
	}
	if longCred == nil {
		writeErr(w, http.StatusBadRequest, "invalid_credential", "long credential not found for this user")
		return
	}
	if longCred.ExchangeID != p.LongExchangeID {
		writeErr(w, http.StatusBadRequest, "invalid_credential_exchange", "long credential exchange does not match long leg exchange")
		return
	}

	shortCred, err := h.userStore.GetExchangeCredentialByID(r.Context(), au.ID, p.ShortCredentialID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to resolve short credential")
		return
	}
	if shortCred == nil {
		writeErr(w, http.StatusBadRequest, "invalid_credential", "short credential not found for this user")
		return
	}
	if shortCred.ExchangeID != p.ShortExchangeID {
		writeErr(w, http.StatusBadRequest, "invalid_credential_exchange", "short credential exchange does not match short leg exchange")
		return
	}

	longEx, err := exchanges.NewPrivateExchangeFromCredential(slog.Default(), *longCred)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "unsupported_exchange", err.Error())
		return
	}
	shortEx, err := exchanges.NewPrivateExchangeFromCredential(slog.Default(), *shortCred)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "unsupported_exchange", err.Error())
		return
	}

	longContracts := deltaBaseQty.Div(p.LongContractSize)
	shortContracts := deltaBaseQty.Div(p.ShortContractSize)
	longContractsF64, _ := longContracts.Float64()
	shortContractsF64, _ := shortContracts.Float64()

	if longContractsF64 <= 0 || shortContractsF64 <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid_contract_amount", "calculated contract amount must be positive on both legs")
		return
	}

	if req.Action == "increase" {
		if _, err := longEx.CreateMarketOrder(r.Context(), p.Symbol, domain.OrderSideBuy, longContractsF64); err != nil {
			writeErr(w, http.StatusBadGateway, "exchange_error", "long leg increase failed: "+err.Error())
			return
		}
		if _, err := shortEx.CreateMarketOrder(r.Context(), p.Symbol, domain.OrderSideSell, shortContractsF64); err != nil {
			_, _ = longEx.CloseMarketOrder(r.Context(), p.Symbol, domain.OrderSideSell, longContractsF64)
			writeErr(w, http.StatusBadGateway, "exchange_error", "short leg increase failed: "+err.Error())
			return
		}
		p.RequestedBaseQty = p.RequestedBaseQty.Add(deltaBaseQty)
		if p.SizeType == domain.SizeTypeQuantity {
			p.RequestedSize = p.RequestedSize.Add(deltaBaseQty)
		}
	} else {
		if _, err := longEx.CloseMarketOrder(r.Context(), p.Symbol, domain.OrderSideSell, longContractsF64); err != nil {
			writeErr(w, http.StatusBadGateway, "exchange_error", "long leg reduce failed: "+err.Error())
			return
		}
		if _, err := shortEx.CloseMarketOrder(r.Context(), p.Symbol, domain.OrderSideBuy, shortContractsF64); err != nil {
			_, _ = longEx.CreateMarketOrder(r.Context(), p.Symbol, domain.OrderSideBuy, longContractsF64)
			writeErr(w, http.StatusBadGateway, "exchange_error", "short leg reduce failed: "+err.Error())
			return
		}
		p.RequestedBaseQty = p.RequestedBaseQty.Sub(deltaBaseQty)
		if p.RequestedBaseQty.LessThan(decimal.Zero) {
			p.RequestedBaseQty = decimal.Zero
		}
		if p.SizeType == domain.SizeTypeQuantity {
			p.RequestedSize = p.RequestedSize.Sub(deltaBaseQty)
			if p.RequestedSize.LessThan(decimal.Zero) {
				p.RequestedSize = decimal.Zero
			}
		}
	}

	if err := h.positionStore.UpdatePosition(r.Context(), p); err != nil {
		writeErr(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	writeOK(w, p)
}

// handleTransferMargin adjusts the isolated margin on one leg of an open position.
func (h *Handler) handleTransferMargin(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}
	if !h.requireUserStore(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "position id is required")
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	p, err := h.positionStore.GetPosition(r.Context(), au.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if p == nil {
		writeErr(w, http.StatusNotFound, "not_found", "position not found")
		return
	}

	// Only open or partially-open positions can have margin adjusted.
	if p.Status != domain.PositionStatusOpen && p.Status != domain.PositionStatusPartiallyOpen {
		writeErr(w, http.StatusConflict, "invalid_status",
			"position must be 'open' or 'partially_open' to adjust margin, current: "+string(p.Status))
		return
	}

	var req transferMarginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}
	if req.Leg != "long" && req.Leg != "short" {
		writeErr(w, http.StatusBadRequest, "invalid_leg", "leg must be 'long' or 'short'")
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.IsZero() {
		writeErr(w, http.StatusBadRequest, "invalid_amount", "amount must be a non-zero decimal number")
		return
	}

	// Resolve the credential and exchange for the chosen leg.
	var exchangeID, credentialID string
	var side domain.PositionSide
	if req.Leg == "long" {
		exchangeID = p.LongExchangeID
		credentialID = p.LongCredentialID
		side = domain.PositionSideLong
	} else {
		exchangeID = p.ShortExchangeID
		credentialID = p.ShortCredentialID
		side = domain.PositionSideShort
	}

	cred, err := h.userStore.GetExchangeCredentialByID(r.Context(), au.ID, credentialID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to resolve credential")
		return
	}
	if cred == nil {
		writeErr(w, http.StatusBadRequest, "invalid_credential", "credential not found for this user")
		return
	}
	if cred.ExchangeID != exchangeID {
		writeErr(w, http.StatusBadRequest, "invalid_credential_exchange", "credential exchange does not match position leg exchange")
		return
	}

	ex, err := exchanges.NewPrivateExchangeFromCredential(slog.Default(), *cred)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "unsupported_exchange", err.Error())
		return
	}

	amountF64, _ := amount.Float64()
	if err := ex.TransferIsolatedMargin(r.Context(), p.Symbol, side, amountF64); err != nil {
		writeErr(w, http.StatusBadGateway, "exchange_error", err.Error())
		return
	}

	writeOK(w, p)
}

// snapshotArbitrage looks up the current FundingArbitrage opportunity for the given
// symbol and exchange pair. Returns the snapshot as JSON.
func (h *Handler) snapshotArbitrage(r *http.Request, symbol, longExch, shortExch string) (json.RawMessage, error) {
	arbs, err := h.store.ListArbitrage(r.Context())
	if err != nil {
		return nil, err
	}

	for _, a := range arbs {
		if a.Type != domain.ArbitrageTypeFunding || a.Funding == nil {
			continue
		}
		if a.Symbol == symbol && a.Funding.Long.ExchangeID == longExch && a.Funding.Short.ExchangeID == shortExch {
			b, err := json.Marshal(a)
			if err != nil {
				return nil, err
			}
			return json.RawMessage(b), nil
		}
	}

	// No matching arbitrage found — store an empty snapshot.
	return json.RawMessage("{}"), nil
}

// snapshotSpreadArbitrage looks up the current SpreadArbitrage opportunity for
// the given symbol and exchange pair. Returns the snapshot as JSON.
func (h *Handler) snapshotSpreadArbitrage(r *http.Request, symbol, buyExch, sellExch string) (json.RawMessage, error) {
	arb, err := h.store.GetArbitrage(r.Context(), domain.ArbitrageTypeSpread, symbol, buyExch, sellExch)
	if err != nil {
		return nil, err
	}
	if arb == nil {
		return json.RawMessage("{}"), nil
	}

	b, err := json.Marshal(arb)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
