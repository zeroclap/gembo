package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// CreateFundingPayment inserts a new funding payment and populates fp.ID and fp.CreatedAt.
func (s *Store) CreateFundingPayment(ctx context.Context, fp *domain.FundingPayment) error {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO funding_payments (
			position_id, exchange_id, leg, symbol, rate, payment, funded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		fp.PositionID, fp.ExchangeID, fp.Leg, fp.Symbol,
		fp.Rate, fp.Payment, fp.FundedAt,
	)
	if err := row.Scan(&fp.ID, &fp.CreatedAt); err != nil {
		return fmt.Errorf("postgres create funding payment: %w", err)
	}
	return nil
}

// ListFundingPayments returns all funding payments for a position, ordered by funded_at desc.
func (s *Store) ListFundingPayments(ctx context.Context, positionID string) ([]domain.FundingPayment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, position_id, exchange_id, leg, symbol,
			rate, payment, funded_at, created_at
		FROM funding_payments
		WHERE position_id = $1
		ORDER BY funded_at DESC`, positionID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres list funding payments: %w", err)
	}
	defer rows.Close()

	var payments []domain.FundingPayment
	for rows.Next() {
		var fp domain.FundingPayment
		if err := rows.Scan(
			&fp.ID, &fp.PositionID, &fp.ExchangeID, &fp.Leg, &fp.Symbol,
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
func (s *Store) SumFundingPnL(ctx context.Context, positionID string) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(payment), 0) FROM funding_payments WHERE position_id = $1`,
		positionID,
	).Scan(&sum)
	if err != nil {
		return decimal.Zero, fmt.Errorf("postgres sum funding pnl: %w", err)
	}
	return sum, nil
}

// LatestFundingPayment returns the most recent funding payment for a position on a specific exchange.
// Returns nil, nil if no payment exists.
func (s *Store) LatestFundingPayment(ctx context.Context, positionID, exchangeID string) (*domain.FundingPayment, error) {
	var fp domain.FundingPayment
	err := s.pool.QueryRow(ctx, `
		SELECT
			id, position_id, exchange_id, leg, symbol,
			rate, payment, funded_at, created_at
		FROM funding_payments
		WHERE position_id = $1 AND exchange_id = $2
		ORDER BY funded_at DESC
		LIMIT 1`, positionID, exchangeID,
	).Scan(
		&fp.ID, &fp.PositionID, &fp.ExchangeID, &fp.Leg, &fp.Symbol,
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
