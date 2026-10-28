package stacks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// PID files make service state survive across process lifetimes. The
// desktop app keeps its Manager in memory, but the CLI is a fresh
// process on every invocation, so `exposely stack stop` would otherwise
// have nothing to stop. Each service stores its PID in
// <stateDir>/<service>.pid.

// PIDFileDir returns the directory used for PID files under the
// supplied app data directory.
func PIDFileDir(appDataDir string) string {
	return filepath.Join(appDataDir, "stacks", "pids")
}

// SavePID records a service's child PID. Best-effort: failures are
// non-fatal because in-memory tracking still works for the desktop app.
func SavePID(appDataDir string, service Service, pid int) error {
	dir := PIDFileDir(appDataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, string(service)+".pid"), []byte(strconv.Itoa(pid)), 0o644)
}

// LoadPID returns the recorded PID for a service (0 when none).
func LoadPID(appDataDir string, service Service) int {
	data, err := os.ReadFile(filepath.Join(PIDFileDir(appDataDir), string(service)+".pid"))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// ClearPID removes a service's PID file.
func ClearPID(appDataDir string, service Service) {
	_ = os.Remove(filepath.Join(PIDFileDir(appDataDir), string(service)+".pid"))
}

// ProcessAlive reports whether a process with the supplied PID is
// currently running on Windows (via the OpenProcess syscall through
// tasklist, kept simple and dependency-free).
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	cmd.SysProcAttr = sysproc.Hidden()
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}

// ProcessMatches reports whether pid's executable image is exePath. A PID
// recorded in a stale pid file can be recycled by an unrelated process
// after a crash or reboot; callers use this to refuse to kill (or adopt)
// something that is not the configured service binary.
func ProcessMatches(pid int, exePath string) bool {
	if pid <= 0 {
		return false
	}
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return false
	}
	actual, err := sysproc.ProcessExecutablePath(pid)
	if err != nil {
		return false
	}
	actualAbs, err := filepath.Abs(actual)
	if err != nil {
		return false
	}
	wantAbs, err := filepath.Abs(exePath)
	if err != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(actualAbs), filepath.Clean(wantAbs))
}

// KillPID terminates a process tree by PID (matching how the rest of
// Exposely stops children with taskkill /T /F).
func KillPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	cmd.SysProcAttr = sysproc.Hidden()
	return cmd.Run()
}

// Ensure ProcessAlive works on non-Windows builds too. tasklist does not
// exist there; signal 0 probing is the portable alternative. This file
// is Windows-first but the fallback keeps tests green under GOOS=linux.
var _ = syscall.Getpid
