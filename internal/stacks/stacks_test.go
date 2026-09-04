package stacks

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// sleepHelperPath returns a command that runs a tiny "server" for the
// duration of the test. We use the Go toolchain itself (go run) so the
// tests stay dependency-free; on every supported platform "go" is
// guaranteed to exist because the tests are compiled with it.
func sleepHelperPath(t *testing.T) (bin string, args []string) {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	return goBin, []string{"run", helperScriptPath(t)}
}

// helperScriptPath writes a trivial long-running program to temp dir.
func helperScriptPath(t *testing.T) string {
	t.Helper()
	src := "package main\n\nimport (\n\t\"os\"\n\t\"os/signal\"\n\t\"time\"\n)\n\nfunc main() {\n\tch := make(chan os.Signal, 1)\n\tsignal.Notify(ch, os.Interrupt)\n\tgo func() { time.Sleep(30 * time.Second); os.Exit(0) }()\n\t<-ch\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "helper_main.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	return path
}

func TestManagerStartStopLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process test in -short mode")
	}
	bin, args := sleepHelperPath(t)

	m := NewManager()
	m.SetConfig(ServiceNginx, ServiceConfig{BinaryPath: bin, Args: args})

	st := m.Start(ServiceNginx)
	if !st.Running {
		t.Fatalf("expected nginx service to start, got: %+v", st)
	}
	if st.PID == 0 {
		t.Fatalf("expected non-zero PID, got %+v", st)
	}

	got := m.Status(ServiceNginx)
	if !got.Running || got.PID != st.PID {
		t.Fatalf("status mismatch: %+v vs %+v", got, st)
	}

	stopped := m.Stop(ServiceNginx)
	if stopped.Running {
		t.Fatalf("expected service to report stopped, got %+v", stopped)
	}
	if after := m.Status(ServiceNginx); after.Running {
		t.Fatalf("expected status running=false after Stop, got %+v", after)
	}
}

func TestManagerStartUnconfigured(t *testing.T) {
	m := NewManager()
	st := m.Start(ServiceMySQL)
	if st.Running {
		t.Fatal("unconfigured service must not run")
	}
	if st.LastError == "" {
		t.Fatal("expected a LastError explaining the service is unconfigured")
	}
}

func TestManagerStartMissingBinary(t *testing.T) {
	m := NewManager()
	m.SetConfig(ServicePHP, ServiceConfig{BinaryPath: filepath.Join(t.TempDir(), "definitely-missing.exe")})
	st := m.Start(ServicePHP)
	if st.Running {
		t.Fatal("missing binary must not run")
	}
	if st.LastError == "" {
		t.Fatal("expected LastError for missing binary")
	}
}

func TestManagerDoubleStartIsSafe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process test in -short mode")
	}
	bin, args := sleepHelperPath(t)
	m := NewManager()
	m.SetConfig(ServiceNginx, ServiceConfig{BinaryPath: bin, Args: args})

	first := m.Start(ServiceNginx)
	second := m.Start(ServiceNginx)
	if !first.Running || !second.Running {
		t.Fatalf("both starts should report running: %+v %+v", first, second)
	}
	if first.PID != second.PID {
		t.Fatalf("double start spawned two processes: %d vs %d", first.PID, second.PID)
	}
	_ = m.Stop(ServiceNginx)
}

func TestStopAllCoversAllServices(t *testing.T) {
	m := NewManager()
	// Nothing configured: StopAll must still return an entry per service.
	all := m.StopAll()
	for _, s := range []Service{ServiceNginx, ServicePHP, ServiceMySQL} {
		st, ok := all[s]
		if !ok {
			t.Fatalf("StopAll missing entry for %s", s)
		}
		if st.Running {
			t.Fatalf("%s should not be running", s)
		}
	}
}

func TestValidateBinaryPath(t *testing.T) {
	if err := ValidateBinaryPath(""); err == nil {
		t.Fatal("empty path must fail validation")
	}
	if err := ValidateBinaryPath(filepath.Join(t.TempDir(), "nope.exe")); err == nil {
		t.Fatal("missing file must fail validation")
	}
	if err := ValidateBinaryPath(t.TempDir()); err == nil {
		t.Fatal("directory must fail validation")
	}
	if err := ValidateBinaryPath(goTestExecutable(t)); err != nil {
		t.Fatalf("real executable should validate: %v", err)
	}
}

func goTestExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("cannot resolve test binary: %v", err)
	}
	return exe
}

func TestFormatArgs(t *testing.T) {
	// strconv.Quote escapes backslashes, so the rendered arg keeps a
	// Go-style quoted form of the Windows path.
	got := FormatArgs([]string{"-p", `C:\Program Files\nginx`})
	want := `-p "C:\\Program Files\\nginx"`
	if got != want {
		t.Fatalf("FormatArgs = %q, want %q", got, want)
	}
	if FormatArgs(nil) != "" {
		t.Fatal("nil args should format to empty string")
	}
}

func TestSetConfigRoundTrip(t *testing.T) {
	m := NewManager()
	cfg := ServiceConfig{BinaryPath: "php-cgi.exe", Args: []string{"-b", "127.0.0.1:9000"}}
	m.SetConfig(ServicePHP, cfg)
	got, ok := m.Config(ServicePHP)
	if !ok {
		t.Fatal("config should be registered")
	}
	if len(got.Args) != 2 || got.Args[1] != "127.0.0.1:9000" {
		t.Fatalf("config round-trip mismatch: %+v", got)
	}
	if _, ok := m.Config(ServiceMySQL); ok {
		t.Fatal("mysql config should not be registered yet")
	}
	if runtime.GOOS == "windows" {
		// Silence unused-import style issues on other platforms.
		_ = time.Now
	}
}
