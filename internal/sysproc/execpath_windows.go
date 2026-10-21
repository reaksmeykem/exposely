//go:build windows

package sysproc

import (
	"fmt"
	"syscall"
	"unsafe"
)

var queryFullProcessImageNameW = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryFullProcessImageNameW")

const processQueryLimitedInformation = 0x1000

// ProcessExecutablePath returns the full image path of a running process.
// Used to tell an orphaned Exposely-managed service (safe to adopt/stop)
// apart from an unrelated process that merely occupies the same port.
func ProcessExecutablePath(pid int) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("invalid pid %d", pid)
	}
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(handle)

	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	ok, _, callErr := queryFullProcessImageNameW.Call(
		uintptr(handle),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 {
		if callErr != nil && callErr.Error() != "The operation completed successfully." {
			return "", callErr
		}
		return "", fmt.Errorf("could not read executable path for pid %d", pid)
	}
	return syscall.UTF16ToString(buf[:size]), nil
}
