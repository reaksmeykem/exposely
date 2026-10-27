// Package stacks runs local service binaries (nginx, PHP FastCGI, MySQL)
// as managed child processes. It complements the detection logic in
// internal/localstack: localstack finds stacks that are already installed
// and running, while this package starts and stops the underlying binaries
// on the user's behalf.
//
// Design notes:
//   - Every service is a plain child process started with
//     sysproc.Hidden() so no console window flashes on Windows.
//   - Services are referenced by explicit binary paths supplied by the
//     caller (Settings / UI). Nothing is auto-downloaded and nothing is
//     run unless the user configures it, keeping this opt-in and safe.
//   - The Manager serialises state changes with a mutex so start/stop
//     calls from the UI and CLI cannot race each other.
package stacks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// Service identifies one of the managed stack services.
type Service string

const (
	// ServiceNginx is the nginx web server binary.
	ServiceNginx Service = "nginx"
	// ServicePHP is the php-cgi (FastCGI Process Manager) binary.
	ServicePHP Service = "php"
	// ServiceMySQL is the mysqld server binary.
	ServiceMySQL Service = "mysql"
)

// ServiceConfig describes how to launch one service.
type ServiceConfig struct {
	// BinaryPath is the absolute path to the executable (nginx.exe,
	// php-cgi.exe, mysqld.exe). Required.
	BinaryPath string `json:"binaryPath"`
	// Args are the extra command-line arguments passed to the binary
	// (e.g. ["-p", "C:\\nginx", "-c", "conf\\nginx.conf"]).
	Args []string `json:"args,omitempty"`
	// WorkDir is the working directory for the process. When empty the
	// binary's parent directory is used, which is what nginx and
	// php-cgi expect (they resolve relative paths against their own
	// location).
	WorkDir string `json:"workDir,omitempty"`
	// Env is a list of KEY=VALUE pairs appended to the child
	// environment (e.g. MYSQL_HOME). Existing environment is kept.
	Env []string `json:"env,omitempty"`
}

// Status describes the current state of one managed service.
type Status struct {
	Service Service `json:"service"`
	Running bool    `json:"running"`
	// PID is the child process id when Running is true, 0 otherwise.
	PID int `json:"pid"`
	// StartedAt is when the process was started as an RFC3339 string
	// (empty when not running). A string rather than time.Time because
	// Wails binding generation cannot resolve time.Time in bound types.
	StartedAt string `json:"startedAt"`
	// LastError holds the most recent start/stop failure message so the
	// UI can surface it without needing a separate log stream.
	LastError string `json:"lastError,omitempty"`
}

// Manager owns the lifecycle of the configured stack services.
type Manager struct {
	mu       sync.Mutex
	configs  map[Service]ServiceConfig
	procs    map[Service]*managedProc
	logLimit int
	// backoff paces auto-restart after a crash; tests replace it with a
	// compressed schedule so they do not depend on wall-clock seconds.
	backoff restartBackoff
}

// managedProc wraps a running child process plus its metadata.
type managedProc struct {
	cmd       *exec.Cmd
	pid       int
	startedAt time.Time
	lastError string
	// desired is true while the user wants this service running. The
	// supervisor only auto-restarts a child that exited while desired;
	// an explicit Stop clears it so the restart loop stands down.
	desired bool
	// done is closed by the exit watcher when the child exits, so
	// status can distinguish a live process from one that already
	// died without blocking on Wait.
	done chan struct{}
}

