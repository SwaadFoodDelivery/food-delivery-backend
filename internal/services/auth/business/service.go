package business

import (
	"context"
	"net/http"
	"strings"
	"time"

	"food-delivery-backend/internal/services/auth/models"
	"food-delivery-backend/internal/services/auth/repository"
	"food-delivery-backend/pkg/config"
	"food-delivery-backend/pkg/utils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	otpTTL               = 10 * time.Minute
	sessionTTL           = 24 * time.Hour
	authRateWindow       = time.Hour
	maxOTPRatePerPhone   = 5
	maxOTPAttempts       = 5
	otpBlockedWindow     = 30 * time.Minute
	accessTokenTTL       = 15 * time.Minute
	refreshTokenTTL      = 30 * 24 * time.Hour
	notFoundProbeTTL     = time.Minute
	captchaRequiredTTL   = 10 * time.Minute
	notFoundProbeMaxHits = 3
)

type Service interface {
	CheckPhone(ctx context.Context, in models.CheckPhoneInput) (*models.CheckPhoneOutput, *models.ServiceError)
	Register(ctx context.Context, in models.RegisterInput) (*models.OTPSendOutput, *models.ServiceError)
	SendOTP(ctx context.Context, in models.SendOTPInput) (*models.OTPSendOutput, *models.ServiceError)
	VerifyOTP(ctx context.Context, in models.VerifyOTPInput) (*models.VerifyOTPOutput, *models.ServiceError)
	Logout(ctx context.Context, in models.LogoutInput) *models.ServiceError
}

type service struct {
	repo repository.Repository
	cfg  *config.Config
	log  zerolog.Logger
}

func NewService(repo repository.Repository, cfg *config.Config, log zerolog.Logger) Service {
	return &service{repo: repo, cfg: cfg, log: log}
}

func (s *service) CheckPhone(ctx context.Context, in models.CheckPhoneInput) (*models.CheckPhoneOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest("INVALID_PHONE", err.Error())
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
				captchaRequired, probeErr := s.repo.IncrementNotFoundProbe(ctx, in.IP, notFoundProbeMaxHits, notFoundProbeTTL, captchaRequiredTTL)
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

	if user.AccountStatus == "suspended" {
		return &models.CheckPhoneOutput{
			Registered:    true,
			AccountStatus: "suspended",
			Message:       "Your account has been suspended. Please contact support.",
		}, nil
	}

	return &models.CheckPhoneOutput{
		Registered:    true,
		AccountStatus: user.AccountStatus,
		Message:       "Proceed to OTP",
	}, nil
}

func (s *service) Register(ctx context.Context, in models.RegisterInput) (*models.OTPSendOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest("INVALID_PHONE", err.Error())
	}
	if strings.TrimSpace(in.Name) == "" || len(strings.TrimSpace(in.Name)) > 100 {
		return nil, badRequest("VALIDATION_ERROR", "name must be non-empty and at most 100 chars")
	}

	rateCount, err := s.repo.GetOTPRateCount(ctx, phone)
	if err != nil {
		return nil, internalErr("failed to check otp rate")
	}
	if rateCount >= maxOTPRatePerPhone {
		return nil, tooManyRequests("RATE_LIMIT_EXCEEDED", "too many otp requests")
	}

	exists, err := s.repo.UserExistsByPhoneRole(ctx, phone, in.Role)
	if err != nil {
		return nil, internalErr("failed to check existing user")
	}
	if exists {
		return nil, &models.ServiceError{StatusCode: http.StatusConflict, Code: "PHONE_ALREADY_REGISTERED", Message: "phone already registered", Details: []string{}}
	}

	otp, hash, expiresAt, svcErr := createOTPBundle()
	if svcErr != nil {
		return nil, svcErr
	}

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		userID, err := tx.CreateUser(ctx, repository.CreateUserInput{
			Phone:        phone,
			Name:         strings.TrimSpace(in.Name),
			Email:        strings.TrimSpace(in.Email),
			Role:         in.Role,
			ReferralCode: strings.TrimSpace(in.ReferralCode),
		})
		if err != nil {
			return err
		}
		if err := tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  in.Role,
			Action:     "create",
			EntityType: "users",
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
		if err := tx.SetOTPHashAndRate(ctx, phone, hash, otpTTL, authRateWindow); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, internalErr("failed to register user")
	}

	if s.cfg.App.Env == "development" && strings.EqualFold(s.cfg.App.LogLevel, "debug") {
		s.log.Debug().Str("phone", utils.MaskPhone(phone)).Str("otp", otp).Msg("auth.otp_dispatched")
	}

	return otpResponse(phone, expiresAt), nil
}

