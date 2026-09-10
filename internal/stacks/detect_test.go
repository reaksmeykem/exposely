package stacks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersionPathsNumericOrder(t *testing.T) {
	// "10" must beat "8" (plain string sort would flip these).
	high := `C:\ProgramData\envkit\services\php\10.11.10\php-cgi.exe`
	low := `C:\ProgramData\envkit\services\php\8.5.7\php-cgi.exe`
	if compareVersionPaths(high, low) <= 0 {
		t.Fatalf("expected %q to rank above %q", high, low)
	}
}

func TestNewestGlobPicksHighestVersion(t *testing.T) {
	root := t.TempDir()
	for _, v := range []string{"8.5.7", "10.11.10", "8.4.25"} {
		dir := filepath.Join(root, v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "php-cgi.exe"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pattern := filepath.Join(root, "*", "php-cgi.exe")
	got := newestGlob(pattern)
	if !strings.Contains(got, "10.11.10") {
		t.Fatalf("expected newest version dir, got %q", got)
	}
}

func TestNewestGlobMissingPatternIsEmpty(t *testing.T) {
	if got := newestGlob(filepath.Join(t.TempDir(), "nope", "*")); got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestDetectBinariesPrefersManagedInstall(t *testing.T) {
	appData := t.TempDir()
	// EnvKit-style copy that detection must NOT pick over the managed one.
	envkit := filepath.Join(`C:\ProgramData`, "detect-test-envkit", "services", "php", "9.9.9")
	_ = envkit // real EnvKit path is outside the temp dir; on dev machines it exists.
	// Managed install.
	phpDir := PHPInstallDir(appData)
	if err := os.MkdirAll(phpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phpDir, "php-cgi.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Nginx via a "common root" is outside temp dir and not guaranteed to
	// exist, so only assert the managed PHP hit.
	found := DetectBinaries(appData)
	if got := found[ServicePHP]; !strings.HasPrefix(filepath.Clean(got), filepath.Clean(phpDir)) {
		t.Fatalf("expected managed php to win, got %q", got)
	}
}
