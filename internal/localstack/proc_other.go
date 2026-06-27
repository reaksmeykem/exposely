//go:build !windows

package localstack

import "syscall"

// hiddenProcAttr is a no-op on non-Windows platforms. The caller sites live in
// Windows-only code paths, but Go still needs a symbol to resolve on every
// platform so we provide an inert implementation here.
func hiddenProcAttr() *syscall.SysProcAttr {
	return nil
}
