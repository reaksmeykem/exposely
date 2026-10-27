package stacks

import (
	"os"
	"strings"
	"testing"
)

func TestSetLastErrorSurfacesWhyAServiceIsBlocked(t *testing.T) {
	m := NewManager()
	service := ServiceMySQL

	status := m.SetLastError(service, "mysql cannot start: port 3306 is already in use by mysqld (PID 4242)")
	if status.Running {
		t.Fatalf("a service with only an error recorded must not report running: %+v", status)
	}
	if !strings.Contains(status.LastError, "3306") {
		t.Fatalf("SetLastError lost the message: %+v", status)
	}

	// The message must survive a later read, which is what the stack panel
	// shows in place of the pid when a service is stopped.
	if got := m.Status(service).LastError; !strings.Contains(got, "already in use") {
		t.Fatalf("Status() = %q, want the recorded reason", got)
	}
	if statuses := m.AllStatus(); len(statuses) != 3 {
		t.Fatalf("AllStatus returned %d entries, want 3", len(statuses))
	}
}

func TestSetLastErrorLeavesARunningServiceAlone(t *testing.T) {
	m := NewManager()
	service := ServiceNginx

	// Adopt this test process so the manager tracks something that is
	// genuinely alive; SetLastError must not overwrite a live child's state.
	if adopted := m.Adopt(service, os.Getpid()); !adopted.Running {
		t.Fatalf("Adopt(os.Getpid()) did not report running: %+v", adopted)
	}

	got := m.SetLastError(service, "this must not appear while the process is alive")
	if !got.Running {
		t.Fatalf("SetLastError stopped reporting a live service as running: %+v", got)
	}
	if got.LastError != "" {
		t.Fatalf("SetLastError wrote %q onto a running service", got.LastError)
	}
}
