DROP INDEX IF EXISTS idx_orders_user_idempotency;
ALTER TABLE orders DROP COLUMN IF EXISTS instructions;
