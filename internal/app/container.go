package app

import (
	"food-delivery-backend/internal/grpc/client"
	"food-delivery-backend/internal/services/common/email"
	"food-delivery-backend/internal/services/common/otp"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/pkg/config"

	"github.com/jmoiron/sqlx"
	natspub "food-delivery-backend/infra/nats"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Container struct {
	Config          *config.Config
	Logger          zerolog.Logger
	DB              *sqlx.DB
	Redis           *redis.Client
	NATSConn        *nats.Conn
	NATSPublisher   *natspub.Publisher
	OrderClient     *client.OrderServiceClient
	OTPProvider     otp.Provider
	EmailProvider   email.Provider
	StorageProvider storage.Provider
}
