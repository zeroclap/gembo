package domain

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// PositionStatus represents the lifecycle state of a position.
type PositionStatus string

const (
	PositionStatusPending       PositionStatus = "pending"
	PositionStatusOpening       PositionStatus = "opening"
	PositionStatusOpen          PositionStatus = "open"
	PositionStatusPartiallyOpen PositionStatus = "partially_open"
	PositionStatusClosing       PositionStatus = "closing"
	PositionStatusClosed        PositionStatus = "closed"
	PositionStatusFailed        PositionStatus = "failed"
)

// SizeType indicates how the user specified the position size.
type SizeType string

const (
	SizeTypeUSD  SizeType = "usd"
	SizeTypeBase SizeType = "base"
	SizeTypeQty  SizeType = "qty" // contract count (used in simulations)
)

// Position represents an open or historical funding rate arbitrage position
// consisting of a long leg on one exchange and a short leg on another.
type Position struct {
	ID            string          `json:"id"`
	Symbol        string          `json:"symbol"`
	Status        PositionStatus  `json:"status"`
	SizeType      SizeType        `json:"size_type"`
	RequestedSize decimal.Decimal `json:"requested_size"`

	// Long leg.
	LongExchangeID string          `json:"long_exchange_id"`
	LongEntryPrice decimal.Decimal `json:"long_entry_price"`
	LongFilledQty  decimal.Decimal `json:"long_filled_qty"`
	LongEntryCost  decimal.Decimal `json:"long_entry_cost"`
	LongEntryFees  decimal.Decimal `json:"long_entry_fees"`

	// Short leg.
	ShortExchangeID string          `json:"short_exchange_id"`
	ShortEntryPrice decimal.Decimal `json:"short_entry_price"`
	ShortFilledQty  decimal.Decimal `json:"short_filled_qty"`
	ShortEntryCost  decimal.Decimal `json:"short_entry_cost"`
	ShortEntryFees  decimal.Decimal `json:"short_entry_fees"`

	// Close details (populated when position is closed).
	LongExitPrice  decimal.Decimal `json:"long_exit_price"`
	LongCloseFees  decimal.Decimal `json:"long_close_fees"`
	ShortExitPrice decimal.Decimal `json:"short_exit_price"`
	ShortCloseFees decimal.Decimal `json:"short_close_fees"`

	// Snapshot of the FundingArbitrage opportunity at position open time.
	// This captures the "intent" — the values that led to the decision.
	Snapshot json.RawMessage `json:"snapshot"`

	// Live state updated by the trader monitor loop.
	LongMarkPrice      decimal.Decimal `json:"long_mark_price"`
	ShortMarkPrice     decimal.Decimal `json:"short_mark_price"`
	LongUnrealizedPnL  decimal.Decimal `json:"long_unrealized_pnl"`
	ShortUnrealizedPnL decimal.Decimal `json:"short_unrealized_pnl"`
	TotalFundingPnL    decimal.Decimal `json:"total_funding_pnl"`

	// Error info (set on failure).
	ErrorMessage string `json:"error_message,omitempty"`

	// Timestamps.
	CreatedAt time.Time  `json:"created_at"`
	OpenedAt  *time.Time `json:"opened_at,omitempty"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ExchangeOrder represents an individual order placed on an exchange
// as part of opening or closing a position leg.
type ExchangeOrder struct {
	ID              string          `json:"id"`
	PositionID      string          `json:"position_id"`
	ExchangeID      string          `json:"exchange_id"`
	ExchangeOrderID string          `json:"exchange_order_id"`
	Symbol          string          `json:"symbol"`
	Side            string          `json:"side"`   // "buy" or "sell"
	Leg             string          `json:"leg"`    // "long" or "short"
	Action          string          `json:"action"` // "open" or "close"
	RequestedQty    decimal.Decimal `json:"requested_qty"`
	FilledQty       decimal.Decimal `json:"filled_qty"`
	AveragePrice    decimal.Decimal `json:"average_price"`
	Cost            decimal.Decimal `json:"cost"`
	FeeCost         decimal.Decimal `json:"fee_cost"`
	Status          string          `json:"status"` // "pending", "submitted", "filled", "cancelled", "failed"
	RawResponse     json.RawMessage `json:"raw_response,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// FundingPayment represents a single funding fee payment received or paid
// while a position is open.
type FundingPayment struct {
	ID         string          `json:"id"`
	PositionID string          `json:"position_id"`
	ExchangeID string          `json:"exchange_id"`
	Leg        string          `json:"leg"` // "long" or "short"
	Symbol     string          `json:"symbol"`
	Rate       decimal.Decimal `json:"rate"`
	Payment    decimal.Decimal `json:"payment"` // positive = received, negative = paid
	FundedAt   time.Time       `json:"funded_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

// PositionFilter controls which positions are returned by list queries.
type PositionFilter struct {
	Status []PositionStatus
	Symbol string
}
