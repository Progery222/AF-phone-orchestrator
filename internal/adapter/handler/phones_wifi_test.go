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

func TestPhonesHTTP_WifiEnable(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	orch := service.NewOrchestratorService(
		store, repository.NewMemoryPhoneLock(), driver.NewStubConnector(),
		driver.NewStubProvisioner(), nil, repository.NewNoopEventPublisher(), nil, 30, 1,
	)
	h := handler.NewPhonesHTTP(phones, orch, driver.NewStubConnector(), nil, nil)

	body, _ := json.Marshal(map[string]string{
		"action":   "enable",
		"ssid":     "Office_5G",
		"password": "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/phones/phone_42/wifi", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPhonesHTTP_WifiDisable(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	orch := service.NewOrchestratorService(
		store, repository.NewMemoryPhoneLock(), driver.NewStubConnector(),
		driver.NewStubProvisioner(), nil, repository.NewNoopEventPublisher(), nil, 30, 1,
	)
	h := handler.NewPhonesHTTP(phones, orch, driver.NewStubConnector(), nil, nil)

	body, _ := json.Marshal(map[string]string{"action": "disable"})
	req := httptest.NewRequest(http.MethodPost, "/phones/phone_42/wifi", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}
