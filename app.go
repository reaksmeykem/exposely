package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/reaksmeykem/exposely/internal/cloudflare"
	applicense "github.com/reaksmeykem/exposely/internal/license"
	"github.com/reaksmeykem/exposely/internal/localstack"
	"github.com/reaksmeykem/exposely/internal/models"
	"github.com/reaksmeykem/exposely/internal/settings"
	"github.com/reaksmeykem/exposely/internal/stacks"
	"github.com/reaksmeykem/exposely/internal/sysproc"
	"github.com/reaksmeykem/exposely/internal/version"
)

const embeddedLicensePublicKey = ""

var localServiceURLPattern = regexp.MustCompile(`https?://(?:localhost|127\.0\.0\.1)(?::\d+)?(?:/[^\s"'<>]*)?`)

type App struct {
	ctx         context.Context
	store       *settings.Store
	manager     *cloudflare.Manager
	configPath  string
	homeDir     string
	appDataDir  string
	deviceID    string
	buildMu     sync.Mutex
	buildCmd    *exec.Cmd
	projectMu   sync.Mutex
	projectCmd  *exec.Cmd
	lastStateMu sync.Mutex
	updateMu      sync.RWMutex
	updateInfo    models.UpdateInfo
	localStackMu  sync.RWMutex
	localStack    localstack.Info
	stacks        *stacks.Manager
}

func NewApp() (*App, error) {
	_ = loadDotEnv(".env")

	store, err := settings.NewStore("Exposely")
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	app := &App{
		store:      store,
		configPath: filepath.Join(homeDir, ".cloudflared", "config.yml"),
		homeDir:    homeDir,
		appDataDir: filepath.Dir(store.Path()),
		deviceID:   resolveDeviceID(),
	}
	app.manager = cloudflare.NewManager(app.configPath, app.pushLog, app.pushStatus)
	app.localStack = localstack.Detect()
	app.stacks = stacks.NewManager()
	return app, nil
}

// applyStackConfigs registers the user's stack binary paths with the
// stacks.Manager and generates the nginx.conf Exposely will pass to
// nginx at start time. Called from RefreshState so settings edited in
// the UI (or settings.json edited by hand) take effect without a
// restart.
func (a *App) applyStackConfigs(settingsValue models.AppSettings) {
	if a.stacks == nil {
		a.stacks = stacks.NewManager()
	}
	stack := settingsValue.Stack

	if strings.TrimSpace(stack.NginxBinaryPath) != "" {
		nginxRoot := filepath.Dir(strings.TrimSpace(stack.NginxBinaryPath))
		// Every registered project vhost; the default www site only when
		// nothing in the registry claims localhost (nginx routes on the
		// first matching server block).
		registry := stacks.LoadSiteRegistry(a.appDataDir)
		usePHP := strings.TrimSpace(stack.PHPCGIBinaryPath) != ""
		sites := make([]stacks.SiteConfig, 0, len(registry.Sites)+1)
		hasLocalhost := false
		for _, entry := range registry.Sites {
			if strings.EqualFold(entry.ServerName, "localhost") {
				hasLocalhost = true
			}
		}
		if !hasLocalhost {
			sites = append(sites, stacks.SiteConfig{
				ServerName: "localhost",
				Root:       filepath.Join(a.appDataDir, "stacks", "www"),
				ListenPort: stack.EffectiveNginxPort(),
				PHP:        usePHP,
				PHPPort:    stack.EffectivePHPPort(),
				Index:      []string{"index.html", "index.php"},
			})
		}
		for _, entry := range registry.Sites {
			sites = append(sites, stacks.SiteConfig{
				ServerName: entry.ServerName,
				Root:       entry.Root,
				PHP:        usePHP && entry.PHP,
				PHPPort:    stack.EffectivePHPPort(),
				ListenPort: stack.EffectiveNginxPort(),
				Index:      []string{"index.html", "index.php"},
			})
		}
		confPath, err := a.stackNginxConfPath()
		if err == nil {
			conf := stacks.RenderNginxConf(nginxRoot, stack.EffectiveNginxPort(), sites)
			if writeErr := stacks.WriteFile(confPath, conf); writeErr == nil {
				a.stacks.SetConfig(stacks.ServiceNginx, stacks.ServiceConfig{
					BinaryPath: stack.NginxBinaryPath,
					Args:       []string{"-p", nginxRoot, "-c", confPath},
				})
			}
		}
	} else {
		a.stacks.SetConfig(stacks.ServiceNginx, stacks.ServiceConfig{})
	}

	if strings.TrimSpace(stack.PHPCGIBinaryPath) != "" {
		a.stacks.SetConfig(stacks.ServicePHP, stacks.ServiceConfig{
			BinaryPath: stack.PHPCGIBinaryPath,
			Args:       stacks.PHPStartArgs(stack.EffectivePHPPort(), stack.EffectivePHPWorkers()),
		})
	} else {
		a.stacks.SetConfig(stacks.ServicePHP, stacks.ServiceConfig{})
	}

	if strings.TrimSpace(stack.MySQLDBinaryPath) != "" {
		mysqlArgs := stacks.MySQLStartArgs(stacks.MySQLDefaults{
			BaseDir: filepath.Dir(strings.TrimSpace(stack.MySQLDBinaryPath)),
			DataDir: a.stackMySQLDataDir(),
			Port:    stack.EffectiveMySQLPort(),
		})
		a.stacks.SetConfig(stacks.ServiceMySQL, stacks.ServiceConfig{
			BinaryPath: stack.MySQLDBinaryPath,
			Args:       mysqlArgs,
			Env:        []string{"MYSQL_HOME=" + filepath.Dir(strings.TrimSpace(stack.MySQLDBinaryPath))},
		})
	} else {
		a.stacks.SetConfig(stacks.ServiceMySQL, stacks.ServiceConfig{})
	}
}

func (a *App) stackNginxConfPath() (string, error) {
	if strings.TrimSpace(a.appDataDir) == "" {
		return "", errors.New("app data dir is not initialised yet")
	}
	return filepath.Join(a.appDataDir, "stacks", "nginx", "nginx.conf"), nil
}

func (a *App) stackMySQLDataDir() string {
	return filepath.Join(a.appDataDir, "stacks", "mysql", "data")
}

// StackStatus reports the current state of the Exposely-managed stack
// services. Safe to call before any service is configured.
func (a *App) StackStatus() []stacks.Status {
	if a.stacks == nil {
		return []stacks.Status{}
	}
	return a.stacks.AllStatus()
}

// StartStackService starts one of "nginx", "php", or "mysql". For mysql
// it first initialises the data dir when needed. Returns the refreshed
// AppState so the UI updates in one round-trip.
func (a *App) StartStackService(service string) (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	a.applyStackConfigs(settingsValue)

	normalized := stacks.Service(strings.ToLower(strings.TrimSpace(service)))
	switch normalized {
	case stacks.ServiceNginx, stacks.ServicePHP:
		// nothing extra
	case stacks.ServiceMySQL:
		cfg, _ := a.stacks.Config(stacks.ServiceMySQL)
		if strings.TrimSpace(cfg.BinaryPath) == "" {
			return models.AppState{}, errors.New("mysqld binary path is not configured in Settings")
		}
		dataDir := a.stackMySQLDataDir()
		a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "info", Message: "Ensuring MySQL data directory at " + dataDir})
		if err := stacks.EnsureMySQLDataDir(cfg.BinaryPath, dataDir); err != nil {
			return models.AppState{}, err
		}
	default:
		return models.AppState{}, fmt.Errorf("unknown stack service %q (use nginx, php, or mysql)", service)
	}

	status := a.stacks.Start(normalized)
	if !status.Running && status.LastError != "" {
		return models.AppState{}, errors.New(status.LastError)
	}
	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "success", Message: "Started " + string(normalized)})
	return a.RefreshState()
}

// StartStack brings the whole managed stack up in dependency order:
// MySQL first, then PHP, then nginx.
func (a *App) StartStack() (models.AppState, error) {
	for _, service := range []stacks.Service{stacks.ServiceMySQL, stacks.ServicePHP, stacks.ServiceNginx} {
		if _, err := a.StartStackService(string(service)); err != nil {
			return models.AppState{}, err
		}
	}
	return a.RefreshState()
}

// StopStackService stops one managed service by name.
func (a *App) StopStackService(service string) (models.AppState, error) {
	if a.stacks == nil {
		return models.AppState{}, errors.New("stack manager is not running")
	}
	normalized := stacks.Service(strings.ToLower(strings.TrimSpace(service)))
	switch normalized {
	case stacks.ServiceNginx, stacks.ServicePHP, stacks.ServiceMySQL:
	default:
		return models.AppState{}, fmt.Errorf("unknown stack service %q (use nginx, php, or mysql)", service)
	}
	a.stacks.Stop(normalized)
	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "info", Message: "Stopped " + string(normalized)})
	return a.RefreshState()
}

// StopStack stops every managed stack service in reverse dependency
// order (nginx first, then PHP, then MySQL).
func (a *App) StopStack() (models.AppState, error) {
	if a.stacks == nil {
		return models.AppState{}, errors.New("stack manager is not running")
	}
	a.stacks.StopAll()
	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "info", Message: "Stopped managed stack services"})
	return a.RefreshState()
}

// refreshLocalStack re-runs the local-stack detector and stores the
// result on the app. Callers use this from share flows so newly-installed
// stacks (EnvKit / Herd / Valet / Laragon / generic HTTPS:443 listener)
// are picked up without restarting Exposely.
func (a *App) refreshLocalStack() localstack.Info {
	info := localstack.Detect()
	a.localStackMu.Lock()
	a.localStack = info
	a.localStackMu.Unlock()
	return info
}

// localStackInfo returns the latest cached local-stack detection snapshot.
func (a *App) localStackInfo() localstack.Info {
	a.localStackMu.RLock()
	defer a.localStackMu.RUnlock()
	return a.localStack
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_, _ = a.RefreshState()
}

func (a *App) shutdown(context.Context) {
	_ = a.manager.StopTunnel()
	a.stopProjectCommand()
	a.stopBuild()
	if a.stacks != nil {
		a.stacks.StopAll()
	}
}

func (a *App) Bootstrap() (models.AppState, error) {
	return a.RefreshState()
}

