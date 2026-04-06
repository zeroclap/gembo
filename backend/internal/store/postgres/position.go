package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// CreatePosition inserts a new position and populates p.ID and p.CreatedAt.
func (s *Store) CreatePosition(ctx context.Context, p *domain.Position) error {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO positions (
			symbol, status, size_type, requested_size,
			long_exchange_id, short_exchange_id, snapshot
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		p.Symbol, p.Status, p.SizeType, p.RequestedSize,
		p.LongExchangeID, p.ShortExchangeID, p.Snapshot,
	)
	if err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("postgres create position: %w", err)
	}
	return nil
}

// GetPosition retrieves a position by ID.
func (s *Store) GetPosition(ctx context.Context, id string) (*domain.Position, error) {
	var p domain.Position
	var snapshot []byte
	err := s.pool.QueryRow(ctx, `
		SELECT
			id, symbol, status, size_type, requested_size,
			long_exchange_id, COALESCE(long_entry_price, 0), COALESCE(long_filled_qty, 0),
			COALESCE(long_entry_cost, 0), COALESCE(long_entry_fees, 0),
			short_exchange_id, COALESCE(short_entry_price, 0), COALESCE(short_filled_qty, 0),
			COALESCE(short_entry_cost, 0), COALESCE(short_entry_fees, 0),
			COALESCE(long_exit_price, 0), COALESCE(long_close_fees, 0),
			COALESCE(short_exit_price, 0), COALESCE(short_close_fees, 0),
			snapshot,
			COALESCE(long_mark_price, 0), COALESCE(short_mark_price, 0),
			COALESCE(long_unrealized_pnl, 0), COALESCE(short_unrealized_pnl, 0),
			COALESCE(total_funding_pnl, 0),
			COALESCE(long_liquidation_price, 0), COALESCE(short_liquidation_price, 0),
			COALESCE(long_leverage, 0), COALESCE(short_leverage, 0),
			COALESCE(long_margin_mode, ''), COALESCE(short_margin_mode, ''),
			COALESCE(long_notional, 0), COALESCE(short_notional, 0),
			COALESCE(close_reason, ''),
			COALESCE(error_message, ''),
			created_at, opened_at, closed_at, updated_at
		FROM positions WHERE id = $1`, id,
	).Scan(
		&p.ID, &p.Symbol, &p.Status, &p.SizeType, &p.RequestedSize,
		&p.LongExchangeID, &p.LongEntryPrice, &p.LongFilledQty,
		&p.LongEntryCost, &p.LongEntryFees,
		&p.ShortExchangeID, &p.ShortEntryPrice, &p.ShortFilledQty,
		&p.ShortEntryCost, &p.ShortEntryFees,
		&p.LongExitPrice, &p.LongCloseFees,
		&p.ShortExitPrice, &p.ShortCloseFees,
		&snapshot,
		&p.LongMarkPrice, &p.ShortMarkPrice,
		&p.LongUnrealizedPnL, &p.ShortUnrealizedPnL,
		&p.TotalFundingPnL,
		&p.LongLiquidationPrice, &p.ShortLiquidationPrice,
		&p.LongLeverage, &p.ShortLeverage,
		&p.LongMarginMode, &p.ShortMarginMode,
		&p.LongNotional, &p.ShortNotional,
		&p.CloseReason,
		&p.ErrorMessage,
		&p.CreatedAt, &p.OpenedAt, &p.ClosedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres get position: %w", err)
	}
	p.Snapshot = json.RawMessage(snapshot)
	return &p, nil
}

