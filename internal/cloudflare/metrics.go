package cloudflare

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/models"
)

// pickFreeLoopbackPort asks the OS for a free TCP port on 127.0.0.1.
// The listener is closed immediately; there is a tiny race window before
// cloudflared binds the same port, but in practice this is reliable.
func pickFreeLoopbackPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address type")
	}
	return addr.Port, nil
}

// parsedMetrics is the low-level shape produced by parsePrometheus.
// We only keep the few series we care about to avoid retaining everything.
type parsedMetrics struct {
	totalRequests   uint64
	haConnections   int64
	activeConns     int64
	responsesByCode map[string]uint64
	edgeLocations   map[string]struct{}
}

// labelRegex extracts prometheus label values from a `{k="v",...}` segment.
var labelRegex = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"`)

// parsePrometheus reads a cloudflared /metrics body and extracts the subset
// of series we surface as "Live Tunnel Usage". Unknown series are ignored.
//
// cloudflared metric names vary by version. We match by prefix and fall back
// gracefully when a counter is missing.
func parsePrometheus(r io.Reader) parsedMetrics {
	out := parsedMetrics{
		responsesByCode: map[string]uint64{},
		edgeLocations:   map[string]struct{}{},
	}

	scanner := bufio.NewScanner(r)
	// Allow long lines; some histograms can be wide.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, value, ok := splitMetricLine(line)
		if !ok {
			continue
		}

		switch {
		case name == "cloudflared_tunnel_total_requests" ||
			name == "cloudflared_tunnel_request_count":
			out.totalRequests += toUint(value)

		case name == "cloudflared_tunnel_ha_connections":
			out.haConnections = toInt(value)

		case name == "cloudflared_tunnel_active_streams" ||
			name == "cloudflared_tunnel_concurrent_requests_per_tunnel":
			if v := toInt(value); v > out.activeConns {
				out.activeConns = v
			}

		case name == "cloudflared_tunnel_response_by_code" ||
			name == "cloudflared_tunnel_responses_by_code":
			code := labels["status_code"]
			if code == "" {
				code = labels["code"]
			}
			if code != "" {
				out.responsesByCode[code] += toUint(value)
			}

		case name == "cloudflared_tunnel_server_locations":
			loc := labels["edge_location"]
			if loc == "" {
				loc = labels["location"]
			}
			if loc != "" && toInt(value) > 0 {
				out.edgeLocations[loc] = struct{}{}
			}
		}
	}

	return out
}

// splitMetricLine parses a single non-comment line in prometheus text format:
//
//	metric_name{k="v",k2="v2"} 12345 [timestamp]
//	metric_name 12345
func splitMetricLine(line string) (name string, labels map[string]string, value string, ok bool) {
	labels = map[string]string{}

	braceOpen := strings.IndexByte(line, '{')
	if braceOpen >= 0 {
		braceClose := strings.IndexByte(line, '}')
		if braceClose < braceOpen {
			return "", nil, "", false
		}
		name = strings.TrimSpace(line[:braceOpen])
		for _, m := range labelRegex.FindAllStringSubmatch(line[braceOpen+1:braceClose], -1) {
			labels[m[1]] = m[2]
		}
		rest := strings.TrimSpace(line[braceClose+1:])
		parts := strings.Fields(rest)
		if len(parts) == 0 {
			return "", nil, "", false
		}
		value = parts[0]
	} else {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return "", nil, "", false
		}
		name = parts[0]
		value = parts[1]
	}
	return name, labels, value, true
}

func toUint(s string) uint64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil && f >= 0 {
		return uint64(f)
	}
	return 0
}

func toInt(s string) int64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	return 0
}

// fetchUsage scrapes the given /metrics endpoint once and returns a usage
// snapshot enriched with uptime and derived fields (requests/min).
func fetchUsage(metricsAddr string, startedAt time.Time, prev *models.TunnelUsage) (*models.TunnelUsage, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	url := "http://" + metricsAddr + "/metrics"
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics endpoint returned %d", resp.StatusCode)
	}

	parsed := parsePrometheus(resp.Body)

	now := time.Now()
	uptime := int64(now.Sub(startedAt).Seconds())
	if uptime < 0 {
		uptime = 0
	}

	locations := make([]string, 0, len(parsed.edgeLocations))
	for loc := range parsed.edgeLocations {
		locations = append(locations, loc)
	}
	sort.Strings(locations)

	// Derive requests-per-minute from delta vs previous snapshot when we have one.
	var rpm float64
	if prev != nil && prev.LastUpdated != "" {
		if prevTime, err := time.Parse(time.RFC3339, prev.LastUpdated); err == nil {
			dt := now.Sub(prevTime).Seconds()
			if dt > 0 && parsed.totalRequests >= prev.TotalRequests {
				delta := float64(parsed.totalRequests - prev.TotalRequests)
				rpm = (delta / dt) * 60.0
			}
		}
	} else if uptime > 0 {
		rpm = (float64(parsed.totalRequests) / float64(uptime)) * 60.0
	}

	usage := &models.TunnelUsage{
		Available:       true,
		StartedAt:       startedAt.Format(time.RFC3339),
		UptimeSeconds:   uptime,
		TotalRequests:   parsed.totalRequests,
		RequestsPerMin:  rpm,
		ActiveConns:     parsed.activeConns,
		HAConnections:   parsed.haConnections,
		ResponsesByCode: parsed.responsesByCode,
		EdgeLocations:   locations,
		LastUpdated:     now.Format(time.RFC3339),
		MetricsAddr:     metricsAddr,
		Note:            "Live usage from local cloudflared. Resets when the tunnel restarts.",
	}
	return usage, nil
}
