package repository

import (
	"context"
	"time"

	"food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/models"
	postgresstore "food-delivery-backend/internal/services/users/repository/postgres"
	redisstore "food-delivery-backend/internal/services/users/repository/redis"

	"github.com/jmoiron/sqlx"
	rds "github.com/redis/go-redis/v9"
)

type Repository interface {
	FindUserByPhoneAndRole(ctx context.Context, phone, role string) (*models.UserRow, error)
	FindUserByPhone(ctx context.Context, phone string) (*models.UserRow, error)
	FindUserByID(ctx context.Context, userID string) (*models.UserRow, error)
	UserExistsByPhoneRole(ctx context.Context, phone, role string) (bool, error)
	UserExistsByEmailRole(ctx context.Context, email, role string) (bool, error)
	CreateUser(ctx context.Context, in CreateUserInput) (string, error)
	InsertAuditLog(ctx context.Context, in AuditLogInput) error

	FindLatestActiveOTPByPhone(ctx context.Context, phone string) (*models.OTPRow, error)
	FindLatestUnverifiedOTPByPhoneDevice(ctx context.Context, phone, deviceID string) (*models.OTPRow, error)
	CreateOTPRequest(ctx context.Context, in CreateOTPRequestInput) error
	IncrementOTPResendCount(ctx context.Context, otpID string) error
	IncrementOTPAttempts(ctx context.Context, otpID string) error
	SetOTPBlockedUntil(ctx context.Context, otpID string, blockedUntil time.Time) error
	MarkOTPVerified(ctx context.Context, otpID string) error
	MarkUserPhoneVerified(ctx context.Context, userID string) error
	MarkUserEmailVerified(ctx context.Context, userID string) error
	CountVerifiedOTPs(ctx context.Context, userID string) (int, error)

	CreateSession(ctx context.Context, in CreateSessionInput) error
	DeactivateSession(ctx context.Context, sessionID string) error

	WithTx(ctx context.Context, fn func(tx Repository) error) error

	GetOTPRateCount(ctx context.Context, phone string) (int, error)
	IncrementNotFoundProbe(ctx context.Context, ip string, threshold int64, probeTTL, captchaTTL time.Duration) (bool, error)
	SetOTPHashAndRate(ctx context.Context, phone, hash string, otpTTL, rateTTL time.Duration) error
	ExpireOTP(ctx context.Context, phone string, ttl time.Duration) error
	DeleteOTP(ctx context.Context, phone string) error
	GetEmailOTPRateCount(ctx context.Context, userID, email string) (int, error)
	SetEmailOTPHashAndRate(ctx context.Context, userID, email, hash string, otpTTL, rateTTL time.Duration) error
	GetEmailOTPHash(ctx context.Context, userID, email string) (string, error)
	IncrementEmailOTPAttempts(ctx context.Context, userID, email string, ttl time.Duration) (int, error)
	DeleteEmailOTP(ctx context.Context, userID, email string) error
	SetSession(ctx context.Context, in SetSessionInput, ttl time.Duration) error
	DeleteSession(ctx context.Context, sessionID string) error

	ListRequiredDocumentTypes(ctx context.Context, role, country string) ([]models.DocumentTypeDefinitionRow, error)
	CreateOnboarding(ctx context.Context, in CreateOnboardingInput) (*models.OnboardingRow, error)
	CreateOnboardingDocument(ctx context.Context, in CreateOnboardingDocumentInput) (*models.OnboardingDocumentRow, error)
	FindOnboardingByIDAndUser(ctx context.Context, onboardingID, userID string) (*models.OnboardingRow, error)
	CountPendingOnboardingDocuments(ctx context.Context, onboardingID string) (int, error)
	UpdateOnboardingStatus(ctx context.Context, in UpdateOnboardingStatusInput) error
	MarkOnboardingDocumentUploadedByS3Key(ctx context.Context, s3Key string) (bool, error)
	SetUserOnboardingComplete(ctx context.Context, userID string, isComplete bool) error
}

