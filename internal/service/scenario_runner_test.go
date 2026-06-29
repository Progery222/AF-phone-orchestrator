package service

import (
	"context"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

type testLogger struct{}

func (testLogger) Info(string, ...any)  {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Error(string, ...any) {}
func (testLogger) Debug(string, ...any) {}

func TestScenarioRunner_Wait(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	runner := NewScenarioRunner(
		driver.NewStubExecutor(),
		driver.NewStubObserver(),
		driver.NewStubVideo(),
		driver.NewStubContent(),
		driver.NewStubScenarios(),
		store,
		testLogger{},
	)
	res, err := runner.RunStep(context.Background(), ScenarioStepRequest{
		Serial: "stub", ScenarioID: "s1", StepID: "w1", Action: "wait",
		Params: map[string]string{"duration_sec": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "completed" {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestScenarioRunner_OpenApp(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	_ = store.Save(context.Background(), domain.Phone{Serial: "stub", ScreenResX: 1080, ScreenResY: 1920})
	runner := NewScenarioRunner(
		driver.NewStubExecutor(),
		driver.NewStubObserver(),
		driver.NewStubVideo(),
		driver.NewStubContent(),
		driver.NewStubScenarios(),
		store,
		testLogger{},
	)
	_, err := runner.RunStep(context.Background(), ScenarioStepRequest{
		Serial: "stub", ScenarioID: "s1", StepID: "o1", Action: "open_app",
		Params: map[string]string{"package": "com.example.app"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFindOrganicLink(t *testing.T) {
	xml := `<node text="Статья на vc.ru" bounds="[100,500][900,600]"/>`
	x, y, ok := findOrganicLink(xml, "vc.ru")
	if !ok || x != 500 || y != 550 {
		t.Fatalf("got %d,%d ok=%v", x, y, ok)
	}
}

func TestParseScenarioVariables(t *testing.T) {
	raw := `warmup_feed:
  scroll_interval_sec: [3, 12]
`
	v, err := parseScenarioVariables(raw)
	if err != nil {
		t.Fatal(err)
	}
	if v.WarmupFeed == nil {
		t.Fatal("expected warmup_feed")
	}
}
