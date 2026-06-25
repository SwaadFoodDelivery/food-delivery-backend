package config

import (
	"strings"
	"time"

	"food-delivery-backend/internal/constants"

	"github.com/spf13/viper"
)

type Config struct {
	App struct {
		Name     string
		Env      string
		Port     string
		LogLevel string
	}
	Postgres struct {
		Host, Port, User, Password, DBName, SSLMode    string
		MaxOpenConns, MaxIdleConns, ConnMaxLifetimeMin int
	}
	Redis struct {
		Addr, Password              string
		DB, PoolSize, ReadTimeoutMS int
	}
	NATS struct {
		URL      string
		ClientID string
	}
	JWT struct {
		Secret string
	}
	OTP struct {
		Provider   string
		AccountSID string
		AuthToken  string
		FromPhone  string
	}
	Email struct {
		Provider         string
		SendGridKey      string
		SendGridEndpoint string
		FromAddress      string
		FromName         string
	}
	S3 struct {
		Provider          string
		Bucket            string
		DefaultBucket     string
		Buckets           map[string]string
		Region            string
		Endpoint          string
		AccessKeyID       string
		SecretAccessKey   string
		PresignTTLSeconds int
		MockBaseURL       string
	}
	GRPC struct {
		OrderAddr     string
		OrderRequired bool
	}
	RateLimit struct {
		DefaultPerMin int
		WindowSec     int
	}
}

func (c Config) RateWindow() time.Duration { return time.Duration(c.RateLimit.WindowSec) * time.Second }

