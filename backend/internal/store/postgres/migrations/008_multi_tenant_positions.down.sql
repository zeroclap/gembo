DROP INDEX IF EXISTS idx_funding_payments_user_position_funded;
DROP INDEX IF EXISTS idx_orders_user_position;
DROP INDEX IF EXISTS idx_positions_user_symbol;
DROP INDEX IF EXISTS idx_positions_user_status_created;

ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_credential_ids_diff;
ALTER TABLE funding_payments DROP CONSTRAINT IF EXISTS funding_payments_user_fk;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_user_fk;
ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_short_credential_fk;
ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_long_credential_fk;
ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_user_fk;

ALTER TABLE funding_payments DROP COLUMN IF EXISTS user_id;
ALTER TABLE orders DROP COLUMN IF EXISTS user_id;

ALTER TABLE positions
    DROP COLUMN IF EXISTS short_credential_id,
    DROP COLUMN IF EXISTS long_credential_id,
    DROP COLUMN IF EXISTS user_id;
