ALTER TABLE positions
    ADD COLUMN IF NOT EXISTS long_adl_quantile  SMALLINT,
    ADD COLUMN IF NOT EXISTS short_adl_quantile SMALLINT;
