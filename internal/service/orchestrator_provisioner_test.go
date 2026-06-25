package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func TestOrchestratorStateMachine_WithProvisionerHTTP(t *testing.T) {
	fake := newMockProvisionerServer(t, 150*time.Millisecond)
	defer fake.Close()

	store := repository.NewMemoryPhoneStore()
	now := time.Now()
	_ = store.Save(context.Background(), domain.Phone{
		Serial: "prov-http-1", State: domain.StateNew,
		CreatedAt: now, UpdatedAt: now, AdbPort: 5555,
	})

	provision := driver.NewProvisionHTTP(config.Config{
		ProvisionerHTTPURL:          fake.URL,
		ProvisionerDefaultProxyIP:   "45.67.89.10",
		ProvisionerDefaultProxyPort: 3128,
	})

	orch := NewOrchestratorService(
		store, repository.NewMemoryPhoneLock(),
		driver.NewStubConnector(), provision,
		NewRecoveryFlowService(stubObserver{}, stubRecovery{}, stubExecutor{}, nopLogger{}),
		repository.NewNoopEventPublisher(), nopLogger{}, 30, 1,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go orch.Run(ctx)

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		p, err := store.Get(ctx, "prov-http-1")
		if err == nil && p.State == domain.StateWorking {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	p, _ := store.Get(ctx, "prov-http-1")
	t.Fatalf("phone did not reach working, state=%s", p.State)
}

func newMockProvisionerServer(t *testing.T, readyAfter time.Duration) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	runs := map[string]string{}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/provision":
			var body struct {
				Serial string `json:"serial"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			runs[body.Serial] = "provisioning"
			mu.Unlock()
			go func(serial string) {
				time.Sleep(readyAfter)
				mu.Lock()
				runs[serial] = "ready"
				mu.Unlock()
			}(body.Serial)
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"serial": body.Serial, "status": "provisioning"})
		case r.Method == http.MethodGet && r.URL.Path == "/status":
			serial := r.URL.Query().Get("serial")
			mu.Lock()
			st, ok := runs[serial]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"serial": serial, "status": st})
		default:
			http.NotFound(w, r)
		}
	}))
}
