package domain

type ScreenDetection struct {
	Serial          string   `json:"serial,omitempty"`
	State           string   `json:"state"`
	Confidence      float64  `json:"confidence"`
	Source          string   `json:"source"`
	BackendUsed     string   `json:"backend_used"`
	Description     string   `json:"description"`
	Elements        []string `json:"elements"`
	MatchedSignals  []string `json:"matched_signals"`
	SuggestedAction string   `json:"suggested_action"`
	PackageName     string   `json:"package_name"`
	ElementCount    int      `json:"element_count"`
	ScreenshotURL   string   `json:"screenshot_url"`
	MinioKey        string   `json:"minio_key"`
	VLMError        string   `json:"vlm_error,omitempty"`
	TakenAt         string   `json:"taken_at"`
}

type StandSeqSyncResult struct {
	Serial         string  `json:"serial"`
	StandSeqNumber int16   `json:"stand_seq_number"`
	Source         string  `json:"source"`
	Confidence     float64 `json:"confidence"`
	MinioKey       string  `json:"minio_key,omitempty"`
	ScreenshotURL  string  `json:"screenshot_url,omitempty"`
}
