package binance

import (
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"
	"github.com/zeroclap/gembo/internal/domain"
)

// mapMarket converts a CCXT MarketInterface to a domain.Market.
func mapMarket(m ccxt.MarketInterface, updatedAt time.Time) domain.Market {
	return domain.Market{
		ExchangeID:  exchangeID,
		Symbol:      derefString(m.Symbol),
		BaseAsset:   derefString(m.BaseCurrency),
		QuoteAsset:  derefString(m.QuoteCurrency),
		SettleAsset: derefString(m.Settle),
		MarketType:  resolveMarketType(m),
		Active:      derefBool(m.Active),
		TakerFee:    floatToDecimal(m.Taker),
		MakerFee:    floatToDecimal(m.Maker),
		UpdatedAt:   updatedAt,
	}
}

// mapTicker converts a CCXT Ticker to a domain.Ticker.
// Used for WatchTickers data which provides price/volume but not bid/ask on Binance.
func mapTicker(t ccxt.Ticker, updatedAt time.Time) domain.Ticker {
	return domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     derefString(t.Symbol),
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
	return domain.FundingRate{
		ExchangeID:          exchangeID,
		Symbol:              derefString(fr.Symbol),
		Rate:                floatToDecimal(fr.FundingRate),
		NextFundingTime:     timestampToTime(fr.FundingTimestamp),
		IntervalHours:       intervalFromString(fr.Interval),
		MarkPrice:           floatToDecimal(fr.MarkPrice),
		IndexPrice:          floatToDecimal(fr.IndexPrice),
		NextFundingRate:     floatToDecimal(fr.NextFundingRate),
		PreviousFundingRate: floatToDecimal(fr.PreviousFundingRate),
		InterestRate:        floatToDecimal(fr.InterestRate),
		UpdatedAt:           updatedAt,
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

// mapBidAsk converts a CCXT Ticker from WatchBidsAsks into a partial
// domain.Ticker containing only the bid/ask fields and symbol metadata.
func mapBidAsk(t ccxt.Ticker, updatedAt time.Time) domain.Ticker {
	return domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     derefString(t.Symbol),
		Bid:        floatToDecimal(t.Bid),
		Ask:        floatToDecimal(t.Ask),
		UpdatedAt:  updatedAt,
	}
}

// applyTickerUpdate merges a full ticker update (from WatchTickers) into the
// in-memory state and returns the merged result. Fields from WatchTickers
// (Price, High24h, Low24h, Volume24h, Change24h) always overwrite state.
// Bid/Ask are only overwritten if the update carries non-zero values, preserving
// values previously received from WatchBidsAsks.
func applyTickerUpdate(state map[string]*domain.Ticker, t domain.Ticker) *domain.Ticker {
	existing, ok := state[t.Symbol]
	if !ok {
		cp := t
		state[t.Symbol] = &cp
		return &cp
	}

	existing.Price = t.Price
	existing.High24h = t.High24h
	existing.Low24h = t.Low24h
	existing.Volume24h = t.Volume24h
	existing.Change24h = t.Change24h
	existing.UpdatedAt = t.UpdatedAt

	// Only overwrite bid/ask if WatchTickers actually provided them (non-zero).
	if !t.Bid.IsZero() {
		existing.Bid = t.Bid
	}
	if !t.Ask.IsZero() {
		existing.Ask = t.Ask
	}

	return existing
}

// applyBidAskUpdate merges a bid/ask update (from WatchBidsAsks) into the
// in-memory state and returns the merged result. Only Bid, Ask, and UpdatedAt
// are overwritten; all other fields are preserved from previous WatchTickers data.
func applyBidAskUpdate(state map[string]*domain.Ticker, t domain.Ticker) *domain.Ticker {
	existing, ok := state[t.Symbol]
	if !ok {
		cp := t
		state[t.Symbol] = &cp
		return &cp
	}

	existing.Bid = t.Bid
	existing.Ask = t.Ask
	existing.UpdatedAt = t.UpdatedAt

	return existing
}
