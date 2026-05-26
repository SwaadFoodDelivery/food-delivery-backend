package redis

import (
	"context"
	"errors"
	"strconv"
	"time"

	appredis "food-delivery-backend/infra/redis"

	rds "github.com/redis/go-redis/v9"
)

type Store struct {
	redis *rds.Client
}

type SetSessionInput struct {
	SessionID string
	UserID    string
	Role      string
	DeviceID  string
	IPAddress string
	Platform  string
}

func NewStore(client *rds.Client) *Store {
	return &Store{redis: client}
}

func (s *Store) GetOTPRateCount(ctx context.Context, phone string) (int, error) {
	val, err := s.redis.Get(ctx, appredis.OTPRateKey(phone)).Result()
	if err != nil {
		if errors.Is(err, rds.Nil) {
			return 0, nil
		}
		return 0, err
	}
	count, convErr := strconv.Atoi(val)
	if convErr != nil {
		return 0, nil
	}
	return count, nil
}

func (s *Store) IncrementNotFoundProbe(ctx context.Context, ip string, threshold int64, probeTTL, captchaTTL time.Duration) (bool, error) {
	probeKey := "auth:probe:" + ip
	cnt, err := s.redis.Incr(ctx, probeKey).Result()
	if err != nil {
		return false, err
	}
	if cnt == 1 {
		_ = s.redis.Expire(ctx, probeKey, probeTTL).Err()
	}
	if cnt >= threshold {
		if err := s.redis.Set(ctx, appredis.CaptchaRequiredKey(ip), "true", captchaTTL).Err(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (s *Store) SetOTPHashAndRate(ctx context.Context, phone, hash string, otpTTL, rateTTL time.Duration) error {
	if err := s.redis.Set(ctx, appredis.OTPKey(phone), hash, otpTTL).Err(); err != nil {
		return err
	}
	cnt, err := s.redis.Incr(ctx, appredis.OTPRateKey(phone)).Result()
	if err != nil {
		return err
	}
	if cnt == 1 {
		_ = s.redis.Expire(ctx, appredis.OTPRateKey(phone), rateTTL).Err()
	}
	return nil
}

func (s *Store) ExpireOTP(ctx context.Context, phone string, ttl time.Duration) error {
	return s.redis.Expire(ctx, appredis.OTPKey(phone), ttl).Err()
}

func (s *Store) DeleteOTP(ctx context.Context, phone string) error {
	return s.redis.Del(ctx, appredis.OTPKey(phone)).Err()
}

func (s *Store) SetSession(ctx context.Context, in SetSessionInput, ttl time.Duration) error {
	key := appredis.SessionKey(in.SessionID)
	if _, err := s.redis.HSet(ctx, key,
		"user_id", in.UserID,
		"role", in.Role,
		"device_id", in.DeviceID,
		"is_active", "true",
		"ip_address", in.IPAddress,
		"platform", in.Platform,
	).Result(); err != nil {
		return err
	}
	return s.redis.Expire(ctx, key, ttl).Err()
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	return s.redis.Del(ctx, appredis.SessionKey(sessionID)).Err()
}
