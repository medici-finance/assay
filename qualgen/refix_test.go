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

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

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

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

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

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

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

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

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

// TestRefix_IssueLessFixNoLinkage_CouldNotMeasure is the pr-review sibling to
// Verify #5 (q19-F1 / assay-reviewer-app finding
// F-refix-fail-open-missing-linkage): a window with TWO traced fixes — one
// carrying a closed issue (so the class path actually calls DefectClass and
// gets the "unconfigured" error), one with NO closed issue at all (so its
// class path is "legitimately absent": there is nothing to classify). Neither
// fix ever resolves a link. The original guard required EVERY traced fix to
// be could-not-measure before reporting the window as could-not-measure; here
// only one of the two is, so resolvedCount stayed 0 while
// couldNotMeasureCount (1) != len(tracedFixes) (2), and the window fell
// through to a fabricated measured-zero. The fix is the whole-window
// could-not-measure the moment ANY traced fix could not be measured while
// none resolved.
func TestRefix_IssueLessFixNoLinkage_CouldNotMeasure(t *testing.T) {
	withIssue := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, ClosedIssue: &IssueRef{Number: 1}, Tier: Tier1, Identified: Measured(true)}
	issueLess := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)} // no ClosedIssue
	trWith := DefectTrace{FixCommit: refixE1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}
	trIssueLess := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd2}}

	linkage := stubRegressionLinkage{
		// No regression-of: named anywhere.
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) { return nil, false, nil },
		// Class prefix unconfigured — the reference adapter's contract is to
		// error whenever it is actually called.
		defectClass: func(ref IssueRef) (string, bool, error) {
			return "", false, fmt.Errorf("defect-class label prefix not configured")
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		refixF1:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		refixInd2: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{withIssue, issueLess}, []DefectTrace{trWith, trIssueLess}, linkage, ct.at, "", time.Now())

	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure (one fix's linkage never resolved), got %+v", rec.RefixRate)
	}
	if rec.RefixCount.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_count could-not-measure, got %+v", rec.RefixCount)
	}
	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure, got %+v", rec.LinkageCoverage)
	}
}

// TestRefix_RegressionOfNamesUnknownFix_CouldNotMeasure is q19-F2(a): F's
// `regression-of:` names an issue that no fix in the mined corpus closes. The
// original code treated "a reference was found" as resolved=true regardless
// of whether it matched anything, scoring F as a measured non-re-fix
// (linkage_coverage measured 1, refix_rate measured-zero) even though the
// author explicitly said "this is a regression" — we simply cannot tell
// whether the named fix landed before or after F's inducer if it is not even
// in the corpus.
func TestRefix_RegressionOfNamesUnknownFix_CouldNotMeasure(t *testing.T) {
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{Issue: &IssueRef{Number: 77}}}, true, nil
			}
			return nil, false, nil
		},
	}
	ct := fixedCommitTime{refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}

	// allFixes contains only f itself — nothing closes issue #77.
	rec := ComputeRefix("w1", []DefectFix{f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (regression-of names an unresolvable fix), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
}

// TestRefix_ClassCandidateLabelError_CouldNotMeasure is q19-F2(b): F's own
// class resolves, but reading the EARLIER fix E's labels errors (a rate
// limit, say). The original code's `if err2 != nil || !ok2 || eClass !=
// class { continue }` treated that identically to "does not match" — silently
// dropping the one candidate that could have made F a re-fix, and scoring F
// as measured/not-a-re-fix instead of could-not-measure.
func TestRefix_ClassCandidateLabelError_CouldNotMeasure(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE2, FixPRNumber: 101, ClosedIssue: &IssueRef{Number: 1}, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF2, FixPRNumber: 201, ClosedIssue: &IssueRef{Number: 2}, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF2, TraceState: TraceTraced, InducingCommits: []string{refixInd2}}

	linkage := stubRegressionLinkage{
		defectClass: func(ref IssueRef) (string, bool, error) {
			if ref.Number == 2 {
				return "class-widget-nil-deref", true, nil
			}
			if ref.Number == 1 {
				return "", false, fmt.Errorf("403 rate limited")
			}
			return "", false, nil
		},
	}
	ct := fixedCommitTime{
		refixE2:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		refixInd2: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (E's label read errored), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
	if len(rec.Refixes) != 0 {
		t.Fatalf("expected no counted re-fix out of an unresolved candidate, got %+v", rec.Refixes)
	}
}

// TestRefix_CandidateFixTimeUnresolvable_CouldNotMeasure is q19-F2(c): F links
// to E via regression-of, but E's own fix-commit time cannot be resolved. The
// original `if !ok { continue }` silently dropped the candidate, falling
// through to "not a re-fix" (measured-zero) even though the ordering rule
// could never actually be evaluated.
func TestRefix_CandidateFixTimeUnresolvable_CouldNotMeasure(t *testing.T) {
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
	// refixE1's fix time is deliberately absent from the map.
	ct := fixedCommitTime{refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (E's fix time unresolvable), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
}

// TestRefix_PartialInducerTime_CouldNotMeasure is q19-F2(d): F has TWO
// inducing commits, one with a known (later) time and one whose time is
// unresolvable. The original earliestTime ignored the unresolvable commit and
// used the known one as "earliest", which can make an E that actually landed
// BEFORE the true (unresolvable) earliest inducer look like it landed after
// it — silently over- or under-counting. The fix requires ALL inducing-commit
// times to resolve before the ordering rule runs at all.
func TestRefix_PartialInducerTime_CouldNotMeasure(t *testing.T) {
	const refixIndUnknown = "c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3"
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixIndUnknown, refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{PRNumber: 100}}, true, nil
			}
			return nil, false, nil
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), // between the unknown inducer and the known one
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		// refixIndUnknown deliberately absent.
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (a partial inducer-time set), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixCount.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_count could-not-measure, got %+v", rec.RefixCount)
	}
}

