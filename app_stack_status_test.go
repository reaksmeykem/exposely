package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/reaksmeykem/exposely/internal/models"
	"github.com/reaksmeykem/exposely/internal/stacks"
	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// These tests cover the case that made the stack panel lie: a service the
// app did not start (a leftover from a crashed instance, or one started by
// the CLI) was reported as stopped, and a port held by a completely
// different program was reported as stopped too — with the real reason
// only visible after clicking Start.

func stackStatusFor(t *testing.T, statuses []stacks.Status, service stacks.Service) stacks.Status {
	t.Helper()
	for _, status := range statuses {
		if status.Service == service {
			return status
		}
	}
	t.Fatalf("no status reported for %s: %+v", service, statuses)
	return stacks.Status{}
}

// isolatedApp builds an App whose settings live in a temp directory, so
// the test never touches the real %AppData%\Exposely.
func isolatedApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	return app
}

func TestStackStatusAdoptsServiceRunningOutsideTheApp(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("finding the listener of a port uses netstat on Windows")
	}

	app := isolatedApp(t)

	// Stand in for a managed service that outlived its parent: a port held
	// by a process that is not this app instance. The test binary itself is
	// the "configured binary", which is what makes it recognisable as ours.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	settingsValue := models.DefaultSettings()
	settingsValue.Stack.NginxBinaryPath = exePath
	settingsValue.Stack.NginxPort = port
	if err := app.store.Save(settingsValue); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	nginx := stackStatusFor(t, app.StackStatus(), stacks.ServiceNginx)
	if !nginx.Running {
		t.Fatalf("nginx on port %d was reported stopped while a matching binary holds the port: %+v", port, nginx)
	}
	if nginx.PID != os.Getpid() {
		t.Fatalf("adopted pid %d, want %d", nginx.PID, os.Getpid())
	}
	if nginx.LastError != "" {
		t.Fatalf("adopted service carries an error: %q", nginx.LastError)
	}

	// A second read must stay consistent (and must not re-adopt or flip).
	again := stackStatusFor(t, app.StackStatus(), stacks.ServiceNginx)
	if !again.Running || again.PID != nginx.PID {
		t.Fatalf("status changed between reads: first %+v, then %+v", nginx, again)
	}
}

