package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	infraNATS "food-delivery-backend/infra/nats"
	"food-delivery-backend/infra/postgres"
	"food-delivery-backend/infra/redis"
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	grpcclient "food-delivery-backend/internal/grpc/client"
	"food-delivery-backend/internal/router"
	"food-delivery-backend/internal/services/common/email"
	"food-delivery-backend/internal/services/common/otp"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/pkg/config"
	"food-delivery-backend/pkg/logger"

	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	rds "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
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
		dbCtx, dbCancel := context.WithTimeout(startupCtx, constants.StartupDBTimeout)
		defer dbCancel()
		return postgres.NewPostgresDB(dbCtx, cfg)
	}()
	if err != nil {
		startupLog.Fatal().Err(err).Msg("db")
	}

	if err := func() error {
		migrationCtx, migrationCancel := context.WithTimeout(startupCtx, constants.StartupMigrationTimeout)
		defer migrationCancel()
		return postgres.RunMigrations(migrationCtx, db.DB, "./migrations")
	}(); err != nil {
		startupLog.Fatal().Err(err).Msg("migrations")
	}

	rdb, err := func() (*rds.Client, error) {
		redisCtx, redisCancel := context.WithTimeout(startupCtx, constants.StartupRedisTimeout)
		defer redisCancel()
		return redis.NewRedisClient(redisCtx, cfg)
	}()
	if err != nil {
		startupLog.Fatal().Err(err).Msg("redis")
	}

	nc, js, err := func() (*nats.Conn, jetstream.JetStream, error) {
		natsCtx, natsCancel := context.WithTimeout(startupCtx, constants.StartupNATSTimeout)
		defer natsCancel()
		return infraNATS.Connect(natsCtx, cfg)
	}()
	if err != nil {
		startupLog.Fatal().Err(err).Msg("nats")
	}

	natsPublisher := infraNATS.NewPublisher(js)

	oc := initOrderClient(startupCtx, startupLog, cfg)

	var otpProvider otp.Provider
	switch cfg.OTP.Provider {
	case constants.ProviderMock:
		otpProvider = otp.NewMockProvider(log)
	case constants.ProviderDev:
		otpProvider = otp.NewTwilioProvider(cfg.OTP.AccountSID, cfg.OTP.AuthToken, cfg.OTP.FromPhone)
	default:
		startupLog.Fatal().Str("otp_provider", cfg.OTP.Provider).Msg("unsupported OTP_PROVIDER, use mock or dev")
	}

	var emailProvider email.Provider
	switch cfg.Email.Provider {
	case constants.ProviderMock:
		emailProvider = email.NewMockProvider(log)
	default:
		startupLog.Fatal().Str("email_provider", cfg.Email.Provider).Msg("unsupported EMAIL_PROVIDER, use mock")
	}

	var storageProvider storage.Provider
	switch cfg.S3.Provider {
	case constants.ProviderMock:
		storageProvider = storage.NewMockProvider(cfg.S3.MockBaseURL)
	case constants.ProviderDev:
		storageProvider = storage.NewDevProvider(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, cfg.S3.Region, cfg.S3.Endpoint)
	default:
		startupLog.Fatal().Str("s3_provider", cfg.S3.Provider).Msg("unsupported S3_PROVIDER, use mock or dev")
	}

	deps := &app.Container{
		Config:          cfg,
		Logger:          log,
		DB:              db,
		Redis:           rdb,
		NATSConn:        nc,
		NATSPublisher:   natsPublisher,
		OrderClient:     oc,
		OTPProvider:     otpProvider,
		EmailProvider:   emailProvider,
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
	nc.Drain() // flushes pending publishes then closes
	_ = db.Close()
	_ = rdb.Close()
	if oc != nil {
		_ = oc.Close()
	}
}

func initOrderClient(ctx context.Context, log *zerolog.Logger, cfg *config.Config) *grpcclient.OrderServiceClient {
	if !cfg.GRPC.OrderRequired {
		log.Warn().Str("addr", cfg.GRPC.OrderAddr).Msg("grpc order client skipped because ORDER_GRPC_REQUIRED=false")
		return nil
	}
	grpcCtx, grpcCancel := context.WithTimeout(ctx, constants.StartupGRPCTimeout)
	defer grpcCancel()
	oc, err := grpcclient.NewOrderServiceClient(grpcCtx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("grpc")
	}
	return oc
}
