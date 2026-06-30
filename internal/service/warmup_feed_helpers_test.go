package service

import "testing"

func TestIsForeignAppMarker(t *testing.T) {
	cases := []struct {
		marker string
		want   bool
	}{
		{"Page 1 of 1. | Gmail | Chrome", true},
		{"Formula 1 profile | 300.5K | 2,850", false},
		{"Robert Palma profile | Add 1st", false},
		{"Recent apps", true},
	}
	for _, tc := range cases {
		if got := isForeignAppMarker(tc.marker); got != tc.want {
			t.Fatalf("isForeignAppMarker(%q) = %v, want %v", tc.marker, got, tc.want)
		}
	}
}

func TestTiktokPackageFromScenario(t *testing.T) {
	req := ScenarioStepRequest{ScenarioYAML: "package: com.zhiliaoapp.musically"}
	if got := tiktokPackageFromScenario(req); got != "com.zhiliaoapp.musically" {
		t.Fatalf("default package: %s", got)
	}
	req2 := ScenarioStepRequest{ScenarioYAML: "com.ss.android.ugc.trill"}
	if got := tiktokPackageFromScenario(req2); got != "com.ss.android.ugc.trill" {
		t.Fatalf("trill package: %s", got)
	}
}
