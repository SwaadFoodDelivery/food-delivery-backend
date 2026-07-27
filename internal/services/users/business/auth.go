package business

import (
	"context"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/common/email"
	"food-delivery-backend/internal/services/common/otp"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/internal/services/users/repository/repository"
	"food-delivery-backend/pkg/config"
	"food-delivery-backend/pkg/utils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type AuthService interface {
	CheckPhone(ctx context.Context, in models.CheckPhoneInput) (*models.CheckPhoneOutput, *models.ServiceError)
	Register(ctx context.Context, in models.RegisterInput) (*models.OTPSendOutput, *models.ServiceError)
	SendOTP(ctx context.Context, in models.SendOTPInput) (*models.OTPSendOutput, *models.ServiceError)
	VerifyOTP(ctx context.Context, in models.VerifyOTPInput) (*models.VerifyOTPOutput, *models.ServiceError)
	SendEmailOTP(ctx context.Context, in models.SendEmailOTPInput) (*models.EmailOTPSendOutput, *models.ServiceError)
	VerifyEmail(ctx context.Context, in models.VerifyEmailInput) (*models.VerifyEmailOutput, *models.ServiceError)
	Logout(ctx context.Context, in models.LogoutInput) *models.ServiceError
}

type Service struct {
	repo            repository.Repository
	cfg             *config.Config
	log             zerolog.Logger
	otpProvider     otp.Provider
	emailProvider   email.Provider
	storageProvider storage.Provider
}

func NewService(repo repository.Repository, cfg *config.Config, log zerolog.Logger, otpProvider otp.Provider, emailProvider email.Provider, storageProvider storage.Provider) *Service {
	return &Service{repo: repo, cfg: cfg, log: log, otpProvider: otpProvider, emailProvider: emailProvider, storageProvider: storageProvider}
}

func (s *Service) CheckPhone(ctx context.Context, in models.CheckPhoneInput) (*models.CheckPhoneOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest(apperrors.CodeInvalidPhone, err.Error())
	}
	role := strings.TrimSpace(in.Role)

	user, err := s.repo.FindUserByPhoneAndRole(ctx, phone, role)
	if err != nil {
		if repository.IsNotFound(err) {
			out := &models.CheckPhoneOutput{
				Registered: false,
				Message:    "Account not found. Please register.",
			}
			if in.IP != "" {
				captchaRequired, probeErr := s.repo.IncrementNotFoundProbe(ctx, in.IP, constants.AuthNotFoundProbeMaxHits, constants.AuthNotFoundProbeTTL, constants.AuthCaptchaRequiredTTL)
				if probeErr != nil {
					return nil, internalErr("failed to process auth probe")
				}
				out.CaptchaRequired = captchaRequired
				if captchaRequired {
					out.Message = "Account not found. Please complete captcha and continue."
				}
			}
			return out, nil
		}
		return nil, internalErr("failed to lookup phone")
	}

	if user.AccountStatus == constants.AccountStatusSuspended {
		return &models.CheckPhoneOutput{
			Registered:    true,
			AccountStatus: constants.AccountStatusSuspended,
			Message:       "Your account has been suspended. Please contact support.",
		}, nil
	}

	return &models.CheckPhoneOutput{
		Registered:    true,
		AccountStatus: user.AccountStatus,
		Message:       "Proceed to OTP",
	}, nil
}

