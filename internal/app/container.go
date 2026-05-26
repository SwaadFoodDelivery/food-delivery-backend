package app

import (
	"food-delivery-backend/internal/grpc/client"
	"food-delivery-backend/internal/services/common/otp"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/pkg/config"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
)

type Container struct {
	Config          *config.Config
	Logger          zerolog.Logger
	DB              *sqlx.DB
	Redis           *redis.Client
	KafkaWriter     *kafka.Writer
	OrderClient     *client.OrderServiceClient
	OTPProvider     otp.Provider
	StorageProvider storage.Provider
}
