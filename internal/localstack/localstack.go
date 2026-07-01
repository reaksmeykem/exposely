// Package localstack detects well-known local development stacks (EnvKit,
// Laravel Herd, Laravel Valet, Laragon) and, as a final fallback, any
// HTTP/HTTPS service answering on 127.0.0.1. The goal is to let Exposely
// pick sensible defaults (origin URL, hints) for *.test / *.local style
// projects without the user hand-editing settings, regardless of which
// stack they happen to use.
//
// Behaviour:
//   - Detect() returns Info{Detected:true, Kind:"…"} whenever it finds
//     EnvKit, Herd, Valet, Laragon, or a live HTTP/HTTPS listener on
//     127.0.0.1:80 / 127.0.0.1:443.
//   - SuggestedOriginURL is picked dynamically per host: HTTPS on 443 is
//     preferred (that matches EnvKit / Herd / Valet defaults) but the
//     probe falls back to HTTP on 80 when only that is reachable. This
//     makes "normal Laravel" setups (Laragon, plain Nginx/Apache with
//     a .test vhost on port 80) work out of the box, not just
//     HTTPS-terminating stacks.
//   - DetectEnvKit() is kept for back-compat and only flags actual EnvKit
//     installs; callers that want stack-agnostic behaviour should use
//     Detect().
//
// The probe is intentionally fast (~1s total worst case): registry /
// disk lookups are local, and the loopback probes use short dial
// timeouts so they do not slow down app startup.
package localstack

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// Kind identifies which local stack was detected. Empty string means
// nothing was found.
type Kind string

const (
	// KindNone means no local stack was detected.
	KindNone Kind = ""
	// KindEnvKit is EnvKit (https://envkit.net).
	KindEnvKit Kind = "envkit"
	// KindHerd is Laravel Herd (https://herd.laravel.com).
	KindHerd Kind = "herd"
	// KindValet is Laravel Valet (macOS, https://laravel.com/docs/valet).
	KindValet Kind = "valet"
	// KindLaragon is Laragon (Windows, https://laragon.org).
	KindLaragon Kind = "laragon"
	// KindHTTPSLoopback means we did not identify a specific stack but a
	// TLS service is answering on 127.0.0.1:443. This is the generic
	// "something local is serving HTTPS" signal.
	KindHTTPSLoopback Kind = "https-loopback"
	// KindHTTPLoopback means we did not identify a specific stack but an
	// HTTP service is answering on 127.0.0.1:80. This covers plain
	// Apache / Nginx / IIS setups (typical "normal Laravel" installs
	// that do not terminate TLS locally).
	KindHTTPLoopback Kind = "http-loopback"
)

// protocol describes which upstream URL a specific stack prefers when it is
// installed. It is only consulted as a fallback when neither loopback port
// is reachable at detection time — the reachability probe wins whenever it
// can tell us what is actually running.
type protocol int

const (
	protocolHTTPS protocol = iota
	protocolHTTP
)

// LoopbackHTTPSOriginURL is the canonical HTTPS loopback upstream. Every
// TLS-terminating local stack (EnvKit, Herd, Valet, Caddy with autoHTTPS,
// Nginx with a local cert, …) points here on 443.
const LoopbackHTTPSOriginURL = "https://127.0.0.1:443"

// LoopbackHTTPOriginURL is the canonical HTTP loopback upstream. It matches
// the built-in DefaultServiceURL and is what Laragon, plain
// Apache / Nginx, and `.test`-style vhosts served over HTTP resolve to.
const LoopbackHTTPOriginURL = "http://127.0.0.1:80"

// EnvKitOriginURL is a legacy alias for LoopbackHTTPSOriginURL. It is kept
// under its original "EnvKit" name for source-level back-compat with
// existing tests / callers but applies to every HTTPS-terminating stack.
const EnvKitOriginURL = LoopbackHTTPSOriginURL

// LocalHTTPSOriginURL is a legacy alias for LoopbackHTTPSOriginURL kept
// so callers that adopted the stack-agnostic naming before the HTTP
// probe was added keep compiling. New code should reference
// LoopbackHTTPSOriginURL / LoopbackHTTPOriginURL directly.
const LocalHTTPSOriginURL = LoopbackHTTPSOriginURL

// BuiltInDefaultServiceURL is the value AppSettings.DefaultSettings ships
// with. Callers should treat this as "user has not customised the
// default" so we can transparently swap to the local-stack origin when
// a stack is detected.
const BuiltInDefaultServiceURL = LoopbackHTTPOriginURL