func (s *Service) Register(ctx context.Context, in models.RegisterInput) (*models.OTPSendOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest(apperrors.CodeInvalidPhone, err.Error())
	}
	if strings.TrimSpace(in.Name) == "" || len(strings.TrimSpace(in.Name)) > 100 {
		return nil, badRequest(apperrors.CodeValidation, "name must be non-empty and at most 100 chars")
	}
	email, svcErr := normalizeEmail(in.Email)
	if svcErr != nil {
		return nil, svcErr
	}

	// An account can only be created for an email this guest session has already
	// verified. Without the marker, registration is refused outright.
	sid := strings.TrimSpace(in.GuestSessionID)
	if sid == "" {
		return nil, badRequest(apperrors.CodeGuestTokenInvalid, "guest session is required")
	}
	verified, err := s.repo.IsEmailVerified(ctx, sid, email)
	if err != nil {
		return nil, internalErr("failed to check email verification")
	}
	if !verified {
		return nil, &models.ServiceError{
			StatusCode: http.StatusForbidden,
			Code:       apperrors.CodeEmailNotVerified,
			Message:    "verify your email via /auth/send-email-otp and /auth/verify-email before registering",
			Details:    []string{},
		}
	}

	rateCount, err := s.repo.GetOTPRateCount(ctx, phone, in.Role)
	if err != nil {
		return nil, internalErr("failed to check otp rate")
	}
	if rateCount >= constants.AuthMaxOTPRatePerPhone {
		return nil, tooManyRequests(apperrors.CodeRateLimited, "too many otp requests")
	}

	exists, err := s.repo.UserExistsByPhoneRole(ctx, phone, in.Role)
	if err != nil {
		return nil, internalErr("failed to check existing user")
	}
	if exists {
		return nil, &models.ServiceError{StatusCode: http.StatusConflict, Code: apperrors.CodePhoneAlreadyRegistered, Message: "phone already registered", Details: []string{}}
	}
	emailExists, err := s.repo.UserExistsByEmailRole(ctx, email, in.Role)
	if err != nil {
		return nil, internalErr("failed to check existing email")
	}
	if emailExists {
		return nil, &models.ServiceError{StatusCode: http.StatusConflict, Code: apperrors.CodeEmailAlreadyRegistered, Message: "email already registered for this role", Details: []string{}}
	}

	otpCode, hash, expiresAt, svcErr := createOTPBundle()
	if svcErr != nil {
		return nil, svcErr
	}

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		userID, err := tx.CreateUser(ctx, repository.CreateUserInput{
			Phone:         phone,
			Name:          strings.TrimSpace(in.Name),
			Email:         email,
			Role:          in.Role,
			ReferralCode:  strings.TrimSpace(in.ReferralCode),
			EmailVerified: true,
		})
		if err != nil {
			return err
		}
		if err := tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  in.Role,
			Action:     constants.AuditActionCreate,
			EntityType: constants.EntityTypeUsers,
			EntityID:   userID,
		}); err != nil {
			return err
		}
		if err := tx.CreateOTPRequest(ctx, repository.CreateOTPRequestInput{
			UserID:    userID,
			Phone:     phone,
			DeviceID:  in.DeviceID,
			IPAddress: in.IPAddress,
			OTPHash:   hash,
			ExpiresAt: expiresAt,
		}); err != nil {
			return err
		}
		if err := tx.SetOTPHashAndRate(ctx, phone, in.Role, hash, constants.AuthOTPTTL, constants.AuthRateWindow); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, internalErr("failed to register user")
	}

	// One verification, one account.
	_ = s.repo.DeleteEmailVerified(ctx, sid, email)

	if err := s.otpProvider.Send(ctx, phone, otpCode); err != nil {
		return nil, internalErr("failed to dispatch otp")
	}

	return otpResponse(phone, expiresAt), nil
}

