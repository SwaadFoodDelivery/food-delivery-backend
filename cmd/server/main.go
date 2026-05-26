package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"food-delivery-backend/infra/kafka"
	"food-delivery-backend/infra/postgres"
	"food-delivery-backend/infra/redis"
	"food-delivery-backend/internal/app"
	grpcclient "food-delivery-backend/internal/grpc/client"
	"food-delivery-backend/internal/router"
	"food-delivery-backend/internal/services/common/otp"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/pkg/config"
	"food-delivery-backend/pkg/logger"

	"github.com/jmoiron/sqlx"
	rds "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	startupDBTimeout        = 45 * time.Second
	startupMigrationTimeout = 60 * time.Second
	startupRedisTimeout     = 20 * time.Second
	startupKafkaTimeout     = 20 * time.Second
	startupGRPCTimeout      = 20 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log, err := logger.New(cfg.App.Env, cfg.App.LogLevel)
	if err != nil {
		panic(err)
	}
	startupCtx := log.WithContext(context.Background())
	startupLog := zerolog.Ctx(startupCtx)
	if startupLog == nil || startupLog.GetLevel() == zerolog.Disabled {
		startupLog = &log
	}

	db, err := func() (*sqlx.DB, error) {
		dbCtx, dbCancel := context.WithTimeout(startupCtx, startupDBTimeout)
		defer dbCancel()
		return postgres.NewPostgresDB(dbCtx, cfg)
	}()
	if err != nil {
		startupLog.Fatal().Err(err).Msg("db")
	}

	if err := func() error {
		migrationCtx, migrationCancel := context.WithTimeout(startupCtx, startupMigrationTimeout)
		defer migrationCancel()
		return postgres.RunMigrations(migrationCtx, db.DB, "./migrations")
	}(); err != nil {
		startupLog.Fatal().Err(err).Msg("migrations")
	}
	rdb, err := func() (*rds.Client, error) {
		redisCtx, redisCancel := context.WithTimeout(startupCtx, startupRedisTimeout)
		defer redisCancel()
		return redis.NewRedisClient(redisCtx, cfg)
	}()
	if err != nil {
		startupLog.Fatal().Err(err).Msg("redis")
	}
	kw, err := func() (*kafkago.Writer, error) {
		kafkaCtx, kafkaCancel := context.WithTimeout(startupCtx, startupKafkaTimeout)
		defer kafkaCancel()
		return kafka.NewProducer(kafkaCtx, cfg)
	}()
	if err != nil {
		startupLog.Fatal().Err(err).Msg("kafka")
	}
	oc, err := func() (*grpcclient.OrderServiceClient, error) {
		grpcCtx, grpcCancel := context.WithTimeout(startupCtx, startupGRPCTimeout)
		defer grpcCancel()
		return grpcclient.NewOrderServiceClient(grpcCtx, cfg)
	}()
	if err != nil {
		startupLog.Fatal().Err(err).Msg("grpc")
	}

	var otpProvider otp.Provider
	switch cfg.OTP.Provider {
	case "mock":
		otpProvider = otp.NewMockProvider(log)
	case "dev":
		otpProvider = otp.NewTwilioProvider(cfg.OTP.AccountSID, cfg.OTP.AuthToken, cfg.OTP.FromPhone)
	default:
		startupLog.Fatal().Str("otp_provider", cfg.OTP.Provider).Msg("unsupported OTP_PROVIDER, use mock or dev")
	}

	var storageProvider storage.Provider
	switch cfg.S3.Provider {
	case "mock":
		storageProvider = storage.NewMockProvider(cfg.S3.MockBaseURL)
	case "dev":
		storageProvider = storage.NewDevProvider(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, cfg.S3.Region, cfg.S3.Endpoint)
	default:
		startupLog.Fatal().Str("s3_provider", cfg.S3.Provider).Msg("unsupported S3_PROVIDER, use mock or dev")
	}

	deps := &app.Container{
		Config:          cfg,
		Logger:          log,
		DB:              db,
		Redis:           rdb,
		KafkaWriter:     kw,
		OrderClient:     oc,
		OTPProvider:     otpProvider,
		StorageProvider: storageProvider,
	}
	eng := router.NewRouter(deps)
	srv := &http.Server{Addr: ":" + cfg.App.Port, Handler: eng}
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	select {
	case err := <-srvErr:
		startupLog.Fatal().Err(err).Str("addr", srv.Addr).Msg("http server start failed")
	case <-quit:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = kw.Close()
	_ = db.Close()
	_ = rdb.Close()
	_ = oc.Close()
}
