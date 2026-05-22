DROP INDEX IF EXISTS uq_users_email_active_ci;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_role_email_active_ci
ON users (role, lower(btrim(email)))
WHERE email IS NOT NULL
  AND btrim(email) <> ''
  AND is_deleted = FALSE;