func (s *Service) SendOTP(ctx context.Context, in models.SendOTPInput) (*models.OTPSendOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest(apperrors.CodeInvalidPhone, err.Error())
	}

	role := strings.TrimSpace(in.Role)
	if role == "" {
		return nil, badRequest(apperrors.CodeValidation, "role is required")
	}

	// Resolved by phone AND role: one number may own a separate account per role,
	// and each is a distinct entity with its own OTP, budget, and session.
	user, err := s.repo.FindUserByPhoneAndRole(ctx, phone, role)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeUserNotFound, Message: "user not found", Details: []string{}}
		}
		return nil, internalErr("failed to find user")
	}
	if user.AccountStatus == constants.AccountStatusSuspended {
		return nil, &models.ServiceError{StatusCode: http.StatusForbidden, Code: apperrors.CodeAccountSuspended, Message: "account suspended", Details: []string{}}
	}

	rateCount, err := s.repo.GetOTPRateCount(ctx, phone, role)
	if err != nil {
		return nil, internalErr("failed to check otp rate")
	}
	if rateCount >= constants.AuthMaxOTPRatePerPhone {
		return nil, tooManyRequests(apperrors.CodeRateLimited, "too many otp requests")
	}

	otpRec, err := s.repo.FindLatestActiveOTPByUser(ctx, user.UserID)
	if err == nil {
		if otpRec.ResendCount >= constants.AuthMaxOTPRatePerPhone {
			return nil, tooManyRequests(apperrors.CodeRateLimited, "otp resend limit exceeded")
		}
		if err := s.repo.IncrementOTPResendCount(ctx, otpRec.OTPID); err != nil {
			return nil, internalErr("failed to update otp resend count")
		}
	} else if !repository.IsNotFound(err) {
		return nil, internalErr("failed to check otp state")
	}

	otpCode, hash, expiresAt, svcErr := createOTPBundle()
	if svcErr != nil {
		return nil, svcErr
	}
	if err := s.repo.CreateOTPRequest(ctx, repository.CreateOTPRequestInput{
		UserID:    user.UserID,
		Phone:     phone,
		DeviceID:  in.DeviceID,
		IPAddress: in.IPAddress,
		OTPHash:   hash,
		ExpiresAt: expiresAt,
	}); err != nil {
		return nil, internalErr("failed to create otp request")
	}
	if err := s.repo.SetOTPHashAndRate(ctx, phone, role, hash, constants.AuthOTPTTL, constants.AuthRateWindow); err != nil {
		return nil, internalErr("failed to store otp cache")
	}

	if err := s.otpProvider.Send(ctx, phone, otpCode); err != nil {
		return nil, internalErr("failed to dispatch otp")
	}

	return otpResponse(phone, expiresAt), nil
}

