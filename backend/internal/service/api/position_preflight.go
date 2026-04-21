package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

const defaultMinContracts = 1

type preflightOpenRequest struct {
	Symbol          string `json:"symbol"`
	LongExchangeID  string `json:"long_exchange_id"`
	ShortExchangeID string `json:"short_exchange_id"`
	SizeType        string `json:"size_type"`
	RequestedSize   string `json:"requested_size"`
	PortionSize     string `json:"portion_size"`
}

type openPreflightResponse struct {
	Symbol          string              `json:"symbol"`
	LongExchangeID  string              `json:"long_exchange_id"`
	ShortExchangeID string              `json:"short_exchange_id"`
	SizeType        domain.SizeType     `json:"size_type"`
	RequestedSize   decimal.Decimal     `json:"requested_size"`
	PortionSize     decimal.Decimal     `json:"portion_size"`
	Steps           int                 `json:"steps"`
	BaseQtyTotal    decimal.Decimal     `json:"base_qty_total"`
	BaseQtyStep     decimal.Decimal     `json:"base_qty_step"`
	RequiredBaseQty decimal.Decimal     `json:"required_base_qty"`
	Long            legOpenPreflight    `json:"long"`
	Short           legOpenPreflight    `json:"short"`
	RulesVersion    string              `json:"rules_version"`
	Errors          []openPreflightItem `json:"errors"`
	CanOpen         bool                `json:"can_open"`
}

type legOpenPreflight struct {
	ExchangeID string          `json:"exchange_id"`
	Price      decimal.Decimal `json:"price"`

	ContractSize decimal.Decimal `json:"contract_size"`
	MinContracts decimal.Decimal `json:"min_contracts"`
	MinCost      decimal.Decimal `json:"min_cost"`
	MinNotional  decimal.Decimal `json:"min_notional"`

	ContractsTotal decimal.Decimal `json:"contracts_total"`
	ContractsStep  decimal.Decimal `json:"contracts_step"`
	NotionalTotal  decimal.Decimal `json:"notional_total"`
	NotionalStep   decimal.Decimal `json:"notional_step"`

	RequiredContracts decimal.Decimal `json:"required_contracts"`
	RequiredBaseQty   decimal.Decimal `json:"required_base_qty"`
	RequiredNotional  decimal.Decimal `json:"required_notional"`
}

type openPreflightItem struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Exchange string `json:"exchange,omitempty"`
	Field    string `json:"field,omitempty"`
}

func (h *Handler) handlePreflightOpenPosition(w http.ResponseWriter, r *http.Request) {
	var req preflightOpenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}

	out, err := h.buildOpenPreflight(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "preflight_input", err.Error())
		return
	}

	writeOK(w, out)
}

func (h *Handler) buildOpenPreflight(ctx context.Context, req preflightOpenRequest) (*openPreflightResponse, error) {

	sizeType := domain.SizeType(req.SizeType)
	if sizeType != domain.SizeTypePrice && sizeType != domain.SizeTypeQuantity {
		return nil, fmt.Errorf("size_type must be 'price' or 'quantity'")
	}

	requestedSize, err := decimal.NewFromString(req.RequestedSize)
	if err != nil || requestedSize.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("requested_size must be a positive number")
	}

	portionSize := decimal.Zero
	if req.PortionSize != "" {
		portionSize, err = decimal.NewFromString(req.PortionSize)
		if err != nil || portionSize.LessThan(decimal.Zero) {
			return nil, fmt.Errorf("portion_size must be a non-negative number")
		}
		if portionSize.GreaterThan(decimal.Zero) && portionSize.GreaterThan(requestedSize) {
			return nil, fmt.Errorf("portion_size must not exceed requested_size")
		}
	}

	if req.Symbol == "" || req.LongExchangeID == "" || req.ShortExchangeID == "" {
		return nil, fmt.Errorf("symbol, long_exchange_id, and short_exchange_id are required")
	}

	steps := 1
	if portionSize.GreaterThan(decimal.Zero) {
		steps = int(requestedSize.Div(portionSize).Ceil().IntPart())
	}

	baseQtyTotal, baseQtyStep, err := h.preflightBaseQuantities(ctx, req.Symbol, req.LongExchangeID, req.ShortExchangeID, sizeType, requestedSize, portionSize)
	if err != nil {
		return nil, err
	}

	longLeg, err := h.preflightLeg(ctx, req.Symbol, req.LongExchangeID, baseQtyTotal, baseQtyStep)
	if err != nil {
		return nil, err
	}
	shortLeg, err := h.preflightLeg(ctx, req.Symbol, req.ShortExchangeID, baseQtyTotal, baseQtyStep)
	if err != nil {
		return nil, err
	}

	errs := validateLegOpen(longLeg)
	errs = append(errs, validateLegOpen(shortLeg)...)
	requiredBaseQty := longLeg.RequiredBaseQty
	if shortLeg.RequiredBaseQty.GreaterThan(requiredBaseQty) {
		requiredBaseQty = shortLeg.RequiredBaseQty
	}

	res := &openPreflightResponse{
		Symbol:          req.Symbol,
		LongExchangeID:  req.LongExchangeID,
		ShortExchangeID: req.ShortExchangeID,
		SizeType:        sizeType,
		RequestedSize:   requestedSize,
		PortionSize:     portionSize,
		Steps:           steps,
		BaseQtyTotal:    baseQtyTotal,
		BaseQtyStep:     baseQtyStep,
		RequiredBaseQty: requiredBaseQty,
		Long:            *longLeg,
		Short:           *shortLeg,
		RulesVersion:    "market-metadata-v2",
		Errors:          errs,
		CanOpen:         len(errs) == 0,
	}

	return res, nil
}