// alive reports whether the tracked process is still running.
func (p *managedProc) alive() bool {
	if p == nil || p.done == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// NewManager returns a Manager with no configured services. Call
// SetConfig for each service the user wants Exposely to manage.
func NewManager() *Manager {
	return &Manager{
		configs:  make(map[Service]ServiceConfig),
		procs:    make(map[Service]*managedProc),
		logLimit: 200,
		backoff:  defaultRestartBackoff(),
	}
}

// SetConfig registers (or replaces) the launch configuration for a
// service. Changing the config of a running service is allowed; it takes
// effect the next time the service is (re)started.
func (m *Manager) SetConfig(service Service, cfg ServiceConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[service] = cfg
}

// Config returns the registered config for a service and whether one was
// registered at all.
func (m *Manager) Config(service Service) (ServiceConfig, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.configs[service]
	return cfg, ok
}

// Start launches the service as a hidden child process. It returns the
// resulting Status; Running is false together with a LastError when the
// launch failed.
//
// Start does not wait for the service to become "ready" (nginx binds its
// port almost instantly, mysqld can take seconds); callers that need
// readiness should probe the port themselves — the same way
// localstack.probeLoopback does for ports 80/443.
func (m *Manager) Start(service Service) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startLocked(service, 0)
}

// startLocked launches (or relaunches) the service; caller holds m.mu.
// failures is the consecutive-crash count carried across restarts so a
// crash-looping service keeps escalating its backoff and eventually
// pauses instead of spinning forever.
func (m *Manager) startLocked(service Service, failures int) Status {
	cfg, ok := m.configs[service]
	if !ok || filepath.Clean(cfg.BinaryPath) == "." || filepath.Clean(cfg.BinaryPath) == "" {
		return m.setStatus(service, false, fmt.Sprintf("%s is not configured: set the binary path first", service))
	}
	if p, running := m.procs[service]; running && p.alive() {
		// Already running — report current state instead of double starting.
		return m.statusLocked(service)
	}

	bin, err := filepath.Abs(cfg.BinaryPath)
	if err != nil {
		return m.setStatus(service, false, fmt.Sprintf("resolve %s binary path: %v", service, err))
	}
	if _, err := os.Stat(bin); err != nil {
		return m.setStatus(service, false, fmt.Sprintf("%s binary not found: %s", service, bin))
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(bin)
	}

	cmd := exec.Command(bin, cfg.Args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), cfg.Env...)
	cmd.SysProcAttr = sysproc.Hidden()

	if err := cmd.Start(); err != nil {
		return m.setStatus(service, false, fmt.Sprintf("start %s: %v", service, err))
	}

	proc := &managedProc{
		cmd:       cmd,
		pid:       cmd.Process.Pid,
		startedAt: time.Now(),
		desired:   true,
		done:      make(chan struct{}),
	}
	// Watch for child exit so status stays honest when a service dies
	// after a successful launch (port clash, config error, crash), and
	// supervise: desired services are relaunched automatically.
	go m.supervise(service, proc, failures)
	m.procs[service] = proc
	return m.statusLocked(service)
}

// restartBackoff controls how fast and how often a crash-looping service is
// relaunched, and when the supervisor gives up.
type restartBackoff struct {
	// first is the delay after the first crash. It doubles with every
	// further consecutive failure until it reaches max.
	first time.Duration
	// max caps the exponential growth and is the steady-state delay.
	max time.Duration
	// maxFailures is how many consecutive crashes are tolerated before
	// auto-restart pauses and asks the user to fix the cause.
	maxFailures int
	// stable is the uptime that counts as a fresh start: a service that
	// stayed up at least this long resets the failure counter, so one
	// bad request (PHP fatal error) recovers in ~1s while a broken
	// binary does not spin.
	stable time.Duration
}

// defaultRestartBackoff is the production schedule: 1s, 2s, 4s, 8s, 8s,
// then 8s; the seventh consecutive crash pauses the supervisor. Tests
// shrink it so they never depend on wall-clock seconds.
func defaultRestartBackoff() restartBackoff {
	return restartBackoff{
		first:       1 * time.Second,
		max:         8 * time.Second,
		maxFailures: 6,
		stable:      30 * time.Second,
	}
}

// delayFor returns how long to wait before the next relaunch after
// failures consecutive crashes: 1s, 2s, 4s, then max (failures < 1 is
// treated as the first crash).
func (b restartBackoff) delayFor(failures int) time.Duration {
	delay := b.first
	for i := 1; i < failures && delay < b.max; i++ {
		delay *= 2
	}
	if delay > b.max {
		delay = b.max
	}
	return delay
}

