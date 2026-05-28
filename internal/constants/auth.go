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
	AuthMaxOTPAttempts     = 5
	AuthOTPBlockedWindow   = 30 * time.Minute
	AuthAccessTokenTTL     = 15 * time.Minute
	AuthRefreshTokenTTL    = 30 * 24 * time.Hour
)

const (
	AuthNotFoundProbeTTL     = time.Minute
	AuthCaptchaRequiredTTL   = 10 * time.Minute
	AuthNotFoundProbeMaxHits = 3
)

const (
	AuditActionCreate = "create"
	AuditActionLogin  = "login"
	AuditActionLogout = "logout"
)

const (
	EntityTypeUsers    = "users"
	EntityTypeSessions = "sessions"
)