func (s *Service) VerifyOTP(ctx context.Context, in models.VerifyOTPInput) (*models.VerifyOTPOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest(apperrors.CodeInvalidPhone, err.Error())
	}
	if strings.TrimSpace(in.DeviceID) == "" {
		return nil, badRequest(apperrors.CodeValidation, "device_id is required")
	}
	if !utils.ValidateOTP(in.OTP) {
		return nil, badRequest(apperrors.CodeValidation, "otp must be exactly 6 digits")
	}
	role := strings.TrimSpace(in.Role)
	if role == "" {
		return nil, badRequest(apperrors.CodeValidation, "role is required")
	}

	// Resolve the account first, then look the OTP up against that account. A
	// phone-scoped lookup would let an OTP issued for one role be redeemed into
	// another account on the same number.
	account, err := s.repo.FindUserByPhoneAndRole(ctx, phone, role)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusUnauthorized, Code: apperrors.CodeOTPInvalid, Message: "invalid otp", Details: []string{}}
		}
		return nil, internalErr("failed to find user")
	}

	otpRec, err := s.repo.FindLatestUnverifiedOTPByUserDevice(ctx, account.UserID, in.DeviceID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusUnauthorized, Code: apperrors.CodeOTPInvalid, Message: "invalid otp", Details: []string{}}
		}
		return nil, internalErr("failed to fetch otp request")
	}

	now := time.Now().UTC()
	if otpRec.BlockedUntil != nil && otpRec.BlockedUntil.After(now) {
		return nil, tooManyRequests(apperrors.CodeOTPMaxAttempts, "otp max attempts reached")
	}
	if otpRec.Attempts >= constants.AuthMaxOTPAttempts {
		if err := s.repo.SetOTPBlockedUntil(ctx, otpRec.OTPID, now.Add(constants.AuthOTPBlockedWindow)); err != nil {
			return nil, internalErr("failed to block otp attempts")
		}
		return nil, tooManyRequests(apperrors.CodeOTPMaxAttempts, "otp max attempts reached")
	}

	if otpRec.ExpiresAt.Before(now) {
		_ = s.repo.IncrementOTPAttempts(ctx, otpRec.OTPID)
		return nil, &models.ServiceError{StatusCode: http.StatusGone, Code: apperrors.CodeOTPExpired, Message: "otp expired", Details: []string{}}
	}
	if !utils.CompareOTP(otpRec.OTPHash, in.OTP) {
		_ = s.repo.IncrementOTPAttempts(ctx, otpRec.OTPID)
		return nil, &models.ServiceError{StatusCode: http.StatusUnauthorized, Code: apperrors.CodeOTPInvalid, Message: "invalid otp", Details: []string{}}
	}
	if otpRec.UserID == nil || *otpRec.UserID == "" {
		return nil, internalErr("otp has no user mapping")
	}

	sessionID := uuid.NewString()
	platform := normalizePlatform(in.Platform)
	expiresAt := now.Add(constants.AuthSessionTTL)
	var user *models.UserRow

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if err := tx.MarkOTPVerified(ctx, otpRec.OTPID); err != nil {
			return err
		}
		if err := tx.MarkUserPhoneVerified(ctx, *otpRec.UserID); err != nil {
			return err
		}
		loadedUser, err := tx.FindUserByID(ctx, *otpRec.UserID)
		if err != nil {
			return err
		}
		user = loadedUser
		if err := tx.CreateSession(ctx, repository.CreateSessionInput{
			SessionID: sessionID,
			UserID:    user.UserID,
			Phone:     user.Phone,
			Role:      user.Role,
			DeviceID:  in.DeviceID,
			IPAddress: in.IPAddress,
			Platform:  platform,
			ExpiresAt: expiresAt,
		}); err != nil {
			return err
		}
		if err := tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    user.UserID,
			ActorRole:  user.Role,
			Action:     constants.AuditActionLogin,
			EntityType: constants.EntityTypeSessions,
			EntityID:   sessionID,
		}); err != nil {
			return err
		}
		if err := tx.DeleteOTP(ctx, phone, role); err != nil {
			return err
		}
		if err := tx.SetSession(ctx, repository.SetSessionInput{
			SessionID: sessionID,
			UserID:    user.UserID,
			Role:      user.Role,
			DeviceID:  in.DeviceID,
			IPAddress: in.IPAddress,
			Platform:  platform,
		}, constants.AuthSessionTTL); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, internalErr("failed to verify otp")
	}

	accessToken, err := utils.CreateAccessToken(s.cfg.JWT.Secret, user.UserID, user.Role, sessionID, constants.AuthAccessTokenTTL)
	if err != nil {
		return nil, internalErr("failed to sign access token")
	}
	refreshToken, err := utils.CreateRefreshToken(s.cfg.JWT.Secret, user.UserID, sessionID, constants.AuthRefreshTokenTTL)
	if err != nil {
		return nil, internalErr("failed to sign refresh token")
	}

	verifiedCount, err := s.repo.CountVerifiedOTPs(ctx, user.UserID)
	if err != nil {
		return nil, internalErr("failed to compute user verification state")
	}

	out := &models.VerifyOTPOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    constants.BearerTokenType,
		ExpiresIn:    int(constants.AuthAccessTokenTTL.Seconds()),
		User: models.VerifiedUser{
			UserID:        user.UserID,
			Name:          user.Name,
			Phone:         user.Phone,
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			AccountStatus: user.AccountStatus,
			RegisteredAt:  user.CreatedAt.Format(time.RFC3339),
			Addresses:     []string{},
			Role:          user.Role,
			FirstTimeUser: verifiedCount == 1,
		},
	}
	if user.Email.Valid {
		email := user.Email.String
		out.User.Email = &email
	}
	if user.ReferralCode.Valid {
		ref := user.ReferralCode.String
		out.User.ReferralCode = &ref
	}

	return out, nil
}

