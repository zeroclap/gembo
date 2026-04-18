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
	// CreateUser inserts a new user record.
	CreateUser(ctx context.Context, u *domain.User) error
}
