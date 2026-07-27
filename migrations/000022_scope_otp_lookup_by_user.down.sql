DROP INDEX IF EXISTS idx_otp_verify_user;

CREATE INDEX IF NOT EXISTS idx_otp_verify
    ON otp_requests(phone, device_id, created_at DESC)
    WHERE is_verified = FALSE;
