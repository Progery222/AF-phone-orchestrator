package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

var feedUITextRe = regexp.MustCompile(`(?:text|content-desc)="([^"]{2,120})"`)

// resolveFeedScreenSize — реальное разрешение с observer (с повторами), иначе из БД / эталон 1080×1920.
func (r *ScenarioRunner) resolveFeedScreenSize(ctx context.Context, serial string, phone domain.Phone) (w, h int32) {
	if r.observer != nil {
		for i := 0; i < 3; i++ {
			if i > 0 {
				_ = sleepCtx(ctx, 800*time.Millisecond)
			}
			reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			cap, err := r.observer.CaptureScreen(reqCtx, serial)
			cancel()
			if err == nil && cap.Width > 0 && cap.Height > 0 {
				return portraitSize(cap.Width, cap.Height)
			}
		}
	}
	if phone.ScreenResX > 0 && phone.ScreenResY > 0 {
		return portraitSize(phone.ScreenResX, phone.ScreenResY)
	}
	return portraitSize(refScreenW, refScreenH)
}

// warmupFeedDeadline — длительность прогрева: duration_sec приоритетнее короткого until от LLM.
func warmupFeedDeadline(now time.Time, durationSec int, until string) time.Time {
	if durationSec <= 0 {
		durationSec = 60
	}
	deadline := now.Add(time.Duration(durationSec) * time.Second)
	if until == "" {
		return deadline
	}
	t, err := time.Parse("15:04", until)
	if err != nil {
		return deadline
	}
	untilTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if !untilTime.After(now) {
		return deadline
	}
	// until как потолок только если даёт окно не короче 30с (иначе это артефакт генерации).
	if untilTime.Sub(now) < 30*time.Second {
		return deadline
	}
	if untilTime.Before(deadline) {
		return untilTime
	}
	return deadline
}

// feedUIMarker — короткая «подпись» ленты из UI dump (автор/описание видео).
func feedUIMarker(xmlDump string) string {
	if xmlDump == "" {
		return ""
	}
	seen := make(map[string]struct{})
	parts := make([]string, 0, 4)
	for _, m := range feedUITextRe.FindAllStringSubmatch(xmlDump, 40) {
		if len(m) < 2 {
			continue
		}
		t := strings.TrimSpace(m[1])
		if t == "" || len(t) < 2 {
			continue
		}
		low := strings.ToLower(t)
		if strings.Contains(low, "follow") || strings.Contains(low, "подпис") ||
			strings.Contains(low, "like") || strings.Contains(low, "comment") {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		parts = append(parts, t)
		if len(parts) >= 3 {
			break
		}
	}
	return strings.Join(parts, " | ")
}

// feedUIMarkerOrFallback — если текстовых узлов нет, слабый отпечаток по структуре XML.
func feedUIMarkerOrFallback(xmlDump string) string {
	if m := feedUIMarker(xmlDump); m != "" {
		return m
	}
	if len(xmlDump) < 80 {
		return ""
	}
	nodes := strings.Count(xmlDump, "<node")
	if nodes == 0 {
		return ""
	}
	return fmt.Sprintf("xml:%d:%d", nodes, len(xmlDump))
}

// feedSwipeOutcome — результат сравнения маркеров до/после свайпа.
// unknown: observer не дал данных — не считать «застряли».
func feedSwipeOutcome(before, after string) (outcome string) {
	if after == "" {
		return "unknown"
	}
	if before == "" {
		return "changed"
	}
	if after == before {
		return "unchanged"
	}
	return "changed"
}

func (r *ScenarioRunner) captureFeedMarker(ctx context.Context, serial string) string {
	marker, _ := r.captureFeedMarkerReliable(ctx, serial, 1)
	return marker
}

// captureFeedMarkerReliable — DumpUI с коротким таймаутом и повторами после свайпа.
func (r *ScenarioRunner) captureFeedMarkerReliable(ctx context.Context, serial string, attempts int) (marker string, ok bool) {
	if r.observer == nil {
		return "", false
	}
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		if i > 0 {
			if err := sleepCtx(ctx, 700*time.Millisecond); err != nil {
				return "", false
			}
		}
		reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		ui, err := r.observer.DumpUI(reqCtx, serial)
		cancel()
		if err != nil {
			continue
		}
		if m := feedUIMarkerOrFallback(ui.XMLDump); m != "" {
			return m, true
		}
	}
	return "", false
}

func (r *ScenarioRunner) observerReady(ctx context.Context, serial string) bool {
	if r.observer == nil {
		return false
	}
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_, err := r.observer.DumpUI(reqCtx, serial)
	return err == nil
}

// isForeignAppMarker — лента ушла из TikTok (Chrome/Gmail/лаунчер).
func isForeignAppMarker(marker string) bool {
	low := strings.ToLower(strings.TrimSpace(marker))
	if low == "" {
		return false
	}
	for _, hint := range []string{
		"gmail", "chrome", "google", "launcher", "recent apps",
		"settings", "page 1 of", "новая вкладка", "new tab",
	} {
		if strings.Contains(low, hint) {
			return true
		}
	}
	return false
}

func tiktokPackageFromScenario(req ScenarioStepRequest) string {
	if p := strings.TrimSpace(req.Params["package"]); p != "" {
		return p
	}
	if strings.Contains(req.ScenarioYAML, "com.ss.android.ugc.trill") {
		return "com.ss.android.ugc.trill"
	}
	return "com.zhiliaoapp.musically"
}

func (r *ScenarioRunner) relaunchTikTokPackage(ctx context.Context, serial, pkg string) bool {
	if pkg == "" {
		pkg = "com.zhiliaoapp.musically"
	}
	if _, err := r.executor.Key(ctx, serial, "home"); err != nil {
		return false
	}
	_ = sleepCtx(ctx, 500*time.Millisecond)
	if err := r.executor.LaunchPackage(ctx, serial, pkg); err != nil {
		return false
	}
	_ = sleepCtx(ctx, 2500*time.Millisecond)
	return true
}

// ensureTikTokForeground — вернуть TikTok на передний план без Home (цепочка после warmup).
func (r *ScenarioRunner) ensureTikTokForeground(ctx context.Context, serial, pkg string) bool {
	if pkg == "" {
		pkg = "com.zhiliaoapp.musically"
	}
	if err := r.executor.LaunchPackage(ctx, serial, pkg); err != nil {
		return false
	}
	_ = sleepCtx(ctx, 1500*time.Millisecond)
	return true
}
