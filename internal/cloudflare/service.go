package cloudflare

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"cloudflaretunnelmanager/internal/models"
)

var quickTunnelURLPattern = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type TunnelInfo struct {
	ID              string
	Name            string
	CredentialsFile string
}

type Manager struct {
	configPath string
	logSink    func(models.LogEntry)
	statusSink func(models.TunnelStatus)

	mu         sync.Mutex
	status     models.TunnelStatus
	cmd        *exec.Cmd
	stopPID    int
	quickHomes map[int]string
}

func NewManager(configPath string, logSink func(models.LogEntry), statusSink func(models.TunnelStatus)) *Manager {
	return &Manager{
		configPath: configPath,
		logSink:    logSink,
		statusSink: statusSink,
		quickHomes: map[int]string{},
		status: models.TunnelStatus{
			Mode:     "named",
			LastLogs: []models.LogEntry{},
		},
	}
}

func (m *Manager) Status() models.TunnelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) DetectCloudflared(configuredPath string) (string, error) {
	candidates := []string{
		strings.TrimSpace(configuredPath),
		filepath.Join(filepath.Dir(m.configPath), "bin", "cloudflared.exe"),
		filepath.Join(filepath.Dir(os.Args[0]), "cloudflared.exe"),
		filepath.Join(filepath.Dir(os.Args[0]), "bin", "cloudflared.exe"),
		"cloudflared",
		`C:\Program Files\cloudflared\cloudflared.exe`,
		`C:\Program Files (x86)\cloudflared\cloudflared.exe`,
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.EqualFold(candidate, "cloudflared") {
			if resolved, err := exec.LookPath(candidate); err == nil {
				return resolved, nil
			}
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}

	return "", errors.New("cloudflared was not found. Install it or set the executable path in settings")
}

func (m *Manager) EnsureNamedTunnel(cloudflaredPath, tunnelName string) (TunnelInfo, error) {
	if strings.TrimSpace(tunnelName) == "" {
		return TunnelInfo{}, errors.New("tunnel name is required")
	}

	tunnels, err := m.ListTunnels(cloudflaredPath)
	if err != nil {
		return TunnelInfo{}, err
	}
	for _, tunnel := range tunnels {
		if strings.EqualFold(tunnel.Name, tunnelName) {
			return tunnel, nil
		}
	}

	return m.CreateTunnel(cloudflaredPath, tunnelName)
}

func (m *Manager) ListTunnels(cloudflaredPath string) ([]TunnelInfo, error) {
	attempts := [][]string{
		{"tunnel", "list", "-o", "json"},
		{"tunnel", "list", "--output", "json"},
		{"tunnel", "list"},
	}

	var lastErr error
	for _, args := range attempts {
		stdout, stderr, err := runCapture(cloudflaredPath, args...)
		if err != nil {
			lastErr = commandError(stderr, err)
			continue
		}

		if len(args) >= 4 {
			if tunnels, jsonErr := parseTunnelListJSON(stdout); jsonErr == nil {
				return tunnels, nil
			}
		}

		if tunnels, textErr := parseTunnelListText(stdout); textErr == nil && len(tunnels) > 0 {
			return tunnels, nil
		}
	}

	if lastErr == nil {
		lastErr = errors.New("failed to read tunnel list")
	}
	return nil, lastErr
}

func (m *Manager) CreateTunnel(cloudflaredPath, tunnelName string) (TunnelInfo, error) {
	stdout, stderr, err := runCapture(cloudflaredPath, "tunnel", "create", tunnelName)
	if err != nil {
		return TunnelInfo{}, commandError(stderr, err)
	}

	info, parseErr := parseTunnelCreateOutput(stdout, tunnelName)
	if parseErr != nil {
		return TunnelInfo{}, parseErr
	}
	m.pushLog("cloudflared", "success", fmt.Sprintf("Created tunnel %s (%s)", info.Name, info.ID))
	return info, nil
}

func (m *Manager) RouteDNS(cloudflaredPath, tunnelName, hostname string) error {
	_, stderr, err := runCapture(cloudflaredPath, "tunnel", "route", "dns", tunnelName, hostname)
	if err != nil {
		if strings.Contains(strings.ToLower(stderr), "already exists") {
			m.pushLog("cloudflared", "info", fmt.Sprintf("DNS route already exists for %s", hostname))
			return nil
		}
		return commandError(stderr, err)
	}
	m.pushLog("cloudflared", "success", fmt.Sprintf("Ensured DNS route for %s", hostname))
	return nil
}

func (m *Manager) StartNamedTunnel(cloudflaredPath, configPath, tunnelName, tunnelID string, hostnames []string) error {
	m.mu.Lock()
	if m.cmd != nil && m.cmd.Process != nil {
		m.mu.Unlock()
		return errors.New("cloudflared is already running")
	}

	reference := strings.TrimSpace(tunnelID)
	if reference == "" {
		reference = strings.TrimSpace(tunnelName)
	}
	if reference == "" {
		m.mu.Unlock()
		return errors.New("tunnel reference is required")
	}

	args := []string{"tunnel", "--config", configPath, "--loglevel", "info", "run", reference}
	cmd := exec.Command(cloudflaredPath, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}

	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		return err
	}

	m.cmd = cmd
	m.stopPID = 0
	m.status.Running = true
	m.status.Mode = "named"
	m.status.PID = cmd.Process.Pid
	m.status.TunnelName = tunnelName
	m.status.TunnelID = tunnelID
	m.status.ActiveHostnames = hostnames
	m.status.ActiveURL = ""
	m.status.QuickURL = ""
	m.status.LastError = ""
	m.status.ConfigPath = configPath
	m.emitStatusLocked()
	m.mu.Unlock()

	go m.StreamPipe("cloudflared", stdout)
	go m.StreamPipe("cloudflared", stderr)
	go m.waitForProcess("named", cmd)
	logLabel := strings.TrimSpace(tunnelName)
	if logLabel == "" {
		logLabel = reference
	}
	m.pushLog("cloudflared", "info", fmt.Sprintf("Started named tunnel %s", logLabel))
	return nil
}

