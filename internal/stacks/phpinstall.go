package stacks

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// PHPVersion is the PHP release Exposely installs when the user asks for
// a managed PHP. Thread-Safe, x64 build (required for stable FastCGI
// worker processes).
const PHPVersion = "8.4.25"

// phpDownloadURLs lists candidate URLs for a PHP release in priority
// order: current releases first, then the archives folder where
// superseded versions are moved. The first URL that downloads wins.
func phpDownloadURLs(version string) []string {
	return []string{
		fmt.Sprintf("https://windows.php.net/downloads/releases/php-%s-Win32-vs17-x64.zip", version),
		fmt.Sprintf("https://windows.php.net/downloads/releases/php-%s-Win32-vs16-x64.zip", version),
		fmt.Sprintf("https://windows.php.net/downloads/releases/archives/php-%s-Win32-vs17-x64.zip", version),
		fmt.Sprintf("https://windows.php.net/downloads/releases/archives/php-%s-Win32-vs16-x64.zip", version),
	}
}

// PHPInstallDir returns where Exposely installs its own PHP.
func PHPInstallDir(appDataDir string) string {
	return filepath.Join(appDataDir, "stacks", "php")
}

// PHPInstalled reports whether Exposely's managed PHP is present and
// looks like a real PHP install (php-cgi.exe + php.exe).
func PHPInstalled(appDataDir string) bool {
	dir := PHPInstallDir(appDataDir)
	if _, err := os.Stat(filepath.Join(dir, "php-cgi.exe")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "php.exe")); err != nil {
		return false
	}
	return true
}

// InstallPHP downloads and extracts the official PHP Windows zip into
// Exposely's data dir. Steps:
//
//  1. Skip when already installed (idempotent).
//  2. Download the archive to a temp file.
//  3. Extract into <appData>/stacks/php (cleaning a partial attempt
//     first).
//  4. Write a production-oriented php.ini with the extensions Laravel
//     needs, enabled by default.
//
// Returns the install directory and the php.ini path.
func InstallPHP(appDataDir string) (string, string, error) {
	dir := PHPInstallDir(appDataDir)
	if PHPInstalled(appDataDir) {
		return dir, PhpIniPath(dir), nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", "", err
	}

	zipPath, err := downloadPHPZip()
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
	if !PHPInstalled(appDataDir) {
		return "", "", fmt.Errorf("php archive did not contain php-cgi.exe/php.exe")
	}

	iniPath := PhpIniPath(dir)
	if err := WriteFile(iniPath, PHPIniTemplate(dir)); err != nil {
		return "", "", fmt.Errorf("write php.ini: %w", err)
	}
	return dir, iniPath, nil
}

func downloadPHPZip() (string, error) {
	var lastErr error
	for _, url := range phpDownloadURLs(PHPVersion) {
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

	memory := iniOr(s.MemoryLimit, "256M")
	upload := iniOr(s.UploadMaxFilesize, "64M")
	post := iniOr(s.PostMaxSize, "64M")
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
	fmt.Fprintf(&b, "cgi.fix_pathinfo = 1\n")
	fmt.Fprintf(&b, "fastcgi.impersonate = 1\n")

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
