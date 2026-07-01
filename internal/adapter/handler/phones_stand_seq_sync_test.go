package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/handler"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

func TestPhonesHTTP_SyncStandSeqFromHome(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	if _, err := phones.AddPhone(context.Background(), domain.AddPhoneRequest{Serial: "phone-304"}); err != nil {
		t.Fatal(err)
	}
	executor := &standSeqHTTPExecutor{}
	mux := standSeqSyncMux(phones, standSeqHTTPObserver{xml: `<hierarchy><node text="Stand 304" /></hierarchy>`}, executor)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/phone-304/stand-seq/sync-from-home", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !executor.homePressed {
		t.Fatal("home key was not pressed")
	}
	var got domain.StandSeqSyncResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.StandSeqNumber != 304 {
		t.Fatalf("stand_seq_number = %d, want 304", got.StandSeqNumber)
	}
	phone, err := phones.GetPhone(context.Background(), "phone-304")
	if err != nil {
		t.Fatal(err)
	}
	if phone.StandSeqNumber == nil || *phone.StandSeqNumber != 304 {
		t.Fatalf("saved stand seq = %v, want 304", phone.StandSeqNumber)
	}
}

func TestPhonesHTTP_SyncStandSeqFromHomeDoesNotOverwriteOnFailure(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	phones := service.NewPhoneService(store)
	old := int16(777)
	if _, err := phones.AddPhone(context.Background(), domain.AddPhoneRequest{Serial: "phone-777", StandSeqNumber: &old}); err != nil {
		t.Fatal(err)
	}
	mux := standSeqSyncMux(phones, standSeqHTTPObserver{xml: `<hierarchy><node text="Home" /></hierarchy>`}, &standSeqHTTPExecutor{})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/phones/phone-777/stand-seq/sync-from-home", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	phone, err := phones.GetPhone(context.Background(), "phone-777")
	if err != nil {
		t.Fatal(err)
	}
	if phone.StandSeqNumber == nil || *phone.StandSeqNumber != old {
		t.Fatalf("stand seq changed to %v, want %d", phone.StandSeqNumber, old)
	}
}

func standSeqSyncMux(phones *service.PhoneService, observer standSeqHTTPObserver, executor *standSeqHTTPExecutor) *http.ServeMux {
	store := repository.NewMemoryPhoneStore()
	orch := service.NewOrchestratorService(store, repository.NewMemoryPhoneLock(), driver.NewStubConnector(), driver.NewStubProvisioner(), nil, repository.NewNoopEventPublisher(), nil, 30, 1)
	h := handler.NewPhonesHTTP(phones, orch, driver.NewStubConnector(), observer, executor, driver.NewStubContent(), driver.NewStubContacts(), driver.NewStubVideo(), driver.NewStubScenarios(), nil)
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

type standSeqHTTPObserver struct {
	xml string
}

func (o standSeqHTTPObserver) CaptureScreen(context.Context, string) (domain.ScreenCapture, error) {
	return domain.ScreenCapture{}, nil
}

func (o standSeqHTTPObserver) DumpUI(_ context.Context, serial string) (domain.UIDump, error) {
	return domain.UIDump{Serial: serial, XMLDump: o.xml, Package: "com.android.launcher"}, nil
}

func (o standSeqHTTPObserver) Ping(context.Context) error { return nil }

type standSeqHTTPExecutor struct {
	homePressed bool
}

func (e *standSeqHTTPExecutor) ExecutePlan(context.Context, string, []domain.SolutionStep) error {
	return nil
}

func (e *standSeqHTTPExecutor) Tap(context.Context, string, int32, int32) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{}, nil
}

func (e *standSeqHTTPExecutor) Swipe(context.Context, string, int32, int32, int32, int32) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{}, nil
}

func (e *standSeqHTTPExecutor) TypeText(context.Context, string, string, bool) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{}, nil
}

func (e *standSeqHTTPExecutor) Key(_ context.Context, _ string, key string) (domain.ExecutorActionResult, error) {
	if key == "home" {
		e.homePressed = true
	}
	return domain.ExecutorActionResult{Action: "key", Status: "ok"}, nil
}

func (e *standSeqHTTPExecutor) LaunchPackage(context.Context, string, string) error { return nil }

func (e *standSeqHTTPExecutor) ForceStopPackage(context.Context, string, string) error { return nil }

func (e *standSeqHTTPExecutor) Ping(context.Context) error { return nil }
