package driver

import (
	"context"
	"fmt"
	"strings"
	"time"

	videov1 "github.com/mobilefarm/af/video-generator/gen/video/v1"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
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
	job, err := c.client.CreateFromScreenshots(ctx, &videov1.CreateFromScreenshotsRequest{
		Serial:          serial,
		ScreenshotKeys:  keys,
		AudioKey:        audioKey,
		OverlayText:     overlay,
		Profile:         toProtoProfile(profile),
	})
	if err != nil {
		return port.VideoJob{}, err
	}
	return fromProtoJob(job), nil
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
	specs := make([]*videov1.EditOperationSpec, 0, len(ops))
	for _, op := range ops {
		specs = append(specs, &videov1.EditOperationSpec{
			Op:       parseEditOp(op.Op),
			TrimSec:  op.TrimSec,
			Crf:      int32(op.CRF),
			AudioKey: op.AudioKey,
			Scale:    toProtoProfile(op.Scale),
		})
	}
	job, err := c.client.EditVideo(ctx, &videov1.EditVideoRequest{
		Serial:     serial,
		SourceKey:  sourceKey,
		Operations: specs,
	})
	if err != nil {
		return port.VideoJob{}, err
	}
	return fromProtoJob(job), nil
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
		FrameSec:    p.FrameSec,
	}
}

func parseEditOp(raw string) videov1.EditOperation {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "trim", "edit_operation_trim":
		return videov1.EditOperation_EDIT_OPERATION_TRIM
	case "compress", "edit_operation_compress":
		return videov1.EditOperation_EDIT_OPERATION_COMPRESS
	case "scale", "edit_operation_scale":
		return videov1.EditOperation_EDIT_OPERATION_SCALE
	case "mux_audio", "edit_operation_mux_audio":
		return videov1.EditOperation_EDIT_OPERATION_MUX_AUDIO
	default:
		return videov1.EditOperation_EDIT_OPERATION_UNSPECIFIED
	}
}

var _ port.VideoClient = (*VideoGRPC)(nil)
