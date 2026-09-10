package stacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MariaDBVersion is the MariaDB release Exposely installs when the
// user asks for a managed database. 11.4 is an LTS series; pin the
// newest patch the mirrors still carry — regional mirrors prune old
// patches, while archive.mariadb.org keeps every release forever as
// the fallback.
var MariaDBVersion = "11.4.13"

// mariaDBDownloadURLs lists candidate URLs for the portable Windows
// zip, fastest-first for common network regions: regional mirrors
// (much faster in Asia/Pacific) then the canonical archive host.
func mariaDBDownloadURLs(version string) []string {
	return []string{
		fmt.Sprintf("https://mirrors.aliyun.com/mariadb/mariadb-%s/winx64-packages/mariadb-%s-winx64.zip", version, version),
		fmt.Sprintf("https://archive.mariadb.org/mariadb-%s/winx64-packages/mariadb-%s-winx64.zip", version, version),
	}
}

// MariaDBInstallDir returns where Exposely installs its own MariaDB.
func MariaDBInstallDir(appDataDir string) string {
	return filepath.Join(appDataDir, "stacks", "mariadb")
}

// MariaDBDataDir returns the managed database data directory. Shared
// with the stack settings flow (stackMySQLDataDir) so an existing
// managed data dir keeps its databases across installer changes.
func MariaDBDataDir(appDataDir string) string {
	return filepath.Join(appDataDir, "stacks", "mysql", "data")
}

// MariaDBInstalled reports whether the managed MariaDB looks complete.
func MariaDBInstalled(appDataDir string) bool {
	dir := MariaDBInstallDir(appDataDir)
	info, err := os.Stat(filepath.Join(dir, "bin", "mysqld.exe"))
	return err == nil && !info.IsDir() && info.Size() > 0
}

// InstallMariaDB downloads and extracts the portable MariaDB Windows
// zip into Exposely's data dir, following the managed-PHP pattern:
//
//  1. Skip when already installed (idempotent).
//  2. Download the archive to a temp file.
//  3. Extract into <appData>/stacks/mariadb (cleaning a partial
//     attempt first). The official zip wraps everything in
//     mariadb-<version>-winx64/, which is flattened so bin/mysqld.exe
//     sits at the expected place.
//
// The data directory is deliberately NOT created here: the normal
// StartStackService flow initialises it via EnsureMySQLDataDir, and
// on this machine the dir already holds the migrated databases.
// Returns the install directory and the mysqld.exe path.
func InstallMariaDB(appDataDir string) (string, string, error) {
	dir := MariaDBInstallDir(appDataDir)
	if MariaDBInstalled(appDataDir) {
		return dir, filepath.Join(dir, "bin", "mysqld.exe"), nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", "", err
	}

	zipPath, err := downloadArchive("mariadb", mariaDBDownloadURLs(MariaDBVersion))
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
		return "", "", fmt.Errorf("extract mariadb: %w", err)
	}

	// The official archive nests everything inside
	// mariadb-<version>-winx64/. Flatten it so bin/mysqld.exe sits
	// directly under the install dir.
	if !MariaDBInstalled(appDataDir) {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			return "", "", readErr
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "mariadb-") {
				continue
			}
			nested := filepath.Join(dir, entry.Name())
			if err := flattenDir(nested, dir); err != nil {
				return "", "", fmt.Errorf("flatten mariadb archive layout: %w", err)
			}
			break
		}
	}

	if !MariaDBInstalled(appDataDir) {
		return "", "", fmt.Errorf("mariadb archive did not contain bin/mysqld.exe")
	}

	return dir, filepath.Join(dir, "bin", "mysqld.exe"), nil
}
