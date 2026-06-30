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

type ScenariosHTTP struct {
	baseURL   string
	healthURL string
	client    *http.Client
}

func NewScenariosHTTP(cfg config.Config) *ScenariosHTTP {
	api := strings.TrimRight(cfg.ScenariosHTTPURL, "/")
	health := strings.TrimRight(cfg.ScenariosHealthURL, "/")
	if health == "" {
		health = api
	}
	return &ScenariosHTTP{
		baseURL:   api,
		healthURL: health,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *ScenariosHTTP) ListForSerial(ctx context.Context, serial string) (port.ScenarioListResult, error) {
	var out port.ScenarioListResult
	if err := c.getJSON(ctx, c.baseURL+"/scenarios/"+serial, &out); err != nil {
		return port.ScenarioListResult{}, err
	}
	return out, nil
}

func (c *ScenariosHTTP) SetActiveScenario(ctx context.Context, serial, scenarioID string) error {
	body, _ := json.Marshal(map[string]string{"scenario_id": scenarioID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/scenarios/"+serial+"/active", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scenarios active PUT HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *ScenariosHTTP) Get(ctx context.Context, serial, scenarioID string) (port.ScenarioFiles, error) {
	var out port.ScenarioFiles
	err := c.getJSON(ctx, c.baseURL+"/scenarios/"+serial+"/"+scenarioID, &out)
	return out, err
}

func (c *ScenariosHTTP) Put(ctx context.Context, serial, scenarioID string, files port.ScenarioFiles) error {
	body, _ := json.Marshal(files)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/scenarios/"+serial+"/"+scenarioID, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scenarios PUT HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *ScenariosHTTP) Delete(ctx context.Context, serial, scenarioID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/scenarios/"+serial+"/"+scenarioID, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scenarios DELETE HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *ScenariosHTTP) GetStatus(ctx context.Context, serial, scenarioID string) (port.ScenarioStatus, error) {
	var out port.ScenarioStatus
	err := c.getJSON(ctx, c.baseURL+"/scenarios/"+serial+"/"+scenarioID+"/status", &out)
	return out, err
}

func (c *ScenariosHTTP) GetLogs(ctx context.Context, serial, scenarioID, date string) (string, error) {
	url := c.baseURL + "/scenarios/" + serial + "/" + scenarioID + "/logs"
	if date != "" {
		url += "?date=" + date
	}
	var out struct {
		Logs string `json:"logs"`
	}
	if err := c.getJSON(ctx, url, &out); err != nil {
		return "", err
	}
	return out.Logs, nil
}

func (c *ScenariosHTTP) Generate(ctx context.Context, serial, prompt string) (port.ScenarioFiles, []string, error) {
	body, _ := json.Marshal(map[string]string{"serial": serial, "prompt": prompt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/scenarios/generate", bytes.NewReader(body))
	if err != nil {
		return port.ScenarioFiles{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return port.ScenarioFiles{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return port.ScenarioFiles{}, nil, fmt.Errorf("scenarios generate HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		ScenarioYAML  string                   `json:"scenario_yaml"`
		VariablesYAML string                   `json:"variables_yaml"`
		Warnings      []string                 `json:"warnings"`
		StepIssues    []map[string]interface{} `json:"step_issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return port.ScenarioFiles{}, nil, err
	}
	return port.ScenarioFiles{ScenarioYAML: out.ScenarioYAML, VariablesYAML: out.VariablesYAML}, out.Warnings, nil
}

// GenerateFull — полный ответ /scenarios/generate (warnings, step_issues).
func (c *ScenariosHTTP) GenerateFull(ctx context.Context, serial, prompt string) (map[string]any, error) {
	body, _ := json.Marshal(map[string]string{"serial": serial, "prompt": prompt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/scenarios/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scenarios generate HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ScenariosHTTP) Validate(ctx context.Context, serial, scenarioYAML, variablesYAML string, normalize bool) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{
		"serial":          serial,
		"scenario_yaml":   scenarioYAML,
		"variables_yaml":  variablesYAML,
		"normalize":       normalize,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/scenarios/validate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scenarios validate HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ScenariosHTTP) AppendScenarioLog(ctx context.Context, serial, scenarioID string, entry port.ScenarioLogEntry) error {
	body, _ := json.Marshal(entry)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/scenarios/"+serial+"/"+scenarioID+"/log", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scenarios log POST HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *ScenariosHTTP) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.healthURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("scenarios health %d", resp.StatusCode)
	}
	return nil
}

func (c *ScenariosHTTP) getJSON(ctx context.Context, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scenarios GET %s HTTP %d: %s", url, resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

type StubScenarios struct{}

func NewStubScenarios() *StubScenarios { return &StubScenarios{} }

func (s *StubScenarios) ListForSerial(_ context.Context, serial string) (port.ScenarioListResult, error) {
	return port.ScenarioListResult{Serial: serial, Items: []port.ScenarioSummary{}}, nil
}

func (s *StubScenarios) SetActiveScenario(context.Context, string, string) error { return nil }

func (s *StubScenarios) Get(_ context.Context, serial, scenarioID string) (port.ScenarioFiles, error) {
	return port.ScenarioFiles{
		ScenarioYAML:  fmt.Sprintf("id: %s\nserial: %s\nname: stub\n", scenarioID, serial),
		VariablesYAML: "# stub\n",
	}, nil
}

func (s *StubScenarios) Put(context.Context, string, string, port.ScenarioFiles) error { return nil }

func (s *StubScenarios) Delete(context.Context, string, string) error { return nil }

func (s *StubScenarios) GetStatus(_ context.Context, serial, scenarioID string) (port.ScenarioStatus, error) {
	return port.ScenarioStatus{Serial: serial, ScenarioID: scenarioID, Active: false}, nil
}

func (s *StubScenarios) GetLogs(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (s *StubScenarios) Generate(_ context.Context, serial, prompt string) (port.ScenarioFiles, []string, error) {
	return port.ScenarioFiles{
		ScenarioYAML:  fmt.Sprintf("id: generated\nserial: %s\nname: %q\n", serial, prompt),
		VariablesYAML: "# generated\n",
	}, []string{"stub mode"}, nil
}

func (s *StubScenarios) GenerateFull(_ context.Context, serial, prompt string) (map[string]any, error) {
	files, warnings, _ := s.Generate(context.Background(), serial, prompt)
	return map[string]any{
		"scenario_yaml":            files.ScenarioYAML,
		"variables_yaml":           files.VariablesYAML,
		"normalized_scenario_yaml": files.ScenarioYAML,
		"warnings":                 warnings,
		"valid":                    true,
		"runnable_by_scheduler":    true,
	}, nil
}

func (s *StubScenarios) Validate(_ context.Context, serial, scenarioYAML, variablesYAML string, normalize bool) (map[string]any, error) {
	return map[string]any{"valid": true, "warnings": []string{"stub mode"}}, nil
}

func (s *StubScenarios) AppendScenarioLog(context.Context, string, string, port.ScenarioLogEntry) error {
	return nil
}

func (s *StubScenarios) Ping(context.Context) error { return nil }

var _ port.ScenariosClient = (*ScenariosHTTP)(nil)
var _ port.ScenariosClient = (*StubScenarios)(nil)
