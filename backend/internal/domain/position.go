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

// CloseReason indicates why a position was closed.
type CloseReason string

const (
	CloseReasonNormal        CloseReason = "normal"         // closed via API request
	CloseReasonAutoClose     CloseReason = "auto_close"     // reactor detected one leg gone, closed other
	CloseReasonLiquidated    CloseReason = "liquidated"     // detected via zero contracts + liquidation indicators
	CloseReasonTargetReached CloseReason = "target_reached" // spread converged to target (spread arb)
	CloseReasonStopLoss      CloseReason = "stop_loss"      // spread widened beyond stop-loss (spread arb)
	CloseReasonTimeout       CloseReason = "timeout"        // max hold duration exceeded (spread arb)
)

// SizeType indicates how the user specified the position size.
type SizeType string

const (
	SizeTypePrice    SizeType = "price"
	SizeTypeQuantity SizeType = "quantity"
)

// StopReason indicates why stepped execution stopped before completing all steps.
type StopReason string

const (
	StopReasonNone          StopReason = ""               // completed all steps
	StopReasonPriceExceeded StopReason = "price_exceeded" // bid/ask outside min/max range
	StopReasonSlippage      StopReason = "slippage"       // spread exceeded slippage tolerance
	StopReasonStepFailed    StopReason = "step_failed"    // a portion order failed on one or both legs
)

