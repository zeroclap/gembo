CREATE TABLE IF NOT EXISTS exchange_credentials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exchange_id TEXT NOT NULL,
    title       TEXT NOT NULL,
    api_key     TEXT NOT NULL,
    api_secret  TEXT NOT NULL,
    passphrase  TEXT,
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT exchange_credentials_exchange_id_check CHECK (
        exchange_id IN ('binance', 'bybit', 'gate', 'okx')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_exchange_credentials_user_exchange_default
    ON exchange_credentials (user_id, exchange_id)
    WHERE is_default;

CREATE INDEX IF NOT EXISTS idx_exchange_credentials_user_exchange
    ON exchange_credentials (user_id, exchange_id);
