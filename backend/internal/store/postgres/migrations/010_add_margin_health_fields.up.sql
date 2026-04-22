-- Add per-leg initial margin, maintenance margin, margin ratio, and collateral fields.
-- These are updated in real-time from exchange position events and allow the UI
-- to display IM/MM values and margin health for each leg of an arbitrage position.
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_initial_margin DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_maintenance_margin DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_margin_ratio DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS long_collateral DECIMAL;

ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_initial_margin DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_maintenance_margin DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_margin_ratio DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS short_collateral DECIMAL;
