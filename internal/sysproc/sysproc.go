// Package sysproc centralises the Windows-specific SysProcAttr values used
// when spawning child processes so the rest of the codebase does not have to
// repeat platform-conditional struct literals (which break compilation on
// macOS because syscall.SysProcAttr on Darwin does not have HideWindow or
// CreationFlags fields).
package sysproc

import "syscall"

// Hidden returns a SysProcAttr that suppresses the console window when
// launching a child process on Windows. It returns nil on every other
// platform so callers can assign the result unconditionally:
//
//	cmd.SysProcAttr = sysproc.Hidden()
//
// The call site still works without a runtime.GOOS guard because the
// non-Windows implementation is a typed nil.
func Hidden() *syscall.SysProcAttr {
	return hiddenProcAttr()
}