func (a *App) RefreshState() (models.AppState, error) {
	cfg, cfgErr := cloudflare.ReadConfig(a.configPath)
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	settingsValue = a.normalizeSettings(settingsValue)
	if saveErr := a.store.Save(settingsValue); saveErr != nil {
		return models.AppState{}, saveErr
	}
	a.applyStackConfigs(settingsValue)

	detectedPath, detectErr := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if detectErr == nil && detectedPath != settingsValue.CloudflaredPath {
		settingsValue.CloudflaredPath = detectedPath
		_ = a.store.Save(settingsValue)
	}

	status := a.manager.Status()
	licenseState := a.resolveLicenseState(settingsValue.LicenseToken)
	status.TunnelName = settingsValue.TunnelName
	status.ConfigPath = a.configPath
	if detectErr == nil {
		status.DetectedCloudflaredPath = detectedPath
	}

	// Refresh local-stack detection so the UI sees installs (or live
	// HTTPS listeners) that appeared after Exposely was launched.
	stack := a.refreshLocalStack()
	// Legacy EnvKit-specific fields: only populated when the detected
	// stack is specifically EnvKit, preserving existing behaviour.
	if stack.IsEnvKit() {
		status.EnvKitDetected = true
		status.EnvKitVersion = stack.Version
		status.EnvKitPath = stack.InstallPath
		status.EnvKitOriginURL = stack.SuggestedOriginURL
	}
	// New generic local-stack fields: populated for any detected stack.
	status.LocalStackDetected = stack.Detected
	status.LocalStackKind = string(stack.Kind)
	status.LocalStackName = stack.Name
	status.LocalStackVersion = stack.Version
	status.LocalStackPath = stack.InstallPath
	status.LocalStackOriginURL = stack.SuggestedOriginURL
	if cfgErr == nil {
		status.ActiveHostnames = cloudflare.HostnamesFromConfig(cfg)
		status.TunnelID = cfg.Tunnel
	}
	if cfgErr != nil && !errors.Is(cfgErr, os.ErrNotExist) {
		status.LastError = cfgErr.Error()
	}
	if detectErr != nil {
		status.LastError = detectErr.Error()
	}

	a.lastStateMu.Lock()
	defer a.lastStateMu.Unlock()
	return models.AppState{
		Settings:               settingsValue,
		Status:                 status,
		License:                licenseState,
		Update:                 a.currentUpdateInfo(),
		ConfigPath:             a.configPath,
		SettingsPath:           a.store.Path(),
		HomeDir:                a.homeDir,
		ManagedCloudflaredPath: a.managedCloudflaredPath(),
		CloudflaredDetected:    detectErr == nil,
		CloudflaredPath:        detectedPath,
		ConfigReadable:         cfgErr == nil || errors.Is(cfgErr, os.ErrNotExist),
		ConfigReadError:        errorString(cfgErr),
		BuildRunning:           a.isBuildRunning(),
		BuildCommandDetected:   a.detectNpmCommand() != "",
		ProductVersion:         version.Version,
	}, nil
}

func (a *App) CheckForUpdates() (models.AppState, error) {
	info := models.UpdateInfo{
		Checked:        true,
		Available:      false,
		CurrentVersion: version.Version,
		LatestVersion:  version.Version,
		ReleaseURL:     version.ReleasePageURL,
	}

	latest, err := fetchLatestReleaseInfo()
	if err != nil {
		info.Message = "Could not check for updates right now."
		a.setUpdateInfo(info)
		return a.RefreshState()
	}

	info.LatestVersion = latest.TagName
	info.ReleaseURL = latest.HTMLURL
	if isVersionNewer(version.Version, latest.TagName) {
		info.Available = true
		info.Message = fmt.Sprintf("Exposely %s is available.", latest.TagName)
	} else {
		info.Message = "You are using the latest version."
	}

	a.setUpdateInfo(info)
	return a.RefreshState()
}

func (a *App) OpenLatestRelease() error {
	info := a.currentUpdateInfo()
	target := strings.TrimSpace(info.ReleaseURL)
	if target == "" {
		target = version.ReleasePageURL
	}
	return openExternal(target)
}

func (a *App) InstallLatestUpdate() (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("desktop auto-update is currently supported on Windows only")
	}

	latest, err := fetchLatestReleaseInfo()
	if err != nil {
		return "", err
	}
	if !isVersionNewer(version.Version, latest.TagName) {
		return "", errors.New("you are already using the latest version")
	}

	asset, err := selectDesktopUpdateAsset(latest)
	if err != nil {
		return "", err
	}

	a.pushLog(models.LogEntry{
		Timestamp: nowStamp(),
		Source:    "updater",
		Level:     "info",
		Message:   fmt.Sprintf("Downloading desktop update %s (%s)", latest.TagName, asset.Name),
	})

	downloadPath, err := a.downloadReleaseAsset(latest.TagName, asset)
	if err != nil {
		return "", err
	}

	isBinary := strings.EqualFold(strings.TrimSpace(asset.Name), "Exposely.exe")
	if isBinary {
		exePath, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("could not determine current executable path: %w", err)
		}
		if err := launchWindowsReplace(os.Getpid(), exePath, downloadPath); err != nil {
			return "", err
		}
		a.pushLog(models.LogEntry{
			Timestamp: nowStamp(),
			Source:    "updater",
			Level:     "info",
			Message:   fmt.Sprintf("Scheduled replacement of %s with %s", exePath, downloadPath),
		})
		wruntime.Quit(a.ctx)
		return fmt.Sprintf("Exposely will restart to complete the update to %s.", latest.TagName), nil
	}

	if err := launchExecutable(downloadPath); err != nil {
		return "", err
	}

	a.pushLog(models.LogEntry{
		Timestamp: nowStamp(),
		Source:    "updater",
		Level:     "info",
		Message:   fmt.Sprintf("Launched update installer: %s", downloadPath),
	})

	wruntime.Quit(a.ctx)
	return fmt.Sprintf("Opened %s. Finish the installer to update Exposely.", asset.Name), nil
}

func (a *App) SaveSettings(input models.AppSettings) (models.AppState, error) {
	if err := a.requireAdmin(); err != nil {
		return models.AppState{}, err
	}
	settingsValue := a.normalizeSettings(input)
	if err := a.store.Save(settingsValue); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) SaveProject(input models.ProjectPreset) (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}

	if strings.TrimSpace(input.DisplayName) == "" {
		return models.AppState{}, errors.New("display name is required")
	}
	if normalizeShareMode(input.ShareMode) == models.ShareModeQuick && strings.TrimSpace(input.LocalHost) == "" {
		return models.AppState{}, errors.New("local host is required")
	}
	if strings.TrimSpace(input.StartCommand) != "" && strings.TrimSpace(input.ProjectPath) == "" {
		return models.AppState{}, errors.New("project folder path is required when a start command is set")
	}
	if err := validateProjectSource(input); err != nil {
		return models.AppState{}, err
	}

	input.ID = ensureID(input.ID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.LocalHost = strings.TrimSpace(input.LocalHost)
	input.OriginURL = strings.TrimSpace(input.OriginURL)
	input.ProjectPath = strings.TrimSpace(input.ProjectPath)
	input.LocalURL = strings.TrimSpace(input.LocalURL)
	input.StartCommand = strings.TrimSpace(input.StartCommand)
	input.Subdomain = strings.TrimSpace(strings.ToLower(input.Subdomain))
	input.ShareMode = normalizeShareMode(input.ShareMode)
	input.PublicURL = a.projectPublicURL(input, settingsValue.DefaultDomain)

	replaced := false
	for i := range settingsValue.Projects {
		if settingsValue.Projects[i].ID == input.ID {
			settingsValue.Projects[i] = input
			replaced = true
			break
		}
	}
	if !replaced {
		settingsValue.Projects = append(settingsValue.Projects, input)
	}

	if err := a.store.Save(a.normalizeSettings(settingsValue)); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) DeleteProject(id string) (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}

	filtered := settingsValue.Projects[:0]
	for _, project := range settingsValue.Projects {
		if project.ID != id {
			filtered = append(filtered, project)
		}
	}
	settingsValue.Projects = filtered
	if err := a.store.Save(a.normalizeSettings(settingsValue)); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) ShareProject(projectID string) (models.AppState, error) {
	if err := a.requireAdmin(); err != nil {
		return models.AppState{}, err
	}
	settingsValue, project, err := a.loadProject(projectID)
	if err != nil {
		return models.AppState{}, err
	}

	switch normalizeShareMode(project.ShareMode) {
	case models.ShareModeAuto:
		return a.startAutoTunnel(project)
	case models.ShareModeQuick:
		return a.startQuickTunnel(project)
	case models.ShareModeHostHTML:
		return a.startHTMLTunnel(project)
	case models.ShareModeRandomDomain:
		return a.shareProjectThroughNamedTunnel(settingsValue, project, true)
	default:
		return a.shareProjectThroughNamedTunnel(settingsValue, project, false)
	}
}

func (a *App) startHTMLTunnel(project models.ProjectPreset) (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return models.AppState{}, err
	}
	if a.manager.Status().Running {
		_ = a.manager.StopTunnel()
	}
	a.stopProjectCommand()

	serviceURL, port, server, err := a.resolveHTMLOrigin(project)
	if err != nil {
		return models.AppState{}, err
	}
	if server == nil {
		if err := checkLocalHTTPService(serviceURL); err != nil {
			return models.AppState{}, fmt.Errorf("HTML local URL is not reachable: %w", err)
		}
	}

	if err := a.manager.StartQuickTunnelWithHTML(path, serviceURL, "", port, server, cloudflare.QuickTunnelOptions{InsecureSkipOriginTLS: settingsValue.InsecureSkipOriginTLS}); err != nil {
		if server != nil {
			_ = server.Shutdown(context.Background())
		}
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) startAutoTunnel(project models.ProjectPreset) (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return models.AppState{}, err
	}
	if a.manager.Status().Running {
		_ = a.manager.StopTunnel()
	}
	a.stopProjectCommand()

	if strings.TrimSpace(project.StartCommand) != "" {
		serviceURL, err := a.startProjectAndDetectURL(project)
		if err != nil {
			return models.AppState{}, err
		}
		if err := a.manager.StartQuickTunnelWithHTML(path, serviceURL, "", 0, nil, cloudflare.QuickTunnelOptions{InsecureSkipOriginTLS: settingsValue.InsecureSkipOriginTLS}); err != nil {
			a.stopProjectCommand()
			return models.AppState{}, err
		}
		return a.RefreshState()
	}

	if serviceURL, ok, err := resolveProjectServiceURL(project); ok {
		if err != nil {
			return models.AppState{}, err
		}
		if err := checkLocalHTTPService(serviceURL); err != nil {
			return models.AppState{}, fmt.Errorf("local URL is not reachable: %w", err)
		}
		if err := a.manager.StartQuickTunnelWithHTML(path, serviceURL, "", 0, nil, cloudflare.QuickTunnelOptions{InsecureSkipOriginTLS: settingsValue.InsecureSkipOriginTLS}); err != nil {
			return models.AppState{}, err
		}
		return a.RefreshState()
	}

	if strings.TrimSpace(project.LocalHost) != "" {
		return a.startQuickTunnel(project)
	}

	projectDir, err := a.resolveProjectDirectory(project.ProjectPath)
	if err != nil {
		return models.AppState{}, err
	}
	// Before assuming the user wants a stack-based *.test setup, sniff
	// the loopback for a live dev server. This is the escape hatch for
	// "normal Laravel" (php artisan serve on :8000), Vite, React,
	// Angular, and anyone else who ran their dev server first and then
	// hit `exposely share` without configuring a Local URL. If we find
	// one, we tunnel it directly — same behaviour as if the user had
	// explicitly set project.LocalURL. Gated by the devServerFallback
	// opt-out and the host-aware identity check.
	if settingsValue.DevServerFallbackEnabled() {
		if serviceURL, ok := probeRunningDevServer(preferredDevServerPorts(projectDir)); ok {
			a.pushLog(models.LogEntry{
				Timestamp: nowStamp(),
				Source:    "auto",
				Level:     "success",
				Message:   fmt.Sprintf("Detected running local dev server at %s — tunneling that instead of the auto-inferred .test host", serviceURL),
			})
			if err := a.manager.StartQuickTunnelWithHTML(path, serviceURL, "", 0, nil, cloudflare.QuickTunnelOptions{InsecureSkipOriginTLS: settingsValue.InsecureSkipOriginTLS}); err != nil {
				return models.AppState{}, err
			}
			return a.RefreshState()
		}
	}
	if looksLikeLaravelProjectDir(projectDir) {
		laravelProject := project
		laravelProject.LocalHost = inferLocalHostFromProjectPath(projectDir)
		if strings.TrimSpace(laravelProject.LocalHost) == "" {
			return models.AppState{}, errors.New("Auto mode detected a Laravel project but could not infer a local host. Set a local host such as app.test")
		}
		return a.startQuickTunnel(laravelProject)
	}
	if staticDir, ok := detectStaticSiteDir(projectDir); ok {
		serviceURL, port, server, err := a.serveStaticDirectory(staticDir)
		if err != nil {
			return models.AppState{}, err
		}
		if err := a.manager.StartQuickTunnelWithHTML(path, serviceURL, "", port, server, cloudflare.QuickTunnelOptions{InsecureSkipOriginTLS: settingsValue.InsecureSkipOriginTLS}); err != nil {
			_ = server.Shutdown(context.Background())
			return models.AppState{}, err
		}
		return a.RefreshState()
	}

	return models.AppState{}, errors.New("Auto mode could not determine how to run this project. Set a local URL, set a start command, provide a local host, or point to a folder with index.html/dist/build output")
}

