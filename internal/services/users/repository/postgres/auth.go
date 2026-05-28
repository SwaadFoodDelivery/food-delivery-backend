package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/models"

	"github.com/jmoiron/sqlx"
)

type Accessor interface {
	Execer() sqlx.ExtContext
	Queryer() sqlx.QueryerContext
}

type Store struct {
	accessor Accessor
}

func NewStore(accessor Accessor) *Store {
	return &Store{accessor: accessor}
}

func (s *Store) FindUserByPhoneAndRole(ctx context.Context, phone, role string) (*models.UserRow, error) {
	var row models.UserRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT user_id::text, phone, role, name, email, email_verified, phone_verified, onboarding_complete, referral_code, account_status, created_at
		FROM users
		WHERE phone = $1 AND role = $2 AND is_deleted = FALSE
		LIMIT 1
	`, phone, role)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) FindUserByPhone(ctx context.Context, phone string) (*models.UserRow, error) {
	var row models.UserRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT user_id::text, phone, role, name, email, email_verified, phone_verified, onboarding_complete, referral_code, account_status, created_at
		FROM users
		WHERE phone = $1 AND is_deleted = FALSE
		ORDER BY created_at ASC
		LIMIT 1
	`, phone)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) FindUserByID(ctx context.Context, userID string) (*models.UserRow, error) {
	var row models.UserRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT user_id::text, phone, role, name, email, email_verified, phone_verified, onboarding_complete, referral_code, account_status, created_at
		FROM users
		WHERE user_id = $1::uuid AND is_deleted = FALSE
	`, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) UserExistsByPhoneRole(ctx context.Context, phone, role string) (bool, error) {
	var exists bool
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &exists, `
		SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1 AND role = $2 AND is_deleted = FALSE)
	`, phone, role)
	return exists, err
}

func (s *Store) UserExistsByEmailRole(ctx context.Context, email, role string) (bool, error) {
	var exists bool
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &exists, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE is_deleted = FALSE
			  AND role = $2
			  AND email IS NOT NULL
			  AND btrim(email) <> ''
			  AND lower(btrim(email)) = lower(btrim($1))
		)
	`, email, role)
	return exists, err
}

func (s *Store) CreateUser(ctx context.Context, phone, name, email, role, referralCode string) (string, error) {
	var userID string
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &userID, `
		INSERT INTO users
		(phone, name, email, role, account_status, email_verified, phone_verified, onboarding_complete, is_deleted, created_at, updated_at, referral_code)
		VALUES ($1, $2, NULLIF($3, ''), $4, 'active', FALSE, FALSE, FALSE, FALSE, NOW(), NOW(), NULLIF($5, ''))
		RETURNING user_id::text
	`, phone, name, email, role, referralCode)
	return userID, err
}

func (s *Store) InsertAuditLog(ctx context.Context, actorID, actorRole, action, entityType, entityID string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, occurred_at)
		VALUES ($1::uuid, $2, $3, $4, $5, NOW())
	`, actorID, actorRole, action, entityType, entityID)
	return err
}

func (s *Store) FindLatestActiveOTPByPhone(ctx context.Context, phone string) (*models.OTPRow, error) {
	var row models.OTPRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT otp_id::text, user_id::text, phone, device_id, otp_hash, expires_at, is_verified, attempts, resend_count, blocked_until
		FROM otp_requests
		WHERE phone = $1 AND is_verified = FALSE AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`, phone)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) FindLatestUnverifiedOTPByPhoneDevice(ctx context.Context, phone, deviceID string) (*models.OTPRow, error) {
	var row models.OTPRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT otp_id::text, user_id::text, phone, device_id, otp_hash, expires_at, is_verified, attempts, resend_count, blocked_until
		FROM otp_requests
		WHERE phone = $1 AND device_id = $2 AND is_verified = FALSE
		ORDER BY created_at DESC
		LIMIT 1
	`, phone, deviceID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) CreateOTPRequest(ctx context.Context, userID, phone, deviceID, ipAddress, otpHash string, expiresAt time.Time) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		INSERT INTO otp_requests
		(user_id, phone, device_id, ip_address, otp_hash, expires_at, is_verified, attempts, resend_count, last_sent_at, created_at)
		VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, '')::inet, $5, $6, FALSE, 0, 0, NOW(), NOW())
	`, userID, phone, deviceID, ipAddress, otpHash, expiresAt)
	return err
}

func (s *Store) IncrementOTPResendCount(ctx context.Context, otpID string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE otp_requests SET resend_count = resend_count + 1, last_sent_at = NOW() WHERE otp_id = $1::uuid
	`, otpID)
	return err
}

func (s *Store) IncrementOTPAttempts(ctx context.Context, otpID string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE otp_requests SET attempts = attempts + 1 WHERE otp_id = $1::uuid
	`, otpID)
	return err
}

func (s *Store) SetOTPBlockedUntil(ctx context.Context, otpID string, blockedUntil time.Time) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE otp_requests SET blocked_until = $2 WHERE otp_id = $1::uuid
	`, otpID, blockedUntil)
	return err
}

func (s *Store) MarkOTPVerified(ctx context.Context, otpID string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE otp_requests SET is_verified = TRUE, attempts = attempts + 1 WHERE otp_id = $1::uuid
	`, otpID)
	return err
}

func (s *Store) MarkUserPhoneVerified(ctx context.Context, userID string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE users SET phone_verified = TRUE, updated_at = NOW() WHERE user_id = $1::uuid
	`, userID)
	return err
}

func (s *Store) CountVerifiedOTPs(ctx context.Context, userID string) (int, error) {
	var count int
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &count, `
		SELECT COUNT(*) FROM otp_requests WHERE user_id = $1::uuid AND is_verified = TRUE
	`, userID)
	return count, err
}

func (s *Store) CreateSession(ctx context.Context, sessionID, userID, phone, role, deviceID, ipAddress, platform string, expiresAt time.Time) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		INSERT INTO sessions
		(session_id, user_id, phone, role, device_id, is_active, ip_address, platform, expires_at, logged_in_at, last_active_at, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, TRUE, NULLIF($6, '')::inet, NULLIF($7, '')::platform_type, $8, NOW(), NOW(), NOW(), NOW())
	`, sessionID, userID, phone, role, deviceID, ipAddress, platform, expiresAt)
	return err
}

func (s *Store) DeactivateSession(ctx context.Context, sessionID string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE sessions SET is_active = FALSE, logged_out_at = NOW(), updated_at = NOW() WHERE session_id = $1::uuid
	`, sessionID)
	return err
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apperrors.ErrNotFound
	}
	return err
}
