package stacks

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DetectBinaries scans the machine for nginx, php-cgi, and mysqld
// binaries and returns the best candidate found for each service.
// Services with no hit anywhere are absent from the map.
//
// Search order per service (first hit wins):
//  1. Exposely's own managed installs (stacks/nginx, stacks/php,
//     stacks/mariadb) — already the preferred copies.
//  2. EnvKit's per-version service folders (newest version wins).
//  3. Well-known roots other dev stacks use (plain C:\nginx,
//     C:\php, C:\mysql, Laragon, XAMPP).
//  4. Whatever is on the system PATH.
func DetectBinaries(appDataDir string) map[Service]string {
	envkitNginx := newestGlob(`C:\ProgramData\envkit\services\nginx\*\nginx.exe`)
	envkitPHP := newestGlob(`C:\ProgramData\envkit\services\php\*\php-cgi.exe`)
	envkitMySQL := newestGlob(`C:\ProgramData\envkit\services\mysql\*\bin\mysqld.exe`)

	candidates := map[Service][]string{
		ServiceNginx: {
			filepath.Join(NginxInstallDir(appDataDir), "nginx.exe"),
			envkitNginx,
			`C:\nginx\nginx.exe`,
			latestGlob(`C:\laragon\bin\nginx\*\nginx.exe`),
		},
		ServicePHP: {
			managedPHPCgi(appDataDir),
			envkitPHP,
			`C:\php\php-cgi.exe`,
			`C:\xampp\php\php-cgi.exe`,
			latestGlob(`C:\laragon\bin\php\*\php-cgi.exe`),
		},
		ServiceMySQL: {
			filepath.Join(MariaDBInstallDir(appDataDir), "bin", "mysqld.exe"),
			envkitMySQL,
			`C:\mysql\bin\mysqld.exe`,
			`C:\xampp\mysql\bin\mysqld.exe`,
			latestGlob(`C:\laragon\bin\mysql\*\bin\mysqld.exe`),
		},
	}

	lookups := map[Service]string{
		ServiceNginx: "nginx.exe",
		ServicePHP:   "php-cgi.exe",
		ServiceMySQL: "mysqld.exe",
	}

	out := make(map[Service]string, len(candidates))
	for service, paths := range candidates {
		for _, p := range paths {
			if p != "" && fileExists(p) {
				out[service] = p
				break
			}
		}
		if _, ok := out[service]; !ok {
			if found, err := exec.LookPath(lookups[service]); err == nil && found != "" {
				out[service] = found
			}
		}
	}
	return out
}

// managedPHPCgi returns the managed php-cgi path when the managed PHP
// install exists.
func managedPHPCgi(appDataDir string) string {
	p := filepath.Join(PHPInstallDir(appDataDir), "php-cgi.exe")
	if fileExists(p) {
		return p
	}
	return ""
}

// newestGlob returns the existing file matching pattern with the
// highest version-ish path segment, or "" when nothing matches.
func newestGlob(pattern string) string {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	existing := make([]string, 0, len(matches))
	for _, m := range matches {
		if fileExists(m) {
			existing = append(existing, m)
		}
	}
	if len(existing) == 0 {
		return ""
	}
	sort.Slice(existing, func(i, j int) bool {
		return compareVersionPaths(existing[i], existing[j]) > 0
	})
	return existing[0]
}

// latestGlob is newestGlob with an empty result allowed to pass
// through; it exists purely to keep the candidate lists readable.
func latestGlob(pattern string) string {
	return newestGlob(pattern)
}

// compareVersionPaths orders two paths by the version numbers embedded
// in them (e.g. ...\php\8.5.7\... vs ...\php\10.11.10\...). Numeric
// runs compare numerically, everything else lexically; a path with a
// higher numeric run wins ("10" > "8" which plain string sorting gets
// wrong). Equal-so-far falls back to plain string order for stability.
func compareVersionPaths(a, b string) int {
	ra := digitRuns(strings.ToLower(a))
	rb := digitRuns(strings.ToLower(b))
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			if ra[i] > rb[i] {
				return 1
			}
			return -1
		}
	}
	switch {
	case len(ra) != len(rb):
		if len(ra) > len(rb) {
			return 1
		}
		return -1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// digitRuns extracts the numeric runs of a string as integers:
// "php-8.5.7" → [8, 5, 7].
func digitRuns(s string) []int {
	var runs []int
	i := 0
	for i < len(s) {
		if s[i] < '0' || s[i] > '9' {
			i++
			continue
		}
		n := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int(s[i]-'0')
			i++
		}
		runs = append(runs, n)
	}
	return runs
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
