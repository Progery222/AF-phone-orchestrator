package driver

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

// StubObserver — заглушка phone-observer для локальных тестов без HTTP.
type StubObserver struct{}

func NewStubObserver() *StubObserver { return &StubObserver{} }

func (s *StubObserver) CaptureScreen(_ context.Context, serial string) (domain.ScreenCapture, error) {
	return domain.ScreenCapture{
		Serial:   serial,
		MinioKey: serial + "/test.png",
		Width:    1080,
		Height:   2400,
	}, nil
}

func (s *StubObserver) DumpUI(_ context.Context, serial string) (domain.UIDump, error) {
	return domain.UIDump{
		Serial:  serial,
		XMLDump: `<hierarchy><node text="CONTINUE"/></hierarchy>`,
		Package: "com.test.app",
	}, nil
}

func (s *StubObserver) Ping(context.Context) error { return nil }

var _ port.ObserverClient = (*StubObserver)(nil)
