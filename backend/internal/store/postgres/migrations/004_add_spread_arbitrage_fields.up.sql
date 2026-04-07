-- Spread arbitrage support: extend positions with arbitrage type and spread parameters.
ALTER TABLE positions ADD COLUMN IF NOT EXISTS arbitrage_type TEXT NOT NULL DEFAULT 'funding';
ALTER TABLE positions ADD COLUMN IF NOT EXISTS entry_spread_pct DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS target_spread_pct DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS stop_loss_spread_pct DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS max_hold_duration BIGINT;
