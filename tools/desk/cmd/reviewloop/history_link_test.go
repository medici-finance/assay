package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #2793
func TestPlanNoHistoryReader(t *testing.T) {
	// Link the shipped command, not this test binary (the producer controls in
	// history_test deliberately use the in-memory completed-write predicate).
	// Removing the reader dependency proves every key count avoids full-history
	// decoding, including corrupt, unreadable and missing historical segments.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "reviewloop.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build planner: %v %s", err, out)
	}
	symbols, err := exec.CommandContext(ctx, "go", "tool", "nm", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect planner dependencies: %v %s", err, symbols)
	}
	// The positive control proves nm read the intended, unstripped executable.
	if !strings.Contains(string(symbols), " main.cmdPlan") {
		t.Fatal("planner symbol missing")
	}
	for _, name := range []string{"LoadEntries", "loadEntriesFrom", "AlreadyDone", "AlreadyDoneIn"} {
		if strings.Contains(string(symbols), "/deskkit."+name) {
			t.Errorf("planner still links full-history dependency %s", name)
		}
	}
}
