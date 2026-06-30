package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	videov1 "github.com/mobilefarm/af/video-generator/gen/video/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type VideoGRPC struct {
	client videov1.VideoServiceClient
	conn   *grpc.ClientConn
}

func NewVideoGRPC(cfg config.Config) (*VideoGRPC, func(), error) {
	conn, err := grpc.NewClient(cfg.VideoGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return &VideoGRPC{
		client: videov1.NewVideoServiceClient(conn),
		conn:   conn,
	}, func() { _ = conn.Close() }, nil
}

func (c *VideoGRPC) CreateFromScreenshots(ctx context.Context, serial string, keys []string, audioKey, overlay string, profile port.VideoOutputProfile) (port.VideoJob, error) {
	return port.VideoJob{}, fmt.Errorf("video-generator grpc CreateFromScreenshots is unsupported by current video API")
}

func (c *VideoGRPC) GenerateAI(ctx context.Context, serial, prompt, provider string, durationSec float64, profile port.VideoOutputProfile) (port.VideoJob, error) {
	job, err := c.client.GenerateAI(ctx, &videov1.GenerateAIRequest{
		Serial:      serial,
		Prompt:      prompt,
		Provider:    provider,
		DurationSec: durationSec,
		Profile:     toProtoProfile(profile),
	})
	if err != nil {
		return port.VideoJob{}, err
	}
	return fromProtoJob(job), nil
}

func (c *VideoGRPC) EditVideo(ctx context.Context, serial, sourceKey string, ops []port.VideoEditOp) (port.VideoJob, error) {
	return port.VideoJob{}, fmt.Errorf("video-generator grpc EditVideo is unsupported by current video API")
}

func (c *VideoGRPC) GetJob(ctx context.Context, id string) (port.VideoJob, error) {
	job, err := c.client.GetJob(ctx, &videov1.GetJobRequest{Id: id})
	if err != nil {
		return port.VideoJob{}, err
	}
	return fromProtoJob(job), nil
}

func (c *VideoGRPC) DeleteVideo(ctx context.Context, id string) error {
	_, err := c.client.DeleteVideo(ctx, &videov1.DeleteVideoRequest{Id: id})
	return err
}

func (c *VideoGRPC) Ping(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("video-generator: нет соединения")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.client.Ping(ctx, &videov1.PingRequest{})
	if err != nil {
		return fmt.Errorf("video-generator: %w", err)
	}
	return nil
}

func fromProtoJob(job *videov1.VideoJob) port.VideoJob {
	if job == nil {
		return port.VideoJob{}
	}
	return port.VideoJob{
		ID:        job.GetId(),
		Serial:    job.GetSerial(),
		Kind:      job.GetKind().String(),
		Status:    job.GetStatus().String(),
		InputKeys: job.GetInputKeys(),
		OutputKey: job.GetOutputKey(),
		Error:     job.GetError(),
		Provider:  job.GetProvider(),
	}
}

func toProtoProfile(p port.VideoOutputProfile) *videov1.OutputProfile {
	if p.Width == 0 && p.Height == 0 && p.DurationSec == 0 && p.FrameSec == 0 {
		return nil
	}
	return &videov1.OutputProfile{
		Width:       int32(p.Width),
		Height:      int32(p.Height),
		DurationSec: p.DurationSec,
	}
}

var _ port.VideoClient = (*VideoGRPC)(nil)
