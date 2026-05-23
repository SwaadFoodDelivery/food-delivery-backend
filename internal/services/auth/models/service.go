package models

import (
	"database/sql"
	"time"
)

type ServiceError struct {
	StatusCode int
	Code       string
	Message    string
	Details    []string
}

func (e *ServiceError) Error() string { return e.Message }

type CheckPhoneInput struct {
	Phone string
	Role  string
	IP    string
}

type RegisterInput struct {
	Phone        string
	Name         string
	Email        string
	ReferralCode string
	Role         string
	DeviceID     string
	IPAddress    string
}

type SendOTPInput struct {
	Phone     string
	DeviceID  string
	IPAddress string
}

type VerifyOTPInput struct {
	Phone      string
	OTP        string
	DeviceID   string
	IPAddress  string
	Platform   string
	ClientType string
}

type LogoutInput struct {
	SessionID string
	UserID    string
	Role      string
}

type CheckPhoneOutput struct {
	Registered      bool   `json:"registered"`
	AccountStatus   string `json:"account_status,omitempty"`
	Message         string `json:"message,omitempty"`
	CaptchaRequired bool   `json:"captcha_required,omitempty"`
}

type OTPSendOutput struct {
	OTPSent      bool   `json:"otp_sent"`
	OTPExpiresAt string `json:"otp_expires_at"`
	MaskedPhone  string `json:"masked_phone"`
	Message      string `json:"message"`
}

type VerifyOTPOutput struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int          `json:"expires_in"`
	User         VerifiedUser `json:"user"`
}

type VerifiedUser struct {
	UserID        string   `json:"user_id"`
	Name          string   `json:"name"`
	Email         *string  `json:"email,omitempty"`
	Phone         string   `json:"phone"`
	EmailVerified bool     `json:"email_verified"`
	PhoneVerified bool     `json:"phone_verified"`
	ReferralCode  *string  `json:"referral_code,omitempty"`
	AccountStatus string   `json:"account_status"`
	RegisteredAt  string   `json:"registered_at"`
	Addresses     []string `json:"addresses"`
	Role          string   `json:"role"`
	FirstTimeUser bool     `json:"first_time_user"`
}

type UserRow struct {
	UserID        string         `db:"user_id"`
	Phone         string         `db:"phone"`
	Role          string         `db:"role"`
	Name          string         `db:"name"`
	Email         sql.NullString `db:"email"`
	EmailVerified bool           `db:"email_verified"`
	PhoneVerified bool           `db:"phone_verified"`
	ReferralCode  sql.NullString `db:"referral_code"`
	AccountStatus string         `db:"account_status"`
	CreatedAt     time.Time      `db:"created_at"`
}

type OTPRow struct {
	OTPID        string     `db:"otp_id"`
	UserID       *string    `db:"user_id"`
	Phone        string     `db:"phone"`
	DeviceID     *string    `db:"device_id"`
	OTPHash      string     `db:"otp_hash"`
	ExpiresAt    time.Time  `db:"expires_at"`
	IsVerified   bool       `db:"is_verified"`
	Attempts     int        `db:"attempts"`
	ResendCount  int        `db:"resend_count"`
	BlockedUntil *time.Time `db:"blocked_until"`
}

type CheckPhoneRequest struct {
	Phone string
	Role  string
}

type RegisterRequest struct {
	Phone        string
	Name         string
	Email        string
	ReferralCode string
	Role         string
}

type SendOTPRequest struct {
	Phone string
}

type VerifyOTPRequest struct {
	Phone string
	OTP   string
}
