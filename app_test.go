package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

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

func TestSelectDesktopUpdateAssetFallsBackToPortableExe(t *testing.T) {
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
