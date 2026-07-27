package constants

import "time"

const (
	ResponseStatusSuccess = "success"
	ResponseStatusError   = "error"
)

const (
	RoleClient            = "client"
	RoleRestaurantOwner   = "restaurant_owner"
	RoleRestaurantManager = "restaurant_manager"
	RoleDriver            = "driver"
)

const (
	AccountStatusActive    = "active"
	AccountStatusSuspended = "suspended"
)

const (
	ProviderMock = "mock"
	ProviderDev  = "dev"
)

const (
	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformWeb     = "web"
)

const (
	HeaderAuthorization = "Authorization"
	HeaderDeviceID      = "X-Device-ID"
	HeaderGuestToken    = "X-Guest-Token"
	HeaderAPIKey        = "X-API-Key"
	HeaderClientType    = "X-Client-Type"
	HeaderPlatform      = "X-Platform"
	HeaderRetryAfter    = "Retry-After"
)

const (
	GuestTokenSessionIDKey = "guest_session_id"
	GuestTokenDeviceIDKey  = "guest_device_id"
)

const (
	BearerTokenType = "Bearer"
	BearerPrefix    = "Bearer "
)

const (
	RedisSessionKeyPattern           = "session:%s"
	RedisCartKeyPattern              = "cart:%s"
	RedisOTPKeyPattern               = "otp:%s:%s"
	RedisOTPRateLimitKeyPattern      = "otp:rate:%s:%s"
	RedisEmailOTPKeyPattern          = "email_otp:%s:%s"
	RedisEmailOTPAttemptsKeyPattern  = "email_otp:attempts:%s:%s"
	RedisEmailOTPRateLimitKeyPattern = "email_otp:rate:%s:%s"
	RedisEmailVerifiedKeyPattern     = "email_verified:%s:%s"
	RedisCaptchaRequiredKeyPattern   = "captcha:required:%s"
	RedisSessionActiveValue          = "true"
)

// Email OTP keys serve two callers: pre-registration verification, scoped to a
// guest session, and email changes, scoped to a logged-in user. Callers prefix
// the identity with one of these so the two namespaces can never address the
// same key even if a guest session ID happened to equal a user ID.
const (
	RedisScopeGuest = "guest"
	RedisScopeUser  = "user"
)

const (
	S3BucketPurposeDefault    = "default"
	S3BucketPurposeOnboarding = "onboarding"
)

// NATS JetStream subjects — format: <domain>.<event>
const (
	NATSSubjectOrderPlaced    = "order.placed"
	NATSSubjectOrderConfirmed = "order.confirmed"
)

const (
	StartupDBTimeout        = 45 * time.Second
	StartupMigrationTimeout = 60 * time.Second
	StartupRedisTimeout     = 20 * time.Second
	StartupNATSTimeout      = 20 * time.Second
	StartupGRPCTimeout      = 20 * time.Second
)

const (
	DefaultAppPort              = "8080"
	DefaultCountryISO2          = "IN"
	DefaultOrderGRPCAddr        = "localhost:50051"
	DefaultS3PresignTTLSeconds  = 300
	DefaultS3Region             = "ap-south-1"
	DefaultRateLimitPerMinute   = 60
	DefaultRateLimitWindowSec   = 60
	DefaultValidatedBodyContext = "validated_body"
	DefaultSendGridEndpoint     = "https://api.sendgrid.com/v3/mail/send"
	DefaultGuestTokenTTLMin     = 60
)