func (s *service) SendOTP(ctx context.Context, in models.SendOTPInput) (*models.OTPSendOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest("INVALID_PHONE", err.Error())
	}

	user, err := s.repo.FindUserByPhone(ctx, phone)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: "USER_NOT_FOUND", Message: "user not found", Details: []string{}}
		}
		return nil, internalErr("failed to find user")
	}
	if user.AccountStatus == "suspended" {
		return nil, &models.ServiceError{StatusCode: http.StatusForbidden, Code: "ACCOUNT_SUSPENDED", Message: "account suspended", Details: []string{}}
	}

	rateCount, err := s.repo.GetOTPRateCount(ctx, phone)
	if err != nil {
		return nil, internalErr("failed to check otp rate")
	}
	if rateCount >= maxOTPRatePerPhone {
		return nil, tooManyRequests("RATE_LIMIT_EXCEEDED", "too many otp requests")
	}

	otpRec, err := s.repo.FindLatestActiveOTPByPhone(ctx, phone)
	if err == nil {
		if otpRec.ResendCount >= maxOTPRatePerPhone {
			return nil, tooManyRequests("RATE_LIMIT_EXCEEDED", "otp resend limit exceeded")
		}
		if err := s.repo.IncrementOTPResendCount(ctx, otpRec.OTPID); err != nil {
			return nil, internalErr("failed to update otp resend count")
		}
		if err := s.repo.ExpireOTP(ctx, phone, otpTTL); err != nil {
			return nil, internalErr("failed to refresh otp expiry")
		}
		return otpResponse(phone, time.Now().UTC().Add(otpTTL)), nil
	}
	if !repository.IsNotFound(err) {
		return nil, internalErr("failed to check otp state")
	}

	otp, hash, expiresAt, svcErr := createOTPBundle()
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
	if err := s.repo.SetOTPHashAndRate(ctx, phone, hash, otpTTL, authRateWindow); err != nil {
		return nil, internalErr("failed to store otp cache")
	}

	if s.cfg.App.Env == "development" && strings.EqualFold(s.cfg.App.LogLevel, "debug") {
		s.log.Debug().Str("phone", utils.MaskPhone(phone)).Str("otp", otp).Msg("auth.otp_dispatched")
	}

	return otpResponse(phone, expiresAt), nil
}

