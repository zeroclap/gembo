package trader

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
)

var oneContractSize = decimal.NewFromInt(1)

func normalizeContractSize(cs decimal.Decimal) decimal.Decimal {
	if cs.LessThanOrEqual(decimal.Zero) {
		return oneContractSize
	}
	return cs
}

func contractsToBaseQty(contracts, contractSize decimal.Decimal) decimal.Decimal {
	return contracts.Mul(normalizeContractSize(contractSize))
}

func baseQtyToContracts(baseQty, contractSize decimal.Decimal) decimal.Decimal {
	cs := normalizeContractSize(contractSize)
	if cs.IsZero() {
		return decimal.Zero
	}
	return baseQty.Div(cs)
}

func (t *Trader) contractSizeFor(ctx context.Context, exchangeID, symbol string) (decimal.Decimal, error) {
	if t.marketStore == nil {
		return oneContractSize, nil
	}
	m, err := t.marketStore.GetMarket(ctx, exchangeID, symbol)
	if err != nil {
		return oneContractSize, fmt.Errorf("get market %s/%s: %w", exchangeID, symbol, err)
	}
	if m == nil {
		return oneContractSize, fmt.Errorf("market %s/%s not found", exchangeID, symbol)
	}
	return normalizeContractSize(m.ContractSize), nil
}

func (t *Trader) resolveLegContractSizes(ctx context.Context, p *domain.Position) (decimal.Decimal, decimal.Decimal, error) {
	longCS := normalizeContractSize(p.LongContractSize)
	shortCS := normalizeContractSize(p.ShortContractSize)

	if p.LongContractSize.GreaterThan(decimal.Zero) && p.ShortContractSize.GreaterThan(decimal.Zero) {
		return longCS, shortCS, nil
	}

	resolvedLongCS, err := t.contractSizeFor(ctx, p.LongExchangeID, p.Symbol)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("resolve long contract size: %w", err)
	}
	resolvedShortCS, err := t.contractSizeFor(ctx, p.ShortExchangeID, p.Symbol)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("resolve short contract size: %w", err)
	}

	return resolvedLongCS, resolvedShortCS, nil
}

func (t *Trader) resolveBaseQuantity(
	ctx context.Context,
	p *domain.Position,
	longExch, shortExch exchange.TradingExchange,
) (decimal.Decimal, error) {
	if p.RequestedBaseQty.GreaterThan(decimal.Zero) {
		return p.RequestedBaseQty, nil
	}

	if p.SizeType == domain.SizeTypeQuantity {
		return p.RequestedSize, nil
	}

	refPrice, err := t.referencePrice(ctx, p.Symbol, longExch, shortExch)
	if err != nil {
		return decimal.Zero, err
	}
	if refPrice.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("reference price must be positive")
	}

	return p.RequestedSize.Div(refPrice), nil
}
