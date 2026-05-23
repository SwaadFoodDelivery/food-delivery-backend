package repository

import (
	"context"
	"time"

	appredis "food-delivery-backend/infra/redis"
	"food-delivery-backend/internal/services/auth/models"

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
	CountVerifiedOTPs(ctx context.Context, userID string) (int, error)

	CreateSession(ctx context.Context, in CreateSessionInput) error
	DeactivateSession(ctx context.Context, sessionID string) error

	WithTx(ctx context.Context, fn func(tx Repository) error) error

	GetOTPRateCount(ctx context.Context, phone string) (int, error)
	IncrementNotFoundProbe(ctx context.Context, ip string, threshold int64, probeTTL, captchaTTL time.Duration) (bool, error)
	SetOTPHashAndRate(ctx context.Context, phone, hash string, otpTTL, rateTTL time.Duration) error
	ExpireOTP(ctx context.Context, phone string, ttl time.Duration) error
	DeleteOTP(ctx context.Context, phone string) error
	SetSession(ctx context.Context, in SetSessionInput, ttl time.Duration) error
	DeleteSession(ctx context.Context, sessionID string) error
}

type repo struct {
	db       *sqlx.DB
	tx       *sqlx.Tx
	redis    *rds.Client
	postgres *postgresStore
	cache    *redisStore
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
	r.postgres = &postgresStore{r: r}
	r.cache = &redisStore{redis: redisClient}
	return r
}

func (r *repo) execer() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

func (r *repo) queryer() sqlx.QueryerContext {
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
	txRepo.postgres = &postgresStore{r: txRepo}
	txRepo.cache = r.cache

	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func otpKey(phone string) string {
	return appredis.OTPKey(phone)
}

func otpRateKey(phone string) string {
	return appredis.OTPRateKey(phone)
}
