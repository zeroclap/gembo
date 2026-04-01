package exchange

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// ExchangePosition represents the current state of a position on an exchange.
type ExchangePosition struct {
	Symbol           string          `json:"symbol"`
	Side             string          `json:"side"` // "long" or "short"
	Contracts        decimal.Decimal `json:"contracts"`
	EntryPrice       decimal.Decimal `json:"entry_price"`
	MarkPrice        decimal.Decimal `json:"mark_price"`
	LiquidationPrice decimal.Decimal `json:"liquidation_price"`
	UnrealizedPnL    decimal.Decimal `json:"unrealized_pnl"`
	Leverage         decimal.Decimal `json:"leverage"`
	MarginMode       string          `json:"margin_mode"`
	Notional         decimal.Decimal `json:"notional"`
}

// Balance represents an account balance for a single currency.
type Balance struct {
	Currency string          `json:"currency"`
	Free     decimal.Decimal `json:"free"`
	Used     decimal.Decimal `json:"used"`
	Total    decimal.Decimal `json:"total"`
}

// TradingExchange extends Exchange with order placement and position management.
// Requires authenticated API credentials.
type TradingExchange interface {
	Exchange

	// CreateMarketOrder places a market order on the exchange.
	// side is "buy" or "sell", amount is in base currency.
	CreateMarketOrder(ctx context.Context, symbol, side string, amount float64) (*domain.ExchangeOrder, error)

	// FetchOrder retrieves the current state of an order.
	FetchOrder(ctx context.Context, orderID, symbol string) (*domain.ExchangeOrder, error)

	// FetchPosition returns the current position for a symbol.
	// Returns nil if no position is open.
	FetchPosition(ctx context.Context, symbol string) (*ExchangePosition, error)

	// FetchBalance returns the account balances.
	FetchBalance(ctx context.Context) ([]Balance, error)

	// SetMarginMode sets the margin mode ("cross" or "isolated") for a symbol.
	// leverage is optional — pass 0 to skip setting leverage.
	// Some exchanges (e.g. OKX) require leverage when changing margin mode.
	SetMarginMode(ctx context.Context, symbol string, mode string, leverage int) error

	// HasCredentials returns true if API credentials are configured.
	HasCredentials() bool
}
