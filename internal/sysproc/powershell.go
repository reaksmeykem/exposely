package sysproc

import "strings"

// PowerShellLiteral renders s as a single-quoted PowerShell string literal
// so paths handed to `powershell -Command` cannot break the script or be
// executed as code.
//
// Windows paths routinely contain characters that a double-quoted
// PowerShell string would interpret: `$` expands a variable (C:\$Recycle.Bin),
// a backtick is the escape character, and an embedded double quote ends the
// string early — which would let the rest of the path run as script. Single
// quotes disable all of that; the only character needing care is the single
// quote itself, which PowerShell escapes by doubling.
func PowerShellLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
