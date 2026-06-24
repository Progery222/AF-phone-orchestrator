package service

import (
	"context"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func TestRecoveryFlowService_RunRecovery(t *testing.T) {
	svc := NewRecoveryFlowService(
		stubObserver{},
		stubRecovery{},
		stubExecutor{},
		nopLogger{},
	)

	plan, err := svc.RunRecovery(context.Background(), "phone_1", "test", "контекст")
	if err != nil {
		t.Fatal(err)
	}
	if plan.ErrorHash == "" || len(plan.Steps) == 0 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

type stubObserver struct{}

func (stubObserver) CaptureScreen(_ context.Context, serial string) (domain.ScreenCapture, error) {
	return domain.ScreenCapture{Serial: serial, MinioKey: serial + "/shot.png"}, nil
}
func (stubObserver) DumpUI(_ context.Context, serial string) (domain.UIDump, error) {
	return domain.UIDump{Serial: serial, XMLDump: "<hierarchy/>"}, nil
}
func (stubObserver) Ping(context.Context) error { return nil }

type stubRecovery struct{}

func (stubRecovery) Solve(_ context.Context, req domain.RecoverySolveRequest) (domain.RecoverySolveResponse, error) {
	return domain.RecoverySolveResponse{
		Success:    true,
		ErrorHash:  "hash-1",
		ScenarioID: "sc-1",
		Source:     "db",
		Solution:   []domain.SolutionStep{{Type: "tap", X: 100, Y: 200}},
	}, nil
}
func (stubRecovery) ReportOutcome(context.Context, domain.RecoveryOutcomeRequest) error { return nil }
func (stubRecovery) Ping(context.Context) error                                         { return nil }

type stubExecutor struct{}

func (stubExecutor) ExecutePlan(context.Context, string, []domain.SolutionStep) error { return nil }
func (stubExecutor) Ping(context.Context) error                                       { return nil }

type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}
func (nopLogger) Debug(string, ...any) {}
