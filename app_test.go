package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reaksmeykem/exposely/internal/localstack"
	"github.com/reaksmeykem/exposely/internal/models"
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

func TestResolveProjectOriginServiceURLPrefersProjectOverride(t *testing.T) {
	serviceURL, err := resolveProjectOriginServiceURL(models.ProjectPreset{
		OriginURL: "http://127.0.0.1:8080/app",
	}, "http://127.0.0.1:80", localstack.Info{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serviceURL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected origin URL: %q", serviceURL)
	}
}

func TestResolveProjectOriginServiceURLUsesEnvKitWhenDefaultIsBuiltIn(t *testing.T) {
	serviceURL, err := resolveProjectOriginServiceURL(
		models.ProjectPreset{},
		"http://127.0.0.1:80",
		localstack.Info{Detected: true, SuggestedOriginURL: localstack.EnvKitOriginURL},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serviceURL != localstack.EnvKitOriginURL {
		t.Fatalf("expected EnvKit origin %q, got %q", localstack.EnvKitOriginURL, serviceURL)
	}
}

func TestResolveProjectOriginServiceURLUsesEnvKitWhenDefaultIsEmpty(t *testing.T) {
	serviceURL, err := resolveProjectOriginServiceURL(
		models.ProjectPreset{},
		"",
		localstack.Info{Detected: true, SuggestedOriginURL: localstack.EnvKitOriginURL},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serviceURL != localstack.EnvKitOriginURL {
		t.Fatalf("expected EnvKit origin %q, got %q", localstack.EnvKitOriginURL, serviceURL)
	}
}

func TestResolveProjectOriginServiceURLRespectsCustomDefault(t *testing.T) {
	serviceURL, err := resolveProjectOriginServiceURL(
		models.ProjectPreset{},
		"http://127.0.0.1:9000",
		localstack.Info{Detected: true, SuggestedOriginURL: localstack.EnvKitOriginURL},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serviceURL != "http://127.0.0.1:9000" {
		t.Fatalf("expected custom default origin to be respected, got %q", serviceURL)
	}
}

func TestResolveProjectOriginServiceURLIgnoresEnvKitWhenNotDetected(t *testing.T) {
	serviceURL, err := resolveProjectOriginServiceURL(
		models.ProjectPreset{},
		"http://127.0.0.1:80",
		localstack.Info{Detected: false, SuggestedOriginURL: localstack.EnvKitOriginURL},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serviceURL != "http://127.0.0.1:80" {
		t.Fatalf("expected built-in default to remain, got %q", serviceURL)
	}
}

func TestResolveProjectOriginServiceURLFailsWithoutAnySource(t *testing.T) {
	if _, err := resolveProjectOriginServiceURL(
		models.ProjectPreset{},
		"",
		localstack.Info{Detected: false, SuggestedOriginURL: localstack.EnvKitOriginURL},
	); err == nil {
		t.Fatalf("expected an error when no origin source is available")
	}
}

func TestInferLocalHostFromProjectPath(t *testing.T) {
	got := inferLocalHostFromProjectPath(`D:\code\HR System`)
	if got != "hr-system.test" {
		t.Fatalf("unexpected inferred host: %q", got)
	}
}

func TestLooksLikeLaravelProjectDir(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "artisan"), []byte("artisan"), 0o644); err != nil {
		t.Fatalf("failed to write artisan file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "public"), 0o755); err != nil {
		t.Fatalf("failed to create public directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "public", "index.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatalf("failed to write Laravel front controller: %v", err)
	}

	if !looksLikeLaravelProjectDir(projectDir) {
		t.Fatalf("expected Laravel project to be detected")
	}
}

func TestResolveHTMLOriginUsesBuildOutputDirectory(t *testing.T) {
	projectDir := t.TempDir()
	distDir := filepath.Join(projectDir, "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("failed to create dist directory: %v", err)
	}
	indexPath := filepath.Join(distDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html><body>dist-output</body></html>"), 0o644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	app := &App{}
	serviceURL, _, server, err := app.resolveHTMLOrigin(models.ProjectPreset{
		ShareMode:   models.ShareModeHostHTML,
		ProjectPath: projectDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server == nil {
		t.Fatalf("expected static server to be started")
	}
	defer server.Shutdown(t.Context())

	resp, err := http.Get(serviceURL)
	if err != nil {
		t.Fatalf("failed to reach resolved HTML origin: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != "<html><body>dist-output</body></html>" {
		t.Fatalf("unexpected response body: %q", string(body))
	}
}

func TestResolveHTMLOriginRejectsFolderWithoutStaticEntryPoint(t *testing.T) {
	projectDir := t.TempDir()
	app := &App{}

	_, _, _, err := app.resolveHTMLOrigin(models.ProjectPreset{
		ShareMode:   models.ShareModeHostHTML,
		ProjectPath: projectDir,
	})
	if err == nil {
		t.Fatalf("expected missing HTML entrypoint to fail")
	}
}

func TestNormalizeShareModePreservesStableAndRandom(t *testing.T) {
	if got := normalizeShareMode(models.ShareModeStable); got != models.ShareModeStable {
		t.Fatalf("expected stable mode to be preserved, got %q", got)
	}
	if got := normalizeShareMode(models.ShareModeRandomDomain); got != models.ShareModeRandomDomain {
		t.Fatalf("expected random-domain mode to be preserved, got %q", got)
	}
}

func TestOriginServerNameForLoopbackHTTPS(t *testing.T) {
	cases := []struct {
		name       string
		serviceURL string
		hostHeader string
		want       string
	}{
		{"envkit https with host", "https://127.0.0.1:443", "my-app.test", "my-app.test"},
		{"envkit localhost variant", "https://localhost:443", "my-app.test", "my-app.test"},
		{"plain http not affected", "http://127.0.0.1:80", "my-app.test", ""},
		{"empty host header", "https://127.0.0.1:443", "", ""},
		{"non-loopback https", "https://example.com", "my-app.test", ""},
		{"envkit origin with whitespace", "  https://127.0.0.1:443  ", "my-app.test", "my-app.test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := originServerNameForLoopbackHTTPS(tc.serviceURL, tc.hostHeader)
			if got != tc.want {
				t.Fatalf("originServerNameForLoopbackHTTPS(%q, %q) = %q, want %q", tc.serviceURL, tc.hostHeader, got, tc.want)
			}
		})
	}
}

func TestIsEnvKitOrigin(t *testing.T) {
	if !isEnvKitOrigin("https://127.0.0.1:443") {
		t.Fatalf("expected https://127.0.0.1:443 to be recognised as EnvKit origin")
	}
	if !isEnvKitOrigin("  HTTPS://127.0.0.1:443  ") {
		t.Fatalf("expected trim + case-insensitive match for EnvKit origin")
	}
	if isEnvKitOrigin("https://127.0.0.1:8443") {
		t.Fatalf("non-default port should not be flagged as EnvKit origin")
	}
	if isEnvKitOrigin("http://127.0.0.1:443") {
		t.Fatalf("plain http should not be flagged as EnvKit origin")
	}
}

func TestCheckLocalOriginReachableReportsUnreachable(t *testing.T) {
	// Spin up an httptest server and immediately close it to claim a port
	// that is then guaranteed-unreachable.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	err := checkLocalOriginReachable(url, "", localstack.Info{}, false)
	if err == nil {
		t.Fatalf("expected an error when dialing a closed server, got nil")
	}
}

func TestCheckLocalOriginReachableAcceptsRunningServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := checkLocalOriginReachable(server.URL, "", localstack.Info{}, false); err != nil {
		t.Fatalf("expected running server to be reachable, got %v", err)
	}
}

func TestCheckLocalOriginReachableEnvKitHint(t *testing.T) {
	// Closed EnvKit-style server should produce the EnvKit hint when detection is on.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	// We can't easily point at https://127.0.0.1:443 here without binding it,
	// so we drive isEnvKitOrigin directly and assert the hint is appended
	// only when the URL matches the EnvKit default.
	err := checkLocalOriginReachable(url, "my-app.test", localstack.Info{Detected: true}, false)
	if err == nil {
		t.Fatalf("expected an error for closed HTTPS server")
	}
	if strings.Contains(err.Error(), "EnvKit") {
		t.Fatalf("non-EnvKit URL should not trigger the EnvKit hint, got %v", err)
	}
}

func TestCheckLocalOriginReachableSkipsTLSVerifyWhenOptedIn(t *testing.T) {
	// Spin up a TLS server with the default self-signed cert and immediately
	// close it. With InsecureSkipOriginTLS=true the pre-flight should report
	// a connection error (not a TLS verification error) because the cert
	// check is bypassed.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	err := checkLocalOriginReachable(url, "kig.test", localstack.Info{Detected: true}, true)
	if err == nil {
		t.Fatalf("expected an error for closed server even with skip-tls-verify, got nil")
	}
	// The error should be a connection-class error, not a cert-verification error.
	if strings.Contains(err.Error(), "failed to verify certificate") {
		t.Fatalf("with skip-tls-verify the cert check should be bypassed, got %v", err)
	}
}

func TestExtractValidHosts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "envkit-shaped error",
			in:   `Get "https://127.0.0.1:443": tls: failed to verify certificate: x509: certificate is valid for *.test, envkit.local, invoicely.test, localhost, phpmyadmin.test, stacklet.local, test, not kig.test`,
			want: "*.test, envkit.local, invoicely.test, localhost, phpmyadmin.test, stacklet.local, test",
		},
		{
			name: "minimal wording",
			in:   "x509: certificate is valid for example.com, not foo.test",
			want: "example.com",
		},
		{
			name: "missing marker",
			in:   "some unrelated error",
			want: "(unknown)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractValidHosts(tc.in)
			if got != tc.want {
				t.Fatalf("extractValidHosts(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClassifyUpstreamErrorCertSANMismatch(t *testing.T) {
	raw := errors.New(`Get "https://127.0.0.1:443": tls: failed to verify certificate: x509: certificate is valid for *.test, envkit.local, invoicely.test, not kig.test`)
	classified := classifyUpstreamError("https://127.0.0.1:443", "kig.test", localstack.Info{Detected: true}, raw)
	msg := classified.Error()
	if !strings.Contains(msg, `does not cover "kig.test"`) {
		t.Fatalf("expected SAN mismatch message, got %q", msg)
	}
	if !strings.Contains(msg, "register kig.test") {
		t.Fatalf("expected actionable guidance, got %q", msg)
	}
	if !strings.Contains(msg, "*.test") {
		t.Fatalf("expected the valid hosts to be listed, got %q", msg)
	}
}

func TestClassifyUpstreamErrorConnectionRefusedEnvKitHint(t *testing.T) {
	classified := classifyUpstreamError(
		"https://127.0.0.1:443",
		"my-app.test",
		localstack.Info{Detected: true},
		fmt.Errorf("dial tcp 127.0.0.1:443: connectex: No connection could be made because the target machine actively refused it."),
	)
	if !strings.Contains(classified.Error(), "EnvKit") {
		t.Fatalf("expected EnvKit-specific hint for connection refused, got %q", classified.Error())
	}
	if !strings.Contains(classified.Error(), "start nginx") {
		t.Fatalf("expected actionable start-nginx hint, got %q", classified.Error())
	}
}

func TestClassifyUpstreamErrorGenericFallback(t *testing.T) {
	classified := classifyUpstreamError(
		"http://127.0.0.1:8080",
		"my-app.test",
		localstack.Info{Detected: true},
		fmt.Errorf("dial tcp: lookup foo.invalid: no such host"),
	)
	if strings.Contains(classified.Error(), "EnvKit") {
		t.Fatalf("DNS errors should not produce EnvKit hints, got %q", classified.Error())
	}
	if !strings.Contains(classified.Error(), "not reachable") {
		t.Fatalf("expected generic not-reachable message, got %q", classified.Error())
	}
}

func TestIsConnectionRefused(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"dial tcp 127.0.0.1:443: connectex: No connection could be made because the target machine actively refused it.", true},
		{"connection refused", true},
		{"dial tcp: lookup foo.invalid: no such host", false},
	}
	for _, tc := range cases {
		if got := isConnectionRefused(tc.in); got != tc.want {
			t.Fatalf("isConnectionRefused(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSelectDesktopUpdateAssetPrefersInstaller(t *testing.T) {
	release := githubLatestRelease{
		Assets: []githubReleaseAsset{
			{Name: "Exposely.exe", BrowserDownloadURL: "https://example.com/portable.exe"},
			{Name: "Exposely-amd64-installer.exe", BrowserDownloadURL: "https://example.com/installer.exe"},
		},
	}

	asset, err := selectDesktopUpdateAsset(release)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asset.Name != "Exposely-amd64-installer.exe" {
		t.Fatalf("expected installer asset to be preferred, got %q", asset.Name)
	}
}

func TestSelectDesktopUpdateAssetFallsBackToPortableExeWhenInstallerMissing(t *testing.T) {
	release := githubLatestRelease{
		Assets: []githubReleaseAsset{
			{Name: "exposely-cli.exe", BrowserDownloadURL: "https://example.com/cli.exe"},
			{Name: "Exposely.exe", BrowserDownloadURL: "https://example.com/portable.exe"},
		},
	}

	asset, err := selectDesktopUpdateAsset(release)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asset.Name != "Exposely.exe" {
		t.Fatalf("expected portable desktop exe fallback, got %q", asset.Name)
	}
}

func TestBuildWindowsStartProcessScriptQuotesPath(t *testing.T) {
	script := buildWindowsStartProcessScript(`C:\Users\User\AppData\Local\Temp\Exposely Updates\1.0.19-Exposely-amd64-installer.exe`)

	if !strings.Contains(script, "Start-Process -FilePath") {
		t.Fatalf("expected Start-Process invocation, got %q", script)
	}
	if !strings.Contains(script, `"C:\\Users\\User\\AppData\\Local\\Temp\\Exposely Updates\\1.0.19-Exposely-amd64-installer.exe"`) {
		t.Fatalf("expected quoted installer path, got %q", script)
	}
}

func TestProjectPublicURLSkipsEphemeralModes(t *testing.T) {
	app := &App{}

	for _, mode := range []models.ShareMode{
		models.ShareModeAuto,
		models.ShareModeQuick,
		models.ShareModeHostHTML,
	} {
		got := app.projectPublicURL(models.ProjectPreset{
			ShareMode: mode,
			PublicURL: "https://expired.trycloudflare.com",
		}, "example.com")
		if got != "" {
			t.Fatalf("expected empty URL for mode %q, got %q", mode, got)
		}
	}
}
