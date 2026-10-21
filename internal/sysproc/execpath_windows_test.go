//go:build windows

package sysproc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessExecutablePathOwnProcess(t *testing.T) {
	got, err := ProcessExecutablePath(os.Getpid())
	if err != nil {
		t.Fatalf("ProcessExecutablePath: %v", err)
	}
	want, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
		t.Fatalf("ProcessExecutablePath = %q, want %q", got, want)
	}
	if _, err := ProcessExecutablePath(0); err == nil {
		t.Fatal("ProcessExecutablePath(0) should fail")
	}
}
