package stacks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// dyingScript writes a tiny script that exits immediately with code 1 —
// a stand-in for a crashing php-cgi.
func dyingScript(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "die.cmd")
	if err := os.WriteFile(p, []byte("@echo off\r\nexit /b 1\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// fastBackoff compresses the restart schedule so the crash-loop tests
// finish in well under a second instead of waiting out the real
// 1s/2s/4s/8s ladder. Production uses defaultRestartBackoff.
func fastBackoff() restartBackoff {
	return restartBackoff{
		first:       20 * time.Millisecond,
		max:         80 * time.Millisecond,
		maxFailures: 6,
		stable:      time.Second,
	}
}

// newFastManager returns a Manager whose backoff is compressed for tests.
func newFastManager() *Manager {
	m := NewManager()
	m.backoff = fastBackoff()
	return m
}

func TestRestartBackoffSchedule(t *testing.T) {
	backoff := defaultRestartBackoff()

	for _, tc := range []struct {
		failures int
		want     time.Duration
	}{
		{0, time.Second},
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 8 * time.Second},
		{6, 8 * time.Second},
	} {
		if got := backoff.delayFor(tc.failures); got != tc.want {
			t.Errorf("delayFor(%d) = %s, want %s", tc.failures, got, tc.want)
		}
	}

	if backoff.maxFailures != 6 {
		t.Errorf("maxFailures = %d, want 6 consecutive crashes before pausing", backoff.maxFailures)
	}
	if backoff.stable != 30*time.Second {
		t.Errorf("stable = %s, want 30s uptime to reset the crash counter", backoff.stable)
	}
}

func TestSupervisorAutoRestartsCrashedService(t *testing.T) {
	dir := t.TempDir()
	m := newFastManager()
	svc := Service("crashy")
	m.SetConfig(svc, ServiceConfig{BinaryPath: dyingScript(t, dir)})

	st := m.Start(svc)
	if !st.Running {
		t.Fatalf("initial start failed: %+v", st)
	}
	firstPID := st.PID

	// The script exits instantly; the supervisor should relaunch it.
	// A dying-in-milliseconds process is almost never caught in the
	// Running state, so detect restarts by the PID generation changing.
	deadline := time.Now().Add(5 * time.Second)
	restarted := false
	for time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		cur := m.Status(svc)
		if cur.PID != 0 && cur.PID != firstPID {
			restarted = true
			break
		}
	}
	if !restarted {
		t.Fatalf("service was not auto-restarted; status: %+v", m.Status(svc))
	}

	// Stop must end the cycle for good.
	m.Stop(svc)
	time.Sleep(500 * time.Millisecond) // any in-flight restart timer must stand down
	if cur := m.Status(svc); cur.Running {
		t.Fatalf("service still running after Stop: %+v", cur)
	}
}

func TestSupervisorStopsRetryingEventually(t *testing.T) {
	dir := t.TempDir()
	m := newFastManager()
	svc := Service("crashy2")
	m.SetConfig(svc, ServiceConfig{BinaryPath: dyingScript(t, dir)})

	if st := m.Start(svc); !st.Running {
		t.Fatalf("initial start failed: %+v", st)
	}

	// Crash-looping: after more than maxFailures crashes the supervisor
	// pauses with a clear LastError instead of spinning forever. With the
	// compressed backoff that ladder takes a few hundred milliseconds.
	deadline := time.Now().Add(15 * time.Second)
	paused := false
	for time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		if cur := m.Status(svc); !cur.Running && cur.LastError != "" && strings.Contains(cur.LastError, "auto-restart paused") {
			paused = true
			break
		}
	}
	if !paused {
		t.Fatalf("supervisor did not pause after repeated crashes: %+v", m.Status(svc))
	}
}
