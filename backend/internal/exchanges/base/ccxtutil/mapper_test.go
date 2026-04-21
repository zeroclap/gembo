package ccxtutil

import (
	"testing"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"
	"github.com/zeroclap/gembo/internal/domain"
)

func TestResolveMarketType(t *testing.T) {
	tests := []struct {
		name   string
		swap   *bool
		future *bool
		want   domain.MarketType
	}{
		{"swap", ptrBool(true), ptrBool(false), domain.MarketTypePerpetual},
		{"future", ptrBool(false), ptrBool(true), domain.MarketTypeFutures},
		{"spot", ptrBool(false), ptrBool(false), domain.MarketTypeSpot},
		{"nil", nil, nil, domain.MarketTypeSpot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ccxt.MarketInterface{Swap: tt.swap, Future: tt.future}
			got := ResolveMarketType(m)
			if got != tt.want {
				t.Errorf("ResolveMarketType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapMarket(t *testing.T) {
	now := time.Now().UTC()
	m := ccxt.MarketInterface{
		Symbol:        ptrString("BTC/USDT:USDT"),
		BaseCurrency:  ptrString("BTC"),
		QuoteCurrency: ptrString("USDT"),
		Settle:        ptrString("USDT"),
		Active:        ptrBool(true),
		Swap:          ptrBool(true),
		Future:        ptrBool(false),
		Taker:         ptrFloat(0.0004),
		Maker:         ptrFloat(0.0002),
		Limits: ccxt.Limits{
			Amount: ccxt.MinMax{Min: ptrFloat(1)},
			Cost:   ccxt.MinMax{Min: ptrFloat(5)},
		},
		Info: map[string]interface{}{
			"filters": []interface{}{
				map[string]interface{}{
					"filterType":  "MIN_NOTIONAL",
					"minNotional": "5",
				},
			},
		},
	}

	dm := MapMarket(m, "testexch", now)

	if dm.ExchangeID != "testexch" {
		t.Errorf("ExchangeID = %q, want %q", dm.ExchangeID, "testexch")
	}
	if dm.Symbol != "BTC/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", dm.Symbol, "BTC/USDT:USDT")
	}
	if dm.BaseAsset != "BTC" {
		t.Errorf("BaseAsset = %q, want %q", dm.BaseAsset, "BTC")
	}
	if dm.QuoteAsset != "USDT" {
		t.Errorf("QuoteAsset = %q, want %q", dm.QuoteAsset, "USDT")
	}
	if dm.MarketType != domain.MarketTypePerpetual {
		t.Errorf("MarketType = %q, want %q", dm.MarketType, domain.MarketTypePerpetual)
	}
	if !dm.Active {
		t.Error("Active = false, want true")
	}
	if !dm.TakerFee.Equal(decimal.NewFromFloat(0.0004)) {
		t.Errorf("TakerFee = %s, want 0.0004", dm.TakerFee)
	}
	if !dm.MinAmount.Equal(decimal.NewFromInt(1)) {
		t.Errorf("MinAmount = %s, want 1", dm.MinAmount)
	}
	if !dm.MinCost.Equal(decimal.NewFromInt(5)) {
		t.Errorf("MinCost = %s, want 5", dm.MinCost)
	}
	if !dm.MinNotional.Equal(decimal.NewFromInt(5)) {
		t.Errorf("MinNotional = %s, want 5", dm.MinNotional)
	}
}

func TestMapMarket_NilFields(t *testing.T) {
	now := time.Now().UTC()
	dm := MapMarket(ccxt.MarketInterface{}, "testexch", now)

	if dm.Symbol != "" {
		t.Errorf("Symbol = %q, want empty", dm.Symbol)
	}
	if dm.MarketType != domain.MarketTypeSpot {
		t.Errorf("MarketType = %q, want %q", dm.MarketType, domain.MarketTypeSpot)
	}
}

func TestMapTicker(t *testing.T) {
	now := time.Now().UTC()
	tk := ccxt.Ticker{
		Symbol:     ptrString("ETH/USDT:USDT"),
		Last:       ptrFloat(3500.50),
		Bid:        ptrFloat(3500.00),
		Ask:        ptrFloat(3501.00),
		High:       ptrFloat(3600.00),
		Low:        ptrFloat(3400.00),
		BaseVolume: ptrFloat(12345.67),
		Percentage: ptrFloat(2.5),
	}

	dt := MapTicker(tk, "testexch", now)

	if dt.ExchangeID != "testexch" {
		t.Errorf("ExchangeID = %q, want %q", dt.ExchangeID, "testexch")
	}
	if dt.Symbol != "ETH/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", dt.Symbol, "ETH/USDT:USDT")
	}
	if !dt.Price.Equal(decimal.NewFromFloat(3500.50)) {
		t.Errorf("Price = %s, want 3500.50", dt.Price)
	}
	if !dt.Volume24h.Equal(decimal.NewFromFloat(12345.67)) {
		t.Errorf("Volume24h = %s, want 12345.67", dt.Volume24h)
	}
}

