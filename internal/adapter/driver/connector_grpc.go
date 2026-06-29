package driver

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	connectorv1 "github.com/mobilefarm/af/phone-connector/gen/connector/v1"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

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

func (c *ConnectorGRPC) ListDevices(ctx context.Context) ([]domain.Phone, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := c.client.ListDevices(ctx, &connectorv1.ListDevicesRequest{})
	if err != nil {
		return nil, err
	}

	phones := make([]domain.Phone, 0, len(resp.GetDevices()))
	for _, dev := range resp.GetDevices() {
		serial := strings.TrimSpace(dev.GetSerial())
		if serial == "" {
			continue
		}
		phone := domain.Phone{
			Serial:  serial,
			State:   domain.StateWorking,
			Model:   dev.GetModel(),
			AdbPort: 5555,
		}
		if ip, port := splitADBTarget(serial); ip != "" {
			phone.CurrentIP = ip
			phone.AdbPort = port
		}
		phones = append(phones, phone)
	}
	return phones, nil
}

func (c *ConnectorGRPC) Connect(ctx context.Context, serial string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.client.Connect(ctx, &connectorv1.ConnectRequest{Serial: serial})
	if err != nil {
		return err
	}
	if !resp.GetSuccess() {
		return fmt.Errorf("connector Connect: %s", resp.GetMessage())
	}
	return nil
}

func (c *ConnectorGRPC) GetStatus(ctx context.Context, serial string) (domain.Phone, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := c.client.GetStatus(ctx, &connectorv1.GetStatusRequest{Serial: serial})
	if err != nil {
		return domain.Phone{}, err
	}
	dev := resp.GetDevice()
	return domain.Phone{
		Serial: dev.GetSerial(),
		Model:  dev.GetModel(),
		State:  domain.PhoneState(dev.GetState()),
	}, nil
}

func (c *ConnectorGRPC) EnableWiFi(ctx context.Context, serial, ssid, password string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	resp, err := c.client.ControlWifi(ctx, &connectorv1.ControlWifiRequest{
		Serial: serial,
		Action: connectorv1.ControlWifiAction_CONTROL_WIFI_ACTION_ENABLE,
		Ssid:   ssid,
		Psk:    password,
	})
	if err != nil {
		return err
	}
	if !resp.GetSuccess() {
		return fmt.Errorf("connector ControlWifi enable: неуспех")
	}
	return nil
}

func (c *ConnectorGRPC) DisableWiFi(ctx context.Context, serial string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.client.ControlWifi(ctx, &connectorv1.ControlWifiRequest{
		Serial: serial,
		Action: connectorv1.ControlWifiAction_CONTROL_WIFI_ACTION_DISABLE,
	})
	if err != nil {
		return err
	}
	if !resp.GetSuccess() {
		return fmt.Errorf("connector ControlWifi disable: неуспех")
	}
	return nil
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

func splitADBTarget(serial string) (string, int) {
	idx := strings.LastIndex(serial, ":")
	if idx <= 0 || idx == len(serial)-1 {
		return "", 0
	}
	port, err := strconv.Atoi(serial[idx+1:])
	if err != nil || port <= 0 {
		return "", 0
	}
	return serial[:idx], port
}

var _ port.ConnectorClient = (*ConnectorGRPC)(nil)
