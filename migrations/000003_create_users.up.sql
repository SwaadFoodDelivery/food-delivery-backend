DO $$ BEGIN
    CREATE TYPE user_role AS ENUM
        ('client','restaurant_owner','restaurant_manager','driver');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE account_status AS ENUM ('active','suspended','deleted');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE platform_type AS ENUM ('android','ios','web');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS users (
    user_id              UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    phone                VARCHAR(15)   NOT NULL,
    name                 VARCHAR(100)  NOT NULL,
    email                VARCHAR(255),
    email_verified       BOOLEAN       NOT NULL DEFAULT FALSE,
    phone_verified       BOOLEAN       NOT NULL DEFAULT FALSE,
    role                 user_role     NOT NULL,
    account_status       account_status NOT NULL DEFAULT 'active',
    profile_image_s3_key VARCHAR(512),
    referral_code        VARCHAR(20),
    onboarding_complete  BOOLEAN       NOT NULL DEFAULT FALSE,
    is_deleted           BOOLEAN       NOT NULL DEFAULT FALSE,
    deleted_at           TIMESTAMPTZ,
    deleted_by           UUID,
    created_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    created_by           UUID,
    updated_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_by           UUID
);

DO $$ BEGIN
    ALTER TABLE users
    ADD CONSTRAINT fk_users_deleted_by
    FOREIGN KEY (deleted_by) REFERENCES users(user_id);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_role
    ON users(phone, role) WHERE is_deleted = FALSE;

CREATE INDEX IF NOT EXISTS idx_users_email
    ON users(email) WHERE email IS NOT NULL AND is_deleted = FALSE;

CREATE INDEX IF NOT EXISTS idx_users_role
    ON users(role) WHERE is_deleted = FALSE;

CREATE INDEX IF NOT EXISTS idx_users_account_status
    ON users(account_status);

CREATE TABLE IF NOT EXISTS otp_requests (
    otp_id        UUID         NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id       UUID         REFERENCES users(user_id) ON DELETE CASCADE,
    phone         VARCHAR(15)  NOT NULL,
    device_id     VARCHAR(255),
    ip_address    INET,
    otp_hash      VARCHAR(255) NOT NULL,
    expires_at    TIMESTAMPTZ  NOT NULL,
    is_verified   BOOLEAN      NOT NULL DEFAULT FALSE,
    attempts      SMALLINT     NOT NULL DEFAULT 0 CHECK (attempts <= 5),
    resend_count  SMALLINT     NOT NULL DEFAULT 0 CHECK (resend_count <= 5),
    last_sent_at  TIMESTAMPTZ,
    blocked_until TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_otp_verify
    ON otp_requests(phone, device_id, created_at DESC)
    WHERE is_verified = FALSE;

CREATE INDEX IF NOT EXISTS idx_otp_user_cooldown
    ON otp_requests(user_id, last_sent_at DESC);

CREATE INDEX IF NOT EXISTS idx_otp_expires
    ON otp_requests(expires_at);

CREATE TABLE IF NOT EXISTS sessions (
    session_id      UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id         UUID          NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    phone           VARCHAR(15)   NOT NULL,
    role            user_role     NOT NULL,
    device_id       VARCHAR(100)  NOT NULL,
    is_active       BOOLEAN       NOT NULL DEFAULT TRUE,
    logged_in_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    last_active_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    logged_out_at   TIMESTAMPTZ,
    ip_address      INET,
    platform        platform_type,
    active_cart_id  VARCHAR(255),
    active_order_id UUID,
    expires_at      TIMESTAMPTZ   NOT NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_active
    ON sessions(user_id) WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_sessions_device
    ON sessions(user_id, device_id);

CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS set_updated_at_users ON users;
CREATE TRIGGER set_updated_at_users
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

DROP TRIGGER IF EXISTS set_updated_at_sessions ON sessions;
CREATE TRIGGER set_updated_at_sessions
    BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
