package service

import (
	"context"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type PhoneService struct {
	store     port.PhoneStore
	allowlist map[string]struct{}
}

func NewPhoneService(store port.PhoneStore, allowlist ...[]string) *PhoneService {
	s := &PhoneService{store: store}
	if len(allowlist) > 0 {
		s.allowlist = make(map[string]struct{}, len(allowlist[0]))
		for _, serial := range allowlist[0] {
			if serial != "" {
				s.allowlist[serial] = struct{}{}
			}
		}
	}
	return s
}

func (s *PhoneService) ListPhones(ctx context.Context) ([]domain.Phone, domain.PhoneStats, error) {
	phones, err := s.store.ListAll(ctx)
	if err != nil {
		return nil, domain.PhoneStats{}, err
	}
	phones = s.filterAllowed(phones)
	return phones, statsFor(phones), nil
}

func (s *PhoneService) GetPhone(ctx context.Context, serial string) (domain.Phone, error) {
	if !s.IsAllowed(serial) {
		return domain.Phone{}, domain.ErrPhoneNotFound
	}
	return s.store.Get(ctx, serial)
}

func (s *PhoneService) AddPhone(ctx context.Context, req domain.AddPhoneRequest) (domain.Phone, error) {
	serial := req.Serial
	if serial == "" {
		return domain.Phone{}, domain.ErrInvalidSerial
	}
	if !s.IsAllowed(serial) {
		return domain.Phone{}, domain.ErrPhoneNotFound
	}
	if _, err := s.store.Get(ctx, serial); err == nil {
		return domain.Phone{}, domain.ErrPhoneAlreadyExists
	} else if err != domain.ErrPhoneNotFound {
		return domain.Phone{}, err
	}
	now := time.Now()
	phone := domain.Phone{
		Serial:        serial,
		State:         domain.StateNew,
		AdbPort:       5555,
		WifiSSID:      req.WifiSSID,
		WiFiPass:      req.WiFiPass,
		ProxyIP:       req.ProxyIP,
		ProxyPort:     req.ProxyPort,
		ProxyUser:     req.ProxyUser,
		ProxyPass:     req.ProxyPass,
		ProvisionApps: req.Apps,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.store.Save(ctx, phone); err != nil {
		return domain.Phone{}, err
	}
	return phone, nil
}

func (s *PhoneService) RemovePhone(ctx context.Context, serial string) error {
	if !s.IsAllowed(serial) {
		return domain.ErrPhoneNotFound
	}
	phone, err := s.store.Get(ctx, serial)
	if err != nil {
		return err
	}
	now := time.Now()
	phone.State = domain.StateRetired
	phone.RetiredAt = &now
	return s.store.Update(ctx, phone)
}

func (s *PhoneService) IsAllowed(serial string) bool {
	if len(s.allowlist) == 0 {
		return true
	}
	_, ok := s.allowlist[serial]
	return ok
}

func (s *PhoneService) filterAllowed(phones []domain.Phone) []domain.Phone {
	if len(s.allowlist) == 0 {
		return phones
	}
	out := phones[:0]
	for _, phone := range phones {
		if s.IsAllowed(phone.Serial) {
			out = append(out, phone)
		}
	}
	return out
}

func statsFor(phones []domain.Phone) domain.PhoneStats {
	var stats domain.PhoneStats
	for _, phone := range phones {
		if phone.State == domain.StateRetired {
			continue
		}
		stats.Total++
		switch phone.State {
		case domain.StateWorking:
			stats.Working++
		case domain.StatePaused:
			stats.Paused++
		case domain.StateError:
			stats.Error++
		case domain.StateNew, domain.StateWifiSetup, domain.StateProxySetup, domain.StateAppsInstall, domain.StateAuth, domain.StateReady:
			stats.SettingUp++
		}
	}
	return stats
}
