package arbitrage

import (
	"errors"
	"log/slog"
	"time"

	"github.com/zeroclap/gembo/internal/domain"
)

// usableTickers returns the tickers that are valid and fresh at now. Dropped
// quotes are summarised per exchange: invalid quotes (notably an exchange that
// reports no source time) are logged at Warn so they cannot disappear
// silently, while ordinary staleness is logged at Debug.
func usableTickers(logger *slog.Logger, exchangeID string, tickers []domain.Ticker, now time.Time, p domain.QualityPolicy) []domain.Ticker {
	out := make([]domain.Ticker, 0, len(tickers))
	var stale, missingSource, otherInvalid int
	var firstInvalid error
	var firstInvalidSymbol string
	for _, t := range tickers {
		if err := t.Validate(now, p); err != nil {
			if errors.Is(err, domain.ErrMissingSource) {
				missingSource++
			} else {
				otherInvalid++
			}
			if firstInvalid == nil {
				firstInvalid, firstInvalidSymbol = err, t.Symbol
			}
			continue
		}
		if t.IsStale(now, p) {
			stale++
			continue
		}
		out = append(out, t)
	}
	if missingSource+otherInvalid > 0 {
		logger.Warn("dropped invalid tickers",
			"exchange", exchangeID,
			"missing_source_time", missingSource,
			"other_invalid", otherInvalid,
			"kept", len(out),
			"first_symbol", firstInvalidSymbol,
			"first_reason", firstInvalid,
		)
	}
	if stale > 0 {
		logger.Debug("dropped stale tickers",
			"exchange", exchangeID, "stale", stale, "kept", len(out), "max_age", p.MaxTickerAge)
	}
	return out
}

// freshFunding returns the funding rates fetched within the policy's max age.
func freshFunding(logger *slog.Logger, exchangeID string, rates []domain.FundingRate, now time.Time, p domain.QualityPolicy) []domain.FundingRate {
	out := make([]domain.FundingRate, 0, len(rates))
	for _, fr := range rates {
		if fr.IsStale(now, p) {
			continue
		}
		out = append(out, fr)
	}
	if dropped := len(rates) - len(out); dropped > 0 {
		logger.Debug("dropped stale funding rates",
			"exchange", exchangeID, "stale", dropped, "kept", len(out), "max_age", p.MaxFundingAge)
	}
	return out
}
