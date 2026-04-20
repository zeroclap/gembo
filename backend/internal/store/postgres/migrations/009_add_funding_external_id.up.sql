ALTER TABLE funding_payments
    ADD COLUMN IF NOT EXISTS external_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_funding_payments_external
    ON funding_payments (user_id, exchange_id, external_id)
    WHERE external_id IS NOT NULL AND external_id <> '';
