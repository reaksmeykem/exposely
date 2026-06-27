package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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
	"strings"
	"sync"
	"time"

	"github.com/reaksmeykem/exposely/internal/cloudflare"
	"github.com/reaksmeykem/exposely/internal/localstack"
	"github.com/reaksmeykem/exposely/internal/models"
	"github.com/reaksmeykem/exposely/internal/settings"
	"github.com/reaksmeykem/exposely/internal/sysproc"
)

var cliLocalServiceURLPattern = regexp.MustCompile(`https?://(?:localhost|127\.0\.0\.1)(?::\d+)?(?:/[^\s"'<>]*)?`)
var cliAnyURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

type cliRunner struct {
	store      *settings.Store
	manager    *cloudflare.Manager
	configPath string
	homeDir    string
	appDataDir string
	workDir    string

	projectMu  sync.Mutex
	projectCmd *exec.Cmd

	lastQuickURL string
	useColor     bool

	envkitMu sync.RWMutex
	envkit   localstack.Info
}

// envKitInfo returns the cached EnvKit detection result. It is refreshed on
// runner construction; share flows that need an up-to-the-second snapshot can
// call refreshEnvKit first.
func (r *cliRunner) envKitInfo() localstack.Info {
	r.envkitMu.RLock()
	defer r.envkitMu.RUnlock()
	return r.envkit
}

func (r *cliRunner) refreshEnvKit() localstack.Info {
	info := localstack.DetectEnvKit()
	r.envkitMu.Lock()
	r.envkit = info
	r.envkitMu.Unlock()
	return info
}

func newCLIRunner() (*cliRunner, error) {
	store, err := settings.NewStore("Exposely")
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	runner := &cliRunner{
		store:      store,
		configPath: filepath.Join(homeDir, ".cloudflared", "config.yml"),
		homeDir:    homeDir,
		appDataDir: filepath.Dir(store.Path()),
		workDir:    workDir,
		useColor:   cliSupportsColor(),
	}
	runner.manager = cloudflare.NewManager(
		runner.configPath,
		runner.handleLog,
		runner.handleStatus,
	)
	runner.envkit = localstack.DetectEnvKit()
	return runner, nil
}

func (r *cliRunner) printStatus() error {
	settingsValue, err := r.store.Load()
	if err != nil {
		return err
	}
	settingsValue = r.normalizeSettings(settingsValue)

	cloudflaredPath, detectErr := r.detectCloudflaredPath(settingsValue.CloudflaredPath)
	status := "not detected"
	if detectErr == nil {
		status = cloudflaredPath
	}

	fmt.Printf("Settings: %s\n", r.store.Path())
	fmt.Printf("Config:    %s\n", r.configPath)
	if cliUsesCustomDomain(settingsValue) {
		fmt.Printf("Domain:    %s\n", settingsValue.DefaultDomain)
	} else {
		fmt.Printf("Domain:    %s\n", "not used in quick tunnel mode")
	}
	fmt.Printf("Tunnel:    %s\n", settingsValue.TunnelName)
	fmt.Printf("Origin:    %s\n", settingsValue.DefaultServiceURL)
	fmt.Printf("Projects:  %d\n", len(settingsValue.Projects))
	fmt.Printf("cloudflared: %s\n", status)
	if detectErr != nil {
		fmt.Printf("cloudflared error: %v\n", detectErr)
	}
	r.printEnvKitStatus()
	return nil
}

// printEnvKitStatus reports whether EnvKit was detected on this host. When
// detected and the user has not customised the default origin, the CLI hints
// that Exposely will route traffic through EnvKit's HTTPS URL automatically.
func (r *cliRunner) printEnvKitStatus() {
	info := r.refreshEnvKit()
	if !info.Detected {
		return
	}
	version := info.Version
	if version == "" {
		version = "unknown"
	}
	fmt.Printf("EnvKit:    detected (%s) at %s\n", version, info.InstallPath)
	if localstack.IsBuiltInDefaultServiceURL(r.currentDefaultServiceURL()) {
		fmt.Printf("EnvKit:    default origin swapped to %s\n", info.SuggestedOriginURL)
	}
}

