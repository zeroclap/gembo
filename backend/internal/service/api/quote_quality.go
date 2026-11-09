package api

import (
	"time"

	"github.com/zeroclap/gembo/internal/domain"
)

// tickerView is a ticker plus its computed quality flags.
type tickerView struct {
	domain.Ticker
	AgeMs         int64  `json:"age_ms"` // now - source_time; 0 when source_time is missing
	Stale         bool   `json:"stale"`  // source age exceeds the max; false when source_time is missing
	Valid         bool   `json:"valid"`  // structurally valid, including a present source_time
	Usable        bool   `json:"usable"` // valid and not stale: what the arbitrage worker accepts
	InvalidReason string `json:"invalid_reason,omitempty"`
}

// fundingView is a funding rate plus its computed freshness flags.
type fundingView struct {
	domain.FundingRate
	AgeMs int64 `json:"age_ms"` // now - fetched_at
	Stale bool  `json:"stale"`
}

func (h *Handler) viewTicker(t domain.Ticker, now time.Time) tickerView {
	v := tickerView{
		Ticker: t,
		AgeMs:  t.SourceAge(now).Milliseconds(),
		Stale:  t.IsStale(now, h.quality),
		Valid:  true,
		Usable: t.Usable(now, h.quality),
	}
	if err := t.Validate(now, h.quality); err != nil {
		v.Valid = false
		v.InvalidReason = err.Error()
	}
	return v
}

func (h *Handler) viewTickers(ts []domain.Ticker, now time.Time) []tickerView {
	out := make([]tickerView, len(ts))
	for i, t := range ts {
		out[i] = h.viewTicker(t, now)
	}
	return out
}

func (h *Handler) viewFunding(f domain.FundingRate, now time.Time) fundingView {
	return fundingView{
		FundingRate: f,
		AgeMs:       f.FetchAge(now).Milliseconds(),
		Stale:       f.IsStale(now, h.quality),
	}
}

func (h *Handler) viewFundings(fs []domain.FundingRate, now time.Time) []fundingView {
	out := make([]fundingView, len(fs))
	for i, f := range fs {
		out[i] = h.viewFunding(f, now)
	}
	return out
}
