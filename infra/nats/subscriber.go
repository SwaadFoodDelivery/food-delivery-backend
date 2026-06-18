package nats

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

// HandlerFunc is called for each received message. Returning a non-nil error
// causes the message to be nack'd so JetStream will redeliver it.
type HandlerFunc func(ctx context.Context, subject string, data []byte) error

// Subscribe creates a durable push consumer on the given stream+subject and
// dispatches messages to handler. It returns when ctx is cancelled.
func Subscribe(
	ctx context.Context,
	js jetstream.JetStream,
	stream, subject, durableName string,
	handler HandlerFunc,
) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("nats create consumer %s: %w", durableName, err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if err := handler(ctx, msg.Subject(), msg.Data()); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("nats consume %s: %w", durableName, err)
	}
	defer cc.Stop()

	<-ctx.Done()
	return nil
}