func (s *service) VerifyOTP(ctx context.Context, in models.VerifyOTPInput) (*models.VerifyOTPOutput, *models.ServiceError) {
	phone, err := utils.RequireValidPhone(in.Phone)
	if err != nil {
		return nil, badRequest("INVALID_PHONE", err.Error())
	}
	if strings.TrimSpace(in.DeviceID) == "" {
		return nil, badRequest("VALIDATION_ERROR", "device_id is required")
	}
	if !utils.ValidateOTP(in.OTP) {
		return nil, badRequest("VALIDATION_ERROR", "otp must be exactly 6 digits")
	}

	otpRec, err := s.repo.FindLatestUnverifiedOTPByPhoneDevice(ctx, phone, in.DeviceID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusUnauthorized, Code: "OTP_INVALID", Message: "invalid otp", Details: []string{}}
		}
		return nil, internalErr("failed to fetch otp request")
	}

	now := time.Now().UTC()
	if otpRec.BlockedUntil != nil && otpRec.BlockedUntil.After(now) {
		return nil, tooManyRequests("OTP_MAX_ATTEMPTS", "otp max attempts reached")
	}
	if otpRec.Attempts >= maxOTPAttempts {
		if err := s.repo.SetOTPBlockedUntil(ctx, otpRec.OTPID, now.Add(otpBlockedWindow)); err != nil {
			return nil, internalErr("failed to block otp attempts")
		}
		return nil, tooManyRequests("OTP_MAX_ATTEMPTS", "otp max attempts reached")
	}

	if otpRec.ExpiresAt.Before(now) {
		_ = s.repo.IncrementOTPAttempts(ctx, otpRec.OTPID)
		return nil, &models.ServiceError{StatusCode: http.StatusGone, Code: "OTP_EXPIRED", Message: "otp expired", Details: []string{}}
	}
	if !utils.CompareOTP(otpRec.OTPHash, in.OTP) {
		_ = s.repo.IncrementOTPAttempts(ctx, otpRec.OTPID)
		return nil, &models.ServiceError{StatusCode: http.StatusUnauthorized, Code: "OTP_INVALID", Message: "invalid otp", Details: []string{}}
	}
	if otpRec.UserID == nil || *otpRec.UserID == "" {
		return nil, internalErr("otp has no user mapping")
	}

	sessionID := uuid.NewString()
	platform := normalizePlatform(in.Platform)
	expiresAt := now.Add(sessionTTL)
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
			Action:     "login",
			EntityType: "sessions",
			EntityID:   sessionID,
		}); err != nil {
			return err
		}
		if err := tx.DeleteOTP(ctx, phone); err != nil {
			return err
		}
		if err := tx.SetSession(ctx, repository.SetSessionInput{
			SessionID: sessionID,
			UserID:    user.UserID,
			Role:      user.Role,
			DeviceID:  in.DeviceID,
			IPAddress: in.IPAddress,
			Platform:  platform,
		}, sessionTTL); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, internalErr("failed to verify otp")
	}

	accessToken, err := utils.CreateAccessToken(s.cfg.JWT.Secret, user.UserID, user.Role, sessionID, accessTokenTTL)
	if err != nil {
		return nil, internalErr("failed to sign access token")
	}
	refreshToken, err := utils.CreateRefreshToken(s.cfg.JWT.Secret, user.UserID, sessionID, refreshTokenTTL)
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
		TokenType:    "Bearer",
		ExpiresIn:    int(accessTokenTTL.Seconds()),
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

func (s *service) Logout(ctx context.Context, in models.LogoutInput) *models.ServiceError {
	if strings.TrimSpace(in.SessionID) == "" || strings.TrimSpace(in.UserID) == "" {
		return badRequest("VALIDATION_ERROR", "missing session or user id")
	}

	err := s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if err := tx.DeactivateSession(ctx, in.SessionID); err != nil {
			return err
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    in.UserID,
			ActorRole:  in.Role,
			Action:     "logout",
			EntityType: "sessions",
			EntityID:   in.SessionID,
		})
	})
	if err != nil {
		return internalErr("failed to logout")
	}

	if err := s.repo.DeleteSession(ctx, in.SessionID); err != nil {
		s.log.Warn().Str("session_id", in.SessionID).Err(err).Msg("auth.logout.redis_delete_failed")
	}
	return nil
}

func createOTPBundle() (string, string, time.Time, *models.ServiceError) {
	otp, err := utils.GenerateOTP()
	if err != nil {
		return "", "", time.Time{}, internalErr("failed to generate otp")
	}
	hash, err := utils.HashOTP(otp)
	if err != nil {
		return "", "", time.Time{}, internalErr("failed to hash otp")
	}
	expiresAt := time.Now().UTC().Add(otpTTL)
	return otp, hash, expiresAt, nil
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
	case "android", "ios", "web":
		return strings.ToLower(strings.TrimSpace(platform))
	default:
		return "web"
	}
}

func badRequest(code, msg string) *models.ServiceError {
	return &models.ServiceError{StatusCode: http.StatusBadRequest, Code: code, Message: msg, Details: []string{}}
}

func tooManyRequests(code, msg string) *models.ServiceError {
	return &models.ServiceError{StatusCode: http.StatusTooManyRequests, Code: code, Message: msg, Details: []string{}}
}

func internalErr(msg string) *models.ServiceError {
	return &models.ServiceError{StatusCode: http.StatusInternalServerError, Code: "INTERNAL_ERROR", Message: msg, Details: []string{}}
}
