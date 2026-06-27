package cloudflare

import "testing"

func TestQuickTunnelArgsWithHostHeader(t *testing.T) {
	args := quickTunnelArgs("http://127.0.0.1:8080", "app.test", QuickTunnelOptions{})

	assertArgsEqual(t, args, []string{
		"tunnel",
		"--url", "http://127.0.0.1:8080",
		"--http-host-header", "app.test",
		"--edge-ip-version", "4",
	})
}

func TestQuickTunnelArgsForEnvKitLoopbackOrigin(t *testing.T) {
	// EnvKit serves *.test over HTTPS at 127.0.0.1:443. cloudflared must use
	// the host header as the SNI so the *.test cert validates during the
	// TLS handshake.
	args := quickTunnelArgs("https://127.0.0.1:443", "my-app.test", QuickTunnelOptions{})

	assertArgsEqual(t, args, []string{
		"tunnel",
		"--url", "https://127.0.0.1:443",
		"--http-host-header", "my-app.test",
		"--origin-server-name", "my-app.test",
		"--edge-ip-version", "4",
	})
}

func TestQuickTunnelArgsDoesNotAddOriginServerNameForHTTP(t *testing.T) {
	// Plain HTTP origins never need an SNI override, even with a host header.
	args := quickTunnelArgs("http://127.0.0.1:80", "my-app.test", QuickTunnelOptions{})

	for i, arg := range args {
		if arg == "--origin-server-name" {
			t.Fatalf("unexpected --origin-server-name for HTTP origin at index %d: %v", i, args)
		}
	}
}

func TestQuickTunnelArgsForLoopbackHTMLOrigin(t *testing.T) {
	args := quickTunnelArgs("http://127.0.0.1:9090", "", QuickTunnelOptions{})

	assertArgsEqual(t, args, []string{
		"tunnel",
		"--url", "http://127.0.0.1:9090",
		"--proxy-connect-timeout", "2s",
		"--edge-ip-version", "4",
	})
}

func TestQuickTunnelArgsDoNotUseUnsupportedFlags(t *testing.T) {
	args := quickTunnelArgs("http://127.0.0.1:9090", "", QuickTunnelOptions{})

	for _, arg := range args {
		if arg == "--origin-request-header" || arg == "--connect-timeout" {
			t.Fatalf("unexpected unsupported flag %q in %v", arg, args)
		}
	}
}

func TestQuickTunnelArgsAddsNoTLSVerifyWhenOptedIn(t *testing.T) {
	args := quickTunnelArgs("https://127.0.0.1:443", "kig.test", QuickTunnelOptions{InsecureSkipOriginTLS: true})

	assertArgsEqual(t, args, []string{
		"tunnel",
		"--url", "https://127.0.0.1:443",
		"--http-host-header", "kig.test",
		"--origin-server-name", "kig.test",
		"--no-tls-verify",
		"--edge-ip-version", "4",
	})
}

func TestQuickTunnelArgsOmitsNoTLSVerifyByDefault(t *testing.T) {
	args := quickTunnelArgs("https://127.0.0.1:443", "kig.test", QuickTunnelOptions{})

	for _, arg := range args {
		if arg == "--no-tls-verify" {
			t.Fatalf("did not expect --no-tls-verify with default options, got %v", args)
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
