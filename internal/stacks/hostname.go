package stacks

import (
	"fmt"
	"strings"
)

// maxHostLength is the DNS name limit (RFC 1035) plus a little slack for
// the wildcard prefix; a longer value is a mistake, not a hostname.
const maxHostLength = 253

// ValidateServerName reports whether host is safe to write into the
// generated nginx.conf as a `server_name` value.
//
// The value reaches the config file as bare text, so anything that could
// end the directive or open a new one — `;`, `{`, `}`, whitespace, a
// newline, a quote or a backslash — would turn a Local host field into
// extra nginx directives (a second server block, a different root). It
// also catches the harmless-but-confusing case of a host that simply
// makes nginx refuse to start.
//
// Hostnames, IPv4 literals, nginx wildcard names and an optional port are
// accepted: `app.test`, `my-app.local`, `127.0.0.1`, `*.test`,
// `app_test.local`, `app.test:8080`.
func ValidateServerName(host string) error {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return fmt.Errorf("local host is empty — set a host such as app.test")
	}
	if len(trimmed) > maxHostLength {
		return fmt.Errorf("local host is too long (%d characters, limit %d): %s", len(trimmed), maxHostLength, host)
	}
	if trimmed != host {
		return fmt.Errorf("local host must not start or end with whitespace: %q", host)
	}

	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == '*':
		// A port is harmless here and some setups name their host with one.
		case r == ':':
		default:
			return fmt.Errorf("local host %q contains %q, which is not allowed in a hostname — use letters, digits, dots, hyphens, underscores and an optional :port (for example app.test)", host, string(r))
		}
	}

	if strings.HasPrefix(trimmed, ".") || strings.HasSuffix(trimmed, ".") || strings.Contains(trimmed, "..") {
		return fmt.Errorf("local host %q has an empty label — use a host such as app.test", host)
	}
	return nil
}

// SafeServerName returns host when it is a valid server name, and
// "localhost" otherwise. Renderers use it so that a hand-edited
// sites.json can never inject directives into the generated config: an
// invalid entry falls back to a name that serves nothing unexpected.
func SafeServerName(host string) string {
	trimmed := strings.TrimSpace(host)
	if err := ValidateServerName(trimmed); err != nil {
		return "localhost"
	}
	return trimmed
}
