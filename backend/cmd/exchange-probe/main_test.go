package main

import (
	"testing"
)

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "simple",
			in:   "bybit,okx",
			want: []string{"bybit", "okx"},
		},
		{
			name: "trim and dedupe",
			in:   " bybit , okx,bybit, ,okx ",
			want: []string{"bybit", "okx"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCSV(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got %d want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("item %d mismatch: got %q want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