// loopbackProbeTimeout caps how long a single loopback probe (TCP dial,
// TLS handshake, or HTTP HEAD) waits before giving up. Loopback dials
// normally complete in single-digit milliseconds; 500ms is generous and
// keeps app startup snappy even on slow machines.
const loopbackProbeTimeout = 500 * time.Millisecond

// Info describes a detected local dev stack on the host.
type Info struct {
	// Detected is true when any local stack was found on this machine
	// (a registered install OR a live HTTP/HTTPS listener on 127.0.0.1).
	Detected bool `json:"detected"`
	// Kind identifies which stack was matched. Empty when Detected is
	// false.
	Kind Kind `json:"kind"`
	// Name is the human-readable display name of the detected stack
	// (e.g. "EnvKit", "Laravel Herd", "HTTPS service on 127.0.0.1:443").
	Name string `json:"name"`
	// Version is the best-effort version string reported by the
	// installer. May be empty for stacks that do not expose a version
	// (e.g. Valet, loopback fallback).
	Version string `json:"version"`
	// InstallPath is the absolute path to the install directory. Empty
	// for the loopback fallback.
	InstallPath string `json:"installPath"`
	// SuggestedOriginURL is the upstream URL Exposely should prefer when
	// the user has not customised their default. It is picked
	// dynamically per host based on what is actually reachable on the
	// loopback address (HTTP:80 vs HTTPS:443). Always populated so
	// callers do not have to know which stack is active.
	SuggestedOriginURL string `json:"suggestedOriginUrl"`
}

// IsEnvKit returns true when the detected stack is specifically EnvKit.
// Callers use it to preserve the original EnvKit-only behaviour where it
// matters (legacy TunnelStatus fields, EnvKit-specific UI text).
func (i Info) IsEnvKit() bool { return i.Detected && i.Kind == KindEnvKit }

// Detect inspects the host for any supported local dev stack and returns
// the first match. Detection order is:
//
//  1. EnvKit (registry / app bundle).
//  2. Laravel Herd (registry / app bundle).
//  3. Laravel Valet (macOS Composer / Homebrew install).
//  4. Laragon (registry / well-known Windows path).
//  5. Generic HTTPS listener on 127.0.0.1:443 (covers Caddy, Nginx w/ SSL,
//     Apache w/ SSL, or any custom local-host HTTPS setup).
//  6. Generic HTTP listener on 127.0.0.1:80 (covers plain Nginx / Apache
//     / IIS / anything else serving `.test` or `.local` vhosts on 80).
//
// SuggestedOriginURL is always populated. Detected is false only when no
// stack was identified and both loopback probes failed.
func Detect() Info {
	// Cache the loopback probe results once per Detect() call: each
	// installer path may want to consult them and re-dialling
	// 127.0.0.1:443 four times would add avoidable startup latency.
	loopback := probeLoopback()

	if info, ok := detectEnvKit(); ok {
		return finaliseOrigin(info, protocolHTTPS, loopback)
	}
	if info, ok := detectHerd(); ok {
		return finaliseOrigin(info, protocolHTTPS, loopback)
	}
	if info, ok := detectValet(); ok {
		return finaliseOrigin(info, protocolHTTPS, loopback)
	}
	if info, ok := detectLaragon(); ok {
		// Laragon's out-of-the-box setup is HTTP on port 80. Users can
		// opt into HTTPS but that is the exception, so we prefer HTTP.
		return finaliseOrigin(info, protocolHTTP, loopback)
	}
	if loopback.httpsUp {
		return Info{
			Detected:           true,
			Kind:               KindHTTPSLoopback,
			Name:               "HTTPS service on 127.0.0.1:443",
			SuggestedOriginURL: LoopbackHTTPSOriginURL,
		}
	}
	if loopback.httpUp {
		return Info{
			Detected:           true,
			Kind:               KindHTTPLoopback,
			Name:               "HTTP service on 127.0.0.1:80",
			SuggestedOriginURL: LoopbackHTTPOriginURL,
		}
	}
	// Nothing detected — fall back to the HTTPS URL so error messages
	// downstream still have a well-defined default to reference.
	return Info{SuggestedOriginURL: LoopbackHTTPSOriginURL}
}