type repo struct {
	db    *sqlx.DB
	tx    *sqlx.Tx
	redis *rds.Client

	pg    *postgresstore.Store
	cache *redisstore.Store
}

type CreateUserInput struct {
	Phone        string
	Name         string
	Email        string
	Role         string
	ReferralCode string
}

type AuditLogInput struct {
	ActorID    string
	ActorRole  string
	Action     string
	EntityType string
	EntityID   string
}

type CreateOTPRequestInput struct {
	UserID    string
	Phone     string
	DeviceID  string
	IPAddress string
	OTPHash   string
	ExpiresAt time.Time
}

type CreateSessionInput struct {
	SessionID string
	UserID    string
	Phone     string
	Role      string
	DeviceID  string
	IPAddress string
	Platform  string
	ExpiresAt time.Time
}

type SetSessionInput struct {
	SessionID string
	UserID    string
	Role      string
	DeviceID  string
	IPAddress string
	Platform  string
}

func NewRepository(db *sqlx.DB, redisClient *rds.Client) Repository {
	r := &repo{db: db, redis: redisClient}
	r.pg = postgresstore.NewStore(r)
	r.cache = redisstore.NewStore(redisClient)
	return r
}

func (r *repo) Execer() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

func (r *repo) Queryer() sqlx.QueryerContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

