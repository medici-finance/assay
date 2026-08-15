package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// TestPlan_QuietBoardExitsZero is the positive control for the exit-code contract below:
// a fresh, complete, quiet board is a successful run.
func TestPlan_QuietBoardExitsZero(t *testing.T) {
	quiet := strings.Replace(goodActions, `"rows": [
    {"repo": "example-org/example-repo", "number": 42, "title": "t", "action": "NEEDS-REVIEW", "score": 9, "note": "n"},
    {"repo": "example-org/example-repo", "number": 43, "title": "u", "action": "READY", "score": 2, "note": "n"}
  ]`, `"rows": []`, 1)
	var sb strings.Builder
	err := cmdPlan([]string{"--actions", writeTemp(t, "a.json", quiet), "--now", fixedNow}, &sb)
	if err != nil {
		t.Fatalf("quiet board = %v, want a clean run", err)
	}
	if !strings.Contains(sb.String(), "IDLE — fresh board") {
		t.Fatalf("plan output does not state the idle verdict:\n%s", sb.String())
	}
}

// TestPlan_CouldNotCheckExitsSix — a run whose idle question the board could not answer
// must NOT exit 0. Anything scripting this tool would otherwise read rc=0 as all-clear,
// which is the #79 shape one layer up from the gate itself.
func TestPlan_CouldNotCheckExitsSix(t *testing.T) {
	// Same payload, aged past the cadence: the ONLY difference from the control above.
	stale := strings.Replace(goodActions, `"rows": [
    {"repo": "example-org/example-repo", "number": 42, "title": "t", "action": "NEEDS-REVIEW", "score": 9, "note": "n"},
    {"repo": "example-org/example-repo", "number": 43, "title": "u", "action": "READY", "score": 2, "note": "n"}
  ]`, `"rows": []`, 1)
	err := cmdPlan([]string{"--actions", writeTemp(t, "a.json", stale), "--now", "2026-08-13T23:00:00Z"}, io.Discard)
	if err == nil {
		t.Fatal("a board too old to answer the idle question exited 0 — rc=0 would be read as all-clear")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (unverifiable)", got, deskkit.ExitUnverifiable)
	}
}

// TestPlan_RefusesWithNoBoard — a reactor invoked against no sweep at all is blind, and
// refusing (exit 5) is the only honest answer.
func TestPlan_RefusesWithNoBoard(t *testing.T) {
	err := cmdPlan(nil, io.Discard)
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitRefused {
		t.Fatalf("exit code = %d, want %d (refused)", got, deskkit.ExitRefused)
	}
}

// TestPlan_StatesItsUnreadSurfacesEveryRun — no silent caps. The roster is printed on
// every plan, not behind a flag.
func TestPlan_StatesItsUnreadSurfacesEveryRun(t *testing.T) {
	var sb strings.Builder
	_ = cmdPlan([]string{"--actions", writeTemp(t, "a.json", goodActions), "--prs", writeTemp(t, "p.json", goodPRs), "--now", fixedNow}, &sb)
	out := sb.String()
	if !strings.Contains(out, "BOARD SURFACES THIS REACTOR DOES NOT READ") {
		t.Fatalf("plan output does not state what the reactor is blind to:\n%s", out)
	}
	for _, want := range []string{"tombstones", "external", "loopengine claims dir", "issue-loop"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the unread-surface roster does not name %q", want)
		}
	}
}

// TestPlan_NeverMerges — the output must state the merge boundary, and must never emit a
// merge verb.
func TestPlan_NeverMerges(t *testing.T) {
	var sb strings.Builder
	_ = cmdPlan([]string{"--actions", writeTemp(t, "a.json", goodActions), "--now", fixedNow}, &sb)
	out := sb.String()
	if !strings.Contains(out, "the desk NEVER merges") {
		t.Fatalf("plan output does not state the merge boundary:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "emit") && strings.Contains(line, ":merge") {
			t.Fatalf("plan emitted a merge verb: %s", line)
		}
	}
}

// TestDispositionZeroValueIsSafe — the zero value of the enum is what a bug hands you.
func TestDispositionZeroValueIsSafe(t *testing.T) {
	var d Disposition
	if d != DispositionUnknown {
		t.Fatalf("zero-value Disposition = %s, want UNKNOWN-DISPOSITION", d)
	}
	if d.Actionable() {
		t.Fatal("the zero-value Disposition reports Actionable() — an unpopulated rule would consume a reviewer slot")
	}
	var r rule
	if r.Verb != "" {
		t.Fatalf("zero-value rule carries verb %q — an unpopulated rule would make an outward write", r.Verb)
	}
}
