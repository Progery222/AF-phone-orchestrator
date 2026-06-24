package driver

import (
	"context"
	"fmt"
	"time"

	connectorv1 "github.com/mobilefarm/af/phone-connector/gen/connector/v1"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// ConnectorGRPC — gRPC-клиент phone-connector (:50052).
type ConnectorGRPC struct {
	client connectorv1.ConnectorServiceClient
	conn   *grpc.ClientConn
}

func NewConnectorGRPC(cfg config.Config) (*ConnectorGRPC, func(), error) {
	conn, err := grpc.NewClient(cfg.ConnectorGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return &ConnectorGRPC{
		client: connectorv1.NewConnectorServiceClient(conn),
		conn:   conn,
	}, func() { _ = conn.Close() }, nil
}

func (c *ConnectorGRPC) Connect(ctx context.Context, serial string) error {
	_, err := c.client.Connect(ctx, &connectorv1.ConnectRequest{Serial: serial})
	return err
}

func (c *ConnectorGRPC) GetStatus(ctx context.Context, serial string) (domain.Phone, error) {
	phone := domain.Phone{Serial: serial, AdbPort: 5555}
	resp, err := c.client.GetStatus(ctx, &connectorv1.GetStatusRequest{Serial: serial})
	if err != nil {
		return phone, err
	}
	if d := resp.GetDevice(); d != nil {
		phone.Model = d.GetModel()
	}
	if snap, err := c.client.GetProvisionStatus(ctx, &connectorv1.GetProvisionStatusRequest{Serial: serial}); err == nil {
		phone.PlatformUserID = snap.GetPlatformUserId()
	}
	return phone, nil
}

func (c *ConnectorGRPC) AdvanceSetup(ctx context.Context, phone *domain.Phone) (domain.PhoneState, error) {
	switch phone.State {
	case domain.StateWifiSetup:
		_, err := c.client.Provision(ctx, &connectorv1.ProvisionRequest{
			IdempotencyKey: fmt.Sprintf("orch-provision-%s", phone.Serial),
			Serial:         phone.Serial,
			Transport:      connectorv1.ConnectTransport_CONNECT_TRANSPORT_USB,
		})
		if err != nil {
			return phone.State, fmt.Errorf("connector provision: %w", err)
		}
		return domain.StateProxySetup, nil
	case domain.StateProxySetup, domain.StateAppsInstall, domain.StateAuth:
		return c.pollProvision(ctx, phone)
	default:
		return phone.State, nil
	}
}

func (c *ConnectorGRPC) pollProvision(ctx context.Context, phone *domain.Phone) (domain.PhoneState, error) {
	snap, err := c.client.GetProvisionStatus(ctx, &connectorv1.GetProvisionStatusRequest{Serial: phone.Serial})
	if err != nil {
		return phone.State, fmt.Errorf("provision status: %w", err)
	}
	switch snap.GetState() {
	case connectorv1.ProvisionState_PROVISION_STATE_READY:
		if uid := snap.GetPlatformUserId(); uid != "" {
			phone.PlatformUserID = uid
		}
		return domain.StateReady, nil
	case connectorv1.ProvisionState_PROVISION_STATE_FAILED,
		connectorv1.ProvisionState_PROVISION_STATE_PARTIAL:
		return phone.State, fmt.Errorf("провижен телефона завершился с ошибкой (%s)", snap.GetState().String())
	default:
		return phone.State, nil
	}
}

func (c *ConnectorGRPC) Ping(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("connector: нет соединения")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		switch c.conn.GetState() {
		case connectivity.Ready, connectivity.Idle:
			return nil
		case connectivity.Shutdown:
			return fmt.Errorf("connector: соединение закрыто")
		default:
			if !c.conn.WaitForStateChange(ctx, c.conn.GetState()) {
				return fmt.Errorf("connector: недоступен")
			}
		}
	}
}

var (
	_ port.ConnectorClient = (*ConnectorGRPC)(nil)
	_ port.ProvisionClient = (*ConnectorGRPC)(nil)
)
