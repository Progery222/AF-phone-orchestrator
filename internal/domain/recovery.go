package domain

type ScreenCapture struct {
	Serial       string
	MinioKey     string
	ScreenshotURL string
	Width        int
	Height       int
}

type UIDump struct {
	Serial   string
	XMLDump  string
	Package  string
}

type RecoverySolveRequest struct {
	Serial         string `json:"serial"`
	XMLDump        string `json:"xml_dump"`
	ScreenshotKey  string `json:"screenshot_key"`
	ScreenshotURL  string `json:"screenshot_url,omitempty"`
	Scenario       string `json:"scenario,omitempty"`
	Context        string `json:"context,omitempty"`
}

type RecoverySolveResponse struct {
	Success           bool           `json:"success"`
	Message           string         `json:"message,omitempty"`
	ErrorHash         string         `json:"error_hash,omitempty"`
	ScenarioID        string         `json:"scenario_id,omitempty"`
	Source            string         `json:"source,omitempty"`
	Solution          []SolutionStep `json:"solution,omitempty"`
	NeedsManualReview bool           `json:"needs_manual_review,omitempty"`
}

type SolutionStep struct {
	Type   string  `json:"type"`
	X      int     `json:"x,omitempty"`
	Y      int     `json:"y,omitempty"`
	Sec    float64 `json:"sec,omitempty"`
	Target string  `json:"target,omitempty"`
}

type RecoveryOutcomeRequest struct {
	ErrorHash              string `json:"error_hash"`
	Serial                 string `json:"serial"`
	Success                bool   `json:"success"`
	ScreenshotKey          string `json:"screenshot_key,omitempty"`
	PreviousScreenshotHash string `json:"previous_screenshot_hash,omitempty"`
}

type RecoveryPlan struct {
	ErrorHash  string
	ScenarioID string
	Source     string
	Steps      []SolutionStep
	Message    string
}
