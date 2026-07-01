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

// TestResolveCLIProjectOriginServiceURLUsesHTTPWhenStackPrefersHTTP asserts
// that when the localstack layer decides the SuggestedOriginURL is the
// HTTP loopback (Laragon default, or a plain Nginx / Apache install
// serving on port 80), the CLI honours that instead of upgrading to
// HTTPS. This is the CLI-side companion to the "normal Laravel" fix in
// internal/localstack.
func TestResolveCLIProjectOriginServiceURLUsesHTTPWhenStackPrefersHTTP(t *testing.T) {
	got, err := resolveCLIProjectOriginServiceURL(
		models.ProjectPreset{},
		"http://127.0.0.1:80",
		localstack.Info{
			Detected:           true,
			Kind:               localstack.KindLaragon,
			Name:               "Laragon",
			SuggestedOriginURL: localstack.LoopbackHTTPOriginURL,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != localstack.LoopbackHTTPOriginURL {
		t.Fatalf("expected HTTP loopback origin, got %q", got)
	}
}

// TestCLIOriginServerNameSkippedForHTTPUpstream guards a subtle
// interaction with the dynamic-origin fix: when the localstack layer
// picks http://127.0.0.1:80 (Laragon / plain Nginx), cloudflared must
// not be handed an OriginServerName override — SNI is meaningless for
// plain HTTP and would just confuse downstream tooling.
func TestCLIOriginServerNameSkippedForHTTPUpstream(t *testing.T) {
	got := cliOriginServerNameForLoopbackHTTPS(localstack.LoopbackHTTPOriginURL, "my-app.test")
	if got != "" {
		t.Fatalf("expected empty SNI override for HTTP upstream, got %q", got)
	}

	// Sanity check the HTTPS branch still wires the host header through.
	got = cliOriginServerNameForLoopbackHTTPS(localstack.LoopbackHTTPSOriginURL, "my-app.test")
	if got != "my-app.test" {
		t.Fatalf("expected SNI override for HTTPS upstream, got %q", got)
	}
}
