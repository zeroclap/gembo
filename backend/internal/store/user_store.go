package store

import (
	"context"

	"github.com/zeroclap/gembo/internal/domain"
)

// UserStore defines the storage interface for user management.
// Implemented by PostgreSQL.
type UserStore interface {
	// GetUserByEmail returns a user by email, or nil if not found.
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	// GetUserByID returns a user by ID, or nil if not found.
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	// CreateUser inserts a new user record.
	CreateUser(ctx context.Context, u *domain.User) error
	// ListExchangeCredentialsByUserID returns all credentials for the user.
	ListExchangeCredentialsByUserID(ctx context.Context, userID string) ([]domain.ExchangeCredential, error)
	// CreateExchangeCredential inserts a new exchange credential.
	CreateExchangeCredential(ctx context.Context, c *domain.ExchangeCredential) error
	// UpdateExchangeCredential updates an existing credential and returns true when found.
	UpdateExchangeCredential(ctx context.Context, c *domain.ExchangeCredential) (bool, error)
	// DeleteExchangeCredential removes a credential by id for a user and returns true when deleted.
	DeleteExchangeCredential(ctx context.Context, userID, credentialID string) (bool, error)
}
