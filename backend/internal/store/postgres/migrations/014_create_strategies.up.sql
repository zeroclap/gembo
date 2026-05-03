CREATE TABLE strategies (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name                    TEXT        NOT NULL,
    enabled                 BOOLEAN     NOT NULL DEFAULT TRUE,
    arbitrage_type          TEXT        NOT NULL DEFAULT 'funding',

    -- Criteria filters
    min_apr                 NUMERIC,
    min_net_spread_pct      NUMERIC,
    symbol_whitelist        TEXT[]      NOT NULL DEFAULT '{}',
    exchange_whitelist      TEXT[]      NOT NULL DEFAULT '{}',

    -- Position parameters
    credentials             JSONB       NOT NULL DEFAULT '{}',
    size_usd                NUMERIC     NOT NULL,
    leverage                INT         NOT NULL DEFAULT 1,
    max_concurrent_positions INT        NOT NULL DEFAULT 1,

    -- Spread arbitrage parameters
    target_spread_pct       NUMERIC,
    stop_loss_spread_pct    NUMERIC,
    max_hold_duration       BIGINT,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX strategies_user_id_idx ON strategies (user_id);
CREATE INDEX strategies_enabled_idx ON strategies (enabled) WHERE enabled = TRUE;
