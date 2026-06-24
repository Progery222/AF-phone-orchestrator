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
	Serial              string
	State               PhoneState
	CurrentStep         int
	LastError           string
	Model               string
	AndroidVersion      string
	ScreenResX          int
	ScreenResY          int
	CurrentIP           string
	ProxyID             *int
	WifiSSID            string
	AdbPort             int
	LastHeartbeat       *time.Time
	HeartbeatCount      int
	RecoveryInProgress  bool
	LastErrorHash       string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ReadyAt             *time.Time
	RetiredAt           *time.Time
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
