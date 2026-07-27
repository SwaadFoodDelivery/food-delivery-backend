-- One phone number may own a separate account per role, so OTP verification is
-- scoped by user_id rather than by phone. The phone-keyed lookup index is no
-- longer used by any query.
DROP INDEX IF EXISTS idx_otp_verify;

CREATE INDEX IF NOT EXISTS idx_otp_verify_user
    ON otp_requests(user_id, device_id, created_at DESC)
    WHERE is_verified = FALSE;
