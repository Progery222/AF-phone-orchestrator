package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type pollingBehaviorClient struct {
	polls atomic.Int32
}

func (c *pollingBehaviorClient) RunSocialAction(_ context.Context, _, _, _ string, _ map[string]any) (port.BehaviorJob, error) {
	return port.BehaviorJob{ID: "job-test-1", Status: "pending"}, nil
}

func (c *pollingBehaviorClient) GetJob(context.Context, string) (port.BehaviorJob, error) {
	n := c.polls.Add(1)
	if n < 3 {
		return port.BehaviorJob{ID: "job-test-1", Status: "running"}, nil
	}
	return port.BehaviorJob{
		ID: "job-test-1", Status: "done",
		Result: map[string]interface{}{"query": "Football", "scrolled": 12},
	}, nil
}

func (c *pollingBehaviorClient) WaitJob(context.Context, string, time.Duration) (port.BehaviorJob, error) {
	return port.BehaviorJob{}, nil
}

func (c *pollingBehaviorClient) Ping(context.Context) error { return nil }

func TestWaitBehaviorJobWithLogs_PollsUntilDone(t *testing.T) {
	behavior := &pollingBehaviorClient{}
	store := repository.NewMemoryPhoneStore()
	_ = store.Save(context.Background(), domain.Phone{Serial: "phone-1", State: domain.StateWorking})
	runner := NewScenarioRunner(
		&swipeCountExecutor{}, nil, nil, nil, nil, behavior, store, nil, testLogger{},
	)
	req := ScenarioStepRequest{
		Serial: "phone-1", ScenarioID: "s1", StepID: "search_football", Action: "social_action",
	}
	job, err := runner.waitBehaviorJobWithLogs(context.Background(), req, "social_action", "job-test-1", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "done" {
		t.Fatalf("status=%s", job.Status)
	}
}

func TestBehaviorJobTerminal(t *testing.T) {
	if !behaviorJobTerminal("done") || !behaviorJobTerminal("failed") {
		t.Fatal("terminal statuses")
	}
	if behaviorJobTerminal("running") {
		t.Fatal("running is not terminal")
	}
}
