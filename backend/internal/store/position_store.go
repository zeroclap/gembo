package store

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// PositionStore defines the storage interface for position management.
// Implemented by PostgreSQL (durable storage for positions, orders, funding payments).
type PositionStore interface {
	// Positions
	CreatePosition(ctx context.Context, p *domain.Position) error
	GetPosition(ctx context.Context, userID, id string) (*domain.Position, error)
	ListPositions(ctx context.Context, f domain.PositionFilter) ([]domain.Position, error)
	UpdatePosition(ctx context.Context, p *domain.Position) error
	DeletePosition(ctx context.Context, userID, id string) error

	// Orders
	CreateOrder(ctx context.Context, o *domain.ExchangeOrder) error
	UpdateOrder(ctx context.Context, o *domain.ExchangeOrder) error
	ListOrdersByPosition(ctx context.Context, userID, positionID string) ([]domain.ExchangeOrder, error)

	// Funding payments
	CreateFundingPayment(ctx context.Context, fp *domain.FundingPayment) error
	ListFundingPayments(ctx context.Context, userID, positionID string) ([]domain.FundingPayment, error)
	SumFundingPnL(ctx context.Context, userID, positionID string) (decimal.Decimal, error)
	LatestFundingPayment(ctx context.Context, userID, positionID, exchangeID string) (*domain.FundingPayment, error)
}