func TestMapFundingRate(t *testing.T) {
	now := time.Now().UTC()
	ts := float64(1700000000000)
	fr := ccxt.FundingRate{
		Symbol:           ptrString("BTC/USDT:USDT"),
		FundingRate:      ptrFloat(0.0001),
		FundingTimestamp: &ts,
		Interval:         ptrString("8h"),
		MarkPrice:        ptrFloat(42000.0),
		IndexPrice:       ptrFloat(41999.0),
	}

	dfr := MapFundingRate(fr, "testexch", now)

	if dfr.ExchangeID != "testexch" {
		t.Errorf("ExchangeID = %q, want %q", dfr.ExchangeID, "testexch")
	}
	if !dfr.Rate.Equal(decimal.NewFromFloat(0.0001)) {
		t.Errorf("Rate = %s, want 0.0001", dfr.Rate)
	}
	if dfr.IntervalHours != 8 {
		t.Errorf("IntervalHours = %d, want 8", dfr.IntervalHours)
	}
	if dfr.NextFundingTime.IsZero() {
		t.Error("NextFundingTime is zero, want non-zero")
	}
}

func TestMapBidAsk(t *testing.T) {
	now := time.Now().UTC()
	tk := ccxt.Ticker{
		Symbol: ptrString("BTC/USDT:USDT"),
		Bid:    ptrFloat(41999.0),
		Ask:    ptrFloat(42001.0),
	}

	dt := MapBidAsk(tk, "testexch", now)

	if dt.ExchangeID != "testexch" {
		t.Errorf("ExchangeID = %q, want %q", dt.ExchangeID, "testexch")
	}
	if dt.Symbol != "BTC/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", dt.Symbol, "BTC/USDT:USDT")
	}
	if !dt.Bid.Equal(decimal.NewFromFloat(41999.0)) {
		t.Errorf("Bid = %s, want 41999", dt.Bid)
	}
	if !dt.Ask.Equal(decimal.NewFromFloat(42001.0)) {
		t.Errorf("Ask = %s, want 42001", dt.Ask)
	}
	// Price should be zero — MapBidAsk only maps bid/ask.
	if !dt.Price.IsZero() {
		t.Errorf("Price = %s, want 0 (bid/ask only)", dt.Price)
	}
}

func TestMapOrder(t *testing.T) {
	o := ccxt.Order{
		Id:      ptrString("ord-123"),
		Symbol:  ptrString("ETH/USDT:USDT"),
		Side:    ptrString("buy"),
		Amount:  ptrFloat(10.0),
		Filled:  ptrFloat(10.0),
		Average: ptrFloat(3500.0),
		Cost:    ptrFloat(35000.0),
		Status:  ptrString("closed"),
		Fee:     ccxt.Fee{Cost: ptrFloat(14.0)},
		Info:    map[string]interface{}{"orderId": "ord-123"},
	}

	eo := MapOrder(o, "testexch")

	if eo.ExchangeID != "testexch" {
		t.Errorf("ExchangeID = %q, want %q", eo.ExchangeID, "testexch")
	}
	if eo.ExchangeOrderID != "ord-123" {
		t.Errorf("ExchangeOrderID = %q, want %q", eo.ExchangeOrderID, "ord-123")
	}
	if eo.Symbol != "ETH/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", eo.Symbol, "ETH/USDT:USDT")
	}
	if eo.Side != "buy" {
		t.Errorf("Side = %q, want %q", eo.Side, "buy")
	}
	if !eo.RequestedQty.Equal(decimal.NewFromFloat(10.0)) {
		t.Errorf("RequestedQty = %s, want 10", eo.RequestedQty)
	}
	if !eo.FilledQty.Equal(decimal.NewFromFloat(10.0)) {
		t.Errorf("FilledQty = %s, want 10", eo.FilledQty)
	}
	if !eo.AveragePrice.Equal(decimal.NewFromFloat(3500.0)) {
		t.Errorf("AveragePrice = %s, want 3500", eo.AveragePrice)
	}
	if !eo.FeeCost.Equal(decimal.NewFromFloat(14.0)) {
		t.Errorf("FeeCost = %s, want 14", eo.FeeCost)
	}
	if eo.Status != "closed" {
		t.Errorf("Status = %q, want %q", eo.Status, "closed")
	}
	if eo.RawResponse == nil {
		t.Error("RawResponse is nil, want non-nil")
	}
}

func TestMapOrder_NilInfo(t *testing.T) {
	o := ccxt.Order{
		Id:     ptrString("ord-456"),
		Symbol: ptrString("BTC/USDT:USDT"),
		Side:   ptrString("sell"),
	}

	eo := MapOrder(o, "testexch")

	if eo.RawResponse != nil {
		t.Errorf("RawResponse = %s, want nil for nil Info", string(eo.RawResponse))
	}
}

// Verify return types satisfy the expected interfaces/types at compile time.
var _ *domain.ExchangeOrder = MapOrder(ccxt.Order{}, "")