func (r *repo) WithTx(ctx context.Context, fn func(tx Repository) error) error {
	if r.tx != nil {
		return fn(r)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	txRepo := &repo{db: r.db, tx: tx, redis: r.redis}
	txRepo.pg = postgresstore.NewStore(txRepo)
	txRepo.cache = r.cache

	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func IsNotFound(err error) bool {
	return errors.IsNotFound(err)
}

func (r *repo) FindUserByPhoneAndRole(ctx context.Context, phone, role string) (*models.UserRow, error) {
	return r.pg.FindUserByPhoneAndRole(ctx, phone, role)
}

func (r *repo) FindUserByPhone(ctx context.Context, phone string) (*models.UserRow, error) {
	return r.pg.FindUserByPhone(ctx, phone)
}

func (r *repo) FindUserByID(ctx context.Context, userID string) (*models.UserRow, error) {
	return r.pg.FindUserByID(ctx, userID)
}

func (r *repo) UserExistsByPhoneRole(ctx context.Context, phone, role string) (bool, error) {
	return r.pg.UserExistsByPhoneRole(ctx, phone, role)
}

func (r *repo) UserExistsByEmailRole(ctx context.Context, email, role string) (bool, error) {
	return r.pg.UserExistsByEmailRole(ctx, email, role)
}

func (r *repo) CreateUser(ctx context.Context, in CreateUserInput) (string, error) {
	return r.pg.CreateUser(ctx, in.Phone, in.Name, in.Email, in.Role, in.ReferralCode)
}

func (r *repo) InsertAuditLog(ctx context.Context, in AuditLogInput) error {
	return r.pg.InsertAuditLog(ctx, in.ActorID, in.ActorRole, in.Action, in.EntityType, in.EntityID)
}

func (r *repo) FindLatestActiveOTPByPhone(ctx context.Context, phone string) (*models.OTPRow, error) {
	return r.pg.FindLatestActiveOTPByPhone(ctx, phone)
}

func (r *repo) FindLatestUnverifiedOTPByPhoneDevice(ctx context.Context, phone, deviceID string) (*models.OTPRow, error) {
	return r.pg.FindLatestUnverifiedOTPByPhoneDevice(ctx, phone, deviceID)
}

func (r *repo) CreateOTPRequest(ctx context.Context, in CreateOTPRequestInput) error {
	return r.pg.CreateOTPRequest(ctx, in.UserID, in.Phone, in.DeviceID, in.IPAddress, in.OTPHash, in.ExpiresAt)
}

func (r *repo) IncrementOTPResendCount(ctx context.Context, otpID string) error {
	return r.pg.IncrementOTPResendCount(ctx, otpID)
}

func (r *repo) IncrementOTPAttempts(ctx context.Context, otpID string) error {
	return r.pg.IncrementOTPAttempts(ctx, otpID)
}

func (r *repo) SetOTPBlockedUntil(ctx context.Context, otpID string, blockedUntil time.Time) error {
	return r.pg.SetOTPBlockedUntil(ctx, otpID, blockedUntil)
}

func (r *repo) MarkOTPVerified(ctx context.Context, otpID string) error {
	return r.pg.MarkOTPVerified(ctx, otpID)
}

func (r *repo) MarkUserPhoneVerified(ctx context.Context, userID string) error {
	return r.pg.MarkUserPhoneVerified(ctx, userID)
}

func (r *repo) MarkUserEmailVerified(ctx context.Context, userID string) error {
	return r.pg.MarkUserEmailVerified(ctx, userID)
}

func (r *repo) CountVerifiedOTPs(ctx context.Context, userID string) (int, error) {
	return r.pg.CountVerifiedOTPs(ctx, userID)
}

func (r *repo) CreateSession(ctx context.Context, in CreateSessionInput) error {
	return r.pg.CreateSession(ctx, in.SessionID, in.UserID, in.Phone, in.Role, in.DeviceID, in.IPAddress, in.Platform, in.ExpiresAt)
}

func (r *repo) DeactivateSession(ctx context.Context, sessionID string) error {
	return r.pg.DeactivateSession(ctx, sessionID)
}

func (r *repo) GetOTPRateCount(ctx context.Context, phone string) (int, error) {
	return r.cache.GetOTPRateCount(ctx, phone)
}

func (r *repo) IncrementNotFoundProbe(ctx context.Context, ip string, threshold int64, probeTTL, captchaTTL time.Duration) (bool, error) {
	return r.cache.IncrementNotFoundProbe(ctx, ip, threshold, probeTTL, captchaTTL)
}

func (r *repo) SetOTPHashAndRate(ctx context.Context, phone, hash string, otpTTL, rateTTL time.Duration) error {
	return r.cache.SetOTPHashAndRate(ctx, phone, hash, otpTTL, rateTTL)
}

func (r *repo) ExpireOTP(ctx context.Context, phone string, ttl time.Duration) error {
	return r.cache.ExpireOTP(ctx, phone, ttl)
}

func (r *repo) DeleteOTP(ctx context.Context, phone string) error {
	return r.cache.DeleteOTP(ctx, phone)
}

func (r *repo) GetEmailOTPRateCount(ctx context.Context, userID, email string) (int, error) {
	return r.cache.GetEmailOTPRateCount(ctx, userID, email)
}

func (r *repo) SetEmailOTPHashAndRate(ctx context.Context, userID, email, hash string, otpTTL, rateTTL time.Duration) error {
	return r.cache.SetEmailOTPHashAndRate(ctx, userID, email, hash, otpTTL, rateTTL)
}

func (r *repo) GetEmailOTPHash(ctx context.Context, userID, email string) (string, error) {
	return r.cache.GetEmailOTPHash(ctx, userID, email)
}

func (r *repo) IncrementEmailOTPAttempts(ctx context.Context, userID, email string, ttl time.Duration) (int, error) {
	return r.cache.IncrementEmailOTPAttempts(ctx, userID, email, ttl)
}

func (r *repo) DeleteEmailOTP(ctx context.Context, userID, email string) error {
	return r.cache.DeleteEmailOTP(ctx, userID, email)
}

func (r *repo) SetSession(ctx context.Context, in SetSessionInput, ttl time.Duration) error {
	return r.cache.SetSession(ctx, redisstore.SetSessionInput{
		SessionID: in.SessionID,
		UserID:    in.UserID,
		Role:      in.Role,
		DeviceID:  in.DeviceID,
		IPAddress: in.IPAddress,
		Platform:  in.Platform,
	}, ttl)
}

func (r *repo) DeleteSession(ctx context.Context, sessionID string) error {
	return r.cache.DeleteSession(ctx, sessionID)
}
