package service

import (
	"regexp"
	"strings"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

var (
	allowButtonNodeRe = regexp.MustCompile(`permission_allow_button"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"`)
	boundsRe          = regexp.MustCompile(`bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"`)
)

// refineTapStepsFromXML подставляет центр кнопки Allow из UI dump перед отправкой в executor.
func refineTapStepsFromXML(steps []domain.SolutionStep, xml string) []domain.SolutionStep {
	if len(steps) == 0 || strings.TrimSpace(xml) == "" {
		return steps
	}
	cx, cy, ok := findAllowButtonCenter(xml)
	if !ok {
		return steps
	}

	out := make([]domain.SolutionStep, len(steps))
	copy(out, steps)
	for i, s := range out {
		if s.Type == "tap" {
			out[i].X = cx
			out[i].Y = cy
		}
	}
	return out
}

func findAllowButtonCenter(xml string) (int, int, bool) {
	if m := allowButtonNodeRe.FindStringSubmatch(xml); len(m) == 5 {
		x1, y1 := atoi(m[1]), atoi(m[2])
		x2, y2 := atoi(m[3]), atoi(m[4])
		return (x1 + x2 + 1) / 2, (y1 + y2 + 1) / 2, true
	}

	bestY := int(^uint(0) >> 1)
	var cx, cy int
	found := false
	for _, chunk := range xmlNodeChunks(xml) {
		lower := strings.ToLower(chunk)
		if !isAllowButtonChunk(lower) {
			continue
		}
		m := boundsRe.FindStringSubmatch(chunk)
		if len(m) != 5 {
			continue
		}
		x1, y1 := atoi(m[1]), atoi(m[2])
		x2, y2 := atoi(m[3]), atoi(m[4])
		y := (y1 + y2 + 1) / 2
		if y < bestY {
			bestY = y
			cx = (x1 + x2 + 1) / 2
			cy = y
			found = true
		}
	}
	return cx, cy, found
}

func isAllowButtonChunk(lower string) bool {
	if strings.Contains(lower, "permission_deny") ||
		strings.Contains(lower, "don't") ||
		strings.Contains(lower, "dont allow") {
		return false
	}
	return strings.Contains(lower, "permission_allow_button") ||
		strings.Contains(lower, `text="allow"`)
}

func xmlNodeChunks(xml string) []string {
	if strings.Contains(xml, "\n") {
		return strings.Split(xml, "\n")
	}
	return strings.Split(xml, "/>")
}

func isPermissionDialog(xml string) bool {
	return strings.Contains(xml, "com.google.android.permissioncontroller") ||
		strings.Contains(xml, "com.android.permissioncontroller")
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
