package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// FundingArbitrage represents a cross-exchange funding rate arbitrage opportunity.
// The strategy: open a long on one exchange and a short on another to capture
// the funding rate differential.
type FundingArbitrage struct {
	Symbol string       `json:"symbol"`
	Long   ArbitrageLeg `json:"long"`  // exchange with lower normalized rate (you benefit as long)
	Short  ArbitrageLeg `json:"short"` // exchange with higher normalized rate (you benefit as short)

	// Computed rates.
	NetRate8h decimal.Decimal `json:"net_rate_8h"` // short_rate_8h - long_rate_8h
	APR       decimal.Decimal `json:"apr"`         // annualized percentage

	// Spread.
	OpenSpread decimal.Decimal `json:"open_spread"` // (short_bid - long_ask) / midpoint * 100

	// Fees based on real per-market taker/maker rates from the exchange.
	Fees ArbitrageFees `json:"fees"`

	// Timing.
	NextPayment NextPaymentInfo `json:"next_payment"`
	FullLoop    FullLoopInfo    `json:"full_loop"`

	UpdatedAt time.Time `json:"updated_at"`
}

// ArbitrageLeg represents one side (long or short) of an arbitrage pair.
type ArbitrageLeg struct {
	ExchangeID      string          `json:"exchange_id"`
	FundingRate     decimal.Decimal `json:"funding_rate"`      // current rate
	IntervalHours   int             `json:"interval_hours"`    // 4, 8, etc.
	Rate8h          decimal.Decimal `json:"rate_8h"`           // normalized to 8h
	NextFundingAt   time.Time       `json:"next_funding_at"`   // next payment timestamp
	NextFundingRate decimal.Decimal `json:"next_funding_rate"` // predicted next rate
	Price           decimal.Decimal `json:"price"`
	Bid             decimal.Decimal `json:"bid"`
	Ask             decimal.Decimal `json:"ask"`
	MarkPrice       decimal.Decimal `json:"mark_price"`
	IndexPrice      decimal.Decimal `json:"index_price"`
	Volume24h       decimal.Decimal `json:"volume_24h"`
	MakerFeePct     decimal.Decimal `json:"maker_fee_pct"`
	TakerFeePct     decimal.Decimal `json:"taker_fee_pct"`
	ContractSize    decimal.Decimal `json:"contract_size"` // smallest tradeable unit in base currency
}

// ArbitrageFees contains fee estimates for entering and exiting an arbitrage position.
type ArbitrageFees struct {
	OpenFeesPct      decimal.Decimal `json:"open_fees_pct"`       // taker on both legs to enter
	CloseFeesPct     decimal.Decimal `json:"close_fees_pct"`      // taker on both legs to exit
	RoundTripFeesPct decimal.Decimal `json:"round_trip_fees_pct"` // open + close
	NetAPRAfterFees  decimal.Decimal `json:"net_apr_after_fees"`  // APR minus round-trip fee drag
}

// NextPaymentInfo describes the very next funding payment across both legs.
type NextPaymentInfo struct {
	ExchangeID    string          `json:"exchange_id"`     // which exchange pays next
	Side          PositionSide    `json:"side"`            // Long or Short
	Rate          decimal.Decimal `json:"rate"`            // the rate for this payment
	PaymentPct    decimal.Decimal `json:"payment_pct"`     // your P&L for this single payment
	At            time.Time       `json:"at"`              // when it occurs
	TimeUntilSecs int64           `json:"time_until_secs"` // seconds from computation time
}

// FullLoopInfo describes the theoretical payment schedule for one complete
// funding cycle (LCM of both leg intervals).
type FullLoopInfo struct {
	DurationHours int             `json:"duration_hours"`   // LCM of both intervals
	Payments      []LoopPayment   `json:"payments"`         // ordered by AtHour
	NetPctPerLoop decimal.Decimal `json:"net_pct_per_loop"` // total P&L for one full cycle
}

// LoopPayment is a single funding payment within a full loop.
type LoopPayment struct {
	ExchangeID string          `json:"exchange_id"`
	Side       PositionSide    `json:"side"`        // Long or Short
	Rate       decimal.Decimal `json:"rate"`        // the funding rate applied
	PaymentPct decimal.Decimal `json:"payment_pct"` // your +/- for this step
	AtHour     int             `json:"at_hour"`     // hour offset within the loop
	Cumulative decimal.Decimal `json:"cumulative"`  // running P&L up to this point
}

// SimulationRequest contains parameters for simulating a position on an arbitrage
// opportunity. The caller provides the opportunity key and position sizing.
type SimulationRequest struct {
	Symbol          string          `json:"symbol"`
	LongExchangeID  string          `json:"long_exchange_id"`
	ShortExchangeID string          `json:"short_exchange_id"`
	PositionSize    decimal.Decimal `json:"position_size"` // notional value in USD (used when SizeType=usd)
	Quantity        decimal.Decimal `json:"quantity"`      // contract count (used when SizeType=qty)
	SizeType        SizeType        `json:"size_type"`     // "usd" or "qty"
	SlippagePct     decimal.Decimal `json:"slippage_pct"`  // slippage percentage, e.g. 0.05
	HoldHours       decimal.Decimal `json:"hold_hours"`    // hold period in hours for hold-based simulation (e.g. 30)
}

