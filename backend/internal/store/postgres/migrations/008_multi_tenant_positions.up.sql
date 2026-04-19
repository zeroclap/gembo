ALTER TABLE positions
    ADD COLUMN IF NOT EXISTS user_id UUID,
    ADD COLUMN IF NOT EXISTS long_credential_id UUID,
    ADD COLUMN IF NOT EXISTS short_credential_id UUID;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS user_id UUID;

ALTER TABLE funding_payments
    ADD COLUMN IF NOT EXISTS user_id UUID;

WITH first_user AS (
    SELECT id
    FROM users
    ORDER BY created_at ASC
    LIMIT 1
)
UPDATE positions p
SET user_id = fu.id
FROM first_user fu
WHERE p.user_id IS NULL;

WITH long_creds AS (
    SELECT DISTINCT ON (user_id, exchange_id)
        id,
        user_id,
        exchange_id
    FROM exchange_credentials
    ORDER BY user_id, exchange_id, is_default DESC, created_at ASC
)
UPDATE positions p
SET long_credential_id = lc.id
FROM long_creds lc
WHERE p.long_credential_id IS NULL
  AND p.user_id = lc.user_id
  AND p.long_exchange_id = lc.exchange_id;

WITH short_creds AS (
    SELECT DISTINCT ON (user_id, exchange_id)
        id,
        user_id,
        exchange_id
    FROM exchange_credentials
    ORDER BY user_id, exchange_id, is_default DESC, created_at ASC
)
UPDATE positions p
SET short_credential_id = sc.id
FROM short_creds sc
WHERE p.short_credential_id IS NULL
  AND p.user_id = sc.user_id
  AND p.short_exchange_id = sc.exchange_id;

UPDATE orders o
SET user_id = p.user_id
FROM positions p
WHERE o.user_id IS NULL
  AND o.position_id = p.id;

UPDATE funding_payments fp
SET user_id = p.user_id
FROM positions p
WHERE fp.user_id IS NULL
  AND fp.position_id = p.id;

ALTER TABLE positions
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN long_credential_id SET NOT NULL,
    ALTER COLUMN short_credential_id SET NOT NULL;

ALTER TABLE orders
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE funding_payments
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE positions
    ADD CONSTRAINT positions_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT positions_long_credential_fk FOREIGN KEY (long_credential_id) REFERENCES exchange_credentials(id) ON DELETE RESTRICT,
    ADD CONSTRAINT positions_short_credential_fk FOREIGN KEY (short_credential_id) REFERENCES exchange_credentials(id) ON DELETE RESTRICT;

ALTER TABLE orders
    ADD CONSTRAINT orders_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE funding_payments
    ADD CONSTRAINT funding_payments_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE positions
    ADD CONSTRAINT positions_credential_ids_diff CHECK (long_credential_id <> short_credential_id);

CREATE INDEX IF NOT EXISTS idx_positions_user_status_created
    ON positions (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_positions_user_symbol
    ON positions (user_id, symbol);

CREATE INDEX IF NOT EXISTS idx_orders_user_position
    ON orders (user_id, position_id);

CREATE INDEX IF NOT EXISTS idx_funding_payments_user_position_funded
    ON funding_payments (user_id, position_id, funded_at DESC);
