// Package localstack detects well-known local development stacks running on the
// host so Exposely can pick sensible defaults (origin URL, hints) without the
// user hand-editing settings.
//
// Right now only EnvKit (https://envkit.net) is detected. EnvKit is similar to
// Herd / Laragon / XAMPP but always serves *.test sites over trusted HTTPS on
// port 443. Exposely already supports the .test TLD via the local-host share
// mode; this package adds an extra, lightweight pass that:
//   - reports whether EnvKit is installed and where,
//   - suggests https://127.0.0.1:443 as the upstream origin URL so cloudflared
//     tunnels the HTTPS site instead of the bare HTTP loopback,
//   - only swaps the default when the user has not customised DefaultServiceURL
//     (or the per-project OriginURL), so existing setups keep working.
package localstack

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EnvKitOriginURL is the upstream URL Exposely uses when EnvKit is detected
// and no custom origin has been configured. EnvKit terminates TLS at its local
// nginx/apache on 127.0.0.1:443 by default.
const EnvKitOriginURL = "https://127.0.0.1:443"

// BuiltInDefaultServiceURL is the value AppSettings.DefaultSettings ships with.
// Callers should treat this as "user has not customised the default" so we
// can transparently swap to the EnvKit origin when EnvKit is detected.
const BuiltInDefaultServiceURL = "http://127.0.0.1:80"

// Info describes a detected EnvKit install on the host.
type Info struct {
	// Detected is true when EnvKit was found on this machine.
	Detected bool `json:"detected"`
	// Version is the best-effort version string reported by the installer
	// (registry DisplayVersion on Windows, CFBundleShortVersionString on
	// macOS). May be empty when the value is not available.
	Version string `json:"version"`
	// InstallPath is the absolute path to the EnvKit install directory
	// (Windows InstallLocation or /Applications/EnvKit.app).
	InstallPath string `json:"installPath"`
	// SuggestedOriginURL is the upstream URL Exposely should prefer when
	// the user has not customised their default. It is always populated so
	// callers do not have to know which stack is active.
	SuggestedOriginURL string `json:"suggestedOriginUrl"`
}

// DetectEnvKit inspects the host for an EnvKit install. It never errors — the
// returned Info simply has Detected=false when nothing is found. Detection
// runs synchronously and shells out to the platform-native lookup tools
// (reg.exe on Windows, defaults on macOS) so the call is cheap but should
// still be cached by the caller if called frequently.
func DetectEnvKit() Info {
	info := Info{SuggestedOriginURL: EnvKitOriginURL}
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
		return info
	}
	info.Detected = true
	info.Version = strings.TrimSpace(version)
	info.InstallPath = strings.TrimSpace(path)
	return info
}

// IsBuiltInDefaultServiceURL reports whether the supplied DefaultServiceURL
// value is still the Exposely built-in default (or empty). Callers use this to
// decide whether it is safe to swap the origin to the EnvKit HTTPS URL
// without overwriting a user preference.
func IsBuiltInDefaultServiceURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.EqualFold(trimmed, BuiltInDefaultServiceURL)
}

func detectEnvKitWindows() (string, string, bool) {
	roots := []string{
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	for _, root := range roots {
		if install, version, ok := findEnvKitInUninstallKey(root); ok {
			return install, version, true
		}
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

// findEnvKitInUninstallKey lists the immediate subkeys of root, asks each one
// for its DisplayName value, and returns InstallLocation+DisplayVersion from
// the first subkey whose DisplayName starts with "EnvKit".
func findEnvKitInUninstallKey(root string) (string, string, bool) {
	subkeys := listRegistrySubkeys(root)
	for _, sub := range subkeys {
		full := root + "\\" + sub
		displayName := readRegistryStringValue(full, "DisplayName")
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(displayName)), "envkit") {
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
	cmd.SysProcAttr = hiddenProcAttr()
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
	cmd.SysProcAttr = hiddenProcAttr()
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

// envKitWindowsFallbackPaths returns a list of candidate install directories
// for EnvKit on Windows. We only use these when the registry does not reveal
// an install — EnvKit's installer writes the registry entries by default but
// power users occasionally unzip the app instead.
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
		// Some users install into a per-user Applications dir.
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

	// The data dir is an extra positive signal but not required — EnvKit
	// creates ~/Library/Application Support/EnvKit on first launch and
	// older builds used a lowercased directory name.
	_ = envKitDataDirPresent()

	version := readMacOSBundleVersion(appPath)
	return appPath, version, true
}

// envKitDataDirPresent returns true when the EnvKit data directory is present
// in the user's Library. It is intentionally permissive: missing data dirs are
// fine (e.g. user just installed EnvKit but has not launched it yet).
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
