package service

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type RecoveryFlowService struct {
	observer port.ObserverClient
	recovery port.RecoveryClient
	executor port.ExecutorClient
	log      port.Logger
}

func NewRecoveryFlowService(
	observer port.ObserverClient,
	recovery port.RecoveryClient,
	executor port.ExecutorClient,
	log port.Logger,
) *RecoveryFlowService {
	return &RecoveryFlowService{
		observer: observer,
		recovery: recovery,
		executor: executor,
		log:      log,
	}
}

func (s *RecoveryFlowService) RunRecovery(ctx context.Context, serial, scenario, contextHint string) (domain.RecoveryPlan, error) {
	if serial == "" {
		return domain.RecoveryPlan{}, domain.ErrInvalidSerial
	}

	screen, err := s.observer.CaptureScreen(ctx, serial)
	if err != nil {
		return domain.RecoveryPlan{}, domain.ErrObserverUnavailable
	}

	ui, err := s.observer.DumpUI(ctx, serial)
	if err != nil {
		return domain.RecoveryPlan{}, domain.ErrObserverUnavailable
	}

	resp, err := s.recovery.Solve(ctx, domain.RecoverySolveRequest{
		Serial:         serial,
		XMLDump:        ui.XMLDump,
		ScreenshotKey:  screen.MinioKey,
		ScreenshotURL:  screen.ScreenshotURL,
		Scenario:       scenario,
		Context:        contextHint,
	})
	if err != nil {
		return domain.RecoveryPlan{}, err
	}
	if !resp.Success {
		if resp.NeedsManualReview {
			return domain.RecoveryPlan{}, domain.ErrRecoveryFailed
		}
		return domain.RecoveryPlan{}, domain.ErrRecoveryFailed
	}

	plan := domain.RecoveryPlan{
		ErrorHash:  resp.ErrorHash,
		ScenarioID: resp.ScenarioID,
		Source:     resp.Source,
		Steps:      refineTapStepsFromXML(resp.Solution, ui.XMLDump),
		Message:    resp.Message,
	}

	if err := s.executor.ExecutePlan(ctx, serial, plan.Steps); err != nil {
		return plan, domain.ErrExecutorUnavailable
	}

	// Дополнительные tap по Allow, если permission-диалог остался (часто 2 шага подряд).
	for i := 0; i < 3; i++ {
		afterUI, err := s.observer.DumpUI(ctx, serial)
		if err != nil || !isPermissionDialog(afterUI.XMLDump) {
			break
		}
		steps := refineTapStepsFromXML([]domain.SolutionStep{{Type: "tap"}}, afterUI.XMLDump)
		if len(steps) == 0 || steps[0].X == 0 && steps[0].Y == 0 {
			break
		}
		if err := s.executor.ExecutePlan(ctx, serial, steps); err != nil {
			break
		}
	}

	// Проверка результата (сценарий 3, шаг 10) и отчёт в recovery (шаг 11).
	after, err := s.observer.CaptureScreen(ctx, serial)
	afterUI, uiErr := s.observer.DumpUI(ctx, serial)
	success := err == nil && (uiErr != nil || !isPermissionDialog(afterUI.XMLDump))
	if success && plan.ErrorHash != "" {
		_ = s.recovery.ReportOutcome(ctx, domain.RecoveryOutcomeRequest{
			ErrorHash:              plan.ErrorHash,
			Serial:                 serial,
			Success:                true,
			ScreenshotKey:          after.MinioKey,
			PreviousScreenshotHash: "",
		})
	} else if !success && plan.ErrorHash != "" {
		_ = s.recovery.ReportOutcome(ctx, domain.RecoveryOutcomeRequest{
			ErrorHash:              plan.ErrorHash,
			Serial:                 serial,
			Success:                false,
			ScreenshotKey:          "",
			PreviousScreenshotHash: "",
		})
	}

	s.log.Info("recovery flow completed", "service", "phone-orchestrator", "serial", serial, "source", plan.Source, "success", success)
	return plan, nil
}

func (s *RecoveryFlowService) ReportOutcome(ctx context.Context, req domain.RecoveryOutcomeRequest) error {
	if req.Serial == "" || req.ErrorHash == "" {
		return domain.ErrInvalidSerial
	}
	return s.recovery.ReportOutcome(ctx, req)
}
