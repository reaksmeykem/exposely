package stacks

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func listenLocal() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func TestRenderNginxConfStaticSite(t *testing.T) {
	conf := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{{
		ServerName: "app.test",
		Root:       `D:\code\site\public`,
		ListenPort: 8090,
		Index:      []string{"index.html"},
	}})
	for _, want := range []string{
		"listen       8090;",
		"server_name  app.test;",
		`root         D:/code/site/public`,
		"include       C:/nginx/conf/mime.types;",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("generated conf missing %q:\n%s", want, conf)
		}
	}
	// Static site: no fastcgi_pass.
	if strings.Contains(conf, "fastcgi_pass") {
		t.Fatalf("static site must not contain fastcgi_pass:\n%s", conf)
	}
}

func TestRenderNginxConfPHPSite(t *testing.T) {
	conf := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{{
		ServerName: "laravel.test",
		Root:       `D:\code\hr-system\public`,
		PHP:        true,
		PHPPort:    9000,
		ListenPort: 8090,
	}})
	if !strings.Contains(conf, "fastcgi_pass   127.0.0.1:9000;") {
		t.Fatalf("php site missing fastcgi_pass:\n%s", conf)
	}
	if !strings.Contains(conf, "SCRIPT_FILENAME") {
		t.Fatalf("php site missing SCRIPT_FILENAME:\n%s", conf)
	}
}

func TestRenderNginxConfEmptySitesStillValid(t *testing.T) {
	conf := RenderNginxConf(`C:\nginx`, 8090, nil)
	if !strings.Contains(conf, "server {") {
		t.Fatalf("empty sites should still emit a default server block:\n%s", conf)
	}
}

func TestRenderNginxConfQuotesPathsWithSpaces(t *testing.T) {
	conf := RenderNginxConf(`C:\Program Files\nginx`, 8090, []SiteConfig{{
		ServerName: "x.test",
		Root:       `C:\My Sites\app`,
		ListenPort: 8090,
	}})
	if !strings.Contains(conf, `"C:/Program Files/nginx/conf/mime.types"`) {
		t.Fatalf("mime.types include should be quoted when path has spaces:\n%s", conf)
	}
	if !strings.Contains(conf, `"C:/My Sites/app"`) {
		t.Fatalf("root should be quoted when path has spaces:\n%s", conf)
	}
}

func TestWriteFileCreatesDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "nginx.conf")
	if err := WriteFile(path, "ok"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "ok" {
		t.Fatalf("written content mismatch: %v %q", err, string(content))
	}
}

func TestMySQLStartArgsDefaults(t *testing.T) {
	args := MySQLStartArgs(MySQLDefaults{BaseDir: `C:\mysql`, DataDir: `D:\data`})
	joined := strings.Join(args, " ")
	for _, want := range []string{"--console", "--basedir=C:/mysql", "--datadir=D:/data", "--port=3306", "--bind-address=127.0.0.1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("mysql args missing %q: %s", want, joined)
		}
	}
}

func TestPHPStartArgs(t *testing.T) {
	args := PHPStartArgs(9005, 4)
	if len(args) != 2 || args[0] != "-b" || args[1] != "127.0.0.1:9005" {
		t.Fatalf("unexpected php args: %v", args)
	}
	// Zero port falls back to 9000.
	args = PHPStartArgs(0, 1)
	if args[1] != "127.0.0.1:9000" {
		t.Fatalf("default php port should be 9000: %v", args)
	}
}

func TestPHPWorkerPorts(t *testing.T) {
	ports := PHPWorkerPorts(9000, 4)
	want := []int{9000, 9001, 9002, 9003}
	if len(ports) != len(want) {
		t.Fatalf("got %v want %v", ports, want)
	}
	for i := range want {
		if ports[i] != want[i] {
			t.Fatalf("got %v want %v", ports, want)
		}
	}
}

func TestWaitForPortInvalid(t *testing.T) {
	if err := WaitForPort("127.0.0.1", 0, 10*1000*1000); err == nil {
		t.Fatal("invalid port must error")
	}
	if err := WaitForPort("127.0.0.1", 99999, 10*1000*1000); err == nil {
		t.Fatal("out-of-range port must error")
	}
}

func TestWaitForPortReachable(t *testing.T) {
	// Bind a real listener so the dial succeeds immediately.
	ln, err := listenLocal()
	if err != nil {
		t.Skipf("cannot open listener: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if err := WaitForPort("127.0.0.1", port, 2*1000*1000*1000); err != nil {
		t.Fatalf("expected reachable port to succeed: %v", err)
	}
}
