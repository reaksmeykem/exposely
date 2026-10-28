package stacks

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// PHPVersion is the default PHP release Exposely installs when the user
// asks for a managed PHP. Thread-Safe, x64 build (required for stable
// FastCGI worker processes).
//
// 8.5 is the newest supported branch (active support through 2027, security
// fixes through 2029). 8.4 still gets active support until the end of 2026.
const PHPVersion = "8.5.12"

// PHPInstallableVersions lists the PHP releases windows.php.net serves
// today (verified through the phpDownloadURLs fallback chain, Oct 2026).
//
// 8.1 is intentionally no longer offered: it reached end-of-life and gets
// no security fixes. 8.2 is security-only and EOL at the end of 2026.
// Users can still install an older x.y.z by typing it, but the default path
// should not lead to an unsupported runtime.
var PHPInstallableVersions = []string{"8.2.35", "8.3.36", "8.4.27", "8.5.12"}

// phpDownloadURLs lists candidate URLs for a PHP release in priority
// order: current releases first, then the archives folder where
// superseded versions are moved. The first URL that downloads wins.
//
// Windows security rebuilds carry a "-1" suffix (for example
// php-8.5.12-1-Win32-vs17-x64.zip) and are the only packages that contain
// the fixed dependencies. The 26 Oct 2026 rebuilds ship Brotli 1.2.1 for
// CVE-2026-111497 (out-of-bounds write reachable through cURL response
// decompression); the plain -Win32 filename does not exist for those
// releases, so both spellings must be tried.
func phpDownloadURLs(version string) []string {
	base := "https://windows.php.net/downloads/releases/"
	archive := base + "archives/"
	names := []string{
		"php-%s-Win32-vs17-x64.zip",
		"php-%s-1-Win32-vs17-x64.zip",
		"php-%s-Win32-vs16-x64.zip",
		"php-%s-1-Win32-vs16-x64.zip",
	}
	urls := make([]string, 0, len(names)*2)
	for _, dir := range []string{base, archive} {
		for _, name := range names {
			urls = append(urls, dir+fmt.Sprintf(name, version))
		}
	}
	return urls
}

// PHPInstallDir returns the legacy managed-PHP root (the pre
// multi-version layout). Installs made before versioned PHP existed
// live directly here; it stays supported and is reported as an
// installed PHPVersion of whatever it contains.
func PHPInstallDir(appDataDir string) string {
	return filepath.Join(appDataDir, "stacks", "php")
}

// PHPVersionsRoot is the parent folder holding one sub-directory per
// installed PHP version.
func PHPVersionsRoot(appDataDir string) string {
	return PHPInstallDir(appDataDir)
}

// PHPVersionDir returns the install directory for a specific PHP
// version, e.g. <appData>/stacks/php/8.3.33.
func PHPVersionDir(appDataDir, version string) string {
	return filepath.Join(appDataDir, "stacks", "php", sanitizeVersion(version))
}

func sanitizeVersion(version string) string {
	v := strings.TrimSpace(version)
	// Keep it to x.y.z so the path can never escape the versions root.
	if !looksLikeVersion(v) {
		return ""
	}
	return v
}

func looksLikeVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// phpDirHasBins reports whether dir looks like a real PHP install
// (php-cgi.exe + php.exe both present).
func PHPDirHasBins(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "php-cgi.exe")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "php.exe")); err != nil {
		return false
	}
	return true
}

// PHPVersionInstall describes one installed managed PHP version.
type PHPVersionInstall struct {
	// Version string like "8.4.25". The pre-multi-version layout is
	// reported as whatever its php.exe says (typically 8.4.25).
	Version string
	// Dir is the install directory.
	Dir string
	// Legacy marks an install in the old un-versioned location.
	Legacy bool
}

