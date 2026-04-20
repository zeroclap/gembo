DROP INDEX IF EXISTS uq_funding_payments_external;

ALTER TABLE funding_payments DROP COLUMN IF EXISTS external_id;
