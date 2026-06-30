package port

import "context"

type ScenarioFiles struct {
	ScenarioYAML  string `json:"scenario_yaml"`
	VariablesYAML string `json:"variables_yaml"`
}

type ScenarioSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Serial     string `json:"serial"`
	ValidFrom  string `json:"valid_from,omitempty"`
	ValidUntil string `json:"valid_until,omitempty"`
	IsActive   bool   `json:"is_active,omitempty"`
}

type ScenarioListResult struct {
	Serial           string            `json:"serial"`
	ActiveScenarioID string            `json:"active_scenario_id,omitempty"`
	Items            []ScenarioSummary `json:"items"`
}

type ScenarioStatus struct {
	Serial      string   `json:"serial"`
	ScenarioID  string   `json:"scenario_id"`
	Active      bool     `json:"active"`
	CurrentStep string   `json:"current_step,omitempty"`
	NextStep    string   `json:"next_step,omitempty"`
	StepsDone   []string `json:"steps_done_today,omitempty"`
	CheckedAt   string   `json:"checked_at"`
	Timezone    string   `json:"timezone,omitempty"`
}

// ScenarioLogEntry — промежуточная запись в JSONL (свайпы warmup_feed и т.п.).
type ScenarioLogEntry struct {
	TS     string `json:"ts,omitempty"`
	MSK    string `json:"msk,omitempty"`
	StepID string `json:"step_id,omitempty"`
	Status string `json:"status"`
	Action string `json:"action,omitempty"`
	Event  string `json:"event,omitempty"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

type ScenariosClient interface {
	ListForSerial(ctx context.Context, serial string) (ScenarioListResult, error)
	SetActiveScenario(ctx context.Context, serial, scenarioID string) error
	Get(ctx context.Context, serial, scenarioID string) (ScenarioFiles, error)
	Put(ctx context.Context, serial, scenarioID string, files ScenarioFiles) error
	Delete(ctx context.Context, serial, scenarioID string) error
	GetStatus(ctx context.Context, serial, scenarioID string) (ScenarioStatus, error)
	GetLogs(ctx context.Context, serial, scenarioID, date string) (string, error)
	Generate(ctx context.Context, serial, prompt string) (ScenarioFiles, []string, error)
	GenerateFull(ctx context.Context, serial, prompt string) (map[string]any, error)
	Validate(ctx context.Context, serial, scenarioYAML, variablesYAML string, normalize bool) (map[string]any, error)
	AppendScenarioLog(ctx context.Context, serial, scenarioID string, entry ScenarioLogEntry) error
	RunNow(ctx context.Context, serial string, scenarioIDs []string) (map[string]any, error)
	Ping(ctx context.Context) error
}
