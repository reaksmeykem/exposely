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
		`C:\\Users\\User\\go\\bin\\exposely.exe`,
		`C:\\Users\\User\\go\\bin\\exposely.exe.download`,
	}

	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Fatalf("expected script to contain %q\nscript:\n%s", want, script)
		}
	}
}
