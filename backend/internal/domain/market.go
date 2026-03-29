package domain

import "time"

// MarketType describes the kind of instrument.
type MarketType string

const (
	MarketTypeSpot      MarketType = "spot"
	MarketTypePerpetual MarketType = "perpetual"
	MarketTypeFutures   MarketType = "futures"
)

// Market represents a tradable instrument on an exchange.
type Market struct {
	ExchangeID string     `json:"exchange_id"` // e.g. "bybit"
	Symbol     string     `json:"symbol"`      // normalized, e.g. "TON/USDT"
	BaseAsset  string     `json:"base_asset"`  // e.g. "TON"
	QuoteAsset string     `json:"quote_asset"` // e.g. "USDT"
	MarketType MarketType `json:"market_type"` // spot | perpetual | futures
	Active     bool       `json:"active"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
