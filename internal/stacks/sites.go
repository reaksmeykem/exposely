package stacks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// SiteRegistry persists the list of vhosts Exposely has generated for
// projects. The desktop app and CLI are separate processes, and the
// nginx.conf is fully regenerated on every stack start, so the site
// list must survive restarts. It lives at
// <appDataDir>/stacks/sites.json.
type SiteRegistry struct {
	path  string
	Sites []SiteEntry `json:"sites"`
}

// SiteEntry is one project vhost.
type SiteEntry struct {
	// ServerName is the *.test / *.local host (e.g. affinity-start.test).
	ServerName string `json:"serverName"`
	// ProjectPath is the project folder on disk.
	ProjectPath string `json:"projectPath"`
	// Root is the document root actually served (usually
	// <projectPath>/public for Laravel, the folder itself for static).
	Root string `json:"root"`
	// PHP true when the site is served through php-cgi.
	PHP bool `json:"php"`
	// UpdatedAt is the last time this entry was written (RFC3339).
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// LoadSiteRegistry reads sites.json; a missing file yields an empty
// registry (not an error) so first runs work out of the box.
func LoadSiteRegistry(appDataDir string) *SiteRegistry {
	r := &SiteRegistry{path: siteRegistryPath(appDataDir)}
	data, err := os.ReadFile(r.path)
	if err != nil {
		return r
	}
	_ = json.Unmarshal(data, r)
	return r
}

func siteRegistryPath(appDataDir string) string {
	return filepath.Join(appDataDir, "stacks", "sites.json")
}

// Upsert adds or replaces the entry for ServerName and persists the
// registry.
func (r *SiteRegistry) Upsert(entry SiteEntry) error {
	entry.ServerName = strings.ToLower(strings.TrimSpace(entry.ServerName))
	entry.UpdatedAt = nowRFC3339()
	replaced := false
	for i := range r.Sites {
		if r.Sites[i].ServerName == entry.ServerName {
			r.Sites[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		r.Sites = append(r.Sites, entry)
	}
	return r.save()
}

// Remove drops the entry for ServerName and persists.
func (r *SiteRegistry) Remove(serverName string) error {
	target := strings.ToLower(strings.TrimSpace(serverName))
	out := r.Sites[:0]
	for _, s := range r.Sites {
		if s.ServerName != target {
			out = append(out, s)
		}
	}
	r.Sites = out
	return r.save()
}

// Get returns the entry for a host, if registered.
func (r *SiteRegistry) Get(serverName string) (SiteEntry, bool) {
	target := strings.ToLower(strings.TrimSpace(serverName))
	for _, s := range r.Sites {
		if s.ServerName == target {
			return s, true
		}
	}
	return SiteEntry{}, false
}

func (r *SiteRegistry) save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}

// ResolveProjectRoot picks the document root for a project folder:
// <project>/public when it exists (Laravel-style front controller),
// otherwise the folder itself (static site).
func ResolveProjectRoot(projectPath string) string {
	publicDir := filepath.Join(projectPath, "public")
	if info, err := os.Stat(publicDir); err == nil && info.IsDir() {
		return publicDir
	}
	return projectPath
}

// IsPHPSite reports whether the document root contains PHP entry
// points (index.php or *.php files), deciding whether the site needs
// php-cgi behind nginx.
func IsPHPSite(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "index.php")); err == nil {
		return true
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	// Bounded scan: PHP presence in the top level is enough; deep
	// framework files (vendor/, storage/) would be false positives.
	checked := 0
	for _, e := range entries {
		if checked >= 100 {
			break
		}
		if e.IsDir() {
			continue
		}
		checked++
		if strings.EqualFold(filepath.Ext(e.Name()), ".php") {
			return true
		}
	}
	return false
}

// ReloadNginx asks a running nginx (started by Exposely with our
// generated conf) to reload its configuration without dropping
// connections. It uses the same -p/-c pair the conf was started with.
// A missing PID file or dead process returns an error so callers can
// fall back to a full start.
func ReloadNginx(nginxBinary, nginxRoot, confPath string) error {
	if strings.TrimSpace(nginxBinary) == "" {
		return fmt.Errorf("nginx binary path is empty")
	}
	cmd := exec.Command(nginxBinary, "-p", nginxRoot+string(filepath.Separator), "-c", confPath, "-s", "reload")
	cmd.Dir = nginxRoot
	cmd.SysProcAttr = sysproc.Hidden()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx reload failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// NginxConfigTest validates a conf without starting the server
// (`nginx -t`). Used before reload so a broken generated conf never
// kills a running site.
func NginxConfigTest(nginxBinary, nginxRoot, confPath string) error {
	cmd := exec.Command(nginxBinary, "-p", nginxRoot+string(filepath.Separator), "-c", confPath, "-t")
	cmd.Dir = nginxRoot
	cmd.SysProcAttr = sysproc.Hidden()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx config test failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
