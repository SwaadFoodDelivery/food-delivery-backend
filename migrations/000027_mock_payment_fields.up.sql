ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS provider VARCHAR(32) NOT NULL DEFAULT 'mock',
    ADD COLUMN IF NOT EXISTS provider_payment_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS failure_code VARCHAR(64),
    ADD COLUMN IF NOT EXISTS failure_message TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_payment_id
    ON payments(provider, provider_payment_id)
    WHERE provider_payment_id IS NOT NULL;
