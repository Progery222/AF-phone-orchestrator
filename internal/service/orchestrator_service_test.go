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
