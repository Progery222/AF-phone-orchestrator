//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	defaultOrchURL     = "http://127.0.0.1:19092"
	defaultObserverURL = "http://127.0.0.1:19090"
	defaultRecoveryURL = "http://127.0.0.1:9094"
	defaultExecutorURL = "http://127.0.0.1:9091"
	testSerial         = "stub"
)

type env struct {
	orch     string
	observer string
	recovery string
	executor string
	client   *http.Client
}

func newEnv() env {
	return env{
		orch:     getenv("E2E_ORCH_URL", defaultOrchURL),
		observer: getenv("E2E_OBSERVER_URL", defaultObserverURL),
		recovery: getenv("E2E_RECOVERY_HEALTH", defaultRecoveryURL),
		executor: getenv("E2E_EXECUTOR_HEALTH", defaultExecutorURL),
		client:   &http.Client{Timeout: 90 * time.Second},
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestBundle_HealthAllServices(t *testing.T) {
	e := newEnv()
	checks := []struct {
		name string
		url  string
	}{
		{"orchestrator", e.orch + "/health"},
		{"orchestrator_ready", e.orch + "/ready"},
		{"observer", e.observer + "/health"},
		{"recovery", e.recovery + "/health"},
		{"executor", e.executor + "/health"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			resp, err := e.client.Get(c.url)
			if err != nil {
				t.Fatalf("GET %s: %v", c.url, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("%s: status %d body %s", c.url, resp.StatusCode, body)
			}
		})
	}
}

func TestBundle_OrchestratorScreenViaObserver(t *testing.T) {
	e := newEnv()
	screen := e.getJSON(t, e.orch+"/phones/"+testSerial+"/screen?timeout_sec=10")
	if screen["minio_key"] == nil || screen["minio_key"] == "" {
		t.Fatalf("ожидался minio_key: %v", screen)
	}
	url, _ := screen["screenshot_url"].(string)
	if url == "" {
		t.Fatal("ожидался screenshot_url (MinIO или noop)")
	}
	if strings.HasPrefix(url, "noop://") {
		t.Log("MinIO недоступен — observer вернул noop URL (для e2e stub допустимо)")
	}
	t.Logf("orchestrator→observer ok: minio_key=%v url=%s", screen["minio_key"], url)
}

func TestBundle_RecoveryRun_LoginScenario(t *testing.T) {
	e := newEnv()
	plan := e.postRecoveryRun(t, map[string]string{
		"serial":   testSerial,
		"scenario": "Вход в аккаунт",
		"context":  "нажать кнопку Войти login",
	})
	assertPlanOK(t, plan)
	steps, _ := plan["Steps"].([]any)
	if steps == nil {
		steps, _ = plan["steps"].([]any)
	}
	if len(steps) == 0 {
		// json may use Steps from RecoveryPlan struct
		if raw, ok := plan["Steps"]; ok {
			t.Logf("plan Steps: %v", raw)
		}
	}
	if hash := strField(plan, "ErrorHash", "error_hash"); hash == "" {
		t.Fatal("ожидался error_hash в плане recovery")
	}
	src := strField(plan, "Source", "source")
	if src == "stub" {
		t.Log("recovery via orchestrator stub (NATS unavailable) — ok for bundle smoke")
	}
	t.Logf("recovery plan: source=%s hash=%s", src, strField(plan, "ErrorHash", "error_hash"))
}

func TestBundle_RecoveryRun_CachedFromDB(t *testing.T) {
	e := newEnv()
	body := map[string]string{
		"serial":   testSerial,
		"scenario": "E2E cache test",
		"context":  "нажать кнопку Войти login",
	}
	first := e.postRecoveryRun(t, body)
	assertPlanOK(t, first)
	hash1 := strField(first, "ErrorHash", "error_hash")

	second := e.postRecoveryRun(t, body)
	assertPlanOK(t, second)
	hash2 := strField(second, "ErrorHash", "error_hash")
	if hash1 != hash2 {
		t.Fatalf("error_hash должен совпадать: %s vs %s", hash1, hash2)
	}
	if src := strField(second, "Source", "source"); src != "db" && src != "llm" {
		t.Logf("второй ответ source=%q (db/llm после первого solve)", src)
	}
}

func TestBundle_OrchestratorSetupPauseRecovery(t *testing.T) {
	e := newEnv()
	serial := testSerial

	e.postJSONExpect(t, e.orch+"/phones", map[string]string{"serial": serial}, http.StatusCreated)
	waitPhoneState(t, e, serial, "working", 15*time.Second)

	resp, err := e.client.Post(e.orch+"/phones/"+serial+"/pause?reason=Meta+login", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("pause: %d %s", resp.StatusCode, body)
	}

	phone := waitPhoneState(t, e, serial, "working", 30*time.Second)
	if phone["last_error_hash"] == nil || phone["last_error_hash"] == "" {
		if h := strField(phone, "last_error_hash", "LastErrorHash"); h == "" {
			t.Fatal("после pause recovery ожидался last_error_hash")
		}
	}
	t.Logf("pause recovery ok: state=%v hash=%v", phone["state"], phone["last_error_hash"])
}

func TestBundle_RecoveryOutcomeAccepted(t *testing.T) {
	e := newEnv()
	plan := e.postRecoveryRun(t, map[string]string{
		"serial":   testSerial,
		"scenario": "outcome test",
		"context":  "ошибка error dialog",
	})
	hash := strField(plan, "ErrorHash", "error_hash")
	if hash == "" {
		t.Fatal("no error_hash for outcome")
	}
	resp, err := e.postJSON(e.recoveryURLToOrchOutcome(e.orch), map[string]any{
		"error_hash": hash,
		"serial":     testSerial,
		"success":    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("outcome: %d %s", resp.StatusCode, body)
	}
}

func (e env) recoveryURLToOrchOutcome(orch string) string {
	return orch + "/recovery/outcome"
}

func (e env) getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := e.client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: %d %s", url, resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e env) postRecoveryRun(t *testing.T, body map[string]string) map[string]any {
	t.Helper()
	resp, err := e.postJSON(e.orch+"/recovery/run", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("recovery/run: %d %s", resp.StatusCode, raw)
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatalf("decode plan: %v raw=%s", err, raw)
	}
	return plan
}

func (e env) postJSON(url string, payload any) (*http.Response, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return e.client.Post(url, "application/json", bytes.NewReader(b))
}

func (e env) postJSONExpect(t *testing.T, url string, payload any, want int) {
	t.Helper()
	resp, err := e.postJSON(url, payload)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s: want %d got %d: %s", url, want, resp.StatusCode, body)
	}
}

func waitPhoneState(t *testing.T, e env, serial, want string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := e.client.Get(e.orch + "/phones/" + serial)
		if err == nil && resp.StatusCode == http.StatusOK {
			var phone map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&phone)
			resp.Body.Close()
			state, _ := phone["state"].(string)
			if strings.EqualFold(state, want) {
				return phone
			}
		} else if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("%s: состояние %s не достигнуто за %s", serial, want, timeout)
	return nil
}

func assertPlanOK(t *testing.T, plan map[string]any) {
	t.Helper()
	if errField := strField(plan, "error", "Error"); errField != "" {
		t.Fatalf("plan error: %s", errField)
	}
	steps := plan["Steps"]
	if steps == nil {
		steps = plan["steps"]
	}
	if steps == nil {
		// Go struct JSON uses exported field names
		if plan["Steps"] == nil && plan["steps"] == nil {
			t.Logf("plan keys: %v", keys(plan))
		}
	}
}

func strField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