func (h *Handler) preflightBaseQuantities(
	ctx context.Context,
	symbol, longExchangeID, shortExchangeID string,
	sizeType domain.SizeType,
	requestedSize, portionSize decimal.Decimal,
) (decimal.Decimal, decimal.Decimal, error) {
	if sizeType == domain.SizeTypeQuantity {
		stepQty := requestedSize
		if portionSize.GreaterThan(decimal.Zero) {
			stepQty = portionSize
		}
		return requestedSize, stepQty, nil
	}

	longTicker, err := h.store.GetTicker(ctx, longExchangeID, symbol)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("load long ticker: %w", err)
	}
	shortTicker, err := h.store.GetTicker(ctx, shortExchangeID, symbol)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("load short ticker: %w", err)
	}
	if longTicker == nil && shortTicker == nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("ticker not found for %s on both exchanges", symbol)
	}

	refPrice := referencePriceFromTickers(longTicker, shortTicker)
	if refPrice.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, fmt.Errorf("reference price must be positive")
	}

	totalBaseQty := requestedSize.Div(refPrice)
	stepQuote := requestedSize
	if portionSize.GreaterThan(decimal.Zero) {
		stepQuote = portionSize
	}
	stepBaseQty := stepQuote.Div(refPrice)

	return totalBaseQty, stepBaseQty, nil
}

func referencePriceFromTickers(longTicker, shortTicker *domain.Ticker) decimal.Decimal {
	longPrice := bestTickerPrice(longTicker)
	shortPrice := bestTickerPrice(shortTicker)

	switch {
	case longPrice.GreaterThan(decimal.Zero) && shortPrice.GreaterThan(decimal.Zero):
		return longPrice.Add(shortPrice).Div(decimal.NewFromInt(2))
	case longPrice.GreaterThan(decimal.Zero):
		return longPrice
	default:
		return shortPrice
	}
}

func bestTickerPrice(t *domain.Ticker) decimal.Decimal {
	if t == nil {
		return decimal.Zero
	}
	if t.Price.GreaterThan(decimal.Zero) {
		return t.Price
	}
	if t.Ask.GreaterThan(decimal.Zero) {
		return t.Ask
	}
	if t.Bid.GreaterThan(decimal.Zero) {
		return t.Bid
	}
	return decimal.Zero
}

