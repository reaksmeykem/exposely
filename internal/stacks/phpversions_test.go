package stacks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledPHPVersionsFindsLegacyAndVersioned(t *testing.T) {
	appData := t.TempDir()

	// Versioned install 8.3.33.
	vDir := PHPVersionDir(appData, "8.3.33")
	if err := os.MkdirAll(vDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"php-cgi.exe", "php.exe"} {
		if err := os.WriteFile(filepath.Join(vDir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Legacy root install.
	root := PHPInstallDir(appData)
	for _, f := range []string{"php-cgi.exe", "php.exe"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	installed := InstalledPHPVersions(appData)
	if len(installed) != 2 {
		t.Fatalf("expected 2 installs (versioned + legacy), got %d: %+v", len(installed), installed)
	}
	var sawVersioned, sawLegacy bool
	for _, inst := range installed {
		switch {
		case inst.Version == "8.3.33" && !inst.Legacy:
			sawVersioned = true
		case inst.Legacy:
			sawLegacy = true
		}
	}
	if !sawVersioned || !sawLegacy {
		t.Fatalf("expected one versioned and one legacy install, got %+v", installed)
	}
}

func TestInstallPHPVersionRejectsBadVersion(t *testing.T) {
	if _, _, err := InstallPHPVersion(t.TempDir(), "../evil"); err == nil {
		t.Fatal("expected error for non-numeric version")
	}
	if _, _, err := InstallPHPVersion(t.TempDir(), ""); err == nil {
		t.Fatal("expected error for empty version")
	}
}

func TestPHPVersionDirStaysUnderVersionsRoot(t *testing.T) {
	appData := t.TempDir()
	if dir := PHPVersionDir(appData, "../../evil"); !filepath.IsAbs(dir) || filepath.Dir(dir) != PHPVersionsRoot(appData) && filepath.Base(dir) != "" {
		// sanitizeVersion("") yields an empty component; the joined path
		// must never contain ".."
		if filepath.IsAbs(dir) && (filepath.Base(dir) == ".." || filepath.Base(dir) == "evil") {
			t.Fatalf("version dir escaped versions root: %q", dir)
		}
	}
	got := PHPVersionDir(appData, "8.3.33")
	if want := filepath.Join(PHPVersionsRoot(appData), "8.3.33"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestActivePHPDirForPrefersVersionedThenLegacy(t *testing.T) {
	appData := t.TempDir()

	// Nothing installed → falls back to default version dir path.
	if got := ActivePHPDirFor(appData, ""); got != PHPVersionDir(appData, PHPVersion) {
		t.Fatalf("empty state: got %q", got)
	}

	// Legacy root present → legacy wins when no version requested.
	root := PHPInstallDir(appData)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"php-cgi.exe", "php.exe"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := ActivePHPDirFor(appData, ""); got != root {
		t.Fatalf("legacy state: got %q want %q", got, root)
	}
}

func TestLooksLikeVersion(t *testing.T) {
	good := []string{"8.4", "8.4.25", "10.11.10"}
	bad := []string{"", "8", "8.x", "../evil", "8.4.25.1", "abc"}
	for _, v := range good {
		if !looksLikeVersion(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	for _, v := range bad {
		if looksLikeVersion(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}
