package exchange

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// ExchangePosition represents the current state of a position on an exchange.
type ExchangePosition struct {
	Symbol           string              `json:"symbol"`
	Side             domain.PositionSide `json:"side"` // Long, Short, or None when closed
	Contracts        decimal.Decimal     `json:"contracts"`
	EntryPrice       decimal.Decimal     `json:"entry_price"`
	MarkPrice        decimal.Decimal     `json:"mark_price"`
	LiquidationPrice decimal.Decimal     `json:"liquidation_price"`
	UnrealizedPnL    decimal.Decimal     `json:"unrealized_pnl"`
	Leverage         decimal.Decimal     `json:"leverage"`
	MarginMode       string              `json:"margin_mode"`
	Notional         decimal.Decimal     `json:"notional"`
}

// PositionWatcher is an optional interface for exchanges that support real-time
// position monitoring via WebSocket (WatchPositions). The collector type-asserts
// on this interface to start the position event loop. Only exchanges with valid
// credentials will satisfy this interface.
type PositionWatcher interface {
	// Positions subscribes to real-time position updates via WebSocket.
	// Returns a channel that emits ExchangePosition slices until ctx is cancelled.
	// Each emission is the full set of current positions on the exchange.
	Positions(ctx context.Context) (<-chan []ExchangePosition, error)
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
	// side is OrderSideBuy or OrderSideSell, amount is in base currency.
	CreateMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error)

	// CloseMarketOrder places a reduce-only market order on the exchange.
	// The exchange will reject the order if it would increase a position
	// (prevents accidentally opening a new position when the intended one
	// is already closed). Returns ErrPositionAlreadyClosed when the
	// exchange reports the position is already at zero.
	CloseMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error)

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
