package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The lifecycle fixture matrix (derived-board/03): one top-level test per row of
// spec §2 (7 cells), plus the demotion cases and the offline arm. They are FLAT
// top-level functions on purpose — the brief's Verify row 1 counts them with
// `grep -c '^--- PASS'`, which matches only top-level test names (subtests print
// indented), so each row must be its own Test function.

// deriveSingle folds one brief and returns its cell, for the single-brief cases.
func deriveSingle(t *testing.T, b BriefIdent, in LifecycleInput) BriefCell {
	t.Helper()
	in.Briefs = []BriefIdent{b}
	cells := DeriveLifecycle(in)
	if len(cells) != 1 {
		t.Fatalf("want 1 cell, got %d", len(cells))
	}
	return cells[0]
}

// ---- 7 cells (spec §2) ----

func TestLifecycleCellTodo(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true})
	if got.Cell != "todo" {
		t.Fatalf("want todo, got %q", got.Cell)
	}
	// A todo is legal ONLY with evidence the search ran — never a bare default.
	if got.Source != "pr" || !strings.Contains(got.Reason, "PR search ran") {
		t.Errorf("todo must carry looked-and-found-nothing evidence; got source=%q reason=%q", got.Source, got.Reason)
	}
}

func TestLifecycleCellInProgress(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true, PRs: []PRRecord{
			{BriefRef: "s/01", Number: 12, State: prOpen, Draft: true, HeadSHA: "abcdef1234"},
		}})
	if got.Cell != "in-progress" {
		t.Fatalf("want in-progress, got %q", got.Cell)
	}
	if !strings.HasPrefix(got.Witness, "PR #12") || !strings.Contains(got.Witness, "draft") {
		t.Errorf("in-progress witness should name the open draft PR; got %q", got.Witness)
	}
}

func TestLifecycleCellImplemented(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true, PRs: []PRRecord{
			{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "0123456789abcdef"},
		}})
	if got.Cell != "implemented" {
		t.Fatalf("want implemented, got %q", got.Cell)
	}
	if !strings.HasPrefix(got.Witness, "PR #67 (merged 0123456") {
		t.Errorf("implemented witness should name the merged PR + merge SHA; got %q", got.Witness)
	}
}

func TestLifecycleCellImplementedLatestMerge(t *testing.T) {
	// Multiple merged PRs with the same trailer → implemented at the LATEST merge.
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true, PRs: []PRRecord{
			{BriefRef: "s/01", Number: 40, State: prMerged, MergeSHA: "aaaaaaa0000"},
			{BriefRef: "s/01", Number: 55, State: prMerged, MergeSHA: "bbbbbbb1111"},
		}})
	if got.Cell != "implemented" || !strings.HasPrefix(got.Witness, "PR #55") {
		t.Fatalf("want implemented at PR #55 (latest), got cell=%q witness=%q", got.Cell, got.Witness)
	}
}

func TestLifecycleCellVerified(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true,
			PRs:       []PRRecord{{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111"}},
			Witnesses: map[string]WitnessInfo{"s/01": {Passed: true, Version: 1}},
		})
	if got.Cell != "verified" {
		t.Fatalf("want verified, got %q", got.Cell)
	}
	if got.Source != "witness" {
		t.Errorf("verified source should be witness; got %q", got.Source)
	}
}

func TestLifecycleCellDone(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true,
			PRs:       []PRRecord{{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111"}},
			Witnesses: map[string]WitnessInfo{"s/01": {Passed: true, Version: 1}},
			Approvals: map[string]ApprovalInfo{"s/01": {Approved: true, AtHead: true}},
		})
	if got.Cell != "done" {
		t.Fatalf("want done, got %q", got.Cell)
	}
	// gate:human variant closes on a ruling, not an approval.
	human := deriveSingle(t, BriefIdent{ID: "h/02", Gate: "human", Version: 1},
		LifecycleInput{LookedAt: true,
			PRs:       []PRRecord{{BriefRef: "h/02", Number: 8, State: prMerged, MergeSHA: "cafef00d222"}},
			Witnesses: map[string]WitnessInfo{"h/02": {Passed: true, Version: 1}},
			Rulings:   map[string]bool{"h/02": true},
		})
	if human.Cell != "done" {
		t.Fatalf("gate:human want done, got %q", human.Cell)
	}
}