// TestRefMatches_RepoQualifiedRef_MatchesMinedRepoSentinel is q19-F3: a
// `regression-of: owner/repo#N` reference's Issue.Repo is populated, while
// GithubLabelsLinkage's ClosedIssue (fixlinkage.go) always leaves Repo == ""
// ("the mined repo itself" — IssueRef's own doc comment). A literal
// ref.Issue.Repo == e.ClosedIssue.Repo comparison can then never match a
// repo-qualified reference to the mined repo's own fix, even though that is
// exactly the form the pickup precondition documents as accepted. The match
// is made through the mined repo's own slug: the qualified reference names
// the mined repo, so it matches the mined repo's issue.
func TestRefMatches_RepoQualifiedRef_MatchesMinedRepoSentinel(t *testing.T) {
	ref := RegressionRef{Issue: &IssueRef{Repo: "medici-finance/assay", Number: 5}}
	e := DefectFix{FixCommitSHA: refixE1, ClosedIssue: &IssueRef{Number: 5}} // Repo == "": the mined repo
	if !refMatches(ref, e, "medici-finance/assay") {
		t.Fatalf("expected a repo-qualified regression-of reference naming the mined repo to match its own closed issue #5")
	}
}

// TestRefMatches_ForeignRepoQualifiedRef_DoesNotMatch is q19-F3 round 2 /
// A-refmatches-foreign-repo-wildcard: a `regression-of:` naming ANOTHER
// repository's issue must never match the mined repo's own same-numbered
// issue. The round-1 fix treated an empty Repo on EITHER side as a wildcard
// match, and GithubLabelsLinkage's ClosedIssue always carries Repo == "", so
// any repo-qualified reference matched — counting a re-fix against an
// unrelated earlier fix and OVER-counting the rate, worse than the original
// silent non-link, which could only under-count. And with the mined repo's
// identity unknown, even a reference naming the mined repo's own slug cannot
// be confirmed: never a wildcard match.
func TestRefMatches_ForeignRepoQualifiedRef_DoesNotMatch(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, ClosedIssue: &IssueRef{Number: 5}} // Repo == "": the mined repo
	foreign := RegressionRef{Issue: &IssueRef{Repo: "other-org/other-repo", Number: 5}}
	if refMatches(foreign, e, "medici-finance/assay") {
		t.Fatalf("a foreign repo-qualified reference must not match the mined repo's issue #5")
	}
	own := RegressionRef{Issue: &IssueRef{Repo: "medici-finance/assay", Number: 5}}
	if refMatches(own, e, "") {
		t.Fatalf("a repo-qualified reference is unverifiable when the mined repo's identity is unknown — must not match")
	}
}

