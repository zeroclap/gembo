package binance

import (
	"testing"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"
	"github.com/zeroclap/gembo/internal/domain"
)

func ptrFloat(f float64) *float64 { return &f }
func ptrString(s string) *string  { return &s }
func ptrBool(b bool) *bool        { return &b }

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
	}

	dm := mapMarket(m, now)

	if dm.ExchangeID != "binance" {
		t.Errorf("ExchangeID = %q, want %q", dm.ExchangeID, "binance")
	}
	if dm.Symbol != "BTC/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", dm.Symbol, "BTC/USDT:USDT")
	}
	if dm.BaseAsset != "BTC" {
		t.Errorf("BaseAsset = %q, want %q", dm.BaseAsset, "BTC")
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

	dt := mapTicker(tk, now)

	if dt.ExchangeID != "binance" {
		t.Errorf("ExchangeID = %q, want %q", dt.ExchangeID, "binance")
	}
	if dt.Symbol != "ETH/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", dt.Symbol, "ETH/USDT:USDT")
	}
	if !dt.Price.Equal(decimal.NewFromFloat(3500.50)) {
		t.Errorf("Price = %s, want 3500.50", dt.Price)
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

	dfr := mapFundingRate(fr, now)

	if dfr.ExchangeID != "binance" {
		t.Errorf("ExchangeID = %q, want %q", dfr.ExchangeID, "binance")
	}
	if !dfr.Rate.Equal(decimal.NewFromFloat(0.0001)) {
		t.Errorf("Rate = %s, want 0.0001", dfr.Rate)
	}
	if dfr.IntervalHours != 8 {
		t.Errorf("IntervalHours = %d, want 8", dfr.IntervalHours)
	}
}

func TestMapMarketNilFields(t *testing.T) {
	now := time.Now().UTC()
	dm := mapMarket(ccxt.MarketInterface{}, now)

	if dm.Symbol != "" {
		t.Errorf("Symbol = %q, want empty", dm.Symbol)
	}
	if dm.MarketType != domain.MarketTypeSpot {
		t.Errorf("MarketType = %q, want %q", dm.MarketType, domain.MarketTypeSpot)
	}
}

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
			got := resolveMarketType(m)
			if got != tt.want {
				t.Errorf("resolveMarketType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapBidAsk(t *testing.T) {
	now := time.Now().UTC()
	tk := ccxt.Ticker{
		Symbol: ptrString("BTC/USDT:USDT"),
		Bid:    ptrFloat(42000.00),
		Ask:    ptrFloat(42001.00),
	}

	dt := mapBidAsk(tk, now)

	if dt.ExchangeID != "binance" {
		t.Errorf("ExchangeID = %q, want %q", dt.ExchangeID, "binance")
	}
	if dt.Symbol != "BTC/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", dt.Symbol, "BTC/USDT:USDT")
	}
	if !dt.Bid.Equal(decimal.NewFromFloat(42000.00)) {
		t.Errorf("Bid = %s, want 42000", dt.Bid)
	}
	if !dt.Ask.Equal(decimal.NewFromFloat(42001.00)) {
		t.Errorf("Ask = %s, want 42001", dt.Ask)
	}
	// Price and volume fields should be zero since WatchBidsAsks doesn't provide them.
	if !dt.Price.IsZero() {
		t.Errorf("Price = %s, want 0", dt.Price)
	}
	if !dt.Volume24h.IsZero() {
		t.Errorf("Volume24h = %s, want 0", dt.Volume24h)
	}
}

func TestApplyTickerUpdate_NewSymbol(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	ticker := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42000),
		High24h:    decimal.NewFromFloat(43000),
		Low24h:     decimal.NewFromFloat(41000),
		Volume24h:  decimal.NewFromFloat(1000),
		Change24h:  decimal.NewFromFloat(2.5),
		UpdatedAt:  now,
	}

	merged := applyTickerUpdate(state, ticker)

	if merged.Symbol != "BTC/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", merged.Symbol, "BTC/USDT:USDT")
	}
	if !merged.Price.Equal(decimal.NewFromFloat(42000)) {
		t.Errorf("Price = %s, want 42000", merged.Price)
	}
	// Bid/Ask should be zero since this is a ticker-only update.
	if !merged.Bid.IsZero() {
		t.Errorf("Bid = %s, want 0", merged.Bid)
	}
	if !merged.Ask.IsZero() {
		t.Errorf("Ask = %s, want 0", merged.Ask)
	}
}

