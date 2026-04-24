package cloudflare

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleMetrics = `# HELP cloudflared_tunnel_total_requests Total requests
# TYPE cloudflared_tunnel_total_requests counter
cloudflared_tunnel_total_requests 1284
# HELP cloudflared_tunnel_ha_connections Active HA connections
# TYPE cloudflared_tunnel_ha_connections gauge
cloudflared_tunnel_ha_connections 4
# HELP cloudflared_tunnel_concurrent_requests_per_tunnel gauge
cloudflared_tunnel_concurrent_requests_per_tunnel 3
cloudflared_tunnel_response_by_code{status_code="200"} 1180
cloudflared_tunnel_response_by_code{status_code="404"} 98
cloudflared_tunnel_response_by_code{status_code="500"} 6
cloudflared_tunnel_server_locations{edge_location="sin01"} 1
cloudflared_tunnel_server_locations{edge_location="nrt08"} 0
unrelated_metric{foo="bar"} 42
`

func TestParsePrometheus(t *testing.T) {
	p := parsePrometheus(strings.NewReader(sampleMetrics))

	if p.totalRequests != 1284 {
		t.Fatalf("totalRequests: got %d want 1284", p.totalRequests)
	}
	if p.haConnections != 4 {
		t.Fatalf("haConnections: got %d want 4", p.haConnections)
	}
	if p.activeConns != 3 {
		t.Fatalf("activeConns: got %d want 3", p.activeConns)
	}
	if p.responsesByCode["200"] != 1180 || p.responsesByCode["404"] != 98 || p.responsesByCode["500"] != 6 {
		t.Fatalf("responsesByCode wrong: %+v", p.responsesByCode)
	}
	if _, ok := p.edgeLocations["sin01"]; !ok {
		t.Fatalf("expected sin01 edge location, got %+v", p.edgeLocations)
	}
	if _, ok := p.edgeLocations["nrt08"]; ok {
		t.Fatalf("nrt08 had value 0 and should be excluded")
	}
}

func TestSplitMetricLine(t *testing.T) {
	cases := []struct {
		line   string
		name   string
		value  string
		labels map[string]string
		ok     bool
	}{
		{`foo 12`, "foo", "12", map[string]string{}, true},
		{`foo{a="b"} 3.14`, "foo", "3.14", map[string]string{"a": "b"}, true},
		{`foo{a="b",c="d"} 1 1700000000`, "foo", "1", map[string]string{"a": "b", "c": "d"}, true},
		{`# a comment`, "", "", nil, false},
	}
	for _, c := range cases {
		name, labels, value, ok := splitMetricLine(c.line)
		if c.ok {
			if !ok || name != c.name || value != c.value {
				t.Errorf("line %q: got name=%q value=%q ok=%v", c.line, name, value, ok)
				continue
			}
			for k, v := range c.labels {
				if labels[k] != v {
					t.Errorf("line %q: label %s=%q want %q", c.line, k, labels[k], v)
				}
			}
		} else if ok && !strings.HasPrefix(c.line, "#") {
			t.Errorf("line %q: expected !ok", c.line)
		}
	}
}

func TestFetchUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(sampleMetrics))
	}))
	defer srv.Close()

	// strip scheme and trailing slash so we get a bare host:port
	addr := strings.TrimPrefix(srv.URL, "http://")

	started := time.Now().Add(-60 * time.Second)
	usage, err := fetchUsage(addr, started, nil)
	if err != nil {
		t.Fatalf("fetchUsage: %v", err)
	}
	if !usage.Available {
		t.Fatalf("expected Available=true")
	}
	if usage.TotalRequests != 1284 {
		t.Fatalf("TotalRequests: got %d want 1284", usage.TotalRequests)
	}
	if usage.HAConnections != 4 {
		t.Fatalf("HAConnections: got %d want 4", usage.HAConnections)
	}
	if usage.UptimeSeconds < 59 || usage.UptimeSeconds > 62 {
		t.Fatalf("UptimeSeconds should be ~60, got %d", usage.UptimeSeconds)
	}
	// With no previous snapshot, rpm is derived from total/uptime
	if usage.RequestsPerMin <= 0 {
		t.Fatalf("expected RequestsPerMin > 0, got %f", usage.RequestsPerMin)
	}
	if len(usage.EdgeLocations) != 1 || usage.EdgeLocations[0] != "sin01" {
		t.Fatalf("EdgeLocations wrong: %+v", usage.EdgeLocations)
	}
}

func TestPickFreeLoopbackPort(t *testing.T) {
	port, err := pickFreeLoopbackPort()
	if err != nil {
		t.Fatalf("pickFreeLoopbackPort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("invalid port: %d", port)
	}
	// Port must be bindable right after we pick it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not bind follow-up listener: %v", err)
	}
	_ = ln.Close()
}
