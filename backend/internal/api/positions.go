package api

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
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
	Symbol          string `json:"symbol"`
	LongExchangeID  string `json:"long_exchange_id"`
	ShortExchangeID string `json:"short_exchange_id"`
	SizeType        string `json:"size_type"`      // "price" or "quantity"
	RequestedSize   string `json:"requested_size"` // total position size
	PortionSize     string `json:"portion_size"`   // size per step (optional; 0 or empty = single-shot)
	PriceMin        string `json:"price_min"`      // min acceptable price (optional)
	PriceMax        string `json:"price_max"`      // max acceptable price (optional)
	SlippagePct     string `json:"slippage_pct"`   // max slippage % per portion (optional)
	Leverage        int    `json:"leverage"`       // per-position leverage (1, 2, 3, etc.)
	MarginCapital   string `json:"margin_capital"` // declared available collateral in USD (optional)

	// Spread arbitrage parameters (optional; only used when arbitrage_type == "spread").
	ArbitrageType     string `json:"arbitrage_type"`       // "funding" (default) or "spread"
	TargetSpreadPct   string `json:"target_spread_pct"`    // target convergence spread %
	StopLossSpreadPct string `json:"stop_loss_spread_pct"` // stop-loss spread %
	MaxHoldDuration   int64  `json:"max_hold_duration"`    // max hold time in seconds; 0 = no limit
}

// handleCreatePosition creates a new position and signals the trader.
func (h *Handler) handleCreatePosition(w http.ResponseWriter, r *http.Request) {
	if !h.requirePositionStore(w) {
		return
	}

	var req createPositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}

	// Validate required fields.
	if req.Symbol == "" || req.LongExchangeID == "" || req.ShortExchangeID == "" {
		writeErr(w, http.StatusBadRequest, "missing_field", "symbol, long_exchange_id, and short_exchange_id are required")
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
		Symbol:          req.Symbol,
		Status:          domain.PositionStatusPending,
		SizeType:        domain.SizeType(req.SizeType),
		RequestedSize:   size,
		ArbitrageType:   domain.ArbitrageTypeFunding,
		PortionSize:     portionSize,
		PriceMin:        priceMin,
		PriceMax:        priceMax,
		SlippagePct:     slippagePct,
		Leverage:        req.Leverage,
		MarginCapital:   marginCapital,
		StepsTotal:      stepsTotal,
		LongExchangeID:  req.LongExchangeID,
		ShortExchangeID: req.ShortExchangeID,
		Snapshot:        snapshot,
	}

	// Handle spread arbitrage type.
	if req.ArbitrageType == string(domain.ArbitrageTypeSpread) {
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

	// Signal the trader via Redis Pub/Sub.
	if h.redis != nil {
		h.redis.Publish(r.Context(), rediskey.PositionPending(), p.ID)
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

	f := domain.PositionFilter{
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

	p, err := h.positionStore.GetPosition(r.Context(), id)
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

	p, err := h.positionStore.GetPosition(r.Context(), id)
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
		h.redis.Publish(r.Context(), rediskey.PositionClosing(), p.ID)
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

	p, err := h.positionStore.GetPosition(r.Context(), id)
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

	if err := h.positionStore.DeletePosition(r.Context(), id); err != nil {
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

	orders, err := h.positionStore.ListOrdersByPosition(r.Context(), id)
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

	payments, err := h.positionStore.ListFundingPayments(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeOK(w, payments)
}

// snapshotArbitrage looks up the current FundingArbitrage opportunity for the given
// symbol and exchange pair. Returns the snapshot as JSON.
func (h *Handler) snapshotArbitrage(r *http.Request, symbol, longExch, shortExch string) (json.RawMessage, error) {
	arbs, err := h.store.ListFundingArbitrage(r.Context())
	if err != nil {
		return nil, err
	}

	for _, a := range arbs {
		if a.Symbol == symbol && a.Long.ExchangeID == longExch && a.Short.ExchangeID == shortExch {
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
	arb, err := h.store.GetSpreadArbitrage(r.Context(), symbol, buyExch, sellExch)
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
