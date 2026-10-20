package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/reaksmeykem/exposely/internal/models"
)

func newTerminalTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("AppData", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	app, err := NewApp()
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	app.ctx = nil
	return app
}

func saveTerminalTestProject(t *testing.T, app *App, projectPath string) string {
	t.Helper()
	state, err := app.SaveProject(models.ProjectPreset{
		DisplayName: "terminal test",
		ProjectPath: projectPath,
		ShareMode:   models.ShareModeQuick,
		LocalHost:   "terminal.test",
	})
	if err != nil {
		t.Fatalf("save project: %v", err)
	}
	if len(state.Settings.Projects) == 0 {
		t.Fatal("expected the saved project in the returned state")
	}
	return state.Settings.Projects[0].ID
}

func TestTerminalStartUsesProjectFolder(t *testing.T) {
	app := newTerminalTestApp(t)
	projectDir := t.TempDir()
	projectID := saveTerminalTestProject(t, app, projectDir)

	status, err := app.TerminalStart(projectID, "echo hello")
	if err != nil {
		t.Fatalf("terminal start: %v", err)
	}
	if !status.Running || status.PID <= 0 {
		t.Fatalf("expected a running command, got %+v", status)
	}
	if status.Dir != projectDir {
		t.Fatalf("expected dir %q, got %q", projectDir, status.Dir)
	}
	if status.ProjectID != projectID {
		t.Fatalf("expected project %q, got %q", projectID, status.ProjectID)
	}

	deadline := time.Now().Add(15 * time.Second)
	for app.TerminalStatus().Running {
		if time.Now().After(deadline) {
			t.Fatal("terminal command did not finish")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if _, err := app.TerminalStart(projectID, "   "); err == nil {
		t.Fatal("expected an error for an empty command")
	}
}

func TestTerminalStartRejectsMissingFolder(t *testing.T) {
	app := newTerminalTestApp(t)
	projectID := saveTerminalTestProject(t, app, filepath.Join(t.TempDir(), "missing"))

	if _, err := app.TerminalStart(projectID, "echo hello"); err == nil {
		t.Fatal("expected an error for a missing folder")
	}
}
