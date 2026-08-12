package main

import (
	"os"
	"strings"
	"testing"
)

// launchFixtures copies the launch fixture tree, loads streams, wires Depends,
// and returns the transitive deps for the given target.
func launchFixtures(t *testing.T, target string) []launchDep {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/launch")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	attachPlaceholders(streams)
	checkBriefFiles(streams)
	deps, err := launchTransitiveClosure(streams, target)
	if err != nil {
		t.Fatal(err)
	}
	return deps
}

func TestLaunchTransitiveClosure(t *testing.T) {
	deps := launchFixtures(t, "gate/01")

	// Should find 3 transitive deps: alpha/01, alpha/02, alpha/03.
	if len(deps) != 3 {
		t.Fatalf("expected 3 transitive deps, got %d: %v", len(deps), deps)
	}

	// Verify each expected dep is present with correct status.
	want := map[string]string{
		"alpha/01": "done",
		"alpha/02": "implemented",
		"alpha/03": "todo",
	}
	for _, d := range deps {
		exp, ok := want[d.ID]
		if !ok {
			t.Errorf("unexpected dep %s", d.ID)
			continue
		}
		if d.Status != exp {
			t.Errorf("%s: expected status %q, got %q", d.ID, exp, d.Status)
		}
	}
	for id := range want {
		found := false
		for _, d := range deps {
			if d.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected dep %s", id)
		}
	}
}

func TestLaunchRenderVerdictBlocked(t *testing.T) {
	deps := launchFixtures(t, "gate/01")
	out := renderLaunchView("gate/01", deps)

	// Verdict should say BLOCKED.
	if !strings.Contains(out, "BLOCKED") {
		t.Errorf("expected BLOCKED in output, got:\n%s", out)
	}
	// Should name the two not-done deps.
	if !strings.Contains(out, "alpha/02") || !strings.Contains(out, "alpha/03") {
		t.Errorf("expected not-done deps alpha/02 and alpha/03 in output, got:\n%s", out)
	}
	// Should NOT say READY.
	if strings.Contains(out, "READY") {
		t.Errorf("output should not say READY when blocked, got:\n%s", out)
	}

	// Should print the disclaimer.
	if !strings.Contains(out, "Readiness per the board") {
		t.Errorf("expected disclaimer in output, got:\n%s", out)
	}
}

func TestLaunchRenderVerdictReady(t *testing.T) {
	// All deps done — verdict should say READY.
	deps := []launchDep{
		{ID: "alpha/01", Title: "First", Status: "done"},
		{ID: "alpha/02", Title: "Second", Status: "verified"},
	}
	out := renderLaunchView("gate/01", deps)
	if !strings.Contains(out, "READY") {
		t.Errorf("expected READY in output, got:\n%s", out)
	}
	if strings.Contains(out, "BLOCKED") {
		t.Errorf("expected no BLOCKED in output when ready, got:\n%s", out)
	}
}

func TestLaunchRenderVerdictNoDeps(t *testing.T) {
	out := renderLaunchView("gate/01", nil)
	if !strings.Contains(out, "READY") {
		t.Errorf("expected READY with no deps, got:\n%s", out)
	}
	if !strings.Contains(out, "nothing blocks the gate") {
		t.Errorf("expected 'nothing blocks the gate', got:\n%s", out)
	}
}

func TestLaunchStatusMark(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"done", "✅"},
		{"verified", "✅"},
		{"implemented", "⏳"},
		{"in-progress", "⏳"},
		{"todo", "❌"},
		{"blocked", "❌"},
		{"unknown", "❌"},
	}
	for _, tc := range tests {
		got := statusMark(tc.status)
		if got != tc.want {
			t.Errorf("statusMark(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestLaunchTargetNotFound(t *testing.T) {
	deps, err := launchTransitiveClosure(nil, "nonexistent/01")
	if err == nil {
		t.Errorf("expected error for nonexistent target, got deps: %v", deps)
	}
	if deps != nil {
		t.Errorf("expected nil deps on error")
	}
}

func TestLaunchCycleSafe(t *testing.T) {
	// Build an in-memory cycle: A depends on B, B depends on A.
	streams := []*Stream{{
		Name:   "loop",
		Status: "active",
		Briefs: []Brief{
			{Num: "01", Title: "A", Status: "todo", Depends: []string{"loop/02"}},
			{Num: "02", Title: "B", Status: "todo", Depends: []string{"loop/01"}},
		},
	}}
	deps, err := launchTransitiveClosure(streams, "loop/01")
	if err != nil {
		t.Fatal(err)
	}
	// Should have exactly one dep (loop/02), not loop forever.
	if len(deps) != 1 {
		t.Errorf("expected 1 dep for simple cycle, got %d: %v", len(deps), deps)
	}
	if deps[0].ID != "loop/02" {
		t.Errorf("expected dep loop/02, got %s", deps[0].ID)
	}
}
