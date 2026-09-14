package main

// unpinnedhead_test.go — the board must stay READABLE when a verdict cannot be pinned to a
// head.
//
// The reported shape: `deskboard actions` on a GitLab project exited 6 with an EMPTY stdout
// and the single line `compare needs both base and head`. No row was classified — not the
// affected merge request, not the others in the sweep — so the review desk could not see its
// queue at all.
//
// The cause is a seam between two correct halves. reduceReviews folds "the head advanced
// past the review" and "one of the two shas was never established" into one `!atHead`, on
// purpose (an unread sha must never read as at-head). The MERGE-CURR arm then treated that
// single bool as if it always meant the first, and asked for a compare between the reviewed
// sha and the head — which, for the second, has no endpoints. changedFilesBetween answers
// that with Unverifiable, and classifyPR propagates a row error into a whole-sweep failure.
//
// It is reachable wherever a forge declines to pin a verdict, and GitLab does so routinely
// and BY DESIGN: an approval there carries no sha and (unless the project resets approvals
// on push) survives a push, so ReviewsAtHead leaves CommitID empty rather than stamping the
// current head and manufacturing the at-head evidence the flip gate exists to require.
// Those verdicts only started reaching this reduction once a role's expected login resolved
// per-forge, which is why this arm had never been exercised on GitLab before.
//
// So the tests below pin the CONSUMER contract, not a forge detail: an unpinnable sha is a
// could-not-check that degrades ONE ROW to the safe side, exactly as a truncated diff does,
// and never takes the sweep down with it.

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const unpinnedHeadRepo = "example-org/tracker"

// unpinnedHead is a plausible head sha for the merge request under test. The reviewed sha is
// the one that goes missing, never this.
const unpinnedHead = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c"

// greenRollupPR builds the PR the classifier sees: a trusted human author (so the trust gate
// admits it without a blessing read) and a single passing check (so the rollup is not 0/0/0
// and the zero-CI probe stays out of a test about review shas).
func greenRollupPR(num int, head string) prBase {
	var p prBase
	p.Number = num
	p.Title = "a change under review"
	p.State = "OPEN"
	p.Author.Login = "ada"
	p.HeadRefOid = head
	p.StatusCheckRollup = []check{{
		TypeName:   "CheckRun",
		Name:       "build",
		Status:     "COMPLETED",
		Conclusion: "SUCCESS",
	}}
	return p
}

// approvalWithNoSHA is the verdict at the centre of this: real, attributed to the rostered
// reviewer, and carrying NO commit id — the forge reporting that it could not establish
// which head this verdict was given at.
func approvalWithNoSHA() []deskkit.Review {
	return []deskkit.Review{{
		ID:          1,
		Author:      deskkit.Account{Login: "gl-reviewer", ID: 41987965},
		State:       "APPROVED",
		CommitID:    "",
		Body:        "Verdict: approve",
		SubmittedAt: "2026-09-14T16:28:00Z",
	}}
}

// TestUnpinnedReviewSHADegradesRowNotSweep is the regression itself. Before the fix,
// classifyPR returned Unverifiable("compare needs both base and head") here, and because
// sweepActionsRepo fails closed on any per-PR error, that one unpinnable verdict exited the
// entire sweep 6 with nothing on stdout.
func TestUnpinnedReviewSHADegradesRowNotSweep(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabReviewer)
	stubForgeHooks(t, forgeHookSet{
		reviews: func(string, int) ([]deskkit.Review, error) { return approvalWithNoSHA(), nil },
		compare: func(_, base, head string) (*deskkit.RefComparison, error) {
			t.Fatalf("CompareRefs was called with base=%q head=%q — there is no interval to "+
				"compare when the reviewed sha was never established; asking for one is the "+
				"call that used to fail the sweep", base, head)
			return nil, nil
		},
	})

	out, err := classifyPR(unpinnedHeadRepo, greenRollupPR(3, unpinnedHead), false, nil, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("classifyPR returned %v — one merge request whose verdict the forge could not "+
			"pin to a head must not fail the whole sweep; every other row in the repo is lost "+
			"with it and the desk sees an empty board", err)
	}
	if out.row == nil {
		t.Fatal("classifyPR produced no action row — the row must still be CLASSIFIED (on the " +
			"safe side), not dropped, or the merge request silently leaves the queue")
	}
	if out.row.Action != actReReview {
		t.Fatalf("action = %s, want %s — with no reviewed sha nobody can claim the PR's own "+
			"files are unchanged since review, so the row must NOT take the benign %s arm",
			out.row.Action, actReReview, actMergeCurr)
	}
}

