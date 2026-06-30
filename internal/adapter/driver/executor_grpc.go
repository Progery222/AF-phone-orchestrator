package driver

import (
	"context"
	"fmt"
	"time"

	executorv1 "github.com/mobilefarm/af/phone-action-executor/gen/executor/v1"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type ExecutorGRPC struct {
	client executorv1.ExecutorServiceClient
	conn   *grpc.ClientConn
}

func NewExecutorGRPC(cfg config.Config) (*ExecutorGRPC, func(), error) {
	conn, err := grpc.NewClient(cfg.ExecutorGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return &ExecutorGRPC{
		client: executorv1.NewExecutorServiceClient(conn),
		conn:   conn,
	}, func() { _ = conn.Close() }, nil
}

func (e *ExecutorGRPC) Tap(ctx context.Context, serial string, x, y int32) (domain.ExecutorActionResult, error) {
	res, err := e.client.Tap(ctx, &executorv1.TapRequest{
		IdempotencyKey: idempotencyKey(serial, "tap"),
		Serial:         serial,
		X:              x,
		Y:              y,
		Params:         &executorv1.GestureParams{Fast: true},
	})
	if err != nil {
		return domain.ExecutorActionResult{}, err
	}
	return fromProtoResult(res), resultError(res)
}

func (e *ExecutorGRPC) Swipe(ctx context.Context, serial string, x0, y0, x1, y1 int32) (domain.ExecutorActionResult, error) {
	res, err := e.client.Swipe(ctx, &executorv1.SwipeRequest{
		IdempotencyKey: idempotencyKey(serial, "swipe"),
		Serial:         serial,
		X0:             x0,
		Y0:             y0,
		X1:             x1,
		Y1:             y1,
		Params: &executorv1.GestureParams{
			DurationMs: 750,
			Bezier:     true,
		},
	})
	if err != nil {
		return domain.ExecutorActionResult{}, err
	}
	return fromProtoResult(res), resultError(res)
}

func (e *ExecutorGRPC) TypeText(ctx context.Context, serial string, text string, typos bool) (domain.ExecutorActionResult, error) {
	res, err := e.client.TypeText(ctx, &executorv1.TypeTextRequest{
		IdempotencyKey: idempotencyKey(serial, "type"),
		Serial:         serial,
		Text:           text,
		Typos:          typos,
		Lang:           "auto",
	})
	if err != nil {
		return domain.ExecutorActionResult{}, err
	}
	return fromProtoResult(res), resultError(res)
}

func (e *ExecutorGRPC) Key(ctx context.Context, serial string, key string) (domain.ExecutorActionResult, error) {
	res, err := e.client.Key(ctx, &executorv1.KeyRequest{
		IdempotencyKey: idempotencyKey(serial, "key"),
		Serial:         serial,
		Key:            key,
	})
	if err != nil {
		return domain.ExecutorActionResult{}, err
	}
	return fromProtoResult(res), resultError(res)
}

func (e *ExecutorGRPC) LaunchPackage(ctx context.Context, serial, packageName string) error {
	res, err := e.client.LaunchApp(ctx, &executorv1.LaunchAppRequest{
		IdempotencyKey: idempotencyKey(serial, "launch"),
		Serial:         serial,
		PackageName:    packageName,
	})
	if err != nil {
		return err
	}
	return resultError(res)
}

func (e *ExecutorGRPC) ForceStopPackage(ctx context.Context, serial, packageName string) error {
	res, err := e.client.ForceStopApp(ctx, &executorv1.ForceStopAppRequest{
		IdempotencyKey: idempotencyKey(serial, "force-stop"),
		Serial:         serial,
		PackageName:    packageName,
	})
	if err != nil {
		return err
	}
	return resultError(res)
}

func (e *ExecutorGRPC) ExecutePlan(ctx context.Context, serial string, steps []domain.SolutionStep) error {
	actions := make([]*executorv1.Action, 0, len(steps))
	for _, step := range steps {
		action, ok := stepToAction(step)
		if !ok {
			continue
		}
		actions = append(actions, action)
	}
	if len(actions) == 0 {
		return nil
	}
	resp, err := e.client.Execute(ctx, &executorv1.ExecuteRequest{
		Serial:  serial,
		Actions: actions,
		Options: &executorv1.ExecuteOptions{
			IdempotencyKey: idempotencyKey(serial, "plan"),
			TimeoutSec:     60,
		},
	})
	if err != nil {
		return err
	}
	if resp.GetStatus() != "ok" {
		return fmt.Errorf("executor plan status %s", resp.GetStatus())
	}
	return nil
}

func (e *ExecutorGRPC) Ping(ctx context.Context) error {
	if e.conn == nil {
		return fmt.Errorf("executor: нет соединения")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		switch e.conn.GetState() {
		case connectivity.Ready, connectivity.Idle:
			return nil
		case connectivity.Shutdown:
			return fmt.Errorf("executor: соединение закрыто")
		default:
			if !e.conn.WaitForStateChange(ctx, e.conn.GetState()) {
				return fmt.Errorf("executor: недоступен")
			}
		}
	}
}

func stepToAction(step domain.SolutionStep) (*executorv1.Action, bool) {
	switch step.Type {
	case "tap":
		return &executorv1.Action{
			Payload: &executorv1.Action_Tap{
				Tap: &executorv1.TapAction{
					X:      int32(step.X),
					Y:      int32(step.Y),
					Params: &executorv1.GestureParams{Fast: true},
				},
			},
		}, true
	case "wait":
		sec := step.Sec
		if sec <= 0 {
			sec = 1
		}
		return &executorv1.Action{
			Payload: &executorv1.Action_Wait{
				Wait: &executorv1.WaitAction{DurationSec: sec, Mode: "idle"},
			},
		}, true
	case "back":
		return &executorv1.Action{
			Payload: &executorv1.Action_Key{
				Key: &executorv1.KeyAction{Key: "back"},
			},
		}, true
	default:
		return nil, false
	}
}

func fromProtoResult(res *executorv1.ActionResult) domain.ExecutorActionResult {
	if res == nil {
		return domain.ExecutorActionResult{}
	}
	return domain.ExecutorActionResult{
		Action:     res.GetAction(),
		Status:     res.GetStatus(),
		Message:    res.GetMessage(),
		DurationMS: res.GetDurationMs(),
	}
}

func resultError(res *executorv1.ActionResult) error {
	if res == nil {
		return fmt.Errorf("пустой ответ executor")
	}
	if res.GetStatus() != "ok" {
		return fmt.Errorf("executor %s: %s", res.GetAction(), res.GetMessage())
	}
	return nil
}

func idempotencyKey(serial, kind string) string {
	return fmt.Sprintf("orch-%s-%s-%d", serial, kind, time.Now().UnixNano())
}

var _ port.ExecutorClient = (*ExecutorGRPC)(nil)
