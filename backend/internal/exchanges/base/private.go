package base

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// ExchangePosition represents the current state of a position on an exchange.
type ExchangePosition struct {
	Symbol           string              `json:"symbol"`
	Side             domain.PositionSide `json:"side"`
	Contracts        decimal.Decimal     `json:"contracts"`
	EntryPrice       decimal.Decimal     `json:"entry_price"`
	MarkPrice        decimal.Decimal     `json:"mark_price"`
	LiquidationPrice decimal.Decimal     `json:"liquidation_price"`
	UnrealizedPnL    decimal.Decimal     `json:"unrealized_pnl"`
	Leverage         decimal.Decimal     `json:"leverage"`
	MarginMode       string              `json:"margin_mode"`
	Notional         decimal.Decimal     `json:"notional"`
}

// Balance represents an account balance for a single currency.
type Balance struct {
	Currency string          `json:"currency"`
	Free     decimal.Decimal `json:"free"`
	Used     decimal.Decimal `json:"used"`
	Total    decimal.Decimal `json:"total"`
}

// FundingHistoryEntry is an exact funding payment record returned by an exchange.
type FundingHistoryEntry struct {
	ID        string          `json:"id"`
	Symbol    string          `json:"symbol"`
	Amount    decimal.Decimal `json:"amount"`
	Currency  string          `json:"currency"`
	Timestamp time.Time       `json:"timestamp"`
}

// PrivateExchange extends Exchange with all authenticated capabilities.
// An exchange either supports the full private suite or none of it — there is
// no partial implementation. Gate.io (read-only, integration in progress) only
// implements Exchange, not PrivateExchange.
//
// All streaming methods return channels for consistency: the caller simply reads
// from a channel regardless of whether the underlying transport is WebSocket or
// periodic REST polling.
type PrivateExchange interface {
	Exchange

	// HasCredentials returns true if API credentials are configured.
	HasCredentials() bool

	// Streaming — all channel-based.

	// Positions subscribes to real-time position updates (WebSocket).
	// Each emission is the full set of current positions on the exchange.
	Positions(ctx context.Context) (<-chan []ExchangePosition, error)

	// Balances emits periodic balance snapshots.
	// The adapter owns the polling interval internally.
	Balances(ctx context.Context) (<-chan []Balance, error)

	// FundingHistory emits periodic funding payment history snapshots.
	// The adapter owns the polling interval internally.
	FundingHistory(ctx context.Context) (<-chan []FundingHistoryEntry, error)

	// Order execution.

	// CreateMarketOrder places a market order on the exchange.
	CreateMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error)

	// CloseMarketOrder places a reduce-only market order on the exchange.
	// Returns ErrPositionAlreadyClosed when the position is already at zero.
	CloseMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error)

	// FetchOrder retrieves the current state of an order by exchange-assigned ID.
	FetchOrder(ctx context.Context, orderID, symbol string) (*domain.ExchangeOrder, error)

	// FetchPosition returns the current position for a symbol, or nil if none.
	FetchPosition(ctx context.Context, symbol string) (*ExchangePosition, error)

	// FetchBalance returns account balances (single REST call, for on-demand use).
	FetchBalance(ctx context.Context) ([]Balance, error)

	// SetMarginMode sets the margin mode ("cross" or "isolated") for a symbol.
	// leverage is optional — pass 0 to skip setting leverage.
	SetMarginMode(ctx context.Context, symbol string, mode string, leverage int) error
}
