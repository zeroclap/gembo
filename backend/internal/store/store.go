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

	// Funding Arbitrage
	SetFundingArbitrage(ctx context.Context, a domain.FundingArbitrage) error
	GetFundingArbitrage(ctx context.Context, symbol, exchangeA, exchangeB string) (*domain.FundingArbitrage, error)
	ListFundingArbitrage(ctx context.Context) ([]domain.FundingArbitrage, error)
	RemoveFundingArbitrage(ctx context.Context, symbol, exchangeA, exchangeB string) error

	// Spread Arbitrage
	SetSpreadArbitrage(ctx context.Context, a domain.SpreadArbitrage) error
	GetSpreadArbitrage(ctx context.Context, symbol, exchangeA, exchangeB string) (*domain.SpreadArbitrage, error)
	ListSpreadArbitrage(ctx context.Context) ([]domain.SpreadArbitrage, error)
	RemoveSpreadArbitrage(ctx context.Context, symbol, exchangeA, exchangeB string) error

	// Admin
	FlushAll(ctx context.Context) (int64, error)
}