// supervise waits for the child to exit and, when the user still wants
// the service running, relaunches it after a backoff. Each relaunch
// spawns a fresh supervise; an explicit Stop is honoured at every step.
func (m *Manager) supervise(service Service, proc *managedProc, failures int) {
	waitErr := proc.cmd.Wait()
	close(proc.done)

	m.mu.Lock()
	backoff := m.backoff
	// Superseded by a newer generation (restart raced a Stop/SetConfig)?
	if m.procs[service] != proc {
		m.mu.Unlock()
		return
	}
	if !proc.desired {
		// Stopped on purpose — record clean stopped state.
		delete(m.procs, service)
		m.setStatus(service, false, "")
		m.mu.Unlock()
		return
	}

	// Unexpected exit while desired: backoff and relaunch.
	if waitErr == nil || time.Since(proc.startedAt) >= backoff.stable {
		failures = 0
	} else {
		failures++
	}
	if failures > backoff.maxFailures {
		delete(m.procs, service)
		m.setStatus(service, false, fmt.Sprintf(
			"%s exited %d times in a row — auto-restart paused. Fix the cause (check logs), then start it again",
			service, failures))
		m.mu.Unlock()
		return
	}
	delay := backoff.delayFor(failures)
	m.setStatus(service, false, fmt.Sprintf("%s exited unexpectedly — restarting in %s (attempt %d)", service, delay, failures+1))
	m.mu.Unlock()

	go func() {
		time.Sleep(delay)
		m.mu.Lock()
		defer m.mu.Unlock()
		// Stopped or replaced while we slept?
		if m.procs[service] != proc || !proc.desired {
			return
		}
		delete(m.procs, service)
		m.startLocked(service, failures)
	}()
}

// Stop terminates a running service gracefully. nginx is stopped via
// "nginx -s stop" semantics are handled by the caller-supplied Args, so
// here we simply kill the tracked child process; on Windows the graceful
// nginx shutdown signal (SIGQUIT equivalent) is unavailable anyway.
func (m *Manager) Stop(service Service) Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.procs[service]
	if !ok || !p.alive() {
		// Clear desired so a pending auto-restart (backoff sleep) stands
		// down instead of relaunching a service the user just stopped.
		if ok {
			p.desired = false
		}
		delete(m.procs, service)
		return m.setStatus(service, false, "")
	}
	// Mark not-desired BEFORE killing: the supervisor fires when Wait
	// returns and must not relaunch a process the user just stopped.
	p.desired = false
	// Kill the whole process tree, not just the direct child: nginx on
	// Windows runs master + worker, and killing the master alone orphans
	// the worker, which keeps serving the stale config and holding the
	// listen ports — the next Start then cannot bind. taskkill /T /F
	// reaches the workers; fall back to a direct kill when it is
	// unavailable (non-Windows) or fails. Ignore errors — the process
	// may already have exited on its own. Adopted procs have no exec.Cmd,
	// so use the recorded PID.
	if p.pid > 0 {
		if err := KillPID(p.pid); err != nil && p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
	}
	delete(m.procs, service)
	return m.setStatus(service, false, "")
}

// StopAll stops every running service. Order matters for a typical
// LEMP stack: stop PHP and MySQL before nginx so in-flight FastCGI
// requests can drain first. StopAll never returns an error; per-service
// failures are recorded in each Status.
func (m *Manager) StopAll() map[Service]Status {
	order := []Service{ServiceNginx, ServicePHP, ServiceMySQL}
	out := make(map[Service]Status, len(order))
	for _, s := range order {
		out[s] = m.Stop(s)
	}
	return out
}

