package repository

import (
	"context"
	"sync"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

type MemoryPhoneStore struct {
	mu     sync.RWMutex
	phones map[string]domain.Phone
	logs   []domain.PhoneStateLog
}

func NewMemoryPhoneStore() *MemoryPhoneStore {
	return &MemoryPhoneStore{phones: make(map[string]domain.Phone)}
}

func (m *MemoryPhoneStore) ListActive(ctx context.Context) ([]domain.Phone, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.Phone
	for _, p := range m.phones {
		if p.State != domain.StateRetired && p.State != domain.StateError {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *MemoryPhoneStore) ListAll(ctx context.Context) ([]domain.Phone, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Phone, 0, len(m.phones))
	for _, p := range m.phones {
		out = append(out, p)
	}
	return out, nil
}

func (m *MemoryPhoneStore) Get(ctx context.Context, serial string) (domain.Phone, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.phones[serial]
	if !ok {
		return domain.Phone{}, domain.ErrPhoneNotFound
	}
	return p, nil
}

func (m *MemoryPhoneStore) Save(ctx context.Context, phone domain.Phone) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.phones[phone.Serial]; ok {
		return domain.ErrPhoneAlreadyExists
	}
	m.phones[phone.Serial] = phone
	return nil
}

func (m *MemoryPhoneStore) Update(ctx context.Context, phone domain.Phone) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.phones[phone.Serial]; !ok {
		return domain.ErrPhoneNotFound
	}
	m.phones[phone.Serial] = phone
	return nil
}

func (m *MemoryPhoneStore) Delete(ctx context.Context, serial string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.phones[serial]; !ok {
		return domain.ErrPhoneNotFound
	}
	delete(m.phones, serial)
	return nil
}

func (m *MemoryPhoneStore) LogTransition(ctx context.Context, log domain.PhoneStateLog) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, log)
	return nil
}

func (m *MemoryPhoneStore) Stats(ctx context.Context) (domain.PhoneStats, error) {
	phones, _ := m.ListAll(ctx)
	var s domain.PhoneStats
	for _, p := range phones {
		if p.State == domain.StateRetired {
			continue
		}
		s.Total++
		switch p.State {
		case domain.StateWorking:
			s.Working++
		case domain.StatePaused:
			s.Paused++
		case domain.StateError:
			s.Error++
		case domain.StateNew, domain.StateWifiSetup, domain.StateProxySetup, domain.StateAppsInstall, domain.StateAuth, domain.StateReady:
			s.SettingUp++
		}
	}
	return s, nil
}

func (m *MemoryPhoneStore) Ping(context.Context) error { return nil }
