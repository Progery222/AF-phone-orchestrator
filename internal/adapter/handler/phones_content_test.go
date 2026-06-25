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
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

func newPhonesHTTPWithContent(content port.ContentClient) *handler.PhonesHTTP {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	orch := service.NewOrchestratorService(
		store, repository.NewMemoryPhoneLock(), driver.NewStubConnector(),
		driver.NewStubProvisioner(), nil, repository.NewNoopEventPublisher(), nil, 30, 1,
	)
	return handler.NewPhonesHTTP(phones, orch, driver.NewStubConnector(), nil, nil, content)
}

func TestPhonesHTTP_ContentRegisterAndList(t *testing.T) {
	dist := httptest.NewServer(fakeDistributorHandler(t))
	defer dist.Close()

	httpClient := driver.NewContentHTTP(config.Config{ContentDistributorHTTPURL: dist.URL})
	content := driver.NewContentClient(httpClient, nil)
	h := newPhonesHTTPWithContent(content)
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]string{
		"object_key": "posts/post.jpg",
		"filename":   "post.jpg",
		"media_type": "photo",
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/phone_7/content/register", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("register proxy: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/phones/phone_7/content", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list proxy: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPhonesHTTP_ContentDownloadByObjectKey(t *testing.T) {
	dist := httptest.NewServer(fakeDistributorHandler(t))
	defer dist.Close()

	httpClient := driver.NewContentHTTP(config.Config{ContentDistributorHTTPURL: dist.URL})
	content := driver.NewContentClient(httpClient, nil)
	h := newPhonesHTTPWithContent(content)
	mux := http.NewServeMux()
	h.Register(mux)

	dlBody, _ := json.Marshal(map[string]string{"object_key": "posts/post.jpg"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/stub/content/download", bytes.NewReader(dlBody)))
	if rec.Code != http.StatusAccepted && rec.Code != http.StatusOK {
		t.Fatalf("download proxy: %d %s", rec.Code, rec.Body.String())
	}
}

func fakeDistributorHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	var lastID string
	mux.HandleFunc("POST /content/register", func(w http.ResponseWriter, r *http.Request) {
		lastID = "test-content-id"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content_id": lastID, "serial": "stub", "object_key": "posts/post.jpg", "status": "queued",
		})
	})
	mux.HandleFunc("POST /content/download", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content_id": lastID, "serial": "stub", "device_path": "/sdcard/Download/post.jpg", "status": "on_device",
		})
	})
	mux.HandleFunc("GET /content/{serial}", func(w http.ResponseWriter, r *http.Request) {
		items := []any{}
		if lastID != "" {
			items = append(items, map[string]string{"content_id": lastID, "status": "queued"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"serial": r.PathValue("serial"), "items": items})
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}