// ListPositions returns positions matching the given filter, ordered by created_at desc.
func (s *Store) ListPositions(ctx context.Context, f domain.PositionFilter) ([]domain.Position, error) {
	query := `
		SELECT
			id, symbol, status, size_type, requested_size,
			long_exchange_id, COALESCE(long_entry_price, 0), COALESCE(long_filled_qty, 0),
			COALESCE(long_entry_cost, 0), COALESCE(long_entry_fees, 0),
			short_exchange_id, COALESCE(short_entry_price, 0), COALESCE(short_filled_qty, 0),
			COALESCE(short_entry_cost, 0), COALESCE(short_entry_fees, 0),
			COALESCE(long_exit_price, 0), COALESCE(long_close_fees, 0),
			COALESCE(short_exit_price, 0), COALESCE(short_close_fees, 0),
			snapshot,
			COALESCE(long_mark_price, 0), COALESCE(short_mark_price, 0),
			COALESCE(long_unrealized_pnl, 0), COALESCE(short_unrealized_pnl, 0),
			COALESCE(total_funding_pnl, 0),
			COALESCE(long_liquidation_price, 0), COALESCE(short_liquidation_price, 0),
			COALESCE(long_leverage, 0), COALESCE(short_leverage, 0),
			COALESCE(long_margin_mode, ''), COALESCE(short_margin_mode, ''),
			COALESCE(long_notional, 0), COALESCE(short_notional, 0),
			COALESCE(close_reason, ''),
			COALESCE(error_message, ''),
			created_at, opened_at, closed_at, updated_at
		FROM positions WHERE 1=1`

	args := []interface{}{}
	argIdx := 1

	if len(f.Status) > 0 {
		query += fmt.Sprintf(" AND status = ANY($%d)", argIdx)
		args = append(args, f.Status)
		argIdx++
	}
	if f.Symbol != "" {
		query += fmt.Sprintf(" AND symbol = $%d", argIdx)
		args = append(args, f.Symbol)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres list positions: %w", err)
	}
	defer rows.Close()

	var positions []domain.Position
	for rows.Next() {
		var p domain.Position
		var snapshot []byte
		if err := rows.Scan(
			&p.ID, &p.Symbol, &p.Status, &p.SizeType, &p.RequestedSize,
			&p.LongExchangeID, &p.LongEntryPrice, &p.LongFilledQty,
			&p.LongEntryCost, &p.LongEntryFees,
			&p.ShortExchangeID, &p.ShortEntryPrice, &p.ShortFilledQty,
			&p.ShortEntryCost, &p.ShortEntryFees,
			&p.LongExitPrice, &p.LongCloseFees,
			&p.ShortExitPrice, &p.ShortCloseFees,
			&snapshot,
			&p.LongMarkPrice, &p.ShortMarkPrice,
			&p.LongUnrealizedPnL, &p.ShortUnrealizedPnL,
			&p.TotalFundingPnL,
			&p.LongLiquidationPrice, &p.ShortLiquidationPrice,
			&p.LongLeverage, &p.ShortLeverage,
			&p.LongMarginMode, &p.ShortMarginMode,
			&p.LongNotional, &p.ShortNotional,
			&p.CloseReason,
			&p.ErrorMessage,
			&p.CreatedAt, &p.OpenedAt, &p.ClosedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres scan position: %w", err)
		}
		p.Snapshot = json.RawMessage(snapshot)
		positions = append(positions, p)
	}

	if positions == nil {
		positions = []domain.Position{}
	}
	return positions, nil
}

// UpdatePosition updates a position's mutable fields.
func (s *Store) UpdatePosition(ctx context.Context, p *domain.Position) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE positions SET
			status = $2,
			long_entry_price = $3, long_filled_qty = $4, long_entry_cost = $5, long_entry_fees = $6,
			short_entry_price = $7, short_filled_qty = $8, short_entry_cost = $9, short_entry_fees = $10,
			long_exit_price = $11, long_close_fees = $12,
			short_exit_price = $13, short_close_fees = $14,
			long_mark_price = $15, short_mark_price = $16,
			long_unrealized_pnl = $17, short_unrealized_pnl = $18,
			total_funding_pnl = $19,
			long_liquidation_price = $20, short_liquidation_price = $21,
			long_leverage = $22, short_leverage = $23,
			long_margin_mode = $24, short_margin_mode = $25,
			long_notional = $26, short_notional = $27,
			close_reason = $28,
			error_message = $29,
			opened_at = $30, closed_at = $31,
			updated_at = NOW()
		WHERE id = $1`,
		p.ID, p.Status,
		decimalToNullable(p.LongEntryPrice), decimalToNullable(p.LongFilledQty),
		decimalToNullable(p.LongEntryCost), decimalToNullable(p.LongEntryFees),
		decimalToNullable(p.ShortEntryPrice), decimalToNullable(p.ShortFilledQty),
		decimalToNullable(p.ShortEntryCost), decimalToNullable(p.ShortEntryFees),
		decimalToNullable(p.LongExitPrice), decimalToNullable(p.LongCloseFees),
		decimalToNullable(p.ShortExitPrice), decimalToNullable(p.ShortCloseFees),
		decimalToNullable(p.LongMarkPrice), decimalToNullable(p.ShortMarkPrice),
		decimalToNullable(p.LongUnrealizedPnL), decimalToNullable(p.ShortUnrealizedPnL),
		decimalToNullable(p.TotalFundingPnL),
		decimalToNullable(p.LongLiquidationPrice), decimalToNullable(p.ShortLiquidationPrice),
		decimalToNullable(p.LongLeverage), decimalToNullable(p.ShortLeverage),
		nullableString(p.LongMarginMode), nullableString(p.ShortMarginMode),
		decimalToNullable(p.LongNotional), decimalToNullable(p.ShortNotional),
		nullableString(string(p.CloseReason)),
		nullableString(p.ErrorMessage),
		p.OpenedAt, p.ClosedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres update position: %w", err)
	}
	return nil
}

// DeletePosition removes a position and its related orders and funding payments.
func (s *Store) DeletePosition(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres delete position begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM funding_payments WHERE position_id = $1`, id); err != nil {
		return fmt.Errorf("postgres delete position funding payments: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orders WHERE position_id = $1`, id); err != nil {
		return fmt.Errorf("postgres delete position orders: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM positions WHERE id = $1`, id); err != nil {
		return fmt.Errorf("postgres delete position: %w", err)
	}

	return tx.Commit(ctx)
}

// decimalToNullable returns nil for zero decimals so they're stored as NULL.
func decimalToNullable(d decimal.Decimal) interface{} {
	if d.IsZero() {
		return nil
	}
	return d
}

// nullableString returns nil for empty strings.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
