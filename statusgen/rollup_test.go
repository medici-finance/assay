package main

import (
	"strings"
	"testing"
)

// Test names stay under 42 characters (the pre-push secret-scan floor).

// ---------- the three-state verdict (rule 16 negative paths) ----------

// TestRollupStateThreeState pins the honest three-state verdict, including the two
// silent-pass traps the brief calls out: an empty backing set must NOT be
// satisfied (vacuous "all of none are done"), and an unresolved backing brief must
// NOT round up to satisfied.
func TestRollupStateThreeState(t *testing.T) {
	// Empty backing set: partial, never a vacuous satisfied.
	if s := rollupState(nil, false); s != rollupStatePartial {
		t.Errorf("no backing brief must be partial, not %q — an empty set is not a vacuous pass", s)
	}
	// One done brief: satisfied.
	done := []rollupBrief{{Brief: "sdlc/01", Status: "done", Resolved: true}}
	if s := rollupState(done, false); s != rollupStateSatisfied {
		t.Errorf("all-done backing must be satisfied, got %q", s)
	}
	// One in-progress brief: partial.
	partial := []rollupBrief{{Brief: "sdlc/01", Status: "in-progress", Resolved: true}}
	if s := rollupState(partial, false); s != rollupStatePartial {
		t.Errorf("a not-done backing brief must be partial, got %q", s)
	}
	// Mixed: one done, one not — partial, never satisfied.
	mixed := []rollupBrief{
		{Brief: "sdlc/01", Status: "done", Resolved: true},
		{Brief: "sdlc/02", Status: "verified", Resolved: true},
	}
	if s := rollupState(mixed, false); s != rollupStatePartial {
		t.Errorf("a mix with a not-done brief must be partial, got %q", s)
	}
	// Unresolved backing: could-not-check, never satisfied even if the resolved
	// ones are all done.
	unresolved := []rollupBrief{{Brief: "sdlc/01", Status: "done", Resolved: true}}
	if s := rollupState(unresolved, true); s != rollupStateCouldNotCheck {
		t.Errorf("an unresolved backing brief must be could-not-check, got %q", s)
	}
	// A resolved brief with an EMPTY status (a real brief with no board row) is
	// treated as not-done — it holds the requirement at partial, never rounds up.
	noRow := []rollupBrief{{Brief: "sdlc/01", Status: "", Resolved: true}}
	if s := rollupState(noRow, false); s != rollupStatePartial {
		t.Errorf("a resolved brief with no board row must be partial, got %q", s)
	}
}

// TestRollupReconcilesDirections: buildRollupRequirement unions the briefs that
// CITE a requirement (satisfies:) with the briefs it CLAIMS (satisfied-by), and a
// satisfied-by naming an unresolvable brief makes the requirement could-not-check.
func TestRollupReconcilesDirections(t *testing.T) {
	citing := tracedBrief{Key: "sdlc/02", Status: "in-progress", Satisfies: []string{"REQ-evidence-visible"}}
	citedBy := map[string][]tracedBrief{"REQ-evidence-visible": {citing}}
	byKey := map[string]tracedBrief{"sdlc/02": citing}

	// satisfied-by names sdlc/02 (resolves) AND sdlc/99 (does not).
	e := requirementEntry{
		ID: "REQ-evidence-visible", Title: "t", Impact: "critical", Status: "satisfied",
		SatisfiedBy: []string{"sdlc/02", "sdlc/99"},
	}
	r := buildRollupRequirement(e, citedBy, byKey)
	if r.State != rollupStateCouldNotCheck {
		t.Errorf("an unresolvable satisfied-by must make the requirement could-not-check, got %q", r.State)
	}
	// sdlc/02 must appear exactly once (deduped across the two directions).
	n := 0
	for _, b := range r.Briefs {
		if b.Brief == "sdlc/02" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("a brief named by both directions must appear once, got %d", n)
	}
}

// ---------- end-to-end over a temp corpus ----------

// TestRollupEndToEnd builds a root with a requirement whose citing brief is only
// in-progress, and asserts the emitted report reports partial (never a silent
// pass) — the Verify row 7 property, exercised in-process.
func TestRollupEndToEnd(t *testing.T) {
	root := t.TempDir()
	// A requirement.
	f := validRequirementFixture()
	f.ID = "REQ-evidence-visible"
	writeFile(t, root, "docs/streams/requirements/req.md", f.render())
	// A stream README with one in-progress row, and its brief file citing the req.
	writeFile(t, root, "docs/streams/sdlc/README.md",
		"---\nstream: sdlc\nserves: assay\nstatus: active\n---\n\n"+
			"# sdlc\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n"+
			"|---|-------|------|--------|----------|----------|\n"+
			"| 01 | [x](brief-01.md) | 0 | in-progress | — | — |\n")
	requirementBrief(t, root+"/docs/streams/sdlc", "01", "satisfies: [\"REQ-evidence-visible\"]\n")

	report, code := buildRollupReport(root, "")
	if code != rollupExitOK {
		t.Fatalf("rollup must succeed, got code %d", code)
	}
	if len(report.Requirements) != 1 {
		t.Fatalf("want 1 requirement in the rollup, got %d", len(report.Requirements))
	}
	r := report.Requirements[0]
	if r.State != rollupStatePartial {
		t.Errorf("a requirement whose only brief is in-progress must be partial, got %q", r.State)
	}
	if len(r.Briefs) != 1 || r.Briefs[0].Brief != "sdlc/01" || r.Briefs[0].Status != "in-progress" {
		t.Errorf("the citing brief must appear with its board status; got %+v", r.Briefs)
	}
	if !strings.Contains(report.Note, "AUTHORED") {
		t.Errorf("the rollup must carry the authored-not-measured note; got %q", report.Note)
	}
}

// TestRollupSinceFilters: --since drops requirements dated before it.
func TestRollupSinceFilters(t *testing.T) {
	root := t.TempDir()
	old := validRequirementFixture()
	old.ID = "REQ-evidence-visible"
	old.Date = "2026-01-01"
	writeFile(t, root, "docs/streams/requirements/old.md", old.render())
	recent := validRequirementFixture()
	recent.ID = "REQ-coverage-boundary"
	recent.Date = "2026-09-04"
	writeFile(t, root, "docs/streams/requirements/recent.md", recent.render())

	report, code := buildRollupReport(root, "2026-06-01")
	if code != rollupExitOK {
		t.Fatalf("code %d", code)
	}
	if len(report.Requirements) != 1 || report.Requirements[0].ID != "REQ-coverage-boundary" {
		t.Errorf("--since must keep only the on-or-after entry; got %+v", report.Requirements)
	}
}
