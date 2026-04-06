-- Stepped execution parameters (set at position creation).
ALTER TABLE positions ADD COLUMN IF NOT EXISTS portion_size DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS price_min DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS price_max DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS slippage_pct DECIMAL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS leverage INTEGER DEFAULT 1;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS margin_capital DECIMAL;

-- Stepped execution progress.
ALTER TABLE positions ADD COLUMN IF NOT EXISTS steps_completed INTEGER DEFAULT 0;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS steps_total INTEGER DEFAULT 0;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS stop_reason TEXT;
