package apibridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type JetStreamBroker struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

func NewJetStreamBroker(url string) (*JetStreamBroker, error) {
	nc, err := nats.Connect(url, nats.Timeout(5*time.Second), nats.MaxReconnects(-1))
	if err != nil {
		return nil, fmt.Errorf("ERR_NATS_CONNECT: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("ERR_JETSTREAM_INIT: %w", err)
	}

	return &JetStreamBroker{nc: nc, js: js}, nil
}

func (b *JetStreamBroker) Publish(ctx context.Context, subject, idempotencyKey string, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("ERR_BROKER_MARSHAL: %w", err)
	}

	_, err = b.js.Publish(subject, payload, nats.MsgId(idempotencyKey), nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("ERR_JETSTREAM_PUBLISH: %w", err)
	}

	return nil
}

func (b *JetStreamBroker) Close() {
	if b.nc != nil {
		b.nc.Close()
	}
}
