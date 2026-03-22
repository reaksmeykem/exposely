package main

import (
	"testing"

	"cloudflaretunnelmanager/internal/models"
)

func TestNormalizeServiceURLStripsPath(t *testing.T) {
	got, ok, err := normalizeServiceURL("http://127.0.0.1:5500/index.html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected URL input to be detected")
	}
	if got != "http://127.0.0.1:5500" {
		t.Fatalf("unexpected normalized URL: got %q", got)
	}
}

func TestNormalizeServiceURLLeavesFolderPathUntouched(t *testing.T) {
	_, ok, err := normalizeServiceURL(`D:\2025\Codes\HTML`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected local folder path to not be treated as URL")
	}
}

func TestNormalizeServiceURLRejectsUnsupportedScheme(t *testing.T) {
	_, ok, err := normalizeServiceURL("ftp://127.0.0.1:5500")
	if !ok {
		t.Fatalf("expected input with scheme to be treated as URL")
	}
	if err == nil {
		t.Fatalf("expected unsupported scheme to fail")
	}
}

func TestDetectCommandPorts(t *testing.T) {
	got := detectCommandPorts("npm run dev -- --port 4173")
	if len(got) != 1 || got[0] != 4173 {
		t.Fatalf("unexpected ports: %v", got)
	}
}

func TestValidateProjectSourceForAuto(t *testing.T) {
	err := validateProjectSource(models.ProjectPreset{
		ShareMode: models.ShareModeAuto,
		LocalURL:  "http://127.0.0.1:5500",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
