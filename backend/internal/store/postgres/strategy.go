package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/zeroclap/gembo/internal/domain"
)

// scanStrategy scans a single row into a domain.Strategy.
func scanStrategy(row interface{ Scan(...any) error }) (*domain.Strategy, error) {
	var s domain.Strategy
	var credRaw []byte
	var symbolWhitelist, exchangeWhitelist []string

	err := row.Scan(
		&s.ID, &s.UserID, &s.Name, &s.Enabled, &s.ArbitrageType,
		&s.MinAPR, &s.MinNetSpreadPct,
		&symbolWhitelist, &exchangeWhitelist,
		&credRaw,
		&s.SizeUSD, &s.Leverage, &s.MaxConcurrentPositions,
		&s.TargetSpreadPct, &s.StopLossSpreadPct, &s.MaxHoldDuration,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	s.SymbolWhitelist = symbolWhitelist
	s.ExchangeWhitelist = exchangeWhitelist

	if len(credRaw) > 0 {
		if err := json.Unmarshal(credRaw, &s.Credentials); err != nil {
			return nil, fmt.Errorf("unmarshal strategy credentials: %w", err)
		}
	}
	if s.Credentials == nil {
		s.Credentials = make(map[string]string)
	}
	if s.SymbolWhitelist == nil {
		s.SymbolWhitelist = []string{}
	}
	if s.ExchangeWhitelist == nil {
		s.ExchangeWhitelist = []string{}
	}

	return &s, nil
}

const strategyColumns = `
	id, user_id, name, enabled, arbitrage_type,
	COALESCE(min_apr, 0), COALESCE(min_net_spread_pct, 0),
	COALESCE(symbol_whitelist, '{}'), COALESCE(exchange_whitelist, '{}'),
	credentials,
	size_usd, leverage, max_concurrent_positions,
	COALESCE(target_spread_pct, 0), COALESCE(stop_loss_spread_pct, 0), COALESCE(max_hold_duration, 0),
	created_at, updated_at`

// CreateStrategy inserts a new strategy and populates generated fields.
func (s *Store) CreateStrategy(ctx context.Context, st *domain.Strategy) error {
	credJSON, err := json.Marshal(st.Credentials)
	if err != nil {
		return fmt.Errorf("marshal strategy credentials: %w", err)
	}

	symWL := st.SymbolWhitelist
	if symWL == nil {
		symWL = []string{}
	}
	exchWL := st.ExchangeWhitelist
	if exchWL == nil {
		exchWL = []string{}
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO strategies (
			user_id, name, enabled, arbitrage_type,
			min_apr, min_net_spread_pct,
			symbol_whitelist, exchange_whitelist,
			credentials, size_usd, leverage, max_concurrent_positions,
			target_spread_pct, stop_loss_spread_pct, max_hold_duration
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at`,
		st.UserID, st.Name, st.Enabled, string(st.ArbitrageType),
		decimalToNullable(st.MinAPR), decimalToNullable(st.MinNetSpreadPct),
		symWL, exchWL,
		credJSON, st.SizeUSD, st.Leverage, st.MaxConcurrentPositions,
		decimalToNullable(st.TargetSpreadPct), decimalToNullable(st.StopLossSpreadPct),
		nullableInt64(st.MaxHoldDuration),
	)
	if err := row.Scan(&st.ID, &st.CreatedAt, &st.UpdatedAt); err != nil {
		return fmt.Errorf("postgres create strategy: %w", err)
	}
	return nil
}

// GetStrategy returns a strategy by ID scoped to the given user, or nil if not found.
func (s *Store) GetStrategy(ctx context.Context, userID, id string) (*domain.Strategy, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+strategyColumns+` FROM strategies WHERE user_id = $1 AND id = $2`,
		userID, id,
	)
	st, err := scanStrategy(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres get strategy: %w", err)
	}
	return st, nil
}

// ListStrategies returns all strategies for the given user, ordered by created_at asc.
func (s *Store) ListStrategies(ctx context.Context, userID string) ([]domain.Strategy, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+strategyColumns+` FROM strategies WHERE user_id = $1 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres list strategies: %w", err)
	}
	defer rows.Close()
	return collectStrategies(rows)
}

// ListAllStrategies returns all enabled strategies across all users (used by the runner).
func (s *Store) ListAllStrategies(ctx context.Context) ([]domain.Strategy, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+strategyColumns+` FROM strategies WHERE enabled = TRUE ORDER BY user_id, created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres list all strategies: %w", err)
	}
	defer rows.Close()
	return collectStrategies(rows)
}

// UpdateStrategy updates a strategy's mutable fields and returns true when found.
func (s *Store) UpdateStrategy(ctx context.Context, st *domain.Strategy) (bool, error) {
	credJSON, err := json.Marshal(st.Credentials)
	if err != nil {
		return false, fmt.Errorf("marshal strategy credentials: %w", err)
	}

	symWL := st.SymbolWhitelist
	if symWL == nil {
		symWL = []string{}
	}
	exchWL := st.ExchangeWhitelist
	if exchWL == nil {
		exchWL = []string{}
	}

	result, err := s.pool.Exec(ctx, `
		UPDATE strategies SET
			name = $3, enabled = $4, arbitrage_type = $5,
			min_apr = $6, min_net_spread_pct = $7,
			symbol_whitelist = $8, exchange_whitelist = $9,
			credentials = $10, size_usd = $11, leverage = $12,
			max_concurrent_positions = $13,
			target_spread_pct = $14, stop_loss_spread_pct = $15,
			max_hold_duration = $16,
			updated_at = NOW()
		WHERE user_id = $1 AND id = $2`,
		st.UserID, st.ID,
		st.Name, st.Enabled, string(st.ArbitrageType),
		decimalToNullable(st.MinAPR), decimalToNullable(st.MinNetSpreadPct),
		symWL, exchWL,
		credJSON, st.SizeUSD, st.Leverage, st.MaxConcurrentPositions,
		decimalToNullable(st.TargetSpreadPct), decimalToNullable(st.StopLossSpreadPct),
		nullableInt64(st.MaxHoldDuration),
	)
	if err != nil {
		return false, fmt.Errorf("postgres update strategy: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// DeleteStrategy removes a strategy by ID for a user and returns true when deleted.
func (s *Store) DeleteStrategy(ctx context.Context, userID, id string) (bool, error) {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM strategies WHERE user_id = $1 AND id = $2`,
		userID, id,
	)
	if err != nil {
		return false, fmt.Errorf("postgres delete strategy: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

func collectStrategies(rows pgx.Rows) ([]domain.Strategy, error) {
	var strategies []domain.Strategy
	for rows.Next() {
		st, err := scanStrategy(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres scan strategy: %w", err)
		}
		strategies = append(strategies, *st)
	}
	if strategies == nil {
		strategies = []domain.Strategy{}
	}
	return strategies, nil
}
