package kafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	"food-delivery-backend/pkg/config"

	"github.com/segmentio/kafka-go"
)

func NewProducer(ctx context.Context, cfg *config.Config) (*kafka.Writer, error) {
	brokers := make([]string, 0, len(cfg.Kafka.Brokers))
	for _, b := range cfg.Kafka.Brokers {
		trimmed := strings.TrimSpace(b)
		if trimmed != "" {
			brokers = append(brokers, trimmed)
		}
	}
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are not configured")
	}

	dialer := &kafka.Dialer{Timeout: 5 * time.Second}
	var lastErr error
	reachable := false
	for _, broker := range brokers {
		conn, err := dialer.DialContext(ctx, "tcp", broker)
		if err != nil {
			lastErr = err
			continue
		}
		_ = conn.Close()
		reachable = true
		break
	}
	if !reachable {
		return nil, fmt.Errorf("kafka bootstrap failed: %w", lastErr)
	}

	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        cfg.Kafka.OrderEventsTopic,
		RequiredAcks: kafka.RequiredAcks(cfg.Kafka.RequiredAcks),
		BatchSize:    cfg.Kafka.BatchSize,
		Balancer:     &kafka.LeastBytes{},
	}, nil
}
