package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// positionColumns is the shared SELECT column list for positions.
// Every Get/List query must use this exact list and scan order.
const positionColumns = `
	id, symbol, status, size_type, requested_size,
	COALESCE(requested_base_qty, 0),
	COALESCE(arbitrage_type, 'funding'),
	COALESCE(entry_spread_pct, 0), COALESCE(target_spread_pct, 0),
	COALESCE(stop_loss_spread_pct, 0), COALESCE(max_hold_duration, 0),
	COALESCE(portion_size, 0), COALESCE(price_min, 0), COALESCE(price_max, 0),
	COALESCE(slippage_pct, 0), COALESCE(leverage, 1), COALESCE(margin_capital, 0),
	COALESCE(steps_completed, 0), COALESCE(steps_total, 0), COALESCE(stop_reason, ''),
	long_exchange_id, COALESCE(long_contract_size, 0), COALESCE(long_entry_price, 0), COALESCE(long_filled_qty, 0),
	COALESCE(long_filled_base_qty, 0),
	COALESCE(long_entry_cost, 0), COALESCE(long_entry_fees, 0),
	short_exchange_id, COALESCE(short_contract_size, 0), COALESCE(short_entry_price, 0), COALESCE(short_filled_qty, 0),
	COALESCE(short_filled_base_qty, 0),
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
	created_at, opened_at, closed_at, updated_at`

// scanPosition scans a row into a domain.Position using the positionColumns order.
func scanPosition(scanner interface{ Scan(...any) error }) (*domain.Position, error) {
	var p domain.Position
	var snapshot []byte
	err := scanner.Scan(
		&p.ID, &p.Symbol, &p.Status, &p.SizeType, &p.RequestedSize,
		&p.RequestedBaseQty,
		&p.ArbitrageType,
		&p.EntrySpreadPct, &p.TargetSpreadPct,
		&p.StopLossSpreadPct, &p.MaxHoldDuration,
		&p.PortionSize, &p.PriceMin, &p.PriceMax,
		&p.SlippagePct, &p.Leverage, &p.MarginCapital,
		&p.StepsCompleted, &p.StepsTotal, &p.StopReason,
		&p.LongExchangeID, &p.LongContractSize, &p.LongEntryPrice, &p.LongFilledQty,
		&p.LongFilledBaseQty,
		&p.LongEntryCost, &p.LongEntryFees,
		&p.ShortExchangeID, &p.ShortContractSize, &p.ShortEntryPrice, &p.ShortFilledQty,
		&p.ShortFilledBaseQty,
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
	if err != nil {
		return nil, err
	}
	p.Snapshot = json.RawMessage(snapshot)
	return &p, nil
}

// CreatePosition inserts a new position and populates p.ID and p.CreatedAt.
func (s *Store) CreatePosition(ctx context.Context, p *domain.Position) error {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO positions (
			symbol, status, size_type, requested_size, requested_base_qty,
			arbitrage_type, entry_spread_pct, target_spread_pct,
			stop_loss_spread_pct, max_hold_duration,
			portion_size, price_min, price_max, slippage_pct,
			leverage, margin_capital, steps_total,
			long_exchange_id, short_exchange_id,
			long_contract_size, short_contract_size,
			snapshot
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		RETURNING id, created_at, updated_at`,
		p.Symbol, p.Status, p.SizeType, p.RequestedSize, decimalToNullable(p.RequestedBaseQty),
		p.ArbitrageType, decimalToNullable(p.EntrySpreadPct),
		decimalToNullable(p.TargetSpreadPct), decimalToNullable(p.StopLossSpreadPct),
		nullableInt64(p.MaxHoldDuration),
		decimalToNullable(p.PortionSize), decimalToNullable(p.PriceMin),
		decimalToNullable(p.PriceMax), decimalToNullable(p.SlippagePct),
		p.Leverage, decimalToNullable(p.MarginCapital), p.StepsTotal,
		p.LongExchangeID, p.ShortExchangeID,
		decimalToNullable(p.LongContractSize), decimalToNullable(p.ShortContractSize),
		p.Snapshot,
	)
	if err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("postgres create position: %w", err)
	}
	return nil
}

// GetPosition retrieves a position by ID.
func (s *Store) GetPosition(ctx context.Context, id string) (*domain.Position, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+positionColumns+` FROM positions WHERE id = $1`, id,
	)
	p, err := scanPosition(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres get position: %w", err)
	}
	return p, nil
}

// ListPositions returns positions matching the given filter, ordered by created_at desc.
func (s *Store) ListPositions(ctx context.Context, f domain.PositionFilter) ([]domain.Position, error) {
	query := `SELECT ` + positionColumns + ` FROM positions WHERE 1=1`

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
		p, err := scanPosition(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres scan position: %w", err)
		}
		positions = append(positions, *p)
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
			requested_base_qty = $3,
			long_contract_size = $4, short_contract_size = $5,
			long_entry_price = $6, long_filled_qty = $7, long_filled_base_qty = $8, long_entry_cost = $9, long_entry_fees = $10,
			short_entry_price = $11, short_filled_qty = $12, short_filled_base_qty = $13, short_entry_cost = $14, short_entry_fees = $15,
			long_exit_price = $16, long_close_fees = $17,
			short_exit_price = $18, short_close_fees = $19,
			long_mark_price = $20, short_mark_price = $21,
			long_unrealized_pnl = $22, short_unrealized_pnl = $23,
			total_funding_pnl = $24,
			long_liquidation_price = $25, short_liquidation_price = $26,
			long_leverage = $27, short_leverage = $28,
			long_margin_mode = $29, short_margin_mode = $30,
			long_notional = $31, short_notional = $32,
			close_reason = $33,
			error_message = $34,
			opened_at = $35, closed_at = $36,
			steps_completed = $37, stop_reason = $38,
			updated_at = NOW()
		WHERE id = $1`,
		p.ID, p.Status,
		decimalToNullable(p.RequestedBaseQty),
		decimalToNullable(p.LongContractSize), decimalToNullable(p.ShortContractSize),
		decimalToNullable(p.LongEntryPrice), decimalToNullable(p.LongFilledQty), decimalToNullable(p.LongFilledBaseQty),
		decimalToNullable(p.LongEntryCost), decimalToNullable(p.LongEntryFees),
		decimalToNullable(p.ShortEntryPrice), decimalToNullable(p.ShortFilledQty), decimalToNullable(p.ShortFilledBaseQty),
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
		p.StepsCompleted, nullableString(string(p.StopReason)),
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

// nullableInt64 returns nil for zero int64 values.
func nullableInt64(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
