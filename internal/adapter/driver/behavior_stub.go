package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type StubBehavior struct{}

func NewStubBehavior() *StubBehavior { return &StubBehavior{} }

func (s *StubBehavior) RunSocialAction(_ context.Context, network, action, serial string, _ map[string]any) (port.BehaviorJob, error) {
	return port.BehaviorJob{}, fmt.Errorf("behavior-engine недоступен (stub): %s/%s для %s", network, action, serial)
}

func (s *StubBehavior) GetJob(context.Context, string) (port.BehaviorJob, error) {
	return port.BehaviorJob{}, fmt.Errorf("behavior-engine stub")
}

func (s *StubBehavior) WaitJob(context.Context, string, time.Duration) (port.BehaviorJob, error) {
	return port.BehaviorJob{}, fmt.Errorf("behavior-engine stub")
}

func (s *StubBehavior) Ping(context.Context) error { return fmt.Errorf("behavior-engine stub") }

var _ port.BehaviorClient = (*StubBehavior)(nil)
