package stacks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiteRegistryUpsertGetRemove(t *testing.T) {
	dir := t.TempDir()
	r := LoadSiteRegistry(dir)
	if len(r.Sites) != 0 {
		t.Fatal("fresh registry should be empty")
	}

	if err := r.Upsert(SiteEntry{ServerName: "App.Test", ProjectPath: `D:\p\a`, Root: `D:\p\a\public`, PHP: true}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	entry, ok := r.Get("app.test")
	if !ok || entry.Root != `D:\p\a\public` {
		t.Fatalf("get after upsert: %+v ok=%v", entry, ok)
	}

	// Update in place.
	if err := r.Upsert(SiteEntry{ServerName: "app.test", ProjectPath: `D:\p\a`, Root: `D:\p\a\public_new`, PHP: false}); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	r2 := LoadSiteRegistry(dir)
	if len(r2.Sites) != 1 {
		t.Fatalf("expected 1 site after replace, got %d", len(r2.Sites))
	}
	if r2.Sites[0].Root != `D:\p\a\public_new` || r2.Sites[0].PHP {
		t.Fatalf("replace mismatch: %+v", r2.Sites[0])
	}

	if err := r2.Remove("app.test"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := LoadSiteRegistry(dir).Get("app.test"); ok {
		t.Fatal("entry should be gone after remove")
	}
}

func TestResolveProjectRootPrefersPublic(t *testing.T) {
	dir := t.TempDir()
	if got := ResolveProjectRoot(dir); got != dir {
		t.Fatalf("plain folder should resolve to itself, got %q", got)
	}
	publicDir := filepath.Join(dir, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ResolveProjectRoot(dir); got != publicDir {
		t.Fatalf("Laravel folder should resolve to public, got %q", got)
	}
}

func TestIsPHPSite(t *testing.T) {
	dir := t.TempDir()
	if IsPHPSite(dir) {
		t.Fatal("empty dir is not a PHP site")
	}
	if err := os.WriteFile(filepath.Join(dir, "index.php"), []byte("<?php echo 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsPHPSite(dir) {
		t.Fatal("dir with index.php should be a PHP site")
	}

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsPHPSite(staticDir) {
		t.Fatal("static dir must not be detected as PHP")
	}
}

func TestNginxConfigTestRejectsBrokenConf(t *testing.T) {
	bin, root := findNginxForTest(t)
	if bin == "" {
		t.Skip("no nginx available for config-test test")
	}
	bad := filepath.Join(t.TempDir(), "bad.conf")
	if err := WriteFile(bad, "this is not nginx config {{{"); err != nil {
		t.Fatal(err)
	}
	if err := NginxConfigTest(bin, root, bad); err == nil {
		t.Fatal("broken conf must fail nginx -t")
	}
}

// findNginxForTest locates a real nginx binary for integration-flavoured
// tests, returning empty strings when none is available.
func findNginxForTest(t *testing.T) (string, string) {
	t.Helper()
	candidates := []string{
		`C:\ProgramData\envkit\services\nginx\1.26.2\nginx.exe`,
		`C:\nginx\nginx.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, filepath.Dir(c)
		}
	}
	return "", ""
}

func TestRenderNginxConfMultipleVHostsSamePort(t *testing.T) {
	conf := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{
		{ServerName: "a.test", Root: `D:\a\public`, PHP: true, PHPPort: 9005, ListenPort: 8090},
		{ServerName: "b.test", Root: `D:\b`, ListenPort: 8090},
	})
	count := strings.Count(conf, "server {")
	if count != 2 {
		t.Fatalf("expected 2 server blocks, got %d:\n%s", count, conf)
	}
	if !strings.Contains(conf, "server_name  a.test;") || !strings.Contains(conf, "server_name  b.test;") {
		t.Fatalf("both vhosts must be present:\n%s", conf)
	}
}

func TestPhpMyAdminServerConfig(t *testing.T) {
	conf := PhpMyAdminServerConfig("127.0.0.1", 3306)
	for _, want := range []string{"'127.0.0.1'", "'3306'", "AllowNoPassword"} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config missing %q:\n%s", want, conf)
		}
	}
}

func TestDetectPhpMyAdminNotFoundInCleanEnv(t *testing.T) {
	// On CI machines without any phpMyAdmin this must return false, not
	// panic. On the dev machine with EnvKit it may legitimately find one;
	// both outcomes are acceptable - the contract is "no panic".
	_, _ = DetectPhpMyAdmin()
}
