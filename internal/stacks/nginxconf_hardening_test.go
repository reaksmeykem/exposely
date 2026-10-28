package stacks

import (
	"strings"
	"testing"
)

// The managed stack is a local dev environment exposed through a tunnel, so
// every generated site must stay on loopback and the PHP location must not
// hand the FastCGI worker a path it would walk back through (the classic
// nginx + cgi.fix_pathinfo execution trick).
func TestRenderNginxConfBindsLoopbackAndHardensPHP(t *testing.T) {
	conf := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{{
		ServerName: "app.test",
		Root:       `D:\code\app`,
		ListenPort: 8090,
		PHP:        true,
		PHPPort:    9000,
		Index:      []string{"index.php"},
	}})

	for _, want := range []string{
		"listen       127.0.0.1:8090;",
		"deny all;",
		"fastcgi_split_path_info ^(.+\\.php)(/.+)$;",
		"try_files $fastcgi_script_name =404;",
		"fastcgi_param  PATH_INFO",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("generated conf missing %q:\n%s", want, conf)
		}
	}

	// A bare (wildcard) listen would publish every site to the LAN.
	if strings.Contains(conf, "listen       8090;") {
		t.Fatalf("wildcard listen emitted:\n%s", conf)
	}
	if strings.Contains(conf, "listen       80;") {
		t.Fatalf("wildcard extra listen emitted:\n%s", conf)
	}
}
