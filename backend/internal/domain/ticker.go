package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Ticker is a real-time price snapshot for a market.
type Ticker struct {
	ExchangeID string          `json:"exchange_id"`
	Symbol     string          `json:"symbol"`
	Price      decimal.Decimal `json:"price"` // last traded price
	Bid        decimal.Decimal `json:"bid"`
	Ask        decimal.Decimal `json:"ask"`
	High24h    decimal.Decimal `json:"high_24h"`
	Low24h     decimal.Decimal `json:"low_24h"`
	Volume24h  decimal.Decimal `json:"volume_24h"`
	Change24h  decimal.Decimal `json:"change_24h"` // percentage
	UpdatedAt  time.Time       `json:"updated_at"` // duplicate of ReceivedAt, kept for API compatibility; remove once API consumers read received_at

	SourceTime time.Time `json:"source_time"` // exchange-reported timestamp of the bid/ask; zero if absent
	ReceivedAt time.Time `json:"received_at"` // local time the update was received
}
