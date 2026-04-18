package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/zeroclap/gembo/internal/domain"
)

func (s *Store) encryptCredentialValue(value string) (string, error) {
	if s.credentialCipher == nil {
		return value, nil
	}
	encrypted, err := s.credentialCipher.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("encrypt credential value: %w", err)
	}
	return encrypted, nil
}

func (s *Store) decryptCredentialValue(value string) (string, error) {
	if s.credentialCipher == nil {
		return value, nil
	}
	decrypted, err := s.credentialCipher.Decrypt(value)
	if err != nil {
		return "", fmt.Errorf("decrypt credential value: %w", err)
	}
	return decrypted, nil
}

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

// GetUserByID returns a user by ID, or nil if not found.
func (s *Store) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	const q = `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)

	var u domain.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
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

// ListExchangeCredentialsByUserID returns all credentials for a user.
func (s *Store) ListExchangeCredentialsByUserID(ctx context.Context, userID string) ([]domain.ExchangeCredential, error) {
	const q = `
		SELECT id, user_id, exchange_id, title, api_key, api_secret, COALESCE(passphrase, ''), is_default, created_at, updated_at
		FROM exchange_credentials
		WHERE user_id = $1
		ORDER BY exchange_id ASC, created_at ASC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list exchange credentials by user id: %w", err)
	}
	defer rows.Close()

	creds := make([]domain.ExchangeCredential, 0)
	for rows.Next() {
		var c domain.ExchangeCredential
		if err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.ExchangeID,
			&c.Title,
			&c.APIKey,
			&c.APISecret,
			&c.Passphrase,
			&c.IsDefault,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan exchange credential: %w", err)
		}

		apiKey, err := s.decryptCredentialValue(c.APIKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt exchange credential api key: %w", err)
		}
		apiSecret, err := s.decryptCredentialValue(c.APISecret)
		if err != nil {
			return nil, fmt.Errorf("decrypt exchange credential api secret: %w", err)
		}
		passphrase, err := s.decryptCredentialValue(c.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("decrypt exchange credential passphrase: %w", err)
		}
		c.APIKey = apiKey
		c.APISecret = apiSecret
		c.Passphrase = passphrase
		creds = append(creds, c)
	}

	if creds == nil {
		return []domain.ExchangeCredential{}, nil
	}
	return creds, nil
}

// CreateExchangeCredential inserts a new exchange credential.
func (s *Store) CreateExchangeCredential(ctx context.Context, c *domain.ExchangeCredential) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create exchange credential begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if c.IsDefault {
		if _, err := tx.Exec(ctx,
			`UPDATE exchange_credentials SET is_default = FALSE, updated_at = now() WHERE user_id = $1 AND exchange_id = $2 AND is_default = TRUE`,
			c.UserID,
			c.ExchangeID,
		); err != nil {
			return fmt.Errorf("create exchange credential clear defaults: %w", err)
		}
	}

	encryptedAPIKey, err := s.encryptCredentialValue(c.APIKey)
	if err != nil {
		return fmt.Errorf("create exchange credential encrypt api key: %w", err)
	}
	encryptedAPISecret, err := s.encryptCredentialValue(c.APISecret)
	if err != nil {
		return fmt.Errorf("create exchange credential encrypt api secret: %w", err)
	}
	encryptedPassphrase, err := s.encryptCredentialValue(c.Passphrase)
	if err != nil {
		return fmt.Errorf("create exchange credential encrypt passphrase: %w", err)
	}

	const q = `
		INSERT INTO exchange_credentials (user_id, exchange_id, title, api_key, api_secret, passphrase, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`
	row := tx.QueryRow(ctx, q,
		c.UserID,
		c.ExchangeID,
		c.Title,
		encryptedAPIKey,
		encryptedAPISecret,
		nullableString(encryptedPassphrase),
		c.IsDefault,
	)
	if err := row.Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return fmt.Errorf("create exchange credential: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create exchange credential commit: %w", err)
	}
	return nil
}

// UpdateExchangeCredential updates an existing credential and returns true when found.
func (s *Store) UpdateExchangeCredential(ctx context.Context, c *domain.ExchangeCredential) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("update exchange credential begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if c.IsDefault {
		if _, err := tx.Exec(ctx,
			`UPDATE exchange_credentials SET is_default = FALSE, updated_at = now() WHERE user_id = $1 AND exchange_id = $2 AND id <> $3 AND is_default = TRUE`,
			c.UserID,
			c.ExchangeID,
			c.ID,
		); err != nil {
			return false, fmt.Errorf("update exchange credential clear defaults: %w", err)
		}
	}

	encryptedAPIKey, err := s.encryptCredentialValue(c.APIKey)
	if err != nil {
		return false, fmt.Errorf("update exchange credential encrypt api key: %w", err)
	}
	encryptedAPISecret, err := s.encryptCredentialValue(c.APISecret)
	if err != nil {
		return false, fmt.Errorf("update exchange credential encrypt api secret: %w", err)
	}
	encryptedPassphrase, err := s.encryptCredentialValue(c.Passphrase)
	if err != nil {
		return false, fmt.Errorf("update exchange credential encrypt passphrase: %w", err)
	}

	const q = `
		UPDATE exchange_credentials
		SET exchange_id = $3,
			title = $4,
			api_key = $5,
			api_secret = $6,
			passphrase = $7,
			is_default = $8,
			updated_at = now()
		WHERE id = $1 AND user_id = $2`
	result, err := tx.Exec(ctx, q,
		c.ID,
		c.UserID,
		c.ExchangeID,
		c.Title,
		encryptedAPIKey,
		encryptedAPISecret,
		nullableString(encryptedPassphrase),
		c.IsDefault,
	)
	if err != nil {
		return false, fmt.Errorf("update exchange credential: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("update exchange credential commit: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// DeleteExchangeCredential removes a credential by id for a user and returns true when deleted.
func (s *Store) DeleteExchangeCredential(ctx context.Context, userID, credentialID string) (bool, error) {
	const q = `DELETE FROM exchange_credentials WHERE user_id = $1 AND id = $2`
	result, err := s.pool.Exec(ctx, q, userID, credentialID)
	if err != nil {
		return false, fmt.Errorf("delete exchange credential: %w", err)
	}
	return result.RowsAffected() > 0, nil
}
