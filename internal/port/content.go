package port

import "context"

type ContentItem struct {
	ContentID  string `json:"content_id"`
	Serial     string `json:"serial"`
	ObjectKey  string `json:"object_key,omitempty"`
	DevicePath string `json:"device_path,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Status     string `json:"status"`
}

// ContentRegisterRequest — файл уже в MinIO (загрузил другой сервис / pipeline).
type ContentRegisterRequest struct {
	Serial    string `json:"serial"`
	ObjectKey string `json:"object_key"`
	Filename  string `json:"filename"`
	MediaType string `json:"media_type"`
}

type ContentClient interface {
	Register(ctx context.Context, req ContentRegisterRequest) (ContentItem, error)
	DownloadAsync(ctx context.Context, serial, contentID, objectKey string) error
	DeleteForSerial(ctx context.Context, serial string) error
	DeleteDeviceForSerial(ctx context.Context, serial string) error
	DeleteStorageForSerial(ctx context.Context, serial, extraObjectKey string) error
	DeleteByContentID(ctx context.Context, serial, contentID string) error
	ListForSerial(ctx context.Context, serial string) ([]ContentItem, error)
	Ping(ctx context.Context) error
}
