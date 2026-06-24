package driver

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type StubExecutor struct{}

func NewStubExecutor() *StubExecutor { return &StubExecutor{} }

func (s *StubExecutor) ExecutePlan(_ context.Context, serial string, steps []domain.SolutionStep) error {
	_ = serial
	_ = steps
	return nil
}

func (s *StubExecutor) Ping(context.Context) error { return nil }

var _ port.ExecutorClient = (*StubExecutor)(nil)
