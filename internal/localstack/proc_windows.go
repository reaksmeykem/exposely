//go:build windows

package localstack

import "syscall"

// hiddenProcAttr returns the SysProcAttr needed to keep a child process from
// flashing a console window on Windows. The shared runCapture helper in the
// cloudflare package does the same thing for cloudflared — we re-implement it
// here so this package does not depend on internal/cloudflare (which would be
// an awkward dependency direction).
func hiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