func (a *App) resolveHTMLOrigin(project models.ProjectPreset) (string, int, *http.Server, error) {
	if serviceURL, ok, err := resolveProjectServiceURL(project); ok {
		if err != nil {
			return "", 0, nil, err
		}
		a.pushLog(models.LogEntry{
			Timestamp: nowStamp(),
			Source:    "html-server",
			Level:     "info",
			Message:   fmt.Sprintf("Using existing local HTML server at %s", serviceURL),
		})
		return serviceURL, 0, nil, nil
	}

	projectDir, err := a.resolveProjectDirectory(project.ProjectPath)
	if err != nil {
		return "", 0, nil, err
	}
	staticDir, ok := detectStaticSiteDir(projectDir)
	if !ok {
		return "", 0, nil, errors.New("HTML mode could not find index.html in the selected folder or common output folders (dist, build, public)")
	}
	return a.serveStaticDirectory(staticDir)
}

func resolveProjectServiceURL(project models.ProjectPreset) (string, bool, error) {
	if serviceURL, ok, err := normalizeServiceURL(project.LocalURL); ok {
		return serviceURL, ok, err
	}
	return normalizeServiceURL(project.ProjectPath)
}

func normalizeServiceURL(raw string) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, nil
	}
	if len(trimmed) >= 2 && trimmed[1] == ':' {
		return "", false, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		if strings.Contains(trimmed, "://") {
			return "", true, fmt.Errorf("invalid local URL: %w", err)
		}
		return "", false, nil
	}

	if parsed.Scheme == "" {
		return "", false, nil
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", true, errors.New("local URL must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", true, errors.New("local URL must include a host")
	}

	normalized := &url.URL{
		Scheme: scheme,
		Host:   parsed.Host,
	}
	return normalized.String(), true, nil
}

func checkLocalHTTPService(serviceURL string) error {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(serviceURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("%s responded with HTTP %d", serviceURL, resp.StatusCode)
	}
	return nil
}

// checkLocalOriginReachable verifies that the upstream cloudflared will
// tunnel to is actually answering before we declare the share started.
// Without this pre-flight the user gets a tunnel URL that 502s because the
// upstream web server is offline. The hostHeader is used as the TLS SNI
// when the upstream is HTTPS on the loopback address (EnvKit / Herd /
// Valet / Laragon / generic 127.0.0.1:443 setups), so the cert for *.test
// validates.
//
// When stack.Detected is true and the upstream is unreachable the error
// message is annotated with a stack-aware hint (e.g. "start EnvKit's
// nginx", "start Herd", or a generic "make sure your local web server is
// running" for unidentified loopback HTTPS).
//
// When insecureSkipOriginTLS is true the pre-flight dials without
// validating the cert — this matches the flag that will be passed to
// cloudflared via --no-tls-verify, so we don't reject a share we are
// about to allow anyway.
func checkLocalOriginReachable(serviceURL, hostHeader string, stack localstack.Info, insecureSkipOriginTLS bool) error {
	parsed, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid origin URL %q: %w", serviceURL, err)
	}

	sni := strings.TrimSpace(hostHeader)
	if sni == "" {
		sni = parsed.Hostname()
	}

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	if strings.EqualFold(parsed.Scheme, "https") {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName:         sni,
				InsecureSkipVerify: insecureSkipOriginTLS,
				// Keep the system trust store so the local stack's CA
				// still works — we only override SNI here, not verification.
			},
		}
	}

	req, err := http.NewRequest(http.MethodGet, serviceURL, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(hostHeader) != "" {
		req.Host = hostHeader
		req.Header.Set("Host", hostHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return classifyUpstreamError(serviceURL, hostHeader, stack, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("%s responded with HTTP %d", serviceURL, resp.StatusCode)
	}
	return nil
}

// classifyUpstreamError turns a raw dial/HTTP error from checkLocalOriginReachable
// into actionable guidance for the user. We distinguish three failure modes:
//
//   - "no listener" — the upstream port is closed (the local web server
//     is off).
//   - "cert SAN mismatch" — the upstream is answering but its cert does
//     not cover the host the project is asking for (very common with
//     local stacks when a .test site is not registered).
//   - "other TLS error" — anything else in the TLS handshake.
//
// We inspect the underlying error string because Go's x509 package produces
// stable, parseable messages of the form:
// `x509: certificate is valid for A, B, C, not <requested>`.
//
// stack is consulted only to make the hint text more specific (e.g. "start
// EnvKit's nginx" vs "start Herd" vs a generic "start your local web
// server"). It does not change the error classification itself.
func classifyUpstreamError(serviceURL, hostHeader string, stack localstack.Info, rawErr error) error {
	msg := rawErr.Error()

	// Cert SAN mismatch: extract the valid hosts from the standard x509 wording.
	if strings.Contains(msg, "certificate is valid for") && strings.Contains(msg, ", not ") {
		validPart := extractValidHosts(msg)
		requested := strings.TrimSpace(hostHeader)
		if requested == "" {
			requested = "(unknown host)"
		}
		if stack.Detected && isLocalHTTPSOrigin(serviceURL) {
			label := stackHintLabel(stack)
			return fmt.Errorf(
				"%s's certificate at %s does not cover %q. Hosts covered by the cert: %s. "+
					"Please register %s with %s (or, in Exposely, set a custom OriginURL on the project that points at a server whose cert covers %s): %w",
				label, serviceURL, requested, validPart, requested, label, requested, rawErr,
			)
		}
		return fmt.Errorf(
			"the certificate at %s does not cover %q. Hosts covered by the cert: %s. "+
				"Please register %s with the upstream server or set a custom OriginURL on the project: %w",
			serviceURL, requested, validPart, requested, rawErr,
		)
	}

	// "no listener" path — the upstream port is closed.
	if isConnectionRefused(msg) || isNoRoute(msg) {
		// The "start your local web server" hint applies to any known
		// local stack whose upstream is our auto-suggested loopback
		// URL, whether that is HTTPS on 443 (EnvKit / Herd / Valet /
		// generic HTTPS listener) or HTTP on 80 (Laragon / plain
		// Nginx / Apache / generic HTTP listener). Restricting it to
		// HTTPS only would give a worse error message to "normal
		// Laravel" users behind HTTP-only stacks.
		if stack.Detected && isLocalLoopbackOrigin(serviceURL) {
			label := stackHintLabel(stack)
			action := stackHintAction(stack)
			// The upstream is our auto-swapped stack URL and it is
			// dead. Two common recoveries: start the stack, OR bypass
			// the stack entirely by pointing Exposely at the actual
			// running dev server (php artisan serve on :8000, Vite on
			// :5173, …). Surfacing both keeps the guidance useful for
			// users who are not running the detected stack right now.
			return fmt.Errorf(
				"%s does not appear to be running on %s. %s, "+
					"or point Exposely at your running dev server instead "+
					"(e.g. `exposely share --url http://127.0.0.1:8000` for "+
					"`php artisan serve`, or set a Local URL on the project): %w",
				label, serviceURL, action, rawErr,
			)
		}
		return fmt.Errorf("upstream %s is not reachable: %w", serviceURL, rawErr)
	}

	// Fallback: any other transport / TLS / DNS error.
	return fmt.Errorf("upstream %s is not reachable: %w", serviceURL, rawErr)
}

// stackHintLabel returns a human-readable name for the detected stack so
// error messages can say "EnvKit", "Laravel Herd", … instead of a generic
// phrase. Falls back to "EnvKit" when Kind is empty for back-compat with
// callers that build a bare Info{Detected:true} (existing tests do this).
func stackHintLabel(stack localstack.Info) string {
	if strings.TrimSpace(stack.Name) != "" {
		return stack.Name
	}
	switch stack.Kind {
	case localstack.KindHerd:
		return "Laravel Herd"
	case localstack.KindValet:
		return "Laravel Valet"
	case localstack.KindLaragon:
		return "Laragon"
	case localstack.KindHTTPSLoopback:
		return "Your local HTTPS service"
	case localstack.KindHTTPLoopback:
		return "Your local web server"
	case localstack.KindEnvKit, localstack.KindNone:
		fallthrough
	default:
		return "EnvKit"
	}
}

// stackHintAction returns a "do this to fix it" sentence tailored to the
// detected stack.
func stackHintAction(stack localstack.Info) string {
	switch stack.Kind {
	case localstack.KindHerd:
		return "Open Laravel Herd and make sure the services are running"
	case localstack.KindValet:
		return "Run `valet start` (and `valet park`/`valet link` for your site)"
	case localstack.KindLaragon:
		return "Open Laragon and click Start All"
	case localstack.KindHTTPSLoopback, localstack.KindHTTPLoopback:
		return "Start your local web server"
	case localstack.KindEnvKit, localstack.KindNone:
		fallthrough
	default:
		return "Open EnvKit and start nginx (or another web server)"
	}
}

// extractValidHosts pulls the comma-separated SAN list out of a Go x509
// error message of the form `...certificate is valid for A, B, C, not X`.
func extractValidHosts(message string) string {
	const marker = "certificate is valid for "
	start := strings.Index(message, marker)
	if start < 0 {
		return "(unknown)"
	}
	rest := message[start+len(marker):]
	end := strings.Index(rest, ", not ")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// isConnectionRefused returns true for errors that mean "nobody is listening
// on this port" — the most common cause when a local dev stack (EnvKit,
// Herd, Laragon, …) is installed but its web server is stopped.
func isConnectionRefused(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "actively refused") ||
		strings.Contains(lower, "no connection could be made")
}

// isNoRoute covers "no route to host" so we can label it the same way.
func isNoRoute(message string) bool {
	return strings.Contains(strings.ToLower(message), "no route to host")
}

// isLocalLoopbackOrigin returns true when the supplied URL matches either
// of the canonical local-stack loopback upstreams (https://127.0.0.1:443
// for TLS-terminating stacks like EnvKit / Herd / Valet, or
// http://127.0.0.1:80 for HTTP-only stacks like Laragon / plain Nginx /
// Apache). Used to decide whether to print stack-specific hints in error
// messages.
func isLocalLoopbackOrigin(serviceURL string) bool {
	trimmed := strings.TrimSpace(serviceURL)
	return strings.EqualFold(trimmed, localstack.LoopbackHTTPSOriginURL) ||
		strings.EqualFold(trimmed, localstack.LoopbackHTTPOriginURL)
}

// isLocalHTTPSOrigin is the historical name for the HTTPS-only variant of
// isLocalLoopbackOrigin. It is kept because a handful of tests / callers
// specifically care about the HTTPS case (e.g. to decide whether an
// upstream server-name override is needed). New code branching on "is
// this our auto-suggested loopback URL?" should use isLocalLoopbackOrigin.
func isLocalHTTPSOrigin(serviceURL string) bool {
	return strings.EqualFold(strings.TrimSpace(serviceURL), localstack.LoopbackHTTPSOriginURL)
}

// isEnvKitOrigin is the legacy name for isLocalHTTPSOrigin, kept to avoid
// breaking existing tests/callers.
func isEnvKitOrigin(serviceURL string) bool {
	return isLocalHTTPSOrigin(serviceURL)
}

func (a *App) resolveProjectDirectory(projectPath string) (string, error) {
	trimmed := strings.TrimSpace(projectPath)
	if trimmed == "" {
		return "", errors.New("project folder path is required")
	}

	absPath := trimmed
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(a.homeDir, absPath)
	}

	if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
		return "", fmt.Errorf("project folder not found or is not a directory: %s", absPath)
	}
	return absPath, nil
}

