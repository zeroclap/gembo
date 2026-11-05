package bybit

import (
	"fmt"
	"strings"
	"testing"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base/ccxtutil"
)

// fundingRateSanityBound is a conservative upper bound on the absolute value
// of a Bybit perpetual funding rate. Real-world BTC/ETH rates sit well under
// 1%; this only catches gross data corruption (e.g. a misplaced decimal
// point or wrong units), not legitimate volatility.
var fundingRateSanityBound = decimal.NewFromFloat(0.05)

// validateMarket asserts that a raw CCXT market and its domain.Market
// mapping satisfy the invariants a Bybit linear perpetual must have. It
// returns a descriptive error rather than calling testing.T so both the live
// integration tests (integration_test.go) and this file's doctored-response
// table tests can share it.
func validateMarket(raw ccxt.MarketInterface, mapped domain.Market) error {
	if mapped.Symbol == "" {
		return fmt.Errorf("market: missing symbol")
	}
	if raw.Id == nil || *raw.Id == "" {
		return fmt.Errorf("market %s: missing native id in raw response", mapped.Symbol)
	}
	if mapped.MarketType != domain.MarketTypePerpetual {
		return fmt.Errorf("market %s: expected perpetual market type, got %q", mapped.Symbol, mapped.MarketType)
	}
	if raw.Linear == nil || !*raw.Linear {
		return fmt.Errorf("market %s: expected a linear contract", mapped.Symbol)
	}
	if !mapped.ContractSize.IsPositive() {
		return fmt.Errorf("market %s: contract size must be positive, got %s", mapped.Symbol, mapped.ContractSize)
	}
	if raw.Taker == nil || raw.Maker == nil {
		return fmt.Errorf("market %s: missing taker/maker fee in raw response", mapped.Symbol)
	}
	return nil
}

// validateTicker asserts that a raw CCXT ticker and its domain.Ticker
// mapping carry a usable, timestamped top-of-book quote.
func validateTicker(raw ccxt.Ticker, mapped domain.Ticker) error {
	if mapped.Symbol == "" {
		return fmt.Errorf("ticker: missing symbol")
	}
	if raw.Timestamp == nil || *raw.Timestamp == 0 {
		return fmt.Errorf("ticker %s: missing exchange timestamp in raw response", mapped.Symbol)
	}
	if !mapped.Bid.IsPositive() {
		return fmt.Errorf("ticker %s: bid must be positive, got %s", mapped.Symbol, mapped.Bid)
	}
	if !mapped.Ask.IsPositive() {
		return fmt.Errorf("ticker %s: ask must be positive, got %s", mapped.Symbol, mapped.Ask)
	}
	if !mapped.Bid.LessThan(mapped.Ask) {
		return fmt.Errorf("ticker %s: crossed quote, bid %s is not less than ask %s", mapped.Symbol, mapped.Bid, mapped.Ask)
	}
	return nil
}

// validateFundingRate asserts that a raw CCXT funding rate and its
// domain.FundingRate mapping have a known interval, a next funding time in
// the future, and a rate within a sanity bound.
func validateFundingRate(raw ccxt.FundingRate, mapped domain.FundingRate, now time.Time) error {
	if mapped.Symbol == "" {
		return fmt.Errorf("funding rate: missing symbol")
	}
	if mapped.IntervalHours <= 0 {
		return fmt.Errorf("funding rate %s: unknown funding interval (raw interval=%q)", mapped.Symbol, ccxtutil.DerefString(raw.Interval))
	}
	if mapped.NextFundingTime.IsZero() || !mapped.NextFundingTime.After(now) {
		return fmt.Errorf("funding rate %s: next funding time %s is not after now (%s)", mapped.Symbol, mapped.NextFundingTime, now)
	}
	if mapped.Rate.Abs().GreaterThan(fundingRateSanityBound) {
		return fmt.Errorf("funding rate %s: rate %s exceeds sanity bound %s", mapped.Symbol, mapped.Rate, fundingRateSanityBound)
	}
	return nil
}

func ptrFloat(f float64) *float64 { return &f }
func ptrString(s string) *string  { return &s }
func ptrBool(b bool) *bool        { return &b }

func assertErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err.Error(), want)
	}
}

func baseValidMarket() ccxt.MarketInterface {
	return ccxt.MarketInterface{
		Id:           ptrString("BTCUSDT"),
		Symbol:       ptrString("BTC/USDT:USDT"),
		BaseCurrency: ptrString("BTC"),
		Settle:       ptrString("USDT"),
		Active:       ptrBool(true),
		Swap:         ptrBool(true),
		Linear:       ptrBool(true),
		Taker:        ptrFloat(0.0006),
		Maker:        ptrFloat(0.0001),
		ContractSize: ptrFloat(1),
	}
}

