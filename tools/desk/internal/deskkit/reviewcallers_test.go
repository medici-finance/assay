package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These acceptance entrypoints exercise the actual command and shipped shell
// adapters, not a second planner in the test package. All providers and homes
// are disposable. The source workflow runs the desk suite on every PR.
func TestReviewCallerFlow(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^TestReviewDriverParity$", "./internal/reviewtest")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestReviewDriverParity ") {
		t.Fatalf("production caller flow: %v\n%s", err, out)
	}
}

func TestReviewCallerRollback(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^TestReviewCallerFaults$", "./internal/reviewtest")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestReviewCallerFaults ") {
		t.Fatalf("caller failure qualification: %v\n%s", err, out)
	}
	// New producer / old recipe and old producer / new caller's legacy mode
	// preserve the existing full monitor behavior, including health and identity.
	script := filepath.Join("..", "..", "..", "..", "plugins", "assay", "scripts", "pr-monitor.test.sh")
	cmd = exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "PR_MONITOR_REVIEW_OBSERVATION=")
	out, err = cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "0 failed") {
		t.Fatalf("legacy rollback: %v\n%s", err, out)
	}
}
