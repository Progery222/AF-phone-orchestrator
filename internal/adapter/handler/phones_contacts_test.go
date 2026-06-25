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

func newPhonesHTTPWithContacts(contacts port.ContactsClient) *handler.PhonesHTTP {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	orch := service.NewOrchestratorService(
		store, repository.NewMemoryPhoneLock(), driver.NewStubConnector(),
		driver.NewStubProvisioner(), nil, repository.NewNoopEventPublisher(), nil, 30, 1,
	)
	return handler.NewPhonesHTTP(phones, orch, driver.NewStubConnector(), nil, nil, driver.NewStubContent(), contacts, driver.NewStubVideo())
}

func TestPhonesHTTP_ContactsStubUpload(t *testing.T) {
	h := newPhonesHTTPWithContacts(driver.NewStubContacts())
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/phone_1/contacts/upload", bytes.NewReader([]byte(`{}`))))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/phones/phone_1/contacts", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPhonesHTTP_ContactsGroupsAndSync(t *testing.T) {
	h := newPhonesHTTPWithContacts(driver.NewStubContacts())
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]any{"groups": []map[string]string{{"name": "Мои"}}})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/stub/contacts/groups", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("groups: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/stub/contacts/sync", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}
}
