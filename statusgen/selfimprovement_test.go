package main

import (
	"strings"
	"testing"
)

// siCfg is a config with no roster (pure) — team set is explicit so the
// classification math is exercised without an env/roster read. Mirrors
// baseIssueCfg (issues_test.go).
func siCfg() issueMetricConfig {
	return issueMetricConfig{
		Now:        issueNow,
		OrgAccount: "the-org",
		TeamLogins: map[string]bool{"teammate": true},
	}
}

// TestSelfImprovementFullyHealed: agent-raised, no comments/reopens, an
// agent-authored merged fixing PR — SELF-HEALED.
func TestSelfImprovementFullyHealed(t *testing.T) {
	rec := issueMetricRecord{Number: 1, State: "CLOSED", Author: "assay-worker-app[bot]"}
	det := selfImprovementDetail{FixingPRFound: true, FixingPRAuthor: "assay-worker-app[bot]"}
	v := classifySelfImprovement(rec, det, siCfg())
	if !v.SelfHealed {
		t.Fatalf("expected SelfHealed, got %+v", v)
	}
	if v.Touch.any() {
		t.Errorf("fully self-healed issue should carry no touch, got %+v", v.Touch)
	}
}

// TestSelfImprovementSteeredByComment: agent-raised, a human comment (no
// needs-decision label), an agent-authored fixing PR — HUMAN-TOUCHED
// (steered-by-comment), even though (a) and (b) would otherwise qualify.
func TestSelfImprovementSteeredByComment(t *testing.T) {
	rec := issueMetricRecord{Number: 2, State: "CLOSED", Author: "assay-worker-app[bot]"}
	det := selfImprovementDetail{
		CommentAuthors: []string{"a-human"},
		FixingPRFound:  true,
		FixingPRAuthor: "assay-worker-app[bot]",
	}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed {
		t.Fatalf("a human-steered issue must not be self-healed: %+v", v)
	}
	if !v.Touch.SteeredByComment {
		t.Errorf("expected SteeredByComment touch, got %+v", v.Touch)
	}
	if v.Touch.Decided {
		t.Errorf("no needs-decision label — should not classify as Decided: %+v", v.Touch)
	}
}

// TestSelfImprovementDecidedViaNeedsDecision: same as steered-by-comment but
// the issue carries needs-decision — the human comment classifies as Decided,
// not SteeredByComment.
func TestSelfImprovementDecidedViaNeedsDecision(t *testing.T) {
	rec := issueMetricRecord{Number: 3, State: "CLOSED", Author: "assay-worker-app[bot]", Labels: []string{"needs-decision"}}
	det := selfImprovementDetail{
		CommentAuthors: []string{"a-human"},
		FixingPRFound:  true,
		FixingPRAuthor: "assay-worker-app[bot]",
	}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed {
		t.Fatalf("a decided issue must not be self-healed: %+v", v)
	}
	if !v.Touch.Decided {
		t.Errorf("expected Decided touch, got %+v", v.Touch)
	}
	if v.Touch.SteeredByComment {
		t.Errorf("needs-decision comment should classify as Decided, not SteeredByComment: %+v", v.Touch)
	}
}

// TestSelfImprovementHumanRaised: a non-team human author — HUMAN-TOUCHED
// (raised), regardless of how it was fixed.
func TestSelfImprovementHumanRaised(t *testing.T) {
	rec := issueMetricRecord{Number: 4, State: "CLOSED", Author: "outsider"}
	det := selfImprovementDetail{FixingPRFound: true, FixingPRAuthor: "assay-worker-app[bot]"}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed {
		t.Fatalf("a human-raised issue must not be self-healed: %+v", v)
	}
	if !v.Touch.Raised {
		t.Errorf("expected Raised touch, got %+v", v.Touch)
	}
}

// TestSelfImprovementTeamHumanIsStillHuman: a TEAM human author is still a
// human touch for THIS metric — distinct from --issues' internal/external
// axis (brief-03 ground rules).
func TestSelfImprovementTeamHumanIsStillHuman(t *testing.T) {
	rec := issueMetricRecord{Number: 5, State: "CLOSED", Author: "teammate"}
	det := selfImprovementDetail{FixingPRFound: true, FixingPRAuthor: "assay-worker-app[bot]"}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed || !v.Touch.Raised {
		t.Errorf("a team human author is still HUMAN-TOUCHED for self-improvement: %+v", v)
	}
}

// TestSelfImprovementHumanAuthoredFix: agent-raised, no comments/reopens, but
// the merged fixing PR's author is human — HUMAN-TOUCHED (fixed-by-human).
func TestSelfImprovementHumanAuthoredFix(t *testing.T) {
	rec := issueMetricRecord{Number: 6, State: "CLOSED", Author: "assay-worker-app[bot]"}
	det := selfImprovementDetail{FixingPRFound: true, FixingPRAuthor: "a-human"}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed {
		t.Fatalf("a human-authored fix must not be self-healed: %+v", v)
	}
	if !v.Touch.FixedByHuman {
		t.Errorf("expected FixedByHuman touch, got %+v", v.Touch)
	}
}