// InstalledPHPVersions enumerates the managed PHP installs on disk:
// every versioned directory plus the legacy root when present. Sorted
// by version, oldest first.
//
// When the legacy root holds the same version as one of the versioned
// directories (the common upgrade path: the old layout was copied into
// a versioned dir), only the versioned entry is reported. Reporting
// both made the UI show two identical "PHP 8.4.25 · ACTIVE" chips and
// made "Use" buttons appear for a version already in use.
func InstalledPHPVersions(appDataDir string) []PHPVersionInstall {
	var out []PHPVersionInstall
	seen := make(map[string]bool)
	root := PHPVersionsRoot(appDataDir)
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || !looksLikeVersion(e.Name()) {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if PHPDirHasBins(dir) {
				out = append(out, PHPVersionInstall{Version: e.Name(), Dir: dir})
				seen[e.Name()] = true
			}
		}
	}
	// Legacy root install (files directly in stacks/php). Skip it when
	// its version is already listed from a versioned directory.
	if PHPDirHasBins(root) {
		version := PHPVersionOf(root)
		if version == "" || !looksLikeVersion(version) {
			version = PHPVersion
		}
		if !seen[version] {
			out = append(out, PHPVersionInstall{Version: version, Dir: root, Legacy: true})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return compareVersionPaths(out[i].Version, out[j].Version) < 0
	})
	return out
}

// ActivePHPDirFor resolves the directory of the active managed PHP for
// a settings-stored version: the versioned dir when it exists, then
// the legacy root, then the default version's (possibly future) dir.
func ActivePHPDirFor(appDataDir, version string) string {
	if v := sanitizeVersion(version); v != "" {
		dir := PHPVersionDir(appDataDir, v)
		if PHPDirHasBins(dir) {
			return dir
		}
	}
	if PHPDirHasBins(PHPInstallDir(appDataDir)) {
		return PHPInstallDir(appDataDir)
	}
	return PHPVersionDir(appDataDir, PHPVersion)
}

// PHPInstalled reports whether Exposely's managed PHP is present and
// looks like a real PHP install (php-cgi.exe + php.exe).
func PHPInstalled(appDataDir string) bool {
	return PHPDirHasBins(PHPInstallDir(appDataDir))
}

// InstallPHP downloads and extracts the default managed PHP version.
// See InstallPHPVersion for the general flow.
func InstallPHP(appDataDir string) (string, string, error) {
	return InstallPHPVersion(appDataDir, PHPVersion)
}

// InstallPHPVersion downloads and extracts the official PHP Windows zip
// for the requested version into <appData>/stacks/php/<version>.
// Steps:
//
//  1. Skip when already installed (idempotent).
//  2. Download the archive to a temp file.
//  3. Extract into the version dir (cleaning a partial attempt first).
//  4. Write a production-oriented php.ini with the extensions Laravel
//     needs, enabled by default (only when no ini exists yet, so user
//     tuning survives a reinstall).
//
// Returns the install directory and the php.ini path.
func InstallPHPVersion(appDataDir, version string) (string, string, error) {
	version = strings.TrimSpace(version)
	if !looksLikeVersion(version) {
		return "", "", fmt.Errorf("invalid PHP version %q (expected x.y.z)", version)
	}
	dir := PHPVersionDir(appDataDir, version)
	if PHPDirHasBins(dir) {
		// Idempotent path: still make sure a CA bundle exists so curl can
		// verify HTTPS (cURL 60) even when the zip is already extracted.
		EnsureCACert(dir)
		iniPath := PhpIniPath(dir)
		if _, err := os.Stat(iniPath); err == nil {
			_ = EnsurePHPIniCASettings(iniPath, dir)
			// Existing installs keep their php.ini; repair the security
			// keys instead of leaving an old file in place.
			_, _ = HardenPHPIni(iniPath)
		}
		return dir, iniPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", "", err
	}

	zipPath, err := downloadPHPZip(version)
	if err != nil {
		return "", "", err
	}
	defer os.Remove(zipPath)

	// Remove any partial previous attempt.
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	if err := extractZip(zipPath, dir); err != nil {
		return "", "", fmt.Errorf("extract php: %w", err)
	}

	// Sanity check: the archive must contain the binaries we run.
	if !PHPDirHasBins(dir) {
		return "", "", fmt.Errorf("php %s archive did not contain php-cgi.exe/php.exe", version)
	}

	// Always try to place a CA bundle first so the generated php.ini can
	// point curl/openssl at it (fixes cURL 60 on outbound HTTPS).
	EnsureCACert(dir)

	iniPath := PhpIniPath(dir)
	if _, err := os.Stat(iniPath); err != nil {
		if err := WriteFile(iniPath, PHPIniTemplate(dir)); err != nil {
			return "", "", fmt.Errorf("write php.ini: %w", err)
		}
	} else {
		// Existing installs may predate the CA settings; patch them in
		// without wiping user tuning.
		if err := EnsurePHPIniCASettings(iniPath, dir); err != nil {
			return dir, iniPath, err
		}
	}
	return dir, iniPath, nil
}

