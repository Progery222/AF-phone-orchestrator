package service

import (
	"os"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func TestRefineTapStepsFromXML_AllowButton(t *testing.T) {
	xml := `<node text="Allow" resource-id="com.android.permissioncontroller:id/permission_allow_button" bounds="[67,1857][1013,1958]"/>`
	steps := []domain.SolutionStep{{Type: "tap", X: 540, Y: 2009}, {Type: "wait", Sec: 2}}
	out := refineTapStepsFromXML(steps, xml)
	if out[0].X != 540 || out[0].Y != 1908 {
		t.Fatalf("got (%d,%d), want (540,1908)", out[0].X, out[0].Y)
	}
}

func TestRefineTapStepsFromXML_RealDeviceDump(t *testing.T) {
	data, err := os.ReadFile("testdata/permission_calendar.xml")
	if err != nil {
		t.Skip("нет testdata/permission_calendar.xml")
	}
	steps := []domain.SolutionStep{{Type: "tap", X: 540, Y: 2009}}
	out := refineTapStepsFromXML(steps, string(data))
	if out[0].Y < 1850 || out[0].Y > 1960 {
		t.Fatalf("refine y=%d, want ~1908", out[0].Y)
	}
}
