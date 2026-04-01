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

func TestFundingArbitrageKeys(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "FundingArbitragePairs",
			fn:       FundingArbitragePairs,
			expected: "gembo:arbitrage:funding:pairs",
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

func TestFundingArbitragePairKey(t *testing.T) {
	got := FundingArbitragePair("BTC/USDT:USDT", "bybit", "okx")
	expected := "gembo:arbitrage:funding:BTC/USDT:USDT|bybit|okx"
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
