package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeroclap/gembo/internal/domain"
)

// CreateOrder inserts a new exchange order and populates o.ID and o.CreatedAt.
func (s *Store) CreateOrder(ctx context.Context, o *domain.ExchangeOrder) error {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO orders (
			position_id, exchange_id, exchange_order_id, symbol,
			side, leg, action, requested_qty, filled_qty,
			average_price, cost, fee_cost, status, raw_response
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at`,
		o.PositionID, o.ExchangeID, nullableString(o.ExchangeOrderID), o.Symbol,
		o.Side, o.Leg, o.Action,
		decimalToNullable(o.RequestedQty), decimalToNullable(o.FilledQty),
		decimalToNullable(o.AveragePrice), decimalToNullable(o.Cost),
		decimalToNullable(o.FeeCost), o.Status, nullableJSON(o.RawResponse),
	)
	if err := row.Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return fmt.Errorf("postgres create order: %w", err)
	}
	return nil
}

// UpdateOrder updates an existing exchange order's mutable fields.
func (s *Store) UpdateOrder(ctx context.Context, o *domain.ExchangeOrder) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE orders SET
			exchange_order_id = $2,
			filled_qty = $3,
			average_price = $4,
			cost = $5,
			fee_cost = $6,
			status = $7,
			raw_response = $8,
			updated_at = NOW()
		WHERE id = $1`,
		o.ID,
		nullableString(o.ExchangeOrderID),
		decimalToNullable(o.FilledQty),
		decimalToNullable(o.AveragePrice),
		decimalToNullable(o.Cost),
		decimalToNullable(o.FeeCost),
		o.Status,
		nullableJSON(o.RawResponse),
	)
	if err != nil {
		return fmt.Errorf("postgres update order: %w", err)
	}
	return nil
}

// ListOrdersByPosition returns all orders for a given position, ordered by created_at asc.
func (s *Store) ListOrdersByPosition(ctx context.Context, positionID string) ([]domain.ExchangeOrder, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, position_id, exchange_id, COALESCE(exchange_order_id, ''),
			symbol, side, leg, action,
			COALESCE(requested_qty, 0), COALESCE(filled_qty, 0),
			COALESCE(average_price, 0), COALESCE(cost, 0),
			COALESCE(fee_cost, 0), status, raw_response,
			created_at, updated_at
		FROM orders
		WHERE position_id = $1
		ORDER BY created_at ASC`, positionID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres list orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.ExchangeOrder
	for rows.Next() {
		var o domain.ExchangeOrder
		var rawResp []byte
		if err := rows.Scan(
			&o.ID, &o.PositionID, &o.ExchangeID, &o.ExchangeOrderID,
			&o.Symbol, &o.Side, &o.Leg, &o.Action,
			&o.RequestedQty, &o.FilledQty,
			&o.AveragePrice, &o.Cost,
			&o.FeeCost, &o.Status, &rawResp,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres scan order: %w", err)
		}
		if rawResp != nil {
			o.RawResponse = json.RawMessage(rawResp)
		}
		orders = append(orders, o)
	}

	if orders == nil {
		orders = []domain.ExchangeOrder{}
	}
	return orders, nil
}

// nullableJSON returns nil for empty or null JSON, otherwise the raw bytes.
func nullableJSON(j json.RawMessage) interface{} {
	if len(j) == 0 || string(j) == "null" {
		return nil
	}
	return []byte(j)
}