// TestSelfImprovementHumanReopened: an agent-raised, agent-fixed issue that a
// human later REOPENED is HUMAN-TOUCHED (reopened), even though the raise+fix
// legs alone would qualify as self-healed.
func TestSelfImprovementHumanReopened(t *testing.T) {
	rec := issueMetricRecord{Number: 7, State: "CLOSED", Author: "assay-worker-app[bot]"}
	det := selfImprovementDetail{
		ReopenActors:   []string{"a-human"},
		FixingPRFound:  true,
		FixingPRAuthor: "assay-worker-app[bot]",
	}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed {
		t.Fatalf("a human-reopened issue must not be self-healed: %+v", v)
	}
	if !v.Touch.Reopened {
		t.Errorf("expected Reopened touch, got %+v", v.Touch)
	}
}

// TestSelfImprovementManualCloseNoFixingPR: no fixing PR was found at all, and
// a human closed the issue directly — HUMAN-TOUCHED (reopened/manual-close
// bucket), and (since there is no fixing PR) never self-healed.
func TestSelfImprovementManualCloseNoFixingPR(t *testing.T) {
	rec := issueMetricRecord{Number: 8, State: "CLOSED", Author: "assay-worker-app[bot]"}
	det := selfImprovementDetail{ManualCloseActors: []string{"a-human"}}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed {
		t.Fatalf("no fixing PR ⇒ never self-healed: %+v", v)
	}
	if !v.Touch.Reopened {
		t.Errorf("expected the manual-close signal to land in the Reopened bucket, got %+v", v.Touch)
	}
}

// TestSelfImprovementUnclassifiedWhenNeitherBucketApplies: an agent closed the
// issue directly (no fixing PR, no human signal anywhere) — neither
// self-healed (no fixing PR ⇒ condition b fails) nor human-touched.
func TestSelfImprovementUnclassifiedWhenNeitherBucketApplies(t *testing.T) {
	rec := issueMetricRecord{Number: 9, State: "CLOSED", Author: "assay-worker-app[bot]"}
	det := selfImprovementDetail{ManualCloseActors: []string{"assay-worker-app[bot]"}}
	v := classifySelfImprovement(rec, det, siCfg())
	if v.SelfHealed {
		t.Errorf("no fixing PR ⇒ never self-healed: %+v", v)
	}
	if v.Touch.any() {
		t.Errorf("an all-agent lifecycle carries no human touch: %+v", v.Touch)
	}
	if v.Classified {
		t.Errorf("neither bucket applies — expected Classified=false, got %+v", v)
	}
}

// TestSelfImprovementMergeIsNotATouch is THE load-bearing test: an
// agent-raised issue fixed by an agent-authored PR that a HUMAN merged (i.e. a
// human actor appears on the ManualCloseActors / merge signal) still counts as
// SELF-HEALED — the standing human merge gate is never read as a touch. This
// models the merge by putting the human merger in ManualCloseActors (the
// signal a naive classifier might read as "closed by a human") while a fixing
// PR IS found; classifySelfImprovement must ignore ManualCloseActors whenever
// FixingPRFound is true and judge the fix ONLY by the fixing PR's author.
func TestSelfImprovementMergeIsNotATouch(t *testing.T) {
	rec := issueMetricRecord{Number: 10, State: "CLOSED", Author: "assay-worker-app[bot]"}
	det := selfImprovementDetail{
		FixingPRFound:     true,
		FixingPRAuthor:    "assay-worker-app[bot]", // the FIX is agent-authored
		ManualCloseActors: []string{"a-human"},     // the human who MERGED it
	}
	v := classifySelfImprovement(rec, det, siCfg())
	if !v.SelfHealed {
		t.Fatalf("the standing human merge gate must NOT count as a touch — expected SelfHealed, got %+v", v)
	}
	if v.Touch.any() {
		t.Errorf("merging is not a touch — expected no touch flags, got %+v", v.Touch)
	}
}

// TestSelfImprovementOpenIssuesNeverClassified: only RESOLVED (closed) issues
// are classified — computeSelfImprovementMetrics must skip open ones.
func TestSelfImprovementOpenIssuesNeverClassified(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, State: "OPEN", Author: "assay-worker-app[bot]"},
	}
	rep := computeSelfImprovementMetrics(recs, map[issueKey]selfImprovementDetail{}, siCfg())
	if rep.Resolved != 0 || rep.SelfHealed != 0 || rep.HumanTouched != 0 {
		t.Errorf("open issues must not be classified: %+v", rep)
	}
}

