package client

import (
	"food-delivery-backend/pkg/config"
	"testing"
)

func TestRejectInsecureOrderGRPCConfiguration(t *testing.T) {
	valid := func() *config.Config {
		c := &config.Config{}
		c.App.Env = "test"
		c.GRPC.OrderAddr = "127.0.0.1:50051"
		c.GRPC.OrderServiceKey = "fictional-local-service-key-1234567890"
		c.GRPC.OrderTimeoutMS = 2000
		return c
	}
	for name, mutate := range map[string]func(*config.Config){
		"production":       func(c *config.Config) { c.App.Env = "production" },
		"public":           func(c *config.Config) { c.GRPC.OrderAddr = "0.0.0.0:50051" },
		"dns":              func(c *config.Config) { c.GRPC.OrderAddr = "localhost:50051" },
		"missingPort":      func(c *config.Config) { c.GRPC.OrderAddr = "127.0.0.1" },
		"invalidPort":      func(c *config.Config) { c.GRPC.OrderAddr = "127.0.0.1:65536" },
		"shortKey":         func(c *config.Config) { c.GRPC.OrderServiceKey = "short" },
		"unboundedTimeout": func(c *config.Config) { c.GRPC.OrderTimeoutMS = 0 },
		"longTimeout":      func(c *config.Config) { c.GRPC.OrderTimeoutMS = 10001 },
	} {
		t.Run(name, func(t *testing.T) {
			c := valid()
			mutate(c)
			if ValidateOrderGRPCConfig(c) == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
	for _, addr := range []string{"127.0.0.1:50051", "[::1]:50051"} {
		c := valid()
		c.GRPC.OrderAddr = addr
		if err := ValidateOrderGRPCConfig(c); err != nil {
			t.Fatal(err)
		}
	}
}
