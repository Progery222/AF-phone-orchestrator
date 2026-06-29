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

type ScenariosClient interface {
	ListForSerial(ctx context.Context, serial string) ([]ScenarioSummary, error)
	Get(ctx context.Context, serial, scenarioID string) (ScenarioFiles, error)
	Put(ctx context.Context, serial, scenarioID string, files ScenarioFiles) error
	Delete(ctx context.Context, serial, scenarioID string) error
	GetStatus(ctx context.Context, serial, scenarioID string) (ScenarioStatus, error)
	GetLogs(ctx context.Context, serial, scenarioID, date string) (string, error)
	Generate(ctx context.Context, serial, prompt string) (ScenarioFiles, []string, error)
	Ping(ctx context.Context) error
}
