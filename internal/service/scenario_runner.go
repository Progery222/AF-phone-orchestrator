package service

import (
	"context"
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
	phones    port.PhoneStore
	log       port.Logger
}

func NewScenarioRunner(
	executor port.ExecutorClient,
	observer port.ObserverClient,
	video port.VideoClient,
	content port.ContentClient,
	scenarios port.ScenariosClient,
	phones port.PhoneStore,
	log port.Logger,
) *ScenarioRunner {
	return &ScenarioRunner{
		executor: executor, observer: observer, video: video, content: content,
		scenarios: scenarios, phones: phones, log: log,
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
	return ScenarioStepResult{Status: "completed", Message: "app launched: " + pkg}, nil
}

func (r *ScenarioRunner) actionCloseApp(ctx context.Context, req ScenarioStepRequest) (ScenarioStepResult, error) {
	pkg := req.Params["package"]
	if pkg != "" {
		if err := r.executor.ForceStopPackage(ctx, req.Serial, pkg); err != nil {
			return ScenarioStepResult{}, err
		}
		return ScenarioStepResult{Status: "completed", Message: "force-stop: " + pkg}, nil
	}
	for i := 0; i < 5; i++ {
		if _, err := r.executor.Key(ctx, req.Serial, "back"); err != nil {
			return ScenarioStepResult{}, err
		}
		time.Sleep(400 * time.Millisecond)
	}
	_, _ = r.executor.Key(ctx, req.Serial, "home")
	return ScenarioStepResult{Status: "completed", Message: "closed via back/home"}, nil
}

func (r *ScenarioRunner) actionWarmupFeed(
	ctx context.Context, req ScenarioStepRequest, vars scenarioVariables, phone domain.Phone, rng *rand.Rand,
) (ScenarioStepResult, error) {
	profile := req.Params["profile"]
	phase := req.Params["phase"]
	until := req.Params["until"]

	var cfg map[string]any
	if profile != "" && phase != "" && vars.WarmupProfiles != nil {
		if p, ok := vars.WarmupProfiles[profile]; ok {
			if ph, ok := p[phase].(map[string]any); ok {
				cfg = ph
			}
		}
	}
	feedVars := map[string]any{}
	for k, v := range vars.WarmupFeed {
		feedVars[k] = v
	}
	for k, v := range cfg {
		feedVars[k] = v
	}

	durationSec := pickRange(rng, feedVars["duration_sec"], 300)
	deadline := time.Now().Add(time.Duration(durationSec) * time.Second)
	if until != "" {
		if t, err := time.Parse("15:04", until); err == nil {
			now := time.Now()
			untilTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			if untilTime.After(now) {
				deadline = untilTime
			}
		}
	}

	likesMax := pickRange(rng, feedVars["likes_max"], 2)
	likesDone := 0
	likeProb := pickFloatRange(rng, feedVars["like_probability"], 0.08)

	w, h := portraitSize(phone.ScreenResX, phone.ScreenResY)
	swipes := 0
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return ScenarioStepResult{}, err
		}
		x0, y0, x1, y1 := scaleSwipe(refScreenW/2, refScreenH*1650/1920, refScreenW/2, refScreenH*450/1920, w, h)
		if _, err := r.executor.Swipe(ctx, req.Serial, x0, y0, x1, y1); err != nil {
			return ScenarioStepResult{}, err
		}
		swipes++
		viewSec := pickRange(rng, feedVars["view_duration_sec"], 8)
		time.Sleep(time.Duration(viewSec) * time.Second)

		if likesDone < likesMax && rng.Float64() < likeProb {
			lx, ly := scalePoint(refScreenW*920/1080, refScreenH*1100/1920, w, h)
			if _, err := r.executor.Tap(ctx, req.Serial, lx, ly); err == nil {
				likesDone++
			}
			time.Sleep(time.Duration(pickRange(rng, feedVars["like_cooldown_sec"], 45)) * time.Second)
		}
		pauseMs := pickRange(rng, feedVars["swipe_pause_ms"], 1200)
		time.Sleep(time.Duration(pauseMs) * time.Millisecond)
	}
	return ScenarioStepResult{
		Status:  "completed",
		Message: fmt.Sprintf("warmup: %d swipes, %d likes", swipes, likesDone),
	}, nil
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
	if _, err := r.executor.TypeText(ctx, req.Serial, searchURL); err != nil {
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
