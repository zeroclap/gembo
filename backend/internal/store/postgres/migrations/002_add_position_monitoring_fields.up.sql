-- Add liquidation, leverage, margin, notional fields per leg.
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_liquidation_price DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_liquidation_price DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_leverage DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_leverage DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_margin_mode TEXT;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_margin_mode TEXT;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_notional DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_notional DECIMAL;

-- Add close reason to track why a position was closed.
ALTER TABLE positions ADD COLUMN IF NOT EXISTS close_reason TEXT;