// currentDefaultServiceURL returns the saved default service URL so the CLI
// can decide whether to apply the EnvKit fallback without re-reading settings
// everywhere. Returns "" when the store cannot be read.
func (r *cliRunner) currentDefaultServiceURL() string {
	settingsValue, err := r.store.Load()
	if err != nil {
		return ""
	}
	return settingsValue.DefaultServiceURL
}

func (r *cliRunner) printProjects() error {
	settingsValue, err := r.store.Load()
	if err != nil {
		return err
	}
	settingsValue = r.normalizeSettings(settingsValue)
	if len(settingsValue.Projects) == 0 {
		fmt.Println("No saved projects.")
		return nil
	}

	idWidth := len("ID")
	nameWidth := len("NAME")
	typeWidth := len("TYPE")
	targetWidth := len("TARGET")

	rows := make([][4]string, 0, len(settingsValue.Projects))
	for _, project := range settingsValue.Projects {
		projectType, target := r.projectRow(project)
		row := [4]string{project.ID, project.DisplayName, projectType, target}
		rows = append(rows, row)
		if len(row[0]) > idWidth {
			idWidth = len(row[0])
		}
		if len(row[1]) > nameWidth {
			nameWidth = len(row[1])
		}
		if len(row[2]) > typeWidth {
			typeWidth = len(row[2])
		}
		if len(row[3]) > targetWidth {
			targetWidth = len(row[3])
		}
	}

	headerFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds\n", idWidth, nameWidth, typeWidth, targetWidth)
	rowFmt := headerFmt
	fmt.Printf(headerFmt, r.colorize("ID", "1;37"), r.colorize("NAME", "1;37"), r.colorize("TYPE", "1;37"), r.colorize("TARGET", "1;37"))
	fmt.Println(strings.Repeat("-", idWidth+nameWidth+typeWidth+targetWidth+6))
	for _, row := range rows {
		fmt.Printf(rowFmt, row[0], row[1], r.styleProjectType(row[2]), r.highlightURLs(row[3]))
	}
	return nil
}

func (r *cliRunner) handleLog(entry models.LogEntry) {
	r.printLog(entry.Source, entry.Level, entry.Message)
}

func (r *cliRunner) handleStatus(status models.TunnelStatus) {
	if strings.TrimSpace(status.ActiveURL) == "" {
		return
	}
	if status.ActiveURL == r.lastQuickURL {
		return
	}
	r.lastQuickURL = status.ActiveURL
	r.printPublicURL(status.ActiveURL)
}

func (r *cliRunner) printLog(source, level, message string) {
	prefix := fmt.Sprintf("%-7s [%s]", strings.ToUpper(strings.TrimSpace(level)), strings.TrimSpace(source))
	fmt.Fprintf(os.Stderr, "%s %s\n", r.styleLogPrefix(prefix, level), r.highlightURLs(message))
}

func (r *cliRunner) detectCloudflaredPath(configuredPath string) (string, error) {
	if path, err := r.manager.DetectCloudflared(configuredPath); err == nil {
		return path, nil
	}
	if managedPath := r.managedCloudflaredPath(); strings.TrimSpace(managedPath) != "" {
		if path, err := r.manager.DetectCloudflared(managedPath); err == nil {
			return path, nil
		}
	}
	return "", errors.New("cloudflared was not found. Install it or configure the executable path in settings")
}

func (r *cliRunner) managedCloudflaredPath() string {
	return filepath.Join(r.appDataDir, "bin", "cloudflared.exe")
}

