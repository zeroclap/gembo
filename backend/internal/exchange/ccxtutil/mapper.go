package ccxtutil

import (
	"encoding/json"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
)

// ResolveMarketType maps CCXT market type fields to domain.MarketType.
func ResolveMarketType(m ccxt.MarketInterface) domain.MarketType {
	if m.Swap != nil && *m.Swap {
		return domain.MarketTypePerpetual
	}
	if m.Future != nil && *m.Future {
		return domain.MarketTypeFutures
	}
	return domain.MarketTypeSpot
}

// MapMarket converts a CCXT MarketInterface to a domain.Market.
func MapMarket(m ccxt.MarketInterface, exchangeID string, updatedAt time.Time) domain.Market {
	return domain.Market{
		ExchangeID:   exchangeID,
		Symbol:       DerefString(m.Symbol),
		BaseAsset:    DerefString(m.BaseCurrency),
		QuoteAsset:   DerefString(m.QuoteCurrency),
		SettleAsset:  DerefString(m.Settle),
		MarketType:   ResolveMarketType(m),
		Active:       DerefBool(m.Active),
		TakerFee:     FloatToDecimal(m.Taker),
		MakerFee:     FloatToDecimal(m.Maker),
		ContractSize: FloatToDecimal(m.ContractSize),
		UpdatedAt:    updatedAt,
	}
}

// MapTicker converts a CCXT Ticker to a domain.Ticker.
func MapTicker(t ccxt.Ticker, exchangeID string, updatedAt time.Time) domain.Ticker {
	return domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     DerefString(t.Symbol),
		Price:      FloatToDecimal(t.Last),
		Bid:        FloatToDecimal(t.Bid),
		Ask:        FloatToDecimal(t.Ask),
		High24h:    FloatToDecimal(t.High),
		Low24h:     FloatToDecimal(t.Low),
		Volume24h:  FloatToDecimal(t.BaseVolume),
		Change24h:  FloatToDecimal(t.Percentage),
		UpdatedAt:  updatedAt,
	}
}

// MapFundingRate converts a CCXT FundingRate to a domain.FundingRate.
func MapFundingRate(fr ccxt.FundingRate, exchangeID string, updatedAt time.Time) domain.FundingRate {
	return domain.FundingRate{
		ExchangeID:          exchangeID,
		Symbol:              DerefString(fr.Symbol),
		Rate:                FloatToDecimal(fr.FundingRate),
		NextFundingTime:     TimestampToTime(fr.FundingTimestamp),
		IntervalHours:       IntervalFromString(fr.Interval),
		MarkPrice:           FloatToDecimal(fr.MarkPrice),
		IndexPrice:          FloatToDecimal(fr.IndexPrice),
		NextFundingRate:     FloatToDecimal(fr.NextFundingRate),
		PreviousFundingRate: FloatToDecimal(fr.PreviousFundingRate),
		InterestRate:        FloatToDecimal(fr.InterestRate),
		UpdatedAt:           updatedAt,
	}
}

// MapBidAsk converts a CCXT Ticker from WatchBidsAsks into a partial
// domain.Ticker containing only the bid/ask fields and symbol metadata.
func MapBidAsk(t ccxt.Ticker, exchangeID string, updatedAt time.Time) domain.Ticker {
	return domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     DerefString(t.Symbol),
		Bid:        FloatToDecimal(t.Bid),
		Ask:        FloatToDecimal(t.Ask),
		UpdatedAt:  updatedAt,
	}
}

// MapOrder converts a CCXT Order to a domain.ExchangeOrder.
func MapOrder(o ccxt.Order, exchangeID string) *domain.ExchangeOrder {
	var rawResp json.RawMessage
	if o.Info != nil {
		if b, err := json.Marshal(o.Info); err == nil {
			rawResp = b
		}
	}

	return &domain.ExchangeOrder{
		ExchangeID:      exchangeID,
		ExchangeOrderID: DerefString(o.Id),
		Symbol:          DerefString(o.Symbol),
		Side:            NormalizeOrderSide(DerefString(o.Side)),
		RequestedQty:    FloatToDecimal(o.Amount),
		FilledQty:       FloatToDecimal(o.Filled),
		AveragePrice:    FloatToDecimal(o.Average),
		Cost:            FloatToDecimal(o.Cost),
		FeeCost:         FloatToDecimal(o.Fee.Cost),
		Status:          DerefString(o.Status),
		RawResponse:     rawResp,
	}
}

// MapPosition converts a CCXT Position to an exchange.ExchangePosition.
func MapPosition(p ccxt.Position) *exchange.ExchangePosition {
	return &exchange.ExchangePosition{
		Symbol:           DerefString(p.Symbol),
		Side:             NormalizePositionSide(DerefString(p.Side)),
		Contracts:        FloatToDecimal(p.Contracts),
		EntryPrice:       FloatToDecimal(p.EntryPrice),
		MarkPrice:        FloatToDecimal(p.MarkPrice),
		LiquidationPrice: FloatToDecimal(p.LiquidationPrice),
		UnrealizedPnL:    FloatToDecimal(p.UnrealizedPnl),
		Leverage:         FloatToDecimal(p.Leverage),
		MarginMode:       DerefString(p.MarginMode),
		Notional:         FloatToDecimal(p.Notional),
	}
}

// MapBalances converts CCXT Balances to a slice of exchange.Balance.
func MapBalances(b ccxt.Balances) []exchange.Balance {
	var out []exchange.Balance
	for currency, bal := range b.Balances {
		out = append(out, exchange.Balance{
			Currency: currency,
			Free:     FloatToDecimal(bal.Free),
			Used:     FloatToDecimal(bal.Used),
			Total:    FloatToDecimal(bal.Total),
		})
	}
	return out
}
