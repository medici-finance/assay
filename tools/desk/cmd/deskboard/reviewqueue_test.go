package main

// reviewqueue_test.go — the consumer half of desktools-v2 brief 09: the actions sweep reads
// its open PRs AND their reviews through ONE ReviewQueueSnapshot instead of the open-PR list
// plus one ReviewsAtHead per PR, with its output unchanged.

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestSweepReviewQueueSnapshotMatchesPerItem runs the SAME queue through the sweep twice —
// once with the snapshot carrying no reviews (every PR read per-item, the pre-snapshot path)
// and once with the snapshot carrying every PR's reviews in full — and requires the two
// partials to be identical while the snapshot run makes ZERO per-item review reads. The
// queue spans the review shapes the reduction distinguishes: approved at head, changes
// requested at head, an approval at an older head, an approval with no pinned head, and
// no review at all.
func TestSweepReviewQueueSnapshotMatchesPerItem(t *testing.T) {
	const repo = "example-org/tracker"
	head := func(c string) string { return strings.Repeat(c, 40) }
	reviewer := deskkit.Account{Login: "gl-reviewer", ID: 41987965}

	type pr struct {
		num     int
		head    string
		reviews []deskkit.Review
	}
	queue := []pr{
		{60, head("a"), []deskkit.Review{{ID: 1, Author: reviewer, State: "APPROVED", CommitID: head("a"),
			Body: "Verdict: approve", SubmittedAt: "2026-09-14T16:00:00Z"}}},
		{61, head("b"), []deskkit.Review{{ID: 2, Author: reviewer, State: "CHANGES_REQUESTED", CommitID: head("b"),
			Body: "Verdict: request-changes", SubmittedAt: "2026-09-14T16:01:00Z"}}},
		{62, head("c"), []deskkit.Review{{ID: 3, Author: reviewer, State: "APPROVED", CommitID: head("9"),
			Body: "Verdict: approve", SubmittedAt: "2026-09-14T16:02:00Z"}}},
		{63, head("d"), approvalWithNoSHA()},
		{64, head("e"), nil},
	}
	var prs []prBase
	byNum := map[int][]deskkit.Review{}
	for _, q := range queue {
		p := greenRollupPR(q.num, q.head)
		p.MergeStateStatus = "CLEAN"
		p.Author.Login = "shared-agent" // trusted, not the accountable-human set (#177)
		prs = append(prs, p)
		byNum[q.num] = q.reviews
	}
	js, err := json.Marshal(prs)
	if err != nil {
		t.Fatalf("marshal PR list fixture: %v", err)
	}
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	sweep := func(snapshot bool) (actionsPartial, int) {
		t.Helper()
		installBoardRoster(t, classDegradeRoster)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON", string(js))
		// The sweep classifies PRs concurrently, so the per-item read count is atomic.
		var perItem atomic.Int64
		hooks := forgeHookSet{
			reviews: func(_ string, num int) ([]deskkit.Review, error) {
				perItem.Add(1)
				return byNum[num], nil
			},
		}
		if snapshot {
			hooks.queueReviews = func(_ string, num int) ([]deskkit.Review, error) { return byNum[num], nil }
		}
		stubForgeHooks(t, hooks)
		part, err := sweepActionsRepo(repo, nil, nil, nil, now)
		if err != nil {
			t.Fatalf("sweepActionsRepo (snapshot=%v): %v", snapshot, err)
		}
		return part, int(perItem.Load())
	}

	want, perItemCalls := sweep(false)
	if perItemCalls != len(queue) {
		t.Fatalf("per-item baseline made %d review reads, want %d — the fallback path is not what it was", perItemCalls, len(queue))
	}
	if len(want.rows) != len(queue) {
		t.Fatalf("per-item baseline rendered %d rows, want %d: %+v", len(want.rows), len(queue), want.rows)
	}
	got, snapCalls := sweep(true)
	if snapCalls != 0 {
		t.Errorf("snapshot sweep made %d per-item review reads, want 0 — the snapshot's reviews were not used", snapCalls)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("snapshot sweep output differs from the per-item sweep\n got: %+v\nwant: %+v", got, want)
	}
	// The five shapes must not all collapse to one action, or identity proves nothing.
	actions := map[string]bool{}
	for _, r := range want.rows {
		actions[r.Action] = true
	}
	if len(actions) < 3 {
		t.Errorf("fixture exercised only %d distinct actions (%v) — widen it", len(actions), actions)
	}
}

// TestFetchReviewQueueKeepsOpenPRFailureText pins that the snapshot read fails the run with
// the same "cannot read open PRs for <repo>" text fetchOpenPRs used, so the out-of-installation
// scoping check and the operator-facing message are unchanged by the migration.
func TestFetchReviewQueueKeepsOpenPRFailureText(t *testing.T) {
	installFakeForge(t)
	t.Setenv("DESKBOARD_GH_FAIL_REPO", "example-org/tracker")
	_, _, _, err := fetchReviewQueue("example-org/tracker")
	if err == nil || !strings.Contains(err.Error(), "cannot read open PRs for example-org/tracker") {
		t.Fatalf("fetchReviewQueue error = %v, want the open-PR read failure text", err)
	}
}
