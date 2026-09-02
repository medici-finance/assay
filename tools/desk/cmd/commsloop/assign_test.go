package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"github.com/medici-finance/assay/tools/desk/internal/runnertable"
)

// --- declared-source vs compiled diff ---------------------------------

// TestAssignCompiledMatchesSourceDiff reads assign.yaml, compiles it via
// LoadAssign, and compares the result against compiledAssign row by row,
// FAILING NAMING THE DIFFERENCE — the same derive-or-diff binding
// TestACLCompiledMatchesSourceDiff has to laneacl.yaml.
func TestAssignCompiledMatchesSourceDiff(t *testing.T) {
	data, err := os.ReadFile(assignSourceFile)
	if err != nil {
		t.Fatalf("read declared source %s: %v", assignSourceFile, err)
	}
	fromSource, err := LoadAssign(data)
	if err != nil {
		t.Fatalf("LoadAssign(%s): %v", assignSourceFile, err)
	}

	var diffs []string
	for k, wantTier := range fromSource {
		gotTier, ok := compiledAssign[k]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("compiledAssign is MISSING %+v (source says tier=%s)", k, tierName(wantTier)))
			continue
		}
		if gotTier != wantTier {
			diffs = append(diffs, fmt.Sprintf("%+v: compiledAssign has tier=%s, source has tier=%s", k, tierName(gotTier), tierName(wantTier)))
		}
	}
	for k, gotTier := range compiledAssign {
		if _, ok := fromSource[k]; !ok {
			diffs = append(diffs, fmt.Sprintf("compiledAssign has EXTRA row %+v (tier=%s) not in the source", k, tierName(gotTier)))
		}
	}
	if len(diffs) > 0 {
		sort.Strings(diffs)
		t.Fatalf("compiledAssign disagrees with %s (edit the source FIRST, then mirror it):\n  %s",
			assignSourceFile, strings.Join(diffs, "\n  "))
	}
}

// TestAssignSourceLoaderRejectsBadSchema pins the fail-closed schema check on
// the loader itself, independent of the live source file.
func TestAssignSourceLoaderRejectsBadSchema(t *testing.T) {
	_, err := LoadAssign([]byte("schema: assign-v0\nrows: []\n"))
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("a wrong schema must refuse, got: %v", err)
	}
}

// TestAssignSourceLoaderRejectsUnknownField proves KnownFields(true) is wired
// (a typo'd key is a refusal, never a silently-ignored field).
func TestAssignSourceLoaderRejectsUnknownField(t *testing.T) {
	_, err := LoadAssign([]byte("schema: assign-v1\nrows:\n  - {action: quarantine, class: routine, risk: \"no\", tier: human, extra: nope}\n"))
	if err == nil {
		t.Fatal("an unknown field in a row must refuse, not be silently ignored")
	}
}

// --- Assign() refusal tests --------------------------------------------

// TestAssignRefusesAbsentTriple: an empty action or class — the absent-triple
// case — refuses, never falls through to a default tier.
func TestAssignRefusesAbsentTriple(t *testing.T) {
	if _, err := Assign("", "routine", false); err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("an absent action must refuse (exit 5), got: %v", err)
	}
	if _, err := Assign("route-work-ready", "", false); err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("an absent class must refuse (exit 5), got: %v", err)
	}
}

// TestAssignRefusesUnknownAction: an action outside the closed vocabulary
// refuses, naming it as an unknown action (not a generic absent-triple).
func TestAssignRefusesUnknownAction(t *testing.T) {
	_, err := Assign("route-teleport", "routine", false)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("an unknown action must refuse naming the action, got: %v", err)
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("unknown action must map to exit 5, got code %d", deskkit.ExitCodeOf(err))
	}
}

// TestAssignRefusesUnknownClass: a class outside {routine, sensitive}
// refuses, naming it as an unknown class.
func TestAssignRefusesUnknownClass(t *testing.T) {
	_, err := Assign("route-work-ready", "urgent", false)
	if err == nil || !strings.Contains(err.Error(), "unknown class") {
		t.Fatalf("an unknown class must refuse naming the class, got: %v", err)
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("unknown class must map to exit 5, got code %d", deskkit.ExitCodeOf(err))
	}
}

