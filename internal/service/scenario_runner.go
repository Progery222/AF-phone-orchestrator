package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"gopkg.in/yaml.v3"
)

const (
	refScreenW = 1080
	refScreenH = 1920
)

type ScenarioStepRequest struct {
	Serial         string
	ScenarioID     string
	StepID         string
	Action         string
	Params         map[string]string
	Uses           string
	VariablesYAML  string
	ScenarioYAML   string
	ScreenshotKeys []string
	VideoOutputKey string
}

type ScenarioStepResult struct {
	Status         string   `json:"status"`
	Message        string   `json:"message,omitempty"`
	ScreenshotKeys []string `json:"screenshot_keys,omitempty"`
	VideoJobID     string   `json:"video_job_id,omitempty"`
	VideoOutputKey string   `json:"video_output_key,omitempty"`
	DurationSec    int      `json:"duration_sec,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type ScenarioRunner struct {
	executor  port.ExecutorClient
	observer  port.ObserverClient
	video     port.VideoClient
	content   port.ContentClient
	scenarios port.ScenariosClient
	behavior  port.BehaviorClient
	phones    port.PhoneStore
	recovery  *RecoveryFlowService
	log       port.Logger
}

func NewScenarioRunner(
	executor port.ExecutorClient,
	observer port.ObserverClient,
	video port.VideoClient,
	content port.ContentClient,
	scenarios port.ScenariosClient,
	behavior port.BehaviorClient,
	phones port.PhoneStore,
	recovery *RecoveryFlowService,
	log port.Logger,
) *ScenarioRunner {
	return &ScenarioRunner{
		executor: executor, observer: observer, video: video, content: content,
		scenarios: scenarios, behavior: behavior, phones: phones, recovery: recovery, log: log,
	}
}

func (r *ScenarioRunner) RunStep(ctx context.Context, req ScenarioStepRequest) (ScenarioStepResult, error) {
	if req.Serial == "" || req.ScenarioID == "" || req.StepID == "" {
		return ScenarioStepResult{}, fmt.Errorf("serial, scenario_id и step_id обязательны")
	}
	start := time.Now()
	vars, err := parseScenarioVariables(req.VariablesYAML)
	if err != nil {
		return ScenarioStepResult{Status: "failed", Error: err.Error()}, err
	}
	if strings.TrimSpace(req.VariablesYAML) == "" && r.scenarios != nil {
		files, err := r.scenarios.Get(ctx, req.Serial, req.ScenarioID)
		if err == nil {
			vars, _ = parseScenarioVariables(files.VariablesYAML)
			if req.ScenarioYAML == "" {
				req.ScenarioYAML = files.ScenarioYAML
			}
		}
	}
	phone, _ := r.phones.Get(ctx, req.Serial)
	if phone.Serial != "" && phone.State != "" && phone.State != domain.StateWorking && phone.State != domain.StateReady {
		err := fmt.Errorf("телефон в состоянии %q — шаг сценария невозможен (нужен working/ready)", phone.State)
		return ScenarioStepResult{Status: "failed", Error: err.Error()}, err
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var result ScenarioStepResult
	switch req.Action {
	case "wait":
		result, err = r.actionWait(ctx, req, rng)
	case "open_app":
		result, err = r.actionOpenApp(ctx, req)
	case "close_app":
		result, err = r.actionCloseApp(ctx, req)
	case "warmup_feed":
		result, err = r.actionWarmupFeed(ctx, req, vars, phone, rng)
	case "browser_research":
		result, err = r.actionBrowserResearch(ctx, req, vars, phone, rng)
	case "create_video_from_screenshots":
		result, err = r.actionCreateVideo(ctx, req, phone)
	case "publish_content":
		result, err = r.actionPublishContent(ctx, req, vars, phone, rng)
	case "social_action":
		result, err = r.actionSocialAction(ctx, req)
	case "custom_execute":
		result, err = r.actionCustomExecute(ctx, req)
	default:
		err = fmt.Errorf("неизвестное действие: %s", req.Action)
		result = ScenarioStepResult{Status: "failed", Error: err.Error()}
	}
	result.DurationSec = int(time.Since(start).Seconds())
	if err != nil {
		r.log.Error("scenario step failed", "serial", req.Serial, "step", req.StepID, "action", req.Action, "error", err)
		if result.Status == "" {
			result.Status = "failed"
		}
		if result.Error == "" {
			result.Error = err.Error()
		}
		return result, err
	}
	if result.Status == "" {
		result.Status = "completed"
	}
	r.log.Info("scenario step done", "serial", req.Serial, "step", req.StepID, "action", req.Action, "duration_sec", result.DurationSec)
	return result, nil
}

func (r *ScenarioRunner) actionWait(ctx context.Context, req ScenarioStepRequest, rng *rand.Rand) (ScenarioStepResult, error) {
	sec := 1
	if v := req.Params["duration_sec"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sec = n
		}
	}
	if err := r.executor.ExecutePlan(ctx, req.Serial, []domain.SolutionStep{{Type: "wait", Sec: float64(sec)}}); err != nil {
		return ScenarioStepResult{}, err
	}
	_ = rng
	return ScenarioStepResult{Status: "completed", Message: fmt.Sprintf("wait %ds", sec)}, nil
}

func (r *ScenarioRunner) actionOpenApp(ctx context.Context, req ScenarioStepRequest) (ScenarioStepResult, error) {
	pkg := req.Params["package"]
	if pkg == "" {
		return ScenarioStepResult{}, fmt.Errorf("params.package обязателен")
	}
	if _, err := r.executor.Key(ctx, req.Serial, "home"); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(800 * time.Millisecond)
	if err := r.executor.LaunchPackage(ctx, req.Serial, pkg); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(2 * time.Second)
	if r.recovery != nil && r.observer != nil {
		if ui, err := r.observer.DumpUI(ctx, req.Serial); err == nil && isPermissionDialog(ui.XMLDump) {
			r.log.Info("scenario open_app: permission dialog, recovery", "serial", req.Serial, "step", req.StepID)
			r.appendStepLog(ctx, req, "open_app", "recovery_attempt", "permission dialog after launch")
			_, _ = r.recovery.RunRecovery(ctx, req.Serial, "permission dialog after open_app", req.StepID)
		}
	}
	r.logObserverSnapshot(ctx, req, "open_app", "after_launch")
	return ScenarioStepResult{Status: "completed", Message: "app launched: " + pkg}, nil
}

func (r *ScenarioRunner) actionCloseApp(ctx context.Context, req ScenarioStepRequest) (ScenarioStepResult, error) {
	pkg := req.Params["package"]
	if pkg != "" {
		if err := r.executor.ForceStopPackage(ctx, req.Serial, pkg); err != nil {
			r.log.Warn("force-stop failed, fallback back/home", "package", pkg, "error", err)
			return r.closeAppViaKeys(ctx, req.Serial)
		}
		time.Sleep(500 * time.Millisecond)
		if _, err := r.executor.Key(ctx, req.Serial, "home"); err != nil {
			return ScenarioStepResult{}, err
		}
		return ScenarioStepResult{Status: "completed", Message: "force-stop + home: " + pkg}, nil
	}
	return r.closeAppViaKeys(ctx, req.Serial)
}

func (r *ScenarioRunner) closeAppViaKeys(ctx context.Context, serial string) (ScenarioStepResult, error) {
	for i := 0; i < 5; i++ {
		if _, err := r.executor.Key(ctx, serial, "back"); err != nil {
			return ScenarioStepResult{}, err
		}
		time.Sleep(400 * time.Millisecond)
	}
	if _, err := r.executor.Key(ctx, serial, "home"); err != nil {
		return ScenarioStepResult{}, err
	}
	return ScenarioStepResult{Status: "completed", Message: "closed via back/home"}, nil
}

func (r *ScenarioRunner) actionWarmupFeed(
	ctx context.Context, req ScenarioStepRequest, vars scenarioVariables, phone domain.Phone, rng *rand.Rand,
) (ScenarioStepResult, error) {
	profile := req.Params["profile"]
	phase := req.Params["phase"]
	until := req.Params["until"]
	feedVars := mergeWarmupFeedVars(vars, profile, phase)

	durationSec := 60
	if raw := strings.TrimSpace(req.Params["duration_sec"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			durationSec = n
		}
	} else if d := pickRange(rng, feedVars["duration_sec"], 0); d > 0 {
		durationSec = d
	}

	likesMax := pickRange(rng, feedVars["likes_max"], 0)
	likesDone := 0
	likeProb := pickFloatRange(rng, feedVars["like_probability"], 0)
	likesEnabled := likesMax > 0 && likeProb > 0

	now := time.Now()
	deadline := warmupFeedDeadline(now, durationSec, until)
	w, h := r.resolveFeedScreenSize(ctx, req.Serial, phone)
	swipes := 0
	centerSwipe := strings.EqualFold(strings.TrimSpace(req.Params["network"]), "instagram") ||
		strings.Contains(strings.ToLower(req.StepID), "instagram")

	r.log.Info("warmup_feed start", "serial", req.Serial, "step", req.StepID,
		"screen", fmt.Sprintf("%dx%d", w, h), "duration_sec", durationSec, "until", until,
		"deadline_sec", int(deadline.Sub(now).Seconds()))

	if err := sleepCtx(ctx, 2*time.Second); err != nil {
		return ScenarioStepResult{}, err
	}

	// Первое видео после open_app — короткий просмотр перед свайпами.
	if err := sleepCtx(ctx, time.Duration(pickRange(rng, feedVars["initial_view_sec"], 3))*time.Second); err != nil {
		return ScenarioStepResult{}, err
	}

	feedBefore := r.captureFeedMarker(ctx, req.Serial)
	unchangedStreak := 0

	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return ScenarioStepResult{}, err
		}

		swipeN := swipes + 1
		x0, y0, x1, y1 := feedSwipeCoordsFor(w, h, centerSwipe)
		r.appendWarmupLog(ctx, req, "swipe_attempt", fmt.Sprintf(
			"#%d coords (%d,%d)→(%d,%d) screen=%dx%d feed=%q",
			swipeN, x0, y0, x1, y1, w, h, feedBefore,
		))

		if err := r.feedSwipeOnce(ctx, req.Serial, x0, y0, x1, y1); err != nil {
			r.appendWarmupLog(ctx, req, "swipe_failed", fmt.Sprintf("#%d err=%v", swipeN, err))
			return ScenarioStepResult{}, err
		}
		swipes++

		if err := sleepCtx(ctx, 1200*time.Millisecond); err != nil {
			return ScenarioStepResult{}, err
		}
		feedAfter, observed := r.captureFeedMarkerReliable(ctx, req.Serial, 3)
		outcome := feedSwipeOutcome(feedBefore, feedAfter)
		r.appendWarmupLog(ctx, req, "swipe_done", fmt.Sprintf(
			"#%d ok feed_after=%q outcome=%s observed=%v", swipeN, feedAfter, outcome, observed,
		))
		switch outcome {
		case "unchanged":
			unchangedStreak++
		case "changed":
			unchangedStreak = 0
			feedBefore = feedAfter
		default: // unknown — observer не подтвердил смену, не считаем застреванием
			if observed && feedAfter != "" {
				feedBefore = feedAfter
			}
		}
		if unchangedStreak >= 2 {
			if r.tryRecoverBlockedFeed(ctx, req, feedBefore, w, h, centerSwipe) {
				unchangedStreak = 0
				if m, ok := r.captureFeedMarkerReliable(ctx, req.Serial, 2); ok {
					feedBefore = m
				}
			}
		}

		watchSec := pickRange(rng, feedVars["scroll_interval_sec"], 0)
		if watchSec <= 0 {
			watchSec = pickRange(rng, feedVars["view_duration_sec"], 6)
		}
		if watchSec <= 0 {
			watchSec = 6
		}
		if err := sleepCtx(ctx, time.Duration(watchSec)*time.Second); err != nil {
			return ScenarioStepResult{}, err
		}

		if likesEnabled && likesDone < likesMax && rng.Float64() < likeProb {
			lx, ly := scalePoint(refScreenW*920/1080, refScreenH*1100/1920, w, h)
			if _, err := r.executor.Tap(ctx, req.Serial, lx, ly); err == nil {
				likesDone++
				cooldown := pickRange(rng, feedVars["like_cooldown_sec"], 12)
				remaining := time.Until(deadline)
				if time.Duration(cooldown)*time.Second > remaining {
					cooldown = int(remaining.Seconds())
				}
				if err := sleepCtx(ctx, time.Duration(cooldown)*time.Second); err != nil {
					return ScenarioStepResult{}, err
				}
			}
		}

		pauseMs := pickRange(rng, feedVars["swipe_pause_ms"], 400)
		if err := sleepCtx(ctx, time.Duration(pauseMs)*time.Millisecond); err != nil {
			return ScenarioStepResult{}, err
		}
	}
	return ScenarioStepResult{
		Status:  "completed",
		Message: fmt.Sprintf("warmup: %d swipes, %d likes, %ds", swipes, likesDone, durationSec),
	}, nil
}

func (r *ScenarioRunner) tryRecoverBlockedFeed(ctx context.Context, req ScenarioStepRequest, feedMarker string, w, h int32, centerSwipe bool) bool {
	if r.observer != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		ui, err := r.observer.DumpUI(reqCtx, req.Serial)
		cancel()
		if err == nil && isPermissionDialog(ui.XMLDump) {
			r.appendWarmupLog(ctx, req, "recovery_attempt", "permission dialog on feed")
		} else {
			r.appendWarmupLog(ctx, req, "recovery_attempt", "feed unchanged after swipes")
		}
	} else {
		r.appendWarmupLog(ctx, req, "recovery_attempt", "feed unchanged after swipes")
	}

	if r.recovery != nil && r.observerReady(ctx, req.Serial) {
		plan, err := r.recovery.RunRecovery(ctx, req.Serial, "feed blocked during scenario warmup", req.StepID+": "+feedMarker)
		if err != nil {
			r.appendWarmupLog(ctx, req, "recovery_failed", err.Error())
		} else {
			r.appendWarmupLog(ctx, req, "recovery_done", plan.Message)
			return true
		}
	} else if r.recovery == nil {
		r.appendWarmupLog(ctx, req, "recovery_skip", "recovery service not configured")
	} else {
		r.appendWarmupLog(ctx, req, "recovery_skip", "observer unavailable for recovery")
	}

	r.feedStuckFallback(ctx, req.Serial, w, h, centerSwipe)
	r.appendWarmupLog(ctx, req, "recovery_fallback", "double swipe + wait")
	return true
}

func (r *ScenarioRunner) feedStuckFallback(ctx context.Context, serial string, w, h int32, centerSwipe bool) {
	x0, y0, x1, y1 := feedSwipeCoordsFor(w, h, centerSwipe)
	_ = r.feedSwipeOnce(ctx, serial, x0, y0, x1, y1)
	_ = sleepCtx(ctx, 400*time.Millisecond)
	_ = r.feedSwipeOnce(ctx, serial, x0, y0, x1, y1)
	_ = sleepCtx(ctx, 2*time.Second)
}

func (r *ScenarioRunner) feedSwipeOnce(ctx context.Context, serial string, x0, y0, x1, y1 int32) error {
	res, err := r.executor.Swipe(ctx, serial, x0, y0, x1, y1)
	if err != nil {
		return err
	}
	r.log.Info("warmup swipe", "serial", serial, "coords", fmt.Sprintf("%d,%d→%d,%d", x0, y0, x1, y1),
		"status", res.Status, "msg", res.Message)
	return nil
}

func (r *ScenarioRunner) appendWarmupLog(ctx context.Context, req ScenarioStepRequest, event, detail string) {
	r.appendStepLog(ctx, req, "warmup_feed", event, detail)
}

func feedSwipeCoordsFor(w, h int32, center bool) (int32, int32, int32, int32) {
	if center {
		return scaleSwipe(refScreenW/2, refScreenH*1650/1920, refScreenW/2, refScreenH*450/1920, w, h)
	}
	return tiktokFeedSwipeCoords(w, h)
}

func tiktokFeedSwipeCoords(w, h int32) (int32, int32, int32, int32) {
	// Как tg-bot / Controls: центр экрана, свайп вверх.
	return scaleSwipe(refScreenW/2, refScreenH*1650/1920, refScreenW/2, refScreenH*450/1920, w, h)
}

func (r *ScenarioRunner) actionBrowserResearch(
	ctx context.Context, req ScenarioStepRequest, vars scenarioVariables, phone domain.Phone, rng *rand.Rand,
) (ScenarioStepResult, error) {
	br, err := parseBrowserResearch(req.ScenarioYAML)
	if err != nil {
		return ScenarioStepResult{}, err
	}
	browserPkg := br.BrowserPackage
	if browserPkg == "" {
		browserPkg = "com.android.chrome"
	}
	searchKeys := br.SearchKeys
	if len(searchKeys) == 0 {
		return ScenarioStepResult{}, fmt.Errorf("search_keys не заданы в scenario.yaml")
	}
	keyIdx := len(req.ScreenshotKeys) % len(searchKeys)
	query := searchKeys[keyIdx]

	domains := br.TargetDomains
	if len(domains) == 0 {
		domains = []string{"vc.ru"}
	}
	targetDomain := domains[rng.Intn(len(domains))]

	googleHost := googleHostForPhone(phone, br.LocaleFrom)
	searchURL := fmt.Sprintf("https://%s/search?q=%s", googleHost, strings.ReplaceAll(query, " ", "+"))

	if err := r.executor.LaunchPackage(ctx, req.Serial, browserPkg); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(2 * time.Second)

	if _, err := r.executor.Key(ctx, req.Serial, "home"); err == nil {
		time.Sleep(500 * time.Millisecond)
		_ = r.executor.LaunchPackage(ctx, req.Serial, browserPkg)
		time.Sleep(1500 * time.Millisecond)
	}

	// Адресная строка: тап по верхней зоне, ввод URL.
	w, h := portraitSize(phone.ScreenResX, phone.ScreenResY)
	ax, ay := scalePoint(refScreenW/2, refScreenH*120/1920, w, h)
	if _, err := r.executor.Tap(ctx, req.Serial, ax, ay); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(800 * time.Millisecond)
	if _, err := r.executor.TypeText(ctx, req.Serial, searchURL, true); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := r.executor.Key(ctx, req.Serial, "enter"); err != nil {
		return ScenarioStepResult{}, err
	}

	brVars := vars.BrowserResearch
	serpWait := pickRange(rng, brVars["search_results_view_sec"], 7)
	time.Sleep(time.Duration(serpWait) * time.Second)

	keys := append([]string{}, req.ScreenshotKeys...)
	if cap1, err := r.observer.CaptureScreen(ctx, req.Serial); err == nil && cap1.MinioKey != "" {
		keys = append(keys, cap1.MinioKey)
	}

	ui, err := r.observer.DumpUI(ctx, req.Serial)
	if err != nil {
		return ScenarioStepResult{}, err
	}
	if x, y, ok := findOrganicLink(ui.XMLDump, targetDomain); ok {
		if _, err := r.executor.Tap(ctx, req.Serial, int32(x), int32(y)); err != nil {
			return ScenarioStepResult{}, err
		}
	} else {
		r.log.Warn("organic link not found, staying on SERP", "domain", targetDomain)
	}

	loadTimeout := pickRange(rng, brVars["page_load_timeout_sec"], 20)
	time.Sleep(time.Duration(loadTimeout) * time.Second)
	if cap2, err := r.observer.CaptureScreen(ctx, req.Serial); err == nil && cap2.MinioKey != "" {
		keys = append(keys, cap2.MinioKey)
	}

	scrollCount := pickRange(rng, brVars["scroll_count"], 5)
	for i := 0; i < scrollCount; i++ {
		x0, y0, x1, y1 := scaleSwipe(refScreenW/2, refScreenH*1500/1920, refScreenW/2, refScreenH*600/1920, w, h)
		_, _ = r.executor.Swipe(ctx, req.Serial, x0, y0, x1, y1)
		interval := pickRange(rng, brVars["scroll_interval_sec"], 12)
		time.Sleep(time.Duration(interval) * time.Second)
		if cap, err := r.observer.CaptureScreen(ctx, req.Serial); err == nil && cap.MinioKey != "" {
			keys = append(keys, cap.MinioKey)
		}
	}

	dwell := pickRange(rng, brVars["page_dwell_sec"], 90)
	time.Sleep(time.Duration(dwell) * time.Second)
	if cap3, err := r.observer.CaptureScreen(ctx, req.Serial); err == nil && cap3.MinioKey != "" {
		keys = append(keys, cap3.MinioKey)
	}

	_, _ = r.executor.Key(ctx, req.Serial, "home")
	return ScenarioStepResult{
		Status:         "completed",
		Message:        fmt.Sprintf("browser: %q → %s, %d screenshots", query, targetDomain, len(keys)),
		ScreenshotKeys: keys,
	}, nil
}

func (r *ScenarioRunner) actionCreateVideo(ctx context.Context, req ScenarioStepRequest, phone domain.Phone) (ScenarioStepResult, error) {
	if r.video == nil {
		return ScenarioStepResult{}, fmt.Errorf("video-generator не настроен")
	}
	keys := req.ScreenshotKeys
	minCount := 3
	if v := req.Params["min_count"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minCount = n
		}
	}
	if len(keys) < minCount {
		return ScenarioStepResult{}, fmt.Errorf("недостаточно скринов: %d < %d", len(keys), minCount)
	}
	profile := port.VideoOutputProfile{Width: 1080, Height: 1920, DurationSec: 15, FrameSec: 2.5}
	if phone.ScreenResX > 0 && phone.ScreenResY > 0 {
		w, h := portraitSize(phone.ScreenResX, phone.ScreenResY)
		profile.Width = int(w)
		profile.Height = int(h)
	}
	job, err := r.video.CreateFromScreenshots(ctx, req.Serial, keys, "", "", profile)
	if err != nil {
		return ScenarioStepResult{}, err
	}
	outKey, err := r.waitVideoJob(ctx, job.ID, 5*time.Minute)
	if err != nil {
		return ScenarioStepResult{}, err
	}
	return ScenarioStepResult{
		Status:         "completed",
		Message:        "video created",
		VideoJobID:     job.ID,
		VideoOutputKey: outKey,
	}, nil
}

func (r *ScenarioRunner) actionPublishContent(
	ctx context.Context, req ScenarioStepRequest, vars scenarioVariables, phone domain.Phone, rng *rand.Rand,
) (ScenarioStepResult, error) {
	videoKey := req.VideoOutputKey
	if videoKey == "" {
		videoKey = req.Params["video_key"]
	}
	if videoKey == "" {
		return ScenarioStepResult{}, fmt.Errorf("video_output_key не задан (шаг create_video)")
	}
	if r.content == nil {
		return ScenarioStepResult{}, fmt.Errorf("content-distributor не настроен")
	}
	item, err := r.content.Register(ctx, port.ContentRegisterRequest{
		Serial:    req.Serial,
		ObjectKey: videoKey,
		Filename:  "scenario_video.mp4",
		MediaType: "video/mp4",
	})
	if err != nil {
		return ScenarioStepResult{}, err
	}
	if err := r.content.DownloadAsync(ctx, req.Serial, item.ContentID, videoKey); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(5 * time.Second)

	platform := strings.ToLower(req.Params["platform"])
	pkg := "com.zhiliaoapp.musically"
	if platform == "instagram" {
		pkg = "com.instagram.android"
	}
	if err := r.executor.LaunchPackage(ctx, req.Serial, pkg); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(3 * time.Second)

	w, h := portraitSize(phone.ScreenResX, phone.ScreenResY)
	// Плюс «создать» (нижняя панель) и выбор из галереи — упрощённый MVP.
	cx, cy := scalePoint(refScreenW/2, refScreenH*1850/1920, w, h)
	if _, err := r.executor.Tap(ctx, req.Serial, cx, cy); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(2 * time.Second)
	gx, gy := scalePoint(refScreenW/4, refScreenH*400/1920, w, h)
	if _, err := r.executor.Tap(ctx, req.Serial, gx, gy); err != nil {
		return ScenarioStepResult{}, err
	}
	time.Sleep(2 * time.Second)
	nx, ny := scalePoint(refScreenW*950/1080, refScreenH*120/1920, w, h)
	if _, err := r.executor.Tap(ctx, req.Serial, nx, ny); err != nil {
		return ScenarioStepResult{}, err
	}
	_ = vars
	_ = rng
	return ScenarioStepResult{
		Status:         "completed",
		Message:        "content registered and upload flow started",
		VideoOutputKey: videoKey,
	}, nil
}

func (r *ScenarioRunner) waitVideoJob(ctx context.Context, jobID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := r.video.GetJob(ctx, jobID)
		if err != nil {
			return "", err
		}
		st := strings.ToLower(job.Status)
		if st == "completed" || st == "done" || st == "success" {
			if job.OutputKey != "" {
				return job.OutputKey, nil
			}
			return "", fmt.Errorf("video job без output_key")
		}
		if st == "failed" || st == "error" {
			return "", fmt.Errorf("video job failed: %s", job.Error)
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("timeout ожидания video job %s", jobID)
}

type browserResearchConfig struct {
	BrowserPackage string
	LocaleFrom     string
	SearchKeys     []string
	TargetDomains  []string
}

func parseBrowserResearch(scenarioYAML string) (browserResearchConfig, error) {
	var root map[string]any
	if err := yaml.Unmarshal([]byte(scenarioYAML), &root); err != nil {
		return browserResearchConfig{}, fmt.Errorf("scenario.yaml: %w", err)
	}
	br := nestedMap(root, "content_sources", "browser_research")
	if br == nil {
		return browserResearchConfig{}, fmt.Errorf("content_sources.browser_research не найден")
	}
	out := browserResearchConfig{
		BrowserPackage: strVal(br["browser_package"]),
		SearchKeys:     nestedSlice(br, "search_keys", "items"),
		TargetDomains:  nestedSlice(br, "target_domains", "items"),
	}
	if g := nestedMap(br, "google"); g != nil {
		out.LocaleFrom = strVal(g["locale_from"])
	}
	return out, nil
}

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func googleHostForPhone(phone domain.Phone, localeFrom string) string {
	if localeFrom != "phone_proxy" {
		return "www.google.com"
	}
	ip := strings.TrimSpace(phone.ProxyIP)
	if strings.HasPrefix(ip, "77.") || strings.HasPrefix(ip, "95.") || strings.HasPrefix(ip, "178.") {
		return "www.google.ru"
	}
	return "www.google.com"
}

var serpBoundsRe = regexp.MustCompile(`bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"`)

func findOrganicLink(xml, domain string) (int, int, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return 0, 0, false
	}
	lines := strings.Split(xml, "\n")
	for _, line := range lines {
		low := strings.ToLower(line)
		if !strings.Contains(low, domain) {
			continue
		}
		if strings.Contains(low, "google.com") || strings.Contains(low, "google.ru") {
			continue
		}
		m := serpBoundsRe.FindStringSubmatch(line)
		if len(m) != 5 {
			continue
		}
		x1, _ := strconv.Atoi(m[1])
		y1, _ := strconv.Atoi(m[2])
		x2, _ := strconv.Atoi(m[3])
		y2, _ := strconv.Atoi(m[4])
		return (x1 + x2) / 2, (y1 + y2) / 2, true
	}
	return 0, 0, false
}