// Adopt attaches to an already-running service process that this Manager
// did not spawn — typically a child left behind when a previous app
// instance was force-killed (an in-place update or crash). Without this
// the service shows as stopped while still holding its port and data
// files, and every Start spawns a duplicate that immediately aborts.
//
// Adopted processes are reported as running and are stopped by Stop, but
// they are never auto-restarted: Exposely did not launch them.
func (m *Manager) Adopt(service Service, pid int) Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pid <= 0 || !ProcessAlive(pid) {
		return m.statusLocked(service)
	}
	if p, ok := m.procs[service]; ok && p.alive() {
		return m.statusLocked(service)
	}
	proc := &managedProc{
		pid:       pid,
		startedAt: time.Now(),
		desired:   false,
		done:      make(chan struct{}),
	}
	m.procs[service] = proc
	go m.watchAdopted(service, proc)
	return m.statusLocked(service)
}

// watchAdopted keeps status honest for a process the manager did not
// spawn (there is no exec.Cmd.Wait) by polling until it exits.
func (m *Manager) watchAdopted(service Service, proc *managedProc) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-proc.done:
			return
		case <-ticker.C:
			if ProcessAlive(proc.pid) {
				continue
			}
			m.mu.Lock()
			if m.procs[service] == proc {
				close(proc.done)
				delete(m.procs, service)
				m.setStatus(service, false, "")
			}
			m.mu.Unlock()
			return
		}
	}
}

// Status returns the current state of one service.
func (m *Manager) Status(service Service) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked(service)
}

// AllStatus returns the state of every service, including ones that are
// merely configured but not running.
func (m *Manager) AllStatus() []Status {
	services := []Service{ServiceNginx, ServicePHP, ServiceMySQL}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(services))
	for _, s := range services {
		out = append(out, m.statusLocked(s))
	}
	return out
}

// statusLocked builds a Status for the service; the caller must hold m.mu.
func (m *Manager) statusLocked(service Service) Status {
	p := m.procs[service]
	if p == nil {
		return Status{Service: service, LastError: p.lastErrorIfAny()}
	}
	running := p.alive()
	startedAt := ""
	if running {
		startedAt = p.startedAt.Format(time.RFC3339)
	}
	return Status{
		Service:   service,
		Running:   running,
		PID:       p.pid,
		StartedAt: startedAt,
		LastError: p.lastError,
	}
}

// SetLastError records a message against a service that is not running,
// so callers that discover a blocked start — a port owned by another
// program, say — can explain it in Status instead of leaving the UI with
// a bare "stopped". The state of a live child is never overwritten.
func (m *Manager) SetLastError(service Service, errMsg string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.procs[service]; ok && p != nil {
		if p.alive() {
			return m.statusLocked(service)
		}
		p.lastError = errMsg
		return m.statusLocked(service)
	}
	m.procs[service] = &managedProc{lastError: errMsg}
	return m.statusLocked(service)
}

// setStatus records a transition (including an error) and returns it.
func (m *Manager) setStatus(service Service, running bool, errMsg string) Status {
	if errMsg != "" {
		if p, ok := m.procs[service]; ok && p != nil {
			p.lastError = errMsg
		} else {
			m.procs[service] = &managedProc{lastError: errMsg}
		}
	}
	return Status{
		Service:   service,
		Running:   running,
		LastError: errMsg,
	}
}

func pidOf(cmd *exec.Cmd) int {
	if cmd == nil || cmd.Process == nil {
		return 0
	}
	return cmd.Process.Pid
}

// lastErrorIfAny avoids dereferencing a nil managedProc.
func (p *managedProc) lastErrorIfAny() string {
	if p == nil {
		return ""
	}
	return p.lastError
}

// ValidateBinaryPath reports whether the supplied path points at an
// existing file. It is a helper for Settings/UI layers so they can show
// a validation message before saving the config.
func ValidateBinaryPath(path string) error {
	if path == "" {
		return fmt.Errorf("binary path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not an executable: %s", path)
	}
	return nil
}

// FormatArgs renders an Args slice the way a user would type it on the
// command line. Handy for logs and debug output.
func FormatArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		if needsQuoting(a) {
			out += strconv.Quote(a)
			continue
		}
		out += a
	}
	return out
}

func needsQuoting(arg string) bool {
	if arg == "" {
		return true
	}
	for _, r := range arg {
		if r == ' ' || r == '"' {
			return true
		}
	}
	return false
}
