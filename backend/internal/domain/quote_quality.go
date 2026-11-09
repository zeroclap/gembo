package domain

import (
	"errors"
	"time"
)

// Default quote-quality thresholds.
//
// PROVISIONAL: these are conservative defaults chosen before the Bybit
// source→receive age measurement requested in issue #6 has been recorded.
// Replace them with values derived from the measured p95/max age and record
// the measurement here. The baseline e2e (#1, §3) saw served quotes up to
// ~59 s old, so anything below the 60 s Redis TTL is a strict improvement.
// Measure with: go run ./cmd/exchange-probe -ws -exchanges bybit
// -symbols BTC/USDT:USDT,ETH/USDT:USDT -ticker-samples 200 -timeout 5m
//
// Rollout: tickers written before source_time existed fail Validate until the
// collectors rewrite them (Redis TTL is 60 s, so they age out on their own);
// deploy the collectors before the worker and API. Legacy funding rows
// (no TTL) lack fetched_at, so FundingRate falls back to updated_at, which
// carried the same local poll time; they are judged by their real age.
const (
	DefaultMaxTickerAge  = 10 * time.Second
	DefaultMaxFundingAge = 150 * time.Second // 2.5x the 60 s funding poll SLA
	DefaultMaxFutureSkew = 2 * time.Second
)

// Reasons a ticker is invalid.
var (
	ErrNonPositiveBid = errors.New("bid must be > 0")
	ErrNonPositiveAsk = errors.New("ask must be > 0")
	ErrCrossedQuote   = errors.New("bid must be < ask")
	ErrMissingSource  = errors.New("source time missing")
	ErrSourceInFuture = errors.New("source time in the future beyond skew tolerance")
)

// QualityPolicy holds the thresholds used to validate and flag quotes.
type QualityPolicy struct {
	MaxTickerAge  time.Duration // max source→now age before a ticker is stale
	MaxFundingAge time.Duration // max fetch→now age before a funding rate is stale
	MaxFutureSkew time.Duration // tolerated source-clock lead over local clock
}

// DefaultQualityPolicy returns the default thresholds.
func DefaultQualityPolicy() QualityPolicy {
	return QualityPolicy{
		MaxTickerAge:  DefaultMaxTickerAge,
		MaxFundingAge: DefaultMaxFundingAge,
		MaxFutureSkew: DefaultMaxFutureSkew,
	}
}

// Validate returns nil if the ticker is structurally valid at now: positive
// bid and ask, bid < ask, and a source time that is present and not in the
// future beyond the skew tolerance.
func (t Ticker) Validate(now time.Time, p QualityPolicy) error {
	if !t.Bid.IsPositive() {
		return ErrNonPositiveBid
	}
	if !t.Ask.IsPositive() {
		return ErrNonPositiveAsk
	}
	if !t.Bid.LessThan(t.Ask) {
		return ErrCrossedQuote
	}
	if t.SourceTime.IsZero() {
		return ErrMissingSource
	}
	if t.SourceTime.After(now.Add(p.MaxFutureSkew)) {
		return ErrSourceInFuture
	}
	return nil
}

// SourceAge is the age of the ticker at now, measured from exchange time.
// It is zero when the source time is missing.
func (t Ticker) SourceAge(now time.Time) time.Duration {
	if t.SourceTime.IsZero() {
		return 0
	}
	return now.Sub(t.SourceTime)
}

// IsStale reports whether the ticker's source age exceeds MaxTickerAge.
// A ticker without a source time is never reported stale here; it fails
// Validate instead.
func (t Ticker) IsStale(now time.Time, p QualityPolicy) bool {
	return !t.SourceTime.IsZero() && t.SourceAge(now) > p.MaxTickerAge
}

// Usable reports whether the ticker is valid and fresh enough for arbitrage.
func (t Ticker) Usable(now time.Time, p QualityPolicy) bool {
	return t.Validate(now, p) == nil && !t.IsStale(now, p)
}

// fetchTime is when the funding rate was polled locally. Rows written before
// fetched_at existed carry the same value in updated_at.
func (f FundingRate) fetchTime() time.Time {
	if f.FetchedAt.IsZero() {
		return f.UpdatedAt
	}
	return f.FetchedAt
}

// FetchAge is the age of the funding rate at now, measured from fetch time.
// It is zero when no fetch time is recorded.
func (f FundingRate) FetchAge(now time.Time) time.Duration {
	ft := f.fetchTime()
	if ft.IsZero() {
		return 0
	}
	return now.Sub(ft)
}

// IsStale reports whether the funding rate was fetched longer ago than
// MaxFundingAge. A missing fetch time is stale.
func (f FundingRate) IsStale(now time.Time, p QualityPolicy) bool {
	return f.fetchTime().IsZero() || f.FetchAge(now) > p.MaxFundingAge
}