func TestStackStatusExplainsPortHeldByAnotherProgram(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("finding the listener of a port uses netstat on Windows")
	}

	app := isolatedApp(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	// The configured mysqld is some other program, so the port owner is not
	// ours: the panel must say why MySQL cannot run instead of "stopped".
	settingsValue := models.DefaultSettings()
	settingsValue.Stack.MySQLDBinaryPath = filepath.Join(t.TempDir(), "mysqld.exe")
	settingsValue.Stack.MySQLPort = port
	if err := app.store.Save(settingsValue); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	mysql := stackStatusFor(t, app.StackStatus(), stacks.ServiceMySQL)
	t.Logf("conflict message shown in the panel: %s", mysql.LastError)
	if mysql.Running {
		t.Fatalf("mysql reported running although the port belongs to another program: %+v", mysql)
	}
	if !strings.Contains(mysql.LastError, "already in use") {
		t.Fatalf("mysql LastError = %q, want the port conflict explained", mysql.LastError)
	}
	// The owner is named with its full path and pid (here it is this test
	// binary, since that is what holds the port), because a bare
	// "mysqld.exe" is ambiguous when the machine runs more than one.
	if !strings.Contains(mysql.LastError, "already in use by ") || !strings.Contains(mysql.LastError, "PID ") {
		t.Fatalf("mysql LastError = %q, want the port owner named with its pid", mysql.LastError)
	}
	if !strings.Contains(mysql.LastError, "not the copy Exposely manages") || !strings.Contains(mysql.LastError, `\`) {
		t.Fatalf("mysql LastError = %q, want the owner's path so the user can tell which program it is", mysql.LastError)
	}
}

func TestStackStatusIgnoresServicesWithoutAConfiguredBinary(t *testing.T) {
	app := isolatedApp(t)

	settingsValue := models.DefaultSettings()
	settingsValue.Stack.NginxBinaryPath = ""
	settingsValue.Stack.PHPCGIBinaryPath = ""
	settingsValue.Stack.MySQLDBinaryPath = ""
	if err := app.store.Save(settingsValue); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	for _, status := range app.StackStatus() {
		if status.Running {
			t.Fatalf("%s reported running with no binary configured: %+v", status.Service, status)
		}
		if status.LastError != "" {
			t.Fatalf("%s carries an error with nothing configured: %q", status.Service, status.LastError)
		}
	}
}

// TestZZListenerHelper is not a test: it is the child process that
// TestStackStatusAdoptsTheOwnerNotTheListener starts. It listens on the
// port named by EXPOSELY_TEST_CHILD_PORT and then blocks, standing in for
// nginx's worker.
func TestZZListenerHelper(t *testing.T) {
	if os.Getenv("EXPOSELY_TEST_CHILD") != "1" {
		t.Skip("helper process, started only by TestStackStatusAdoptsTheOwnerNotTheListener")
	}
	port := os.Getenv("EXPOSELY_TEST_CHILD_PORT")
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatalf("child could not listen on %s: %v", port, err)
	}
	defer listener.Close()
	fmt.Println("LISTENING")
	// Sleep rather than blocking on an empty select: this test binary has no
	// other runnable goroutine, so the runtime's deadlock detector would kill
	// the child (freeing the port) before the parent gets to look at it.
	for {
		time.Sleep(time.Minute)
	}
}

func TestStackStatusAdoptsTheOwnerNotTheListener(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("parent process lookup is Windows-only")
	}

	app := isolatedApp(t)

	// A child of this test process that holds a port: the same situation as
	// nginx, where the worker listens but the master owns start/stop. The
	// child and this process are the same executable, so the owner walk has
	// to climb from the child to this test process.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	child := exec.Command(exePath, "-test.run=TestZZListenerHelper", "-test.v")
	child.Env = append(os.Environ(), "EXPOSELY_TEST_CHILD=1", fmt.Sprintf("EXPOSELY_TEST_CHILD_PORT=%d", port))
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := child.Start(); err != nil {
		t.Fatalf("starting the listener child: %v", err)
	}
	defer func() {
		if child.Process != nil {
			_ = child.Process.Kill()
		}
	}()

	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "LISTENING") {
				ready <- scanner.Text()
				return
			}
		}
		ready <- ""
	}()
	select {
	case line := <-ready:
		if line == "" {
			t.Fatal("listener child never reported that it was listening")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for the listener child")
	}

	settingsValue := models.DefaultSettings()
	settingsValue.Stack.NginxBinaryPath = exePath
	settingsValue.Stack.NginxPort = port
	if err := app.store.Save(settingsValue); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	nginx := stackStatusFor(t, app.StackStatus(), stacks.ServiceNginx)
	if !nginx.Running {
		// Diagnose: who owns the port and what does the walk see?
		owner := stacks.ListeningPID(port)
		ownerPath, ownerErr := sysproc.ProcessExecutablePath(owner)
		parent, parentErr := sysproc.ParentPID(owner)
		parentPath, parentPathErr := sysproc.ProcessExecutablePath(parent)
		t.Logf("port=%d listenerPID=%d listenerPath=%q (%v)", port, owner, ownerPath, ownerErr)
		t.Logf("parent=%d parentPath=%q (parentErr=%v, pathErr=%v)", parent, parentPath, parentErr, parentPathErr)
		t.Logf("configured=%q exePath=%q os.Args[0]=%q", exePath, exePath, os.Args[0])
		t.Logf("configured matches listener: %v", strings.EqualFold(filepath.Clean(ownerPath), filepath.Clean(exePath)))
		t.Fatalf("nginx was not adopted from the child listener: %+v", nginx)
	}
	if child.Process != nil && nginx.PID == child.Process.Pid {
		t.Fatalf("adopted the listening child (pid %d); the owner walk should land on this process", nginx.PID)
	}
	if nginx.PID != os.Getpid() {
		t.Fatalf("adopted pid %d, want this test process (%d) as the service owner", nginx.PID, os.Getpid())
	}
}
