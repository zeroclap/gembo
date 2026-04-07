ALTER TABLE positions DROP COLUMN IF EXISTS max_hold_duration;
ALTER TABLE positions DROP COLUMN IF EXISTS stop_loss_spread_pct;
ALTER TABLE positions DROP COLUMN IF EXISTS target_spread_pct;
ALTER TABLE positions DROP COLUMN IF EXISTS entry_spread_pct;
ALTER TABLE positions DROP COLUMN IF EXISTS arbitrage_type;