// SendEmailOTP issues an email OTP before any account exists. It is scoped to the
// caller's guest session, so the resulting verification cannot be replayed from a
// different device or session.
func (s *Service) SendEmailOTP(ctx context.Context, in models.SendEmailOTPInput) (*models.EmailOTPSendOutput, *models.ServiceError) {
	sid := strings.TrimSpace(in.GuestSessionID)
	if sid == "" {
		return nil, badRequest(apperrors.CodeGuestTokenInvalid, "guest session is required")
	}
	emailAddr, svcErr := normalizeEmail(in.Email)
	if svcErr != nil {
		return nil, svcErr
	}
	if s.emailProvider == nil {
		return nil, internalErr("email provider is not configured")
	}
	if svcErr := s.enforceEmailOTPRate(ctx, sid, emailAddr); svcErr != nil {
		return nil, svcErr
	}

	otpCode, hash, expiresAt, svcErr := createOTPBundle()
	if svcErr != nil {
		return nil, svcErr
	}
	if err := s.repo.SetEmailOTPHashAndRate(ctx, sid, emailAddr, hash, constants.AuthOTPTTL, constants.AuthRateWindow); err != nil {
		return nil, internalErr("failed to store email otp")
	}
	if err := s.emailProvider.SendVerificationOTP(ctx, emailAddr, otpCode); err != nil {
		return nil, internalErr("failed to dispatch email otp")
	}
	return emailOTPResponse(emailAddr, expiresAt, "OTP sent to email and valid for 10 minutes", true), nil
}

// VerifyEmail confirms the OTP and records the verification against the guest
// session. Register then requires that marker before it will create an account.
func (s *Service) VerifyEmail(ctx context.Context, in models.VerifyEmailInput) (*models.VerifyEmailOutput, *models.ServiceError) {
	sid := strings.TrimSpace(in.GuestSessionID)
	if sid == "" {
		return nil, badRequest(apperrors.CodeGuestTokenInvalid, "guest session is required")
	}
	emailAddr, svcErr := normalizeEmail(in.Email)
	if svcErr != nil {
		return nil, svcErr
	}
	if !utils.ValidateOTP(in.OTP) {
		return nil, badRequest(apperrors.CodeValidation, "otp must be exactly 6 digits")
	}

	hash, err := s.repo.GetEmailOTPHash(ctx, sid, emailAddr)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusGone, Code: apperrors.CodeOTPExpired, Message: "email otp expired or not found", Details: []string{}}
		}
		return nil, internalErr("failed to fetch email otp")
	}
	if svcErr := s.verifyEmailOTPHash(ctx, sid, emailAddr, hash, in.OTP); svcErr != nil {
		return nil, svcErr
	}
	if err := s.repo.SetEmailVerified(ctx, sid, emailAddr, constants.AuthEmailVerifiedTTL); err != nil {
		return nil, internalErr("failed to record email verification")
	}
	_ = s.repo.DeleteEmailOTP(ctx, sid, emailAddr)
	return verifyEmailResponse(emailAddr, true, "Email verified. Continue to registration within 30 minutes."), nil
}

func (s *Service) Logout(ctx context.Context, in models.LogoutInput) *models.ServiceError {
	if strings.TrimSpace(in.SessionID) == "" || strings.TrimSpace(in.UserID) == "" {
		return badRequest(apperrors.CodeValidation, "missing session or user id")
	}

	err := s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if err := tx.DeactivateSession(ctx, in.SessionID); err != nil {
			return err
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    in.UserID,
			ActorRole:  in.Role,
			Action:     constants.AuditActionLogout,
			EntityType: constants.EntityTypeSessions,
			EntityID:   in.SessionID,
		})
	})
	if err != nil {
		return internalErr("failed to logout")
	}

	if err := s.repo.DeleteSession(ctx, in.SessionID); err != nil {
		ctxLog := zerolog.Ctx(ctx)
		if ctxLog == nil || ctxLog.GetLevel() == zerolog.Disabled {
			ctxLog = &s.log
		}
		ctxLog.Warn().Str("session_id", in.SessionID).Err(err).Msg("users.logout.redis_delete_failed")
	}
	return nil
}

