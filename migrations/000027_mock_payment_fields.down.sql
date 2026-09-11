DROP INDEX IF EXISTS idx_payments_provider_payment_id;

ALTER TABLE payments
    DROP COLUMN IF EXISTS failure_message,
    DROP COLUMN IF EXISTS failure_code,
    DROP COLUMN IF EXISTS provider_payment_id,
    DROP COLUMN IF EXISTS provider;