func TestApplyTickerUpdate_PreservesBidAsk(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	// First, apply a bid/ask update.
	bidAsk := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Bid:        decimal.NewFromFloat(41999),
		Ask:        decimal.NewFromFloat(42001),
		UpdatedAt:  now,
	}
	applyBidAskUpdate(state, bidAsk)

	// Now apply a ticker update with zero bid/ask (Binance WatchTickers behavior).
	ticker := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42000),
		High24h:    decimal.NewFromFloat(43000),
		Low24h:     decimal.NewFromFloat(41000),
		Volume24h:  decimal.NewFromFloat(1000),
		Change24h:  decimal.NewFromFloat(2.5),
		UpdatedAt:  now,
	}

	merged := applyTickerUpdate(state, ticker)

	// Price fields should be updated from the ticker.
	if !merged.Price.Equal(decimal.NewFromFloat(42000)) {
		t.Errorf("Price = %s, want 42000", merged.Price)
	}
	if !merged.Volume24h.Equal(decimal.NewFromFloat(1000)) {
		t.Errorf("Volume24h = %s, want 1000", merged.Volume24h)
	}
	// Bid/Ask should be preserved from the earlier bid/ask update.
	if !merged.Bid.Equal(decimal.NewFromFloat(41999)) {
		t.Errorf("Bid = %s, want 41999 (preserved from bid/ask update)", merged.Bid)
	}
	if !merged.Ask.Equal(decimal.NewFromFloat(42001)) {
		t.Errorf("Ask = %s, want 42001 (preserved from bid/ask update)", merged.Ask)
	}
}

func TestApplyTickerUpdate_OverwritesBidAskWhenProvided(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	// Seed with initial bid/ask.
	applyBidAskUpdate(state, domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Bid:        decimal.NewFromFloat(41999),
		Ask:        decimal.NewFromFloat(42001),
		UpdatedAt:  now,
	})

	// Ticker update that also carries bid/ask (hypothetical or future change).
	ticker := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42500),
		Bid:        decimal.NewFromFloat(42499),
		Ask:        decimal.NewFromFloat(42501),
		UpdatedAt:  now,
	}

	merged := applyTickerUpdate(state, ticker)

	if !merged.Bid.Equal(decimal.NewFromFloat(42499)) {
		t.Errorf("Bid = %s, want 42499 (overwritten)", merged.Bid)
	}
	if !merged.Ask.Equal(decimal.NewFromFloat(42501)) {
		t.Errorf("Ask = %s, want 42501 (overwritten)", merged.Ask)
	}
}

func TestApplyBidAskUpdate_NewSymbol(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	bidAsk := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "ETH/USDT:USDT",
		Bid:        decimal.NewFromFloat(3500),
		Ask:        decimal.NewFromFloat(3501),
		UpdatedAt:  now,
	}

	merged := applyBidAskUpdate(state, bidAsk)

	if merged.Symbol != "ETH/USDT:USDT" {
		t.Errorf("Symbol = %q, want %q", merged.Symbol, "ETH/USDT:USDT")
	}
	if !merged.Bid.Equal(decimal.NewFromFloat(3500)) {
		t.Errorf("Bid = %s, want 3500", merged.Bid)
	}
	if !merged.Ask.Equal(decimal.NewFromFloat(3501)) {
		t.Errorf("Ask = %s, want 3501", merged.Ask)
	}
	// Price should be zero since only bid/ask was provided.
	if !merged.Price.IsZero() {
		t.Errorf("Price = %s, want 0", merged.Price)
	}
}

func TestApplyBidAskUpdate_PreservesTickerFields(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	// First, apply a ticker update.
	ticker := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "ETH/USDT:USDT",
		Price:      decimal.NewFromFloat(3500),
		High24h:    decimal.NewFromFloat(3600),
		Low24h:     decimal.NewFromFloat(3400),
		Volume24h:  decimal.NewFromFloat(5000),
		Change24h:  decimal.NewFromFloat(1.5),
		UpdatedAt:  now,
	}
	applyTickerUpdate(state, ticker)

	// Now apply a bid/ask update.
	bidAsk := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "ETH/USDT:USDT",
		Bid:        decimal.NewFromFloat(3499),
		Ask:        decimal.NewFromFloat(3501),
		UpdatedAt:  now,
	}

	merged := applyBidAskUpdate(state, bidAsk)

	// Bid/Ask should be updated.
	if !merged.Bid.Equal(decimal.NewFromFloat(3499)) {
		t.Errorf("Bid = %s, want 3499", merged.Bid)
	}
	if !merged.Ask.Equal(decimal.NewFromFloat(3501)) {
		t.Errorf("Ask = %s, want 3501", merged.Ask)
	}
	// Price/volume fields should be preserved from the earlier ticker update.
	if !merged.Price.Equal(decimal.NewFromFloat(3500)) {
		t.Errorf("Price = %s, want 3500 (preserved from ticker update)", merged.Price)
	}
	if !merged.High24h.Equal(decimal.NewFromFloat(3600)) {
		t.Errorf("High24h = %s, want 3600 (preserved)", merged.High24h)
	}
	if !merged.Volume24h.Equal(decimal.NewFromFloat(5000)) {
		t.Errorf("Volume24h = %s, want 5000 (preserved)", merged.Volume24h)
	}
}

