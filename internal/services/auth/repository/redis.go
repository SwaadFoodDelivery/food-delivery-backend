package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	appredis "food-delivery-backend/infra/redis"

	rds "github.com/redis/go-redis/v9"
)

type redisStore struct {
	redis *rds.Client
}

func (r *repo) GetOTPRateCount(ctx context.Context, phone string) (int, error) {
	return r.cache.getOTPRateCount(ctx, phone)
}

func (r *repo) IncrementNotFoundProbe(ctx context.Context, ip string, threshold int64, probeTTL, captchaTTL time.Duration) (bool, error) {
	return r.cache.incrementNotFoundProbe(ctx, ip, threshold, probeTTL, captchaTTL)
}

func (r *repo) SetOTPHashAndRate(ctx context.Context, phone, hash string, otpTTL, rateTTL time.Duration) error {
	return r.cache.setOTPHashAndRate(ctx, phone, hash, otpTTL, rateTTL)
}

func (r *repo) ExpireOTP(ctx context.Context, phone string, ttl time.Duration) error {
	return r.cache.expireOTP(ctx, phone, ttl)
}

func (r *repo) DeleteOTP(ctx context.Context, phone string) error {
	return r.cache.deleteOTP(ctx, phone)
}

func (r *repo) SetSession(ctx context.Context, in SetSessionInput, ttl time.Duration) error {
	return r.cache.setSession(ctx, in, ttl)
}

func (r *repo) DeleteSession(ctx context.Context, sessionID string) error {
	return r.cache.deleteSession(ctx, sessionID)
}

func (s *redisStore) getOTPRateCount(ctx context.Context, phone string) (int, error) {
	val, err := s.redis.Get(ctx, otpRateKey(phone)).Result()
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

func (s *redisStore) incrementNotFoundProbe(ctx context.Context, ip string, threshold int64, probeTTL, captchaTTL time.Duration) (bool, error) {
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

func (s *redisStore) setOTPHashAndRate(ctx context.Context, phone, hash string, otpTTL, rateTTL time.Duration) error {
	if err := s.redis.Set(ctx, otpKey(phone), hash, otpTTL).Err(); err != nil {
		return err
	}
	cnt, err := s.redis.Incr(ctx, otpRateKey(phone)).Result()
	if err != nil {
		return err
	}
	if cnt == 1 {
		_ = s.redis.Expire(ctx, otpRateKey(phone), rateTTL).Err()
	}
	return nil
}

func (s *redisStore) expireOTP(ctx context.Context, phone string, ttl time.Duration) error {
	return s.redis.Expire(ctx, otpKey(phone), ttl).Err()
}

func (s *redisStore) deleteOTP(ctx context.Context, phone string) error {
	return s.redis.Del(ctx, otpKey(phone)).Err()
}

func (s *redisStore) setSession(ctx context.Context, in SetSessionInput, ttl time.Duration) error {
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

func (s *redisStore) deleteSession(ctx context.Context, sessionID string) error {
	return s.redis.Del(ctx, appredis.SessionKey(sessionID)).Err()
}
