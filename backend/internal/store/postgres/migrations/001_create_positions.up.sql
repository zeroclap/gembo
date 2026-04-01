CREATE TABLE IF NOT EXISTS positions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol          TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    size_type       TEXT NOT NULL,
    requested_size  DECIMAL NOT NULL,

    -- Long leg
    long_exchange_id   TEXT NOT NULL,
    long_entry_price   DECIMAL,
    long_filled_qty    DECIMAL,
    long_entry_cost    DECIMAL,
    long_entry_fees    DECIMAL,

    -- Short leg
    short_exchange_id  TEXT NOT NULL,
    short_entry_price  DECIMAL,
    short_filled_qty   DECIMAL,
    short_entry_cost   DECIMAL,
    short_entry_fees   DECIMAL,

    -- Close details
    long_exit_price    DECIMAL,
    long_close_fees    DECIMAL,
    short_exit_price   DECIMAL,
    short_close_fees   DECIMAL,

    -- Opportunity snapshot at open time (the "intent")
    snapshot           JSONB NOT NULL DEFAULT '{}',

    -- Live state (updated by trader monitor loop)
    long_mark_price       DECIMAL,
    short_mark_price      DECIMAL,
    long_unrealized_pnl   DECIMAL,
    short_unrealized_pnl  DECIMAL,
    total_funding_pnl     DECIMAL DEFAULT 0,

    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    opened_at     TIMESTAMPTZ,
    closed_at     TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_id        UUID NOT NULL REFERENCES positions(id),
    exchange_id        TEXT NOT NULL,
    exchange_order_id  TEXT,
    symbol             TEXT NOT NULL,
    side               TEXT NOT NULL,
    leg                TEXT NOT NULL,
    action             TEXT NOT NULL,
    requested_qty      DECIMAL,
    filled_qty         DECIMAL,
    average_price      DECIMAL,
    cost               DECIMAL,
    fee_cost           DECIMAL,
    status             TEXT NOT NULL DEFAULT 'pending',
    raw_response       JSONB,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS funding_payments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_id  UUID NOT NULL REFERENCES positions(id),
    exchange_id  TEXT NOT NULL,
    leg          TEXT NOT NULL,
    symbol       TEXT NOT NULL,
    rate         DECIMAL NOT NULL,
    payment      DECIMAL NOT NULL,
    funded_at    TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_positions_status ON positions(status);
CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions(symbol);
CREATE INDEX IF NOT EXISTS idx_orders_position ON orders(position_id);
CREATE INDEX IF NOT EXISTS idx_funding_payments_position ON funding_payments(position_id);
CREATE INDEX IF NOT EXISTS idx_funding_payments_funded_at ON funding_payments(position_id, funded_at);
