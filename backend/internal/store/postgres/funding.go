package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// CreateFundingPayment inserts a new funding payment and populates fp.ID and fp.CreatedAt.
func (s *Store) CreateFundingPayment(ctx context.Context, fp *domain.FundingPayment) error {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO funding_payments (
			user_id, position_id, exchange_id, leg, symbol, rate, payment, funded_at, external_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`,
		fp.UserID, fp.PositionID, fp.ExchangeID, fp.Leg, fp.Symbol,
		fp.Rate, fp.Payment, fp.FundedAt, nullableString(fp.ExternalID),
	)
	if err := row.Scan(&fp.ID, &fp.CreatedAt); err != nil {
		return fmt.Errorf("postgres create funding payment: %w", err)
	}
	return nil
}

// CreateFundingPaymentIfNotExists inserts a funding payment, returning false
// when an equivalent external payment already exists.
func (s *Store) CreateFundingPaymentIfNotExists(ctx context.Context, fp *domain.FundingPayment) (bool, error) {
	err := s.CreateFundingPayment(ctx, fp)
	if err == nil {
		return true, nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false, err
	}
	if pgErr.Code == "23505" && pgErr.ConstraintName == "uq_funding_payments_external" {
		return false, nil
	}
	return false, err
}

// ListFundingPayments returns all funding payments for a position, ordered by funded_at desc.
func (s *Store) ListFundingPayments(ctx context.Context, userID, positionID string) ([]domain.FundingPayment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, COALESCE(external_id, ''), user_id, position_id, exchange_id, leg, symbol,
			rate, payment, funded_at, created_at
		FROM funding_payments
		WHERE user_id = $1 AND position_id = $2
		ORDER BY funded_at DESC`, userID, positionID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres list funding payments: %w", err)
	}
	defer rows.Close()

	var payments []domain.FundingPayment
	for rows.Next() {
		var fp domain.FundingPayment
		if err := rows.Scan(
			&fp.ID, &fp.ExternalID, &fp.UserID, &fp.PositionID, &fp.ExchangeID, &fp.Leg, &fp.Symbol,
			&fp.Rate, &fp.Payment, &fp.FundedAt, &fp.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres scan funding payment: %w", err)
		}
		payments = append(payments, fp)
	}

	if payments == nil {
		payments = []domain.FundingPayment{}
	}
	return payments, nil
}

// SumFundingPnL returns the total sum of all funding payments for a position.
func (s *Store) SumFundingPnL(ctx context.Context, userID, positionID string) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(payment), 0) FROM funding_payments WHERE user_id = $1 AND position_id = $2`,
		userID, positionID,
	).Scan(&sum)
	if err != nil {
		return decimal.Zero, fmt.Errorf("postgres sum funding pnl: %w", err)
	}
	return sum, nil
}

// LatestFundingPayment returns the most recent funding payment for a position on a specific exchange.
// Returns nil, nil if no payment exists.
func (s *Store) LatestFundingPayment(ctx context.Context, userID, positionID, exchangeID string) (*domain.FundingPayment, error) {
	var fp domain.FundingPayment
	err := s.pool.QueryRow(ctx, `
		SELECT
			id, COALESCE(external_id, ''), user_id, position_id, exchange_id, leg, symbol,
			rate, payment, funded_at, created_at
		FROM funding_payments
		WHERE user_id = $1 AND position_id = $2 AND exchange_id = $3
		ORDER BY funded_at DESC
		LIMIT 1`, userID, positionID, exchangeID,
	).Scan(
		&fp.ID, &fp.ExternalID, &fp.UserID, &fp.PositionID, &fp.ExchangeID, &fp.Leg, &fp.Symbol,
		&fp.Rate, &fp.Payment, &fp.FundedAt, &fp.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres latest funding payment: %w", err)
	}
	return &fp, nil
}
