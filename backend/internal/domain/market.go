package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// MarketType describes the kind of instrument.
type MarketType string

const (
	MarketTypeSpot      MarketType = "spot"
	MarketTypePerpetual MarketType = "perpetual"
	MarketTypeFutures   MarketType = "futures"
)

// Market represents a tradable instrument on an exchange.
type Market struct {
	ExchangeID  string     `json:"exchange_id"`  // e.g. "bybit"
	Symbol      string     `json:"symbol"`       // normalized, e.g. "BTC/USDT:USDT"
	BaseAsset   string     `json:"base_asset"`   // e.g. "BTC"
	QuoteAsset  string     `json:"quote_asset"`  // e.g. "USDT"
	SettleAsset string     `json:"settle_asset"` // e.g. "USDT" (derivatives only)
	MarketType  MarketType `json:"market_type"`  // spot | perpetual | futures
	Active      bool       `json:"active"`

	// Trading fees as reported by the exchange (ratios, e.g. 0.00055 = 0.055%).
	TakerFee decimal.Decimal `json:"taker_fee"`
	MakerFee decimal.Decimal `json:"maker_fee"`

	UpdatedAt time.Time `json:"updated_at"`
}

// MarketFilter defines criteria for including markets. Empty slices mean "no filter" (include all).
type MarketFilter struct {
	QuoteAssets  []string     // e.g. ["USDT"] — only include markets with these quote assets
	SettleAssets []string     // e.g. ["USDT"] — only include markets settled in these assets
	MarketTypes  []MarketType // e.g. ["perpetual"] — only include these market types
}

// IsEmpty returns true if the filter has no constraints.
func (f MarketFilter) IsEmpty() bool {
	return len(f.QuoteAssets) == 0 && len(f.SettleAssets) == 0 && len(f.MarketTypes) == 0
}

// Match returns true if the market passes all filter criteria.
func (f MarketFilter) Match(m Market) bool {
	if len(f.QuoteAssets) > 0 && !containsStr(f.QuoteAssets, m.QuoteAsset) {
		return false
	}
	if len(f.SettleAssets) > 0 && !containsStr(f.SettleAssets, m.SettleAsset) {
		return false
	}
	if len(f.MarketTypes) > 0 && !containsMT(f.MarketTypes, m.MarketType) {
		return false
	}
	return true
}

// FilterMarkets returns only the markets that pass the filter.
// If the filter is empty, all markets are returned unchanged.
func FilterMarkets(markets []Market, f MarketFilter) []Market {
	if f.IsEmpty() {
		return markets
	}
	filtered := make([]Market, 0, len(markets))
	for _, m := range markets {
		if f.Match(m) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func containsMT(ts []MarketType, t MarketType) bool {
	for _, v := range ts {
		if v == t {
			return true
		}
	}
	return false
}
