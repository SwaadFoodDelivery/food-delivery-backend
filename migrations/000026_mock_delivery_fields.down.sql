DROP INDEX IF EXISTS idx_deliveries_mock_due;
ALTER TABLE deliveries DROP COLUMN IF EXISTS updated_by;
ALTER TABLE deliveries DROP COLUMN IF EXISTS next_transition_at;
ALTER TABLE deliveries DROP COLUMN IF EXISTS provider;
ALTER TABLE orders DROP COLUMN IF EXISTS updated_by;
