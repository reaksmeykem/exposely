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
	updateMu    sync.RWMutex
	updateInfo  models.UpdateInfo
	envkitMu    sync.RWMutex
	envkit      localstack.Info
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
	app.envkit = localstack.DetectEnvKit()
	return app, nil
}

// refreshEnvKit re-runs the EnvKit detector and stores the result on the app.
// Callers use this from share flows when they want the latest detection in
// case EnvKit was installed after the app started.
func (a *App) refreshEnvKit() localstack.Info {
	info := localstack.DetectEnvKit()
	a.envkitMu.Lock()
	a.envkit = info
	a.envkitMu.Unlock()
	return info
}

// envKitInfo returns the latest cached EnvKit detection snapshot.
func (a *App) envKitInfo() localstack.Info {
	a.envkitMu.RLock()
	defer a.envkitMu.RUnlock()
	return a.envkit
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_, _ = a.RefreshState()
}

func (a *App) shutdown(context.Context) {
	_ = a.manager.StopTunnel()
	a.stopProjectCommand()
	a.stopBuild()
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

	// Refresh EnvKit detection so the UI sees installs that happened after
	// Exposely was launched (and stays accurate across long sessions).
	envkit := a.refreshEnvKit()
	status.EnvKitDetected = envkit.Detected
	status.EnvKitVersion = envkit.Version
	status.EnvKitPath = envkit.InstallPath
	status.EnvKitOriginURL = envkit.SuggestedOriginURL
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
// Laragon HTTPS-style setups), so the cert for *.test validates.
//
// When envkit.Detected is true and the upstream is unreachable the error
// message is annotated with a hint to start EnvKit's nginx, which is by
// far the most common cause.
//
// When insecureSkipOriginTLS is true the pre-flight dials without
// validating the cert — this matches the flag that will be passed to
// cloudflared via --no-tls-verify, so we don't reject a share we are
// about to allow anyway.
func checkLocalOriginReachable(serviceURL, hostHeader string, envkit localstack.Info, insecureSkipOriginTLS bool) error {
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
				// Keep the system trust store so EnvKit's local CA still
				// works — we only override SNI here, not verification.
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
		return classifyUpstreamError(serviceURL, hostHeader, envkit, err)
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
//   - "no listener" — the upstream port is closed (EnvKit's nginx is off).
//   - "cert SAN mismatch" — the upstream is answering but its cert does not
//     cover the host the project is asking for (very common with EnvKit when
//     a .test site is not registered).
//   - "other TLS error" — anything else in the TLS handshake.
//
// We inspect the underlying error string because Go's x509 package produces
// stable, parseable messages of the form:
// `x509: certificate is valid for A, B, C, not <requested>`.
func classifyUpstreamError(serviceURL, hostHeader string, envkit localstack.Info, rawErr error) error {
	msg := rawErr.Error()

	// Cert SAN mismatch: extract the valid hosts from the standard x509 wording.
	if strings.Contains(msg, "certificate is valid for") && strings.Contains(msg, ", not ") {
		validPart := extractValidHosts(msg)
		requested := strings.TrimSpace(hostHeader)
		if requested == "" {
			requested = "(unknown host)"
		}
		if envkit.Detected && isEnvKitOrigin(serviceURL) {
			return fmt.Errorf(
				"EnvKit's certificate at %s does not cover %q. Hosts covered by the cert: %s. "+
					"Open EnvKit and register %s as a site (or, in Exposely, set a custom OriginURL on the project that points at a server whose cert covers %s): %w",
				serviceURL, requested, validPart, requested, requested, rawErr,
			)
		}
		return fmt.Errorf(
			"the certificate at %s does not cover %q. Hosts covered by the cert: %s. "+
				"Register %s with the upstream server or set a custom OriginURL on the project: %w",
			serviceURL, requested, validPart, requested, rawErr,
		)
	}

	// "no listener" path — the upstream port is closed.
	if isConnectionRefused(msg) || isNoRoute(msg) {
		if envkit.Detected && isEnvKitOrigin(serviceURL) {
			return fmt.Errorf("EnvKit's nginx does not appear to be running on %s. Open EnvKit and start nginx (or another web server) before sharing: %w", serviceURL, rawErr)
		}
		return fmt.Errorf("upstream %s is not reachable: %w", serviceURL, rawErr)
	}

	// Fallback: any other transport / TLS / DNS error.
	return fmt.Errorf("upstream %s is not reachable: %w", serviceURL, rawErr)
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

// isEnvKitOrigin returns true when the supplied URL matches the default
// EnvKit upstream (https://127.0.0.1:443). Used to decide whether to print
// the EnvKit-specific hint in error messages.
func isEnvKitOrigin(serviceURL string) bool {
	return strings.EqualFold(strings.TrimSpace(serviceURL), localstack.EnvKitOriginURL)
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

	serviceURL, err := resolveProjectOriginServiceURL(project, settingsValue.DefaultServiceURL, a.envKitInfo())
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
	originServiceURL, err := resolveProjectOriginServiceURL(project, settingsValue.DefaultServiceURL, a.envKitInfo())
	if err != nil {
		return models.AppState{}, err
	}
	if err := checkLocalOriginReachable(originServiceURL, project.LocalHost, a.envKitInfo(), settingsValue.InsecureSkipOriginTLS); err != nil {
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
	envkit := a.envKitInfo()
	originServiceURL, err := resolveProjectOriginServiceURL(project, settingsValue.DefaultServiceURL, envkit)
	if err != nil {
		return models.AppState{}, err
	}
	if err := checkLocalOriginReachable(originServiceURL, project.LocalHost, envkit, settingsValue.InsecureSkipOriginTLS); err != nil {
		return models.AppState{}, err
	}
	if err := a.manager.StartQuickTunnel(path, originServiceURL, project.LocalHost, cloudflare.QuickTunnelOptions{InsecureSkipOriginTLS: settingsValue.InsecureSkipOriginTLS}); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
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
//   1. The project's explicit OriginURL (highest priority).
//   2. The user-configured DefaultServiceURL.
//   3. When neither is set or both are still the built-in default, the
//      EnvKit-suggested origin (https://127.0.0.1:443) if EnvKit is detected.
//   4. Otherwise an error.
//
// The EnvKit fallback is intentionally conservative: we only swap in the
// HTTPS URL when both the project and the global default are still the
// built-in Defaultsettings value, so users who have already picked a custom
// URL keep their current behaviour.
func resolveProjectOriginServiceURL(project models.ProjectPreset, fallback string, envkit localstack.Info) (string, error) {
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
		if envkit.Detected && localstack.IsBuiltInDefaultServiceURL(fallback) {
			return envkit.SuggestedOriginURL, nil
		}
		return serviceURL, nil
	}
	if envkit.Detected {
		return envkit.SuggestedOriginURL, nil
	}
	return "", errors.New("a valid origin service URL is required")
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