func (r *cliRunner) loadProject(projectRef string) (models.AppSettings, models.ProjectPreset, error) {
	settingsValue, err := r.store.Load()
	if err != nil {
		return models.AppSettings{}, models.ProjectPreset{}, err
	}
	settingsValue = r.normalizeSettings(settingsValue)

	normalizedRef := strings.TrimSpace(strings.ToLower(projectRef))
	for _, project := range settingsValue.Projects {
		if strings.EqualFold(strings.TrimSpace(project.ID), normalizedRef) || strings.EqualFold(strings.TrimSpace(project.DisplayName), normalizedRef) {
			return settingsValue, r.normalizeProject(project), nil
		}
	}
	return models.AppSettings{}, models.ProjectPreset{}, fmt.Errorf("project %q not found", projectRef)
}

func (r *cliRunner) normalizeSettings(input models.AppSettings) models.AppSettings {
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
	for i := range output.Projects {
		output.Projects[i] = r.normalizeProject(output.Projects[i])
	}
	return output
}

func (r *cliRunner) normalizeProject(project models.ProjectPreset) models.ProjectPreset {
	project.ID = strings.TrimSpace(project.ID)
	project.DisplayName = strings.TrimSpace(project.DisplayName)
	project.LocalHost = strings.TrimSpace(project.LocalHost)
	project.OriginURL = strings.TrimSpace(project.OriginURL)
	project.Subdomain = strings.TrimSpace(strings.ToLower(project.Subdomain))
	project.PublicURL = strings.TrimSpace(project.PublicURL)
	project.ProjectPath = strings.TrimSpace(project.ProjectPath)
	project.LocalURL = strings.TrimSpace(project.LocalURL)
	project.StartCommand = strings.TrimSpace(project.StartCommand)
	project.ShareMode = normalizeCLIShareMode(project.ShareMode)
	return project
}

func (r *cliRunner) projectSummary(project models.ProjectPreset) string {
	projectType, target := r.projectRow(project)
	return projectType + " -> " + target
}

func (r *cliRunner) projectRow(project models.ProjectPreset) (string, string) {
	switch normalizeCLIShareMode(project.ShareMode) {
	case models.ShareModeHostHTML:
		if project.LocalURL != "" {
			return "HTML", project.LocalURL
		}
		return "HTML", project.ProjectPath
	case models.ShareModeAuto:
		switch {
		case project.LocalURL != "":
			return "AUTO", project.LocalURL
		case project.StartCommand != "":
			return "AUTO", project.StartCommand
		case project.LocalHost != "":
			return "AUTO", project.LocalHost
		default:
			return "AUTO", project.ProjectPath
		}
	case models.ShareModeStable:
		return "STABLE", project.Subdomain
	case models.ShareModeRandomDomain:
		return "RANDOM", "generated subdomain"
	default:
		return "HOST", project.LocalHost
	}
}

func (r *cliRunner) resolveProjectDirectory(projectPath string) (string, error) {
	trimmed := strings.TrimSpace(projectPath)
	if trimmed == "" {
		return "", errors.New("project folder path is required")
	}

	absPath := trimmed
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(r.workDir, absPath)
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("project folder not found or is not a directory: %s", absPath)
	}
	return absPath, nil
}

func (r *cliRunner) serveStaticDirectory(absPath string) (string, int, *http.Server, error) {
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
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			r.printLog("html-server", "info", fmt.Sprintf("Request: %s %s from %s [Host: %s]", req.Method, req.URL.Path, req.RemoteAddr, req.Host))
			http.FileServer(http.Dir(absPath)).ServeHTTP(w, req)
		}),
	}

	go func() {
		r.printLog("html-server", "info", fmt.Sprintf("Static server started on %s for %s", serviceURL, absPath))
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			r.printLog("html-server", "error", fmt.Sprintf("Static server failed: %v", err))
		}
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := checkCLIHTTPService(serviceURL); err != nil {
			r.printLog("html-server", "error", fmt.Sprintf("Self-connectivity check failed for %s: %v", serviceURL, err))
			return
		}
		r.printLog("html-server", "info", fmt.Sprintf("Self-connectivity check successful for %s", serviceURL))
	}()

	return serviceURL, port, server, nil
}