// TestRefix_ClassErrorBehindOrderingFailingCandidate_CouldNotMeasure is the
// q19-F2(b) residual (round 2) and the security lane's second surviving
// F-refix-fail-open-missing-linkage occurrence: F's class path produces a
// CLEAN candidate E1 that then fails the ordering rule (E1's fix landed after
// F's inducer), while ANOTHER earlier fix E2's label read errored. Round 1
// surfaced the error only when NO clean candidate existed, so E1's ordering
// failure hid E2 entirely and F was scored a covered, measured non-re-fix —
// even though E2 (fixed BEFORE the inducer) could have been the qualifying
// earlier fix. With no candidate satisfying the ordering rule and an E-side
// read errored, F is could-not-measure (resolved=false).
func TestRefix_ClassErrorBehindOrderingFailingCandidate_CouldNotMeasure(t *testing.T) {
	const refixE3 = "e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3e3"
	e1 := DefectFix{FixCommitSHA: refixE2, FixPRNumber: 101, ClosedIssue: &IssueRef{Number: 1}, Tier: Tier1, Identified: Measured(true)}
	e2 := DefectFix{FixCommitSHA: refixE3, FixPRNumber: 102, ClosedIssue: &IssueRef{Number: 3}, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF2, FixPRNumber: 201, ClosedIssue: &IssueRef{Number: 2}, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF2, TraceState: TraceTraced, InducingCommits: []string{refixInd2}}

	linkage := stubRegressionLinkage{
		defectClass: func(ref IssueRef) (string, bool, error) {
			switch ref.Number {
			case 1, 2:
				return "class-widget-nil-deref", true, nil
			case 3:
				return "", false, fmt.Errorf("403 rate limited")
			}
			return "", false, nil
		},
	}
	ct := fixedCommitTime{
		refixE2:   time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC), // E1: same class but fixed AFTER F's inducer
		refixE3:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // E2: fixed BEFORE the inducer — but its labels never read
		refixInd2: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e1, e2, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (E2's label read errored behind an ordering-failing candidate), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
	if len(rec.Refixes) != 0 {
		t.Fatalf("expected no counted re-fix out of an unresolved candidate, got %+v", rec.Refixes)
	}
}

// TestRefix_ExplicitPathError_ClassPathResolvedNoMatch_CouldNotMeasure is the
// security lane's first surviving F-refix-fail-open-missing-linkage occurrence
// (evaluateFix, round 2): the explicit path ERRORS (a RegressionOf failure —
// including this round's own unparseable regression-of value) while the class
// path resolves cleanly with ZERO matching candidates. Round 1's
// `resolved = expResolved || classResolved` discarded the explicit path's
// could-not-measure the moment the class path resolved, scoring F a covered,
// measured non-re-fix. With no candidate satisfying the ordering rule and the
// explicit path could-not-measure, F is could-not-measure (resolved=false).
func TestRefix_ExplicitPathError_ClassPathResolvedNoMatch_CouldNotMeasure(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE2, FixPRNumber: 101, ClosedIssue: &IssueRef{Number: 1}, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF2, FixPRNumber: 201, ClosedIssue: &IssueRef{Number: 2}, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF2, TraceState: TraceTraced, InducingCommits: []string{refixInd2}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF2 {
				return nil, false, fmt.Errorf(`regressionlink: unparseable regression-of value(s): docs/streams/quality/brief-2-f.md: "#5""`)
			}
			return nil, false, nil
		},
		defectClass: func(ref IssueRef) (string, bool, error) {
			// F's own class reads fine; E's resolves to a DIFFERENT class —
			// the class path resolved cleanly and matched nobody.
			if ref.Number == 2 {
				return "class-widget-nil-deref", true, nil
			}
			return "class-other", true, nil
		},
	}
	ct := fixedCommitTime{
		refixE2:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		refixInd2: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (the explicit path errored behind a resolved class path), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
	if len(rec.Refixes) != 0 {
		t.Fatalf("expected no counted re-fix, got %+v", rec.Refixes)
	}
}

