package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zeroclap/gembo/internal/domain"
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

func TestAgePercentiles(t *testing.T) {
	ages := make([]time.Duration, 0, 100)
	for i := 100; i >= 1; i-- {
		ages = append(ages, time.Duration(i)*time.Millisecond)
	}
	p50, p95, maxAge := agePercentiles(ages)
	if p50 != 50*time.Millisecond || p95 != 95*time.Millisecond || maxAge != 100*time.Millisecond {
		t.Fatalf("p50=%s p95=%s max=%s", p50, p95, maxAge)
	}
	if ages[0] != 100*time.Millisecond {
		t.Fatal("input must not be reordered")
	}
	one, _, _ := agePercentiles([]time.Duration{7 * time.Millisecond})
	if one != 7*time.Millisecond {
		t.Fatalf("single sample p50 = %s", one)
	}
}

func TestTickerResult(t *testing.T) {
	tk := domain.Ticker{Symbol: "BTC/USDT:USDT"}
	ages := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}

	if r := tickerResult(3, tk, ages, 0, ""); !r.ok || !strings.Contains(r.detail, "p95=") {
		t.Errorf("healthy result = %+v", r)
	}
	if r := tickerResult(3, tk, ages, 1, ""); r.ok || !strings.Contains(r.detail, "without source time") {
		t.Errorf("missing source time must fail, got %+v", r)
	}
	if r := tickerResult(0, domain.Ticker{}, nil, 0, "no ticker before timeout"); r.ok {
		t.Errorf("no tickers must fail, got %+v", r)
	}
}
