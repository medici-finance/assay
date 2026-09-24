package main

import (
	"fmt"
	"strings"
	"time"
)

// refix.go — the re-fix join and metric (brief quality/19, Task 2): whether a
// regression-suite of tests, class guards and stub-coverage rules is worth its
// cost shows up as fewer defects coming BACK. A fix F is a re-fix when its
// trace is `traced` and there is an earlier fix E, linked to F either by F's
// `regression-of:` naming E or by F and E's closed issues sharing a defect-
// class key, whose fix landed BEFORE F's earliest inducing commit — the defect
// was fixed, then reintroduced.
//
// PURE over DefectFix / DefectTrace / RegressionLinkage / commit-time records —
// no git, no tracker access of its own (regressionlink.go's reference adapter
// holds those effects; a test here supplies a fixture RegressionLinkage and a
// plain commit-time map). REPORT-ONLY: no threshold, budget or alarm is
// computed here or anywhere downstream of it in this brief.

// CommitTime resolves a commit SHA's author time. The ordering rule needs the
// fix/inducing-commit timestamps; supplying it as a plain lookup (the caller
// builds it from the ALREADY-MINED commits table, e.g. via Store.ReadCommits)
// keeps this join pure — it never opens git itself.
type CommitTime func(sha string) (t time.Time, ok bool)

// MetricRefix is the metrics-table discriminator for RefixRecord (the
// "MetricRecord pattern" fact: a "metric" field plus a mined-at stamp).
const MetricRefix = "refix"

// RefixEntry names one counted re-fix — the specific F/E pairing — so the
// rendered report can dereference the metric back to a concrete record (Verify
// #6), not only the aggregate number.
type RefixEntry struct {
	FixCommitSHA        string `json:"fix_commit_sha"`
	FixPRNumber         int    `json:"fix_pr_number,omitempty"`
	EarlierFixCommitSHA string `json:"earlier_fix_commit_sha"`
	EarlierFixPRNumber  int    `json:"earlier_fix_pr_number,omitempty"`
	// LinkKind names which path resolved the link: "regression-of" or
	// "defect-class". Never both — refix.go stops at the first candidate that
	// satisfies the ordering rule (§ evaluateFix), preferring the earliest
	// discovered explicit link over a class match, mirroring
	// fixlinkage.go's ClassifyFix "stop at the strongest evidence" shape.
	LinkKind string `json:"link_kind"`
}

// RefixRecord is one window's re-fix metric point, appended to metrics.jsonl
// (Task 3 renders it as the trend line — one RefixRecord per window,
// accumulated across mine runs the same way the sibling M1/M2 families are:
// append-only, latest-per-window selected by the report). Window is a caller-
// chosen label (a calendar bucket, a run index, ...) — wiring qualgen mine to
// choose one is out of THIS brief's scope (fact: "Wiring M2 into mine is out
// of scope"); ComputeRefix is the pure per-window computation a future wiring
// brief calls once per bucket.
type RefixRecord struct {
	Metric string `json:"metric"` // "refix"
	Window string `json:"window"`

	TracedFixCount int `json:"traced_fix_count"`

	// RefixCount / RefixRate / LinkageCoverage are three-state Measures
	// (spec-equivalent honesty discipline): a window whose regression linkage
	// never resolved is could-not-measure, NEVER a fabricated 0.
	RefixCount      Measure[float64] `json:"refix_count"`
	RefixRate       Measure[float64] `json:"refix_rate"`
	LinkageCoverage Measure[float64] `json:"linkage_coverage"`

	// TierComposition is the evidence-tier composition of the TRACED fixes
	// counted in this window (fact: "the evidence-tier composition of the
	// counted fixes" travels beside the rate).
	TierComposition TierComposition `json:"tier_composition"`

	// Refixes names each counted re-fix (Verify #6's dereference target).
	Refixes []RefixEntry `json:"refixes,omitempty"`

	MinedAt time.Time `json:"mined_at"`
}

// refixBasis / refixNote are the honest-claims labels for this family (spec §10
// pattern the M1/M2 families already carry).
const (
	refixBasis = "regression-of + defect-class linkage, traced fixes only"
	refixNote  = "report-only: no threshold, budget or alarm; linkage coverage and evidence-tier composition travel beside the rate — never a bare number"
)

// refMatches reports whether ref (an explicit `regression-of:` reference)
// names e as the earlier fix — by commit sha (prefix match, since a
// `regression-of:` value may be an abbreviated sha), by closed issue, or by PR
// number.
func refMatches(ref RegressionRef, e DefectFix) bool {
	if ref.CommitSHA != "" {
		full := strings.ToLower(e.FixCommitSHA)
		short := strings.ToLower(ref.CommitSHA)
		if len(short) <= len(full) && strings.HasPrefix(full, short) {
			return true
		}
	}
	if ref.PRNumber != 0 && e.FixPRNumber != 0 && ref.PRNumber == e.FixPRNumber {
		return true
	}
	if ref.Issue != nil && e.ClosedIssue != nil &&
		ref.Issue.Number == e.ClosedIssue.Number && ref.Issue.Repo == e.ClosedIssue.Repo {
		return true
	}
	return false
}