func (r *cliRunner) startProjectCommand(projectDir, commandText string) (<-chan string, error) {
	r.projectMu.Lock()
	defer r.projectMu.Unlock()

	if r.projectCmd != nil && r.projectCmd.Process != nil {
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

	r.projectCmd = cmd
	urlCh := make(chan string, 8)
	r.printLog("project", "info", "Started project command: "+commandText)

	go r.streamProjectPipe("project", stdout, urlCh)
	go r.streamProjectPipe("project", stderr, urlCh)
	go r.waitForProjectCommand(cmd, commandText)

	return urlCh, nil
}

func (r *cliRunner) streamProjectPipe(source string, pipe io.ReadCloser, urlCh chan<- string) {
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
		r.printLog(source, level, line)

		if match := cliLocalServiceURLPattern.FindString(line); match != "" {
			if normalized, ok, err := normalizeCLIServiceURL(match); ok && err == nil {
				select {
				case urlCh <- normalized:
				default:
				}
			}
		}
	}
}

func (r *cliRunner) waitForProjectCommand(cmd *exec.Cmd, commandText string) {
	err := cmd.Wait()

	r.projectMu.Lock()
	if r.projectCmd == cmd {
		r.projectCmd = nil
	}
	r.projectMu.Unlock()

	if err != nil {
		r.printLog("project", "error", "Project command exited: "+err.Error())
		return
	}
	r.printLog("project", "info", "Project command exited: "+commandText)
}

func (r *cliRunner) stopProjectCommand() {
	r.projectMu.Lock()
	defer r.projectMu.Unlock()

	if r.projectCmd == nil || r.projectCmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/PID", fmt.Sprint(r.projectCmd.Process.Pid), "/T", "/F").Run()
	r.projectCmd = nil
}

func (r *cliRunner) resolveHostname(project models.ProjectPreset, domain string, useRandom bool) (string, string) {
	var subdomain string
	if useRandom || normalizeCLIShareMode(project.ShareMode) == models.ShareModeRandomDomain {
		subdomain = randomCLISubdomain()
	} else {
		subdomain = strings.TrimSpace(strings.ToLower(project.Subdomain))
	}
	if subdomain == "" || strings.TrimSpace(domain) == "" {
		return "", ""
	}
	hostname := subdomain + "." + strings.TrimSpace(domain)
	return hostname, "https://" + hostname
}

func (r *cliRunner) updateProjectShare(settingsValue models.AppSettings, projectID, subdomain, fullURL string) models.AppSettings {
	for i := range settingsValue.Projects {
		if settingsValue.Projects[i].ID == projectID {
			settingsValue.Projects[i].PublicURL = fullURL
			if normalizeCLIShareMode(settingsValue.Projects[i].ShareMode) == models.ShareModeStable {
				settingsValue.Projects[i].Subdomain = subdomain
			}
			break
		}
	}
	return settingsValue
}

func validateCLIProject(project models.ProjectPreset) error {
	switch normalizeCLIShareMode(project.ShareMode) {
	case models.ShareModeAuto:
		if project.ProjectPath == "" && project.LocalURL == "" && project.LocalHost == "" {
			return errors.New("Auto mode requires a project folder, local URL, or local host")
		}
	case models.ShareModeHostHTML:
		if project.ProjectPath == "" && project.LocalURL == "" {
			return errors.New("HTML mode requires a project folder or local URL")
		}
	case models.ShareModeStable:
		if project.LocalHost == "" {
			return errors.New("stable mode requires --host")
		}
		if project.Subdomain == "" {
			return errors.New("stable mode requires --subdomain")
		}
	case models.ShareModeRandomDomain:
		if project.LocalHost == "" {
			return errors.New("random-domain mode requires --host")
		}
	case models.ShareModeQuick:
		if project.LocalHost == "" {
			return errors.New("quick mode requires --host")
		}
	default:
		return fmt.Errorf("unsupported share mode %q", project.ShareMode)
	}

	if project.StartCommand != "" && project.ProjectPath == "" {
		return errors.New("a project folder path is required when --start is set")
	}
	return nil
}

