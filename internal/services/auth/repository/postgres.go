package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"food-delivery-backend/internal/services/auth/models"

	"github.com/jmoiron/sqlx"
)

type postgresStore struct {
	r *repo
}

func (r *repo) FindUserByPhoneAndRole(ctx context.Context, phone, role string) (*models.UserRow, error) {
	return r.postgres.findUserByPhoneAndRole(ctx, phone, role)
}

func (r *repo) FindUserByPhone(ctx context.Context, phone string) (*models.UserRow, error) {
	return r.postgres.findUserByPhone(ctx, phone)
}

func (r *repo) FindUserByID(ctx context.Context, userID string) (*models.UserRow, error) {
	return r.postgres.findUserByID(ctx, userID)
}

func (r *repo) UserExistsByPhoneRole(ctx context.Context, phone, role string) (bool, error) {
	return r.postgres.userExistsByPhoneRole(ctx, phone, role)
}

func (r *repo) CreateUser(ctx context.Context, in CreateUserInput) (string, error) {
	return r.postgres.createUser(ctx, in)
}

func (r *repo) InsertAuditLog(ctx context.Context, in AuditLogInput) error {
	return r.postgres.insertAuditLog(ctx, in)
}

func (r *repo) FindLatestActiveOTPByPhone(ctx context.Context, phone string) (*models.OTPRow, error) {
	return r.postgres.findLatestActiveOTPByPhone(ctx, phone)
}

func (r *repo) FindLatestUnverifiedOTPByPhoneDevice(ctx context.Context, phone, deviceID string) (*models.OTPRow, error) {
	return r.postgres.findLatestUnverifiedOTPByPhoneDevice(ctx, phone, deviceID)
}

func (r *repo) CreateOTPRequest(ctx context.Context, in CreateOTPRequestInput) error {
	return r.postgres.createOTPRequest(ctx, in)
}

func (r *repo) IncrementOTPResendCount(ctx context.Context, otpID string) error {
	return r.postgres.incrementOTPResendCount(ctx, otpID)
}

func (r *repo) IncrementOTPAttempts(ctx context.Context, otpID string) error {
	return r.postgres.incrementOTPAttempts(ctx, otpID)
}

func (r *repo) SetOTPBlockedUntil(ctx context.Context, otpID string, blockedUntil time.Time) error {
	return r.postgres.setOTPBlockedUntil(ctx, otpID, blockedUntil)
}

func (r *repo) MarkOTPVerified(ctx context.Context, otpID string) error {
	return r.postgres.markOTPVerified(ctx, otpID)
}

func (r *repo) MarkUserPhoneVerified(ctx context.Context, userID string) error {
	return r.postgres.markUserPhoneVerified(ctx, userID)
}

func (r *repo) CountVerifiedOTPs(ctx context.Context, userID string) (int, error) {
	return r.postgres.countVerifiedOTPs(ctx, userID)
}

func (r *repo) CreateSession(ctx context.Context, in CreateSessionInput) error {
	return r.postgres.createSession(ctx, in)
}

func (r *repo) DeactivateSession(ctx context.Context, sessionID string) error {
	return r.postgres.deactivateSession(ctx, sessionID)
}

func (s *postgresStore) findUserByPhoneAndRole(ctx context.Context, phone, role string) (*models.UserRow, error) {
	var row models.UserRow
	err := sqlx.GetContext(ctx, s.r.queryer(), &row, `
		SELECT user_id::text, phone, role, name, email, email_verified, phone_verified, referral_code, account_status, created_at
		FROM users
		WHERE phone = $1 AND role = $2 AND is_deleted = FALSE
		LIMIT 1
	`, phone, role)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *postgresStore) findUserByPhone(ctx context.Context, phone string) (*models.UserRow, error) {
	var row models.UserRow
	err := sqlx.GetContext(ctx, s.r.queryer(), &row, `
		SELECT user_id::text, phone, role, name, email, email_verified, phone_verified, referral_code, account_status, created_at
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

func (s *postgresStore) findUserByID(ctx context.Context, userID string) (*models.UserRow, error) {
	var row models.UserRow
	err := sqlx.GetContext(ctx, s.r.queryer(), &row, `
		SELECT user_id::text, phone, role, name, email, email_verified, phone_verified, referral_code, account_status, created_at
		FROM users WHERE user_id = $1::uuid
	`, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *postgresStore) userExistsByPhoneRole(ctx context.Context, phone, role string) (bool, error) {
	var exists bool
	err := sqlx.GetContext(ctx, s.r.queryer(), &exists, `
		SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1 AND role = $2 AND is_deleted = FALSE)
	`, phone, role)
	return exists, err
}

func (s *postgresStore) createUser(ctx context.Context, in CreateUserInput) (string, error) {
	var userID string
	err := sqlx.GetContext(ctx, s.r.queryer(), &userID, `
		INSERT INTO users
		(phone, name, email, role, account_status, email_verified, phone_verified, onboarding_complete, is_deleted, created_at, updated_at, referral_code)
		VALUES ($1, $2, NULLIF($3, ''), $4, 'active', FALSE, FALSE, FALSE, FALSE, NOW(), NOW(), NULLIF($5, ''))
		RETURNING user_id::text
	`, in.Phone, in.Name, in.Email, in.Role, in.ReferralCode)
	return userID, err
}

func (s *postgresStore) insertAuditLog(ctx context.Context, in AuditLogInput) error {
	_, err := s.r.execer().ExecContext(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, occurred_at)
		VALUES ($1::uuid, $2, $3, $4, $5, NOW())
	`, in.ActorID, in.ActorRole, in.Action, in.EntityType, in.EntityID)
	return err
}

