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

	db, err := postgres.NewPostgresDB(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("db")
	}
	if err := postgres.RunMigrations(db.DB, "./migrations"); err != nil {
		log.Fatal().Err(err).Msg("migrations")
	}
	rdb, err := redis.NewRedisClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("redis")
	}
	kw, err := kafka.NewProducer(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("kafka")
	}
	oc, err := grpcclient.NewOrderServiceClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("grpc")
	}

	var otpProvider otp.Provider
	switch cfg.OTP.Provider {
	case "mock":
		otpProvider = otp.NewMockProvider(log)
	case "dev":
		otpProvider = otp.NewTwilioProvider(cfg.OTP.AccountSID, cfg.OTP.AuthToken, cfg.OTP.FromPhone)
	default:
		log.Fatal().Str("otp_provider", cfg.OTP.Provider).Msg("unsupported OTP_PROVIDER, use mock or dev")
	}

	var storageProvider storage.Provider
	switch cfg.S3.Provider {
	case "mock":
		storageProvider = storage.NewMockProvider(cfg.S3.MockBaseURL)
	case "dev":
		storageProvider = storage.NewDevProvider(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, cfg.S3.Region, cfg.S3.Endpoint)
	default:
		log.Fatal().Str("s3_provider", cfg.S3.Provider).Msg("unsupported S3_PROVIDER, use mock or dev")
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
	select {
	case err := <-srvErr:
		log.Fatal().Err(err).Str("addr", srv.Addr).Msg("http server start failed")
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
