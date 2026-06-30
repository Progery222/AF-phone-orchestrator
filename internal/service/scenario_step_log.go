package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

func (r *ScenarioRunner) appendStepLog(ctx context.Context, req ScenarioStepRequest, action, event, detail string) {
	if r.scenarios == nil || req.ScenarioID == "" {
		return
	}
	now := time.Now()
	_ = r.scenarios.AppendScenarioLog(ctx, req.Serial, req.ScenarioID, port.ScenarioLogEntry{
		TS:     now.UTC().Format(time.RFC3339),
		MSK:    now.In(time.FixedZone("MSK", 3*3600)).Format("15:04:05"),
		StepID: req.StepID,
		Status: event,
		Action: action,
		Event:  event,
		Detail: detail,
	})
}

func (r *ScenarioRunner) logObserverSnapshot(ctx context.Context, req ScenarioStepRequest, action, event string) {
	r.logObserverSnapshotN(ctx, req, action, event, 2)
}

func (r *ScenarioRunner) logObserverSnapshotN(ctx context.Context, req ScenarioStepRequest, action, event string, attempts int) {
	if r.observer == nil {
		r.appendStepLog(ctx, req, action, event+"_no_observer", "observer not configured")
		return
	}
	w, h := r.resolveFeedScreenSize(ctx, req.Serial, domainPhoneOrEmpty(ctx, r, req.Serial))
	marker, ok := r.captureFeedMarkerReliable(ctx, req.Serial, attempts)
	if !ok {
		r.appendStepLog(ctx, req, action, event+"_screen_miss", fmt.Sprintf("screen=%dx%d marker=empty", w, h))
		return
	}
	r.appendStepLog(ctx, req, action, event+"_screen_ok", fmt.Sprintf("screen=%dx%d feed=%q", w, h, marker))
}

func domainPhoneOrEmpty(ctx context.Context, r *ScenarioRunner, serial string) domain.Phone {
	if r.phones == nil {
		return domain.Phone{Serial: serial}
	}
	p, err := r.phones.Get(ctx, serial)
	if err != nil {
		return domain.Phone{Serial: serial}
	}
	return p
}

func formatBehaviorResult(m map[string]interface{}) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("%v", m)
	}
	return string(b)
}

func behaviorJobTerminal(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "completed", "success", "failed", "error":
		return true
	default:
		return false
	}
}

func behaviorJobSuccess(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "completed", "success":
		return true
	default:
		return false
	}
}

// waitBehaviorJobWithLogs — опрос job с записью статусов в scenarios log.
func (r *ScenarioRunner) waitBehaviorJobWithLogs(
	ctx context.Context, req ScenarioStepRequest, action, jobID string, timeout time.Duration,
) (port.BehaviorJob, error) {
	deadline := time.Now().Add(timeout)
	lastStatus := ""
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		job, err := r.behavior.GetJob(ctx, jobID)
		if err != nil {
			r.appendStepLog(ctx, req, action, "job_poll_error", err.Error())
			return port.BehaviorJob{}, err
		}
		if job.Status != lastStatus {
			r.appendStepLog(ctx, req, action, "job_"+strings.ToLower(job.Status), fmt.Sprintf("job=%s", jobID))
			lastStatus = job.Status
		}
		if behaviorJobTerminal(job.Status) {
			if behaviorJobSuccess(job.Status) {
				detail := formatBehaviorResult(job.Result)
				if detail == "" {
					detail = fmt.Sprintf("job=%s", jobID)
				}
				r.appendStepLog(ctx, req, action, "job_done", detail)
				return job, nil
			}
			msg := job.Error
			if msg == "" {
				msg = "behavior job failed"
			}
			r.appendStepLog(ctx, req, action, "job_failed", msg)
			return job, fmt.Errorf("%s", msg)
		}
		if time.Now().After(deadline) {
			r.appendStepLog(ctx, req, action, "job_timeout", fmt.Sprintf("job=%s status=%s", jobID, job.Status))
			return job, fmt.Errorf("таймаут ожидания job %s (status=%s)", jobID, job.Status)
		}
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		case <-ticker.C:
		}
	}
}
