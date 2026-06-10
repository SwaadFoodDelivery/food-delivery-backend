package client

import (
	"context"
	"fmt"
	"strings"

	"food-delivery-backend/pkg/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OrderServiceClient struct {
	conn *grpc.ClientConn
}

func NewOrderServiceClient(ctx context.Context, cfg *config.Config) (*OrderServiceClient, error) {
	addr := strings.TrimSpace(cfg.GRPC.OrderAddr)
	if addr == "" {
		return nil, fmt.Errorf("ORDER_GRPC_ADDR is required")
	}

	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	return &OrderServiceClient{conn: conn}, nil
}

func (c *OrderServiceClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
func (c *OrderServiceClient) PlaceOrder(ctx context.Context, in any) (any, error) {
	_ = ctx
	return in, nil
}
func (c *OrderServiceClient) GetOrder(ctx context.Context, in any) (any, error) {
	_ = ctx
	return in, nil
}
func (c *OrderServiceClient) CancelOrder(ctx context.Context, in any) (any, error) {
	_ = ctx
	return in, nil
}
func (c *OrderServiceClient) UpdateOrderStatus(ctx context.Context, in any) (any, error) {
	_ = ctx
	return in, nil
}
func (c *OrderServiceClient) GetOrderTracking(ctx context.Context, in any) (any, error) {
	_ = ctx
	return in, nil
}
func (c *OrderServiceClient) GetUserOrders(ctx context.Context, in any) (any, error) {
	_ = ctx
	return in, nil
}
