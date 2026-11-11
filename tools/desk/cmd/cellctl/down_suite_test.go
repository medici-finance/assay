//go:build darwin || linux

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDownShellSuite runs tools/cellctl/tests/down.test.sh against a binary built from this tree.
// No workflow names the suite, and run by hand without CELLCTL it meets the shell oracle, so its
// refusal assertions never held the Go binary to anything; two of them had gone stale against it
// unnoticed (#6148). This test is what makes the suite part of every `go test`.
//
// regression: #6148
func TestDownShellSuite(t *testing.T) {
	path := []string{}
	for _, tool := range []string{"bash", "git"} {
		found, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is not on PATH", tool)
		}
		path = append(path, filepath.Dir(found))
	}
	path = append(path, "/usr/bin", "/bin")
	suite, err := filepath.Abs(filepath.Join("..", "..", "..", "cellctl", "tests", "down.test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", suite)
	tmp, err := os.MkdirTemp("", "cellctl-down-suite-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })
	cmd.Env = []string{
		"PATH=" + strings.Join(path, string(os.PathListSeparator)),
		"CELLCTL=" + cellctlBinary(t), "TMPDIR=" + tmp, "HOME=" + tmp, "LC_ALL=C",
	}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "\ndown.test.sh: OK\n") {
		t.Fatalf("down.test.sh: %v\n%s", err, out)
	}
	if n := strings.Count(string(out), "\n  ok    "); n < 10 {
		t.Fatalf("down.test.sh ran only %d assertions:\n%s", n, out)
	}
}
