package stacks

import (
	"strings"
	"testing"
)

func TestValidateServerNameAcceptsHostnames(t *testing.T) {
	valid := []string{
		"app.test",
		"my-app.local",
		"App.Test",
		"127.0.0.1",
		"*.test",
		"app_test.local",
		"app.test:8080",
		"127.0.0.1:8000",
		"a",
		"sub.domain.example.com",
	}

	for _, host := range valid {
		if err := ValidateServerName(host); err != nil {
			t.Errorf("ValidateServerName(%q) = %v, want nil", host, err)
		}
	}
}

func TestValidateServerNameRejectsDirectiveInjection(t *testing.T) {
	invalid := []string{
		"",                        // empty
		"app.test; root C:/;",     // would open a new directive
		"app.test\n    root C:/;", // newline starts a directive
		"app.test { return 301 https://evil.test; }",
		"app.test}",  // closes the server block early
		"app test",   // whitespace splits the name list
		" app.test ", // stray whitespace
		`app\test`,
		`app"test`,
		"app/test",
		".test", // empty leading label
		"app..test",
		"app.test.",
		strings.Repeat("a", maxHostLength+1),
	}

	for _, host := range invalid {
		if err := ValidateServerName(host); err == nil {
			t.Errorf("ValidateServerName(%q) = nil, want an error", host)
		}
	}
}

func TestSafeServerNameFallsBackToLocalhost(t *testing.T) {
	if got := SafeServerName("app.test"); got != "app.test" {
		t.Fatalf("SafeServerName(app.test) = %q, want app.test", got)
	}
	if got := SafeServerName("  app.test  "); got != "app.test" {
		t.Fatalf("SafeServerName with padding = %q, want the trimmed host", got)
	}
	for _, host := range []string{"", "app.test; root C:/;", "app.test\n}"} {
		if got := SafeServerName(host); got != "localhost" {
			t.Errorf("SafeServerName(%q) = %q, want localhost", host, got)
		}
	}
}

func TestRenderNginxConfCannotBeInjectedThroughServerName(t *testing.T) {
	hostile := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{{
		ServerName: "app.test; root C:/Windows; }",
		Root:       `D:\code\site`,
		ListenPort: 8090,
	}})

	if !strings.Contains(hostile, "server_name  localhost;") {
		t.Fatalf("expected the invalid host to fall back to localhost:\n%s", hostile)
	}
	if strings.Contains(hostile, "root C:/Windows") || strings.Contains(hostile, "; }") {
		t.Fatalf("generated conf carries injected directives:\n%s", hostile)
	}

	// The invalid value must change nothing but the server name: compare the
	// result against a clean render of the same site.
	clean := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{{
		ServerName: "app.test",
		Root:       `D:\code\site`,
		ListenPort: 8090,
	}})
	want := strings.Replace(clean, "server_name  app.test;", "server_name  localhost;", 1)
	if hostile != want {
		t.Fatalf("hostile server name changed the generated config:\ngot:\n%s\nwant:\n%s", hostile, want)
	}
}

func TestSiteRegistryUpsertRejectsInvalidServerName(t *testing.T) {
	dir := t.TempDir()
	registry := LoadSiteRegistry(dir)

	err := registry.Upsert(SiteEntry{ServerName: "app.test; root C:/;", Root: `D:\code\site`})
	if err == nil {
		t.Fatal("Upsert accepted a server name carrying extra directives")
	}
	if len(registry.Sites) != 0 {
		t.Fatalf("registry kept %d entries after a rejected upsert, want 0", len(registry.Sites))
	}

	if err := registry.Upsert(SiteEntry{ServerName: "App.Test", Root: `D:\code\site`}); err != nil {
		t.Fatalf("Upsert rejected a valid host: %v", err)
	}
	if entry, ok := registry.Get("app.test"); !ok || entry.ServerName != "app.test" {
		t.Fatalf("expected the valid host to be registered lowercased, got %+v (found=%v)", entry, ok)
	}
}
