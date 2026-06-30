package driver

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

// StubExecutor — заглушка phone-action-executor для локальных тестов без gRPC.
type StubExecutor struct{}

func NewStubExecutor() *StubExecutor { return &StubExecutor{} }

func (s *StubExecutor) ExecutePlan(_ context.Context, serial string, steps []domain.SolutionStep) error {
	_ = serial
	_ = steps
	return nil
}

func (s *StubExecutor) Tap(_ context.Context, serial string, x, y int32) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{
		Action: "tap", Status: "ok", Message: "stub",
	}, nil
}

func (s *StubExecutor) Swipe(_ context.Context, serial string, x0, y0, x1, y1 int32) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{
		Action: "swipe", Status: "ok", Message: "stub",
	}, nil
}

func (s *StubExecutor) TypeText(_ context.Context, serial string, text string, _ bool) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{
		Action: "type_text", Status: "ok", Message: "stub",
	}, nil
}

func (s *StubExecutor) Key(_ context.Context, serial string, key string) (domain.ExecutorActionResult, error) {
	return domain.ExecutorActionResult{
		Action: "key", Status: "ok", Message: key + " stub",
	}, nil
}

func (s *StubExecutor) LaunchPackage(_ context.Context, serial, packageName string) error {
	_ = serial
	_ = packageName
	return nil
}

func (s *StubExecutor) ForceStopPackage(_ context.Context, serial, packageName string) error {
	_ = serial
	_ = packageName
	return nil
}

func (s *StubExecutor) Ping(context.Context) error { return nil }

var _ port.ExecutorClient = (*StubExecutor)(nil)
