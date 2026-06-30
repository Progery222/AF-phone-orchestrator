package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/handler"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

func TestPhoneApps_ListAndOpen(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	orch := service.NewOrchestratorService(store, repository.NewMemoryPhoneLock(), driver.NewStubConnector(), driver.NewStubProvisioner(), nil, repository.NewNoopEventPublisher(), nil, 30, 1)
	executor := driver.NewStubExecutor()
	h := handler.NewPhonesHTTP(phones, orch, driver.NewStubConnector(), driver.NewStubObserver(), executor, driver.NewStubContent(), driver.NewStubContacts(), driver.NewStubVideo(), driver.NewStubScenarios(), nil)
	mux := http.NewServeMux()
	h.Register(mux)

	serial := "stub"

	req := httptest.NewRequest(http.MethodGet, "/phones/"+serial+"/apps", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list apps: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Apps []struct{ Package string `json:"package"` }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Apps) == 0 {
		t.Fatal("expected system apps")
	}

	body, _ := json.Marshal(map[string]string{"package": "com.android.chrome"})
	req2 := httptest.NewRequest(http.MethodPost, "/phones/"+serial+"/apps/open", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("open app: %d %s", rec2.Code, rec2.Body.String())
	}
}
