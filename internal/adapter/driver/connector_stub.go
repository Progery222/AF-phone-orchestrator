package driver

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

// StubConnector — заглушка phone-connector (gRPC :50052) до интеграции.
type StubConnector struct{}

func NewStubConnector() *StubConnector { return &StubConnector{} }

func (s *StubConnector) Connect(_ context.Context, serial string) error {
	_ = serial
	return nil
}

func (s *StubConnector) GetStatus(_ context.Context, serial string) (domain.Phone, error) {
	return domain.Phone{Serial: serial, State: domain.StateWorking, AdbPort: 5555}, nil
}

func (s *StubConnector) Ping(context.Context) error { return nil }

var _ port.ConnectorClient = (*StubConnector)(nil)

// StubProvisioner — заглушка Provisioner: переводит на следующий этап настройки.
type StubProvisioner struct{}

func NewStubProvisioner() *StubProvisioner { return &StubProvisioner{} }

func (s *StubProvisioner) AdvanceSetup(_ context.Context, phone *domain.Phone) (domain.PhoneState, error) {
	switch phone.State {
	case domain.StateWifiSetup:
		return domain.StateProxySetup, nil
	case domain.StateProxySetup:
		return domain.StateAppsInstall, nil
	case domain.StateAppsInstall:
		return domain.StateAuth, nil
	case domain.StateAuth:
		return domain.StateReady, nil
	default:
		return phone.State, nil
	}
}

func (s *StubProvisioner) Ping(context.Context) error { return nil }

var _ port.ProvisionClient = (*StubProvisioner)(nil)
