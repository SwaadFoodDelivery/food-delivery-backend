package middleware

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/pkg/response"
	"food-delivery-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	rds "github.com/redis/go-redis/v9"
)

type RateLimitKeyFunc func(c *gin.Context) string

var leakyBucketScript = rds.NewScript(`
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])

local bucket = redis.call('HMGET', KEYS[1], 'tokens', 'last_refill')
local tokens = tonumber(bucket[1]) or capacity
local last_refill = tonumber(bucket[2]) or now

local elapsed = math.max(0, now - last_refill)
tokens = math.min(capacity, tokens + elapsed * rate)

if tokens < 1 then
	local retry_after_ms = math.ceil((1 - tokens) / rate)
	return {0, retry_after_ms}
end

tokens = tokens - 1
redis.call('HMSET', KEYS[1], 'tokens', tokens, 'last_refill', now)
redis.call('EXPIRE', KEYS[1], ttl)
return {1, 0}
`)

func SlidingWindowRateLimit(rc *rds.Client, limit int, window time.Duration) gin.HandlerFunc {
	if limit <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return LeakyBucketRateLimit(rc, "default", float64(limit)/window.Seconds(), limit, int(math.Ceil(window.Seconds())), IPKeyFunc)
}

func LeakyBucketRateLimit(rc *rds.Client, scope string, rate float64, capacity int, ttlSeconds int, keyFn RateLimitKeyFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rc == nil || keyFn == nil || rate <= 0 || capacity <= 0 || ttlSeconds <= 0 {
			c.Next()
			return
		}
		key := keyFn(c)
		if key == "" {
			key = c.ClientIP()
		}

		nowMS := time.Now().UnixMilli()
		redisKey := "ratelimit:" + scope + ":" + key
		out, err := leakyBucketScript.Run(
			c.Request.Context(),
			rc,
			[]string{redisKey},
			nowMS,
			rate/1000.0,
			capacity,
			ttlSeconds,
		).Result()
		if err != nil {
			c.Next()
			return
		}

		res, ok := out.([]any)
		if !ok || len(res) < 2 {
			c.Next()
			return
		}

		allowed := parseInt64Value(res[0])
		if allowed == 1 {
			c.Next()
			return
		}

		retryAfterMS := parseInt64Value(res[1])
		retryAfterSeconds := int(math.Ceil(float64(retryAfterMS) / 1000.0))
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1
		}
		c.Header(constants.HeaderRetryAfter, strconv.Itoa(retryAfterSeconds))
		response.AbortError(c, http.StatusTooManyRequests, apperrors.CodeRateLimited, "Too many requests. Please try again later.", []string{})
	}
}

func IPKeyFunc(c *gin.Context) string {
	return strings.TrimSpace(c.ClientIP())
}

func PhoneRoleKeyFunc(c *gin.Context) string {
	body, ok := parseJSONBody(c)
	if !ok {
		return IPKeyFunc(c)
	}

	phoneVal, _ := body["phone"].(string)
	phone := utils.NormalizeIndianPhone(phoneVal)
	if !utils.ValidateIndianPhone(phone) {
		return IPKeyFunc(c)
	}

	roleVal, _ := body["role"].(string)
	role := normalizeRoleForRateLimit(roleVal)
	if role == "" {
		return phone
	}
	return phone + ":" + role
}

// GuestSessionKeyFunc keys a limit by the guest session established at
// /auth/init-session, for public endpoints that run before any user exists.
func GuestSessionKeyFunc(c *gin.Context) string {
	raw, _ := c.Get(constants.GuestTokenSessionIDKey)
	sid, _ := raw.(string)
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return IPKeyFunc(c)
	}
	return sid
}

func UserIDKeyFunc(c *gin.Context) string {
	raw, _ := c.Get(constants.AuthContextUserIDKey)
	userID, _ := raw.(string)
	return strings.TrimSpace(userID)
}

func PhoneKeyFunc(c *gin.Context) string {
	body, ok := parseJSONBody(c)
	if !ok {
		return IPKeyFunc(c)
	}
	phoneVal, ok := body["phone"].(string)
	if !ok {
		return IPKeyFunc(c)
	}
	phone := utils.NormalizeIndianPhone(phoneVal)
	if !utils.ValidateIndianPhone(phone) {
		return IPKeyFunc(c)
	}
	return phone
}

func parseJSONBody(c *gin.Context) (map[string]any, bool) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, false
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	if len(bodyBytes) == 0 {
		return nil, false
	}
	body := map[string]any{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return nil, false
	}
	return body, true
}

func normalizeRoleForRateLimit(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case constants.RoleClient, constants.RoleRestaurantOwner, constants.RoleRestaurantManager, constants.RoleDriver:
		return role
	default:
		return ""
	}
}

func parseInt64Value(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

func IncrementCounterWithTTL(ctx context.Context, rc *rds.Client, key string, ttl time.Duration) (int64, error) {
	cnt, err := rc.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if cnt == 1 {
		_ = rc.Expire(ctx, key, ttl).Err()
	}
	return cnt, nil
}
