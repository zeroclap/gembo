package store

import (
	"context"

	"github.com/zeroclap/gembo/internal/domain"
)

// Store defines the storage interface for market data.
type Store interface {
	// Markets
	SetMarket(ctx context.Context, m domain.Market) error
	GetMarket(ctx context.Context, exchangeID, symbol string) (*domain.Market, error)
	ListMarkets(ctx context.Context, exchangeID string) ([]domain.Market, error)

	// Tickers
	SetTicker(ctx context.Context, t domain.Ticker) error
	GetTicker(ctx context.Context, exchangeID, symbol string) (*domain.Ticker, error)
	ListTickers(ctx context.Context, exchangeID string) ([]domain.Ticker, error)

	// Funding Rates
	SetFundingRate(ctx context.Context, fr domain.FundingRate) error
	GetFundingRate(ctx context.Context, exchangeID, symbol string) (*domain.FundingRate, error)
	ListFundingRates(ctx context.Context, exchangeID string) ([]domain.FundingRate, error)

	// Balances
	SetBalance(ctx context.Context, b domain.Balance) error
	ListBalances(ctx context.Context, userID, exchangeID string) ([]domain.Balance, error)

	// Arbitrage (unified collection across all strategy types)
	SetArbitrage(ctx context.Context, a domain.Arbitrage) error
	GetArbitrage(ctx context.Context, arbType domain.ArbitrageType, symbol, exchangeA, exchangeB string) (*domain.Arbitrage, error)
	ListArbitrage(ctx context.Context) ([]domain.Arbitrage, error)
	RemoveArbitrage(ctx context.Context, arbType domain.ArbitrageType, symbol, exchangeA, exchangeB string) error

	// Ticker Snapshots — per-exchange time-series of market state, stored as a
	// rolling 1h window in a Redis sorted set (score = unix timestamp).
	AppendTickerSnapshot(ctx context.Context, s domain.TickerSnapshot) error
	GetTickerSnapshots(ctx context.Context, exchangeID, symbol string, from, to int64) ([]domain.TickerSnapshot, error)

	// Admin
	FlushAll(ctx context.Context) (int64, error)
}
