package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type ObserverHTTP struct {
	baseURL string
	client  *http.Client
}

func NewObserverHTTP(cfg config.Config) *ObserverHTTP {
	return &ObserverHTTP{
		baseURL: strings.TrimRight(cfg.ObserverHTTPURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *ObserverHTTP) CaptureScreen(ctx context.Context, serial string) (domain.ScreenCapture, error) {
	url := fmt.Sprintf("%s/screen/%s?timeout_sec=30", o.baseURL, serial)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.ScreenCapture{}, err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return domain.ScreenCapture{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return domain.ScreenCapture{}, fmt.Errorf("observer screen HTTP %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Serial        string `json:"serial"`
		MinioKey      string `json:"minio_key"`
		ScreenshotURL string `json:"screenshot_url"`
		Resolution    struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"resolution"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.ScreenCapture{}, err
	}
	return domain.ScreenCapture{
		Serial:        serial,
		MinioKey:      parsed.MinioKey,
		ScreenshotURL: parsed.ScreenshotURL,
		Width:         parsed.Resolution.Width,
		Height:        parsed.Resolution.Height,
	}, nil
}

func (o *ObserverHTTP) DumpUI(ctx context.Context, serial string) (domain.UIDump, error) {
	url := fmt.Sprintf("%s/ui/%s?format=xml&timeout_sec=30", o.baseURL, serial)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.UIDump{}, err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return domain.UIDump{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return domain.UIDump{}, fmt.Errorf("observer ui HTTP %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Serial      string `json:"serial"`
		XMLDump     string `json:"xml_dump"`
		PackageName string `json:"package_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.UIDump{}, err
	}
	return domain.UIDump{
		Serial:  serial,
		XMLDump: parsed.XMLDump,
		Package: parsed.PackageName,
	}, nil
}

func (o *ObserverHTTP) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("observer health %d", resp.StatusCode)
	}
	return nil
}

var _ port.ObserverClient = (*ObserverHTTP)(nil)