// TestRefix_ClassPathError_ExplicitCandidateFailsOrdering_CouldNotMeasure is
// the mirror shape (the correctness lane's advisory probe, covered here
// because the security lane holds the class): the explicit path resolves a
// candidate that FAILS the ordering rule while the class path errors (here:
// the class-label prefix unconfigured — Verify #5's own error). The resolved
// explicit candidate proves only that E landed after F's inducer; it says
// nothing about whether an uncheckable class-linked fix would have qualified.
// F is could-not-measure (resolved=false), not a measured non-re-fix.
func TestRefix_ClassPathError_ExplicitCandidateFailsOrdering_CouldNotMeasure(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, ClosedIssue: &IssueRef{Number: 2}, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{PRNumber: 100}}, true, nil
			}
			return nil, false, nil
		},
		defectClass: func(ref IssueRef) (string, bool, error) {
			return "", false, fmt.Errorf("defect-class label prefix not configured")
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC), // E fixed AFTER F's inducer
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (the class path errored behind an ordering-failing candidate), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
}

// TestRefix_ForeignRepoRegressionOf_CouldNotMeasure is q19-F3 end to end: a
// `regression-of:` naming a foreign repository's issue names a fix outside
// the mined corpus — the same could-not-measure shape as q19-F2(a)'s
// unknown-fix reference, never a measured non-re-fix (which the round-1
// wildcard matching fabricated by "matching" the mined repo's same-numbered
// issue).
func TestRefix_ForeignRepoRegressionOf_CouldNotMeasure(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, ClosedIssue: &IssueRef{Number: 5}, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{Issue: &IssueRef{Repo: "other-org/other-repo", Number: 5}}}, true, nil
			}
			return nil, false, nil
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "medici-finance/assay", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (a foreign regression-of names no fix in the mined corpus), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
	if len(rec.Refixes) != 0 {
		t.Fatalf("a foreign reference must never count a re-fix, got %+v", rec.Refixes)
	}
}

// TestRefix_UnmatchedRefBehindOrderingFailingCandidate_CouldNotMeasure is the
// security lane's round-3 surviving F-refix-fail-open-missing-linkage
// occurrence (explicitCandidates): F names TWO regression-of references — one
// matching E1, whose fix landed AFTER F's inducer (fails the ordering rule),
// and one (#9) matching NO identified fix. Round 2's fix surfaced an unmatched
// reference only when ALL references were unmatched, so E1's ordering failure
// scored F a covered, measured non-re-fix while the author's own #9 reference
// — which could have named the qualifying earlier fix — was silently
// discarded. With an unmatched named reference and no candidate satisfying
// the ordering rule, F is could-not-measure (resolved=false).
func TestRefix_UnmatchedRefBehindOrderingFailingCandidate_CouldNotMeasure(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{PRNumber: 100}, {Issue: &IssueRef{Number: 9}}}, true, nil
			}
			return nil, false, nil
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC), // E fixed AFTER F's inducer — fails the ordering rule
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.LinkageCoverage.State != StateCouldNotMeasure {
		t.Fatalf("expected linkage_coverage could-not-measure (an unmatched regression-of reference behind an ordering-failing candidate), got %+v", rec.LinkageCoverage)
	}
	if rec.RefixRate.State != StateCouldNotMeasure {
		t.Fatalf("expected refix_rate could-not-measure, got %+v", rec.RefixRate)
	}
	if len(rec.Refixes) != 0 {
		t.Fatalf("expected no counted re-fix, got %+v", rec.Refixes)
	}
}

