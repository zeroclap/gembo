package ccxtutil

import (
	"encoding/json"
	"fmt"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"

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
	minNotional := decimalFromMarketNotional(m)
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
		MinAmount:    FloatToDecimal(m.Limits.Amount.Min),
		MinCost:      FloatToDecimal(m.Limits.Cost.Min),
		MinNotional:  minNotional,
		UpdatedAt:    updatedAt,
	}
}

func decimalFromMarketNotional(m ccxt.MarketInterface) decimal.Decimal {
	if m.Info == nil {
		return decimal.Zero
	}

	// Binance/derivatives often expose this under "notional" filter.
	filters, ok := m.Info["filters"]
	if !ok {
		return decimal.Zero
	}

	filterList, ok := filters.([]interface{})
	if !ok {
		return decimal.Zero
	}

	for _, f := range filterList {
		filterMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		filterType, _ := filterMap["filterType"].(string)
		if filterType != "MIN_NOTIONAL" && filterType != "NOTIONAL" {
			continue
		}

		if v, ok := filterMap["notional"]; ok {
			if d, err := decimalFromAny(v); err == nil {
				return d
			}
		}
		if v, ok := filterMap["minNotional"]; ok {
			if d, err := decimalFromAny(v); err == nil {
				return d
			}
		}
	}

	return decimal.Zero
}

func decimalFromAny(v interface{}) (decimal.Decimal, error) {
	switch val := v.(type) {
	case string:
		return decimal.NewFromString(val)
	case float64:
		return decimal.NewFromFloat(val), nil
	case float32:
		return decimal.NewFromFloat32(val), nil
	case int:
		return decimal.NewFromInt(int64(val)), nil
	case int64:
		return decimal.NewFromInt(val), nil
	case json.Number:
		return decimal.NewFromString(val.String())
	default:
		return decimal.Zero, fmt.Errorf("unsupported numeric type %T", v)
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
