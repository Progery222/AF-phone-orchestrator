package port

import (
	"context"
	"time"
)

type BehaviorJob struct {
	ID     string                 `json:"id"`
	Status string                 `json:"status"`
	Error  string                 `json:"error,omitempty"`
	Result map[string]interface{} `json:"result,omitempty"`
}

type BehaviorClient interface {
	RunSocialAction(ctx context.Context, network, action, serial string, body map[string]any) (BehaviorJob, error)
	GetJob(ctx context.Context, jobID string) (BehaviorJob, error)
	WaitJob(ctx context.Context, jobID string, timeout time.Duration) (BehaviorJob, error)
	Ping(ctx context.Context) error
}
