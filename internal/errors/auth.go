package errors

const (
	CodeInvalidPhone            = "INVALID_PHONE"
	CodeTokenExpired            = "TOKEN_EXPIRED"
	CodeInvalidToken            = "INVALID_TOKEN"
	CodeSessionNotFound         = "SESSION_NOT_FOUND"
	CodeSessionRevoked          = "SESSION_REVOKED"
	CodePhoneAlreadyRegistered  = "PHONE_ALREADY_REGISTERED"
	CodeEmailAlreadyRegistered  = "EMAIL_ALREADY_REGISTERED"
	CodeEmailAlreadyVerified    = "EMAIL_ALREADY_VERIFIED"
	CodeEmailMissing            = "EMAIL_MISSING"
	CodeEmailNotVerified        = "EMAIL_NOT_VERIFIED"
	CodeAccountSuspended        = "ACCOUNT_SUSPENDED"
	CodeOTPInvalid              = "OTP_INVALID"
	CodeOTPMaxAttempts          = "OTP_MAX_ATTEMPTS"
	CodeOTPExpired              = "OTP_EXPIRED"
	CodeGuestTokenMissing       = "GUEST_TOKEN_MISSING"
	CodeGuestTokenInvalid       = "GUEST_TOKEN_INVALID"
	CodeRoleNotSelfRegisterable = "ROLE_NOT_SELF_REGISTERABLE"
)
