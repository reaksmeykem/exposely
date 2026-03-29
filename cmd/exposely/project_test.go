package main

import (
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
		LocalURL:     "http://127.0.0.1:5500",
		ProjectPath:  `D:\site`,
		StartCommand: "npm run dev",
		Subdomain:    "my-app",
	}
	values := &projectFlagValues{
		name:        "Updated",
		localHost:   "",
		localURL:    "",
		projectPath: `D:\new-site`,
		start:       "",
		subdomain:   "New-App",
	}
	visited := map[string]bool{
		"name":      true,
		"host":      true,
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
