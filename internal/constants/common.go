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
	HeaderClientType    = "X-Client-Type"
	HeaderPlatform      = "X-Platform"
	HeaderRetryAfter    = "Retry-After"
)

const (
	BearerTokenType = "Bearer"
	BearerPrefix    = "Bearer "
)

const (
	RedisSessionKeyPattern           = "session:%s"
	RedisCartKeyPattern              = "cart:%s"
	RedisOTPKeyPattern               = "otp:%s"
	RedisOTPRateLimitKeyPattern      = "otp:rate:%s"
	RedisEmailOTPKeyPattern          = "email_otp:%s:%s"
	RedisEmailOTPAttemptsKeyPattern  = "email_otp:attempts:%s:%s"
	RedisEmailOTPRateLimitKeyPattern = "email_otp:rate:%s:%s"
	RedisCaptchaRequiredKeyPattern   = "captcha:required:%s"
	RedisSessionActiveValue          = "true"
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
)