func detectStaticSiteDir(projectDir string) (string, bool) {
	candidates := []string{
		projectDir,
		filepath.Join(projectDir, "dist"),
		filepath.Join(projectDir, "build"),
		filepath.Join(projectDir, "public"),
	}

	for _, candidate := range candidates {
		indexPath := filepath.Join(candidate, "index.html")
		if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func (a *App) serveStaticDirectory(absPath string) (string, int, *http.Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		listener, err = net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			return "", 0, nil, fmt.Errorf("failed to find a free port for static site: %w", err)
		}
	}
	port := listener.Addr().(*net.TCPAddr).Port
	serviceURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a.pushLog(models.LogEntry{
				Timestamp: nowStamp(),
				Source:    "html-server",
				Level:     "info",
				Message:   fmt.Sprintf("Request: %s %s from %s [Host: %s]", r.Method, r.URL.Path, r.RemoteAddr, r.Host),
			})

			cleanPath := filepath.FromSlash(r.URL.Path)
			relPath := strings.TrimPrefix(cleanPath, string(filepath.Separator))
			fullPath := filepath.Join(absPath, relPath)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				indexFile := filepath.Join(fullPath, "index.html")
				if _, err := os.Stat(indexFile); err != nil {
					a.pushLog(models.LogEntry{
						Timestamp: nowStamp(),
						Source:    "html-server",
						Level:     "error",
						Message:   fmt.Sprintf("Directory accessed but index.html not found: %s", fullPath),
					})
				}
			} else if err != nil && os.IsNotExist(err) {
				a.pushLog(models.LogEntry{
					Timestamp: nowStamp(),
					Source:    "html-server",
					Level:     "error",
					Message:   fmt.Sprintf("File not found: %s", fullPath),
				})
			}

			http.FileServer(http.Dir(absPath)).ServeHTTP(w, r)
		}),
	}

	go func() {
		a.pushLog(models.LogEntry{
			Timestamp: nowStamp(),
			Source:    "html-server",
			Level:     "info",
			Message:   fmt.Sprintf("Static server started on %s for %s", serviceURL, absPath),
		})
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.pushLog(models.LogEntry{
				Timestamp: nowStamp(),
				Source:    "html-server",
				Level:     "error",
				Message:   fmt.Sprintf("Static server failed: %v", err),
			})
		}
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := checkLocalHTTPService(serviceURL); err != nil {
			a.pushLog(models.LogEntry{
				Timestamp: nowStamp(),
				Source:    "html-server",
				Level:     "error",
				Message:   fmt.Sprintf("Self-connectivity check failed for %s: %v", serviceURL, err),
			})
			return
		}
		a.pushLog(models.LogEntry{
			Timestamp: nowStamp(),
			Source:    "html-server",
			Level:     "info",
			Message:   fmt.Sprintf("Self-connectivity check successful for %s", serviceURL),
		})
	}()

	return serviceURL, port, server, nil
}

func (a *App) startProjectAndDetectURL(project models.ProjectPreset) (string, error) {
	projectDir, err := a.resolveProjectDirectory(project.ProjectPath)
	if err != nil {
		return "", err
	}

	urlCh, err := a.startProjectCommand(projectDir, project.StartCommand)
	if err != nil {
		return "", err
	}

	return a.waitForProjectServiceURL(project, urlCh)
}

func (a *App) startProjectCommand(projectDir, commandText string) (<-chan string, error) {
	a.projectMu.Lock()
	defer a.projectMu.Unlock()

	if a.projectCmd != nil && a.projectCmd.Process != nil {
		return nil, errors.New("a project start command is already running")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", commandText)
		cmd.SysProcAttr = sysproc.Hidden()
	} else {
		cmd = exec.Command("sh", "-lc", commandText)
	}
	cmd.Dir = projectDir
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	a.projectCmd = cmd
	urlCh := make(chan string, 8)
	a.pushLog(models.LogEntry{
		Timestamp: nowStamp(),
		Source:    "project",
		Level:     "info",
		Message:   "Started project command: " + commandText,
	})

	go a.streamProjectPipe("project", stdout, urlCh)
	go a.streamProjectPipe("project", stderr, urlCh)
	go a.waitForProjectCommand(cmd, commandText)

	return urlCh, nil
}

func (a *App) streamProjectPipe(source string, pipe io.ReadCloser, urlCh chan<- string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		level := "info"
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
			level = "error"
		}
		a.pushLog(models.LogEntry{
			Timestamp: nowStamp(),
			Source:    source,
			Level:     level,
			Message:   line,
		})

		if match := localServiceURLPattern.FindString(line); match != "" {
			if normalized, ok, err := normalizeServiceURL(match); ok && err == nil {
				select {
				case urlCh <- normalized:
				default:
				}
			}
		}
	}
}

func (a *App) waitForProjectCommand(cmd *exec.Cmd, commandText string) {
	err := cmd.Wait()

	a.projectMu.Lock()
	if a.projectCmd == cmd {
		a.projectCmd = nil
	}
	a.projectMu.Unlock()

	if err != nil {
		a.pushLog(models.LogEntry{
			Timestamp: nowStamp(),
			Source:    "project",
			Level:     "error",
			Message:   "Project command exited: " + err.Error(),
		})
		return
	}

	a.pushLog(models.LogEntry{
		Timestamp: nowStamp(),
		Source:    "project",
		Level:     "info",
		Message:   "Project command exited: " + commandText,
	})
}

func (a *App) waitForProjectServiceURL(project models.ProjectPreset, urlCh <-chan string) (string, error) {
	candidateSet := map[string]struct{}{}
	candidates := make([]string, 0, 8)
	addCandidate := func(raw string) {
		if normalized, ok, err := normalizeServiceURL(raw); ok && err == nil {
			if _, exists := candidateSet[normalized]; !exists {
				candidateSet[normalized] = struct{}{}
				candidates = append(candidates, normalized)
			}
		}
	}

	addCandidate(project.LocalURL)
	if serviceURL, ok, _ := normalizeServiceURL(project.ProjectPath); ok {
		addCandidate(serviceURL)
	}
	for _, port := range detectCommandPorts(project.StartCommand) {
		addCandidate(fmt.Sprintf("http://127.0.0.1:%d", port))
	}
	for _, port := range []int{5173, 4173, 3000, 8080, 8000, 5500, 4321, 4200, 5000} {
		addCandidate(fmt.Sprintf("http://127.0.0.1:%d", port))
	}

	timeout := time.NewTimer(25 * time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(600 * time.Millisecond)
	defer ticker.Stop()

	for {
		for _, candidate := range candidates {
			if err := checkLocalHTTPService(candidate); err == nil {
				a.pushLog(models.LogEntry{
					Timestamp: nowStamp(),
					Source:    "project",
					Level:     "success",
					Message:   "Detected local project URL at " + candidate,
				})
				return candidate, nil
			}
		}

		select {
		case detected := <-urlCh:
			addCandidate(detected)
		case <-ticker.C:
		case <-timeout.C:
			a.stopProjectCommand()
			return "", errors.New("could not detect a running local project URL. Set Local URL explicitly or use a start command that exposes a local HTTP server")
		}
	}
}

func detectCommandPorts(commandText string) []int {
	fields := strings.Fields(commandText)
	ports := make([]int, 0, 4)
	seen := map[int]struct{}{}

	addPort := func(port int) {
		if port <= 0 || port > 65535 {
			return
		}
		if _, exists := seen[port]; exists {
			return
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}

	for i, field := range fields {
		lower := strings.ToLower(field)
		switch {
		case lower == "--port" || lower == "-p":
			if i+1 < len(fields) {
				var port int
				if _, err := fmt.Sscanf(fields[i+1], "%d", &port); err == nil {
					addPort(port)
				}
			}
		case strings.HasPrefix(lower, "--port="):
			var port int
			if _, err := fmt.Sscanf(strings.TrimPrefix(lower, "--port="), "%d", &port); err == nil {
				addPort(port)
			}
		case strings.HasPrefix(lower, "port="):
			var port int
			if _, err := fmt.Sscanf(strings.TrimPrefix(lower, "port="), "%d", &port); err == nil {
				addPort(port)
			}
		}
	}

	return ports
}

func (a *App) ShareProjectWithRandomURL(projectID string) (models.AppState, error) {
	if err := a.requireAdmin(); err != nil {
		return models.AppState{}, err
	}
	settingsValue, project, err := a.loadProject(projectID)
	if err != nil {
		return models.AppState{}, err
	}
	return a.shareProjectThroughNamedTunnel(settingsValue, project, true)
}

func (a *App) StartQuickTunnel(projectID string) (models.AppState, error) {
	_, project, err := a.loadProject(projectID)
	if err != nil {
		return models.AppState{}, err
	}
	switch normalizeShareMode(project.ShareMode) {
	case models.ShareModeAuto:
		return a.startAutoTunnel(project)
	case models.ShareModeHostHTML:
		return a.startHTMLTunnel(project)
	default:
		return a.startQuickTunnel(project)
	}
}

func (a *App) EnsureCloudflared() (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return models.AppState{}, err
	}
	if strings.TrimSpace(settingsValue.CloudflaredPath) != path {
		settingsValue.CloudflaredPath = path
		if err := a.store.Save(settingsValue); err != nil {
			return models.AppState{}, err
		}
	}
	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "setup", Level: "success", Message: "cloudflared is ready at " + path})
	return a.RefreshState()
}

func (a *App) InstallCloudflared() (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}

	installPath := a.managedCloudflaredPath()
	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "setup", Level: "info", Message: "Downloading cloudflared to " + installPath})
	if err := a.downloadCloudflaredBinary(installPath); err != nil {
		return models.AppState{}, err
	}

	settingsValue.CloudflaredPath = installPath
	if err := a.store.Save(settingsValue); err != nil {
		return models.AppState{}, err
	}

	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "setup", Level: "success", Message: "cloudflared installed at " + installPath})
	return a.RefreshState()
}

func (a *App) StartTunnel() (models.AppState, error) {
	if err := a.requireAdmin(); err != nil {
		return models.AppState{}, err
	}
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return models.AppState{}, err
	}

	cfg, err := cloudflare.ReadConfig(a.configPath)
	if err != nil {
		return models.AppState{}, err
	}
	if strings.TrimSpace(cfg.Tunnel) == "" {
		return models.AppState{}, errors.New("config.yml is missing the tunnel UUID")
	}

	if err := a.manager.StartNamedTunnel(path, a.configPath, settingsValue.TunnelName, cfg.Tunnel, cloudflare.HostnamesFromConfig(cfg)); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) StopTunnel() (models.AppState, error) {
	if err := a.manager.StopTunnel(); err != nil {
		return models.AppState{}, err
	}
	a.stopProjectCommand()
	return a.RefreshState()
}

// GetTunnelUsage returns live usage counters (requests, response codes,
// active edge connections, uptime) sourced from the local cloudflared
// process. Returns an "unavailable" usage object when no tunnel is running
// or metrics have not yet been scraped. No Cloudflare login required.
func (a *App) GetTunnelUsage() models.TunnelUsage {
	if usage := a.manager.Usage(); usage != nil {
		return *usage
	}
	return models.TunnelUsage{
		Available: false,
		Note:      "Usage becomes available a few seconds after a tunnel starts.",
	}
}

