package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// --- a minimal stub RegressionLinkage, for exercising ComputeRefix's join and
// ordering logic directly (independent of the reference adapter under test in
// regressionlink_test.go) — mirrors fixlinkage_test.go's stubLinkageAdapter. ---

type stubRegressionLinkage struct {
	regressionOf func(f DefectFix) ([]RegressionRef, bool, error)
	defectClass  func(ref IssueRef) (string, bool, error)
}

func (s stubRegressionLinkage) RegressionOf(f DefectFix) ([]RegressionRef, bool, error) {
	if s.regressionOf == nil {
		return nil, false, nil
	}
	return s.regressionOf(f)
}

func (s stubRegressionLinkage) DefectClass(ref IssueRef) (string, bool, error) {
	if s.defectClass == nil {
		return "", false, nil
	}
	return s.defectClass(ref)
}

// fixedCommitTime is a test CommitTime: a plain map from sha to author time. A
// sha absent from the map has no resolvable time.
type fixedCommitTime map[string]time.Time

func (f fixedCommitTime) at(sha string) (time.Time, bool) {
	t, ok := f[sha]
	return t, ok
}

var (
	refixE1   = "e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1"
	refixF1   = "f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1"
	refixE2   = "e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2"
	refixF2   = "f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2"
	refixInd1 = "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"
	refixInd2 = "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2"
)

// TestRefix_RegressionOfLink_Counted is Verify #2: a traced fix whose brief's
// `regression-of:` names an earlier fix is counted as a re-fix, when the earlier
// fix landed before the traced fix's earliest inducing commit.
func TestRefix_RegressionOfLink_Counted(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{PRNumber: 100}}, true, nil
			}
			return nil, false, nil
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // E's fix time
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), // F's inducer — AFTER E's fix
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, time.Now())

	if rec.TracedFixCount != 1 {
		t.Fatalf("expected 1 traced fix, got %d", rec.TracedFixCount)
	}
	if rec.RefixCount.State != StateMeasured || rec.RefixCount.Value != 1 {
		t.Fatalf("expected refix_count measured 1, got %+v", rec.RefixCount)
	}
	if rec.RefixRate.State != StateMeasured || rec.RefixRate.Value != 1.0 {
		t.Fatalf("expected refix_rate measured 1.0, got %+v", rec.RefixRate)
	}
	if len(rec.Refixes) != 1 {
		t.Fatalf("expected exactly one counted re-fix, got %+v", rec.Refixes)
	}
	got := rec.Refixes[0]
	if got.FixPRNumber != 200 || got.EarlierFixPRNumber != 100 || got.LinkKind != "regression-of" {
		t.Fatalf("unexpected re-fix entry: %+v", got)
	}
}

// TestRefix_SameDefectClass_Counted is Verify #3: with a class prefix
// configured, two fixes whose closed issues share a class key yield one
// re-fix.
func TestRefix_SameDefectClass_Counted(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE2, FixPRNumber: 101, ClosedIssue: &IssueRef{Number: 1}, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF2, FixPRNumber: 201, ClosedIssue: &IssueRef{Number: 2}, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF2, TraceState: TraceTraced, InducingCommits: []string{refixInd2}}

	linkage := stubRegressionLinkage{
		defectClass: func(ref IssueRef) (string, bool, error) {
			if ref.Number == 1 || ref.Number == 2 {
				return "class-widget-nil-deref", true, nil
			}
			return "", false, nil
		},
	}
	ct := fixedCommitTime{
		refixE2:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		refixInd2: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, time.Now())

	if rec.RefixCount.State != StateMeasured || rec.RefixCount.Value != 1 {
		t.Fatalf("expected refix_count measured 1, got %+v", rec.RefixCount)
	}
	if len(rec.Refixes) != 1 || rec.Refixes[0].LinkKind != "defect-class" {
		t.Fatalf("expected one defect-class re-fix, got %+v", rec.Refixes)
	}
	if rec.Refixes[0].FixPRNumber != 201 || rec.Refixes[0].EarlierFixPRNumber != 101 {
		t.Fatalf("unexpected re-fix pairing: %+v", rec.Refixes[0])
	}
}

