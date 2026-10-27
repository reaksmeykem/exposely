//go:build windows

package sysproc

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	createToolhelp32Snapshot = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateToolhelp32Snapshot")
	process32First           = syscall.NewLazyDLL("kernel32.dll").NewProc("Process32FirstW")
	process32Next            = syscall.NewLazyDLL("kernel32.dll").NewProc("Process32NextW")
)

const (
	th32csSnapProcess   = 0x00000002
	invalidHandleValue  = ^uintptr(0)
	processEntry32WSize = 568
)

// processEntry32W mirrors PROCESSENTRY32W. The struct is fixed-size in the
// Win32 API, so the byte size is asserted rather than derived.
type processEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [syscall.MAX_PATH]uint16
}

// ParentPID returns the process id that started pid.
//
// Used to walk up from a child that merely holds a socket to the process
// that owns the service's life cycle: nginx's worker owns the listen
// sockets while its master owns start/stop, and only the master's death
// stops the service.
func ParentPID(pid int) (int, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid %d", pid)
	}

	snapshot, _, callErr := createToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == invalidHandleValue {
		if callErr != nil && callErr.Error() != "The operation completed successfully." {
			return 0, callErr
		}
		return 0, fmt.Errorf("could not snapshot processes")
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ok, _, _ := process32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		if int(entry.ProcessID) == pid {
			parent := int(entry.ParentProcessID)
			if parent <= 0 {
				return 0, fmt.Errorf("pid %d has no parent", pid)
			}
			return parent, nil
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		ok, _, _ = process32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return 0, fmt.Errorf("pid %d is not running", pid)
}
