package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #3093 — GitHub RE-ANCHORS an old review's commit_id to a later head. The reported case:
// CHANGES_REQUESTED at commit A, then an APPROVED submitted while A was still the head, which
// the forge now reports at head B — a merge commit whose committer date is seven hours AFTER
// the review was submitted. Under the #37 reduction a NEW commit_id reads as a genuine push,
// so that APPROVED cleared the standing CR and the row read approved-at-head / MERGE-NOW. A
// grant at head submitted BEFORE the head commit's committer date verified nothing there.
const (
	reanchorCommitA   = "a0a0a0a0b1b1b1b1c2c2c2c2d3d3d3d3e4e4e4e4"
	reanchorHead      = "b5b5b5b5c6c6c6c6d7d7d7d7e8e8e8e8f9f9f9f9"
	reanchorBlockAt   = "2026-10-14T00:22:30Z"
	reanchorApproveAt = "2026-10-15T14:41:06Z" // BEFORE the head commit existed
	reanchorHeadDate  = "2026-10-15T21:35:48Z" // head B's committer date
	reanchorFreshAt   = "2026-10-15T22:05:00Z" // AFTER the head commit — a genuine re-review
)

// reanchorRow runs `deskboard actions` over one draft, CLEAN, CI-green PR at head B whose
// head commit date is headDate, with the given reviewer-App reviews, and returns its row.
// The A..B interval touches the PR's own file, so a verdict pinned at A reads RE-REVIEW.
func reanchorRow(t *testing.T, headDate string, reviews ...string) actionRow {
	t.Helper()
	return reanchorRowWith(t, headDate, nil, reviews...)
}

// reanchorRowWith is reanchorRow with a hook run after the fake forge is installed (which
// resets the forge hooks), for a test that injects a read failure.
func reanchorRowWith(t *testing.T, headDate string, setup func(), reviews ...string) actionRow {
	t.Helper()
	installFakeGH(t)
	if setup != nil {
		setup()
	}
	repo := "example-org/tracker"
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":17,"title":"example change","body":"Fixes the example.","isDraft":true,`+
			`"author":{"login":"ada"},"headRefOid":"`+reanchorHead+`","headCommittedDate":`+jsonStr(headDate)+`,`+
			`"mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"docs/example.md"}]`)
	t.Setenv("DESKBOARD_GH_COMPARE_JSON", `{"status":"ahead","files":[{"filename":"docs/example.md"}]}`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", "["+strings.Join(reviews, ",")+"]")
	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	for _, r := range rep.Rows {
		if r.Number == 17 {
			return r
		}
	}
	t.Fatalf("row 17 missing from %s", out.String())
	return actionRow{}
}

func reanchorReview(state, commit, at string) string {
	return `{"user":{"login":"` + reviewerBotDisplay() + `"},"state":"` + state + `","commit_id":"` + commit +
		`","body":"review","submitted_at":"` + at + `"}`
}

// notApproved asserts the row did not read as approved at head: no merge/flip action, and no
// approved age (which the board sets only off an approval at head).
func notApproved(t *testing.T, row actionRow) {
	t.Helper()
	if row.Action == actMergeNow || row.Action == actFlip || row.Action == actReady {
		t.Fatalf("action = %s (%s) — a re-anchored APPROVED is not an approval at head", row.Action, row.Note)
	}
	if row.ApprovedAge != "" {
		t.Fatalf("approvedAge = %q (action %s) — the row read approved at head", row.ApprovedAge, row.Action)
	}
}

// The exact reported sequence: CR@A, then an APPROVED re-anchored to head B whose
// submitted_at predates B's committer date. The CR at A stands; the row is not approved.
func TestReanchoredApprovalDoesNotClearBlockOnBoard(t *testing.T) {
	row := reanchorRow(t, reanchorHeadDate,
		reanchorReview("CHANGES_REQUESTED", reanchorCommitA, reanchorBlockAt),
		reanchorReview("APPROVED", reanchorHead, reanchorApproveAt))
	notApproved(t, row)
	if row.Action != actReReview {
		t.Errorf("action = %s (%s), want %s — the governing verdict is the CR at A, and A..B touched "+
			"the PR's own files", row.Action, row.Note, actReReview)
	}
}

// The same stale APPROVED with no prior block: nobody has reviewed head B.
func TestReanchoredLoneApprovalIsNotApprovalOnBoard(t *testing.T) {
	row := reanchorRow(t, reanchorHeadDate, reanchorReview("APPROVED", reanchorHead, reanchorApproveAt))
	notApproved(t, row)
}

// Positive control: the APPROVED submitted AFTER head B's committer date is a genuine verdict
// at B — it clears the CR at A as a re-review after a push, and the row reads MERGE-NOW.
func TestApprovalAfterHeadCommitIsApprovedOnBoard(t *testing.T) {
	row := reanchorRow(t, reanchorHeadDate,
		reanchorReview("CHANGES_REQUESTED", reanchorCommitA, reanchorBlockAt),
		reanchorReview("APPROVED", reanchorHead, reanchorFreshAt))
	if row.Action != actMergeNow {
		t.Fatalf("action = %s (%s), want %s", row.Action, row.Note, actMergeNow)
	}
}

// Fail closed: the list read carries no usable head commit date and the per-commit read
// fails — the approval cannot be shown to postdate the head, so it is not approval at head.
func TestHeadCommitDateUnreadableIsNotApprovalOnBoard(t *testing.T) {
	row := reanchorRowWith(t, "not-a-date", func() {
		forgeHooks.getCommit = func(repo, sha string) (*deskkit.RepoCommit, error) {
			return nil, errors.New("HTTP 502 on the commit read")
		}
	}, reanchorReview("APPROVED", reanchorHead, reanchorFreshAt))
	notApproved(t, row)
	if !strings.Contains(row.Note, "DEGRADED") {
		t.Errorf("note does not say the row degraded: %s", row.Note)
	}
}
