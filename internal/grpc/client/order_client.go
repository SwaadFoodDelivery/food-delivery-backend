package client

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"food-delivery-backend/pkg/config"
	orderpb "github.com/SwaadFoodDelivery/proto/order"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type OrderServiceClient struct {
	conn    *grpc.ClientConn
	rpc     orderpb.OrderServiceClient
	key     string
	timeout time.Duration
}

func ValidateOrderGRPCConfig(cfg *config.Config) error {
	if cfg.App.Env != "development" && cfg.App.Env != "test" {
		return fmt.Errorf("order gRPC plaintext transport is development/test only; TLS is not implemented")
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(cfg.GRPC.OrderAddr))
	ip := net.ParseIP(host)
	n, portErr := strconv.Atoi(port)
	if err != nil || ip == nil || !ip.IsLoopback() || portErr != nil || n < 1 || n > 65535 {
		return fmt.Errorf("ORDER_GRPC_ADDR requires a numeric loopback IP and valid port")
	}
	if len(strings.TrimSpace(cfg.GRPC.OrderServiceKey)) < 32 {
		return fmt.Errorf("ORDER_GRPC_SERVICE_KEY must contain at least 32 characters")
	}
	if cfg.GRPC.OrderTimeoutMS < 100 || cfg.GRPC.OrderTimeoutMS > 10000 {
		return fmt.Errorf("ORDER_GRPC_TIMEOUT_MS must be between 100 and 10000")
	}
	return nil
}

func NewOrderServiceClient(ctx context.Context, cfg *config.Config) (*OrderServiceClient, error) {
	if err := ValidateOrderGRPCConfig(cfg); err != nil {
		return nil, err
	}
	addr := strings.TrimSpace(cfg.GRPC.OrderAddr)
	timeout := time.Duration(cfg.GRPC.OrderTimeoutMS) * time.Millisecond
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	return &OrderServiceClient{conn: conn, rpc: orderpb.NewOrderServiceClient(conn), key: cfg.GRPC.OrderServiceKey, timeout: timeout}, nil
}

func (c *OrderServiceClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Only real typed reads are exposed. The former echo-success placeholders are
// removed; checkout and all mutations remain backend-owned during extraction.
func (c *OrderServiceClient) GetOrder(ctx context.Context, in *orderpb.GetOrderRequest) (*orderpb.OrderResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set("x-order-service-key", c.key)
	return c.rpc.GetOrder(metadata.NewOutgoingContext(ctx, md), in)
}
