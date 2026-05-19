package rediskey

import "testing"

func TestPositionKeys(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "PositionPending",
			fn:       PositionPending,
			expected: "gembo:positions:pending",
		},
		{
			name:     "PositionClosing",
			fn:       PositionClosing,
			expected: "gembo:positions:closing",
		},
		{
			name:     "PositionUpdates",
			fn:       PositionUpdates,
			expected: "gembo:positions:updates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestArbitrageKeys(t *testing.T) {
	got := ArbitragePairs()
	expected := "gembo:arbitrage:pairs"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = ArbitragePubSub()
	expected = "gembo:arbitrage:updates"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestArbitragePairKey(t *testing.T) {
	got := ArbitragePair("funding", "BTC/USDT:USDT", "bybit", "okx")
	expected := "gembo:arbitrage:BTC/USDT:USDT|funding|bybit|okx"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = ArbitragePair("spread", "ETH/USDT:USDT", "binance", "okx")
	expected = "gembo:arbitrage:ETH/USDT:USDT|spread|binance|okx"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMarketKeys(t *testing.T) {
	got := Market("bybit", "BTC/USDT:USDT")
	expected := "gembo:bybit:market:BTC/USDT:USDT"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = Markets("bybit")
	expected = "gembo:bybit:markets"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestTickerKeys(t *testing.T) {
	got := Ticker("okx", "ETH/USDT:USDT")
	expected := "gembo:okx:ticker:ETH/USDT:USDT"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = Tickers("okx")
	expected = "gembo:okx:tickers"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = TickerPubSub("okx")
	expected = "gembo:okx:ticker:updates"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFundingRateKeys(t *testing.T) {
	got := FundingRate("gate", "SOL/USDT:USDT")
	expected := "gembo:gate:funding:SOL/USDT:USDT"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = FundingRates("gate")
	expected = "gembo:gate:funding"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = FundingRatePubSub("gate")
	expected = "gembo:gate:funding:updates"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestBalanceKeys(t *testing.T) {
	got := Balance("bybit", "USDT")
	expected := "gembo:bybit:balance:USDT"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = Balances("bybit")
	expected = "gembo:bybit:balances"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	got = BalancePubSub("bybit")
	expected = "gembo:bybit:balance:updates"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
