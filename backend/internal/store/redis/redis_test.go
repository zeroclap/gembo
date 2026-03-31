package redis

import "testing"

func TestSortedExchangeIDs(t *testing.T) {
	tests := []struct {
		a, b  string
		wantA string
		wantB string
	}{
		{"binance", "bybit", "binance", "bybit"},
		{"bybit", "binance", "binance", "bybit"},
		{"okx", "binance", "binance", "okx"},
		{"binance", "binance", "binance", "binance"},
		{"a", "z", "a", "z"},
		{"z", "a", "a", "z"},
	}

	for _, tt := range tests {
		gotA, gotB := sortedExchangeIDs(tt.a, tt.b)
		if gotA != tt.wantA || gotB != tt.wantB {
			t.Errorf("sortedExchangeIDs(%q, %q) = (%q, %q), want (%q, %q)",
				tt.a, tt.b, gotA, gotB, tt.wantA, tt.wantB)
		}
	}
}
