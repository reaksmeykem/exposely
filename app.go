package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"cloudflaretunnelmanager/internal/cloudflare"
	"cloudflaretunnelmanager/internal/models"
	"cloudflaretunnelmanager/internal/settings"
)

type App struct {
	ctx         context.Context
	store       *settings.Store
	manager     *cloudflare.Manager
	configPath  string
	homeDir     string
	appDataDir  string
	buildMu     sync.Mutex
	buildCmd    *exec.Cmd
	lastStateMu sync.Mutex
}

func NewApp() (*App, error) {
	store, err := settings.NewStore("CloudflareTunnelManager")
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
	}
	app.manager = cloudflare.NewManager(app.configPath, app.pushLog, app.pushStatus)
	return app, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_, _ = a.RefreshState()
}

func (a *App) shutdown(context.Context) {
	_ = a.manager.StopTunnel()
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

	detectedPath, detectErr := a.manager.DetectCloudflared(settingsValue.CloudflaredPath)
	if detectErr == nil && detectedPath != settingsValue.CloudflaredPath {
		settingsValue.CloudflaredPath = detectedPath
		_ = a.store.Save(settingsValue)
	}

	status := a.manager.Status()
	status.TunnelName = settingsValue.TunnelName
	status.ConfigPath = a.configPath
	if detectErr == nil {
		status.DetectedCloudflaredPath = detectedPath
	}
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
	}, nil
}

func (a *App) SaveSettings(input models.AppSettings) (models.AppState, error) {
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
	if strings.TrimSpace(input.LocalHost) == "" {
		return models.AppState{}, errors.New("local herd hostname is required")
	}
	if strings.TrimSpace(input.ProjectPath) == "" {
		return models.AppState{}, errors.New("project folder path is required")
	}

	input.ID = ensureID(input.ID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.LocalHost = strings.TrimSpace(input.LocalHost)
	input.ProjectPath = strings.TrimSpace(input.ProjectPath)
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
	settingsValue, project, err := a.loadProject(projectID)
	if err != nil {
		return models.AppState{}, err
	}

	switch normalizeShareMode(project.ShareMode) {
	case models.ShareModeQuick:
		return a.startQuickTunnel(project)
	case models.ShareModeRandomDomain:
		return a.shareProjectThroughNamedTunnel(settingsValue, project, true)
	default:
		return a.shareProjectThroughNamedTunnel(settingsValue, project, false)
	}
}

func (a *App) ShareProjectWithRandomURL(projectID string) (models.AppState, error) {
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
	return a.startQuickTunnel(project)
}

func (a *App) EnsureCloudflared() (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.ensureCloudflared(settingsValue)
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

func (a *App) StartTunnel() (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.manager.DetectCloudflared(settingsValue.CloudflaredPath)
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
	return a.RefreshState()
}

func (a *App) CreateTunnel() (models.AppState, error) {
	settingsValue, err := a.store.Load()
	if err != nil {
		return models.AppState{}, err
	}
	path, err := a.manager.DetectCloudflared(settingsValue.CloudflaredPath)
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
	target := strings.TrimSpace(project.PublicURL)
	if target == "" && normalizeShareMode(project.ShareMode) == models.ShareModeQuick {
		target = strings.TrimSpace(a.manager.Status().ActiveURL)
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
	if _, err := os.Stat(a.configPath); err != nil {
		return err
	}
	return openExternal(a.configPath)
}

func (a *App) OpenSettingsFile() error {
	return openExternal(a.store.Path())
}

func (a *App) BrowseProjectFolder(currentPath string) (string, error) {
	defaultDir := a.resolveBrowseDirectory(currentPath)
	selected, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:                "Select Laravel Project Folder",
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

	req, err := http.NewRequest(http.MethodGet, settingsValue.DefaultServiceURL, nil)
	if err != nil {
		return "", err
	}
	req.Host = project.LocalHost
	req.Header.Set("Host", project.LocalHost)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach local service %s with Host %s: %w", settingsValue.DefaultServiceURL, project.LocalHost, err)
	}
	defer resp.Body.Close()

	message := fmt.Sprintf("%s responded with HTTP %d for host %s", settingsValue.DefaultServiceURL, resp.StatusCode, project.LocalHost)
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
	path, err := a.manager.DetectCloudflared(settingsValue.CloudflaredPath)
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
	cloudflare.UpsertIngressRule(&cfg, cloudflare.IngressRule{
		Hostname: hostname,
		Service:  settingsValue.DefaultServiceURL,
		OriginRequest: &cloudflare.OriginRequest{
			HTTPHostHeader: project.LocalHost,
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
	path, err := a.ensureCloudflared(settingsValue)
	if err != nil {
		return models.AppState{}, err
	}
	if a.manager.Status().Running {
		_ = a.manager.StopTunnel()
	}
	if err := a.manager.StartQuickTunnel(path, settingsValue.DefaultServiceURL, project.LocalHost); err != nil {
		return models.AppState{}, err
	}
	return a.RefreshState()
}

func (a *App) managedCloudflaredPath() string {
	return filepath.Join(a.appDataDir, "bin", "cloudflared.exe")
}

func (a *App) ensureCloudflared(settingsValue models.AppSettings) (string, error) {
	if path, err := a.manager.DetectCloudflared(settingsValue.CloudflaredPath); err == nil {
		return path, nil
	}

	managedPath := a.managedCloudflaredPath()
	if _, err := os.Stat(managedPath); err == nil {
		return managedPath, nil
	}

	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "setup", Level: "info", Message: "cloudflared not found, downloading managed copy"})
	if err := a.downloadManagedCloudflared(managedPath); err != nil {
		return "", err
	}
	return managedPath, nil
}

func (a *App) downloadManagedCloudflared(destination string) error {
	const downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"

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
	if err := os.Rename(tempPath, destination); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	a.pushLog(models.LogEntry{Timestamp: nowStamp(), Source: "setup", Level: "success", Message: "Downloaded managed cloudflared to " + destination})
	return nil
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
		output.DefaultDomain = "reaksmeykem.dev"
	}
	if strings.TrimSpace(output.TunnelName) == "" {
		output.TunnelName = "laravel-herd"
	}
	if strings.TrimSpace(output.DefaultServiceURL) == "" {
		output.DefaultServiceURL = "http://127.0.0.1:80"
	}
	for i := range output.Projects {
		output.Projects[i].ID = ensureID(output.Projects[i].ID)
		output.Projects[i].ShareMode = normalizeShareMode(output.Projects[i].ShareMode)
		if strings.TrimSpace(output.Projects[i].PublicURL) == "" {
			output.Projects[i].PublicURL = a.projectPublicURL(output.Projects[i], output.DefaultDomain)
		}
	}
	return output
}

func (a *App) projectPublicURL(project models.ProjectPreset, domain string) string {
	switch normalizeShareMode(project.ShareMode) {
	case models.ShareModeQuick:
		return strings.TrimSpace(project.PublicURL)
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
			settingsValue.Projects[i].PublicURL = fullURL
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

func normalizeShareMode(mode models.ShareMode) models.ShareMode {
	switch mode {
	case models.ShareModeRandomDomain, models.ShareModeQuick:
		return mode
	default:
		return models.ShareModeStable
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
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", filepath.Clean(target)).Start()
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