// TestRefix_UnmatchedRefAlongsideQualifyingCandidate_StillCounted is the
// positive-control mirror of the round-3 fix: the unmatched reference rides
// along as could-not-measure, but a matched candidate that SATISFIES the
// ordering rule still earns the measured re-fix — positive evidence stands;
// only a measured NON-re-fix requires every named reference resolvable.
func TestRefix_UnmatchedRefAlongsideQualifyingCandidate_StillCounted(t *testing.T) {
	e := DefectFix{FixCommitSHA: refixE1, FixPRNumber: 100, Tier: Tier1, Identified: Measured(true)}
	f := DefectFix{FixCommitSHA: refixF1, FixPRNumber: 200, Tier: Tier1, Identified: Measured(true)}
	tr := DefectTrace{FixCommit: refixF1, TraceState: TraceTraced, InducingCommits: []string{refixInd1}}

	linkage := stubRegressionLinkage{
		regressionOf: func(cand DefectFix) ([]RegressionRef, bool, error) {
			if cand.FixCommitSHA == refixF1 {
				return []RegressionRef{{PRNumber: 100}, {Issue: &IssueRef{Number: 9}}}, true, nil
			}
			return nil, false, nil
		},
	}
	ct := fixedCommitTime{
		refixE1:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // E fixed BEFORE F's inducer — satisfies the ordering rule
		refixInd1: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := ComputeRefix("w1", []DefectFix{e, f}, []DefectTrace{tr}, linkage, ct.at, "", time.Now())

	if rec.RefixCount.State != StateMeasured || rec.RefixCount.Value != 1 {
		t.Fatalf("expected refix_count measured 1 (a qualifying matched candidate stands despite an unmatched sibling reference), got %+v", rec.RefixCount)
	}
	if len(rec.Refixes) != 1 || rec.Refixes[0].EarlierFixPRNumber != 100 {
		t.Fatalf("expected the E pairing counted, got %+v", rec.Refixes)
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

// TestReport_RefixSection_ReminedWindow_RendersOnce is q19-F4: a window that
// gets re-mined appends a SECOND RefixRecord for the same Window (RefixRecord's
// own doc comment: "append-only, latest-per-window selected by the report").
// The original writeRefixSection rendered every appended record with no
// latest-per-window reduction, so the trend table gained a duplicate row and
// "Counted re-fixes:" named the same fix twice. Only the newest record's row
// and re-fix entries must appear.
func TestReport_RefixSection_ReminedWindow_RendersOnce(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	first := RefixRecord{
		Metric: MetricRefix, Window: "2026-09",
		TracedFixCount: 3,
		RefixCount:     Measured(1.0), RefixRate: Measured(1.0 / 3), LinkageCoverage: Measured(1.0),
		Refixes: []RefixEntry{{FixCommitSHA: refixF1, FixPRNumber: 4242, EarlierFixCommitSHA: refixE1, EarlierFixPRNumber: 4100, LinkKind: "regression-of"}},
		MinedAt: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
	}
	reMined := RefixRecord{
		Metric: MetricRefix, Window: "2026-09",
		TracedFixCount: 4,
		RefixCount:     Measured(1.0), RefixRate: Measured(0.25), LinkageCoverage: Measured(0.75),
		Refixes: []RefixEntry{{FixCommitSHA: refixF1, FixPRNumber: 4242, EarlierFixCommitSHA: refixE1, EarlierFixPRNumber: 4100, LinkKind: "regression-of"}},
		MinedAt: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), // mined LATER, same window
	}
	if err := store.Append(KindMetric, first); err != nil {
		t.Fatalf("seed first record: %v", err)
	}
	if err := store.Append(KindMetric, reMined); err != nil {
		t.Fatalf("seed re-mined record: %v", err)
	}

	var out, errb bytes.Buffer
	if rc := runReport([]string{"--out", root}, &out, &errb); rc != 0 {
		t.Fatalf("report run failed: rc=%d stderr=%s", rc, errb.String())
	}
	view := out.String()

	if got := strings.Count(view, "| 2026-09 |"); got != 1 {
		t.Fatalf("expected exactly one trend row for window 2026-09, got %d; view:\n%s", got, view)
	}
	if got := strings.Count(view, "PR #4242"); got != 1 {
		t.Fatalf("expected the re-fix named exactly once (latest record only), got %d; view:\n%s", got, view)
	}
	if !strings.Contains(view, "0.25") {
		t.Fatalf("expected the LATEST record's refix_rate 0.25 rendered, not the stale 1/3; view:\n%s", view)
	}
}
