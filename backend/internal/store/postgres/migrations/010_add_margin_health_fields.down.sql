-- Rollback: drop per-leg IM/MM/collateral columns.
ALTER TABLE positions DROP COLUMN IF EXISTS long_initial_margin;
ALTER TABLE positions DROP COLUMN IF EXISTS long_maintenance_margin;
ALTER TABLE positions DROP COLUMN IF EXISTS long_margin_ratio;
ALTER TABLE positions DROP COLUMN IF EXISTS long_collateral;
ALTER TABLE positions DROP COLUMN IF EXISTS short_initial_margin;
ALTER TABLE positions DROP COLUMN IF EXISTS short_maintenance_margin;
ALTER TABLE positions DROP COLUMN IF EXISTS short_margin_ratio;
ALTER TABLE positions DROP COLUMN IF EXISTS short_collateral;
