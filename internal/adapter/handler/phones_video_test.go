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
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

func newPhonesHTTPWithVideo(video port.VideoClient) *handler.PhonesHTTP {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	orch := service.NewOrchestratorService(store, repository.NewMemoryPhoneLock(), driver.NewStubConnector(), driver.NewStubProvisioner(), nil, repository.NewNoopEventPublisher(), nil, 30, 2)
	return handler.NewPhonesHTTP(phones, orch, driver.NewStubConnector(), nil, nil, driver.NewStubContent(), driver.NewStubContacts(), video, driver.NewStubScenarios(), nil)
}

func TestPhonesHTTP_VideoScreenshotsStub(t *testing.T) {
	h := newPhonesHTTPWithVideo(driver.NewStubVideo())
	mux := http.NewServeMux()
	h.Register(mux)

	body := []byte(`{"screenshot_keys":["shots/1.jpg","shots/2.jpg"]}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/R5CY331L8NF/video/screenshots", bytes.NewReader(body)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var job map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job["id"] == "" || job["status"] != "JOB_STATUS_PENDING" {
		t.Fatalf("unexpected job: %+v", job)
	}
}

func TestPhonesHTTP_VideoGetJobStub(t *testing.T) {
	h := newPhonesHTTPWithVideo(driver.NewStubVideo())
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/phones/stub/video/jobs/job-1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPhonesHTTP_VideoAIBadRequest(t *testing.T) {
	h := newPhonesHTTPWithVideo(driver.NewStubVideo())
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/stub/video/ai", bytes.NewReader([]byte(`{}`))))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