// candidateLink is one resolved candidate earlier fix, tagged with the path
// that produced it.
type candidateLink struct {
	e    DefectFix
	kind string
}

// explicitCandidates resolves F's `regression-of:` path against allFixes.
// resolved=true means the linkage check itself succeeded (a reference was
// found), independent of whether that reference matches a fix in allFixes.
func explicitCandidates(linkage RegressionLinkage, f DefectFix, allFixes []DefectFix) (cands []candidateLink, resolved, couldNotMeasure bool, reason string) {
	refs, ok, err := linkage.RegressionOf(f)
	if err != nil {
		return nil, false, true, fmt.Sprintf("regression-of resolution: %v", err)
	}
	if !ok || len(refs) == 0 {
		return nil, false, false, "" // legitimately absent: no regression-of: named
	}
	for _, ref := range refs {
		for _, e := range allFixes {
			if e.FixCommitSHA == f.FixCommitSHA {
				continue
			}
			if refMatches(ref, e) {
				cands = append(cands, candidateLink{e: e, kind: "regression-of"})
			}
		}
	}
	return cands, true, false, ""
}

// classCandidates resolves F's defect-class path: reads F's own class, then
// looks for other fixes in allFixes whose closed issue resolves to the SAME
// class. resolved=true once F's own class was read successfully, independent
// of whether any other fix shares it.
func classCandidates(linkage RegressionLinkage, f DefectFix, allFixes []DefectFix) (cands []candidateLink, resolved, couldNotMeasure bool, reason string) {
	if f.ClosedIssue == nil {
		return nil, false, false, "" // no issue to classify: legitimately absent
	}
	class, ok, err := linkage.DefectClass(*f.ClosedIssue)
	if err != nil {
		return nil, false, true, fmt.Sprintf("defect-class resolution: %v", err)
	}
	if !ok || class == "" {
		return nil, false, false, ""
	}
	for _, e := range allFixes {
		if e.FixCommitSHA == f.FixCommitSHA || e.ClosedIssue == nil {
			continue
		}
		eClass, ok2, err2 := linkage.DefectClass(*e.ClosedIssue)
		if err2 != nil || !ok2 || eClass != class {
			continue
		}
		cands = append(cands, candidateLink{e: e, kind: "defect-class"})
	}
	return cands, true, false, ""
}

// earliestTime resolves the earliest of shas' commit times. ok is false when
// none resolve.
func earliestTime(shas []string, commitTime CommitTime) (time.Time, bool) {
	var earliest time.Time
	found := false
	for _, sha := range shas {
		t, ok := commitTime(sha)
		if !ok {
			continue
		}
		if !found || t.Before(earliest) {
			earliest, found = t, true
		}
	}
	return earliest, found
}

// evaluateFix resolves F's linkage and, when resolved, applies the ordering
// rule against each candidate earlier fix: E's fix commit time must be BEFORE
// F's earliest inducing commit time. Returns the first candidate that
// satisfies it (explicit-path candidates are tried before class-path ones,
// mirroring the tier-precedence "stop at the strongest evidence" shape).
func evaluateFix(f DefectFix, tr DefectTrace, allFixes []DefectFix, linkage RegressionLinkage, commitTime CommitTime) (resolved, isRefix, couldNotMeasure bool, reason string, matched *DefectFix, linkKind string) {
	expCands, expResolved, expCNM, expReason := explicitCandidates(linkage, f, allFixes)
	classCands, classResolved, classCNM, classReason := classCandidates(linkage, f, allFixes)

	resolved = expResolved || classResolved
	if !resolved {
		if expCNM || classCNM {
			var reasons []string
			if expCNM {
				reasons = append(reasons, expReason)
			}
			if classCNM {
				reasons = append(reasons, classReason)
			}
			return false, false, true, strings.Join(reasons, "; "), nil, ""
		}
		return false, false, false, "", nil, "" // legitimately no link on either path
	}

	if tr.TraceState != TraceTraced || len(tr.InducingCommits) == 0 {
		// Linked, but there is no inducing commit to order against (should not
		// arise for a traced fix — TraceTraced always carries ≥1 — but guarded
		// rather than assumed).
		return true, false, false, "", nil, ""
	}
	earliestInducing, ok := earliestTime(tr.InducingCommits, commitTime)
	if !ok {
		return true, false, true, "inducing-commit time unavailable: ordering rule cannot be evaluated", nil, ""
	}

	candidates := append(append([]candidateLink{}, expCands...), classCands...)
	for _, c := range candidates {
		et, ok := commitTime(c.e.FixCommitSHA)
		if !ok {
			continue
		}
		if et.Before(earliestInducing) {
			e := c.e
			return true, true, false, "", &e, c.kind
		}
	}
	// Linked but no candidate satisfies the ordering rule — the earlier fix
	// landed at or after F's inducer: not a re-fix (spec: "A later or
	// concurrent E is not a re-fix").
	return true, false, false, "", nil, ""
}

