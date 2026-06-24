package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"github.com/nats-io/nats.go"
)

type RecoveryNATS struct {
	conn         *nats.Conn
	subjectIn    string
	subjectOut   string
	subjectOutcome string
	timeout      time.Duration
}

func NewRecoveryNATS(cfg config.Config) (*RecoveryNATS, func(), error) {
	conn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, nil, err
	}
	return &RecoveryNATS{
		conn:           conn,
		subjectIn:      cfg.NATSSubjectRecoveryIn,
		subjectOut:     cfg.NATSSubjectRecoveryOut,
		subjectOutcome: cfg.NATSSubjectOutcome,
		timeout:        time.Duration(cfg.RecoveryTimeoutSec) * time.Second,
	}, func() { conn.Close() }, nil
}

func (r *RecoveryNATS) Solve(ctx context.Context, req domain.RecoverySolveRequest) (domain.RecoverySolveResponse, error) {
	sub, err := r.conn.SubscribeSync(r.subjectOut)
	if err != nil {
		return domain.RecoverySolveResponse{}, err
	}
	defer sub.Unsubscribe()

	data, err := json.Marshal(req)
	if err != nil {
		return domain.RecoverySolveResponse{}, err
	}
	if err := r.conn.Publish(r.subjectIn, data); err != nil {
		return domain.RecoverySolveResponse{}, err
	}

	deadline := r.timeout
	if d, ok := ctx.Deadline(); ok {
		deadline = time.Until(d)
	}
	msg, err := sub.NextMsg(deadline)
	if err != nil {
		return domain.RecoverySolveResponse{}, fmt.Errorf("%w: %v", domain.ErrRecoveryUnavailable, err)
	}

	var resp domain.RecoverySolveResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return domain.RecoverySolveResponse{}, err
	}
	return resp, nil
}

func (r *RecoveryNATS) ReportOutcome(_ context.Context, req domain.RecoveryOutcomeRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return r.conn.Publish(r.subjectOutcome, data)
}

func (r *RecoveryNATS) Ping(context.Context) error {
	if !r.conn.IsConnected() {
		return nats.ErrConnectionClosed
	}
	return nil
}

var _ port.RecoveryClient = (*RecoveryNATS)(nil)

type StubRecovery struct{}

func NewStubRecovery() *StubRecovery { return &StubRecovery{} }

func (s *StubRecovery) Solve(_ context.Context, _ domain.RecoverySolveRequest) (domain.RecoverySolveResponse, error) {
	return domain.RecoverySolveResponse{
		Success: true,
		Source:  "stub",
		Solution: []domain.SolutionStep{
			{Type: "back"},
			{Type: "wait", Sec: 2},
		},
		ErrorHash:  "stub-hash",
		ScenarioID: "stub-id",
		Message:    "тестовый план",
	}, nil
}

func (s *StubRecovery) ReportOutcome(context.Context, domain.RecoveryOutcomeRequest) error { return nil }
func (s *StubRecovery) Ping(context.Context) error                                         { return nil }

var _ port.RecoveryClient = (*StubRecovery)(nil)
