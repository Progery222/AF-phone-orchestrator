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

type BehaviorHTTP struct {
	baseURL string
	client  *http.Client
}

func NewBehaviorHTTP(cfg config.Config) *BehaviorHTTP {
	return &BehaviorHTTP{
		baseURL: strings.TrimRight(cfg.BehaviorHTTPURL, "/"),
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *BehaviorHTTP) RunSocialAction(ctx context.Context, network, action, serial string, body map[string]any) (port.BehaviorJob, error) {
	if body == nil {
		body = map[string]any{}
	}
	body["serial"] = serial
	url := fmt.Sprintf("%s/social/%s/%s", c.baseURL, network, action)
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return port.BehaviorJob{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return port.BehaviorJob{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return port.BehaviorJob{}, fmt.Errorf("behavior POST %s HTTP %d: %s", url, resp.StatusCode, string(b))
	}
	var job port.BehaviorJob
	if err := json.Unmarshal(b, &job); err != nil {
		return port.BehaviorJob{}, err
	}
	if job.ID == "" {
		return port.BehaviorJob{}, fmt.Errorf("behavior-engine не вернул job id")
	}
	return job, nil
}

func (c *BehaviorHTTP) GetJob(ctx context.Context, jobID string) (port.BehaviorJob, error) {
	url := fmt.Sprintf("%s/jobs/%s", c.baseURL, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return port.BehaviorJob{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return port.BehaviorJob{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return port.BehaviorJob{}, fmt.Errorf("behavior GET job HTTP %d: %s", resp.StatusCode, string(b))
	}
	var job port.BehaviorJob
	if err := json.Unmarshal(b, &job); err != nil {
		return port.BehaviorJob{}, err
	}
	return job, nil
}

func (c *BehaviorHTTP) WaitJob(ctx context.Context, jobID string, timeout time.Duration) (port.BehaviorJob, error) {
	deadline := time.Now().Add(timeout)
	for {
		job, err := c.GetJob(ctx, jobID)
		if err != nil {
			return port.BehaviorJob{}, err
		}
		switch strings.ToLower(job.Status) {
		case "done", "completed", "success":
			return job, nil
		case "failed", "error":
			msg := job.Error
			if msg == "" {
				msg = "behavior job failed"
			}
			return job, fmt.Errorf("%s", msg)
		}
		if time.Now().After(deadline) {
			return job, fmt.Errorf("таймаут ожидания job %s (status=%s)", jobID, job.Status)
		}
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *BehaviorHTTP) Ping(ctx context.Context) error {
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
		return fmt.Errorf("behavior health %d", resp.StatusCode)
	}
	return nil
}

var _ port.BehaviorClient = (*BehaviorHTTP)(nil)