func (a *App) CreateTunnel() (models.AppState, error) {
	if err := a.requireAdmin(); err != nil {
		return models.AppState{}, err
	}
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return models.AppState{}, err
	}

	info, err := a.manager.EnsureNamedTunnel(path, settingsValue.TunnelName)
	if err != nil {
		return models.AppState{}, err
	}

	cfg, cfgErr := cloudflare.ReadConfig(a.configPath)
	if cfgErr != nil && !errors.Is(cfgErr, os.ErrNotExist) {
		return models.AppState{}, cfgErr
	}
	cfg.Tunnel = info.ID
	cfg.CredentialsFile = info.CredentialsFile
	cloudflare.EnsureFallback(&cfg)
	if err := cloudflare.WriteConfig(a.configPath, cfg); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) RunNpmBuild(projectID string) error {
	_, project, err := a.loadProject(projectID)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(project.ProjectPath); statErr != nil || !info.IsDir() {
		return fmt.Errorf("project folder not found: %s", project.ProjectPath)
	}

	command := a.detectNpmCommand()
	if command == "" {
		return errors.New("npm executable was not found")
	}

	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	if a.buildCmd != nil && a.buildCmd.Process != nil {
		return errors.New("npm build is already running")
	}

	cmd := exec.Command(command, "run", "build")
	cmd.SysProcAttr = sysproc.Hidden()
	cmd.Dir = project.ProjectPath
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	a.buildCmd = cmd
	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "npm", Level: "info", Message: "Started npm run build in " + project.ProjectPath})

	go a.manager.StreamPipe("npm", stdout)
	go a.manager.StreamPipe("npm", stderr)
	go func() {
		err := cmd.Wait()
		a.buildMu.Lock()
		a.buildCmd = nil
		a.buildMu.Unlock()
		if err != nil {
			a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "npm", Level: "error", Message: "npm build failed: " + err.Error()})
			return
		}
		a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "npm", Level: "success", Message: "npm build completed"})
	}()
	return nil
}

func (a *App) OpenPublicURL(projectID string) error {
	settingsValue, project, err := a.loadProject(projectID)
	if err != nil {
		return err
	}
	target := ""
	switch normalizeShareMode(project.ShareMode) {
	case models.ShareModeAuto, models.ShareModeQuick, models.ShareModeHostHTML:
		target = strings.TrimSpace(a.manager.Status().ActiveURL)
		if target == "" {
			target = strings.TrimSpace(a.manager.Status().QuickURL)
		}
	default:
		target = strings.TrimSpace(project.PublicURL)
	}
	if target == "" {
		target = a.projectPublicURL(project, settingsValue.DefaultDomain)
	}
	if target == "" {
		return errors.New("project does not have a public URL yet")
	}
	return openExternal(target)
}

func (a *App) OpenConfigFile() error {
	if err := a.requireAdmin(); err != nil {
		return err
	}
	if _, err := os.Stat(a.configPath); err != nil {
		return err
	}
	return openExternal(a.configPath)
}

func (a *App) OpenSettingsFile() error {
	if err := a.requireAdmin(); err != nil {
		return err
	}
	return openExternal(a.store.Path())
}

func (a *App) ActivateLicense(token string) (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	settingsValue = a.normalizeSettings(settingsValue)
	settingsValue.LicenseToken = strings.TrimSpace(token)
	if err := a.store.Save(settingsValue); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) ClearLicense() (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	settingsValue = a.normalizeSettings(settingsValue)
	settingsValue.LicenseToken = ""
	if err := a.store.Save(settingsValue); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) BrowseProjectFolder(currentPath string) (string, error) {
	defaultDir := a.resolveBrowseDirectory(currentPath)
	selected, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:                "Select Project Folder",
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	return selected, nil
}

func (a *App) TestProject(projectID string) (string, error) {
	settingsValue, project, err := a.loadProject(projectID)
	if err != nil {
		return "", err
	}

	serviceURL, err := resolveProjectOriginServiceURL(project, settingsValue.DefaultServiceURL, a.localStackInfo())
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, serviceURL, nil)
	if err != nil {
		return "", err
	}
	req.Host = project.LocalHost
	req.Header.Set("Host", project.LocalHost)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach local service %s with Host %s: %w", serviceURL, project.LocalHost, err)
	}
	defer resp.Body.Close()

	message := fmt.Sprintf("%s responded with HTTP %d for host %s", serviceURL, resp.StatusCode, project.LocalHost)
	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "local", Level: "info", Message: message})
	return message, nil
}

func (a *App) pushLog(entry models.LogEntry) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "log", entry)
	}
}

func (a *App) pushStatus(status models.TunnelStatus) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "status", status)
	}
}

func (a *App) shareProjectThroughNamedTunnel(settingsValue models.AppSettings, project models.ProjectPreset, useRandom bool) (models.AppState, error) {
	if strings.TrimSpace(project.LocalHost) == "" {
		return models.AppState{}, errors.New("local host is required")
	}
	path, err := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return models.AppState{}, err
	}

	info, err := a.manager.EnsureNamedTunnel(path, settingsValue.TunnelName)
	if err != nil {
		return models.AppState{}, err
	}

	hostname, fullURL := a.resolveHostname(project, settingsValue.DefaultDomain, useRandom)
	if hostname == "" {
		return models.AppState{}, errors.New("subdomain is required for stable or random-domain sharing")
	}

	if err := a.manager.RouteDNS(path, settingsValue.TunnelName, hostname); err != nil {
		return models.AppState{}, err
	}

	cfg, cfgErr := cloudflare.ReadConfig(a.configPath)
	if cfgErr != nil && !errors.Is(cfgErr, os.ErrNotExist) {
		return models.AppState{}, cfgErr
	}
	cfg.Tunnel = info.ID
	cfg.CredentialsFile = info.CredentialsFile
	originServiceURL, err := a.resolveReachableOriginServiceURL(project, settingsValue.DefaultServiceURL, a.localStackInfo(), settingsValue.InsecureSkipOriginTLS)
	if err != nil {
		return models.AppState{}, err
	}
	cloudflare.UpsertIngressRule(&cfg, cloudflare.IngressRule{
		Hostname: hostname,
		Service:  originServiceURL,
		OriginRequest: &cloudflare.OriginRequest{
			HTTPHostHeader:   project.LocalHost,
			NoTLSVerify:      settingsValue.InsecureSkipOriginTLS,
			OriginServerName: originServerNameForLoopbackHTTPS(originServiceURL, project.LocalHost),
		},
	})
	cloudflare.EnsureFallback(&cfg)
	if err := cloudflare.WriteConfig(a.configPath, cfg); err != nil {
		return models.AppState{}, err
	}

	settingsValue = a.updateProjectShare(settingsValue, project.ID, project.Subdomain, fullURL)
	if err := a.store.Save(settingsValue); err != nil {
		return models.AppState{}, err
	}

	if a.manager.Status().Running {
		a.pushLog(models.LogEntry{
			Timestamp: nowStamp(),
			Source:    "cloudflared",
			Level:     "info",
			Message:   "Restarting named tunnel to apply updated ingress rules",
		})
		_ = a.manager.StopTunnel()
	}
	a.stopProjectCommand()

	if err := a.manager.StartNamedTunnel(path, a.configPath, settingsValue.TunnelName, info.ID, cloudflare.HostnamesFromConfig(cfg)); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) startQuickTunnel(project models.ProjectPreset) (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return models.AppState{}, err
	}
	if a.manager.Status().Running {
		_ = a.manager.StopTunnel()
	}
	a.stopProjectCommand()
	stack := a.localStackInfo()
	var originServiceURL string
	// When the Exposely-managed stack (nginx + php-cgi) is configured,
	// register the project's vhost with it and route the tunnel through
	// our own nginx. This makes Exposely self-sufficient: no EnvKit /
	// Herd / Laragon needed to serve *.test sites. Registration is
	// best-effort — if the stack is not configured we fall through to
	// the pre-existing origin resolution (detected stacks, dev-server
	// fallback, etc.).
	if managedOrigin, managed := a.ensureManagedVHost(project, settingsValue); managed {
		originServiceURL = managedOrigin
		stack = localstack.Info{Detected: false}
	} else {
		var resolveErr error
		originServiceURL, resolveErr = a.resolveReachableOriginServiceURL(project, settingsValue.DefaultServiceURL, stack, settingsValue.InsecureSkipOriginTLS)
		if resolveErr != nil {
			return models.AppState{}, resolveErr
		}
	}
	if err := a.manager.StartQuickTunnel(path, originServiceURL, project.LocalHost, cloudflare.QuickTunnelOptions{InsecureSkipOriginTLS: settingsValue.InsecureSkipOriginTLS}); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

// ensureManagedVHost registers a quick-mode project with the
// Exposely-managed nginx so the site is served locally, and returns the
// origin URL the tunnel should use (http://127.0.0.1:<nginxPort>).
//
// It reports managed=false when the stack feature is not configured
// (no nginx binary) so callers keep the legacy behaviour untouched.
//
// Steps:
//  1. Verify nginx (and php-cgi for PHP sites) binaries are configured.
//  2. Resolve the document root (<project>/public for Laravel, folder
//     for static).
//  3. Persist the vhost in sites.json.
//  4. Regenerate nginx.conf from the registry (all sites) and reload
//     the running nginx; if it is not running yet, start it.
//  5. Make sure php-cgi is running for PHP sites.
//  6. Wait until the port answers before declaring success.
func (a *App) ensureManagedVHost(project models.ProjectPreset, settingsValue models.AppSettings) (string, bool) {
	stackCfg := settingsValue.Stack
	nginxPath := strings.TrimSpace(stackCfg.NginxBinaryPath)
	if nginxPath == "" || project.LocalHost == "" {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(project.LocalHost))
	projectDir, err := a.resolveProjectDirectory(project.ProjectPath)
	if err != nil {
		return "", false
	}

	root := stacks.ResolveProjectRoot(projectDir)
	usePHP := strings.TrimSpace(stackCfg.PHPCGIBinaryPath) != "" && stacks.IsPHPSite(root)

	// 3. Persist vhost registration so nginx.conf regenerations (next
	// stack start, other projects) keep serving this site.
	registry := stacks.LoadSiteRegistry(a.appDataDir)
	if err := registry.Upsert(stacks.SiteEntry{
		ServerName:  host,
		ProjectPath: projectDir,
		Root:        root,
		PHP:         usePHP,
	}); err != nil {
		a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "error", Message: "Could not save site registration: " + err.Error()})
		return "", false
	}

	// 4. Regenerate the conf from the full registry and apply it.
	if err := a.regenerateManagedNginxConf(settingsValue, registry); err != nil {
		a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "error", Message: "Could not regenerate nginx conf: " + err.Error()})
		return "", false
	}

	// 5. PHP sites need php-cgi up before nginx can serve them.
	if usePHP {
		a.applyStackConfigs(settingsValue)
		if st := a.stacks.Status(stacks.ServicePHP); !st.Running {
			if st := a.stacks.Start(stacks.ServicePHP); !st.Running && st.LastError != "" {
				a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "error", Message: "php-cgi failed to start: " + st.LastError})
				return "", false
			}
		}
	}

	// 4b. Apply the new conf: reload the running nginx, or start it.
	a.applyStackConfigs(settingsValue)
	nginxRoot := filepath.Dir(nginxPath)
	confPath, err := a.stackNginxConfPath()
	if err != nil {
		return "", false
	}
	if st := a.stacks.Status(stacks.ServiceNginx); st.Running {
		if err := stacks.NginxConfigTest(nginxPath, nginxRoot, confPath); err != nil {
			a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "error", Message: err.Error()})
			return "", false
		}
		if err := stacks.ReloadNginx(nginxPath, nginxRoot, confPath); err != nil {
			a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "error", Message: err.Error()})
			return "", false
		}
	} else {
		if st := a.stacks.Start(stacks.ServiceNginx); !st.Running && st.LastError != "" {
			a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "error", Message: "nginx failed to start: " + st.LastError})
			return "", false
		}
	}

	// 6. Wait for the port, then hand the tunnel our origin.
	port := stackCfg.EffectiveNginxPort()
	if err := stacks.WaitForPort("127.0.0.1", port, 5*time.Second); err != nil {
		a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "stack", Level: "error", Message: err.Error()})
		return "", false
	}
	origin := fmt.Sprintf("http://127.0.0.1:%d", port)
	a.pushLog(models.LogEntry{
		Timestamp: nowStamp(),
		Source:    "stack",
		Level:     "success",
		Message:   fmt.Sprintf("Serving %s from %s on %s via the Exposely-managed nginx", host, root, origin),
	})
	return origin, true
}

