package main

import (
	"testing"
	"time"
)

// TestRefine_PostdatesReport pins the postdating refinement's boundary: an inducer
// dated after the report is filtered; one dated on or before is kept; and with NO
// resolvable report date the refinement is inapplicable (never a silent filter).
func TestRefine_PostdatesReport(t *testing.T) {
	report := mustTime(t, "2020-02-01T00:00:00Z")
	after := mustTime(t, "2020-03-01T00:00:00Z")
	before := mustTime(t, "2020-01-01T00:00:00Z")

	if !postdatesReport(after, report, true) {
		t.Fatal("an inducer dated after the report must be filtered")
	}
	if postdatesReport(before, report, true) {
		t.Fatal("an inducer dated before the report must be kept")
	}
	if postdatesReport(report, report, true) {
		t.Fatal("an inducer dated exactly on the report is not strictly after — kept")
	}
	// No report date: the refinement is inapplicable and must never drop a candidate.
	if postdatesReport(after, time.Time{}, false) {
		t.Fatal("with no report date the postdating refinement must not filter anything")
	}
}

// TestRefine_ConfidenceScore pins the confidence scorer's shape: a single surviving
// inducer is full confidence; more inducers lower it; no survivor is a measured-zero
// (never a could-not-measure — the instrument ran).
func TestRefine_ConfidenceScore(t *testing.T) {
	single := scoreConfidence(1, 1)
	if single.State != StateMeasured || single.Value != 1.0 {
		t.Fatalf("a single inducer that survived the only candidate is full confidence 1.0, got %+v", single)
	}
	split := scoreConfidence(2, 2)
	if split.State != StateMeasured || !(split.Value < single.Value) {
		t.Fatalf("two inducers must score below one, got %+v vs %+v", split, single)
	}
	none := scoreConfidence(3, 0)
	if none.State != StateMeasuredZero {
		t.Fatalf("no surviving inducer is a measured-zero confidence, got %+v", none)
	}
}

// TestRefine_NormalizeWhitespace pins the cosmetic-change definition: two lines that
// differ only in whitespace normalize equal; a real content change does not.
func TestRefine_NormalizeWhitespace(t *testing.T) {
	if normalizeWhitespace("  a := f( x , y ) ") != normalizeWhitespace("a:=f(x,y)") {
		t.Fatal("whitespace-only differences must normalize equal")
	}
	if normalizeWhitespace("a := f(x, y)") == normalizeWhitespace("a := f(x, y, z)") {
		t.Fatal("a real content change must NOT normalize equal")
	}
}

// TestRefine_CosmeticInducerExcluded is Verify row 7: the blamed line's
// last-touching commit was a whitespace/format-only change, so it must be DROPPED
// and blame must fall through to the prior real inducer. A introduces the line, W
// reformats it (whitespace only), F fixes it. The trace must name A — not W.
func TestRefine_CosmeticInducerExcluded(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")

	a := commitFile(t, dir, "2020-01-01T00:00:00Z", "util.go",
		"package util\nresult := compute(a,b)\ntail\n", "A: introduce the real inducing line")
	// Whitespace/format-only reformat of the inducing line — a cosmetic commit.
	w := commitFile(t, dir, "2020-02-01T00:00:00Z", "util.go",
		"package util\nresult := compute( a, b )\ntail\n", "W: reformat spacing (cosmetic)")
	f := commitFile(t, dir, "2020-03-01T00:00:00Z", "util.go",
		"package util\nresult := compute(a, b, c)\ntail\n", "fix: pass the missing argument (#13)")

	repo := openRepo(t, dir)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 13, 13)}, nil)
	tr, ok := traceOf(corpus.Traces, f)
	if !ok {
		t.Fatalf("no trace for fix %s", f)
	}
	if tr.TraceState != TraceTraced {
		t.Fatalf("expected traced (cosmetic fall-through to A), got %q/%q", tr.TraceState, tr.CouldNotTraceReason)
	}
	if !contains(tr.InducingCommits, a) {
		t.Fatalf("inducing_commits %v must contain the real inducer A=%s after falling through the cosmetic W", tr.InducingCommits, a)
	}
	if contains(tr.InducingCommits, w) {
		t.Fatalf("the cosmetic commit W=%s must be DROPPED, not recorded as an inducer: %v", w, tr.InducingCommits)
	}
}
