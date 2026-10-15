package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #3093 on the lane's merge step. A ready PR takes a keep-current merge (head B); the forge
// re-anchors the reviewer App's APPROVED from head A onto B. Nothing in that path re-runs
// `deskpost ready`, so the merge step's own reviewer-approved check is the only gate between
// that APPROVED — submitted before B existed — and a merge of B nobody reviewed.
const (
	reanchorCommitA   = "a0a0a0a0b1b1b1b1c2c2c2c2d3d3d3d3e4e4e4e4"
	reanchorBlockAt   = "2026-10-14T00:22:30Z"
	reanchorApproveAt = "2026-10-15T14:41:06Z" // BEFORE the head commit existed
	reanchorHeadDate  = "2026-10-15T21:35:48Z" // head B's committer date
	reanchorFreshAt   = "2026-10-15T22:05:00Z" // AFTER the head commit — a genuine re-review
)

func reanchorLaneReviews(approveAt string) []deskkit.Review {
	return []deskkit.Review{
		{ID: 1, Author: deskkit.Account{Login: fxReviewer}, State: "CHANGES_REQUESTED", CommitID: reanchorCommitA,
			SubmittedAt: reanchorBlockAt},
		{ID: 2, Author: deskkit.Account{Login: fxReviewer}, State: "APPROVED", CommitID: fxHead,
			SubmittedAt: approveAt},
	}
}

// The reported sequence reaches reviewer-approved: an APPROVED the forge reports at head B,
// submitted before B's committer date, is not a verdict at B — the merge is refused there.
func TestAutoLane_MergeRefusesReanchoredApproval(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.headCommittedAt = reanchorHeadDate
	e.fg.reviews = reanchorLaneReviews(reanchorApproveAt)[1:]
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condReviewerApproved) ||
		!strings.Contains(stderr, "#3093") {
		t.Fatalf("exit %d stderr %q — want refused: %s naming #3093", code, stderr, condReviewerApproved)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
}

// Positive control: the same APPROVED submitted AFTER head B's committer date is a verdict at
// B — reviewer-approved passes and the dry run reaches the merge.
func TestAutoLane_MergeAcceptsApprovalAfterHeadCommit(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.headCommittedAt = reanchorHeadDate
	e.fg.reviews = reanchorLaneReviews(reanchorFreshAt)[1:]
	code, stdout, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitOK || !strings.Contains(stdout, "would merge") {
		t.Fatalf("exit %d stdout %q stderr %q — want the dry run to reach the merge", code, stdout, stderr)
	}
}

// Fail closed: the head commit's date cannot be read, so the APPROVED cannot be shown to
// postdate the head — could-not-check, never a merge.
func TestAutoLane_MergeHeadCommitDateUnreadableFailsClosed(t *testing.T) {
	for name, setup := range map[string]func(*fakeForge){
		"read error": func(f *fakeForge) { f.fail["GetCommit"] = true },
		"no date":    func(f *fakeForge) { f.headCommittedAt = "" },
	} {
		t.Run(name, func(t *testing.T) {
			e := install(t, fixtureLaneKeys, rulingsSigned)
			setup(e.fg)
			code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
			if code == deskkit.ExitOK || !strings.Contains(stderr, condReviewerApproved) {
				t.Fatalf("exit %d stderr %q — an unreadable head date must refuse at %s", code, stderr, condReviewerApproved)
			}
		})
	}
}