func (s *Service) enforceEmailOTPRate(ctx context.Context, userID, emailAddr string) *models.ServiceError {
	rateCount, err := s.repo.GetEmailOTPRateCount(ctx, userID, emailAddr)
	if err != nil {
		return internalErr("failed to check email otp rate")
	}
	if rateCount >= constants.AuthMaxOTPRatePerEmail {
		return tooManyRequests(apperrors.CodeRateLimited, "too many email otp requests")
	}
	return nil
}

func (s *Service) verifyEmailOTPHash(ctx context.Context, userID, emailAddr, hash, otpCode string) *models.ServiceError {
	if utils.CompareOTP(hash, otpCode) {
		return nil
	}
	attempts, err := s.repo.IncrementEmailOTPAttempts(ctx, userID, emailAddr, constants.AuthOTPTTL)
	if err != nil {
		return internalErr("failed to update email otp attempts")
	}
	if attempts >= constants.AuthMaxOTPAttempts {
		_ = s.repo.DeleteEmailOTP(ctx, userID, emailAddr)
		return tooManyRequests(apperrors.CodeOTPMaxAttempts, "email otp max attempts reached")
	}
	return &models.ServiceError{StatusCode: http.StatusUnauthorized, Code: apperrors.CodeOTPInvalid, Message: "invalid email otp", Details: []string{}}
}

func normalizeEmail(emailAddr string) (string, *models.ServiceError) {
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))
	if emailAddr == "" {
		return "", badRequest(apperrors.CodeEmailMissing, "email is required")
	}
	parsed, err := mail.ParseAddress(emailAddr)
	if err != nil {
		return "", badRequest(apperrors.CodeValidation, "email must be valid")
	}
	return strings.ToLower(strings.TrimSpace(parsed.Address)), nil
}

func createOTPBundle() (string, string, time.Time, *models.ServiceError) {
	otpCode, err := utils.GenerateOTP()
	if err != nil {
		return "", "", time.Time{}, internalErr("failed to generate otp")
	}
	hash, err := utils.HashOTP(otpCode)
	if err != nil {
		return "", "", time.Time{}, internalErr("failed to hash otp")
	}
	expiresAt := time.Now().UTC().Add(constants.AuthOTPTTL)
	return otpCode, hash, expiresAt, nil
}

func emailOTPResponse(emailAddr string, expiresAt time.Time, message string, sent bool) *models.EmailOTPSendOutput {
	expires := ""
	if sent {
		expires = expiresAt.UTC().Format(time.RFC3339)
	}
	return &models.EmailOTPSendOutput{
		OTPSent:      sent,
		OTPExpiresAt: expires,
		MaskedEmail:  utils.MaskEmail(emailAddr),
		Message:      message,
	}
}

func verifyEmailResponse(emailAddr string, verified bool, message string) *models.VerifyEmailOutput {
	return &models.VerifyEmailOutput{
		EmailVerified: verified,
		MaskedEmail:   utils.MaskEmail(emailAddr),
		Message:       message,
	}
}

func otpResponse(phone string, expiresAt time.Time) *models.OTPSendOutput {
	return &models.OTPSendOutput{
		OTPSent:      true,
		OTPExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		MaskedPhone:  "+91" + utils.MaskPhone(phone),
		Message:      "OTP sent to registered phone number and valid for 10 minutes",
	}
}

func normalizePlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case constants.PlatformAndroid, constants.PlatformIOS, constants.PlatformWeb:
		return strings.ToLower(strings.TrimSpace(platform))
	default:
		return constants.PlatformWeb
	}
}

func badRequest(code, msg string) *models.ServiceError {
	return &models.ServiceError{StatusCode: http.StatusBadRequest, Code: code, Message: msg, Details: []string{}}
}

func tooManyRequests(code, msg string) *models.ServiceError {
	return &models.ServiceError{StatusCode: http.StatusTooManyRequests, Code: code, Message: msg, Details: []string{}}
}

func internalErr(msg string) *models.ServiceError {
	return &models.ServiceError{StatusCode: http.StatusInternalServerError, Code: apperrors.CodeInternal, Message: msg, Details: []string{}}
}
