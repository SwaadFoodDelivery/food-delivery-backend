ALTER TABLE deliveries
    ADD COLUMN IF NOT EXISTS provider VARCHAR(32) NOT NULL DEFAULT 'mock',
    ADD COLUMN IF NOT EXISTS next_transition_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES users(user_id);

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES users(user_id);

CREATE INDEX IF NOT EXISTS idx_deliveries_mock_due
    ON deliveries(provider, next_transition_at)
    WHERE next_transition_at IS NOT NULL;
