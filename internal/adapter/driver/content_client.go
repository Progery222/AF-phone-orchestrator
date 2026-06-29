package driver

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

// ContentClient — HTTP sync (register/list) + NATS async (download/delete).
type ContentClient struct {
	http *ContentHTTP
	nats *ContentNATS
}

func NewContentClient(http *ContentHTTP, nats *ContentNATS) *ContentClient {
	return &ContentClient{http: http, nats: nats}
}

func (c *ContentClient) Register(ctx context.Context, req port.ContentRegisterRequest) (port.ContentItem, error) {
	return c.http.Register(ctx, req)
}

func (c *ContentClient) DownloadAsync(ctx context.Context, serial, contentID, objectKey string) error {
	if c.nats != nil {
		return c.nats.PublishDownload(ctx, serial, contentID, objectKey)
	}
	_, err := c.http.DownloadSync(ctx, serial, contentID, objectKey)
	return err
}

func (c *ContentClient) DeleteForSerial(ctx context.Context, serial string) error {
	if c.nats == nil {
		return c.http.DeleteForSerialHTTP(ctx, serial)
	}
	return c.nats.PublishDelete(ctx, serial, "")
}

func (c *ContentClient) DeleteDeviceForSerial(ctx context.Context, serial string) error {
	return c.http.DeleteDeviceForSerialHTTP(ctx, serial)
}

func (c *ContentClient) DeleteStorageForSerial(ctx context.Context, serial, extraObjectKey string) error {
	return c.http.DeleteStorageForSerialHTTP(ctx, serial, extraObjectKey)
}

func (c *ContentClient) DeleteByContentID(ctx context.Context, serial, contentID string) error {
	if c.nats == nil {
		return c.http.DeleteByContentIDHTTP(ctx, serial, contentID)
	}
	return c.nats.PublishDelete(ctx, serial, contentID)
}

func (c *ContentClient) ListForSerial(ctx context.Context, serial string) ([]port.ContentItem, error) {
	return c.http.ListForSerial(ctx, serial)
}

func (c *ContentClient) Ping(ctx context.Context) error {
	if err := c.http.Ping(ctx); err != nil {
		return err
	}
	if c.nats != nil {
		return c.nats.Ping(ctx)
	}
	return nil
}

var _ port.ContentClient = (*ContentClient)(nil)

type StubContent struct{}

func NewStubContent() *StubContent { return &StubContent{} }

func (s *StubContent) Register(_ context.Context, req port.ContentRegisterRequest) (port.ContentItem, error) {
	return port.ContentItem{
		ContentID: "stub-content",
		Serial:    req.Serial,
		ObjectKey: req.ObjectKey,
		Status:    "queued",
	}, nil
}

func (s *StubContent) DownloadAsync(context.Context, string, string, string) error { return nil }

func (s *StubContent) DeleteForSerial(context.Context, string) error { return nil }

func (s *StubContent) DeleteDeviceForSerial(context.Context, string) error { return nil }

func (s *StubContent) DeleteStorageForSerial(context.Context, string, string) error { return nil }

func (s *StubContent) DeleteByContentID(context.Context, string, string) error { return nil }

func (s *StubContent) ListForSerial(_ context.Context, serial string) ([]port.ContentItem, error) {
	return []port.ContentItem{}, nil
}

func (s *StubContent) Ping(context.Context) error { return nil }

var _ port.ContentClient = (*StubContent)(nil)