func portraitSize(rx, ry int) (w, h int32) {
	if rx <= 0 || ry <= 0 {
		return refScreenW, refScreenH
	}
	w = int32(min(rx, ry))
	h = int32(max(rx, ry))
	return w, h
}

func scalePoint(x, y int, w, h int32) (int32, int32) {
	return scaleCoord(x, refScreenW, w), scaleCoord(y, refScreenH, h)
}

func scaleSwipe(x0, y0, x1, y1 int, w, h int32) (int32, int32, int32, int32) {
	return scaleCoord(x0, refScreenW, w), scaleCoord(y0, refScreenH, h),
		scaleCoord(x1, refScreenW, w), scaleCoord(y1, refScreenH, h)
}

func scaleCoord(value, ref int, size int32) int32 {
	if ref <= 0 || size <= 1 {
		return int32(value)
	}
	v := int32((value * int(size)) / ref)
	if v < 1 {
		return 1
	}
	if v >= size {
		return size - 1
	}
	return v
}

func (r *ScenarioRunner) actionSocialAction(ctx context.Context, req ScenarioStepRequest) (ScenarioStepResult, error) {
	if r.behavior == nil {
		return ScenarioStepResult{}, fmt.Errorf("behavior-engine не настроен")
	}
	network := strings.TrimSpace(req.Params["network"])
	if network == "" {
		network = "tiktok"
	}
	behavior := strings.TrimSpace(req.Params["behavior"])
	if behavior == "" {
		behavior = "feed"
	}
	body := map[string]any{"serial": req.Serial}
	for k, v := range req.Params {
		switch k {
		case "network", "behavior", "duration_sec":
			continue
		case "count":
			if n, err := strconv.Atoi(v); err == nil {
				body["count"] = n
			} else {
				body["count"] = v
			}
		case "like_probability":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				body["like_probability"] = f
			}
		case "skip_launch":
			body["skip_launch"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
		default:
			body[k] = v
		}
	}
	if behavior == "search-feed" {
		if _, ok := body["skip_launch"]; !ok {
			body["skip_launch"] = true
		}
		r.appendStepLog(ctx, req, "social_action", "observer_cooldown", "wait 3s after warmup")
		if err := sleepCtx(ctx, 3*time.Second); err != nil {
			return ScenarioStepResult{}, err
		}
	}
	// Быстрый снимок — не блокировать поиск на 30+ с пока observer занят после warmup.
	if behavior == "search-feed" {
		r.logObserverSnapshotN(ctx, req, "social_action", "before", 1)
	} else {
		r.logObserverSnapshot(ctx, req, "social_action", "before")
	}
	r.appendStepLog(ctx, req, "social_action", "job_submit", fmt.Sprintf(
		"network=%s behavior=%s query=%v count=%v skip_launch=%v",
		network, behavior, body["query"], body["count"], body["skip_launch"],
	))
	job, err := r.behavior.RunSocialAction(ctx, network, behavior, req.Serial, body)
	if err != nil {
		r.appendStepLog(ctx, req, "social_action", "job_submit_failed", err.Error())
		return ScenarioStepResult{}, err
	}
	r.appendStepLog(ctx, req, "social_action", "job_started", fmt.Sprintf("job=%s", job.ID))
	timeout := 10 * time.Minute
	if ds := req.Params["duration_sec"]; ds != "" {
		if n, err := strconv.Atoi(ds); err == nil && n > 0 {
			timeout = time.Duration(n+90) * time.Second
		}
	}
	final, err := r.waitBehaviorJobWithLogs(ctx, req, "social_action", job.ID, timeout)
	if err != nil {
		return ScenarioStepResult{Status: "failed", Error: err.Error()}, err
	}
	r.logObserverSnapshot(ctx, req, "social_action", "after")
	return ScenarioStepResult{
		Status:  "completed",
		Message: fmt.Sprintf("social %s/%s job=%s", network, behavior, final.ID),
	}, nil
}

func (r *ScenarioRunner) actionCustomExecute(ctx context.Context, req ScenarioStepRequest) (ScenarioStepResult, error) {
	raw := req.Params["steps_json"]
	if raw == "" {
		return ScenarioStepResult{}, fmt.Errorf("params.steps_json обязателен")
	}
	var steps []domain.SolutionStep
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return ScenarioStepResult{}, fmt.Errorf("steps_json: %w", err)
	}
	if len(steps) == 0 {
		return ScenarioStepResult{}, fmt.Errorf("steps_json пуст")
	}
	if err := r.executor.ExecutePlan(ctx, req.Serial, steps); err != nil {
		return ScenarioStepResult{}, err
	}
	return ScenarioStepResult{
		Status:  "completed",
		Message: fmt.Sprintf("custom_execute: %d шагов", len(steps)),
	}, nil
}