func (m *Manager) StartQuickTunnel(cloudflaredPath, serviceURL, hostHeader string) error {
	m.mu.Lock()
	if m.cmd != nil && m.cmd.Process != nil {
		m.mu.Unlock()
		return errors.New("cloudflared is already running")
	}

	tempHome, err := os.MkdirTemp("", "cloudflared-quick-*")
	if err != nil {
		m.mu.Unlock()
		return err
	}

	args := []string{"tunnel", "--url", serviceURL}
	if strings.TrimSpace(hostHeader) != "" {
		args = append(args, "--http-host-header", hostHeader)
	}
	cmd := exec.Command(cloudflaredPath, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
	}
	cmd.Env = append(os.Environ(),
		"HOME="+tempHome,
		"USERPROFILE="+tempHome,
		"HOMEDRIVE="+filepath.VolumeName(tempHome),
		"HOMEPATH="+strings.TrimPrefix(tempHome, filepath.VolumeName(tempHome)),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}

	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		return err
	}

	m.cmd = cmd
	m.stopPID = 0
	m.quickHomes[cmd.Process.Pid] = tempHome
	m.status.Running = true
	m.status.Mode = "quick"
	m.status.PID = cmd.Process.Pid
	m.status.ActiveHostnames = nil
	m.status.ActiveURL = ""
	m.status.QuickURL = ""
	m.status.LastError = ""
	m.emitStatusLocked()
	m.mu.Unlock()

	go m.StreamPipe("cloudflared", stdout)
	go m.StreamPipe("cloudflared", stderr)
	go m.waitForProcess("quick", cmd)
	m.pushLog("cloudflared", "info", "Started quick tunnel")
	return nil
}

func (m *Manager) StopTunnel() error {
	m.mu.Lock()
	cmd := m.cmd
	if cmd != nil && cmd.Process != nil {
		m.stopPID = cmd.Process.Pid
	}
	m.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	killCmd := exec.Command("taskkill", "/PID", fmt.Sprint(cmd.Process.Pid), "/T", "/F")
	if runtime.GOOS == "windows" {
		killCmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
	}
	if err := killCmd.Run(); err != nil {
		return err
	}

	m.mu.Lock()
	m.cmd = nil
	m.status.Running = false
	m.status.PID = 0
	m.status.ActiveURL = ""
	m.status.QuickURL = ""
	m.emitStatusLocked()
	m.mu.Unlock()

	m.pushLog("cloudflared", "info", "Stopped cloudflared process")
	return nil
}

