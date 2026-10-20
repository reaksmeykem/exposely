package terminal

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStripANSI(t *testing.T) {
	input := "\x1b[32mready\x1b[0m in \x1b[1m350\x1b[0m ms\x1b]0;title\x07"
	if got := StripANSI(input); got != "ready in 350 ms" {
		t.Fatalf("unexpected stripped output: %q", got)
	}
}

func TestStartStreamsOutputAndExit(t *testing.T) {
	lines := make(chan Event, 16)
	exits := make(chan Exit, 1)
	runner := &Runner{}

	pid, err := runner.Start(t.TempDir(), "echo hello", func(event Event) {
		lines <- event
	}, func(exit Exit) {
		exits <- exit
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("expected a pid, got %d", pid)
	}

	deadline := time.After(10 * time.Second)
	var output []string
	for {
		select {
		case event := <-lines:
			output = append(output, event.Line)
			continue
		case exit := <-exits:
			if exit.Err != nil {
				t.Fatalf("command failed: %v", exit.Err)
			}
			if exit.Code != 0 {
				t.Fatalf("expected exit 0, got %d", exit.Code)
			}
		case <-deadline:
			t.Fatal("timed out waiting for command output")
		}
		break
	}

	joined := strings.Join(output, "\n")
	if !strings.Contains(joined, "hello") {
		t.Fatalf("expected output to contain hello, got %q", joined)
	}
	if status := runner.Status(); status.Running {
		t.Fatalf("runner should be idle after exit, got %+v", status)
	}
}

func TestStopKillsRunningCommand(t *testing.T) {
	exits := make(chan Exit, 1)
	runner := &Runner{}

	command := "sleep 30"
	if runtime.GOOS == "windows" {
		command = "ping -n 30 127.0.0.1"
	}
	if _, err := runner.Start(t.TempDir(), command, nil, func(exit Exit) {
		exits <- exit
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !runner.Status().Running {
		t.Fatal("expected the command to be running")
	}
	if err := runner.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	select {
	case <-exits:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for stopped command to exit")
	}
	if runner.Status().Running {
		t.Fatal("runner should be idle after stop")
	}
}

func TestSendInputRequiresRunningCommand(t *testing.T) {
	runner := &Runner{}
	if err := runner.SendInput("y"); err == nil {
		t.Fatal("expected an error when nothing is running")
	}
}