// TestSelfImprovementRateMath exercises the aggregate rate over a small mixed
// corpus, plus the by-type breakdown counts.
func TestSelfImprovementRateMath(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, Repo: "r", State: "CLOSED", Author: "assay-worker-app[bot]"}, // self-healed
		{Number: 2, Repo: "r", State: "CLOSED", Author: "assay-worker-app[bot]"}, // self-healed
		{Number: 3, Repo: "r", State: "CLOSED", Author: "outsider"},             // human-touched (raised)
		{Number: 4, Repo: "r", State: "CLOSED", Author: "assay-worker-app[bot]"}, // unclassified (no fixing PR, no touch)
	}
	details := map[issueKey]selfImprovementDetail{
		{Repo: "r", Number: 1}: {FixingPRFound: true, FixingPRAuthor: "assay-worker-app[bot]"},
		{Repo: "r", Number: 2}: {FixingPRFound: true, FixingPRAuthor: "assay-worker-app[bot]"},
		{Repo: "r", Number: 3}: {FixingPRFound: true, FixingPRAuthor: "assay-worker-app[bot]"},
		{Repo: "r", Number: 4}: {},
	}
	rep := computeSelfImprovementMetrics(recs, details, siCfg())
	if rep.Resolved != 4 {
		t.Fatalf("resolved = %d, want 4", rep.Resolved)
	}
	if rep.SelfHealed != 2 {
		t.Errorf("selfHealed = %d, want 2", rep.SelfHealed)
	}
	if rep.HumanTouched != 1 {
		t.Errorf("humanTouched = %d, want 1", rep.HumanTouched)
	}
	if rep.Unclassified != 1 {
		t.Errorf("unclassified = %d, want 1", rep.Unclassified)
	}
	// rate = 2 / (2+1) = 0.67
	if rep.SelfImprovementRate != "0.67" {
		t.Errorf("rate = %q, want 0.67", rep.SelfImprovementRate)
	}
	if rep.HumanTouchedByType.Raised != 1 {
		t.Errorf("humanTouchedByType.raised = %d, want 1", rep.HumanTouchedByType.Raised)
	}
}

// TestSelfImprovementEmptyCorpusDegrades: zero resolved issues renders "n/a",
// never a divide-by-zero or a fabricated 0/0 rate.
func TestSelfImprovementEmptyCorpusDegrades(t *testing.T) {
	rep := computeSelfImprovementMetrics(nil, map[issueKey]selfImprovementDetail{}, siCfg())
	if rep.SelfImprovementRate != "n/a" {
		t.Errorf("empty rate = %q, want n/a", rep.SelfImprovementRate)
	}
}

// TestSelfImprovementBannerAndSegmentsRender proves the text output carries
// the DIAGNOSTIC banner, both segment words, and the merge-is-not-a-touch
// caveat verbatim enough to satisfy the Verify-table grep.
func TestSelfImprovementBannerAndSegmentsRender(t *testing.T) {
	rep := computeSelfImprovementMetrics(nil, map[issueKey]selfImprovementDetail{}, siCfg())
	text := renderSelfImprovementText(rep)
	if !strings.Contains(text, "DIAGNOSTIC") {
		t.Errorf("text output missing DIAGNOSTIC banner")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "self-healed") {
		t.Errorf("text output missing the self-healed segment")
	}
	if !strings.Contains(lower, "human-touched") {
		t.Errorf("text output missing the human-touched segment")
	}
	if !strings.Contains(lower, "merge") || !strings.Contains(lower, "not a touch") {
		t.Errorf("text output missing the merge-gate-is-not-a-touch caveat")
	}
}

// TestSelfImprovementGatherDetailsExcludesFailures: a per-issue detail-fetch
// failure excludes that issue from classification entirely (C4 — could-not-
// check is never silently folded into a bucket) and is named in the
// could-not-check list.
func TestSelfImprovementGatherDetailsExcludesFailures(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, Repo: "r", OwnerRepo: "o/r", State: "CLOSED", Author: "assay-worker-app[bot]"},
		{Number: 2, Repo: "r", OwnerRepo: "o/r", State: "CLOSED", Author: "assay-worker-app[bot]"},
		{Number: 3, Repo: "r", OwnerRepo: "o/r", State: "OPEN", Author: "assay-worker-app[bot]"},
	}
	fetch := func(ownerRepo string, number int) (selfImprovementDetail, error) {
		if number == 2 {
			return selfImprovementDetail{}, errBoom
		}
		return selfImprovementDetail{FixingPRFound: true, FixingPRAuthor: "assay-worker-app[bot]"}, nil
	}
	classifiable, details, couldNotCheck := gatherSelfImprovementDetails(recs, fetch)
	if len(classifiable) != 1 || classifiable[0].Number != 1 {
		t.Fatalf("classifiable = %+v, want just #1 (open #3 skipped, failed #2 excluded)", classifiable)
	}
	if len(details) != 1 {
		t.Fatalf("details = %+v, want exactly 1 entry", details)
	}
	if len(couldNotCheck) != 1 || !strings.Contains(couldNotCheck[0], "#2") {
		t.Fatalf("couldNotCheck = %v, want one entry naming #2", couldNotCheck)
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

const errBoom = staticErr("boom")
