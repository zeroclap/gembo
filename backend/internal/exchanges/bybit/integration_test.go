//go:build integration

// Package bybit integration tests call Bybit's real public REST/WebSocket
// endpoints. They require non-US network egress (Bybit geo-blocks US IPs;
// see issue #4) and are gated behind the "integration" build tag so `go
// test ./...` never dials the network. Run with:
//
//	go test -tags integration ./internal/exchanges/bybit/...
package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
)

const (
	integrationWindow           = 30 * time.Second
	integrationMinTickerUpdates = 3
)

var (
	integrationSymbols = []string{"BTC/USDT:USDT", "ETH/USDT:USDT"}
	// integrationNativeIDs is Bybit's own instrument id for each symbol
	// above (see issue #5: "native id matches").
	integrationNativeIDs = map[string]string{
		"BTC/USDT:USDT": "BTCUSDT",
		"ETH/USDT:USDT": "ETHUSDT",
	}
)

func newIntegrationClient(t *testing.T) *Client {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(logger)
}

// writeFixture records a sanitized (scoped to the symbols this test suite
// cares about) raw CCXT response under testdata/fixtures/ for the replay
// e2e (issue #7) to consume without dialing the network.
func writeFixture(t *testing.T, name string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture %s: %v", name, err)
	}
	path := filepath.Join("testdata", "fixtures", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture dir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// TestIntegrationFetchMarkets asserts FetchMarkets -> domain.Market mapping
// for BTC/USDT:USDT and ETH/USDT:USDT: present, linear perpetual, positive
// contract size, fees present, and native id matching Bybit's own symbol.
func TestIntegrationFetchMarkets(t *testing.T) {
	c := newIntegrationClient(t)

	raw, err := c.api.FetchMarkets()
	if err != nil {
		t.Fatalf("FetchMarkets: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("FetchMarkets returned no markets")
	}

	bySymbol := make(map[string]ccxt.MarketInterface, len(raw))
	for _, m := range raw {
		if m.Symbol != nil {
			bySymbol[*m.Symbol] = m
		}
	}

	fixture := make(map[string]ccxt.MarketInterface, len(integrationSymbols))
	now := time.Now().UTC()
	for _, symbol := range integrationSymbols {
		rawMarket, ok := bySymbol[symbol]
		if !ok {
			t.Errorf("market %s: not present in FetchMarkets response (count=%d)", symbol, len(raw))
			continue
		}
		fixture[symbol] = rawMarket

		mapped := mapMarket(rawMarket, now)
		if err := validateMarket(rawMarket, mapped); err != nil {
			t.Errorf("market %s: %v", symbol, err)
			continue
		}

		wantNativeID := integrationNativeIDs[symbol]
		if rawMarket.Id == nil || *rawMarket.Id != wantNativeID {
			t.Errorf("market %s: native id = %v, want %s", symbol, rawMarket.Id, wantNativeID)
		}
	}

	writeFixture(t, "markets.json", fixture)
}

// TestIntegrationFetchFundingRates asserts FetchFundingRates ->
// domain.FundingRate mapping for BTC/USDT:USDT and ETH/USDT:USDT: known
// interval (Bybit currently reports 8h for both), next funding time in the
// future, and a rate within a sanity bound.
func TestIntegrationFetchFundingRates(t *testing.T) {
	c := newIntegrationClient(t)

	rates, err := c.api.FetchFundingRates(ccxt.WithFetchFundingRatesSymbols(integrationSymbols))
	if err != nil {
		t.Fatalf("FetchFundingRates: %v", err)
	}

	bySymbol := make(map[string]ccxt.FundingRate, len(rates.FundingRates))
	for _, fr := range rates.FundingRates {
		if fr.Symbol != nil {
			bySymbol[*fr.Symbol] = fr
		}
	}

	fixture := make(map[string]ccxt.FundingRate, len(integrationSymbols))
	now := time.Now().UTC()
	for _, symbol := range integrationSymbols {
		rawRate, ok := bySymbol[symbol]
		if !ok {
			t.Errorf("funding rate %s: not present in FetchFundingRates response", symbol)
			continue
		}
		fixture[symbol] = rawRate

		mapped := mapFundingRate(rawRate, now)
		if err := validateFundingRate(rawRate, mapped, now); err != nil {
			t.Errorf("funding rate %s: %v", symbol, err)
		}
	}

	writeFixture(t, "funding_rates.json", fixture)
}

// TestIntegrationWatchTickers asserts WatchTickers -> domain.Ticker mapping:
// within a bounded window, at least integrationMinTickerUpdates valid
// updates per symbol with a positive, uncrossed quote and an exchange
// timestamp.
func TestIntegrationWatchTickers(t *testing.T) {
	c := newIntegrationClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), integrationWindow)
	defer cancel()

	counts := make(map[string]int, len(integrationSymbols))
	fixture := make(map[string]ccxt.Ticker, len(integrationSymbols))
	var lastErr error

	for ctx.Err() == nil {
		done := true
		for _, symbol := range integrationSymbols {
			if counts[symbol] < integrationMinTickerUpdates {
				done = false
			}
		}
		if done {
			break
		}

		tickers, err := c.api.WatchTickers(ccxt.WithWatchTickersSymbols(integrationSymbols))
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			lastErr = err
			continue
		}

		for _, rawTicker := range tickers.Tickers {
			mapped := mapTicker(rawTicker, time.Now().UTC())
			if err := validateTicker(rawTicker, mapped); err != nil {
				t.Errorf("ticker update: %v", err)
				continue
			}
			if _, ok := fixture[mapped.Symbol]; !ok {
				fixture[mapped.Symbol] = rawTicker
			}
			counts[mapped.Symbol]++
		}
	}

	for _, symbol := range integrationSymbols {
		if counts[symbol] < integrationMinTickerUpdates {
			detail := fmt.Sprintf("ticker %s: got %d valid updates in %s, want >= %d",
				symbol, counts[symbol], integrationWindow, integrationMinTickerUpdates)
			if lastErr != nil {
				detail = fmt.Sprintf("%s (last watch error: %v)", detail, lastErr)
			}
			t.Error(detail)
		}
	}

	if len(fixture) > 0 {
		writeFixture(t, "tickers.json", fixture)
	}
}
