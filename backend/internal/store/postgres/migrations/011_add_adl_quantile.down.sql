ALTER TABLE positions
    DROP COLUMN IF EXISTS long_adl_quantile,
    DROP COLUMN IF EXISTS short_adl_quantile;
