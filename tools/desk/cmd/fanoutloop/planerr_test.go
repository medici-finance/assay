package main

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// runCapturingStderr runs fn with os.Stderr redirected to a pipe and returns what it wrote.
func runCapturingStderr(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	// Restored by defer, so a panic in fn never leaves stderr redirected for later tests.
	defer func() { os.Stderr = old }()
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	code := fn()
	_ = w.Close()
	return code, <-done
}

// regression: #5029
func TestPlanPrintsUnreadableBoard(t *testing.T) {
	setupDeskHome(t)
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// No STATUS.md on refs/remotes/origin/main and none in the working tree.
	code, stderr := runCapturingStderr(t, func() int { return run([]string{"plan", "--root", root}) })
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable); stderr:\n%s", code, deskkit.ExitUnverifiable, stderr)
	}
	if !strings.Contains(stderr, "cannot read the Next-up queue") {
		t.Fatalf("stderr does not name the refusal; got:\n%s", stderr)
	}
	if n := strings.Count(stderr, "cannot read the Next-up queue"); n != 1 {
		t.Fatalf("refusal printed %d times, want once; stderr:\n%s", n, stderr)
	}
}
