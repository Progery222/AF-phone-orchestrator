package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

type feedChangingObserver struct {
	stubObserver
	swipeN atomic.Int32
}

func (o *feedChangingObserver) CaptureScreen(_ context.Context, _ string) (domain.ScreenCapture, error) {
	return domain.ScreenCapture{Width: 720, Height: 1600}, nil
}

func (o *feedChangingObserver) DumpUI(_ context.Context, _ string) (domain.UIDump, error) {
	n := o.swipeN.Load()
	return domain.UIDump{XMLDump: fmt.Sprintf(`<node text="video-%d title"/>`, n+1)}, nil
}

type swipeCountExecutor struct {
	stubExecutor
	n   atomic.Int32
	obs *feedChangingObserver
}

func (e *swipeCountExecutor) Swipe(_ context.Context, _ string, _, _, _, _ int32) (domain.ExecutorActionResult, error) {
	e.n.Add(1)
	if e.obs != nil {
		e.obs.swipeN.Add(1)
	}
	return domain.ExecutorActionResult{Status: "ok"}, nil
}

func TestScenarioChain_WarmupFeed_SwipesAndVerifiesChange(t *testing.T) {
	obs := &feedChangingObserver{}
	exec := &swipeCountExecutor{obs: obs}
	store := repository.NewMemoryPhoneStore()
	_ = store.Save(context.Background(), domain.Phone{Serial: "phone-1", State: domain.StateWorking, ScreenResX: 720, ScreenResY: 1600})
	runner := NewScenarioRunner(exec, obs, nil, nil, nil, nil, store, nil, testLogger{})

	res, err := runner.RunStep(context.Background(), ScenarioStepRequest{
		Serial: "phone-1", ScenarioID: "s1", StepID: "scroll_tiktok", Action: "warmup_feed",
		Params: map[string]string{"duration_sec": "8"},
		VariablesYAML: `warmup_feed:
  scroll_interval_sec: [1, 1]
  view_duration_sec: [1, 1]
  swipe_pause_ms: [50, 50]
  initial_view_sec: [1, 1]
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "completed" {
		t.Fatalf("status=%s msg=%s", res.Status, res.Message)
	}
	if exec.n.Load() < 1 {
		t.Fatalf("executor swipe not called, count=%d", exec.n.Load())
	}
}

type staticFeedObserver struct {
	stubObserver
}

func (staticFeedObserver) DumpUI(context.Context, string) (domain.UIDump, error) {
	return domain.UIDump{XMLDump: `<node text="same video always"/>`}, nil
}

type countingRecoveryClient struct {
	stubRecovery
	calls atomic.Int32
}

func (c *countingRecoveryClient) Solve(_ context.Context, _ domain.RecoverySolveRequest) (domain.RecoverySolveResponse, error) {
	c.calls.Add(1)
	return domain.RecoverySolveResponse{
		Success: true, ErrorHash: "hash-test", Source: "stub",
		Solution: []domain.SolutionStep{{Type: "wait", Sec: 1}},
	}, nil
}

type alwaysEmptyFeedObserver struct {
	stubObserver
}

func (alwaysEmptyFeedObserver) CaptureScreen(_ context.Context, _ string) (domain.ScreenCapture, error) {
	return domain.ScreenCapture{Width: 720, Height: 1600}, nil
}

func (alwaysEmptyFeedObserver) DumpUI(_ context.Context, _ string) (domain.UIDump, error) {
	return domain.UIDump{XMLDump: `<node/>`}, nil
}

func TestScenarioChain_WarmupFeed_EmptyMarkerNoRecovery(t *testing.T) {
	obs := alwaysEmptyFeedObserver{}
	rec := &countingRecoveryClient{}
	exec := &swipeCountExecutor{}
	store := repository.NewMemoryPhoneStore()
	_ = store.Save(context.Background(), domain.Phone{Serial: "phone-1", State: domain.StateWorking, ScreenResX: 720, ScreenResY: 1600})
	flow := NewRecoveryFlowService(obs, rec, exec, testLogger{})
	runner := NewScenarioRunner(exec, obs, nil, nil, nil, nil, store, flow, testLogger{})

	_, err := runner.RunStep(context.Background(), ScenarioStepRequest{
		Serial: "phone-1", ScenarioID: "s1", StepID: "scroll_tiktok", Action: "warmup_feed",
		Params: map[string]string{"duration_sec": "10"},
		VariablesYAML: `warmup_feed:
  scroll_interval_sec: [1, 1]
  view_duration_sec: [1, 1]
  swipe_pause_ms: [50, 50]
  initial_view_sec: [1, 1]
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.calls.Load() > 0 {
		t.Fatalf("recovery не должен вызываться при пустых маркерах observer, calls=%d", rec.calls.Load())
	}
}

func TestScenarioChain_RecoveryOnUnchangedFeed(t *testing.T) {
	obs := staticFeedObserver{}
	rec := &countingRecoveryClient{}
	store := repository.NewMemoryPhoneStore()
	_ = store.Save(context.Background(), domain.Phone{Serial: "phone-1", State: domain.StateWorking})
	flow := NewRecoveryFlowService(obs, rec, &swipeCountExecutor{}, testLogger{})
	runner := NewScenarioRunner(&swipeCountExecutor{}, obs, nil, nil, nil, nil, store, flow, testLogger{})

	res, err := runner.RunStep(context.Background(), ScenarioStepRequest{
		Serial: "phone-1", ScenarioID: "s1", StepID: "scroll_tiktok", Action: "warmup_feed",
		Params: map[string]string{"duration_sec": "12"},
		VariablesYAML: `warmup_feed:
  scroll_interval_sec: [1, 1]
  view_duration_sec: [1, 1]
  swipe_pause_ms: [50, 50]
  initial_view_sec: [1, 1]
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "completed" {
		t.Fatalf("status=%s", res.Status)
	}
	if rec.calls.Load() < 1 {
		t.Fatal("ожидался вызов recovery при неизменной ленте")
	}
}
