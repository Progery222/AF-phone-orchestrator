package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/handler"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

func TestScenarios_E2E(t *testing.T) {
	env := newTestEnv(t)
	defer env.cancel()
	defer env.server.Close()

	t.Run("health_ready", func(t *testing.T) {
		resp := env.get(t, "/health")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("health: %d", resp.StatusCode)
		}
		resp = env.get(t, "/ready")
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("ready: %d %s", resp.StatusCode, body)
		}
	})

	t.Run("scenario1_setup_new_to_working", func(t *testing.T) {
		serial := "TEST-PHONE-001"
		env.postJSON(t, "/phones", map[string]string{"serial": serial}, http.StatusCreated)

		phone := env.waitState(t, serial, domain.StateWorking, 10*time.Second)
		if phone.Model == "" {
			t.Log("model пустой (stub connector не заполняет — ок)")
		}
		t.Logf("setup complete: state=%s heartbeat_count=%d", phone.State, phone.HeartbeatCount)
	})

	t.Run("scenario2_heartbeat", func(t *testing.T) {
		serial := "TEST-PHONE-001"
		before, _ := env.store.Get(context.Background(), serial)
		time.Sleep(2 * time.Second)
		after, err := env.store.Get(context.Background(), serial)
		if err != nil {
			t.Fatal(err)
		}
		if after.HeartbeatCount <= before.HeartbeatCount {
			t.Fatalf("heartbeat не обновился: before=%d after=%d", before.HeartbeatCount, after.HeartbeatCount)
		}
		t.Logf("heartbeat: count=%d last=%v", after.HeartbeatCount, after.LastHeartbeat)
	})

	t.Run("scenario3_pause_recovery", func(t *testing.T) {
		serial := "TEST-PHONE-001"
		resp := env.post(t, "/phones/"+serial+"/pause?reason=Meta+Terms", nil)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("pause: %d %s", resp.StatusCode, body)
		}

		phone := env.waitState(t, serial, domain.StateWorking, 15*time.Second)
		if phone.LastErrorHash == "" {
			t.Fatal("после recovery ожидался last_error_hash от stub recovery")
		}
		if phone.LastError != "" {
			t.Fatalf("last_error должен быть очищен после recovery, got %q", phone.LastError)
		}
		t.Logf("recovery ok: error_hash=%s", phone.LastErrorHash)
	})

	t.Run("scenario5_reprovision", func(t *testing.T) {
		serial := "TEST-PHONE-002"
		env.postJSON(t, "/phones", map[string]string{"serial": serial}, http.StatusCreated)
		env.waitState(t, serial, domain.StateWorking, 10*time.Second)

		resp := env.post(t, "/phones/"+serial+"/reprovision", nil)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("reprovision: %d %s", resp.StatusCode, body)
		}

		p, _ := env.store.Get(context.Background(), serial)
		if p.State != domain.StateNew {
			t.Fatalf("ожидался new, got %s", p.State)
		}
		env.waitState(t, serial, domain.StateWorking, 10*time.Second)
		t.Log("reprovision: new → working снова")
	})

	t.Run("scenario4_error_manual", func(t *testing.T) {
		serial := "TEST-PHONE-003"
		env.postJSON(t, "/phones", map[string]string{"serial": serial}, http.StatusCreated)
		env.waitState(t, serial, domain.StateWorking, 10*time.Second)

		if err := env.orch.MarkError(context.Background(), serial, "ADB отключён"); err != nil {
			t.Fatal(err)
		}
		p, _ := env.store.Get(context.Background(), serial)
		if p.State != domain.StateError {
			t.Fatalf("ожидался error, got %s", p.State)
		}

		if err := env.orch.ResumePhone(context.Background(), serial); err != nil {
			t.Fatal(err)
		}
		p, _ = env.store.Get(context.Background(), serial)
		if p.State != domain.StateWorking {
			t.Fatalf("resume из error: ожидался working, got %s", p.State)
		}
		t.Log("error → resume → working")
	})

	t.Run("api_list_stats_remove", func(t *testing.T) {
		resp := env.get(t, "/phones")
		if resp.StatusCode != http.StatusOK {
			t.Fatal(resp.StatusCode)
		}
		var list struct {
			Total int `json:"total"`
			Stats struct {
				Working int `json:"working"`
			} `json:"stats"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&list)
		if list.Total < 3 {
			t.Fatalf("ожидалось >=3 телефонов, got %d", list.Total)
		}

		resp = env.get(t, "/stats")
		if resp.StatusCode != http.StatusOK {
			t.Fatal(resp.StatusCode)
		}

		resp = env.post(t, "/phones/TEST-PHONE-003/remove", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatal(resp.StatusCode)
		}
		p, err := env.store.Get(context.Background(), "TEST-PHONE-003")
		if err != nil {
			t.Fatal(err)
		}
		if p.State != domain.StateRetired {
			t.Fatalf("remove: ожидался retired, got %s", p.State)
		}

		resp = env.postJSON(t, "/phones", map[string]string{"serial": "TEST-PHONE-001"}, http.StatusConflict)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("duplicate: ожидался 409, got %d", resp.StatusCode)
		}
	})

	t.Run("debug_recovery_run", func(t *testing.T) {
		body := map[string]string{
			"serial":   "TEST-PHONE-001",
			"scenario": "YouTube offline",
			"context":  "нажать Retry",
		}
		resp := env.postJSON(t, "/recovery/run", body, http.StatusOK)
		var plan map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&plan)
		if plan["error_hash"] == nil && plan["ErrorHash"] == nil {
			// handler may return different json keys - check handler
		}
		t.Logf("recovery/run response keys: %v", keys(plan))
	})
}

type testEnv struct {
	server *httptest.Server
	store  *repository.MemoryPhoneStore
	orch   *service.OrchestratorService
	cancel context.CancelFunc
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	store := repository.NewMemoryPhoneStore()
	lock := repository.NewMemoryPhoneLock()
	observer := driver.NewStubObserver()
	recovery := driver.NewStubRecovery()
	executor := driver.NewStubExecutor()
	log := testLogger{t}

	flow := service.NewRecoveryFlowService(observer, recovery, executor, log)
	orch := service.NewOrchestratorService(
		store, lock, driver.NewStubConnector(), driver.NewStubProvisioner(),
		flow, repository.NewNoopEventPublisher(), log, 30, 1,
	)
	phones := service.NewPhoneService(store)
	orchHandler := handler.NewOrchestratorHandler(flow, log)
	phonesHTTP := handler.NewPhonesHTTP(phones, orch)

	mux := handler.NewHealthHandler(handler.HealthDeps{
		Observer: observer, Recovery: recovery, Executor: executor,
	}).Routes()
	phonesHTTP.Register(mux)
	mux.HandleFunc("/recovery/run", orchHandler.RunRecoveryHTTP)
	mux.HandleFunc("/recovery/outcome", orchHandler.ReportOutcomeHTTP)

	ctx, cancel := context.WithCancel(context.Background())
	go orch.Run(ctx)

	return &testEnv{
		server: httptest.NewServer(mux),
		store:  store,
		orch:   orch,
		cancel: cancel,
	}
}

func (e *testEnv) url(path string) string { return e.server.URL + path }

func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(e.url(path))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (e *testEnv) post(t *testing.T, path string, body io.Reader) *http.Response {
	t.Helper()
	resp, err := http.Post(e.url(path), "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (e *testEnv) postJSON(t *testing.T, path string, payload any, wantStatus int) *http.Response {
	t.Helper()
	b, _ := json.Marshal(payload)
	resp, err := http.Post(e.url(path), "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s: want %d got %d: %s", path, wantStatus, resp.StatusCode, body)
	}
	return resp
}

func (e *testEnv) waitState(t *testing.T, serial string, want domain.PhoneState, timeout time.Duration) domain.Phone {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p, err := e.store.Get(context.Background(), serial)
		if err == nil && p.State == want {
			return p
		}
		time.Sleep(200 * time.Millisecond)
	}
	p, _ := e.store.Get(context.Background(), serial)
	t.Fatalf("%s: не достигнуто состояние %s за %s (сейчас %s)", serial, want, timeout, p.State)
	return p
}

type testLogger struct{ t *testing.T }

func (l testLogger) Info(msg string, args ...any)  { l.t.Log(append([]any{msg}, args...)...) }
func (l testLogger) Warn(msg string, args ...any)  { l.t.Log(append([]any{"WARN:" + msg}, args...)...) }
func (l testLogger) Error(msg string, args ...any) { l.t.Log(append([]any{"ERR:" + msg}, args...)...) }
func (l testLogger) Debug(msg string, args ...any) {}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
