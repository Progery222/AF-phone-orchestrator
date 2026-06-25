package service

import (
	"context"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type PhoneService struct {
	store port.PhoneStore
}

func NewPhoneService(store port.PhoneStore) *PhoneService {
	return &PhoneService{store: store}
}

func (s *PhoneService) ListPhones(ctx context.Context) ([]domain.Phone, domain.PhoneStats, error) {
	phones, err := s.store.ListActive(ctx)
	if err != nil {
		return nil, domain.PhoneStats{}, err
	}
	stats, err := s.store.Stats(ctx)
	if err != nil {
		return nil, domain.PhoneStats{}, err
	}
	return phones, stats, nil
}

func (s *PhoneService) GetPhone(ctx context.Context, serial string) (domain.Phone, error) {
	return s.store.Get(ctx, serial)
}

func (s *PhoneService) AddPhone(ctx context.Context, serial string) (domain.Phone, error) {
	if serial == "" {
		return domain.Phone{}, domain.ErrInvalidSerial
	}
	if _, err := s.store.Get(ctx, serial); err == nil {
		return domain.Phone{}, domain.ErrPhoneAlreadyExists
	} else if err != domain.ErrPhoneNotFound {
		return domain.Phone{}, err
	}
	now := time.Now()
	phone := domain.Phone{
		Serial:    serial,
		State:     domain.StateNew,
		AdbPort:   5555,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.Save(ctx, phone); err != nil {
		return domain.Phone{}, err
	}
	return phone, nil
}

func (s *PhoneService) RemovePhone(ctx context.Context, serial string) error {
	phone, err := s.store.Get(ctx, serial)
	if err != nil {
		return err
	}
	now := time.Now()
	phone.State = domain.StateRetired
	phone.RetiredAt = &now
	return s.store.Update(ctx, phone)
}
