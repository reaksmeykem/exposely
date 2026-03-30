package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reaksmeykem/exposely/internal/models"
)

func TestResolveProjectReferenceUsesExplicitValue(t *testing.T) {
	got := resolveProjectReference("project-123", []string{"HR", "System"})
	if got != "project-123" {
		t.Fatalf("expected explicit project reference, got %q", got)
	}
}

func TestResolveProjectReferenceJoinsPositionalArgs(t *testing.T) {
	got := resolveProjectReference("", []string{"HR", "System"})
	if got != "HR System" {
		t.Fatalf("expected joined positional reference, got %q", got)
	}
}

func TestApplyProjectFlagValuesAllowsClearingFields(t *testing.T) {
	project := models.ProjectPreset{
		DisplayName:  "Existing",
		LocalHost:    "old.test",
		OriginURL:    "http://127.0.0.1:80",
		LocalURL:     "http://127.0.0.1:5500",
		ProjectPath:  `D:\site`,
		StartCommand: "npm run dev",
		Subdomain:    "my-app",
	}
	values := &projectFlagValues{
		name:        "Updated",
		localHost:   "",
		originURL:   "http://127.0.0.1:8080",
		localURL:    "",
		projectPath: `D:\new-site`,
		start:       "",
		subdomain:   "New-App",
	}
	visited := map[string]bool{
		"name":      true,
		"host":      true,
		"origin":    true,
		"url":       true,
		"path":      true,
		"start":     true,
		"subdomain": true,
	}

	applyProjectFlagValues(&project, values, visited)

	if project.DisplayName != "Updated" {
		t.Fatalf("expected updated display name, got %q", project.DisplayName)
	}
	if project.LocalHost != "" {
		t.Fatalf("expected local host to be cleared, got %q", project.LocalHost)
	}
	if project.OriginURL != "http://127.0.0.1:8080" {
		t.Fatalf("expected updated origin URL, got %q", project.OriginURL)
	}
	if project.LocalURL != "" {
		t.Fatalf("expected local URL to be cleared, got %q", project.LocalURL)
	}
	if project.ProjectPath != `D:\new-site` {
		t.Fatalf("expected updated project path, got %q", project.ProjectPath)
	}
	if project.StartCommand != "" {
		t.Fatalf("expected start command to be cleared, got %q", project.StartCommand)
	}
	if project.Subdomain != "new-app" {
		t.Fatalf("expected lowercase subdomain, got %q", project.Subdomain)
	}
}

func TestDefaultProjectNameFromPathUsesFolderName(t *testing.T) {
	got := defaultProjectNameFromPath(`D:\code\hr-system`)
	if got != "hr-system" {
		t.Fatalf("expected folder name, got %q", got)
	}
}

func TestApplyInitDefaultsUsesCurrentWorkingDirectory(t *testing.T) {
	runner := &cliRunner{workDir: `D:\code\hr-system`}
	project := runner.applyInitDefaults(models.ProjectPreset{})

	if project.ProjectPath != `D:\code\hr-system` {
		t.Fatalf("expected cwd as project path, got %q", project.ProjectPath)
	}
	if project.DisplayName != "hr-system" {
		t.Fatalf("expected folder name as display name, got %q", project.DisplayName)
	}
}

func TestApplyInitDefaultsPreservesExplicitValues(t *testing.T) {
	runner := &cliRunner{workDir: `D:\code\hr-system`}
	project := runner.applyInitDefaults(models.ProjectPreset{
		DisplayName: "HR System",
		ProjectPath: `D:\code\custom-path`,
	})

	if project.ProjectPath != `D:\code\custom-path` {
		t.Fatalf("expected explicit project path to be preserved, got %q", project.ProjectPath)
	}
	if project.DisplayName != "HR System" {
		t.Fatalf("expected explicit display name to be preserved, got %q", project.DisplayName)
	}
}

func TestApplyShareDefaultsUsesCwdForStartCommand(t *testing.T) {
	runner := &cliRunner{workDir: `D:\code\frontend`}
	project, mode, err := runner.applyShareDefaults(models.ProjectPreset{
		StartCommand: "npm run dev -- --port 5173",
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.ProjectPath != `D:\code\frontend` {
		t.Fatalf("expected cwd path, got %q", project.ProjectPath)
	}
	if project.DisplayName != "frontend" {
		t.Fatalf("expected cwd folder name, got %q", project.DisplayName)
	}
	if mode != "" {
		t.Fatalf("expected mode to stay empty for inference, got %q", mode)
	}
}

func TestApplyShareDefaultsDetectsHtmlFolderFromCwd(t *testing.T) {
	workDir := t.TempDir()
	indexPath := filepath.Join(workDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	runner := &cliRunner{workDir: workDir}
	project, mode, err := runner.applyShareDefaults(models.ProjectPreset{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.ProjectPath != workDir {
		t.Fatalf("expected cwd path, got %q", project.ProjectPath)
	}
	if mode != string(models.ShareModeHostHTML) {
		t.Fatalf("expected host-html mode, got %q", mode)
	}
	if project.DisplayName != filepath.Base(workDir) {
		t.Fatalf("expected folder name, got %q", project.DisplayName)
	}
}

func TestInferCLIHostFromProjectPath(t *testing.T) {
	got := inferCLIHostFromProjectPath(`D:\code\HR System`)
	if got != "hr-system.test" {
		t.Fatalf("unexpected inferred host: %q", got)
	}
}

func TestDetectCLILaravelProjectDir(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "artisan"), []byte("artisan"), 0o644); err != nil {
		t.Fatalf("failed to write artisan file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "public"), 0o755); err != nil {
		t.Fatalf("failed to create public directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "public", "index.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatalf("failed to write Laravel front controller: %v", err)
	}

	if !detectCLILaravelProjectDir(workDir) {
		t.Fatalf("expected Laravel project to be detected")
	}
}
