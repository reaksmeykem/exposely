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

func TestSupervisorAutoRestartsCrashedService(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
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
	deadline := time.Now().Add(12 * time.Second)
	restarted := false
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
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
	time.Sleep(3 * time.Second) // any in-flight restart timer must stand down
	if cur := m.Status(svc); cur.Running {
		t.Fatalf("service still running after Stop: %+v", cur)
	}
}

func TestSupervisorStopsRetryingEventually(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	svc := Service("crashy2")
	m.SetConfig(svc, ServiceConfig{BinaryPath: dyingScript(t, dir)})

	if st := m.Start(svc); !st.Running {
		t.Fatalf("initial start failed: %+v", st)
	}

	// Crash-looping: after >6 failures the supervisor pauses with a
	// clear LastError instead of spinning forever. With 1s,2s,4s,8s,8s,8s
	// delays, give it up to 45s.
	deadline := time.Now().Add(45 * time.Second)
	paused := false
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if cur := m.Status(svc); !cur.Running && cur.LastError != "" && strings.Contains(cur.LastError, "auto-restart paused") {
			paused = true
			break
		}
	}
	if !paused {
		t.Fatalf("supervisor did not pause after repeated crashes: %+v", m.Status(svc))
	}
}
