package ccxtutil

import (
	"github.com/zeroclap/gembo/internal/domain"
)

// ApplyTickerUpdate merges a full ticker update (from WatchTickers) into the
// in-memory state and returns the merged result. Fields from WatchTickers
// (Price, High24h, Low24h, Volume24h, Change24h) always overwrite state.
// Bid/Ask are only overwritten if the update carries non-zero values, preserving
// values previously received from WatchBidsAsks.
func ApplyTickerUpdate(state map[string]*domain.Ticker, t domain.Ticker) *domain.Ticker {
	existing, ok := state[t.Symbol]
	if !ok {
		cp := t
		state[t.Symbol] = &cp
		return &cp
	}

	existing.Price = t.Price
	existing.High24h = t.High24h
	existing.Low24h = t.Low24h
	existing.Volume24h = t.Volume24h
	existing.Change24h = t.Change24h
	existing.UpdatedAt = t.UpdatedAt

	// Only overwrite bid/ask if WatchTickers actually provided them (non-zero).
	if !t.Bid.IsZero() {
		existing.Bid = t.Bid
	}
	if !t.Ask.IsZero() {
		existing.Ask = t.Ask
	}

	return existing
}

// ApplyBidAskUpdate merges a bid/ask update (from WatchBidsAsks) into the
// in-memory state and returns the merged result. Only Bid, Ask, and UpdatedAt
// are overwritten; all other fields are preserved from previous WatchTickers data.
func ApplyBidAskUpdate(state map[string]*domain.Ticker, t domain.Ticker) *domain.Ticker {
	existing, ok := state[t.Symbol]
	if !ok {
		cp := t
		state[t.Symbol] = &cp
		return &cp
	}

	existing.Bid = t.Bid
	existing.Ask = t.Ask
	existing.UpdatedAt = t.UpdatedAt

	return existing
}