func (h *Handler) preflightLeg(
	ctx context.Context,
	symbol, exchangeID string,
	baseQtyTotal, baseQtyStep decimal.Decimal,
) (*legOpenPreflight, error) {
	m, err := h.store.GetMarket(ctx, exchangeID, symbol)
	if err != nil {
		return nil, fmt.Errorf("load market %s/%s: %w", exchangeID, symbol, err)
	}
	if m == nil {
		return nil, fmt.Errorf("market %s/%s not found", exchangeID, symbol)
	}
	t, err := h.store.GetTicker(ctx, exchangeID, symbol)
	if err != nil {
		return nil, fmt.Errorf("load ticker %s/%s: %w", exchangeID, symbol, err)
	}
	if t == nil {
		return nil, fmt.Errorf("ticker %s/%s not found", exchangeID, symbol)
	}

	price := bestTickerPrice(t)
	if price.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("ticker %s/%s has no positive price", exchangeID, symbol)
	}

	contractSize := m.ContractSize
	if contractSize.LessThanOrEqual(decimal.Zero) {
		contractSize = decimal.NewFromInt(1)
	}

	contractsTotal := baseQtyTotal.Div(contractSize)
	contractsStep := baseQtyStep.Div(contractSize)
	notionalTotal := baseQtyTotal.Mul(price)
	notionalStep := baseQtyStep.Mul(price)

	minContracts := m.MinAmount
	if minContracts.LessThanOrEqual(decimal.Zero) {
		minContracts = decimal.NewFromInt(defaultMinContracts)
	}

	requiredContracts, requiredNotional, requiredBaseQty := deriveOrderPlanForLeg(
		baseQtyStep,
		contractSize,
		price,
		minContracts,
		m.MinCost,
		m.MinNotional,
	)

	return &legOpenPreflight{
		ExchangeID:        exchangeID,
		Price:             price,
		ContractSize:      contractSize,
		MinContracts:      minContracts,
		MinCost:           m.MinCost,
		MinNotional:       m.MinNotional,
		ContractsTotal:    contractsTotal,
		ContractsStep:     contractsStep,
		NotionalTotal:     notionalTotal,
		NotionalStep:      notionalStep,
		RequiredContracts: requiredContracts,
		RequiredBaseQty:   requiredBaseQty,
		RequiredNotional:  requiredNotional,
	}, nil
}

func validateLegOpen(leg *legOpenPreflight) []openPreflightItem {
	errOut := make([]openPreflightItem, 0, 4)

	if leg.ContractsStep.LessThan(leg.MinContracts) {
		errOut = append(errOut, openPreflightItem{
			Code:     "min_contracts",
			Exchange: leg.ExchangeID,
			Field:    "contracts_step",
			Message:  fmt.Sprintf("step contracts %s is below minimum %s", leg.ContractsStep, leg.MinContracts),
		})
	}

	if leg.MinCost.GreaterThan(decimal.Zero) && leg.NotionalStep.LessThan(leg.MinCost) {
		errOut = append(errOut, openPreflightItem{
			Code:     "min_cost",
			Exchange: leg.ExchangeID,
			Field:    "notional_step",
			Message:  fmt.Sprintf("step notional %s is below minimum cost %s", leg.NotionalStep, leg.MinCost),
		})
	}

	if leg.MinNotional.GreaterThan(decimal.Zero) && leg.NotionalStep.LessThan(leg.MinNotional) {
		errOut = append(errOut, openPreflightItem{
			Code:     "min_notional",
			Exchange: leg.ExchangeID,
			Field:    "notional_step",
			Message:  fmt.Sprintf("step notional %s is below minimum notional %s", leg.NotionalStep, leg.MinNotional),
		})
	}

	return errOut
}

func deriveOrderPlanForLeg(
	baseQtyTarget decimal.Decimal,
	contractSize decimal.Decimal,
	price decimal.Decimal,
	minContracts decimal.Decimal,
	minCost decimal.Decimal,
	minNotional decimal.Decimal,
) (contracts decimal.Decimal, notional decimal.Decimal, adjustedBaseQty decimal.Decimal) {
	if contractSize.LessThanOrEqual(decimal.Zero) {
		contractSize = decimal.NewFromInt(1)
	}

	contracts = baseQtyTarget.Div(contractSize)
	if contracts.LessThan(minContracts) {
		contracts = minContracts
	}

	if price.GreaterThan(decimal.Zero) && minCost.GreaterThan(decimal.Zero) {
		reqContractsByCost := minCost.Div(price).Div(contractSize).Ceil()
		if reqContractsByCost.GreaterThan(contracts) {
			contracts = reqContractsByCost
		}
	}

	if price.GreaterThan(decimal.Zero) && minNotional.GreaterThan(decimal.Zero) {
		reqContractsByNotional := minNotional.Div(price).Div(contractSize).Ceil()
		if reqContractsByNotional.GreaterThan(contracts) {
			contracts = reqContractsByNotional
		}
	}

	adjustedBaseQty = contracts.Mul(contractSize)
	notional = adjustedBaseQty.Mul(price)
	return contracts, notional, adjustedBaseQty
}
