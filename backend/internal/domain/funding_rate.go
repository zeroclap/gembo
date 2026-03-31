package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// FundingRate is the periodic funding payment for a perpetual contract.
type FundingRate struct {
	ExchangeID      string          `json:"exchange_id"`
	Symbol          string          `json:"symbol"`
	Rate            decimal.Decimal `json:"rate"` // e.g. -0.04 means -4%
	NextFundingTime time.Time       `json:"next_funding_time"`
	IntervalHours   int             `json:"interval_hours"` // typically 4 or 8

	// Additional CCXT fields for richer analysis.
	MarkPrice           decimal.Decimal `json:"mark_price"`
	IndexPrice          decimal.Decimal `json:"index_price"`
	NextFundingRate     decimal.Decimal `json:"next_funding_rate"`     // predicted next rate (exchange-provided)
	PreviousFundingRate decimal.Decimal `json:"previous_funding_rate"` // previous period's rate
	InterestRate        decimal.Decimal `json:"interest_rate"`         // base interest rate component

	UpdatedAt time.Time `json:"updated_at"`
}
