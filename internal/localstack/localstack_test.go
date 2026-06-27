package localstack

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsBuiltInDefaultServiceURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"http://127.0.0.1:80", true},
		{"HTTP://127.0.0.1:80", true},
		{"  http://127.0.0.1:80  ", true},
		{"http://127.0.0.1:8080", false},
		{"https://127.0.0.1:443", false},
	}
	for _, tc := range cases {
		got := IsBuiltInDefaultServiceURL(tc.in)
		if got != tc.want {
			t.Fatalf("IsBuiltInDefaultServiceURL(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestDetectEnvKitAlwaysPopulatesSuggestedOrigin(t *testing.T) {
	info := DetectEnvKit()
	if info.SuggestedOriginURL != EnvKitOriginURL {
		t.Fatalf("expected SuggestedOriginURL to be %q, got %q", EnvKitOriginURL, info.SuggestedOriginURL)
	}
	if info.Detected {
		if info.InstallPath == "" {
			t.Fatalf("Detected=true but InstallPath is empty")
		}
	} else {
		if info.InstallPath != "" || info.Version != "" {
			t.Fatalf("Detected=false but extra fields populated: %+v", info)
		}
	}
}

func TestIsEnvKitWindowsDirRecognisesMarkers(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only marker check")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "EnvKit.exe"), []byte("fake"), 0o644); err != nil {
		t.Fatalf("failed to write EnvKit.exe marker: %v", err)
	}
	if !isEnvKitWindowsDir(dir) {
		t.Fatalf("expected directory with EnvKit.exe to be recognised as EnvKit install")
	}
}

func TestIsEnvKitWindowsDirRejectsUnrelatedDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only marker check")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write notes.txt: %v", err)
	}
	if isEnvKitWindowsDir(dir) {
		t.Fatalf("expected directory without EnvKit markers to be rejected")
	}
}

func TestIsEnvKitWindowsDirRejectsMissingDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only marker check")
	}
	if isEnvKitWindowsDir(filepath.Join(t.TempDir(), "missing")) {
		t.Fatalf("expected missing directory to be rejected")
	}
}

func TestListRegistrySubkeysParsesRegQuery(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only parser test")
	}
	// `reg query HKLM\SOFTWARE` is harmless and always available; just ensure
	// the parser does not panic on a real registry listing. The exact set of
	// subkeys differs per machine so we only assert non-panic behaviour.
	keys := listRegistrySubkeys(`HKLM\SOFTWARE`)
	if keys == nil {
		// Tolerate failure on locked-down machines — the parser only matters
		// when we actually have something to parse.
		t.Skip("could not list HKLM\\SOFTWARE on this machine")
	}
	for _, key := range keys {
		if key == "" || strings.HasPrefix(key, "HKEY_") {
			t.Fatalf("unexpected key in parser output: %q", key)
		}
	}
}
