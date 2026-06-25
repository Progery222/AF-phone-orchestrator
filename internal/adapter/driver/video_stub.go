package driver

import (
	"context"

	"github.com/google/uuid"

	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type StubVideo struct{}

func NewStubVideo() *StubVideo { return &StubVideo{} }

func (s *StubVideo) CreateFromScreenshots(ctx context.Context, serial string, keys []string, audioKey, overlay string, profile port.VideoOutputProfile) (port.VideoJob, error) {
	return port.VideoJob{
		ID: uuid.NewString(), Serial: serial, Kind: "JOB_KIND_SCREENSHOTS",
		Status: "JOB_STATUS_PENDING", InputKeys: keys,
	}, nil
}

func (s *StubVideo) GenerateAI(ctx context.Context, serial, prompt, provider string, durationSec float64, profile port.VideoOutputProfile) (port.VideoJob, error) {
	return port.VideoJob{
		ID: uuid.NewString(), Serial: serial, Kind: "JOB_KIND_AI",
		Status: "JOB_STATUS_PENDING", Provider: "stub",
	}, nil
}

func (s *StubVideo) EditVideo(ctx context.Context, serial, sourceKey string, ops []port.VideoEditOp) (port.VideoJob, error) {
	return port.VideoJob{
		ID: uuid.NewString(), Serial: serial, Kind: "JOB_KIND_EDIT",
		Status: "JOB_STATUS_PENDING", InputKeys: []string{sourceKey},
	}, nil
}

func (s *StubVideo) GetJob(ctx context.Context, id string) (port.VideoJob, error) {
	return port.VideoJob{
		ID: id, Status: "JOB_STATUS_READY", OutputKey: "videos/stub/" + id + ".mp4",
	}, nil
}

func (s *StubVideo) DeleteVideo(ctx context.Context, id string) error { return nil }

func (s *StubVideo) Ping(context.Context) error { return nil }

var _ port.VideoClient = (*StubVideo)(nil)
