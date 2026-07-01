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
	// DetectEnvKit resolves the URL dynamically from the loopback probe,
	// but it must always be one of the two well-known loopback URLs. It
	// falls back to LoopbackHTTPSOriginURL when nothing is reachable so
	// error messages downstream still have a valid URL to reference.
	if info.SuggestedOriginURL != LoopbackHTTPSOriginURL && info.SuggestedOriginURL != LoopbackHTTPOriginURL {
		t.Fatalf("expected SuggestedOriginURL to be a canonical loopback URL, got %q", info.SuggestedOriginURL)
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

func TestDetectAlwaysPopulatesSuggestedOrigin(t *testing.T) {
	// Detect() must always set SuggestedOriginURL so callers do not have
	// to special-case the "no local stack" path. The URL must be one of
	// the two canonical loopback endpoints.
	info := Detect()
	if info.SuggestedOriginURL != LoopbackHTTPSOriginURL && info.SuggestedOriginURL != LoopbackHTTPOriginURL {
		t.Fatalf("expected SuggestedOriginURL to be a canonical loopback URL, got %q", info.SuggestedOriginURL)
	}
	if info.Detected {
		// Detected=true → Kind/Name must be set so the UI has something
		// to render and downstream code can branch on Kind.
		if info.Kind == KindNone {
			t.Fatalf("Detected=true but Kind is empty: %+v", info)
		}
		if strings.TrimSpace(info.Name) == "" {
			t.Fatalf("Detected=true but Name is empty: %+v", info)
		}
	} else {
		// Detected=false → no install metadata leaks through.
		if info.Kind != KindNone || info.Name != "" || info.InstallPath != "" || info.Version != "" {
			t.Fatalf("Detected=false but extra fields populated: %+v", info)
		}
	}
}

func TestIsEnvKitOnlyTrueForEnvKitKind(t *testing.T) {
	cases := []struct {
		info Info
		want bool
	}{
		{Info{Detected: true, Kind: KindEnvKit}, true},
		{Info{Detected: true, Kind: KindHerd}, false},
		{Info{Detected: true, Kind: KindHTTPSLoopback}, false},
		{Info{Detected: true, Kind: KindHTTPLoopback}, false},
		{Info{Detected: false, Kind: KindEnvKit}, false},
		{Info{}, false},
	}
	for _, tc := range cases {
		if got := tc.info.IsEnvKit(); got != tc.want {
			t.Fatalf("IsEnvKit() for %+v = %v, want %v", tc.info, got, tc.want)
		}
	}
}

// TestFinaliseOriginPrefersReachablePortMatchingStackDefault covers the
// happy path: an HTTPS-preferring stack (EnvKit/Herd/Valet) sees HTTPS
// reachable → pick https://127.0.0.1:443.
func TestFinaliseOriginPrefersReachablePortMatchingStackDefault(t *testing.T) {
	got := finaliseOrigin(
		Info{Detected: true, Kind: KindEnvKit},
		protocolHTTPS,
		loopbackResult{httpsUp: true, httpUp: true},
	)
	if got.SuggestedOriginURL != LoopbackHTTPSOriginURL {
		t.Fatalf("expected HTTPS origin, got %q", got.SuggestedOriginURL)
	}
}

// TestFinaliseOriginFallsBackToOtherPortWhenPreferredDown covers the
// core "normal Laravel" fix: an HTTPS-preferring stack sees only HTTP
// running → pick http://127.0.0.1:80 instead of the stale HTTPS URL.
func TestFinaliseOriginFallsBackToOtherPortWhenPreferredDown(t *testing.T) {
	got := finaliseOrigin(
		Info{Detected: true, Kind: KindEnvKit},
		protocolHTTPS,
		loopbackResult{httpsUp: false, httpUp: true},
	)
	if got.SuggestedOriginURL != LoopbackHTTPOriginURL {
		t.Fatalf("expected HTTP origin fallback, got %q", got.SuggestedOriginURL)
	}
}

// TestFinaliseOriginLaragonPrefersHTTP asserts that Laragon (an HTTP-first
// stack) picks HTTP:80 when both ports are up, not HTTPS:443 — this is
// the Laragon-specific regression fix.
func TestFinaliseOriginLaragonPrefersHTTP(t *testing.T) {
	got := finaliseOrigin(
		Info{Detected: true, Kind: KindLaragon},
		protocolHTTP,
		loopbackResult{httpsUp: true, httpUp: true},
	)
	if got.SuggestedOriginURL != LoopbackHTTPOriginURL {
		t.Fatalf("expected Laragon to prefer HTTP, got %q", got.SuggestedOriginURL)
	}
}

// TestFinaliseOriginLaragonFallsBackToHTTPS covers the rare Laragon-with-SSL
// setup: HTTP is not running (unusual for Laragon) but HTTPS is → use HTTPS.
func TestFinaliseOriginLaragonFallsBackToHTTPS(t *testing.T) {
	got := finaliseOrigin(
		Info{Detected: true, Kind: KindLaragon},
		protocolHTTP,
		loopbackResult{httpsUp: true, httpUp: false},
	)
	if got.SuggestedOriginURL != LoopbackHTTPSOriginURL {
		t.Fatalf("expected HTTPS fallback for Laragon-with-SSL, got %q", got.SuggestedOriginURL)
	}
}

// TestFinaliseOriginNoPortsReachableKeepsStackDefault documents the
// "user hasn't started their stack yet" path: neither port answers, so
// we surface the stack's own default URL. The downstream reachability
// check produces a stack-aware "start your local web server" error.
func TestFinaliseOriginNoPortsReachableKeepsStackDefault(t *testing.T) {
	cases := []struct {
		name      string
		preferred protocol
		want      string
	}{
		{"envkit prefers https default", protocolHTTPS, LoopbackHTTPSOriginURL},
		{"laragon prefers http default", protocolHTTP, LoopbackHTTPOriginURL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := finaliseOrigin(
				Info{Detected: true},
				tc.preferred,
				loopbackResult{httpsUp: false, httpUp: false},
			)
			if got.SuggestedOriginURL != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got.SuggestedOriginURL)
			}
		})
	}
}

// TestLoopbackConstantsAreDistinct guards against future refactors that
// might accidentally collapse the two loopback URLs into one — the whole
// point of the dynamic-origin fix is that they are handled separately.
func TestLoopbackConstantsAreDistinct(t *testing.T) {
	if LoopbackHTTPOriginURL == LoopbackHTTPSOriginURL {
		t.Fatalf("HTTP and HTTPS loopback URLs must not be equal, both = %q", LoopbackHTTPOriginURL)
	}
	if !strings.HasPrefix(LoopbackHTTPOriginURL, "http://") {
		t.Fatalf("LoopbackHTTPOriginURL should use http:// scheme, got %q", LoopbackHTTPOriginURL)
	}
	if !strings.HasPrefix(LoopbackHTTPSOriginURL, "https://") {
		t.Fatalf("LoopbackHTTPSOriginURL should use https:// scheme, got %q", LoopbackHTTPSOriginURL)
	}
	// EnvKitOriginURL is a legacy alias — must still resolve to the
	// canonical HTTPS URL so callers that reference it keep working.
	if EnvKitOriginURL != LoopbackHTTPSOriginURL {
		t.Fatalf("EnvKitOriginURL legacy alias drifted: got %q, want %q", EnvKitOriginURL, LoopbackHTTPSOriginURL)
	}
}
