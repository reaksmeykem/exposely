//go:build !windows

package sysproc

import "syscall"

func hiddenProcAttr() *syscall.SysProcAttr {
	return nil
}
