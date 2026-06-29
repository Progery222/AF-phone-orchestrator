package service

import (
	"context"
	"testing"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func TestOrchestratorStateMachine_NewToWorking(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	now := time.Now()
	_ = store.Save(context.Background(), domain.Phone{
		Serial: "p1", State: domain.StateNew, CreatedAt: now, UpdatedAt: now, AdbPort: 5555,
	})

	orch := NewOrchestratorService(
		store, repository.NewMemoryPhoneLock(),
		driver.NewStubConnector(), driver.NewStubProvisioner(),
		NewRecoveryFlowService(stubObserver{}, stubRecovery{}, stubExecutor{}, nopLogger{}),
		repository.NewNoopEventPublisher(), nopLogger{}, 30, 1,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go orch.Run(ctx)

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		p, err := store.Get(ctx, "p1")
		if err == nil && p.State == domain.StateWorking {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("phone did not reach working state")
}

func TestPhoneService_AddDuplicate(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	svc := NewPhoneService(store)
	ctx := context.Background()
	if _, err := svc.AddPhone(ctx, domain.AddPhoneRequest{Serial: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddPhone(ctx, domain.AddPhoneRequest{Serial: "x"}); err != domain.ErrPhoneAlreadyExists {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestPhoneService_SetStandSeqNumber(t *testing.T) {
	store := repository.NewMemoryPhoneStore()
	svc := NewPhoneService(store)
	ctx := context.Background()
	seq := int16(181)

	phone, err := svc.AddPhone(ctx, domain.AddPhoneRequest{Serial: "phone-181", StandSeqNumber: &seq})
	if err != nil {
		t.Fatal(err)
	}
	if phone.StandSeqNumber == nil || *phone.StandSeqNumber != 181 {
		t.Fatalf("stand seq on add = %v, want 181", phone.StandSeqNumber)
	}

	next := int16(166)
	phone, err = svc.SetStandSeqNumber(ctx, "phone-181", &next)
	if err != nil {
		t.Fatal(err)
	}
	if phone.StandSeqNumber == nil || *phone.StandSeqNumber != 166 {
		t.Fatalf("stand seq after update = %v, want 166", phone.StandSeqNumber)
	}
}
