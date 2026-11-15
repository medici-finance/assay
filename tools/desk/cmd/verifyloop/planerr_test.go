package main

import (
	"io"
	"os"
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
func TestPlanPrintsReturnedRefusal(t *testing.T) {
	plantFixtureRoster(t)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "verify-desk")
	a, b := twoRoots(t)
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+a+",example-org/agents="+b)
	stubPreflight(t, func(string) error { return nil })
	// Multi-root plan with --sha: buildPlan RETURNS a refusal and prints nothing.
	code, stderr := runCapturingStderr(t, func() int { return run([]string{"plan", "--sha", "abc123"}) })
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (refused); stderr:\n%s", code, deskkit.ExitRefused, stderr)
	}
	const want = "--sha names ONE merged-main SHA"
	if n := strings.Count(stderr, want); n != 1 {
		t.Fatalf("refusal printed %d times, want once; stderr:\n%s", n, stderr)
	}
}
