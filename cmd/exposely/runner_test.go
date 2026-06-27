package main

import (
	"testing"

	"github.com/reaksmeykem/exposely/internal/localstack"
	"github.com/reaksmeykem/exposely/internal/models"
)

func TestResolveCLIProjectOriginServiceURLPrefersProjectOverride(t *testing.T) {
	got, err := resolveCLIProjectOriginServiceURL(
		models.ProjectPreset{OriginURL: "http://127.0.0.1:8080/app"},
		"http://127.0.0.1:80",
		localstack.Info{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("expected project origin to win, got %q", got)
	}
}

func TestResolveCLIProjectOriginServiceURLUsesEnvKitWhenDefaultBuiltIn(t *testing.T) {
	got, err := resolveCLIProjectOriginServiceURL(
		models.ProjectPreset{},
		"http://127.0.0.1:80",
		localstack.Info{Detected: true, SuggestedOriginURL: localstack.EnvKitOriginURL},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != localstack.EnvKitOriginURL {
		t.Fatalf("expected EnvKit origin %q, got %q", localstack.EnvKitOriginURL, got)
	}
}

func TestResolveCLIProjectOriginServiceURLRespectsCustomDefault(t *testing.T) {
	got, err := resolveCLIProjectOriginServiceURL(
		models.ProjectPreset{},
		"http://127.0.0.1:9000",
		localstack.Info{Detected: true, SuggestedOriginURL: localstack.EnvKitOriginURL},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://127.0.0.1:9000" {
		t.Fatalf("expected custom default to be respected, got %q", got)
	}
}

func TestResolveCLIProjectOriginServiceURLIgnoresEnvKitWhenNotDetected(t *testing.T) {
	got, err := resolveCLIProjectOriginServiceURL(
		models.ProjectPreset{},
		"http://127.0.0.1:80",
		localstack.Info{Detected: false, SuggestedOriginURL: localstack.EnvKitOriginURL},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://127.0.0.1:80" {
		t.Fatalf("expected built-in default to remain, got %q", got)
	}
}
