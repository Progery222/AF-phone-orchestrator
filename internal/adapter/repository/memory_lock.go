package repository

import (
	"context"
	"sync"
	"time"
)

type MemoryPhoneLock struct {
	mu    sync.Mutex
	locks map[string]time.Time
}

func NewMemoryPhoneLock() *MemoryPhoneLock {
	return &MemoryPhoneLock{locks: make(map[string]time.Time)}
}

func (m *MemoryPhoneLock) TryLock(_ context.Context, serial string, ttlSec int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if exp, ok := m.locks[serial]; ok && exp.After(now) {
		return false, nil
	}
	m.locks[serial] = now.Add(time.Duration(ttlSec) * time.Second)
	return true, nil
}

func (m *MemoryPhoneLock) Unlock(_ context.Context, serial string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.locks, serial)
	return nil
}
