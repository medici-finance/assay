package main

// gitlab_boardread_test.go — the cross-package guard the GitLab board-read fix (issue #686)
// depends on. The GitLab backend's ListOpenChanges serves each open change in a DEGRADED
// shape: real metadata, MergeStateStatus left could-not-check (empty), and a SINGLE
// deskkit.GitLabRollupUnmapped rollup entry standing in for the unmapped CI rollup. The board
// must read that shape as "CI could-not-check, merge state could-not-check" — never as green,
// never as mergeable — so NEEDS-REVIEW / RE-REVIEW still fire while MERGE-NOW and FLIP stay
// withheld. This test pins that contract from the board's side: if ciState ever stopped
// counting the sentinel as unknown, or an empty mergeStateStatus stopped reading as unknown,
// the degraded shape would silently fail OPEN on a CI-less repo (compare
// TestNote_NoCIConfiguredIsNotAGreenVerdict_400N9, where an EMPTY rollup there reaches
// MERGE-NOW).

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestGitLabDegradedBoardShape(t *testing.T) {
	// The exact shape gitlabOpenChange emits, expressed through the board's own prBase: the
	// merge state is could-not-check (empty) and the rollup is one entry the board cannot
	// interpret (neither a CheckRun status nor a StatusContext state).
	p := prBase{
		Number: 7, Title: "add the thing", HeadRefOid: "abc123",
		MergeStateStatus:  "",
		StatusCheckRollup: []check{{TypeName: deskkit.GitLabRollupUnmapped}},
	}

	// The rollup sentinel is counted UNINTERPRETABLE — never a pass/pending/fail.
	pass, pending, fail, unknown := ciState(p)
	if unknown != 1 || pass != 0 || pending != 0 || fail != 0 {
		t.Fatalf("ciState of the GitLab could-not-check rollup = pass=%d pending=%d fail=%d unknown=%d, want 0/0/0/1",
			pass, pending, fail, unknown)
	}

	// No reviewer verdict at head → the review-dispatch trigger still fires. This is the whole
	// point of the degraded shape: NEEDS-REVIEW is restored on a GitLab adopter.
	if action, note := classify(buildClassifyInput(p, reviewState{}, true, "")); action != actNeedsReview {
		t.Errorf("no-review GitLab change → action=%s, want %s; note: %s", action, actNeedsReview, note)
	}

	// Approved at head — the case an EMPTY rollup would classify MERGE-NOW on a CI-less repo.
	// The could-not-check rollup must PREEMPT that on BOTH repo policies: CI is not established,
	// so no MERGE-NOW and no FLIP.
	approved := reviewState{ever: true, atHead: true, approved: true}
	for _, ciRequired := range []bool{true, false} {
		action, note := classify(buildClassifyInput(p, approved, ciRequired, ""))
		if action != actCIUnknown {
			t.Errorf("approved GitLab change (ciRequired=%v) → action=%s, want %s (a could-not-check rollup must block the flip); note: %s",
				ciRequired, action, actCIUnknown, note)
		}
		if action == actMergeNow || action == actFlip {
			t.Errorf("approved GitLab change (ciRequired=%v) reached %s — a could-not-check rollup must never flip", ciRequired, action)
		}
	}
}