func TestMergeSequence_TickerThenBidAsk(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	// Simulate real Binance flow: ticker arrives first (no bid/ask), then bid/ask arrives.
	ticker := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42000),
		High24h:    decimal.NewFromFloat(43000),
		Low24h:     decimal.NewFromFloat(41000),
		Volume24h:  decimal.NewFromFloat(1000),
		Change24h:  decimal.NewFromFloat(2.5),
		UpdatedAt:  now,
	}

	first := applyTickerUpdate(state, ticker)
	if !first.Bid.IsZero() {
		t.Errorf("after ticker-only update: Bid = %s, want 0", first.Bid)
	}

	bidAsk := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Bid:        decimal.NewFromFloat(41999),
		Ask:        decimal.NewFromFloat(42001),
		UpdatedAt:  now,
	}

	second := applyBidAskUpdate(state, bidAsk)

	// After both updates, the merged ticker should have all fields.
	if !second.Price.Equal(decimal.NewFromFloat(42000)) {
		t.Errorf("Price = %s, want 42000", second.Price)
	}
	if !second.Bid.Equal(decimal.NewFromFloat(41999)) {
		t.Errorf("Bid = %s, want 41999", second.Bid)
	}
	if !second.Ask.Equal(decimal.NewFromFloat(42001)) {
		t.Errorf("Ask = %s, want 42001", second.Ask)
	}
	if !second.Volume24h.Equal(decimal.NewFromFloat(1000)) {
		t.Errorf("Volume24h = %s, want 1000", second.Volume24h)
	}
}

func TestMergeSequence_BidAskThenTicker(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	// Bid/ask arrives before ticker data.
	bidAsk := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Bid:        decimal.NewFromFloat(41999),
		Ask:        decimal.NewFromFloat(42001),
		UpdatedAt:  now,
	}

	first := applyBidAskUpdate(state, bidAsk)
	if !first.Price.IsZero() {
		t.Errorf("after bid/ask-only update: Price = %s, want 0", first.Price)
	}

	ticker := domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42000),
		High24h:    decimal.NewFromFloat(43000),
		Low24h:     decimal.NewFromFloat(41000),
		Volume24h:  decimal.NewFromFloat(1000),
		Change24h:  decimal.NewFromFloat(2.5),
		UpdatedAt:  now,
	}

	second := applyTickerUpdate(state, ticker)

	// After both updates, merged ticker should have all fields.
	if !second.Price.Equal(decimal.NewFromFloat(42000)) {
		t.Errorf("Price = %s, want 42000", second.Price)
	}
	if !second.Bid.Equal(decimal.NewFromFloat(41999)) {
		t.Errorf("Bid = %s, want 41999", second.Bid)
	}
	if !second.Ask.Equal(decimal.NewFromFloat(42001)) {
		t.Errorf("Ask = %s, want 42001", second.Ask)
	}
}

func TestMergeMultipleSymbols(t *testing.T) {
	state := make(map[string]*domain.Ticker)
	now := time.Now().UTC()

	applyTickerUpdate(state, domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Price:      decimal.NewFromFloat(42000),
		UpdatedAt:  now,
	})
	applyTickerUpdate(state, domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "ETH/USDT:USDT",
		Price:      decimal.NewFromFloat(3500),
		UpdatedAt:  now,
	})

	applyBidAskUpdate(state, domain.Ticker{
		ExchangeID: exchangeID,
		Symbol:     "BTC/USDT:USDT",
		Bid:        decimal.NewFromFloat(41999),
		Ask:        decimal.NewFromFloat(42001),
		UpdatedAt:  now,
	})

	btc := state["BTC/USDT:USDT"]
	eth := state["ETH/USDT:USDT"]

	if !btc.Bid.Equal(decimal.NewFromFloat(41999)) {
		t.Errorf("BTC Bid = %s, want 41999", btc.Bid)
	}
	if !btc.Price.Equal(decimal.NewFromFloat(42000)) {
		t.Errorf("BTC Price = %s, want 42000", btc.Price)
	}
	// ETH should not have bid/ask yet.
	if !eth.Bid.IsZero() {
		t.Errorf("ETH Bid = %s, want 0 (no bid/ask update yet)", eth.Bid)
	}
	if !eth.Price.Equal(decimal.NewFromFloat(3500)) {
		t.Errorf("ETH Price = %s, want 3500", eth.Price)
	}
}
