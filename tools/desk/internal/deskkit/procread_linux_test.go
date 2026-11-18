//go:build linux

package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHotfixReadProcIsProcfs: the /proc reader the namespace gate uses reads the kernel's
// uid_map, and refuses a file of the same content that is not on procfs.
func TestHotfixReadProcIsProcfs(t *testing.T) {
	if _, err := readProcFile(procUIDMap); err != nil {
		t.Fatalf("control: %s unreadable through readProcFile: %v", procUIDMap, err)
	}
	f := filepath.Join(t.TempDir(), "uid_map")
	if err := os.WriteFile(f, []byte("         0          0 4294967295\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readProcFile(f); err == nil || !strings.Contains(err.Error(), "not on procfs") {
		t.Fatalf("a non-procfs uid_map = %v, want a not-on-procfs refusal", err)
	}
}
