package stacks

import (
	"strings"
	"testing"
)

func TestRenderNginxConfEmitsExtraListenPorts(t *testing.T) {
	conf := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{{
		ServerName:       "app.test",
		Root:             `D:\www\app`,
		ListenPort:       8090,
		ExtraListenPorts: []int{80},
		PHP:              true,
		PHPPort:          9000,
	}})
	if !strings.Contains(conf, "listen       8090;") || !strings.Contains(conf, "listen       80;") {
		t.Fatalf("expected both listen directives:\n%s", conf)
	}
}

func TestRenderNginxConfSkipsDuplicateExtraPort(t *testing.T) {
	conf := RenderNginxConf(`C:\nginx`, 8090, []SiteConfig{{
		ServerName:       "app.test",
		Root:             `D:\www\app`,
		ListenPort:       8090,
		ExtraListenPorts: []int{8090, 0, -1},
	}})
	if strings.Count(conf, "listen       8090;") != 1 {
		t.Fatalf("duplicate/invalid extra ports must be skipped:\n%s", conf)
	}
	if strings.Contains(conf, "listen       0;") || strings.Contains(conf, "listen       -1;") {
		t.Fatalf("invalid ports must not be emitted:\n%s", conf)
	}
}

func TestPortAvailableRejectsInvalid(t *testing.T) {
	if PortAvailable(0) || PortAvailable(-1) || PortAvailable(70000) {
		t.Fatal("invalid ports must report unavailable")
	}
}