// Position represents an open or historical arbitrage position consisting of
// a long leg on one exchange and a short leg on another. Supports both funding
// rate arbitrage and cross-exchange spread arbitrage.
type Position struct {
	ID            string          `json:"id"`
	Symbol        string          `json:"symbol"`
	Status        PositionStatus  `json:"status"`
	SizeType      SizeType        `json:"size_type"`
	RequestedSize decimal.Decimal `json:"requested_size"`
	// RequestedBaseQty is the canonical requested size in base asset units.
	// For size_type=quantity it equals requested_size. For size_type=price it is
	// resolved at open time using a reference price.
	RequestedBaseQty decimal.Decimal `json:"requested_base_qty"`

	// Arbitrage type: "funding" (default) or "spread".
	ArbitrageType ArbitrageType `json:"arbitrage_type"`

	// Spread arbitrage parameters (only used when ArbitrageType == "spread").
	EntrySpreadPct    decimal.Decimal `json:"entry_spread_pct"`     // spread % at position open time
	TargetSpreadPct   decimal.Decimal `json:"target_spread_pct"`    // close when spread <= this (convergence target)
	StopLossSpreadPct decimal.Decimal `json:"stop_loss_spread_pct"` // close when spread >= this (risk limit)
	MaxHoldDuration   int64           `json:"max_hold_duration"`    // max hold time in seconds; 0 = no limit

	// Stepped execution parameters (set at creation, immutable).
	PortionSize   decimal.Decimal `json:"portion_size"`   // size per step (same unit as SizeType); 0 = single-shot
	PriceMin      decimal.Decimal `json:"price_min"`      // min acceptable price; 0 = no lower bound
	PriceMax      decimal.Decimal `json:"price_max"`      // max acceptable price; 0 = no upper bound
	SlippagePct   decimal.Decimal `json:"slippage_pct"`   // max slippage % per portion (e.g. 0.1 = 0.1%)
	Leverage      int             `json:"leverage"`       // per-position leverage (1, 2, 3, etc.)
	MarginCapital decimal.Decimal `json:"margin_capital"` // declared available collateral in USD; 0 = unchecked

	// Stepped execution progress.
	StepsCompleted int        `json:"steps_completed"` // portions successfully filled
	StepsTotal     int        `json:"steps_total"`     // total planned portions (0 = single-shot)
	StopReason     StopReason `json:"stop_reason"`     // why execution stopped early; "" = normal

	// Long leg.
	LongExchangeID    string          `json:"long_exchange_id"`
	LongContractSize  decimal.Decimal `json:"long_contract_size"`
	LongEntryPrice    decimal.Decimal `json:"long_entry_price"`
	LongFilledQty     decimal.Decimal `json:"long_filled_qty"` // contracts
	LongFilledBaseQty decimal.Decimal `json:"long_filled_base_qty"`
	LongEntryCost     decimal.Decimal `json:"long_entry_cost"`
	LongEntryFees     decimal.Decimal `json:"long_entry_fees"`

	// Short leg.
	ShortExchangeID    string          `json:"short_exchange_id"`
	ShortContractSize  decimal.Decimal `json:"short_contract_size"`
	ShortEntryPrice    decimal.Decimal `json:"short_entry_price"`
	ShortFilledQty     decimal.Decimal `json:"short_filled_qty"` // contracts
	ShortFilledBaseQty decimal.Decimal `json:"short_filled_base_qty"`
	ShortEntryCost     decimal.Decimal `json:"short_entry_cost"`
	ShortEntryFees     decimal.Decimal `json:"short_entry_fees"`

	// Close details (populated when position is closed).
	LongExitPrice  decimal.Decimal `json:"long_exit_price"`
	LongCloseFees  decimal.Decimal `json:"long_close_fees"`
	ShortExitPrice decimal.Decimal `json:"short_exit_price"`
	ShortCloseFees decimal.Decimal `json:"short_close_fees"`

	// Snapshot of the FundingArbitrage opportunity at position open time.
	// This captures the "intent" — the values that led to the decision.
	Snapshot json.RawMessage `json:"snapshot"`

	// Live state updated by the position reactor via collector events.
	LongMarkPrice      decimal.Decimal `json:"long_mark_price"`
	ShortMarkPrice     decimal.Decimal `json:"short_mark_price"`
	LongUnrealizedPnL  decimal.Decimal `json:"long_unrealized_pnl"`
	ShortUnrealizedPnL decimal.Decimal `json:"short_unrealized_pnl"`
	TotalFundingPnL    decimal.Decimal `json:"total_funding_pnl"`

	// Liquidation & margin state (per-leg, updated by position reactor).
	LongLiquidationPrice  decimal.Decimal `json:"long_liquidation_price"`
	ShortLiquidationPrice decimal.Decimal `json:"short_liquidation_price"`
	LongLeverage          decimal.Decimal `json:"long_leverage"`
	ShortLeverage         decimal.Decimal `json:"short_leverage"`
	LongMarginMode        string          `json:"long_margin_mode"`
	ShortMarginMode       string          `json:"short_margin_mode"`
	LongNotional          decimal.Decimal `json:"long_notional"`
	ShortNotional         decimal.Decimal `json:"short_notional"`

	// Close metadata.
	CloseReason CloseReason `json:"close_reason,omitempty"`

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
	ID               string          `json:"id"`
	PositionID       string          `json:"position_id"`
	ExchangeID       string          `json:"exchange_id"`
	ExchangeOrderID  string          `json:"exchange_order_id"`
	Symbol           string          `json:"symbol"`
	Side             OrderSide       `json:"side"`          // Buy or Sell
	Leg              string          `json:"leg"`           // "long" or "short"
	Action           string          `json:"action"`        // "open" or "close"
	RequestedQty     decimal.Decimal `json:"requested_qty"` // contracts
	RequestedBaseQty decimal.Decimal `json:"requested_base_qty"`
	ContractSize     decimal.Decimal `json:"contract_size"`
	FilledQty        decimal.Decimal `json:"filled_qty"` // contracts
	FilledBaseQty    decimal.Decimal `json:"filled_base_qty"`
	AveragePrice     decimal.Decimal `json:"average_price"`
	Cost             decimal.Decimal `json:"cost"`
	FeeCost          decimal.Decimal `json:"fee_cost"`
	Status           string          `json:"status"` // "pending", "submitted", "filled", "cancelled", "failed"
	RawResponse      json.RawMessage `json:"raw_response,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
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

// ExchangePositionEvent represents a position state update from an exchange,
// published by the collector via Redis pubsub. The trader subscribes and
// reacts to these events (update PG, detect closures, auto-close, rebalance).
type ExchangePositionEvent struct {
	ExchangeID       string          `json:"exchange_id"`
	Symbol           string          `json:"symbol"`
	Side             PositionSide    `json:"side"` // Long, Short, or None when closed
	Contracts        decimal.Decimal `json:"contracts"`
	ContractSize     decimal.Decimal `json:"contract_size"`
	BaseQty          decimal.Decimal `json:"base_qty"`
	EntryPrice       decimal.Decimal `json:"entry_price"`
	MarkPrice        decimal.Decimal `json:"mark_price"`
	LiquidationPrice decimal.Decimal `json:"liquidation_price"`
	UnrealizedPnL    decimal.Decimal `json:"unrealized_pnl"`
	Leverage         decimal.Decimal `json:"leverage"`
	MarginMode       string          `json:"margin_mode"`
	Notional         decimal.Decimal `json:"notional"`
	Timestamp        time.Time       `json:"timestamp"`
}
