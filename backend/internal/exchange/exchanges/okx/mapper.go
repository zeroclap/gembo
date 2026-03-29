package okx

import (
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"
	"github.com/zeroclap/gembo/internal/domain"
)

// mapMarket converts a CCXT MarketInterface to a domain.Market.
func mapMarket(m ccxt.MarketInterface, updatedAt time.Time) domain.Market {
	symbol := derefString(m.Symbol)
	base := derefString(m.BaseCurrency)
	quote := derefString(m.QuoteCurrency)
	active := derefBool(m.Active)
	mtype := resolveMarketType(m)

	return domain.Market{
		ExchangeID: exchangeID,
		Symbol:     symbol,
		BaseAsset:  base,
		QuoteAsset: quote,
		MarketType: mtype,
		Active:     active,
		UpdatedAt:  updatedAt,
	}
}

// mapTicker converts a CCXT Ticker to a domain.Ticker.
func mapTicker(t ccxt.Ticker, updatedAt time.Time) domain.Ticker {
	symbol := derefString(t.Symbol)
	return domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     symbol,
		Price:      floatToDecimal(t.Last),
		Bid:        floatToDecimal(t.Bid),
		Ask:        floatToDecimal(t.Ask),
		High24h:    floatToDecimal(t.High),
		Low24h:     floatToDecimal(t.Low),
		Volume24h:  floatToDecimal(t.BaseVolume),
		Change24h:  floatToDecimal(t.Percentage),
		UpdatedAt:  updatedAt,
	}
}

// mapFundingRate converts a CCXT FundingRate to a domain.FundingRate.
func mapFundingRate(fr ccxt.FundingRate, updatedAt time.Time) domain.FundingRate {
	symbol := derefString(fr.Symbol)
	rate := floatToDecimal(fr.FundingRate)
	nextFunding := timestampToTime(fr.NextFundingTimestamp)
	intervalHours := intervalFromString(fr.Interval)

	return domain.FundingRate{
		ExchangeID:      exchangeID,
		Symbol:          symbol,
		Rate:            rate,
		NextFundingTime: nextFunding,
		IntervalHours:   intervalHours,
		UpdatedAt:       updatedAt,
	}
}

// resolveMarketType maps CCXT market type fields to domain.MarketType.
func resolveMarketType(m ccxt.MarketInterface) domain.MarketType {
	if m.Swap != nil && *m.Swap {
		return domain.MarketTypePerpetual
	}
	if m.Future != nil && *m.Future {
		return domain.MarketTypeFutures
	}
	return domain.MarketTypeSpot
}

// floatToDecimal safely converts a *float64 to decimal.Decimal.
func floatToDecimal(f *float64) decimal.Decimal {
	if f == nil {
		return decimal.Zero
	}
	return decimal.NewFromFloat(*f)
}

// derefString safely dereferences a *string.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// derefBool safely dereferences a *bool.
func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// timestampToTime converts a CCXT *float64 millisecond timestamp to time.Time.
func timestampToTime(ts *float64) time.Time {
	if ts == nil || *ts == 0 {
		return time.Time{}
	}
	sec := int64(*ts) / 1000
	ns := (int64(*ts) % 1000) * int64(time.Millisecond)
	return time.Unix(sec, ns).UTC()
}

// intervalFromString parses CCXT funding interval strings like "4h" or "8h".
func intervalFromString(s *string) int {
	if s == nil {
		return 0
	}
	switch *s {
	case "1h":
		return 1
	case "2h":
		return 2
	case "4h":
		return 4
	case "8h":
		return 8
	case "12h":
		return 12
	case "24h":
		return 24
	}
	return 0
}
