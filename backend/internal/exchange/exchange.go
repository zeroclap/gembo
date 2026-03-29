package exchange

import (
	"context"

	"github.com/zeroclap/gembo/internal/domain"
)

// Exchange is the contract every exchange adapter must satisfy.
type Exchange interface {
	// ID returns the unique identifier, e.g. "bybit".
	ID() string

	// Run establishes underlying connections via CCXT and drives the
	// adapter's internal event loops. It blocks until ctx is cancelled.
	Run(ctx context.Context) error

	// Markets returns the current list of available markets.
	// Implemented via CCXT REST: exchange.LoadMarkets() / FetchMarkets().
	Markets(ctx context.Context) ([]domain.Market, error)

	// Tickers subscribes to real-time ticker updates via CCXT WebSocket.
	// Returns a channel that emits updates until ctx is cancelled.
	// symbols is the list of market symbols to subscribe to.
	Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error)

	// FundingRates returns current funding rates via periodic REST polling.
	// Returns a channel that emits updates until ctx is cancelled.
	FundingRates(ctx context.Context) (<-chan domain.FundingRate, error)
}
