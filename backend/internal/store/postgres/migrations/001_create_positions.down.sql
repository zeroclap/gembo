DROP INDEX IF EXISTS idx_funding_payments_funded_at;
DROP INDEX IF EXISTS idx_funding_payments_position;
DROP INDEX IF EXISTS idx_orders_position;
DROP INDEX IF EXISTS idx_positions_symbol;
DROP INDEX IF EXISTS idx_positions_status;

DROP TABLE IF EXISTS funding_payments;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS positions;
