package port

import "context"

type VideoJob struct {
	ID        string   `json:"id"`
	Serial    string   `json:"serial"`
	Kind      string   `json:"kind"`
	Status    string   `json:"status"`
	InputKeys []string `json:"input_keys,omitempty"`
	OutputKey string   `json:"output_key,omitempty"`
	Error     string   `json:"error,omitempty"`
	Provider  string   `json:"provider,omitempty"`
}

type VideoOutputProfile struct {
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	FrameSec    float64 `json:"frame_sec,omitempty"`
}

type VideoEditOp struct {
	Op       string             `json:"op"`
	TrimSec  float64            `json:"trim_sec,omitempty"`
	CRF      int                `json:"crf,omitempty"`
	AudioKey string             `json:"audio_key,omitempty"`
	Scale    VideoOutputProfile `json:"scale,omitempty"`
}

type VideoClient interface {
	CreateFromScreenshots(ctx context.Context, serial string, keys []string, audioKey, overlay string, profile VideoOutputProfile) (VideoJob, error)
	GenerateAI(ctx context.Context, serial, prompt, provider string, durationSec float64, profile VideoOutputProfile) (VideoJob, error)
	EditVideo(ctx context.Context, serial, sourceKey string, ops []VideoEditOp) (VideoJob, error)
	GetJob(ctx context.Context, id string) (VideoJob, error)
	DeleteVideo(ctx context.Context, id string) error
	Ping(ctx context.Context) error
}
