package stacks

import "testing"

// A php.ini limit is written straight into the generated file, so anything
// that is not a plain size must fall back instead of injecting directives.
func TestSanitizeIniSize(t *testing.T) {
	cases := []struct {
		in       string
		fallback string
		want     string
	}{
		{"256M", "256M", "256M"},
		{"512m", "256M", "512M"},
		{"2G", "256M", "2G"},
		{"1024K", "256M", "1024K"},
		{"", "64M", "64M"},
		{"  128M  ", "64M", "128M"},
		{"128M\nauto_prepend_file = C:/evil.php", "64M", "64M"},
		{"huge", "64M", "64M"},
		{"-1", "64M", "64M"},
	}
	for _, tc := range cases {
		if got := sanitizeIniSize(tc.in, tc.fallback); got != tc.want {
			t.Fatalf("sanitizeIniSize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
