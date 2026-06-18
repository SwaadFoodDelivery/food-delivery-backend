package nats

import (
	"context"
	"fmt"
	"time"

	"food-delivery-backend/pkg/config"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	connectTimeout = 5 * time.Second
	reconnectWait  = 2 * time.Second
	maxReconnects  = -1 // unlimited
)

// Connect establishes a NATS connection with JetStream enabled and returns
// both the core connection and the JetStream context.
func Connect(ctx context.Context, cfg *config.Config) (*nats.Conn, jetstream.JetStream, error) {
	if cfg.NATS.URL == "" {
		return nil, nil, fmt.Errorf("NATS_URL is not configured")
	}

	opts := []nats.Option{
		nats.Name(cfg.App.Name),
		nats.Timeout(connectTimeout),
		nats.ReconnectWait(reconnectWait),
		nats.MaxReconnects(maxReconnects),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				fmt.Printf("nats: disconnected: %v\n", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("nats: reconnected to %s\n", nc.ConnectedUrl())
		}),
	}

	nc, err := nats.Connect(cfg.NATS.URL, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect: %w", err)
	}

	// verify connection is alive before returning
	if err := nc.FlushWithContext(ctx); err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("nats flush: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("nats jetstream: %w", err)
	}

	return nc, js, nil
}
