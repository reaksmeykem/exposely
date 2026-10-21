//go:build !windows

package sysproc

import "errors"

// ProcessExecutablePath is only implemented on Windows; callers treat an
// error as "unknown process" and fall back to their conservative path.
func ProcessExecutablePath(pid int) (string, error) {
	return "", errors.New("process executable path is only available on Windows")
}
