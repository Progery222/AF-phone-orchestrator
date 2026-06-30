package service

import (
	"strings"
	"testing"
	"time"
)

func TestWarmupFeedDeadline_UntilTooShortIgnored(t *testing.T) {
	now := time.Date(2026, 6, 30, 15, 35, 40, 0, time.FixedZone("MSK", 3*3600))
	deadline := warmupFeedDeadline(now, 60, "15:36")
	got := int(deadline.Sub(now).Seconds())
	if got < 55 || got > 65 {
		t.Fatalf("deadline sec=%d want ~60 (until 15:36 ignored)", got)
	}
}

func TestWarmupFeedDeadline_UntilCapsLongRun(t *testing.T) {
	now := time.Date(2026, 6, 30, 17, 56, 0, 0, time.FixedZone("MSK", 3*3600))
	deadline := warmupFeedDeadline(now, 1200, "18:10")
	got := int(deadline.Sub(now).Seconds())
	if got < 800 || got > 860 {
		t.Fatalf("deadline sec=%d want ~14min until 18:10 caps 20min duration", got)
	}
}

func TestFeedSwipeOutcome(t *testing.T) {
	cases := []struct{ before, after, want string }{
		{"a", "b", "changed"},
		{"a", "a", "unchanged"},
		{"a", "", "unknown"},
		{"", "b", "changed"},
		{"", "", "unknown"},
	}
	for _, c := range cases {
		if got := feedSwipeOutcome(c.before, c.after); got != c.want {
			t.Fatalf("feedSwipeOutcome(%q,%q)=%q want %q", c.before, c.after, got, c.want)
		}
	}
}

func TestFeedUIMarkerOrFallback(t *testing.T) {
	xml := strings.Repeat(`<node bounds="[0,0][1,1]"/>`, 50)
	m := feedUIMarkerOrFallback(xml)
	if m == "" || !strings.HasPrefix(m, "xml:") {
		t.Fatalf("expected xml fingerprint, got %q", m)
	}
}

func TestFeedUIMarker(t *testing.T) {
	xml := `<node text="@" content-desc="user123"/><node text="Funny cat video"/><node text="Another title"/>`
	m := feedUIMarker(xml)
	if m == "" {
		t.Fatal("expected marker")
	}
	if !strings.Contains(m, "user123") || !strings.Contains(m, "Funny cat") {
		t.Fatalf("marker=%q", m)
	}
}