// TestUnpinnedHeadSHADegradesRowNotSweep is the mirror: the reviewed sha is known and the
// HEAD is the unreadable one. Same could-not-check, same degrade — the guard must not be
// written around one field.
func TestUnpinnedHeadSHADegradesRowNotSweep(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabReviewer)
	reviewed := "1122334455667788990011223344556677889900"
	stubForgeHooks(t, forgeHookSet{
		reviews: func(string, int) ([]deskkit.Review, error) {
			r := approvalWithNoSHA()
			r[0].CommitID = reviewed
			return r, nil
		},
		compare: func(_, base, head string) (*deskkit.RefComparison, error) {
			t.Fatalf("CompareRefs was called with base=%q head=%q — an unreadable head is no "+
				"more comparable than an unreadable reviewed sha", base, head)
			return nil, nil
		},
	})

	out, err := classifyPR(unpinnedHeadRepo, greenRollupPR(4, ""), false, nil, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("classifyPR returned %v — an unread head sha is a could-not-check for ONE row, "+
			"never a reason to fail the sweep", err)
	}
	if out.row == nil || out.row.Action != actReReview {
		t.Fatalf("row = %+v — want a classified %s row", out.row, actReReview)
	}
}

// TestPinnedShasStillCompareForMergeCurr is the no-regression half: where both endpoints ARE
// established, the benign-merge compare must still run and must still be able to reach
// MERGE-CURR. A fix that reached the first two tests by simply never comparing would pass
// them and quietly retire the keep-current classification.
func TestPinnedShasStillCompareForMergeCurr(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabReviewer)
	reviewed := "1122334455667788990011223344556677889900"
	compared := false

	// The PR's own contribution is one file; the interval since the review touches a
	// DIFFERENT one (a keep-current merge), so the sets do not intersect.
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"tools/desk/own.go"}]`)
	t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `{"state":"open","changed_files":1}`)
	stubForgeHooks(t, forgeHookSet{
		reviews: func(string, int) ([]deskkit.Review, error) {
			r := approvalWithNoSHA()
			r[0].CommitID = reviewed
			return r, nil
		},
		compare: func(_, base, head string) (*deskkit.RefComparison, error) {
			compared = true
			if base != reviewed || head != unpinnedHead {
				t.Errorf("CompareRefs(base=%q, head=%q), want (%q, %q) — the compare must span "+
					"the reviewed sha to the current head", base, head, reviewed, unpinnedHead)
			}
			return &deskkit.RefComparison{
				Status: "ahead",
				Files:  []deskkit.ChangedFile{{Filename: "docs/elsewhere.md"}},
			}, nil
		},
	})

	out, err := classifyPR(unpinnedHeadRepo, greenRollupPR(5, unpinnedHead), false, nil, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("classifyPR: %v", err)
	}
	if !compared {
		t.Fatal("CompareRefs was never called — with BOTH shas established the benign-merge " +
			"check must still run; skipping it would make every advanced head a re-review")
	}
	if out.row == nil || out.row.Action != actMergeCurr {
		t.Fatalf("row = %+v — want %s: the PR's own files are untouched by the interval since "+
			"the review", out.row, actMergeCurr)
	}
}

// TestShasComparableSeparatesUnknownFromDifferent pins the predicate the arm turns on,
// alongside sameHead, because the whole defect was that ONE of these two questions was being
// asked where BOTH were needed.
func TestShasComparableSeparatesUnknownFromDifferent(t *testing.T) {
	a, b := "aaaa", "bbbb"
	cases := []struct {
		reviewed, head   string
		same, comparable bool
	}{
		{a, a, true, true},    // at head
		{a, b, false, true},   // head advanced — a real interval
		{"", b, false, false}, // verdict not pinned — no interval
		{a, "", false, false}, // head unread — no interval
		{"", "", false, false},
	}
	for _, c := range cases {
		if got := sameHead(c.reviewed, c.head); got != c.same {
			t.Errorf("sameHead(%q,%q) = %v, want %v", c.reviewed, c.head, got, c.same)
		}
		if got := shasComparable(c.reviewed, c.head); got != c.comparable {
			t.Errorf("shasComparable(%q,%q) = %v, want %v — an absent sha is not a different "+
				"sha, and only the second bounds a compare", c.reviewed, c.head, got, c.comparable)
		}
	}
}

// TestUnreadableSHAFieldsNamesTheMissingOne pins the diagnostic. The reported symptom was a
// bare `compare needs both base and head` that named neither field and neither PR, which is
// why the operator had no way to tell an unpinned verdict from an unread head.
func TestUnreadableSHAFieldsNamesTheMissingOne(t *testing.T) {
	if got := unreadableSHAFields("abc", "def"); got != "" {
		t.Errorf("unreadableSHAFields with both shas present = %q, want \"\" — the string is "+
			"only built for a row that is actually degrading", got)
	}
	missingReviewed := unreadableSHAFields("", "def")
	if missingReviewed == "" || missingReviewed == unreadableSHAFields("abc", "") {
		t.Errorf("an absent reviewed sha (%q) and an absent head (%q) must read DIFFERENTLY — "+
			"they send the reader to different forge surfaces",
			missingReviewed, unreadableSHAFields("abc", ""))
	}
}