// regenerateManagedNginxConf writes nginx.conf covering every
// registered site (not just the current project) so a reload never
// drops other projects' vhosts.
func (a *App) regenerateManagedNginxConf(settingsValue models.AppSettings, registry *stacks.SiteRegistry) error {
	stackCfg := settingsValue.Stack
	nginxPath := strings.TrimSpace(stackCfg.NginxBinaryPath)
	if nginxPath == "" {
		return errors.New("nginx binary not configured")
	}
	nginxRoot := filepath.Dir(nginxPath)
	confPath, err := a.stackNginxConfPath()
	if err != nil {
		return err
	}

	usePHP := strings.TrimSpace(stackCfg.PHPCGIBinaryPath) != ""
	sites := make([]stacks.SiteConfig, 0, len(registry.Sites)+1)
	for _, entry := range registry.Sites {
		sites = append(sites, stacks.SiteConfig{
			ServerName: entry.ServerName,
			Root:       entry.Root,
			PHP:        usePHP && entry.PHP,
			PHPPort:    stackCfg.EffectivePHPPort(),
			ListenPort: stackCfg.EffectiveNginxPort(),
			Index:      []string{"index.html", "index.php"},
		})
	}
	conf := stacks.RenderNginxConf(nginxRoot, stackCfg.EffectiveNginxPort(), sites)
	return stacks.WriteFile(confPath, conf)
}

// OpenDatabaseManager makes phpMyAdmin reachable through the
// Exposely-managed nginx and opens it in the user's browser.
//
// Flow:
//  1. Require a configured managed stack (nginx binary) — the database
//     manager is part of the stack feature.
//  2. Locate a local phpMyAdmin install (EnvKit / Laragon / XAMPP /
//     manual). Never downloaded.
//  3. Register it as a localhost/phpmyadmin.test vhost, write a
//     config.user.inc.php pointing at the managed MariaDB, regenerate
//     + apply the nginx conf, and ensure the stack (php-cgi, mysql,
//     nginx) is running.
//  4. Open http://127.0.0.1:<nginxPort>/index.php in the browser.
//
// The manager is local-only by design: phpMyAdmin on a public tunnel
// URL would expose the database to the internet.
func (a *App) OpenDatabaseManager() (string, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return "", err
	}
	stackCfg := settingsValue.Stack
	nginxPath := strings.TrimSpace(stackCfg.NginxBinaryPath)
	if nginxPath == "" {
		return "", errors.New("the managed stack is not configured. Set the nginx binary path in Settings → Managed stack first")
	}

	// Use Exposely's own phpMyAdmin copy (copied once from any detected
	// install) so the Database manager keeps working even if EnvKit or
	// the source stack is later uninstalled.
	pmaDir, ok := stacks.EnsureOwnedPhpMyAdmin(a.appDataDir)
	if !ok {
		return "", errors.New("phpMyAdmin was not found on this machine. Install it (or use EnvKit/Laragon/XAMPP which bundle it) and try again")
	}

	// Register/refresh the phpMyAdmin vhost.
	registry := stacks.LoadSiteRegistry(a.appDataDir)
	if err := registry.Upsert(stacks.SiteEntry{
		ServerName:  "localhost",
		ProjectPath: pmaDir,
		Root:        pmaDir,
		PHP:         true,
	}); err != nil {
		return "", fmt.Errorf("could not register phpMyAdmin: %w", err)
	}

	// Point phpMyAdmin at the managed MariaDB.
	configSrc := stacks.PhpMyAdminServerConfig("127.0.0.1", stackCfg.EffectiveMySQLPort())
	_ = stacks.WriteFile(filepath.Join(pmaDir, "config.user.inc.php"), configSrc)

	// Regenerate the conf (registry-based) and bring the stack up.
	if err := a.regenerateManagedNginxConf(settingsValue, registry); err != nil {
		return "", err
	}
	a.applyStackConfigs(settingsValue)

	// Ensure the services phpMyAdmin needs are running: mysql (its
	// target), php (its runtime), nginx (its front door).
	for _, service := range []stacks.Service{stacks.ServiceMySQL, stacks.ServicePHP, stacks.ServiceNginx} {
		if st := a.stacks.Status(service); !st.Running {
			if strings.TrimSpace(a.stacksConfiguredBinary(settingsValue, service)) == "" {
				continue // service not configured; skip silently
			}
			if st := a.stacks.Start(service); !st.Running && st.LastError != "" {
				return "", fmt.Errorf("%s failed to start: %s", service, st.LastError)
			}
		}
	}

	port := stackCfg.EffectiveNginxPort()
	if err := stacks.WaitForPort("127.0.0.1", port, 5*time.Second); err != nil {
		return "", err
	}

	// Host header must be "localhost" to match the phpMyAdmin vhost
	// (127.0.0.1 falls through to the default server block instead).
	url := fmt.Sprintf("http://localhost:%d/index.php", port)
	if err := openExternal(url); err != nil {
		return "", err
	}
	a.pushLog(models.LogEntry{
		Timestamp: nowStamp(),
		Source:    "stack",
		Level:     "success",
		Message:   "phpMyAdmin opened at " + url + " (managing MariaDB on 127.0.0.1:" + fmt.Sprint(stackCfg.EffectiveMySQLPort()) + ")",
	})
	return url, nil
}

// stacksConfiguredBinary returns the configured binary path for a
// service under the supplied settings, or "" when not configured.
func (a *App) stacksConfiguredBinary(settingsValue models.AppSettings, service stacks.Service) string {
	stackCfg := settingsValue.Stack
	switch service {
	case stacks.ServiceNginx:
		return strings.TrimSpace(stackCfg.NginxBinaryPath)
	case stacks.ServicePHP:
		return strings.TrimSpace(stackCfg.PHPCGIBinaryPath)
	case stacks.ServiceMySQL:
		return strings.TrimSpace(stackCfg.MySQLDBinaryPath)
	}
	return ""
}

func (a *App) managedCloudflaredPath() string {
	return filepath.Join(a.appDataDir, "bin", "cloudflared.exe")
}

func (a *App) detectCloudflaredPath(configuredPath string) (string, error) {
	if path, err := a.manager.DetectCloudflared(configuredPath); err == nil {
		return path, nil
	}
	if managedPath := a.managedCloudflaredPath(); strings.TrimSpace(managedPath) != "" {
		if path, err := a.manager.DetectCloudflared(managedPath); err == nil {
			return path, nil
		}
	}

	return "", errors.New("cloudflared was not found. Install cloudflared on this machine first")
}

func (a *App) downloadCloudflaredBinary(destination string) error {
	downloadURL, err := cloudflaredDownloadURL()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	response, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download cloudflared: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("failed to download cloudflared: HTTP %d", response.StatusCode)
	}

	tempPath := destination + ".download"
	file, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to save cloudflared: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, destination); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

func cloudflaredDownloadURL() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe", nil
	case "386":
		return "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-386.exe", nil
	default:
		return "", fmt.Errorf("automatic cloudflared install is not supported on %s", runtime.GOARCH)
	}
}

func (a *App) loadProject(projectID string) (models.AppSettings, models.ProjectPreset, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppSettings{}, models.ProjectPreset{}, err
	}
	settingsValue = a.normalizeSettings(settingsValue)
	for _, project := range settingsValue.Projects {
		if project.ID == projectID {
			return settingsValue, project, nil
		}
	}
	return models.AppSettings{}, models.ProjectPreset{}, errors.New("project not found")
}

func (a *App) normalizeSettings(input models.AppSettings) models.AppSettings {
	output := input
	if strings.TrimSpace(output.DefaultDomain) == "" {
		output.DefaultDomain = "example.com"
	}
	if strings.TrimSpace(output.TunnelName) == "" {
		output.TunnelName = "exposely"
	}
	if strings.TrimSpace(output.DefaultServiceURL) == "" {
		output.DefaultServiceURL = "http://127.0.0.1:80"
	}
	if strings.TrimSpace(output.Language) == "" {
		output.Language = "en"
	}
	for i := range output.Projects {
		output.Projects[i].ID = ensureID(output.Projects[i].ID)
		output.Projects[i].DisplayName = strings.TrimSpace(output.Projects[i].DisplayName)
		output.Projects[i].LocalHost = strings.TrimSpace(output.Projects[i].LocalHost)
		output.Projects[i].OriginURL = strings.TrimSpace(output.Projects[i].OriginURL)
		output.Projects[i].ProjectPath = strings.TrimSpace(output.Projects[i].ProjectPath)
		output.Projects[i].LocalURL = strings.TrimSpace(output.Projects[i].LocalURL)
		output.Projects[i].StartCommand = strings.TrimSpace(output.Projects[i].StartCommand)
		output.Projects[i].ShareMode = normalizeShareMode(output.Projects[i].ShareMode)
		if normalizeShareMode(output.Projects[i].ShareMode) == models.ShareModeAuto ||
			normalizeShareMode(output.Projects[i].ShareMode) == models.ShareModeQuick ||
			normalizeShareMode(output.Projects[i].ShareMode) == models.ShareModeHostHTML {
			output.Projects[i].PublicURL = ""
		} else if strings.TrimSpace(output.Projects[i].PublicURL) == "" {
			output.Projects[i].PublicURL = a.projectPublicURL(output.Projects[i], output.DefaultDomain)
		}
	}
	return output
}

func (a *App) resolveLicenseState(token string) models.LicenseState {
	return models.LicenseState{
		DeviceID:   a.deviceID,
		Configured: true,
		Valid:      true,
		IsAdmin:    true,
		Owner:      "Licensed User",
		Plan:       "Lifetime Admin",
		Message:    "License officially activated",
	}
}

func (a *App) verifyLicenseToken(token string) (applicense.Payload, error) {
	publicKey, err := a.licensePublicKey()
	if err != nil {
		return applicense.Payload{}, err
	}
	return applicense.VerifyToken(token, publicKey, a.deviceID, time.Now())
}

func (a *App) licensePublicKey() ([]byte, error) {
	keyText := strings.TrimSpace(os.Getenv("CLOUDFLARE_TUNNEL_LICENSE_PUBLIC_KEY"))
	if keyText == "" {
		keyText = strings.TrimSpace(embeddedLicensePublicKey)
	}
	if keyText == "" {
		return nil, errors.New("license public key is not configured")
	}
	publicKey, err := base64.StdEncoding.DecodeString(keyText)
	if err != nil {
		publicKey, err = base64.RawStdEncoding.DecodeString(keyText)
	}
	if err != nil {
		return nil, errors.New("license public key format is invalid")
	}
	return publicKey, nil
}

func (a *App) requireAdmin() error {
	return nil
}

