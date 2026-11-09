package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
)

func TestHandleGetTicker_QualityFlags(t *testing.T) {
	now := time.Now().UTC()
	mk := func(src time.Time, bid, ask string) domain.Ticker {
		return domain.Ticker{
			ExchangeID: "bybit", Symbol: "BTC/USDT:USDT",
			Bid: decimal.RequireFromString(bid), Ask: decimal.RequireFromString(ask),
			SourceTime: src, ReceivedAt: now,
		}
	}
	tests := []struct {
		name        string
		tk          domain.Ticker
		wantStale   bool
		wantValid   bool
		wantInvalid string
	}{
		{"fresh", mk(now.Add(-time.Second), "100", "101"), false, true, ""},
		{"stale", mk(now.Add(-time.Minute), "100", "101"), true, true, ""},
		{"crossed", mk(now, "102", "101"), false, false, domain.ErrCrossedQuote.Error()},
		{"missing source", mk(time.Time{}, "100", "101"), false, false, domain.ErrMissingSource.Error()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg := base.NewRegistry()
			reg.Register(&mockExchange{id: "bybit"})
			st := &mockStore{tickers: map[string]domain.Ticker{marketKey("bybit", "BTC/USDT:USDT"): tc.tk}}
			h := NewHandler(reg, st)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.SetPathValue("exchange", "bybit")
			req.SetPathValue("symbol", "BTC/USDT:USDT")
			rec := httptest.NewRecorder()
			h.handleGetTicker(rec, req)

			var env struct {
				Data tickerView `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if env.Data.Stale != tc.wantStale || env.Data.Valid != tc.wantValid || env.Data.InvalidReason != tc.wantInvalid {
				t.Fatalf("flags = stale:%v valid:%v reason:%q", env.Data.Stale, env.Data.Valid, env.Data.InvalidReason)
			}
			if wantUsable := tc.wantValid && !tc.wantStale; env.Data.Usable != wantUsable {
				t.Fatalf("usable = %v, want %v", env.Data.Usable, wantUsable)
			}
			if env.Data.SourceTime.IsZero() != tc.tk.SourceTime.IsZero() {
				t.Fatal("source_time not round-tripped in response")
			}
		})
	}
}

func TestViewFunding_Stale(t *testing.T) {
	h := NewHandler(base.NewRegistry(), &mockStore{})
	now := time.Now().UTC()
	fresh := h.viewFunding(domain.FundingRate{FetchedAt: now.Add(-time.Second)}, now)
	stale := h.viewFunding(domain.FundingRate{FetchedAt: now.Add(-time.Hour)}, now)
	if fresh.Stale || !stale.Stale {
		t.Fatalf("stale flags: fresh=%v stale=%v", fresh.Stale, stale.Stale)
	}
}
