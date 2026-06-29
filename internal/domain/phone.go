package domain

import "time"

type PhoneState string

const (
	StateNew         PhoneState = "new"
	StateWifiSetup   PhoneState = "wifi_setup"
	StateProxySetup  PhoneState = "proxy_setup"
	StateAppsInstall PhoneState = "apps_install"
	StateAuth        PhoneState = "auth"
	StateReady       PhoneState = "ready"
	StateWorking     PhoneState = "working"
	StatePaused      PhoneState = "paused"
	StateError       PhoneState = "error"
	StateRetired     PhoneState = "retired"
)

func (s PhoneState) IsActive() bool {
	return s != StateRetired && s != StateError
}

type Phone struct {
	Serial             string
	State              PhoneState
	CurrentStep        int
	LastError          string
	Model              string
	AndroidVersion     string
	ScreenResX         int
	ScreenResY         int
	CurrentIP          string
	ProxyID            *int
	WifiSSID           string
	WiFiPass           string
	ProxyIP            string
	ProxyPort          int
	ProxyUser          string
	ProxyPass          string
	ProvisionApps      []ProvisionApp
	AdbPort            int
	LastHeartbeat      *time.Time
	HeartbeatCount     int
	RecoveryInProgress bool
	LastErrorHash      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ReadyAt            *time.Time
	RetiredAt          *time.Time
	StandSeqNumber     *int16
}

type PhoneStateLog struct {
	ID         int64
	Serial     string
	FromState  PhoneState
	ToState    PhoneState
	Step       int
	Error      string
	DurationMS int
	CreatedAt  time.Time
}

type PhoneStats struct {
	Total     int
	Working   int
	Paused    int
	Error     int
	SettingUp int
}

type PhoneStateEvent struct {
	Serial   string `json:"serial"`
	OldState string `json:"old_state"`
	NewState string `json:"new_state"`
	Error    string `json:"error,omitempty"`
}

type PhoneSyncResult struct {
	Detected int `json:"detected"`
	Added    int `json:"added"`
	Updated  int `json:"updated"`
	Missing  int `json:"missing"`
}

// ProvisionApp — приложение для передачи в phone-provisioner.
type ProvisionApp struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

// AddPhoneRequest — регистрация телефона в orchestrator.
type AddPhoneRequest struct {
	Serial         string         `json:"serial"`
	StandSeqNumber *int16         `json:"stand_seq_number,omitempty"`
	WifiSSID       string         `json:"wifi_ssid,omitempty"`
	WiFiPass       string         `json:"wifi_password,omitempty"`
	ProxyIP        string         `json:"proxy_ip,omitempty"`
	ProxyPort      int            `json:"proxy_port,omitempty"`
	ProxyUser      string         `json:"proxy_username,omitempty"`
	ProxyPass      string         `json:"proxy_password,omitempty"`
	Apps           []ProvisionApp `json:"apps,omitempty"`
}

// UpdateStandSeqRequest — обновление номера на стенде.
type UpdateStandSeqRequest struct {
	StandSeqNumber *int16 `json:"stand_seq_number"`
}
