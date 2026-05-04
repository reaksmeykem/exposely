package cloudflare

import "testing"

func TestQuickTunnelArgsWithHostHeader(t *testing.T) {
	args := quickTunnelArgs("http://127.0.0.1:8080", "app.test")

	assertArgsEqual(t, args, []string{
		"tunnel",
		"--url", "http://127.0.0.1:8080",
		"--http-host-header", "app.test",
	})
}

func TestQuickTunnelArgsForLoopbackHTMLOrigin(t *testing.T) {
	args := quickTunnelArgs("http://127.0.0.1:9090", "")

	assertArgsEqual(t, args, []string{
		"tunnel",
		"--url", "http://127.0.0.1:9090",
		"--proxy-connect-timeout", "2s",
		"--edge-ip-version", "4",
	})
}

func TestQuickTunnelArgsDoNotUseUnsupportedFlags(t *testing.T) {
	args := quickTunnelArgs("http://127.0.0.1:9090", "")

	for _, arg := range args {
		if arg == "--origin-request-header" || arg == "--connect-timeout" {
			t.Fatalf("unexpected unsupported flag %q in %v", arg, args)
		}
	}
}

func assertArgsEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unexpected arg length: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected arg at index %d: got %q, want %q; full args=%v", i, got[i], want[i], got)
		}
	}
}