func (m *Manager) StreamPipe(source string, pipe io.ReadCloser) {
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
		if strings.Contains(lower, "registered tunnel connection") || strings.Contains(lower, "connected") {
			level = "success"
		}
		m.pushLog(source, level, line)

		if matches := quickTunnelURLPattern.FindString(line); matches != "" {
			m.mu.Lock()
			m.status.ActiveURL = matches
			m.status.QuickURL = matches
			m.emitStatusLocked()
			m.mu.Unlock()
		}
	}
}

func (m *Manager) waitForProcess(mode string, cmd *exec.Cmd) {
	err := cmd.Wait()
	message := "cloudflared exited"
	level := "info"
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	m.mu.Lock()
	expectedStop := m.stopPID != 0 && pid == m.stopPID
	if expectedStop {
		m.stopPID = 0
	}
	if m.cmd == cmd {
		m.cmd = nil
	}
	quickHome := ""
	if pid != 0 {
		quickHome = m.quickHomes[pid]
		delete(m.quickHomes, pid)
	}
	m.status.Running = false
	m.status.PID = 0
	if mode == "quick" {
		m.status.ActiveURL = ""
		m.status.QuickURL = ""
	}
	if err != nil && !expectedStop {
		m.status.LastError = err.Error()
		message = "cloudflared exited: " + err.Error()
		level = "error"
	} else if expectedStop {
		message = "cloudflared stopped"
	}
	m.emitStatusLocked()
	m.mu.Unlock()

	if quickHome != "" {
		_ = os.RemoveAll(quickHome)
	}
	m.pushLog("cloudflared", level, message)
}

func (m *Manager) pushLog(source, level, message string) {
	entry := models.LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Source:    source,
		Level:     level,
		Message:   message,
	}

	m.mu.Lock()
	m.status.LastLogs = append(m.status.LastLogs, entry)
	if len(m.status.LastLogs) > 300 {
		m.status.LastLogs = m.status.LastLogs[len(m.status.LastLogs)-300:]
	}
	if level == "error" {
		m.status.LastError = message
	}
	m.emitStatusLocked()
	m.mu.Unlock()

	if m.logSink != nil {
		m.logSink(entry)
	}
}

func (m *Manager) emitStatusLocked() {
	if m.statusSink != nil {
		m.statusSink(m.status)
	}
}

func runCapture(executable string, args ...string) (string, string, error) {
	cmd := exec.Command(executable, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func parseTunnelListJSON(content string) ([]TunnelInfo, error) {
	type jsonTunnel struct {
		ID   string `json:"id"`
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}

	var items []jsonTunnel
	if err := json.Unmarshal([]byte(content), &items); err != nil {
		return nil, err
	}

	result := make([]TunnelInfo, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = strings.TrimSpace(item.UUID)
		}
		if id == "" || strings.TrimSpace(item.Name) == "" {
			continue
		}
		result = append(result, TunnelInfo{
			ID:              id,
			Name:            item.Name,
			CredentialsFile: defaultCredentialsPath(id),
		})
	}
	return result, nil
}

func parseTunnelListText(content string) ([]TunnelInfo, error) {
	lines := strings.Split(content, "\n")
	result := make([]TunnelInfo, 0)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(strings.ToUpper(line), "ID") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		id := fields[0]
		name := fields[1]
		if !looksLikeTunnelID(id) {
			continue
		}
		result = append(result, TunnelInfo{
			ID:              id,
			Name:            name,
			CredentialsFile: defaultCredentialsPath(id),
		})
	}

	if len(result) == 0 {
		return nil, errors.New("no tunnels found in cloudflared output")
	}
	return result, nil
}

func parseTunnelCreateOutput(content, tunnelName string) (TunnelInfo, error) {
	idPattern := regexp.MustCompile(`(?i)([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
	match := idPattern.FindString(content)
	if match == "" {
		return TunnelInfo{}, errors.New("could not parse tunnel UUID from cloudflared output")
	}
	return TunnelInfo{
		ID:              match,
		Name:            tunnelName,
		CredentialsFile: defaultCredentialsPath(match),
	}, nil
}

func defaultCredentialsPath(tunnelID string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cloudflared", tunnelID+".json")
	}
	return filepath.Join(homeDir, ".cloudflared", tunnelID+".json")
}

func looksLikeTunnelID(value string) bool {
	matched, _ := regexp.MatchString(`(?i)^[0-9a-f-]{32,36}$`, value)
	return matched
}

func commandError(stderr string, err error) error {
	if strings.TrimSpace(stderr) == "" {
		return err
	}
	return fmt.Errorf("%s: %s", err.Error(), strings.TrimSpace(stderr))
}