func TestValidateMarket(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		mutate  func(ccxt.MarketInterface) ccxt.MarketInterface
		wantErr string // substring expected in the error; "" means valid
	}{
		{name: "valid", mutate: func(m ccxt.MarketInterface) ccxt.MarketInterface { return m }},
		{
			name:    "missing symbol",
			mutate:  func(m ccxt.MarketInterface) ccxt.MarketInterface { m.Symbol = nil; return m },
			wantErr: "missing symbol",
		},
		{
			name:    "missing native id",
			mutate:  func(m ccxt.MarketInterface) ccxt.MarketInterface { m.Id = nil; return m },
			wantErr: "missing native id",
		},
		{
			name:    "not perpetual",
			mutate:  func(m ccxt.MarketInterface) ccxt.MarketInterface { m.Swap = ptrBool(false); return m },
			wantErr: "expected perpetual market type",
		},
		{
			name:    "not linear",
			mutate:  func(m ccxt.MarketInterface) ccxt.MarketInterface { m.Linear = ptrBool(false); return m },
			wantErr: "expected a linear contract",
		},
		{
			name:    "zero contract size",
			mutate:  func(m ccxt.MarketInterface) ccxt.MarketInterface { m.ContractSize = ptrFloat(0); return m },
			wantErr: "contract size must be positive",
		},
		{
			name: "missing fees",
			mutate: func(m ccxt.MarketInterface) ccxt.MarketInterface {
				m.Taker = nil
				m.Maker = nil
				return m
			},
			wantErr: "missing taker/maker fee",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.mutate(baseValidMarket())
			mapped := mapMarket(raw, now)
			assertErrContains(t, validateMarket(raw, mapped), tt.wantErr)
		})
	}
}

func baseValidTicker() ccxt.Ticker {
	ts := time.Now().UTC().UnixMilli()
	return ccxt.Ticker{
		Symbol:    ptrString("BTC/USDT:USDT"),
		Timestamp: &ts,
		Bid:       ptrFloat(50000),
		Ask:       ptrFloat(50001),
		Last:      ptrFloat(50000.5),
	}
}

func TestValidateTicker(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(ccxt.Ticker) ccxt.Ticker
		wantErr string
	}{
		{name: "valid", mutate: func(tk ccxt.Ticker) ccxt.Ticker { return tk }},
		{
			name:    "missing symbol",
			mutate:  func(tk ccxt.Ticker) ccxt.Ticker { tk.Symbol = nil; return tk },
			wantErr: "missing symbol",
		},
		{
			name:    "missing timestamp",
			mutate:  func(tk ccxt.Ticker) ccxt.Ticker { tk.Timestamp = nil; return tk },
			wantErr: "missing exchange timestamp",
		},
		{
			name:    "zero bid",
			mutate:  func(tk ccxt.Ticker) ccxt.Ticker { tk.Bid = ptrFloat(0); return tk },
			wantErr: "bid must be positive",
		},
		{
			name:    "zero ask",
			mutate:  func(tk ccxt.Ticker) ccxt.Ticker { tk.Ask = ptrFloat(0); return tk },
			wantErr: "ask must be positive",
		},
		{
			name:    "crossed quote",
			mutate:  func(tk ccxt.Ticker) ccxt.Ticker { tk.Bid = ptrFloat(50002); return tk },
			wantErr: "crossed quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.mutate(baseValidTicker())
			mapped := mapTicker(raw, time.Now().UTC())
			assertErrContains(t, validateTicker(raw, mapped), tt.wantErr)
		})
	}
}

func baseValidFundingRate(now time.Time) ccxt.FundingRate {
	nextFunding := float64(now.Add(4 * time.Hour).UnixMilli())
	return ccxt.FundingRate{
		Symbol:           ptrString("BTC/USDT:USDT"),
		FundingRate:      ptrFloat(0.0001),
		FundingTimestamp: &nextFunding,
		Interval:         ptrString("8h"),
	}
}

func TestValidateFundingRate(t *testing.T) {
	now := time.Now().UTC()
	pastFunding := float64(now.Add(-4 * time.Hour).UnixMilli())

	tests := []struct {
		name    string
		mutate  func(ccxt.FundingRate) ccxt.FundingRate
		wantErr string
	}{
		{name: "valid", mutate: func(fr ccxt.FundingRate) ccxt.FundingRate { return fr }},
		{
			name:    "missing symbol",
			mutate:  func(fr ccxt.FundingRate) ccxt.FundingRate { fr.Symbol = nil; return fr },
			wantErr: "missing symbol",
		},
		{
			name:    "unknown interval",
			mutate:  func(fr ccxt.FundingRate) ccxt.FundingRate { fr.Interval = nil; return fr },
			wantErr: "unknown funding interval",
		},
		{
			name:    "next funding time in the past",
			mutate:  func(fr ccxt.FundingRate) ccxt.FundingRate { fr.FundingTimestamp = &pastFunding; return fr },
			wantErr: "is not after now",
		},
		{
			name:    "rate exceeds sanity bound",
			mutate:  func(fr ccxt.FundingRate) ccxt.FundingRate { fr.FundingRate = ptrFloat(0.5); return fr },
			wantErr: "exceeds sanity bound",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.mutate(baseValidFundingRate(now))
			mapped := mapFundingRate(raw, now)
			assertErrContains(t, validateFundingRate(raw, mapped, now), tt.wantErr)
		})
	}
}
