package repository

import (
	"context"
	"encoding/json"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/nats-io/nats.go"
)

type NATSEventPublisher struct {
	conn    *nats.Conn
	subject string
}

func NewNATSEventPublisher(cfg config.Config) (*NATSEventPublisher, func(), error) {
	conn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, nil, err
	}
	return &NATSEventPublisher{conn: conn, subject: cfg.NATSSubjectStateChanged}, func() { conn.Close() }, nil
}

func (p *NATSEventPublisher) PublishPhoneStateChanged(_ context.Context, event domain.PhoneStateEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.conn.Publish(p.subject, data)
}

func (p *NATSEventPublisher) Ping(context.Context) error {
	if !p.conn.IsConnected() {
		return nats.ErrConnectionClosed
	}
	return nil
}

type NoopEventPublisher struct{}

func NewNoopEventPublisher() *NoopEventPublisher { return &NoopEventPublisher{} }

func (n *NoopEventPublisher) PublishPhoneStateChanged(context.Context, domain.PhoneStateEvent) error {
	return nil
}
func (n *NoopEventPublisher) Ping(context.Context) error { return nil }
