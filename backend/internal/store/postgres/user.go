package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/zeroclap/gembo/internal/domain"
)

// GetUserByEmail returns a user by email, or nil if not found.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1`
	row := s.pool.QueryRow(ctx, q, email)

	var u domain.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

// CreateUser inserts a new user record and populates generated fields.
func (s *Store) CreateUser(ctx context.Context, u *domain.User) error {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, u.Email, u.PasswordHash)
	if err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}
