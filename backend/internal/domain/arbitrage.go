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

	// Fees (mock values for now).
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
	MakerFeePct     decimal.Decimal `json:"maker_fee_pct"` // mock
	TakerFeePct     decimal.Decimal `json:"taker_fee_pct"` // mock
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
	Side          string          `json:"side"`            // "long" or "short"
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
	Side       string          `json:"side"`        // "long" or "short"
	Rate       decimal.Decimal `json:"rate"`        // the funding rate applied
	PaymentPct decimal.Decimal `json:"payment_pct"` // your +/- for this step
	AtHour     int             `json:"at_hour"`     // hour offset within the loop
	Cumulative decimal.Decimal `json:"cumulative"`  // running P&L up to this point
}