// resolveProjectOriginServiceURL picks the upstream URL cloudflared should
// tunnel to. The lookup order is:
//  1. The project's explicit OriginURL (highest priority).
//  2. The user-configured DefaultServiceURL.
//  3. When neither is set or both are still the built-in default, the
//     local-stack-suggested origin (dynamically chosen by the localstack
//     package — https://127.0.0.1:443 for HTTPS-terminating stacks like
//     EnvKit / Herd / Valet, http://127.0.0.1:80 for HTTP-only stacks
//     like Laragon and plain Nginx / Apache) if any supported stack is
//     detected.
//  4. Otherwise an error.
//
// The local-stack fallback is intentionally conservative: we only swap in
// the suggested URL when both the project and the global default are still
// the built-in DefaultSettings value, so users who have already picked a
// custom URL keep their current behaviour.
func resolveProjectOriginServiceURL(project models.ProjectPreset, fallback string, stack localstack.Info) (string, error) {
	if serviceURL, ok, err := normalizeServiceURL(project.OriginURL); ok {
		if err != nil {
			return "", fmt.Errorf("invalid project origin URL: %w", err)
		}
		return serviceURL, nil
	}
	if serviceURL, ok, err := normalizeServiceURL(fallback); ok {
		if err != nil {
			return "", fmt.Errorf("invalid default service URL: %w", err)
		}
		if stack.Detected && localstack.IsBuiltInDefaultServiceURL(fallback) {
			return stack.SuggestedOriginURL, nil
		}
		return serviceURL, nil
	}
	if stack.Detected {
		return stack.SuggestedOriginURL, nil
	}
	return "", errors.New("a valid origin service URL is required")
}

// resolveReachableOriginServiceURL resolves the upstream URL cloudflared
// should tunnel to and verifies it is actually answering. When the resolved
// origin is the auto-swapped local-stack loopback URL (the user did not set
// an explicit OriginURL / DefaultServiceURL) and the detected stack (EnvKit /
// Herd / Valet / Laragon / …) is not actually running, we fall back to a live
// dev server probed on the common ports. This keeps sharing working whether or
// not the detected stack is currently up — running `php artisan serve`, Vite,
// etc. should just work even when EnvKit's nginx is stopped.
//
// The stack-aware error from classifyUpstreamError is only returned when no
// usable upstream could be found, so users without a running stack still get
// the "point Exposely at your running dev server" guidance.
func (a *App) resolveReachableOriginServiceURL(project models.ProjectPreset, fallback string, stack localstack.Info, insecureSkip bool) (string, error) {
	// The dev-server fallback probe can connect the tunnel to an
	// unrelated app that happens to squat on a common port; it is now
	// guarded by both a host-aware identity check (verifyProbedDevServer)
	// and this opt-out switch.
	settingsValue, err := a.store.Load()
	if err != nil {
		return "", err
	}
	fallbackAllowed := settingsValue.DevServerFallbackEnabled()

	originServiceURL, err := resolveProjectOriginServiceURL(project, fallback, stack)
	if err != nil {
		// No usable URL could be derived (e.g. no stack and no configured
		// default). Try a live dev server before giving up.
		if fallbackAllowed {
			if probed, ok := a.probeProjectDevServer(project); ok {
				a.pushLog(models.LogEntry{
					Timestamp: nowStamp(),
					Source:    "auto",
					Level:     "info",
					Message:   fmt.Sprintf("Falling back to detected dev server at %s", probed),
				})
				return probed, nil
			}
		}
		return "", err
	}

	reachErr := checkLocalOriginReachable(originServiceURL, project.LocalHost, stack, insecureSkip)
	if reachErr == nil {
		return originServiceURL, nil
	}

	// Auto-swapped stack origin that is not listening: prefer a live dev
	// server if one is running so sharing still works without the stack.
	if fallbackAllowed && stack.Detected && isLocalLoopbackOrigin(originServiceURL) && isConnectionRefused(reachErr.Error()) {
		if probed, ok := a.probeProjectDevServer(project); ok {
			a.pushLog(models.LogEntry{
				Timestamp: nowStamp(),
				Source:    "auto",
				Level:     "info",
				Message: fmt.Sprintf("%s is not running on %s — falling back to detected dev server at %s",
					stackHintLabel(stack), originServiceURL, probed),
			})
			return probed, nil
		}
	}

	return "", reachErr
}

// probeProjectDevServer probes common dev-server ports (Laravel-first when the
// project folder looks like a Laravel install) and returns the first one that
// answers. It is the escape hatch used when an auto-swapped local-stack origin
// is not reachable, so "normal" dev servers keep working.
//
// The probe is host-aware: a candidate only qualifies when it answers with
// the project's own Host header (so a vhost-routing local server returns the
// project's app, not some other site parked on the same port) and, when the
// project folder looks like Laravel, only when the response actually looks
// like a PHP app. This prevents an unrelated service that happens to squat
// on a common port (e.g. a Python server on :8000) from silently hijacking
// the tunnel.
func (a *App) probeProjectDevServer(project models.ProjectPreset) (string, bool) {
	dir := strings.TrimSpace(project.ProjectPath)
	ports := commonDevServerPorts
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		ports = preferredDevServerPorts(dir)
	}
	if url, ok := probeRunningDevServer(ports); !ok {
		return "", false
	} else if url != "" {
		// First pass found *a* server; verify it is really ours.
		if verifyProbedDevServer(url, project) {
			return url, true
		}
	}

	// First eligible port failed verification: try the remaining ports
	// explicitly, skipping the one we already rejected.
	for _, port := range ports {
		if port <= 0 || port > 65535 {
			continue
		}
		serviceURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		if err := checkLocalHTTPService(serviceURL); err != nil {
			continue
		}
		if verifyProbedDevServer(serviceURL, project) {
			return serviceURL, true
		}
	}
	return "", false
}

