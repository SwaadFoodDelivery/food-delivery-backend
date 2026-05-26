package redis

import (
	"context"
	"time"

	"food-delivery-backend/pkg/config"

	rds "github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, cfg *config.Config) (*rds.Client, error) {
	c := rds.NewClient(&rds.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB, PoolSize: cfg.Redis.PoolSize, ReadTimeout: time.Duration(cfg.Redis.ReadTimeoutMS) * time.Millisecond})
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}