func Load() (*Config, error) {
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		viper.SetConfigFile(".env.development")
		_ = viper.ReadInConfig()
	}
	cfg := &Config{}
	cfg.App.Name = viper.GetString("APP_NAME")
	cfg.App.Env = viper.GetString("APP_ENV")
	cfg.App.Port = viper.GetString("APP_PORT")
	cfg.App.LogLevel = viper.GetString("LOG_LEVEL")
	cfg.Postgres.Host = viper.GetString("POSTGRES_HOST")
	cfg.Postgres.Port = viper.GetString("POSTGRES_PORT")
	cfg.Postgres.User = viper.GetString("POSTGRES_USER")
	cfg.Postgres.Password = viper.GetString("POSTGRES_PASSWORD")
	cfg.Postgres.DBName = viper.GetString("POSTGRES_DB")
	cfg.Postgres.SSLMode = viper.GetString("POSTGRES_SSLMODE")
	cfg.Postgres.MaxOpenConns = viper.GetInt("POSTGRES_MAX_OPEN_CONNS")
	cfg.Postgres.MaxIdleConns = viper.GetInt("POSTGRES_MAX_IDLE_CONNS")
	cfg.Postgres.ConnMaxLifetimeMin = viper.GetInt("POSTGRES_CONN_MAX_LIFETIME_MIN")
	cfg.Redis.Addr = viper.GetString("REDIS_ADDR")
	cfg.Redis.Password = viper.GetString("REDIS_PASSWORD")
	cfg.Redis.DB = viper.GetInt("REDIS_DB")
	cfg.Redis.PoolSize = viper.GetInt("REDIS_POOL_SIZE")
	cfg.Redis.ReadTimeoutMS = viper.GetInt("REDIS_READ_TIMEOUT_MS")
	cfg.NATS.URL = viper.GetString("NATS_URL")
	cfg.NATS.ClientID = viper.GetString("NATS_CLIENT_ID")
	cfg.JWT.Secret = viper.GetString("JWT_SECRET")
	cfg.OTP.Provider = strings.ToLower(strings.TrimSpace(viper.GetString("OTP_PROVIDER")))
	cfg.OTP.AccountSID = viper.GetString("TWILIO_ACCOUNT_SID")
	cfg.OTP.AuthToken = viper.GetString("TWILIO_AUTH_TOKEN")
	cfg.OTP.FromPhone = viper.GetString("TWILIO_FROM_PHONE")
	cfg.Email.Provider = strings.ToLower(strings.TrimSpace(viper.GetString("EMAIL_PROVIDER")))
	cfg.Email.SendGridKey = strings.TrimSpace(viper.GetString("SENDGRID_API_KEY"))
	cfg.Email.SendGridEndpoint = strings.TrimSpace(viper.GetString("SENDGRID_ENDPOINT"))
	cfg.Email.FromAddress = strings.TrimSpace(viper.GetString("EMAIL_FROM_ADDRESS"))
	cfg.Email.FromName = strings.TrimSpace(viper.GetString("EMAIL_FROM_NAME"))
	cfg.S3.Provider = strings.ToLower(strings.TrimSpace(viper.GetString("S3_PROVIDER")))
	cfg.S3.Bucket = strings.TrimSpace(viper.GetString("S3_BUCKET"))
	cfg.S3.Buckets = parseS3Buckets(viper.GetString("S3_BUCKETS"))
	cfg.S3.DefaultBucket = firstNonEmpty(viper.GetString("S3_DEFAULT_BUCKET"), cfg.S3.Buckets[constants.S3BucketPurposeDefault], cfg.S3.Bucket)
	cfg.S3.Buckets[constants.S3BucketPurposeDefault] = cfg.S3.DefaultBucket
	cfg.S3.Buckets[constants.S3BucketPurposeOnboarding] = firstNonEmpty(viper.GetString("S3_ONBOARDING_BUCKET"), cfg.S3.Buckets[constants.S3BucketPurposeOnboarding], cfg.S3.DefaultBucket)
	cfg.S3.Region = strings.TrimSpace(viper.GetString("S3_REGION"))
	cfg.S3.Endpoint = strings.TrimSpace(viper.GetString("S3_ENDPOINT"))
	cfg.S3.AccessKeyID = strings.TrimSpace(viper.GetString("S3_ACCESS_KEY_ID"))
	cfg.S3.SecretAccessKey = strings.TrimSpace(viper.GetString("S3_SECRET_ACCESS_KEY"))
	cfg.S3.PresignTTLSeconds = viper.GetInt("S3_PRESIGN_TTL_SECONDS")
	cfg.S3.MockBaseURL = strings.TrimSpace(viper.GetString("S3_MOCK_BASE_URL"))
	cfg.GRPC.OrderAddr = viper.GetString("ORDER_GRPC_ADDR")
	cfg.GRPC.OrderRequired = viper.GetBool("ORDER_GRPC_REQUIRED")
	cfg.RateLimit.DefaultPerMin = viper.GetInt("RATE_LIMIT_DEFAULT_PER_MIN")
	cfg.RateLimit.WindowSec = viper.GetInt("RATE_LIMIT_WINDOW_SEC")
	if cfg.App.Port == "" {
		cfg.App.Port = constants.DefaultAppPort
	}
	if cfg.GRPC.OrderAddr == "" {
		cfg.GRPC.OrderAddr = constants.DefaultOrderGRPCAddr
	}
	if cfg.OTP.Provider == "" {
		cfg.OTP.Provider = constants.ProviderMock
	}
	if cfg.Email.Provider == "" {
		cfg.Email.Provider = constants.ProviderMock
	}
	if cfg.Email.SendGridEndpoint == "" {
		cfg.Email.SendGridEndpoint = constants.DefaultSendGridEndpoint
	}
	if cfg.S3.Provider == "" {
		cfg.S3.Provider = constants.ProviderMock
	}
	if cfg.S3.PresignTTLSeconds == 0 {
		cfg.S3.PresignTTLSeconds = constants.DefaultS3PresignTTLSeconds
	}
	if cfg.S3.Region == "" {
		cfg.S3.Region = constants.DefaultS3Region
	}
	if cfg.RateLimit.DefaultPerMin == 0 {
		cfg.RateLimit.DefaultPerMin = constants.DefaultRateLimitPerMinute
	}
	if cfg.RateLimit.WindowSec == 0 {
		cfg.RateLimit.WindowSec = constants.DefaultRateLimitWindowSec
	}
	return cfg, nil
}

func (c Config) S3BucketFor(purpose string) string {
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	if bucket := strings.TrimSpace(c.S3.Buckets[purpose]); bucket != "" {
		return bucket
	}
	if bucket := strings.TrimSpace(c.S3.DefaultBucket); bucket != "" {
		return bucket
	}
	return strings.TrimSpace(c.S3.Bucket)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseS3Buckets(raw string) map[string]string {
	buckets := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(item, ":")
		if !ok {
			key, value, ok = strings.Cut(item, "=")
		}
		if !ok {
			continue
		}
		purpose := strings.ToLower(strings.TrimSpace(key))
		bucket := strings.TrimSpace(value)
		if purpose != "" && bucket != "" {
			buckets[purpose] = bucket
		}
	}
	return buckets
}
