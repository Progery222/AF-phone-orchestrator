package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

// ProvisionHTTP — клиент phone-provisioner (REST :19092).
type ProvisionHTTP struct {
	baseURL       string
	client        *http.Client
	cfg           config.Config
	awaitingReady sync.Map // serial → struct{}; true после POST /provision до получения ready
}

func NewProvisionHTTP(cfg config.Config) *ProvisionHTTP {
	return &ProvisionHTTP{
		baseURL: strings.TrimRight(cfg.ProvisionerHTTPURL, "/"),
		client:  &http.Client{Timeout: 120 * time.Second},
		cfg:     cfg,
	}
}

type provisionStatus struct {
	Serial string `json:"serial"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (c *ProvisionHTTP) AdvanceSetup(ctx context.Context, phone domain.Phone) (domain.PhoneState, error) {
	switch phone.State {
	case domain.StateWifiSetup, domain.StateProxySetup, domain.StateAppsInstall, domain.StateAuth:
	default:
		return phone.State, nil
	}

	run, err := c.getStatus(ctx, phone.Serial)
	if err != nil && !errors.Is(err, errProvisionerNotFound) {
		return phone.State, err
	}

	if c.shouldStartProvision(phone, run) {
		if err := c.startProvision(ctx, phone); err != nil {
			if !errors.Is(err, errProvisionerConflict) {
				return phone.State, err
			}
		}
		run, err = c.getStatus(ctx, phone.Serial)
		if err != nil {
			return phone.State, err
		}
	}

	switch run.Status {
	case "ready":
		if _, waiting := c.awaitingReady.Load(phone.Serial); waiting {
			c.awaitingReady.Delete(phone.Serial)
			return domain.StateReady, nil
		}
		return phone.State, nil
	case "failed":
		msg := run.Error
		if msg == "" {
			msg = "настройка завершилась с ошибкой"
		}
		return phone.State, fmt.Errorf("provisioner: %s", msg)
	case "provisioning", "pending":
		return phone.State, nil
	default:
		return phone.State, nil
	}
}

func (c *ProvisionHTTP) shouldStartProvision(phone domain.Phone, run *provisionStatus) bool {
	if run == nil {
		return true
	}
	switch run.Status {
	case "provisioning", "pending":
		return false
	case "failed":
		return true
	case "ready":
		if _, waiting := c.awaitingReady.Load(phone.Serial); waiting {
			return false
		}
		return phone.ReadyAt == nil && phone.State == domain.StateWifiSetup
	default:
		return true
	}
}

func (c *ProvisionHTTP) startProvision(ctx context.Context, phone domain.Phone) error {
	body := c.buildRequest(phone)
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/provision", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return errProvisionerConflict
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("provisioner POST /provision HTTP %d: %s", resp.StatusCode, string(b))
	}
	c.awaitingReady.Store(phone.Serial, struct{}{})
	return nil
}

func (c *ProvisionHTTP) buildRequest(phone domain.Phone) map[string]any {
	proxyIP := phone.ProxyIP
	if proxyIP == "" {
		proxyIP = c.cfg.ProvisionerDefaultProxyIP
	}
	proxyPort := phone.ProxyPort
	if proxyPort == 0 {
		proxyPort = c.cfg.ProvisionerDefaultProxyPort
	}
	proxyUser := phone.ProxyUser
	if proxyUser == "" {
		proxyUser = c.cfg.ProvisionerDefaultProxyUser
	}
	proxyPass := phone.ProxyPass
	if proxyPass == "" {
		proxyPass = c.cfg.ProvisionerDefaultProxyPass
	}
	wifiSSID := phone.WifiSSID
	if wifiSSID == "" {
		wifiSSID = c.cfg.ProvisionerDefaultWiFiSSID
	}
	wifiPass := phone.WiFiPass
	if wifiPass == "" {
		wifiPass = c.cfg.ProvisionerDefaultWiFiPass
	}

	apps := phone.ProvisionApps
	if len(apps) == 0 && c.cfg.ProvisionerDefaultAppsJSON != "" {
		_ = json.Unmarshal([]byte(c.cfg.ProvisionerDefaultAppsJSON), &apps)
	}

	appDTOs := make([]map[string]string, 0, len(apps))
	for _, a := range apps {
		appDTOs = append(appDTOs, map[string]string{
			"name": a.Name,
			"type": a.Type,
			"url":  a.URL,
		})
	}

	return map[string]any{
		"serial": phone.Serial,
		"proxy": map[string]any{
			"ip":       proxyIP,
			"port":     proxyPort,
			"username": proxyUser,
			"password": proxyPass,
		},
		"apps":          appDTOs,
		"wifi_ssid":     wifiSSID,
		"wifi_password": wifiPass,
	}
}

func (c *ProvisionHTTP) getStatus(ctx context.Context, serial string) (*provisionStatus, error) {
	url := fmt.Sprintf("%s/status?serial=%s", c.baseURL, serial)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errProvisionerNotFound
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provisioner GET /status HTTP %d: %s", resp.StatusCode, string(b))
	}
	var run provisionStatus
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *ProvisionHTTP) Ping(ctx context.Context) error {
	healthBase := strings.TrimRight(c.cfg.ProvisionerHealthURL, "/")
	if healthBase == "" {
		healthBase = c.baseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthBase+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("provisioner health %d", resp.StatusCode)
	}
	return nil
}

var (
	errProvisionerNotFound = errors.New("provisioner: прогон не найден")
	errProvisionerConflict = errors.New("provisioner: настройка уже выполняется")
)

var _ port.ProvisionClient = (*ProvisionHTTP)(nil)
