ALTER TABLE orders DROP COLUMN IF EXISTS filled_base_qty;
ALTER TABLE orders DROP COLUMN IF EXISTS contract_size;
ALTER TABLE orders DROP COLUMN IF EXISTS requested_base_qty;

ALTER TABLE positions DROP COLUMN IF EXISTS short_filled_base_qty;
ALTER TABLE positions DROP COLUMN IF EXISTS long_filled_base_qty;
ALTER TABLE positions DROP COLUMN IF EXISTS short_contract_size;
ALTER TABLE positions DROP COLUMN IF EXISTS long_contract_size;
ALTER TABLE positions DROP COLUMN IF EXISTS requested_base_qty;
