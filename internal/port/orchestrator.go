package port

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

type ObserverClient interface {
	CaptureScreen(ctx context.Context, serial string) (domain.ScreenCapture, error)
	DumpUI(ctx context.Context, serial string) (domain.UIDump, error)
	Ping(ctx context.Context) error
}

type RecoveryClient interface {
	Solve(ctx context.Context, req domain.RecoverySolveRequest) (domain.RecoverySolveResponse, error)
	ReportOutcome(ctx context.Context, req domain.RecoveryOutcomeRequest) error
	Ping(ctx context.Context) error
}

type ExecutorClient interface {
	ExecutePlan(ctx context.Context, serial string, steps []domain.SolutionStep) error
	Tap(ctx context.Context, serial string, x, y int32) (domain.ExecutorActionResult, error)
	Swipe(ctx context.Context, serial string, x0, y0, x1, y1 int32) (domain.ExecutorActionResult, error)
	Key(ctx context.Context, serial string, key string) (domain.ExecutorActionResult, error)
	Ping(ctx context.Context) error
}
