package port

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

type PhoneStore interface {
	ListActive(ctx context.Context) ([]domain.Phone, error)
	ListAll(ctx context.Context) ([]domain.Phone, error)
	Get(ctx context.Context, serial string) (domain.Phone, error)
	Save(ctx context.Context, phone domain.Phone) error
	Update(ctx context.Context, phone domain.Phone) error
	Delete(ctx context.Context, serial string) error
	LogTransition(ctx context.Context, log domain.PhoneStateLog) error
	Stats(ctx context.Context) (domain.PhoneStats, error)
	Ping(ctx context.Context) error
}

type PhoneLock interface {
	TryLock(ctx context.Context, serial string, ttlSec int) (bool, error)
	Unlock(ctx context.Context, serial string) error
}

type ConnectorClient interface {
	ListDevices(ctx context.Context) ([]domain.Phone, error)
	Connect(ctx context.Context, serial string) error
	GetStatus(ctx context.Context, serial string) (domain.Phone, error)
	EnableWiFi(ctx context.Context, serial, ssid, password string) error
	DisableWiFi(ctx context.Context, serial string) error
	Ping(ctx context.Context) error
}

type ProvisionClient interface {
	AdvanceSetup(ctx context.Context, phone domain.Phone) (domain.PhoneState, error)
	Ping(ctx context.Context) error
}

type EventPublisher interface {
	PublishPhoneStateChanged(ctx context.Context, event domain.PhoneStateEvent) error
	Ping(ctx context.Context) error
}
