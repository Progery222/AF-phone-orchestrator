package service_test

import (
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

func TestLookupStepFromYAML(t *testing.T) {
	yaml := `steps:
  - id: open_tiktok
    action: open_app
    params:
      package: com.ss.android.ugc.trill
`
	action, _, params, err := service.LookupStepFromYAML(yaml, "open_tiktok")
	if err != nil {
		t.Fatal(err)
	}
	if action != "open_app" || params["package"] != "com.ss.android.ugc.trill" {
		t.Fatalf("got action=%s params=%v", action, params)
	}
}

func TestMergeStepRequest(t *testing.T) {
	yaml := `steps:
  - id: s1
    action: wait
    params:
      duration_sec: "2"
`
	a, _, p, err := service.MergeStepRequest(yaml, "s1", "", "", nil)
	if err != nil || a != "wait" || p["duration_sec"] != "2" {
		t.Fatalf("merge failed: %v %s %v", err, a, p)
	}
}