// ComputeRefix runs the join over a window's fixes/traces and produces the
// metric record (Task 2). fixes is the FULL identified fix set (candidates for
// "earlier fix" E include untraced fixes — E only needs a fix commit time, not
// a trace); traces is the traced set to evaluate as candidate re-fixes F.
func ComputeRefix(window string, fixes []DefectFix, traces []DefectTrace, linkage RegressionLinkage, commitTime CommitTime, minedAt time.Time) RefixRecord {
	rec := RefixRecord{Metric: MetricRefix, Window: window, MinedAt: minedAt}

	fixByCommit := make(map[string]DefectFix, len(fixes))
	for _, f := range fixes {
		fixByCommit[f.FixCommitSHA] = f
	}

	var tracedFixes []DefectFix
	var tracedTraces []DefectTrace
	for _, tr := range traces {
		if tr.TraceState != TraceTraced {
			continue
		}
		f, ok := fixByCommit[tr.FixCommit]
		if !ok {
			continue
		}
		tracedFixes = append(tracedFixes, f)
		tracedTraces = append(tracedTraces, tr)
	}
	rec.TracedFixCount = len(tracedFixes)

	if len(tracedFixes) == 0 {
		reason := "no traced fix in this window"
		rec.RefixCount = CouldNotMeasure[float64](reason)
		rec.RefixRate = CouldNotMeasure[float64](reason)
		rec.LinkageCoverage = CouldNotMeasure[float64](reason)
		return rec
	}

	var tierFixes []DefectFix
	resolvedCount, couldNotMeasureCount, refixCount := 0, 0, 0
	var firstCNMReason string

	for i, f := range tracedFixes {
		tr := tracedTraces[i]
		tierFixes = append(tierFixes, DefectFix{Identified: Measured(true), Tier: f.Tier})

		resolved, isRefix, cnm, reason, matchedE, linkKind := evaluateFix(f, tr, fixes, linkage, commitTime)
		if resolved {
			resolvedCount++
		}
		if cnm {
			couldNotMeasureCount++
			if firstCNMReason == "" {
				firstCNMReason = reason
			}
		}
		if isRefix {
			refixCount++
			rec.Refixes = append(rec.Refixes, RefixEntry{
				FixCommitSHA:        f.FixCommitSHA,
				FixPRNumber:         f.FixPRNumber,
				EarlierFixCommitSHA: matchedE.FixCommitSHA,
				EarlierFixPRNumber:  matchedE.FixPRNumber,
				LinkKind:            linkKind,
			})
		}
	}
	rec.TierComposition = ComputeTierComposition(tierFixes)

	// Every traced fix's linkage came back could-not-measure and NONE resolved:
	// the window's rate and coverage are undecidable, never a fabricated 0
	// (fact 5 / Verify #5 — "no regression-of: anywhere and no class prefix").
	if resolvedCount == 0 && couldNotMeasureCount == len(tracedFixes) {
		reason := "regression linkage could not be resolved for any traced fix in this window"
		if firstCNMReason != "" {
			reason += ": " + firstCNMReason
		}
		rec.RefixCount = CouldNotMeasure[float64](reason)
		rec.RefixRate = CouldNotMeasure[float64](reason)
		rec.LinkageCoverage = CouldNotMeasure[float64](reason)
		return rec
	}

	if resolvedCount == 0 {
		rec.LinkageCoverage = MeasuredZero[float64]()
	} else {
		rec.LinkageCoverage = Measured(float64(resolvedCount) / float64(len(tracedFixes)))
	}
	if refixCount == 0 {
		rec.RefixCount = MeasuredZero[float64]()
		rec.RefixRate = MeasuredZero[float64]()
	} else {
		rec.RefixCount = Measured(float64(refixCount))
		rec.RefixRate = Measured(float64(refixCount) / float64(len(tracedFixes)))
	}
	return rec
}

// WriteRefix appends rec to the metrics table through the existing Store,
// following the MetricRecord pattern (fact: "written through the existing
// Store... a new metric name in the metrics table").
func WriteRefix(store *Store, rec RefixRecord) error {
	return store.Append(KindMetric, rec)
}
