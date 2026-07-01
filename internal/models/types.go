package models

type ShareMode string

const (
	ShareModeAuto         ShareMode = "auto"
	ShareModeStable       ShareMode = "stable"
	ShareModeRandomDomain ShareMode = "random-domain"
	ShareModeQuick        ShareMode = "quick"
	ShareModeHostHTML     ShareMode = "host-html"
)

type ProjectPreset struct {
	ID           string    `json:"id"`
	DisplayName  string    `json:"displayName"`
	LocalHost    string    `json:"localHost"`
	OriginURL    string    `json:"originURL"`
	Subdomain    string    `json:"subdomain"`
	PublicURL    string    `json:"publicURL"`
	ProjectPath  string    `json:"projectPath"`
	LocalURL     string    `json:"localURL"`
	StartCommand string    `json:"startCommand"`
	ShareMode    ShareMode `json:"shareMode"`
}

type AppSettings struct {
	DefaultDomain         string          `json:"defaultDomain"`
	TunnelName            string          `json:"tunnelName"`
	CloudflaredPath       string          `json:"cloudflaredPath"`
	DefaultServiceURL     string          `json:"defaultServiceURL"`
	InsecureSkipOriginTLS bool            `json:"insecureSkipOriginTls"`
	LicenseToken          string          `json:"licenseToken,omitempty"`
	Language              string          `json:"language"`
	Theme                 string          `json:"theme"`
	Projects              []ProjectPreset `json:"projects"`
}

func DefaultSettings() AppSettings {
	return AppSettings{
		DefaultDomain:         "example.com",
		TunnelName:            "exposely",
		CloudflaredPath:       "",
		DefaultServiceURL:     "http://127.0.0.1:80",
		InsecureSkipOriginTLS: false,
		Language:              "en",
		Theme:                 "dark",
		Projects:              []ProjectPreset{},
	}
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type TunnelStatus struct {
	TunnelName              string       `json:"tunnelName"`
	TunnelID                string       `json:"tunnelId"`
	Running                 bool         `json:"running"`
	Mode                    string       `json:"mode"`
	PID                     int          `json:"pid"`
	ActiveURL               string       `json:"activeUrl"`
	QuickURL                string       `json:"quickUrl"`
	HTMLServerPort          int          `json:"htmlServerPort"`
	ActiveHostnames         []string     `json:"activeHostnames"`
	LastLogs                []LogEntry   `json:"lastLogs"`
	LastError               string       `json:"lastError"`
	DetectedCloudflaredPath string       `json:"detectedCloudflaredPath"`
	ConfigPath              string       `json:"configPath"`
	Usage                   *TunnelUsage `json:"usage,omitempty"`
	// Legacy EnvKit-specific status fields. Populated only when the
	// detected local stack is specifically EnvKit so existing UI keeps
	// rendering the same way. New code should read the LocalStack*
	// fields below for stack-agnostic behaviour.
	EnvKitDetected  bool   `json:"envkitDetected"`
	EnvKitVersion   string `json:"envkitVersion"`
	EnvKitPath      string `json:"envkitPath"`
	EnvKitOriginURL string `json:"envkitOriginUrl"`
	// LocalStack* fields describe whichever local dev stack Exposely
	// auto-detected on the host (EnvKit, Laravel Herd, Laravel Valet,
	// Laragon, or a generic HTTPS listener on 127.0.0.1:443). They are
	// populated whenever a stack was found, regardless of which one.
	LocalStackDetected  bool   `json:"localStackDetected"`
	LocalStackKind      string `json:"localStackKind"`
	LocalStackName      string `json:"localStackName"`
	LocalStackVersion   string `json:"localStackVersion"`
	LocalStackPath      string `json:"localStackPath"`
	LocalStackOriginURL string `json:"localStackOriginUrl"`
}

// TunnelUsage captures live usage counters exposed by the local cloudflared
// process via its Prometheus metrics endpoint. Numbers are per-process and
// reset when the tunnel restarts. No Cloudflare account or login is required.
type TunnelUsage struct {
	Available       bool              `json:"available"`
	StartedAt       string            `json:"startedAt"`
	UptimeSeconds   int64             `json:"uptimeSeconds"`
	TotalRequests   uint64            `json:"totalRequests"`
	RequestsPerMin  float64           `json:"requestsPerMin"`
	ActiveConns     int64             `json:"activeConns"`
	HAConnections   int64             `json:"haConnections"`
	ResponsesByCode map[string]uint64 `json:"responsesByCode,omitempty"`
	EdgeLocations   []string          `json:"edgeLocations,omitempty"`
	LastUpdated     string            `json:"lastUpdated"`
	MetricsAddr     string            `json:"metricsAddr,omitempty"`
	Note            string            `json:"note,omitempty"`
}

type AppState struct {
	Settings               AppSettings  `json:"settings"`
	Status                 TunnelStatus `json:"status"`
	License                LicenseState `json:"license"`
	Update                 UpdateInfo   `json:"update"`
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

type UpdateInfo struct {
	Checked        bool   `json:"checked"`
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseUrl"`
	Message        string `json:"message"`
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
