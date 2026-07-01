package service

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

var (
	ErrStandSeqNotFound       = errors.New("stand sequence number was not found on screen")
	ErrStandSeqAmbiguous      = errors.New("multiple stand sequence numbers found on screen")
	ErrStandSeqOCRUnavailable = errors.New("stand sequence OCR/VLM is unavailable")
)

type standSeqScreenDetector interface {
	DetectState(ctx context.Context, serial string) (domain.ScreenDetection, error)
}

type StandSeqSyncService struct {
	phones   *PhoneService
	observer port.ObserverClient
	executor port.ExecutorClient
	homeWait time.Duration
}

func NewStandSeqSyncService(phones *PhoneService, observer port.ObserverClient, executor port.ExecutorClient) *StandSeqSyncService {
	return &StandSeqSyncService{
		phones:   phones,
		observer: observer,
		executor: executor,
		homeWait: 1200 * time.Millisecond,
	}
}

func (s *StandSeqSyncService) SyncFromHome(ctx context.Context, serial string) (domain.StandSeqSyncResult, error) {
	if s == nil || s.phones == nil {
		return domain.StandSeqSyncResult{}, domain.ErrStoreUnavailable
	}
	if s.executor == nil {
		return domain.StandSeqSyncResult{}, domain.ErrExecutorUnavailable
	}
	if s.observer == nil {
		return domain.StandSeqSyncResult{}, domain.ErrObserverUnavailable
	}
	if _, err := s.phones.GetPhone(ctx, serial); err != nil {
		return domain.StandSeqSyncResult{}, err
	}

	if _, err := s.executor.Key(ctx, serial, "home"); err != nil {
		return domain.StandSeqSyncResult{}, fmt.Errorf("press home: %w", err)
	}
	if err := sleepContext(ctx, s.homeWait); err != nil {
		return domain.StandSeqSyncResult{}, err
	}

	ui, uiErr := s.observer.DumpUI(ctx, serial)
	if uiErr == nil {
		if seq, err := extractStandSeqFromUIDump(ui.XMLDump); err == nil {
			return s.save(ctx, serial, seq, "ui_dump", 1, "", "")
		} else if errors.Is(err, ErrStandSeqAmbiguous) {
			return domain.StandSeqSyncResult{}, err
		}
	}

	detector, ok := s.observer.(standSeqScreenDetector)
	if !ok {
		if uiErr != nil {
			return domain.StandSeqSyncResult{}, fmt.Errorf("%w: ui dump failed: %v", ErrStandSeqOCRUnavailable, uiErr)
		}
		return domain.StandSeqSyncResult{}, ErrStandSeqOCRUnavailable
	}

	detection, err := detector.DetectState(ctx, serial)
	if err != nil {
		return domain.StandSeqSyncResult{}, fmt.Errorf("%w: %v", ErrStandSeqOCRUnavailable, err)
	}
	seq, err := extractStandSeqFromTexts(append(detection.Elements, detection.Description))
	if err != nil {
		return domain.StandSeqSyncResult{}, err
	}
	confidence := detection.Confidence
	if confidence <= 0 {
		confidence = 0.6
	}
	return s.save(ctx, serial, seq, "vlm_screenshot", confidence, detection.MinioKey, detection.ScreenshotURL)
}

func (s *StandSeqSyncService) save(ctx context.Context, serial string, seq int16, source string, confidence float64, minioKey, screenshotURL string) (domain.StandSeqSyncResult, error) {
	if _, err := s.phones.SetStandSeqNumber(ctx, serial, &seq); err != nil {
		return domain.StandSeqSyncResult{}, err
	}
	return domain.StandSeqSyncResult{
		Serial:         serial,
		StandSeqNumber: seq,
		Source:         source,
		Confidence:     confidence,
		MinioKey:       minioKey,
		ScreenshotURL:  screenshotURL,
	}, nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func extractStandSeqFromUIDump(xmlDump string) (int16, error) {
	return extractStandSeqFromTexts(visibleTextsFromUIDump(xmlDump))
}

func visibleTextsFromUIDump(xmlDump string) []string {
	decoder := xml.NewDecoder(strings.NewReader(xmlDump))
	values := make([]string, 0)
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "text", "content-desc", "hint":
				value := strings.TrimSpace(attr.Value)
				if value != "" {
					values = append(values, value)
				}
			}
		}
	}
	return values
}

var (
	standSeqLabeledRE    = regexp.MustCompile(`(?i)(?:^|[\s#№:;,\-])(?:stand|стенд|phone|телефон|device|устройство|№|#)\s*[:#№-]?\s*([0-9]{1,5})(?:\D|$)`)
	standSeqStandaloneRE = regexp.MustCompile(`^\s*[#№]?\s*([0-9]{1,5})\s*$`)
)

func extractStandSeqFromTexts(texts []string) (int16, error) {
	candidates := map[int16]struct{}{}
	for _, text := range texts {
		for _, part := range splitStandSeqText(text) {
			for _, match := range standSeqLabeledRE.FindAllStringSubmatch(part, -1) {
				addStandSeqCandidate(candidates, match[1])
			}
			if match := standSeqStandaloneRE.FindStringSubmatch(part); match != nil {
				addStandSeqCandidate(candidates, match[1])
			}
		}
	}

	switch len(candidates) {
	case 0:
		return 0, ErrStandSeqNotFound
	case 1:
		for seq := range candidates {
			return seq, nil
		}
	}
	return 0, ErrStandSeqAmbiguous
}

func splitStandSeqText(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == '|' || r == '•'
	})
}

func addStandSeqCandidate(candidates map[int16]struct{}, raw string) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > 32767 {
		return
	}
	candidates[int16(n)] = struct{}{}
}