// TestAssignResolvesKnownTriples spot-checks a handful of rows across the
// table's three shapes (dispatch-class default, class-scaled bookkeeping,
// always-human), and proves risk:true overrides every one of them to human.
func TestAssignResolvesKnownTriples(t *testing.T) {
	cases := []struct {
		action, class string
		risk          bool
		want          loopengine.Tier
	}{
		{"route-work-ready", "routine", false, loopengine.TierSession},
		{"route-work-ready", "sensitive", true, loopengine.TierHuman}, // risk overrides dispatch-class
		{"land-report", "routine", false, loopengine.TierLocal},
		{"land-report", "sensitive", false, loopengine.TierCheap},
		{"land-report", "routine", true, loopengine.TierHuman}, // risk overrides bookkeeping too
		{"escalate-human-issue", "sensitive", false, loopengine.TierHuman},
		{"quarantine", "routine", false, loopengine.TierHuman},
	}
	for _, c := range cases {
		got, err := Assign(c.action, c.class, c.risk)
		if err != nil {
			t.Fatalf("Assign(%q, %q, %v): %v", c.action, c.class, c.risk, err)
		}
		if got != c.want {
			t.Fatalf("Assign(%q, %q, %v) = %s, want %s", c.action, c.class, c.risk, tierName(got), tierName(c.want))
		}
	}
}

// --- human-tier-cannot-dispatch boundary assertion ----------------------

// TestAssignHumanTierNeverDispatches proves the boundary between this table
// and the runner table: a TierHuman assignment (e.g. escalate-human-issue)
// never resolves to a dispatchable runner. internal/runnertable.LoadRunnerTable
// already refuses a "human" runner KEY at load (TierHuman is non-dispatchable
// by construction there); this test proves the two surfaces agree — Assign
// may legally PRODUCE TierHuman, but no RunnerTable built from this module can
// ever RESOLVE it.
func TestAssignHumanTierNeverDispatches(t *testing.T) {
	tier, err := Assign("escalate-human-issue", "sensitive", false)
	if err != nil {
		t.Fatalf("Assign(escalate-human-issue, ...): %v", err)
	}
	if tier != loopengine.TierHuman {
		t.Fatalf("escalate-human-issue must assign TierHuman, got %s", tierName(tier))
	}

	tbl, err := runnertable.LoadRunnerTable(func(k string) string {
		if k == "ASSAY_RUNNER_LOCAL" {
			return `{"cmd":["x"],"pin":"1"}`
		}
		return ""
	}, nil)
	if err != nil {
		t.Fatalf("build a runner table: %v", err)
	}
	if _, err := tbl.Resolve(tier); err == nil {
		t.Fatal("Resolve(TierHuman) must refuse — a human-assigned tier must never resolve to a dispatchable runner")
	}
}

// --- end-to-end flow -----------------------------------------------------

// TestFlowActionToRunner drives the full chain the brief names: action ->
// class -> tier (Assign) -> pinned runner (internal/runnertable), in one
// test, for a dispatch-class action on the non-risk path.
func TestFlowActionToRunner(t *testing.T) {
	tier, err := Assign("route-work-ready", "routine", false)
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if tier != loopengine.TierSession {
		t.Fatalf("route-work-ready/routine/no-risk must assign TierSession, got %s", tierName(tier))
	}

	tbl, err := runnertable.LoadRunnerTable(func(k string) string {
		if k == "ASSAY_RUNNER_SESSION" {
			return `{"cmd":["codex-acp"],"model":"strong","pin":"1.2.0"}`
		}
		return ""
	}, nil)
	if err != nil {
		t.Fatalf("build runner table: %v", err)
	}
	runner, err := tbl.Resolve(tier)
	if err != nil {
		t.Fatalf("resolve pinned runner for %s: %v", tierName(tier), err)
	}
	if runner.Pin != "1.2.0" || runner.Model != "strong" {
		t.Fatalf("resolved runner mismatch: %+v", runner)
	}
	if id := runner.RunnerID(tier); !strings.HasPrefix(id, "acp:session:") || !strings.Contains(id, "1.2.0") {
		t.Fatalf("RunnerID not derived from the resolved entry: %q", id)
	}
}
