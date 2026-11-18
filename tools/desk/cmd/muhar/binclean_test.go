package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestToolBinaryFixture is a compiled subprocess: it builds the tool binary the CLI
// tests share, then exits like any test process.
func TestToolBinaryFixture(t *testing.T) {
	if os.Getenv("MUHAR_BIN_FIXTURE") == "" {
		return
	}
	toolBinary(t)
}

// The shared tool binary's temp dir is removed when the test process exits, so
// repeated runs do not strand one built binary each in the temp dir.
//
// regression: #5560
func TestToolBinaryRemoved(t *testing.T) {
	tmp := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestToolBinaryFixture$", "-test.count=1")
	cmd.Env = append(os.Environ(), "MUHAR_BIN_FIXTURE=1", "TMPDIR="+tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	}
	left, err := filepath.Glob(filepath.Join(tmp, "muhar-cli-bin-*"))
	if err != nil || len(left) != 0 {
		t.Fatalf("test process stranded its tool binary dir: %v %v", left, err)
	}
}
