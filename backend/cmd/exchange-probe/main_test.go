package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestFetchEgressCountry(t *testing.T) {
	t.Run("trims and uppercases a plain-text body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(" fr \n"))
		}))
		defer srv.Close()

		got, err := fetchEgressCountry(context.Background(), srv.URL, time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "FR" {
			t.Fatalf("got %q, want %q", got, "FR")
		}
	})

	t.Run("errors on a non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		if _, err := fetchEgressCountry(context.Background(), srv.URL, time.Second); err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}
