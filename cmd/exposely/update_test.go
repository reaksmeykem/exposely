package main

import (
	"strings"
	"testing"
)

func TestBuildWindowsReplaceScriptWaitsForParentAndTargetsPaths(t *testing.T) {
	script := buildWindowsReplaceScript(4321, `C:\Users\User\go\bin\exposely.exe`, `C:\Users\User\go\bin\exposely.exe.download`)

	checks := []string{
		`$parentPID = 4321`,
		`Get-Process -Id $parentPID`,
		`Remove-Item -LiteralPath $target -Force`,
		`Move-Item -LiteralPath $downloaded -Destination $target -Force`,
		// Single-quoted literals: PowerShell must not expand `$` or treat a
		// backtick as an escape inside a path.
		`$target = 'C:\Users\User\go\bin\exposely.exe'`,
		`$downloaded = 'C:\Users\User\go\bin\exposely.exe.download'`,
	}

	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Fatalf("expected script to contain %q\nscript:\n%s", want, script)
		}
	}
}

func TestBuildWindowsReplaceScriptEscapesQuotesInPaths(t *testing.T) {
	script := buildWindowsReplaceScript(7, `C:\it's\exposely.exe`, `C:\it's\exposely.exe.download`)

	if !strings.Contains(script, `$target = 'C:\it''s\exposely.exe'`) {
		t.Fatalf("expected the embedded quote doubled, got %q", script)
	}
	if !strings.Contains(script, `$downloaded = 'C:\it''s\exposely.exe.download'`) {
		t.Fatalf("expected the embedded quote doubled, got %q", script)
	}
}
