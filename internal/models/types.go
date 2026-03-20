package models

type ShareMode string

const (
	ShareModeStable       ShareMode = "stable"
	ShareModeRandomDomain ShareMode = "random-domain"
	ShareModeQuick        ShareMode = "quick"
)

type ProjectPreset struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	LocalHost   string    `json:"localHost"`
	Subdomain   string    `json:"subdomain"`
	PublicURL   string    `json:"publicURL"`
	ProjectPath string    `json:"projectPath"`
	ShareMode   ShareMode `json:"shareMode"`
}

type AppSettings struct {
	DefaultDomain     string          `json:"defaultDomain"`
	TunnelName        string          `json:"tunnelName"`
	CloudflaredPath   string          `json:"cloudflaredPath"`
	DefaultServiceURL string          `json:"defaultServiceURL"`
	LicenseToken      string          `json:"licenseToken,omitempty"`
	Projects          []ProjectPreset `json:"projects"`
}

func DefaultSettings() AppSettings {
	return AppSettings{
		DefaultDomain:     "reaksmeykem.dev",
		TunnelName:        "laravel-herd",
		CloudflaredPath:   "",
		DefaultServiceURL: "http://127.0.0.1:80",
		Projects:          []ProjectPreset{},
	}
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type TunnelStatus struct {
	TunnelName              string     `json:"tunnelName"`
	TunnelID                string     `json:"tunnelId"`
	Running                 bool       `json:"running"`
	Mode                    string     `json:"mode"`
	PID                     int        `json:"pid"`
	ActiveURL               string     `json:"activeUrl"`
	QuickURL                string     `json:"quickUrl"`
	ActiveHostnames         []string   `json:"activeHostnames"`
	LastLogs                []LogEntry `json:"lastLogs"`
	LastError               string     `json:"lastError"`
	DetectedCloudflaredPath string     `json:"detectedCloudflaredPath"`
	ConfigPath              string     `json:"configPath"`
}

type AppState struct {
	Settings               AppSettings  `json:"settings"`
	Status                 TunnelStatus `json:"status"`
	License                LicenseState `json:"license"`
	ConfigPath             string       `json:"configPath"`
	SettingsPath           string       `json:"settingsPath"`
	HomeDir                string       `json:"homeDir"`
	ManagedCloudflaredPath string       `json:"managedCloudflaredPath"`
	CloudflaredDetected    bool         `json:"cloudflaredDetected"`
	CloudflaredPath        string       `json:"cloudflaredPath"`
	ConfigReadable         bool         `json:"configReadable"`
	ConfigReadError        string       `json:"configReadError"`
	BuildRunning           bool         `json:"buildRunning"`
	BuildCommandDetected   bool         `json:"buildCommandDetected"`
	ProductVersion         string       `json:"productVersion"`
}

type LicenseState struct {
	Valid      bool   `json:"valid"`
	IsAdmin    bool   `json:"isAdmin"`
	Owner      string `json:"owner"`
	Plan       string `json:"plan"`
	ExpiresAt  string `json:"expiresAt"`
	DeviceID   string `json:"deviceId"`
	Message    string `json:"message"`
	Configured bool   `json:"configured"`
}