// TestRefix_EarlierFixAfterInducer_NotCounted is Verify #4 (a mutation test —
// see Task 5's fail-first): the negative path. A linked fix E whose fix time is
// AFTER F's earliest inducing commit is NOT a re-fix — the defect must have been
// fixed BEFORE it was reintroduced, never the reverse or a concurrent fix.
func TestRefix_EarlierFixAfterInducer_NotCounted(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{PRNumber: 100}}, true, nil
			}
			return nil, false, nil
		},
	}
	// E's fix (refixE1) lands AFTER F's inducer (refixInd1) — the reverse of
	// the counted case above.
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		refixInd1: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, time.Now())

	if rec.RefixCount.State != StateMeasuredZero {
		t.Fatalf("expected refix_count measured-zero (linked but wrong order), got %+v", rec.RefixCount)
	}
	if rec.RefixRate.State != StateMeasuredZero {
		t.Fatalf("expected refix_rate measured-zero, got %+v", rec.RefixRate)
	}
	if len(rec.Refixes) != 0 {
		t.Fatalf("expected no counted re-fix, got %+v", rec.Refixes)
	}
	// The link itself DID resolve (regression-of was found) — coverage is a
	// real measured value, distinct from the ordering-driven zero rate.
	if rec.LinkageCoverage.State != StateMeasured || rec.LinkageCoverage.Value != 1.0 {
		t.Fatalf("expected linkage_coverage measured 1.0 (link resolved, ordering failed), got %+v", rec.LinkageCoverage)
	}
}

// TestRefix_NoLinkageConfigured_CouldNotMeasure is Verify #5: with no
// `regression-of:` anywhere and no class prefix configured, the rate and
// coverage are could-not-measure — NEVER rounded down to a fabricated
// measured-zero, because the run never actually got to look.
func TestRefix_NoLinkageConfigured_CouldNotMeasure(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, ClosedIssue: &IssueRef{Number: 1}, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, ClosedIssue: &IssueRef{Number: 2}, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		// No regression-of: named anywhere — legitimately absent, not an error.
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) { return nil, false, nil },
		// No class prefix configured — the reference adapter's own contract
		// (regressionlink.go) is to ERROR here, never silently answer "no
		// class"; the stub mirrors that contract directly.
		defectClass: func(ref IssueRef) (string, bool, error) {
			return "", false, fmt.Errorf("defect-class label prefix not configured")
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, time.Now())

	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
	if rec.RefixCount.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_count could-not-measure, got %+v", rec.RefixCount)
	}
	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure, got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.Reason == "" || rec.LinkageCoverage.Reason == "" {
		t.Fatalf("could-not-measure Measures must carry a reason, got rate=%+v coverage=%+v", rec.RefixRate, rec.LinkageCoverage)
	}
}

// TestReport_RefixSection_Renders is Verify #6: over a fixture store holding
// one planted re-fix, the rendered report's re-fix section names that fix's PR
// number AND a rate of the expected value — dereferencing the rendered number
// back to the planted record, not only the heading.
func TestReport_RefixSection_Renders(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	planted := RefixRecord{
		Metric:          MetricRefix,
		Window:          "2026-09",
		TracedFixCount:  4,
		RefixCount:      Measured(1.0),
		RefixRate:       Measured(0.25),
		LinkageCoverage: Measured(0.75),
		TierComposition: TierComposition{Tier1Count: 3, Tier2Count: 1},
		Refixes: []RefixEntry{
			{FixCommitSHA: refixF1, FixPRNumber: 4242, EarlierFixCommitSHA: refixE1, EarlierFixPRNumber: 4100, LinkKind: "regression-of"},
		},
		MinedAt: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
	}
	if err := store.Append(KindMetric, planted); err != nil {
		t.Fatalf("seed planted re-fix record: %v", err)
	}

	var out, errb bytes.Buffer
	if rc := runReport([]string{"--out", root}, &out, &errb); rc != 0 {
		t.Fatalf("report run failed: rc=%d stderr=%s", rc, errb.String())
	}
	view := out.String()

	if !strings.Contains(view, "## Re-fix rate (regression-suite effectiveness)") {
		t.Fatalf("expected the re-fix section heading; view:\n%s", view)
	}
	if !strings.Contains(view, "PR #4242") {
		t.Fatalf("expected the planted fix's PR number 4242 named in the render; view:\n%s", view)
	}
	if !strings.Contains(view, "0.25") {
		t.Fatalf("expected the planted refix_rate 0.25 rendered; view:\n%s", view)
	}
	if !strings.Contains(view, "PR #4100") {
		t.Fatalf("expected the earlier fix's PR number 4100 named beside it; view:\n%s", view)
	}
}
