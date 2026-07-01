package service

import (
	"context"
	"errors"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func TestExtractStandSeqFromUIDump(t *testing.T) {
	tests := []struct {
		name string
		xml  string
		want int16
		err  error
	}{
		{name: "single labeled number", xml: `<hierarchy><node text="№ 304" /></hierarchy>`, want: 304},
		{name: "single standalone number", xml: `<hierarchy><node text="398" /></hierarchy>`, want: 398},
		{name: "no number", xml: `<hierarchy><node text="Instagram" /></hierarchy>`, err: ErrStandSeqNotFound},
		{name: "multiple numbers", xml: `<hierarchy><node text="Stand 304" /><node content-desc="Stand 305" /></hierarchy>`, err: ErrStandSeqAmbiguous},
		{name: "duplicate same number", xml: `<hierarchy><node text="Stand 304" /><node content-desc="№ 304" /></hierarchy>`, want: 304},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractStandSeqFromUIDump(tc.xml)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("error = %v, want %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("seq = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestStandSeqSyncServiceSyncFromHomeSavesParsedNumber(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	phones := NewPhoneService(store)
	ctx := context.Background()
	old := int16(999)
	if _, err := phones.AddPhone(ctx, domain.AddPhoneRequest{Serial: "phone-304", StandSeqNumber: &old}); err != nil {
		t.Fatal(err)
	}
	executor := &standSeqSyncExecutor{}
	svc := NewStandSeqSyncService(phones, standSeqSyncObserver{
		xml: `<hierarchy><node text="№ 304" /></hierarchy>`,
	}, executor)
	svc.homeWait = 0

	result, err := svc.SyncFromHome(ctx, "phone-304")
	if err != nil {
		t.Fatal(err)
	}
	if !executor.homePressed {
		t.Fatal("home key was not pressed")
	}
	if result.StandSeqNumber != 304 || result.Source != "ui_dump" {
		t.Fatalf("result = %+v, want seq 304 from ui_dump", result)
	}
	phone, err := phones.GetPhone(ctx, "phone-304")
	if err != nil {
		t.Fatal(err)
	}
	if phone.StandSeqNumber == nil || *phone.StandSeqNumber != 304 {
		t.Fatalf("saved stand seq = %v, want 304", phone.StandSeqNumber)
	}
}

func TestStandSeqSyncServiceDoesNotOverwriteWhenNumberMissing(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	phones := NewPhoneService(store)
	ctx := context.Background()
	old := int16(777)
	if _, err := phones.AddPhone(ctx, domain.AddPhoneRequest{Serial: "phone-777", StandSeqNumber: &old}); err != nil {
		t.Fatal(err)
	}
	svc := NewStandSeqSyncService(phones, standSeqSyncObserver{
		xml: `<hierarchy><node text="Home" /></hierarchy>`,
	}, &standSeqSyncExecutor{})
	svc.homeWait = 0

	_, err := svc.SyncFromHome(ctx, "phone-777")
	if !errors.Is(err, ErrStandSeqOCRUnavailable) {
		t.Fatalf("error = %v, want %v", err, ErrStandSeqOCRUnavailable)
	}
	phone, err := phones.GetPhone(ctx, "phone-777")
	if err != nil {
		t.Fatal(err)
	}
	if phone.StandSeqNumber == nil || *phone.StandSeqNumber != old {
		t.Fatalf("stand seq changed to %v, want %d", phone.StandSeqNumber, old)
	}
}

type standSeqSyncObserver struct {
	xml string
}

func (o standSeqSyncObserver) CaptureScreen(context.Context, string) (domain.ScreenCapture, error) {
	return domain.ScreenCapture{}, nil
}

func (o standSeqSyncObserver) DumpUI(_ context.Context, serial string) (domain.UIDump, error) {
	return domain.UIDump{Serial: serial, XMLDump: o.xml, Package: "com.android.launcher"}, nil
}

func (o standSeqSyncObserver) Ping(context.Context) error { return nil }

type standSeqSyncExecutor struct {
	homePressed bool
}

func (e *standSeqSyncExecutor) ExecutePlan(context.Context, string, []domain.SolutionStep) error {
	return nil
}

func (e *standSeqSyncExecutor) Tap(context.Context, string, int32, int32) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{}, nil
}

func (e *standSeqSyncExecutor) Swipe(context.Context, string, int32, int32, int32, int32) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{}, nil
}

func (e *standSeqSyncExecutor) TypeText(context.Context, string, string, bool) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{}, nil
}

func (e *standSeqSyncExecutor) Key(_ context.Context, _ string, key string) (domain.ExecutorActionResult, error) {
	if key == "home" {
		e.homePressed = true
	}
	return domain.ExecutorActionResult{Action: "key", Status: "ok"}, nil
}

func (e *standSeqSyncExecutor) LaunchPackage(context.Context, string, string) error { return nil }

func (e *standSeqSyncExecutor) ForceStopPackage(context.Context, string, string) error { return nil }

func (e *standSeqSyncExecutor) Ping(context.Context) error { return nil }