// finaliseOrigin picks the SuggestedOriginURL for an installer-detected
// stack based on what is actually reachable on the loopback address.
//
// Priority is:
//  1. The stack's preferred protocol if that port is reachable.
//  2. The other protocol if only that one is reachable.
//  3. The stack's preferred protocol as a last-resort default (used when
//     the user is not currently running the stack — the resulting URL
//     will fail the reachability check with a helpful "start your local
//     web server" message, which is better than silently picking the
//     wrong port).
func finaliseOrigin(info Info, preferred protocol, probe loopbackResult) Info {
	preferredURL := LoopbackHTTPSOriginURL
	otherURL := LoopbackHTTPOriginURL
	preferredUp := probe.httpsUp
	otherUp := probe.httpUp
	if preferred == protocolHTTP {
		preferredURL, otherURL = otherURL, preferredURL
		preferredUp, otherUp = otherUp, preferredUp
	}

	switch {
	case preferredUp:
		info.SuggestedOriginURL = preferredURL
	case otherUp:
		info.SuggestedOriginURL = otherURL
	default:
		info.SuggestedOriginURL = preferredURL
	}
	return info
}

// DetectEnvKit is the legacy entry point. It only flags actual EnvKit
// installs and is preserved so callers / tests that specifically care
// about EnvKit keep working. New code should use Detect().
func DetectEnvKit() Info {
	if info, ok := detectEnvKit(); ok {
		return finaliseOrigin(info, protocolHTTPS, probeLoopback())
	}
	return Info{SuggestedOriginURL: LoopbackHTTPSOriginURL}
}

// IsBuiltInDefaultServiceURL reports whether the supplied DefaultServiceURL
// value is still the Exposely built-in default (or empty). Callers use this to
// decide whether it is safe to swap the origin to the local-stack HTTPS URL
// without overwriting a user preference.
func IsBuiltInDefaultServiceURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.EqualFold(trimmed, BuiltInDefaultServiceURL)
}

// ----- Loopback probe (HTTP + HTTPS) ---------------------------------------

// loopbackResult captures what is currently answering on 127.0.0.1's
// canonical ports. Both fields default to false when the corresponding
// probe fails or times out.
type loopbackResult struct {
	httpsUp bool
	httpUp  bool
}

// probeLoopback fires the HTTP:80 and HTTPS:443 probes in parallel so the
// worst-case total wait time is bounded by loopbackProbeTimeout, not the
// sum of the two.
func probeLoopback() loopbackResult {
	httpsCh := make(chan bool, 1)
	httpCh := make(chan bool, 1)
	go func() { httpsCh <- probeLoopbackHTTPS() }()
	go func() { httpCh <- probeLoopbackHTTP() }()
	return loopbackResult{
		httpsUp: <-httpsCh,
		httpUp:  <-httpCh,
	}
}

// probeLoopbackHTTPS returns true when a TLS service is answering on
// 127.0.0.1:443. We deliberately do a real TLS handshake (with
// InsecureSkipVerify) so we don't false-positive on a bare TCP port; some
// local stacks share 443 with TCP-only services.
func probeLoopbackHTTPS() bool {
	dialer := &net.Dialer{Timeout: loopbackProbeTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", "127.0.0.1:443", &tls.Config{
		InsecureSkipVerify: true,
		// Send a host we expect any local web server to accept — most
		// loopback HTTPS terminators are SNI-aware and will refuse an
		// empty SNI. "localhost" is universally accepted.
		ServerName: "localhost",
	})
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// probeLoopbackHTTP returns true when an HTTP service is answering on
// 127.0.0.1:80. We send a real HEAD request (rather than a bare TCP dial)
// so we don't false-positive on TCP-only services that happen to squat
// on port 80. Any HTTP response — including 4xx/5xx — counts as "there
// is a web server here"; only transport errors mean nothing is listening.
func probeLoopbackHTTP() bool {
	client := &http.Client{
		Timeout: loopbackProbeTimeout,
		// Do not chase redirects: we just want to confirm a web server
		// answered on 80.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodHead, "http://127.0.0.1:80/", nil)
	if err != nil {
		return false
	}
	req.Host = "localhost"
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// ----- EnvKit ---------------------------------------------------------------

func detectEnvKit() (Info, bool) {
	info := Info{SuggestedOriginURL: LoopbackHTTPSOriginURL, Kind: KindEnvKit, Name: "EnvKit"}
	var (
		path    string
		version string
		ok      bool
	)
	switch runtime.GOOS {
	case "windows":
		path, version, ok = detectEnvKitWindows()
	case "darwin":
		path, version, ok = detectEnvKitDarwin()
	default:
		ok = false
	}
	if !ok {
		return Info{}, false
	}
	info.Detected = true
	info.Version = strings.TrimSpace(version)
	info.InstallPath = strings.TrimSpace(path)
	return info, true
}

func detectEnvKitWindows() (string, string, bool) {
	if install, version, ok := findInUninstallKeys("envkit"); ok {
		return install, version, true
	}
	// Fallback: even without registry, EnvKit may have been unzipped into a
	// well-known location. Only treat this as a positive signal when the
	// directory really looks like an EnvKit install.
	for _, candidate := range envKitWindowsFallbackPaths() {
		if isEnvKitWindowsDir(candidate) {
			return candidate, "", true
		}
	}
	return "", "", false
}

func envKitWindowsFallbackPaths() []string {
	candidates := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "envkit"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "envkit"),
		filepath.Join(os.Getenv("APPDATA"), "envkit"),
		`C:\Program Files\envkit`,
		`C:\Program Files (x86)\envkit`,
	}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func isEnvKitWindowsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	// EnvKit.exe is the desktop binary the installer drops.
	if _, err := os.Stat(filepath.Join(path, "EnvKit.exe")); err == nil {
		return true
	}
	// resources/app.asar is the Electron payload inside an unzipped install.
	if _, err := os.Stat(filepath.Join(path, "resources", "app.asar")); err == nil {
		return true
	}
	return false
}

