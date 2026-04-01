package okx

import (
	"context"
	"encoding/json"
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
)

// CreateMarketOrder places a market order on OKX.
func (c *Client) CreateMarketOrder(ctx context.Context, symbol, side string, amount float64) (*domain.ExchangeOrder, error) {
	order, err := c.api.CreateOrder(symbol, "market", side, amount)
	if err != nil {
		return nil, fmt.Errorf("okx create market order %s %s: %w", side, symbol, err)
	}
	return mapOrder(order, exchangeID), nil
}

// FetchOrder retrieves an order by its exchange-assigned ID.
func (c *Client) FetchOrder(ctx context.Context, orderID, symbol string) (*domain.ExchangeOrder, error) {
	order, err := c.api.FetchOrder(orderID, ccxt.WithFetchOrderSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("okx fetch order %s: %w", orderID, err)
	}
	return mapOrder(order, exchangeID), nil
}

// FetchPosition returns the current position for a symbol, or nil if none is open.
func (c *Client) FetchPosition(ctx context.Context, symbol string) (*exchange.ExchangePosition, error) {
	pos, err := c.api.FetchPosition(symbol)
	if err != nil {
		return nil, fmt.Errorf("okx fetch position %s: %w", symbol, err)
	}
	ep := mapPosition(pos)
	if ep.Contracts.IsZero() {
		return nil, nil
	}
	return ep, nil
}

// FetchBalance returns account balances.
func (c *Client) FetchBalance(ctx context.Context) ([]exchange.Balance, error) {
	balances, err := c.api.FetchBalance()
	if err != nil {
		return nil, fmt.Errorf("okx fetch balance: %w", err)
	}
	return mapBalances(balances), nil
}

// SetMarginMode sets the margin mode ("cross" or "isolated") for a symbol.
func (c *Client) SetMarginMode(ctx context.Context, symbol string, mode string, leverage int) error {
	_, err := c.api.SetMarginMode(
		mode,
		ccxt.WithSetMarginModeSymbol(symbol),
		ccxt.WithSetMarginModeParams(map[string]any{
			"leverage": leverage,
		}),
	)
	if err != nil {
		return fmt.Errorf("okx set margin mode %s %s: %w", symbol, mode, err)
	}
	return nil
}

// HasCredentials returns true if API credentials are configured.
func (c *Client) HasCredentials() bool {
	return c.apiKey != "" && c.secret != "" && c.passphrase != ""
}

// mapOrder converts a CCXT Order to a domain.ExchangeOrder.
func mapOrder(o ccxt.Order, exchID string) *domain.ExchangeOrder {
	var rawResp json.RawMessage
	if o.Info != nil {
		if b, err := json.Marshal(o.Info); err == nil {
			rawResp = b
		}
	}

	return &domain.ExchangeOrder{
		ExchangeID:      exchID,
		ExchangeOrderID: derefString(o.Id),
		Symbol:          derefString(o.Symbol),
		Side:            derefString(o.Side),
		RequestedQty:    floatToDecimal(o.Amount),
		FilledQty:       floatToDecimal(o.Filled),
		AveragePrice:    floatToDecimal(o.Average),
		Cost:            floatToDecimal(o.Cost),
		FeeCost:         floatToDecimal(o.Fee.Cost),
		Status:          derefString(o.Status),
		RawResponse:     rawResp,
	}
}

// mapPosition converts a CCXT Position to an exchange.ExchangePosition.
func mapPosition(p ccxt.Position) *exchange.ExchangePosition {
	return &exchange.ExchangePosition{
		Symbol:           derefString(p.Symbol),
		Side:             derefString(p.Side),
		Contracts:        floatToDecimal(p.Contracts),
		EntryPrice:       floatToDecimal(p.EntryPrice),
		MarkPrice:        floatToDecimal(p.MarkPrice),
		LiquidationPrice: floatToDecimal(p.LiquidationPrice),
		UnrealizedPnL:    floatToDecimal(p.UnrealizedPnl),
		Leverage:         floatToDecimal(p.Leverage),
		MarginMode:       derefString(p.MarginMode),
		Notional:         floatToDecimal(p.Notional),
	}
}

// mapBalances converts CCXT Balances to a slice of exchange.Balance.
func mapBalances(b ccxt.Balances) []exchange.Balance {
	var out []exchange.Balance
	for currency, bal := range b.Balances {
		out = append(out, exchange.Balance{
			Currency: currency,
			Free:     floatToDecimal(bal.Free),
			Used:     floatToDecimal(bal.Used),
			Total:    floatToDecimal(bal.Total),
		})
	}
	return out
}