func TestLifecycleCellBlocked(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true,
			PRs:         []PRRecord{{BriefRef: "s/01", Number: 12, State: prOpen, HeadSHA: "abcdef1234"}},
			IssueLabels: map[string][]string{"s/01": {"needs-decision"}},
		})
	if got.Cell != "blocked" {
		t.Fatalf("want blocked, got %q", got.Cell)
	}
	// Blocked overlays in-progress/todo ONLY — a merged brief is never overlaid.
	notBlocked := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true,
			PRs:         []PRRecord{{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111"}},
			IssueLabels: map[string][]string{"s/01": {"needs-decision"}},
		})
	if notBlocked.Cell != "implemented" {
		t.Errorf("blocked must NOT overlay implemented; got %q", notBlocked.Cell)
	}
}

func TestLifecycleCellUnknown(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: false, Reason: "offline (--offline)",
			PRs: []PRRecord{{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111"}},
		})
	if got.Cell != "unknown" {
		t.Fatalf("want unknown, got %q", got.Cell)
	}
	if got.Source != "pr" || got.Reason == "" {
		t.Errorf("unknown must stay source=pr and carry a reason; got source=%q reason=%q", got.Source, got.Reason)
	}
}

// ---- demotions (spec §2 "demotion is automatic") ----

func TestLifecycleDemotionReopenedPR(t *testing.T) {
	// A merged PR reverted and reopened no longer witnesses `implemented`; the cell
	// falls back to the highest state still witnessed — in-progress (open again).
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true, PRs: []PRRecord{
			{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111", Reopened: true, HeadSHA: "feed1234567"},
		}})
	if got.Cell != "in-progress" {
		t.Fatalf("reverted/reopened merge should demote implemented→in-progress; got %q", got.Cell)
	}
}

func TestLifecycleDemotionRedWitness(t *testing.T) {
	// A verify witness that RAN and FAILED cannot claim verified; the cell falls
	// back to implemented (still witnessed by the merge).
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true,
			PRs:       []PRRecord{{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111"}},
			Witnesses: map[string]WitnessInfo{"s/01": {Passed: false, Version: 1}},
		})
	if got.Cell != "implemented" {
		t.Fatalf("red witness should demote verified→implemented; got %q", got.Cell)
	}
}

func TestLifecycleDemotionDismissedApproval(t *testing.T) {
	// A dismissed / stale App approval cannot close done; the cell falls back to
	// verified (the passing witness still stands).
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true,
			PRs:       []PRRecord{{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111"}},
			Witnesses: map[string]WitnessInfo{"s/01": {Passed: true, Version: 1}},
			Approvals: map[string]ApprovalInfo{"s/01": {Approved: false, AtHead: false}},
		})
	if got.Cell != "verified" {
		t.Fatalf("dismissed approval should demote done→verified; got %q", got.Cell)
	}
}

func TestLifecycleDemotionStaleVersion(t *testing.T) {
	// A witness run against an older version than the brief now carries renders
	// unknown with the reason (spec §5), instead of a false verified.
	got := deriveSingle(t, BriefIdent{ID: "s/01", Gate: "model", Version: 3},
		LifecycleInput{LookedAt: true,
			PRs:       []PRRecord{{BriefRef: "s/01", Number: 67, State: prMerged, MergeSHA: "deadbeef111"}},
			Witnesses: map[string]WitnessInfo{"s/01": {Passed: true, Version: 2}},
		})
	if got.Cell != "unknown" {
		t.Fatalf("stale-version witness should render unknown; got %q", got.Cell)
	}
	if got.Reason != "witness for v2, brief is v3" {
		t.Errorf("unknown reason should name both versions; got %q", got.Reason)
	}
}

// ---- offline arm ----

func TestLifecycleOffline(t *testing.T) {
	// When the instrument could not look, EVERY PR-derived cell is unknown — never
	// a silent todo. Determinism: JSON of the same input is byte-identical.
	in := LifecycleInput{LookedAt: false, Reason: "offline (--offline)", Briefs: []BriefIdent{
		{ID: "s/01", Gate: "model", Version: 1},
		{ID: "s/02", Gate: "human", Version: 1},
	}}
	cells := DeriveLifecycle(in)
	for _, c := range cells {
		if c.Cell != "unknown" || c.Source != "pr" {
			t.Fatalf("offline brief %s: want unknown/pr, got %q/%q", c.ID, c.Cell, c.Source)
		}
	}
	a, _ := json.Marshal(DeriveLifecycle(in))
	b, _ := json.Marshal(DeriveLifecycle(in))
	if string(a) != string(b) {
		t.Errorf("derivation must be deterministic; two runs differ")
	}
}
