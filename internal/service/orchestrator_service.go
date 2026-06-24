package service

import (
	"context"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type OrchestratorService struct {
	store      port.PhoneStore
	lock       port.PhoneLock
	connector  port.ConnectorClient
	provision  port.ProvisionClient
	recovery   *RecoveryFlowService
	events     port.EventPublisher
	log        port.Logger
	lockTTL    int
	tickSec    int
}

func NewOrchestratorService(
	store port.PhoneStore,
	lock port.PhoneLock,
	connector port.ConnectorClient,
	provision port.ProvisionClient,
	recovery *RecoveryFlowService,
	events port.EventPublisher,
	log port.Logger,
	lockTTL, tickSec int,
) *OrchestratorService {
	return &OrchestratorService{
		store: store, lock: lock, connector: connector, provision: provision,
		recovery: recovery, events: events, log: log, lockTTL: lockTTL, tickSec: tickSec,
	}
}

func (o *OrchestratorService) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(o.tickSec) * time.Second)
	defer ticker.Stop()
	o.log.Info("orchestrator loop started", "service", "phone-orchestrator", "tick_sec", o.tickSec)
	o.processAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.processAll(ctx)
		}
	}
}

func (o *OrchestratorService) processAll(ctx context.Context) {
	phones, err := o.store.ListActive(ctx)
	if err != nil {
		o.log.Error("list active phones", "error", err)
		return
	}
	for _, phone := range phones {
		p := phone
		go o.processPhone(ctx, p)
	}
}

func (o *OrchestratorService) processPhone(ctx context.Context, phone domain.Phone) {
	ok, err := o.lock.TryLock(ctx, phone.Serial, o.lockTTL)
	if err != nil || !ok {
		return
	}
	defer o.lock.Unlock(ctx, phone.Serial)

	phone, err = o.store.Get(ctx, phone.Serial)
	if err != nil {
		return
	}

	start := time.Now()
	var stepErr error

	switch phone.State {
	case domain.StateNew:
		stepErr = o.connector.Connect(ctx, phone.Serial)
		if stepErr == nil {
			o.transition(ctx, &phone, domain.StateWifiSetup, stepErr, start)
		}
	case domain.StateWifiSetup, domain.StateProxySetup, domain.StateAppsInstall, domain.StateAuth:
		next, err := o.provision.AdvanceSetup(ctx, phone)
		if err != nil {
			stepErr = err
		} else if next != phone.State {
			o.transition(ctx, &phone, next, nil, start)
		}
	case domain.StateReady:
		o.transition(ctx, &phone, domain.StateWorking, nil, start)
	case domain.StateWorking:
		o.heartbeat(ctx, &phone)
	case domain.StatePaused:
		o.handlePaused(ctx, &phone)
	case domain.StateError:
		// ждём ручного вмешательства
	}
	if stepErr != nil {
		phone.LastError = stepErr.Error()
		_ = o.store.Update(ctx, phone)
	}
}

func (o *OrchestratorService) handlePaused(ctx context.Context, phone *domain.Phone) {
	if phone.RecoveryInProgress {
		return
	}
	phone.RecoveryInProgress = true
	_ = o.store.Update(ctx, *phone)
	defer func() {
		phone.RecoveryInProgress = false
		_ = o.store.Update(ctx, *phone)
	}()

	scenario := phone.LastError
	if scenario == "" {
		scenario = "восстановление зависшего экрана"
	}

	plan, err := o.recovery.RunRecovery(ctx, phone.Serial, scenario, scenario)
	if err != nil {
		phone.LastError = err.Error()
		_ = o.store.Update(ctx, *phone)
		o.log.Warn("recovery failed", "serial", phone.Serial, "error", err)
		return
	}

	phone.LastErrorHash = plan.ErrorHash
	phone.LastError = ""
	o.transition(ctx, phone, domain.StateWorking, nil, time.Now())
}

func (o *OrchestratorService) heartbeat(ctx context.Context, phone *domain.Phone) {
	now := time.Now()
	phone.LastHeartbeat = &now
	phone.HeartbeatCount++
	if st, err := o.connector.GetStatus(ctx, phone.Serial); err == nil && st.Model != "" {
		phone.Model = st.Model
	}
	_ = o.store.Update(ctx, *phone)
}

func (o *OrchestratorService) transition(ctx context.Context, phone *domain.Phone, newState domain.PhoneState, err error, started time.Time) {
	old := phone.State
	phone.State = newState
	if newState == domain.StateReady || newState == domain.StateWorking {
		if phone.ReadyAt == nil {
			now := time.Now()
			phone.ReadyAt = &now
		}
	}
	if newState == domain.StateRetired {
		now := time.Now()
		phone.RetiredAt = &now
	}
	errText := ""
	if err != nil {
		errText = err.Error()
		phone.LastError = errText
	}
	_ = o.store.Update(ctx, *phone)
	_ = o.store.LogTransition(ctx, domain.PhoneStateLog{
		Serial: phone.Serial, FromState: old, ToState: newState,
		Step: phone.CurrentStep, Error: errText, DurationMS: int(time.Since(started).Milliseconds()),
	})
	_ = o.events.PublishPhoneStateChanged(ctx, domain.PhoneStateEvent{
		Serial: phone.Serial, OldState: string(old), NewState: string(newState), Error: errText,
	})
	o.log.Info("phone state changed", "serial", phone.Serial, "from", old, "to", newState)
}

// PausePhone переводит телефон в PAUSED (сценарий 3).
func (o *OrchestratorService) PausePhone(ctx context.Context, serial, reason string) error {
	phone, err := o.store.Get(ctx, serial)
	if err != nil {
		return err
	}
	phone.LastError = reason
	o.transition(ctx, &phone, domain.StatePaused, nil, time.Now())
	return nil
}

func (o *OrchestratorService) ResumePhone(ctx context.Context, serial string) error {
	phone, err := o.store.Get(ctx, serial)
	if err != nil {
		return err
	}
	o.transition(ctx, &phone, domain.StateWorking, nil, time.Now())
	return nil
}

func (o *OrchestratorService) MarkError(ctx context.Context, serial, reason string) error {
	phone, err := o.store.Get(ctx, serial)
	if err != nil {
		return err
	}
	phone.LastError = reason
	o.transition(ctx, &phone, domain.StateError, nil, time.Now())
	return nil
}

func (o *OrchestratorService) ReprovisionPhone(ctx context.Context, serial string) error {
	phone, err := o.store.Get(ctx, serial)
	if err != nil {
		return err
	}
	phone.CurrentStep = 0
	o.transition(ctx, &phone, domain.StateNew, nil, time.Now())
	return nil
}
