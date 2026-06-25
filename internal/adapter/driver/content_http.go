package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type ContentHTTP struct {
	baseURL string
	client  *http.Client
}

func NewContentHTTP(cfg config.Config) *ContentHTTP {
	return &ContentHTTP{
		baseURL: strings.TrimRight(cfg.ContentDistributorHTTPURL, "/"),
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *ContentHTTP) Register(ctx context.Context, req port.ContentRegisterRequest) (port.ContentItem, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return port.ContentItem{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/content/register", bytes.NewReader(payload))
	if err != nil {
		return port.ContentItem{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return port.ContentItem{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return port.ContentItem{}, fmt.Errorf("content-distributor POST /content/register HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		ContentID string `json:"content_id"`
		Serial    string `json:"serial"`
		ObjectKey string `json:"object_key"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return port.ContentItem{}, err
	}
	return port.ContentItem{
		ContentID: out.ContentID,
		Serial:    out.Serial,
		ObjectKey: out.ObjectKey,
		Status:    out.Status,
	}, nil
}

func (c *ContentHTTP) DownloadSync(ctx context.Context, serial, contentID, objectKey string) (port.ContentItem, error) {
	body, _ := json.Marshal(map[string]string{
		"serial": serial, "content_id": contentID, "object_key": objectKey,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/content/download", bytes.NewReader(body))
	if err != nil {
		return port.ContentItem{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return port.ContentItem{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return port.ContentItem{}, fmt.Errorf("content-distributor POST /content/download HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Serial     string `json:"serial"`
		ContentID  string `json:"content_id"`
		DevicePath string `json:"device_path"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return port.ContentItem{}, err
	}
	return port.ContentItem{
		ContentID:  out.ContentID,
		Serial:     out.Serial,
		DevicePath: out.DevicePath,
		Status:     out.Status,
	}, nil
}

func (c *ContentHTTP) DeleteForSerialHTTP(ctx context.Context, serial string) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/content/"+serial, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("content-distributor DELETE /content/%s HTTP %d: %s", serial, resp.StatusCode, string(b))
	}
	return nil
}

func (c *ContentHTTP) DeleteByContentIDHTTP(ctx context.Context, serial, contentID string) error {
	url := fmt.Sprintf("%s/content/%s/%s", c.baseURL, serial, contentID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("content-distributor DELETE %s HTTP %d: %s", url, resp.StatusCode, string(b))
	}
	return nil
}

func (c *ContentHTTP) ListForSerial(ctx context.Context, serial string) ([]port.ContentItem, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/content/"+serial, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("content-distributor GET /content/%s HTTP %d: %s", serial, resp.StatusCode, string(b))
	}
	var out struct {
		Items []struct {
			ContentID  string `json:"content_id"`
			Filename   string `json:"filename"`
			Status     string `json:"status"`
			DevicePath string `json:"device_path"`
			ObjectKey  string `json:"object_key"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	items := make([]port.ContentItem, 0, len(out.Items))
	for _, it := range out.Items {
		items = append(items, port.ContentItem{
			ContentID:  it.ContentID,
			Filename:   it.Filename,
			Status:     it.Status,
			DevicePath: it.DevicePath,
			ObjectKey:  it.ObjectKey,
			Serial:     serial,
		})
	}
	return items, nil
}

func (c *ContentHTTP) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("content-distributor health %d", resp.StatusCode)
	}
	return nil
}

var _ interface {
	Register(context.Context, port.ContentRegisterRequest) (port.ContentItem, error)
	ListForSerial(context.Context, string) ([]port.ContentItem, error)
	Ping(context.Context) error
} = (*ContentHTTP)(nil)
