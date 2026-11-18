//go:build linux

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestStatusgenReadProcIsProcfs: the /proc reader the namespace gate uses reads the kernel's
// uid_map, and refuses a file of the same content that is not on procfs.
func TestStatusgenReadProcIsProcfs(t *testing.T) {
	if _, err := sgReadProc(sgProcUIDMap); err != nil {
		t.Fatalf("control: %s unreadable through sgReadProc: %v", sgProcUIDMap, err)
	}
	f := filepath.Join(t.TempDir(), "uid_map")
	sgFixWrite(t, f, "         0          0 4294967295\n", 0o644)
	if _, err := sgReadProc(f); err == nil || !strings.Contains(err.Error(), "not on procfs") {
		t.Fatalf("a non-procfs uid_map = %v, want a not-on-procfs refusal", err)
	}
}
