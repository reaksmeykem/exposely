//go:build !windows

package sysproc

import "errors"

// ParentPID is only implemented on Windows; callers treat an error as
// "no known parent" and keep the process they started with.
func ParentPID(pid int) (int, error) {
	return 0, errors.New("parent process lookup is only available on Windows")
}