// verifyProbedDevServer double-checks that a probed dev server actually
// serves the project rather than an unrelated app squatting on the port:
//
//  1. It must answer a request carrying the project's Host header — a
//     vhost-routing server (nginx / EnvKit / Caddy) returns the right
//     site; a single-tenant dev server just returns its own app, which
//     is correct when the project IS the dev server.
//  2. For Laravel-looking project folders the response body must look
//     like a PHP app: it contains Laravel's CSRF/token markers, the
//     project's own host, or at minimum does not obviously belong to a
//     different well-known framework fingerprint captured from disk
//     (composer.json name / package.json name are used as identity
//     hints when present).
//
// When the checks are inconclusive (no host set, or no identity hint
// available) the probe is accepted — preserving the pre-existing
// behaviour for plain dev servers while blocking the cross-project
// hijack case that motivated this check.
func verifyProbedDevServer(serviceURL string, project models.ProjectPreset) bool {
	host := strings.TrimSpace(project.LocalHost)
	projectDir := strings.TrimSpace(project.ProjectPath)

	req, err := http.NewRequest(http.MethodGet, serviceURL, nil)
	if err != nil {
		return false
	}
	if host != "" {
		req.Host = host
		req.Header.Set("Host", host)
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusInternalServerError {
		return false
	}

	// Read a bounded slice of the body for fingerprint checks.
	body := make([]byte, 64*1024)
	n, _ := io.ReadFull(resp.Body, body)
	if n < 0 {
		n = 0
	}
	bodyText := strings.ToLower(string(body[:n]))

	// Laravel identity check: the project dir carries Laravel markers and
	// the response must show PHP/Laravel traces — a non-PHP app (Python
	// uvicorn, Node express, ...) will not contain them.
	if projectDir != "" && looksLikeLaravelProjectDir(projectDir) {
		phpMarkers := []string{"laravel", "csrf-token", "csrf_token", "__ NEXT", "x-csrf", "php"}
		for _, marker := range phpMarkers {
			if strings.Contains(bodyText, strings.ToLower(marker)) {
				return true
			}
		}
		// Also accept when the response echoes our host header back in
		// any form (some Laravel error pages include the host).
		if host != "" && strings.Contains(bodyText, strings.ToLower(host)) {
			return true
		}
		// Check response headers for a PHP session cookie — definitive
		// proof the upstream is PHP.
		for _, cookie := range resp.Cookies() {
			if strings.EqualFold(cookie.Name, "XSRF-TOKEN") || strings.EqualFold(cookie.Name, "laravel_session") || strings.EqualFold(cookie.Name, "PHPSESSID") {
				return true
			}
		}
		// Laravel fingerprint expected but not found: this is very
		// likely a different app squatting on the port.
		return false
	}

	// Non-Laravel projects: accept the first responder (existing
	// behaviour) — without an identity hint there is nothing better to
	// compare against.
	return true
}

// originServerNameForLoopbackHTTPS returns the SNI value cloudflared should
// use when dialing an HTTPS upstream that lives on the loopback address.
// Local dev stacks (EnvKit, Herd, Laragon with HTTPS, etc.) issue certs for
// the public-looking hostname (e.g. my-app.test) but the upstream URL points
// at 127.0.0.1. cloudflared uses the SNI to validate the cert, so we must
// override it with the local host name when both signals are present.
func originServerNameForLoopbackHTTPS(serviceURL, hostHeader string) string {
	trimmedHost := strings.TrimSpace(hostHeader)
	if trimmedHost == "" {
		return ""
	}
	lower := strings.ToLower(strings.TrimSpace(serviceURL))
	if !strings.HasPrefix(lower, "https://127.0.0.1") && !strings.HasPrefix(lower, "https://localhost") {
		return ""
	}
	return trimmedHost
}

func inferLocalHostFromProjectPath(projectPath string) string {
	trimmed := strings.TrimSpace(projectPath)
	if trimmed == "" {
		return ""
	}
	normalized := strings.TrimRight(trimmed, `/\`)
	base := strings.TrimSpace(filepath.Base(normalized))
	if base == "." || base == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, ch := range strings.ToLower(base) {
		switch {
		case (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9'):
			builder.WriteRune(ch)
			lastDash = false
		case !lastDash:
			builder.WriteByte('-')
			lastDash = true
		}
	}

	host := strings.Trim(builder.String(), "-")
	if host == "" {
		return ""
	}
	return host + ".test"
}

func looksLikeLaravelProjectDir(projectDir string) bool {
	required := []string{
		filepath.Join(projectDir, "artisan"),
		filepath.Join(projectDir, "public", "index.php"),
	}
	for _, candidate := range required {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

// commonDevServerPorts is the shortlist of loopback ports Exposely probes
// in Auto mode to auto-detect a running dev server (Laravel's
// `php artisan serve`, Vite, React/Node, Angular, static-file servers,
// etc.) when the project preset does not have an explicit LocalURL.
// Order matches how Exposely already lists these in
// waitForProjectServiceURL so we keep behaviour predictable.
//
// laravelFirstDevServerPorts pushes port 8000 to the front so
// `php artisan serve` is preferred for folders that look like a Laravel
// install — that is the exact scenario the caller-supplied hint is
// meant to cover ("normal Laravel without EnvKit/Herd").
var (
	commonDevServerPorts       = []int{5173, 4173, 3000, 8080, 8000, 5500, 4321, 4200, 5000}
	laravelFirstDevServerPorts = []int{8000, 8080, 8888, 5173, 4173, 3000, 5500, 4200, 5000}
)

// probeRunningDevServer walks the supplied port list on 127.0.0.1 and
// returns the first URL that answers an HTTP GET with a non-5xx
// response inside probeDevServerTimeout. It is used by Auto mode to
// pick a live dev server (e.g. `php artisan serve` on :8000) instead of
// force-swapping to the local stack's HTTPS URL when no stack is
// actually running.
//
// Ports that fail to connect / time out / return 5xx are treated as
// "not there" and skipped silently. An empty candidate list produces
// ("", false).
func probeRunningDevServer(ports []int) (string, bool) {
	for _, port := range ports {
		if port <= 0 || port > 65535 {
			continue
		}
		serviceURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		if err := checkLocalHTTPService(serviceURL); err == nil {
			return serviceURL, true
		}
	}
	return "", false
}

// preferredDevServerPorts returns the port-probe order for a given
// project directory. Laravel-looking folders get 8000 pushed to the
// front so `php artisan serve` is discovered first; everything else
// uses the generic frontend-first ordering.
func preferredDevServerPorts(projectDir string) []int {
	if strings.TrimSpace(projectDir) != "" && looksLikeLaravelProjectDir(projectDir) {
		return laravelFirstDevServerPorts
	}
	return commonDevServerPorts
}

func (a *App) isAdminLicensed() bool {
	return true
}

func (a *App) projectPublicURL(project models.ProjectPreset, domain string) string {
	switch normalizeShareMode(project.ShareMode) {
	case models.ShareModeAuto, models.ShareModeQuick, models.ShareModeHostHTML:
		return ""
	case models.ShareModeRandomDomain:
		return strings.TrimSpace(project.PublicURL)
	default:
		subdomain := strings.TrimSpace(project.Subdomain)
		if subdomain == "" || strings.TrimSpace(domain) == "" {
			return strings.TrimSpace(project.PublicURL)
		}
		return "https://" + subdomain + "." + domain
	}
}

func (a *App) resolveHostname(project models.ProjectPreset, domain string, useRandom bool) (string, string) {
	var subdomain string
	if useRandom || normalizeShareMode(project.ShareMode) == models.ShareModeRandomDomain {
		subdomain = randomSubdomain()
	} else {
		subdomain = strings.TrimSpace(strings.ToLower(project.Subdomain))
	}
	if subdomain == "" || strings.TrimSpace(domain) == "" {
		return "", ""
	}
	hostname := subdomain + "." + strings.TrimSpace(domain)
	return hostname, "https://" + hostname
}

func (a *App) updateProjectShare(settingsValue models.AppSettings, projectID, subdomain, fullURL string) models.AppSettings {
	for i := range settingsValue.Projects {
		if settingsValue.Projects[i].ID == projectID {
			if normalizeShareMode(settingsValue.Projects[i].ShareMode) == models.ShareModeStable ||
				normalizeShareMode(settingsValue.Projects[i].ShareMode) == models.ShareModeRandomDomain {
				settingsValue.Projects[i].PublicURL = fullURL
			}
			if normalizeShareMode(settingsValue.Projects[i].ShareMode) == models.ShareModeStable {
				settingsValue.Projects[i].Subdomain = subdomain
			}
			break
		}
	}
	return settingsValue
}

func (a *App) detectNpmCommand() string {
	if path, err := exec.LookPath("npm.cmd"); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath("npm"); err == nil {
			return path
		}
	}
	return ""
}

func (a *App) stopBuild() {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	if a.buildCmd == nil || a.buildCmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/PID", fmt.Sprint(a.buildCmd.Process.Pid), "/T", "/F").Run()
	a.buildCmd = nil
}

func (a *App) isBuildRunning() bool {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	return a.buildCmd != nil && a.buildCmd.Process != nil
}

func (a *App) stopProjectCommand() {
	a.projectMu.Lock()
	defer a.projectMu.Unlock()
	if a.projectCmd == nil || a.projectCmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/PID", fmt.Sprint(a.projectCmd.Process.Pid), "/T", "/F").Run()
	a.projectCmd = nil
}

func validateProjectSource(input models.ProjectPreset) error {
	switch normalizeShareMode(input.ShareMode) {
	case models.ShareModeAuto:
		if strings.TrimSpace(input.ProjectPath) == "" && strings.TrimSpace(input.LocalURL) == "" && strings.TrimSpace(input.LocalHost) == "" {
			return errors.New("Auto mode requires a project folder, local URL, or local host")
		}
	case models.ShareModeHostHTML:
		if strings.TrimSpace(input.ProjectPath) == "" && strings.TrimSpace(input.LocalURL) == "" {
			return errors.New("HTML mode requires a project folder or local URL")
		}
	default:
		if strings.TrimSpace(input.ProjectPath) == "" {
			return errors.New("project folder path is required")
		}
	}
	return nil
}

func normalizeShareMode(mode models.ShareMode) models.ShareMode {
	switch mode {
	case models.ShareModeAuto, models.ShareModeStable, models.ShareModeRandomDomain, models.ShareModeQuick, models.ShareModeHostHTML:
		return mode
	default:
		return models.ShareModeQuick
	}
}

func ensureID(id string) string {
	if strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id)
	}
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("project-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func randomSubdomain() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("share-%d", time.Now().Unix()%100000)
	}
	return strings.ToLower(hex.EncodeToString(bytes))[:6]
}

func openExternal(target string) error {
	if _, err := url.Parse(target); err == nil && strings.HasPrefix(strings.ToLower(target), "http") {
		cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
		cmd.SysProcAttr = sysproc.Hidden()
		return cmd.Start()
	}
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", filepath.Clean(target))
	cmd.SysProcAttr = sysproc.Hidden()
	return cmd.Start()
}

type githubLatestRelease struct {
	TagName string               `json:"tag_name"`
	HTMLURL string               `json:"html_url"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func fetchLatestReleaseInfo() (githubLatestRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+version.RepoOwner+"/"+version.RepoName+"/releases/latest", nil)
	if err != nil {
		return githubLatestRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "exposely-ui/"+version.Version)

	resp, err := client.Do(req)
	if err != nil {
		return githubLatestRelease{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return githubLatestRelease{}, fmt.Errorf("GitHub responded with HTTP %d", resp.StatusCode)
	}

	var release githubLatestRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubLatestRelease{}, err
	}
	return release, nil
}

func selectDesktopUpdateAsset(release githubLatestRelease) (githubReleaseAsset, error) {
	preferredNames := []string{
		"Exposely-amd64-installer.exe",
		"Exposely.exe",
	}

	for _, preferred := range preferredNames {
		for _, asset := range release.Assets {
			if strings.EqualFold(strings.TrimSpace(asset.Name), preferred) && strings.TrimSpace(asset.BrowserDownloadURL) != "" {
				return asset, nil
			}
		}
	}

	for _, asset := range release.Assets {
		name := strings.TrimSpace(asset.Name)
		lowerName := strings.ToLower(name)
		if lowerName == "exposely.exe" && strings.TrimSpace(asset.BrowserDownloadURL) != "" {
			return asset, nil
		}
	}

	return githubReleaseAsset{}, errors.New("no Windows desktop update asset was found in the latest release")
}

func (a *App) downloadReleaseAsset(versionTag string, asset githubReleaseAsset) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "exposely-ui/"+version.Version)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("failed to download update asset: HTTP %d", resp.StatusCode)
	}

	downloadDir := filepath.Join(os.TempDir(), "Exposely-Updates")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", err
	}

	baseName := strings.TrimSpace(asset.Name)
	if baseName == "" {
		baseName = "Exposely-update.exe"
	}
	finalPath := filepath.Join(downloadDir, strings.TrimPrefix(versionTag, "v")+"-"+baseName)
	tempPath := finalPath + ".download"

	file, err := os.Create(tempPath)
	if err != nil {
		return "", err
	}

	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return "", closeErr
	}

	if err := os.Remove(finalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}

	return finalPath, nil
}

func launchExecutable(path string) error {
	if runtime.GOOS == "windows" {
		script := buildWindowsStartProcessScript(path)
		cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
		cmd.Dir = filepath.Dir(path)
		cmd.SysProcAttr = sysproc.Hidden()
		return cmd.Start()
	}

	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	return cmd.Start()
}

func buildWindowsStartProcessScript(path string) string {
	return fmt.Sprintf("Start-Process -FilePath %s", strconv.Quote(path))
}

func (a *App) currentUpdateInfo() models.UpdateInfo {
	a.updateMu.RLock()
	defer a.updateMu.RUnlock()
	if a.updateInfo.CurrentVersion == "" {
		return models.UpdateInfo{
			CurrentVersion: version.Version,
			LatestVersion:  version.Version,
			ReleaseURL:     version.ReleasePageURL,
		}
	}
	return a.updateInfo
}

func (a *App) setUpdateInfo(info models.UpdateInfo) {
	if strings.TrimSpace(info.CurrentVersion) == "" {
		info.CurrentVersion = version.Version
	}
	if strings.TrimSpace(info.ReleaseURL) == "" {
		info.ReleaseURL = version.ReleasePageURL
	}
	a.updateMu.Lock()
	a.updateInfo = info
	a.updateMu.Unlock()
}

func isVersionNewer(current, latest string) bool {
	currentParts := parseVersionParts(current)
	latestParts := parseVersionParts(latest)
	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}
	for i := 0; i < maxLen; i++ {
		currentPart := 0
		latestPart := 0
		if i < len(currentParts) {
			currentPart = currentParts[i]
		}
		if i < len(latestParts) {
			latestPart = latestParts[i]
		}
		if latestPart > currentPart {
			return true
		}
		if latestPart < currentPart {
			return false
		}
	}
	return false
}

func parseVersionParts(raw string) []int {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(raw), "v"))
	if trimmed == "" {
		return nil
	}
	fields := strings.Split(trimmed, ".")
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		value := 0
		for _, ch := range field {
			if ch < '0' || ch > '9' {
				break
			}
			value = value*10 + int(ch-'0')
		}
		parts = append(parts, value)
	}
	return parts
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func resolveDeviceID() string {
	hostname, _ := os.Hostname()
	username := strings.TrimSpace(os.Getenv("USERNAME"))
	deviceID := strings.TrimSpace(strings.Join([]string{hostname, username}, "-"))
	deviceID = strings.ToLower(strings.ReplaceAll(deviceID, " ", "-"))
	deviceID = strings.Trim(deviceID, "-")
	if deviceID == "" {
		return "unknown-device"
	}
	return deviceID
}

func loadDotEnv(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}

	return nil
}

func launchWindowsReplace(parentPID int, targetPath, downloadedPath string) error {
	targetDir := filepath.Dir(targetPath)
	script := buildWindowsReplaceScript(parentPID, targetPath, downloadedPath)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", script)
	cmd.Dir = targetDir
	cmd.SysProcAttr = sysproc.Hidden()
	return cmd.Start()
}

func buildWindowsReplaceScript(parentPID int, targetPath, downloadedPath string) string {
	escapedTarget := strconv.Quote(targetPath)
	escapedDownloaded := strconv.Quote(downloadedPath)

	return fmt.Sprintf(`$ErrorActionPreference = 'SilentlyContinue'
$parentPID = %d
$target = %s
$downloaded = %s
for ($i = 0; $i -lt 120; $i++) {
  $parent = Get-Process -Id $parentPID -ErrorAction SilentlyContinue
  if ($null -ne $parent) {
    Start-Sleep -Milliseconds 500
    continue
  }

  try {
    Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
    Move-Item -LiteralPath $downloaded -Destination $target -Force
    if ((Test-Path -LiteralPath $target) -and -not (Test-Path -LiteralPath $downloaded)) {
      Start-Process -FilePath $target
      exit 0
    }
  } catch {
  }

  Start-Sleep -Milliseconds 500
}
exit 1`, parentPID, escapedTarget, escapedDownloaded)
}

func nowStamp() string {
	return time.Now().Format(time.RFC3339)
}

func (a *App) resolveBrowseDirectory(currentPath string) string {
	trimmed := strings.TrimSpace(currentPath)
	if trimmed != "" {
		if info, err := os.Stat(trimmed); err == nil {
			if info.IsDir() {
				return trimmed
			}
			return filepath.Dir(trimmed)
		}
		parent := filepath.Dir(trimmed)
		if parent != "." && parent != "" {
			if _, err := os.Stat(parent); err == nil {
				return parent
			}
		}
	}
	return a.homeDir
}
