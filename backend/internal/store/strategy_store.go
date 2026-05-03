package store

import (
	"context"

	"github.com/zeroclap/gembo/internal/domain"
)

// StrategyStore defines the persistence interface for automated trading strategies.
type StrategyStore interface {
	// CreateStrategy inserts a new strategy and populates generated fields.
	CreateStrategy(ctx context.Context, s *domain.Strategy) error
	// GetStrategy returns a strategy by ID scoped to the given user, or nil if not found.
	GetStrategy(ctx context.Context, userID, id string) (*domain.Strategy, error)
	// ListStrategies returns all strategies for the given user.
	ListStrategies(ctx context.Context, userID string) ([]domain.Strategy, error)
	// ListAllStrategies returns all enabled strategies across all users (used by runner).
	ListAllStrategies(ctx context.Context) ([]domain.Strategy, error)
	// UpdateStrategy updates a strategy's mutable fields and returns true when found.
	UpdateStrategy(ctx context.Context, s *domain.Strategy) (bool, error)
	// DeleteStrategy removes a strategy by ID for a user and returns true when deleted.
	DeleteStrategy(ctx context.Context, userID, id string) (bool, error)
}