// EnsurePHPIniCASettings inserts or refreshes curl.cainfo / openssl.cafile
// in an existing php.ini so outbound HTTPS works after a PHP upgrade that
// did not rewrite the file. User values elsewhere are left untouched.
func EnsurePHPIniCASettings(iniPath, phpDir string) error {
	caPath := existingOrEmpty(filepath.Join(phpDir, CACertFileName))
	if caPath == "" {
		caPath = filepath.Join(phpDir, CACertFileName)
	}
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return err
	}
	content := string(data)
	wantCainfo := "curl.cainfo = " + quoteIniPath(caPath)
	wantCafile := "openssl.cafile = " + quoteIniPath(caPath)

	if strings.Contains(content, "curl.cainfo") && strings.Contains(content, "openssl.cafile") {
		return nil
	}

	var b strings.Builder
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n; CA bundle for curl/openssl HTTPS verification (added by Exposely)\n")
	if !strings.Contains(content, "curl.cainfo") {
		b.WriteString(wantCainfo + "\n")
	}
	if !strings.Contains(content, "openssl.cafile") {
		b.WriteString(wantCafile + "\n")
	}
	return WriteFile(iniPath, b.String())
}

func downloadPHPZip(version string) (string, error) {
	var lastErr error
	for _, url := range phpDownloadURLs(version) {
		zipPath, err := downloadFile(url)
		if err == nil {
			return zipPath, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func downloadFile(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download php: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download php: HTTP %d from %s", resp.StatusCode, url)
	}

	tmp, err := os.CreateTemp("", "exposely-php-*.zip")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("save php zip: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

func extractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, f := range reader.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		// Zip-slip guard: the resolved path must stay under destDir.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// PhpIniPath returns the php.ini location Exposely manages inside a PHP
// install directory.
func PhpIniPath(phpDir string) string {
	return filepath.Join(phpDir, "php.ini")
}

// PHPIniSettings captures the values the UI exposes for tuning.
type PHPIniSettings struct {
	// MemoryLimit like "256M".
	MemoryLimit string
	// UploadMaxFilesize like "64M".
	UploadMaxFilesize string
	// PostMaxSize like "64M".
	PostMaxSize string
	// MaxExecutionTime in seconds.
	MaxExecutionTime int
	// Extensions beyond the always-on baseline (e.g. "soap", "xsl").
	ExtraExtensions []string
}

// DefaultPHPIniSettings matches what Laravel projects typically need.
func DefaultPHPIniSettings() PHPIniSettings {
	return PHPIniSettings{
		MemoryLimit:       "256M",
		UploadMaxFilesize: "64M",
		PostMaxSize:       "64M",
		MaxExecutionTime:  120,
		ExtraExtensions:   nil,
	}
}

// phpExtensionDLLs maps logical extension names to the DLL filenames
// shipped in the official PHP zip. Only entries the user can toggle are
// listed; core extensions (json, filter, etc.) are compiled in.
var phpExtensionDLLs = map[string]string{
	"curl":      "php_curl.dll",
	"fileinfo":  "php_fileinfo.dll",
	"gd":        "php_gd.dll",
	"mysqli":    "php_mysqli.dll",
	"openssl":   "php_openssl.dll",
	"pdo_mysql": "php_pdo_mysql.dll",
	"mbstring":  "php_mbstring.dll",
	"intl":      "php_intl.dll",
	"zip":       "php_zip.dll",
	"soap":      "php_soap.dll",
	"xsl":       "php_xsl.dll",
	"exif":      "php_exif.dll",
	"imap":      "php_imap.dll",
	"ldap":      "php_ldap.dll",
	"pgsql":     "php_pgsql.dll",
	"pdo_pgsql": "php_pdo_pgsql.dll",
	"sqlite3":   "php_sqlite3.dll",
	"bz2":       "php_bz2.dll",
}

// baselineExtensions are enabled by default — the minimum Laravel and
// most CMS projects need.
var baselineExtensions = []string{
	"curl", "fileinfo", "gd", "mysqli", "openssl",
	"pdo_mysql", "mbstring", "intl", "zip",
}

// PHPIniTemplate renders a complete php.ini for FastCGI use.
func PHPIniTemplate(phpDir string) string {
	return PHPIniTemplateWith(phpDir, DefaultPHPIniSettings())
}

// CACertFileName is the CA bundle Exposely keeps next to each managed
// PHP install so curl/openssl can verify HTTPS peers (Turnstile, mail
// APIs, composer, …). Windows PHP builds do not ship a CA store.
const CACertFileName = "cacert.pem"

// CACertURL is the canonical Mozilla CA bundle used by curl.se and most
// PHP Windows setups.
const CACertURL = "https://curl.se/ca/cacert.pem"

// EnsureCACert downloads cacert.pem into the PHP install directory when
// it is missing. Returns the bundle path (always under phpDir) or an
// empty string when the download failed and no bundle is present yet.
func EnsureCACert(phpDir string) string {
	path := filepath.Join(phpDir, CACertFileName)
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(CACertURL)
	if err != nil {
		return existingOrEmpty(path)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return existingOrEmpty(path)
	}

	tmp, err := os.CreateTemp(phpDir, "cacert-*.tmp")
	if err != nil {
		return existingOrEmpty(path)
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return existingOrEmpty(path)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return existingOrEmpty(path)
	}
	// Only accept a plausible PEM bundle so a captive portal HTML page
	// never becomes the CA store.
	data, err := os.ReadFile(tmp.Name())
	if err != nil || !bytes.Contains(data, []byte("BEGIN CERTIFICATE")) {
		os.Remove(tmp.Name())
		return existingOrEmpty(path)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return existingOrEmpty(path)
	}
	return path
}

func existingOrEmpty(path string) string {
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path
	}
	return ""
}

// PHPIniTemplateWith renders php.ini honouring the supplied settings.
// Extension lines are only emitted when the DLL exists in the ext dir,
// so enabling an extension whose file is missing cannot stop PHP from
// booting.
func PHPIniTemplateWith(phpDir string, s PHPIniSettings) string {
	var b strings.Builder
	extDir := filepath.Join(phpDir, "ext")

	fmt.Fprintf(&b, "; Generated by Exposely — safe to edit by hand; the\n")
	fmt.Fprintf(&b, "; UI regenerates it when you save PHP settings.\n\n")
	fmt.Fprintf(&b, "extension_dir = %s\n", quoteIniPath(extDir))

	// Windows PHP has no built-in CA store. Without curl.cainfo /
	// openssl.cafile every outbound HTTPS call from PHP (Laravel HTTP
	// client, Guzzle, Turnstile siteverify, composer) fails with
	// "SSL certificate problem: unable to get local issuer certificate".
	caPath := filepath.Join(phpDir, CACertFileName)
	if existingOrEmpty(caPath) != "" {
		fmt.Fprintf(&b, "\n; CA bundle for curl/openssl HTTPS verification\n")
		fmt.Fprintf(&b, "curl.cainfo = %s\n", quoteIniPath(caPath))
		fmt.Fprintf(&b, "openssl.cafile = %s\n", quoteIniPath(caPath))
	} else {
		fmt.Fprintf(&b, "\n; CA bundle missing — run Install PHP again or place cacert.pem here:\n")
		fmt.Fprintf(&b, "; curl.cainfo = %s\n", quoteIniPath(caPath))
		fmt.Fprintf(&b, "; openssl.cafile = %s\n", quoteIniPath(caPath))
	}

	memory := sanitizeIniSize(s.MemoryLimit, "256M")
	upload := sanitizeIniSize(s.UploadMaxFilesize, "64M")
	post := sanitizeIniSize(s.PostMaxSize, "64M")
	execTime := s.MaxExecutionTime
	if execTime <= 0 {
		execTime = 120
	}

	fmt.Fprintf(&b, "\n; Limits\n")
	fmt.Fprintf(&b, "memory_limit = %s\n", memory)
	fmt.Fprintf(&b, "upload_max_filesize = %s\n", upload)
	fmt.Fprintf(&b, "post_max_size = %s\n", post)
	fmt.Fprintf(&b, "max_execution_time = %d\n", execTime)
	fmt.Fprintf(&b, "max_input_time = %d\n", execTime)

	fmt.Fprintf(&b, "\n; FastCGI friendly error handling\n")
	fmt.Fprintf(&b, "display_errors = Off\n")
	fmt.Fprintf(&b, "log_errors = On\n")
	fmt.Fprintf(&b, "error_log = %s\n", quoteIniPath(filepath.Join(phpDir, "php_errors.log")))
	fmt.Fprintf(&b, "cgi.force_redirect = 0\n")
	// fix_pathinfo=1 is the classic nginx/PHP path-info RCE enabler; the
	// generated nginx.conf also guards it, but defence in depth.
	fmt.Fprintf(&b, "cgi.fix_pathinfo = 0\n")
	fmt.Fprintf(&b, "fastcgi.impersonate = 1\n")

	fmt.Fprintf(&b, "\n; Security defaults\n")
	// Do not advertise the exact PHP build in the X-Powered-By header:
	// scanners use it to pick an exploit for a known patch level.
	fmt.Fprintf(&b, "expose_php = Off\n")
	fmt.Fprintf(&b, "allow_url_include = Off\n")
	fmt.Fprintf(&b, "session.cookie_httponly = 1\n")
	fmt.Fprintf(&b, "session.use_strict_mode = 1\n")

	fmt.Fprintf(&b, "\n; Timezone (override by hand if needed)\n")
	fmt.Fprintf(&b, "date.timezone = UTC\n")

	fmt.Fprintf(&b, "\n; Extensions (emitted only when the DLL exists)\n")
	enabled := append([]string{}, baselineExtensions...)
	enabled = append(enabled, s.ExtraExtensions...)
	seen := map[string]bool{}
	for _, name := range enabled {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		dll, ok := phpExtensionDLLs[name]
		if !ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(extDir, dll)); err != nil {
			fmt.Fprintf(&b, "; extension = %s   (not present in this build)\n", dll)
			continue
		}
		fmt.Fprintf(&b, "extension = %s\n", dll)
	}

	return b.String()
}

var phpSizePattern = regexp.MustCompile(`^[0-9]+[KMG]?$`)

// sanitizeIniSize accepts only a plain php.ini size ("256M", "1G", "512K").
// A value carrying a newline would append arbitrary directives to the
// generated php.ini, and anything unparsable would stop PHP from booting.
func sanitizeIniSize(value, fallback string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if phpSizePattern.MatchString(normalized) {
		return normalized
	}
	return fallback
}

// HardenPHPIni applies the security-relevant directives to an existing
// php.ini without touching any other setting. Installs made before these
// defaults existed (and versions upgraded in place) keep a php.ini with
// cgi.fix_pathinfo = 1 and no expose_php line; fixing only those keys is
// safe for user tuning elsewhere. Returns true when the file was changed.
func HardenPHPIni(iniPath string) (bool, error) {
	data, err := os.ReadFile(iniPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	content := string(data)
	for _, kv := range [][2]string{
		{"cgi.fix_pathinfo", "0"},
		{"expose_php", "Off"},
		{"allow_url_include", "Off"},
		{"session.cookie_httponly", "1"},
		{"session.use_strict_mode", "1"},
	} {
		content = setIniDirective(content, kv[0], kv[1])
	}

	if content == string(data) {
		return false, nil
	}
	return true, WriteFile(iniPath, content)
}

// setIniDirective rewrites each active `key = value` line, or appends the
// key when the file has no active line for it. Commented-out examples are
// left alone.
func setIniDirective(content, key, value string) string {
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, rest, ok := strings.Cut(trimmed, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), key) {
			continue
		}
		found = true
		if strings.EqualFold(strings.TrimSpace(rest), value) {
			continue
		}
		lines[i] = key + " = " + value
	}
	if found {
		return strings.Join(lines, "\n")
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + key + " = " + value + "\n"
}

func iniOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func quoteIniPath(path string) string {
	return `"` + strings.ReplaceAll(filepath.ToSlash(path), `\`, `/`) + `"`
}

// PHPVersionOf runs `php -v` in the supplied directory and extracts the
// version string; used by the UI to show what is installed.
func PHPVersionOf(phpDir string) string {
	phpExe := filepath.Join(phpDir, "php.exe")
	if _, err := os.Stat(phpExe); err != nil {
		return ""
	}
	cmd := exec.Command(phpExe, "-n", "-v")
	cmd.SysProcAttr = sysproc.Hidden()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	// "PHP 8.3.14 (cli) ..." -> "8.3.14"
	fields := strings.Fields(first)
	if len(fields) >= 2 && strings.HasPrefix(strings.ToLower(fields[0]), "php") {
		return fields[1]
	}
	return strings.TrimSpace(first)
}