func (s *postgresStore) findLatestActiveOTPByPhone(ctx context.Context, phone string) (*models.OTPRow, error) {
	var row models.OTPRow
	err := sqlx.GetContext(ctx, s.r.queryer(), &row, `
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

func (s *postgresStore) findLatestUnverifiedOTPByPhoneDevice(ctx context.Context, phone, deviceID string) (*models.OTPRow, error) {
	var row models.OTPRow
	err := sqlx.GetContext(ctx, s.r.queryer(), &row, `
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

func (s *postgresStore) createOTPRequest(ctx context.Context, in CreateOTPRequestInput) error {
	_, err := s.r.execer().ExecContext(ctx, `
		INSERT INTO otp_requests
		(user_id, phone, device_id, ip_address, otp_hash, expires_at, is_verified, attempts, resend_count, last_sent_at, created_at)
		VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, '')::inet, $5, $6, FALSE, 0, 0, NOW(), NOW())
	`, in.UserID, in.Phone, in.DeviceID, in.IPAddress, in.OTPHash, in.ExpiresAt)
	return err
}

func (s *postgresStore) incrementOTPResendCount(ctx context.Context, otpID string) error {
	_, err := s.r.execer().ExecContext(ctx, `
		UPDATE otp_requests SET resend_count = resend_count + 1, last_sent_at = NOW() WHERE otp_id = $1::uuid
	`, otpID)
	return err
}

func (s *postgresStore) incrementOTPAttempts(ctx context.Context, otpID string) error {
	_, err := s.r.execer().ExecContext(ctx, `
		UPDATE otp_requests SET attempts = attempts + 1 WHERE otp_id = $1::uuid
	`, otpID)
	return err
}

func (s *postgresStore) setOTPBlockedUntil(ctx context.Context, otpID string, blockedUntil time.Time) error {
	_, err := s.r.execer().ExecContext(ctx, `
		UPDATE otp_requests SET blocked_until = $2 WHERE otp_id = $1::uuid
	`, otpID, blockedUntil)
	return err
}

func (s *postgresStore) markOTPVerified(ctx context.Context, otpID string) error {
	_, err := s.r.execer().ExecContext(ctx, `
		UPDATE otp_requests SET is_verified = TRUE, attempts = attempts + 1 WHERE otp_id = $1::uuid
	`, otpID)
	return err
}

func (s *postgresStore) markUserPhoneVerified(ctx context.Context, userID string) error {
	_, err := s.r.execer().ExecContext(ctx, `
		UPDATE users SET phone_verified = TRUE, updated_at = NOW() WHERE user_id = $1::uuid
	`, userID)
	return err
}

func (s *postgresStore) countVerifiedOTPs(ctx context.Context, userID string) (int, error) {
	var count int
	err := sqlx.GetContext(ctx, s.r.queryer(), &count, `
		SELECT COUNT(*) FROM otp_requests WHERE user_id = $1::uuid AND is_verified = TRUE
	`, userID)
	return count, err
}

func (s *postgresStore) createSession(ctx context.Context, in CreateSessionInput) error {
	_, err := s.r.execer().ExecContext(ctx, `
		INSERT INTO sessions
		(session_id, user_id, phone, role, device_id, is_active, ip_address, platform, expires_at, logged_in_at, last_active_at, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, TRUE, NULLIF($6, '')::inet, NULLIF($7, '')::platform_type, $8, NOW(), NOW(), NOW(), NOW())
	`, in.SessionID, in.UserID, in.Phone, in.Role, in.DeviceID, in.IPAddress, in.Platform, in.ExpiresAt)
	return err
}

func (s *postgresStore) deactivateSession(ctx context.Context, sessionID string) error {
	_, err := s.r.execer().ExecContext(ctx, `
		UPDATE sessions SET is_active = FALSE, logged_out_at = NOW(), updated_at = NOW() WHERE session_id = $1::uuid
	`, sessionID)
	return err
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
