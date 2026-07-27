package constants

import "time"

const (
	AuthContextUserIDKey    = "user_id"
	AuthContextRoleKey      = "role"
	AuthContextSessionIDKey = "sid"
	AuthContextClaimsKey    = "auth_claims"
)

const (
	AuthOTPTTL             = 10 * time.Minute
	AuthOTPHashCost        = 12
	AuthSessionTTL         = 24 * time.Hour
	AuthRateWindow         = time.Hour
	AuthMaxOTPRatePerPhone = 5
	AuthMaxOTPRatePerEmail = 5
	AuthMaxOTPAttempts     = 5
	AuthOTPBlockedWindow   = 30 * time.Minute
	// AuthEmailVerifiedTTL is how long a pre-registration email verification stays
	// valid. The user must complete registration within this window.
	AuthEmailVerifiedTTL = 30 * time.Minute
	AuthAccessTokenTTL   = 60 * time.Minute // change it back to 15 minutes in production after testing
	AuthRefreshTokenTTL  = 30 * 24 * time.Hour
)

const (
	AuthNotFoundProbeTTL     = time.Minute
	AuthCaptchaRequiredTTL   = 10 * time.Minute
	AuthNotFoundProbeMaxHits = 3
)

const (
	AuditActionCreate      = "create"
	AuditActionLogin       = "login"
	AuditActionLogout      = "logout"
	AuditActionVerifyEmail = "verify_email"
)

const (
	EntityTypeUsers    = "users"
	EntityTypeSessions = "sessions"
)
