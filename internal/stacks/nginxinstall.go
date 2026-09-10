package stacks

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NginxVersion is the nginx release Exposely installs when the user
// asks for a managed nginx. Kept as a variable so tests can pin it.
var NginxVersion = "1.26.3"

// nginxDownloadURLs lists candidate URLs for the official nginx
// Windows zip in priority order: the mainline directory first, then
// the archives folder where superseded versions are moved. The first
// URL that downloads wins.
func nginxDownloadURLs(version string) []string {
	return []string{
		fmt.Sprintf("https://nginx.org/download/nginx-%s.zip", version),
		fmt.Sprintf("https://nginx.org/download/archive/nginx-%s.zip", version),
	}
}

// NginxInstallDir returns where Exposely installs its own nginx.
func NginxInstallDir(appDataDir string) string {
	return filepath.Join(appDataDir, "stacks", "nginx")
}

// NginxInstalled reports whether the managed nginx looks complete.
func NginxInstalled(appDataDir string) bool {
	dir := NginxInstallDir(appDataDir)
	info, err := os.Stat(filepath.Join(dir, "nginx.exe"))
	return err == nil && !info.IsDir() && info.Size() > 0
}

// InstallNginx downloads and extracts the official nginx Windows zip
// into Exposely's data dir, following the managed-PHP pattern:
//
//  1. Skip when already installed (idempotent).
//  2. Download the archive to a temp file.
//  3. Extract into <appData>/stacks/nginx (cleaning a partial attempt
//     first). The official zip wraps everything in nginx-<version>/,
//     which is flattened so nginx.exe sits at the root.
//  4. Ensure logs/temp dirs exist; nginx creates conf copies itself on
//     first start from the generated conf Exposely passes via -c.
//
// Returns the install directory and the nginx.exe path.
func InstallNginx(appDataDir string) (string, string, error) {
	dir := NginxInstallDir(appDataDir)
	if NginxInstalled(appDataDir) {
		return dir, filepath.Join(dir, "nginx.exe"), nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", "", err
	}

	zipPath, err := downloadArchive("nginx", nginxDownloadURLs(NginxVersion))
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
		return "", "", fmt.Errorf("extract nginx: %w", err)
	}

	// The official archive nests everything inside nginx-<version>/.
	// Flatten it so nginx.exe sits directly in the install dir.
	if !NginxInstalled(appDataDir) {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			return "", "", readErr
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "nginx-") {
				continue
			}
			nested := filepath.Join(dir, entry.Name())
			if err := flattenDir(nested, dir); err != nil {
				return "", "", fmt.Errorf("flatten nginx archive layout: %w", err)
			}
			break
		}
	}

	if !NginxInstalled(appDataDir) {
		return "", "", fmt.Errorf("nginx archive did not contain nginx.exe")
	}

	// nginx expects these relative to the -p prefix; create them up
	// front so a first start never fails on a missing logs dir.
	for _, sub := range []string{"logs", "temp"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return "", "", err
		}
	}
	return dir, filepath.Join(dir, "nginx.exe"), nil
}

// flattenDir moves the contents of src into dst (which must exist),
// then removes the now-empty src. Existing files in dst are replaced.
func flattenDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if _, statErr := os.Stat(to); statErr == nil {
				// Merge directory contents recursively.
				if err := flattenDir(from, to); err != nil {
					return err
				}
				continue
			}
			if err := os.Rename(from, to); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(to); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			// Cross-device or locked: fall back to a streaming copy
			// (copyFile from phpmyadmin.go reads the whole file, but
			// nginx archives hold no huge files, so either is fine).
			if copyErr := copyFileStreaming(from, to); copyErr != nil {
				return err
			}
			_ = os.Remove(from)
		}
	}
	return os.Remove(src)
}

func copyFileStreaming(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// downloadArchive fetches the first reachable URL into a temp file.
// kind only names the temp file (and clarifies error messages).
func downloadArchive(kind string, urls []string) (string, error) {
	var lastErr error
	for _, url := range urls {
		zipPath, err := downloadFileTo(kind, url)
		if err == nil {
			return zipPath, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func downloadFileTo(kind, url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d from %s", kind, resp.StatusCode, url)
	}

	pattern := "exposely-" + kind + "-*.zip"
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("save %s archive: %w", kind, err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}
