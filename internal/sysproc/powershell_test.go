package sysproc

import (
	"strings"
	"testing"
)

func TestPowerShellLiteralKeepsPathsLiteral(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"plain path", `C:\Users\me\AppData\Local\Exposely.exe`, `'C:\Users\me\AppData\Local\Exposely.exe'`},
		{"dollar is not a variable", `C:\$Recycle.Bin\Exposely.exe`, `'C:\$Recycle.Bin\Exposely.exe'`},
		{"backtick is not an escape", "C:\\odd`name\\Exposely.exe", "'C:\\odd`name\\Exposely.exe'"},
		{"double quote stays inside", `C:\we"ird\Exposely.exe`, `'C:\we"ird\Exposely.exe'`},
		{"single quote is doubled", `C:\it's\Exposely.exe`, `'C:\it''s\Exposely.exe'`},
		{"space stays quoted", `C:\Program Files\Exposely.exe`, `'C:\Program Files\Exposely.exe'`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PowerShellLiteral(tc.path)
			if got != tc.want {
				t.Fatalf("PowerShellLiteral(%q) = %s, want %s", tc.path, got, tc.want)
			}
			// A double-quoted literal would let PowerShell expand `$` and
			// treat a backtick as an escape, so never emit one.
			if strings.HasPrefix(got, `"`) || !strings.HasPrefix(got, "'") {
				t.Fatalf("PowerShellLiteral(%q) = %s, want a single-quoted literal", tc.path, got)
			}
		})
	}
}

func TestPowerShellLiteralBalancesQuotesForInjectionAttempts(t *testing.T) {
	// A crafted path must not be able to close the literal and start a new
	// statement: every single quote is doubled, so no lone quote survives
	// inside the literal.
	hostile := `C:\x'; Remove-Item -Recurse -Force C:\ ; '`
	got := PowerShellLiteral(hostile)

	// Removing every doubled quote must leave exactly the two wrapping
	// quotes — PowerShell would otherwise end the string at the first lone
	// quote and run the rest of the path as code.
	interior := strings.ReplaceAll(got, "''", "")
	if strings.Count(interior, "'") != 2 {
		t.Fatalf("PowerShellLiteral(%q) = %s, leaves an unescaped quote inside the literal", hostile, got)
	}
	if !strings.HasPrefix(interior, "'") || !strings.HasSuffix(interior, "'") {
		t.Fatalf("PowerShellLiteral(%q) = %s, want a single-quoted literal", hostile, got)
	}
}