// SimulationResult contains the computed P&L simulation for an arbitrage opportunity.
type SimulationResult struct {
	PositionSize       decimal.Decimal `json:"position_size"`        // notional value in USD (may differ from input if size_type=qty)
	ActualPositionSize decimal.Decimal `json:"actual_position_size"` // effective notional after floor-rounding (qty * price * contract_size)
	Quantity           decimal.Decimal `json:"quantity"`             // number of contracts
	Capital            decimal.Decimal `json:"capital"`              // actual capital required

	EntryCost decimal.Decimal `json:"entry_cost"` // $ cost to open (spread loss + open fees + slippage)
	ExitCost  decimal.Decimal `json:"exit_cost"`  // $ cost to close (close fees + slippage)
	TotalCost decimal.Decimal `json:"total_cost"` // entry + exit

	Funding8h   decimal.Decimal `json:"funding_8h"`   // $ funding P&L per 8h period
	FundingHold decimal.Decimal `json:"funding_hold"` // $ funding P&L over the hold period

	HoldHours decimal.Decimal `json:"hold_hours"` // effective hold period in hours

	BreakevenDays decimal.Decimal `json:"breakeven_days"` // days to recoup entry+exit cost from funding

	LcmPnL  decimal.Decimal `json:"lcm_pnl"`  // $ P&L for one full LCM loop (funding only)
	NextPnL decimal.Decimal `json:"next_pnl"` // $ P&L for the next funding cycle
	PnlHold decimal.Decimal `json:"pnl_hold"` // $ P&L for the hold period of funding

	// Both modes are always returned; the frontend switches display.
	CleanLoop      LoopSimulation `json:"clean_loop"`      // theoretical LCM-based loop with open/close fee rows
	HoldSimulation LoopSimulation `json:"hold_simulation"` // time-aware hold simulation with actual funding timings

	// Deprecated: kept for backward compatibility. Use CleanLoop.Rows instead.
	LoopPayments []SimulationLoopPayment `json:"loop_payments"`
}

// LoopSimulation represents a complete funding schedule with open/close fee bookends.
type LoopSimulation struct {
	DurationHours decimal.Decimal `json:"duration_hours"` // total duration (LCM or hold period)
	Rows          []SimulationRow `json:"rows"`           // ordered rows: open fee, funding payments..., close fee
	NetPnlPct     decimal.Decimal `json:"net_pnl_pct"`    // total P&L % (fees + funding)
	NetPnlUsd     decimal.Decimal `json:"net_pnl_usd"`    // total P&L $ (fees + funding)
}

// SimulationRowKind identifies what type of event a simulation row represents.
type SimulationRowKind string

const (
	RowKindOpen    SimulationRowKind = "open"    // opening fees + slippage + spread cost
	RowKindFunding SimulationRowKind = "funding" // a funding payment
	RowKindClose   SimulationRowKind = "close"   // closing fees + slippage
)

// SimulationEvent is a single per-exchange event within a SimulationRow.
// Every row always has at least one event (1 leg or 2 simultaneous legs).
type SimulationEvent struct {
	ExchangeID string          `json:"exchange_id"`    // which exchange
	Side       PositionSide    `json:"side"`           // Long or Short
	Rate       decimal.Decimal `json:"rate,omitempty"` // rate as fraction: funding rate or taker fee
	PnlPct     decimal.Decimal `json:"pnl_pct"`        // P&L % for this event
	PnlUsd     decimal.Decimal `json:"pnl_usd"`        // P&L $ for this event
}

// SimulationRow is a summary row in the loop simulation table.
// It represents either a fee event (open/close) or a funding payment.
// The row carries aggregate totals; per-exchange breakdown is in Events.
// A row always has at least one event.
type SimulationRow struct {
	Kind          SimulationRowKind `json:"kind"`           // "open", "funding", or "close"
	Label         string            `json:"label"`          // human-readable label, e.g. "Open fees", "4h", "Close fees"
	AtHour        decimal.Decimal   `json:"at_hour"`        // hour offset (decimal for sub-hour precision)
	PnlPct        decimal.Decimal   `json:"pnl_pct"`        // P&L % for this row (negative for fees)
	PnlUsd        decimal.Decimal   `json:"pnl_usd"`        // P&L $ for this row
	CumulativePct decimal.Decimal   `json:"cumulative_pct"` // running total %
	CumulativeUsd decimal.Decimal   `json:"cumulative_usd"` // running total $
	At            *time.Time        `json:"at,omitempty"`   // absolute timestamp (only for hold-based rows)
	Events        []SimulationEvent `json:"events"`         // per-exchange breakdown (always ≥ 1)
}

// SimulationLoopPayment mirrors LoopPayment but includes USD P&L values
// computed from the position size.
// Deprecated: use SimulationRow in CleanLoop/HoldSimulation instead.
type SimulationLoopPayment struct {
	ExchangeID    string          `json:"exchange_id"`
	Side          PositionSide    `json:"side"`
	Rate          decimal.Decimal `json:"rate"`
	PaymentPct    decimal.Decimal `json:"payment_pct"`
	AtHour        int             `json:"at_hour"`
	Cumulative    decimal.Decimal `json:"cumulative"`
	PaymentUsd    decimal.Decimal `json:"payment_usd"`    // payment_pct * position_size
	CumulativeUsd decimal.Decimal `json:"cumulative_usd"` // cumulative * position_size
}