func detectEnvKitDarwin() (string, string, bool) {
	appPath := "/Applications/EnvKit.app"
	if info, err := os.Stat(appPath); err != nil || !info.IsDir() {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", "", false
		}
		alt := filepath.Join(home, "Applications", "EnvKit.app")
		if info, err := os.Stat(alt); err != nil || !info.IsDir() {
			return "", "", false
		}
		appPath = alt
	}
	// Data dir is an extra positive signal but not required.
	_ = envKitDataDirPresent()
	version := readMacOSBundleVersion(appPath)
	return appPath, version, true
}

// envKitDataDirPresent returns true when the EnvKit data directory is present
// in the user's Library. It is intentionally permissive: missing data dirs
// are fine (e.g. user just installed EnvKit but has not launched it yet).
func envKitDataDirPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, name := range []string{"EnvKit", "envkit"} {
		if _, err := os.Stat(filepath.Join(home, "Library", "Application Support", name)); err == nil {
			return true
		}
	}
	return false
}

// ----- Herd -----------------------------------------------------------------

func detectHerd() (Info, bool) {
	info := Info{SuggestedOriginURL: LoopbackHTTPSOriginURL, Kind: KindHerd, Name: "Laravel Herd"}
	var (
		path    string
		version string
		ok      bool
	)
	switch runtime.GOOS {
	case "windows":
		path, version, ok = detectHerdWindows()
	case "darwin":
		path, version, ok = detectHerdDarwin()
	default:
		ok = false
	}
	if !ok {
		return Info{}, false
	}
	info.Detected = true
	info.Version = strings.TrimSpace(version)
	info.InstallPath = strings.TrimSpace(path)
	return info, true
}

func detectHerdWindows() (string, string, bool) {
	// Herd's installer drops Uninstall entries with DisplayName starting
	// with "Herd" (sometimes "Laravel Herd").
	for _, needle := range []string{"laravel herd", "herd"} {
		if install, version, ok := findInUninstallKeys(needle); ok {
			return install, version, true
		}
	}
	// Disk fallback — Herd installs into %LOCALAPPDATA%\Programs\Herd by default.
	candidates := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Herd"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Herd"),
		`C:\Program Files\Herd`,
	}
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(c, "Herd.exe")); err == nil {
				return c, "", true
			}
		}
	}
	return "", "", false
}

func detectHerdDarwin() (string, string, bool) {
	appPath := "/Applications/Herd.app"
	if info, err := os.Stat(appPath); err == nil && info.IsDir() {
		return appPath, readMacOSBundleVersion(appPath), true
	}
	home, err := os.UserHomeDir()
	if err == nil {
		alt := filepath.Join(home, "Applications", "Herd.app")
		if info, err := os.Stat(alt); err == nil && info.IsDir() {
			return alt, readMacOSBundleVersion(alt), true
		}
	}
	return "", "", false
}

