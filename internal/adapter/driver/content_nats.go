package driver

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
)

type ContentNATS struct {
	conn           *nats.Conn
	downloadSubj   string
	deleteSubj     string
}

func NewContentNATS(cfg config.Config) (*ContentNATS, func(), error) {
	conn, err := nats.Connect(cfg.NATSURL,
		nats.Name("orchestrator-content"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, nil, err
	}
	return &ContentNATS{
		conn:         conn,
		downloadSubj: cfg.NATSSubjectContentDownload,
		deleteSubj:   cfg.NATSSubjectContentDelete,
	}, func() { _ = conn.Drain() }, nil
}

func (c *ContentNATS) PublishDownload(ctx context.Context, serial, contentID, objectKey string) error {
	msg := map[string]string{"serial": serial}
	if contentID != "" {
		msg["content_id"] = contentID
	}
	if objectKey != "" {
		msg["object_key"] = objectKey
	}
	return c.publish(ctx, c.downloadSubj, msg)
}

func (c *ContentNATS) PublishDelete(ctx context.Context, serial, contentID string) error {
	msg := map[string]string{"serial": serial}
	if contentID != "" {
		msg["content_id"] = contentID
	}
	return c.publish(ctx, c.deleteSubj, msg)
}

func (c *ContentNATS) publish(_ context.Context, subject string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.conn.Publish(subject, b)
}

func (c *ContentNATS) Ping(context.Context) error {
	if c.conn == nil || !c.conn.IsConnected() {
		return nats.ErrConnectionClosed
	}
	return nil
}
