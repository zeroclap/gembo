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
}
