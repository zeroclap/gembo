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
	UpdatedAt       time.Time       `json:"updated_at"`
}