func normalizeCLIShareMode(mode models.ShareMode) models.ShareMode {
	switch mode {
	case models.ShareModeAuto, models.ShareModeStable, models.ShareModeRandomDomain, models.ShareModeQuick, models.ShareModeHostHTML:
		return mode
	default:
		return models.ShareModeQuick
	}
}

func normalizeCLIServiceURL(raw string) (string, bool, error) {
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

	return (&url.URL{Scheme: scheme, Host: parsed.Host}).String(), true, nil
}

func checkCLIHTTPService(serviceURL string) error {
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

func detectCLIStaticSiteDir(projectDir string) (string, bool) {
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

// resolveCLIProjectOriginServiceURL mirrors app.resolveProjectOriginServiceURL
// for the CLI: it picks project.OriginURL first, then settings.DefaultServiceURL,
// and finally falls back to the EnvKit-suggested HTTPS origin when EnvKit is
// detected and the user has not customised the default. This keeps the
// desktop app and the CLI consistent in how they handle EnvKit installs.
func resolveCLIProjectOriginServiceURL(project models.ProjectPreset, fallback string, envkit localstack.Info) (string, error) {
	if serviceURL, ok, err := normalizeCLIServiceURL(project.OriginURL); ok {
		if err != nil {
			return "", fmt.Errorf("invalid project origin URL: %w", err)
		}
		return serviceURL, nil
	}
	if serviceURL, ok, err := normalizeCLIServiceURL(fallback); ok {
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

func inferCLIHostFromProjectPath(projectPath string) string {
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

func detectCLILaravelProjectDir(projectDir string) bool {
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

func detectCLICommandPorts(commandText string) []int {
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

func randomCLISubdomain() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("share-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func cliSupportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if runtime.GOOS != "windows" && strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return true
}

func (r *cliRunner) colorize(text, code string) string {
	if !r.useColor || strings.TrimSpace(text) == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (r *cliRunner) styleLogPrefix(prefix, level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "success":
		return r.colorize(prefix, "1;32")
	case "error":
		return r.colorize(prefix, "1;31")
	case "warning", "warn":
		return r.colorize(prefix, "1;33")
	default:
		return r.colorize(prefix, "1;36")
	}
}

func (r *cliRunner) highlightURLs(message string) string {
	if !r.useColor || strings.TrimSpace(message) == "" {
		return message
	}
	return cliAnyURLPattern.ReplaceAllStringFunc(message, func(match string) string {
		return r.colorize(match, "1;96;4")
	})
}

func (r *cliRunner) printPublicURL(rawURL string) {
	label := "PUBLIC URL"
	if r.useColor {
		label = r.colorize(label, "1;30;42")
		rawURL = r.colorize(rawURL, "1;96;4")
	}
	fmt.Printf("\n%s %s\n\n", label, rawURL)
}

func (r *cliRunner) styleProjectType(projectType string) string {
	switch strings.ToUpper(strings.TrimSpace(projectType)) {
	case "HOST":
		return r.colorize(projectType, "1;34")
	case "HTML":
		return r.colorize(projectType, "1;35")
	case "AUTO":
		return r.colorize(projectType, "1;36")
	case "STABLE":
		return r.colorize(projectType, "1;32")
	case "RANDOM":
		return r.colorize(projectType, "1;33")
	default:
		return projectType
	}
}

func cliUsesCustomDomain(settingsValue models.AppSettings) bool {
	domain := strings.TrimSpace(strings.ToLower(settingsValue.DefaultDomain))
	if domain != "" && domain != "example.com" {
		return true
	}
	for _, project := range settingsValue.Projects {
		switch normalizeCLIShareMode(project.ShareMode) {
		case models.ShareModeStable, models.ShareModeRandomDomain:
			return true
		}
	}
	return false
}