// ----- Valet (macOS) --------------------------------------------------------

func detectValet() (Info, bool) {
	if runtime.GOOS != "darwin" {
		return Info{}, false
	}
	candidates := []string{
		"/usr/local/bin/valet",
		"/opt/homebrew/bin/valet",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".composer", "vendor", "bin", "valet"),
			filepath.Join(home, ".config", "composer", "vendor", "bin", "valet"),
		)
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return Info{
				Detected:           true,
				Kind:               KindValet,
				Name:               "Laravel Valet",
				InstallPath:        c,
				SuggestedOriginURL: LoopbackHTTPSOriginURL,
			}, true
		}
	}
	return Info{}, false
}

// ----- Laragon (Windows) ----------------------------------------------------

func detectLaragon() (Info, bool) {
	if runtime.GOOS != "windows" {
		return Info{}, false
	}
	info := Info{SuggestedOriginURL: LoopbackHTTPOriginURL, Kind: KindLaragon, Name: "Laragon"}
	if install, version, ok := findInUninstallKeys("laragon"); ok {
		info.Detected = true
		info.InstallPath = strings.TrimSpace(install)
		info.Version = strings.TrimSpace(version)
		return info, true
	}
	// Disk fallback — Laragon defaults to C:\laragon.
	for _, c := range []string{`C:\laragon`, `D:\laragon`} {
		if dirInfo, err := os.Stat(c); err == nil && dirInfo.IsDir() {
			if _, err := os.Stat(filepath.Join(c, "laragon.exe")); err == nil {
				info.Detected = true
				info.InstallPath = c
				return info, true
			}
		}
	}
	return Info{}, false
}

// ----- Windows registry helpers (shared) -----------------------------------

// findInUninstallKeys walks the three standard Uninstall registry roots
// looking for a DisplayName that starts with the supplied (lowercased)
// prefix. Returns InstallLocation + DisplayVersion from the first match.
func findInUninstallKeys(displayNamePrefix string) (string, string, bool) {
	if runtime.GOOS != "windows" {
		return "", "", false
	}
	prefix := strings.ToLower(strings.TrimSpace(displayNamePrefix))
	if prefix == "" {
		return "", "", false
	}
	roots := []string{
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	for _, root := range roots {
		if install, version, ok := findInUninstallKey(root, prefix); ok {
			return install, version, true
		}
	}
	return "", "", false
}

func findInUninstallKey(root, prefix string) (string, string, bool) {
	subkeys := listRegistrySubkeys(root)
	for _, sub := range subkeys {
		full := root + "\\" + sub
		displayName := readRegistryStringValue(full, "DisplayName")
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(displayName)), prefix) {
			continue
		}
		install := readRegistryStringValue(full, "InstallLocation")
		version := readRegistryStringValue(full, "DisplayVersion")
		return install, version, true
	}
	return "", "", false
}

// listRegistrySubkeys returns the immediate subkey names of the given registry
// root by parsing `reg query root`. It returns nil on failure.
func listRegistrySubkeys(root string) []string {
	cmd := exec.Command("reg", "query", root)
	cmd.SysProcAttr = sysproc.Hidden()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var keys []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "HKEY_") {
			continue
		}
		keys = append(keys, line)
	}
	return keys
}

// readRegistryStringValue fetches a single named REG_SZ value out of the
// supplied registry key by shelling out to reg query. It returns "" when the
// key or value is missing or non-string.
func readRegistryStringValue(key, name string) string {
	cmd := exec.Command("reg", "query", key, "/v", name)
	cmd.SysProcAttr = sysproc.Hidden()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.EqualFold(fields[0], name) {
			continue
		}
		rest := strings.TrimSpace(strings.Join(fields[2:], " "))
		// reg query prints REG_SZ values unquoted, REG_EXPAND_SZ with %...%.
		// Strip surrounding quotes if present.
		if len(rest) >= 2 && rest[0] == '"' && rest[len(rest)-1] == '"' {
			rest = rest[1 : len(rest)-1]
		}
		return rest
	}
	return ""
}

// ----- macOS bundle helpers -------------------------------------------------

func readMacOSBundleVersion(appPath string) string {
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	if _, err := os.Stat(plist); err != nil {
		return ""
	}
	cmd := exec.Command("/usr/bin/defaults", "read", filepath.Join(appPath, "Contents", "Info"), "CFBundleShortVersionString")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
