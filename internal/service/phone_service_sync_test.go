package service

import (
	"context"
	"testing"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func TestPhoneServiceSyncLiveDevicesAddsAndMarksMissing(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryPhoneStore()
	svc := NewPhoneService(store)
	now := time.Now()

	if err := store.Save(ctx, domain.Phone{
		Serial: "missing-phone", State: domain.StateWorking, CreatedAt: now, UpdatedAt: now, AdbPort: 5555,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.SyncLiveDevices(ctx, []domain.Phone{{
		Serial: "10.16.181.150:5555", State: domain.StateWorking, Model: "SM_A266B", CurrentIP: "10.16.181.150", AdbPort: 5555,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Missing != 1 {
		t.Fatalf("sync result = %+v, want added=1 missing=1", result)
	}

	added, err := store.Get(ctx, "10.16.181.150:5555")
	if err != nil {
		t.Fatal(err)
	}
	if added.State != domain.StateWorking || added.Model != "SM_A266B" || added.CurrentIP != "10.16.181.150" {
		t.Fatalf("added phone = %+v", added)
	}

	missing, err := store.Get(ctx, "missing-phone")
	if err != nil {
		t.Fatal(err)
	}
	if missing.State != domain.StateError || missing.LastError != adbMissingError {
		t.Fatalf("missing phone = %+v", missing)
	}
}

func TestPhoneServiceSyncLiveDevicesRespectsAllowlist(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryPhoneStore()
	svc := NewPhoneService(store, []string{"allowed-phone"})

	result, err := svc.SyncLiveDevices(ctx, []domain.Phone{
		{Serial: "allowed-phone", State: domain.StateWorking},
		{Serial: "ignored-phone", State: domain.StateWorking},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 {
		t.Fatalf("sync result = %+v, want added=1", result)
	}
	if _, err := store.Get(ctx, "allowed-phone"); err != nil {
		t.Fatalf("allowed phone was not added: %v", err)
	}
	if _, err := store.Get(ctx, "ignored-phone"); err != domain.ErrPhoneNotFound {
		t.Fatalf("ignored phone err = %v, want ErrPhoneNotFound", err)
	}
}

func TestPhoneServiceSyncLiveDevicesRestoresADBMissingError(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryPhoneStore()
	svc := NewPhoneService(store)
	now := time.Now()

	if err := store.Save(ctx, domain.Phone{
		Serial: "back-online", State: domain.StateError, LastError: adbMissingError,
		CreatedAt: now, UpdatedAt: now, AdbPort: 5555,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.SyncLiveDevices(ctx, []domain.Phone{{Serial: "back-online", State: domain.StateWorking}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 1 || result.Missing != 0 {
		t.Fatalf("sync result = %+v, want updated=1 missing=0", result)
	}

	phone, err := store.Get(ctx, "back-online")
	if err != nil {
		t.Fatal(err)
	}
	if phone.State != domain.StateWorking || phone.LastError != "" {
		t.Fatalf("restored phone = %+v", phone)
	}
}
