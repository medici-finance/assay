package main

// classdegrade_test.go — the CLASS-level regression for the per-PR-read-degrades-row-not-run fix.
//
// PR #1068 repaired ONE arm of classifyPR: the benign-merge compare, guarded so an
// unpinnable reviewed sha degrades its own row (RE-REVIEW) instead of failing the whole
// sweep with `compare needs both base and head`. Four further per-change reads in the same
// function propagated their own read failures the identical whole-sweep way: fetchReviews,
// detectNonCommitResolution, the own-files fetch feeding the benign-merge compare, and the
// changed-files read feeding risk classification. This file pins that every one of them now
// degrades its OWN row — never the sweep — and that the degraded row both lands on the safe
// side (never the benign/cleared outcome) and carries its own could-not-check reason in the
// RENDERED row text, not just a stderr line.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// classDegradeRoster binds the GitLab-shaped reviewer role (gl-reviewer, no [bot] suffix)
// and separates the bless authority/accountable-human set (ada) from a merely-trusted,
// non-human author (shared-agent) — needed so a PR with NO review at all reaches
// actNeedsReview rather than actHumanOwned (#177), which only exempts accountable humans.
const classDegradeRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=reviewer=gitlab:gl-reviewer:41987965
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
`

// TestSweepDegradesOneRowNotTheBoard is the sweep-level regression: several open PRs in one
// repo, exactly one carrying an EMPTY reviewed sha — the exact GitLab shape #1067 reports
// (a GitLab approval pins no commit by design) — still renders every row, and the affected
// row lands on the safe side, never the whole sweep.
func TestSweepDegradesOneRowNotTheBoard(t *testing.T) {
	installBoardRoster(t, classDegradeRoster)

	const (
		prApproved          = 50 // fully readable, approved+green+mergeable — must render MERGE-NOW
		prEmptySHA          = 51 // the GitLab shape #1067 reports: reviewed sha never established
		prUnreadableReviews = 52 // a hard read failure on the reviews call itself (not "no reviews")
	)
	headApproved := strings.Repeat("a", 40)
	headEmptySHA := strings.Repeat("b", 40)
	headUnreadable := strings.Repeat("c", 40)

	approved := greenRollupPR(prApproved, headApproved)
	approved.MergeStateStatus = "CLEAN" // otherwise an unread merge state withholds MERGE-NOW

	emptySHA := greenRollupPR(prEmptySHA, headEmptySHA)

	unreadable := greenRollupPR(prUnreadableReviews, headUnreadable)
	unreadable.Author.Login = "shared-agent" // trusted, but NOT the accountable-human set

	js, err := json.Marshal([]prBase{approved, emptySHA, unreadable})
	if err != nil {
		t.Fatalf("marshal PR list fixture: %v", err)
	}
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", string(js))

	stubForgeHooks(t, forgeHookSet{
		reviews: func(_ string, num int) ([]deskkit.Review, error) {
			switch num {
			case prApproved:
				return []deskkit.Review{{
					Author: deskkit.Account{Login: "gl-reviewer", ID: 41987965},
					State:  "APPROVED", CommitID: headApproved,
					Body: "Verdict: approve", SubmittedAt: "2026-09-14T16:00:00Z",
				}}, nil
			case prEmptySHA:
				return approvalWithNoSHA(), nil
			case prUnreadableReviews:
				return nil, errors.New("simulated: cannot read reviews")
			}
			return nil, fmt.Errorf("unstubbed PR %d in TestSweepDegradesOneRowNotTheBoard", num)
		},
		compare: func(_, base, head string) (*deskkit.RefComparison, error) {
			t.Fatalf("CompareRefs called with base=%q head=%q — the empty-reviewed-sha row has no "+
				"interval to compare; asking for one is the call that used to fail the whole sweep", base, head)
			return nil, nil
		},
	})

	part, err := sweepActionsRepo("example-org/tracker", nil, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("sweepActionsRepo returned %v — one PR whose verdict the forge could not pin to a "+
			"head must degrade its OWN row, never fail the whole sweep (the #1067 symptom: exit 6, "+
			"empty board, one line of diagnosis)", err)
	}
	if len(part.rows) != 3 {
		t.Fatalf("got %d rows, want 3 — every PR in the sweep must still render, including the two "+
			"UNAFFECTED by the degrading PR's read failure; rows=%+v", len(part.rows), part.rows)
	}

	byNum := map[int]actionRow{}
	for _, r := range part.rows {
		byNum[r.Number] = r
	}

	if r, ok := byNum[prApproved]; !ok || r.Action != actMergeNow {
		t.Errorf("PR %d (unaffected, fully readable) row=%+v, want action=%s — a sibling PR's "+
			"degrade must never touch an unrelated row", prApproved, r, actMergeNow)
	}
	shaDegraded, ok := byNum[prEmptySHA]
	if !ok {
		t.Fatalf("PR %d (the empty-reviewed-sha row) is missing from the rendered board entirely", prEmptySHA)
	}
	if shaDegraded.Action != actReReview {
		t.Errorf("PR %d (empty reviewed sha) action = %s, want %s — an unpinnable verdict must land "+
			"on the safe side, never the benign %s", prEmptySHA, shaDegraded.Action, actReReview, actMergeCurr)
	}
	reviewsDegraded, ok := byNum[prUnreadableReviews]
	if !ok {
		t.Fatalf("PR %d (unreadable reviews) is missing from the rendered board entirely", prUnreadableReviews)
	}
	if reviewsDegraded.Action == actMergeNow || reviewsDegraded.Action == actFlip || reviewsDegraded.Action == actMergeCurr {
		t.Errorf("PR %d (reviews could not be read) action = %s — must never read as cleared/benign",
			prUnreadableReviews, reviewsDegraded.Action)
	}
	if !strings.Contains(reviewsDegraded.Note, "DEGRADED:") {
		t.Errorf("PR %d row.Note = %q — must carry the could-not-check reason", prUnreadableReviews, reviewsDegraded.Note)
	}
}

// TestDegradedRowNeverReadsBenign is the negative-path proof task 4 asks for, aimed at the
// TWO new sites this brief adds (the own-files read and the compare read feeding the
// benign-merge check) rather than the arm PR #1068 already covers: with BOTH shas
// comparable (so the precondition guard does not fire), a read failure on either call must
// still degrade to RE-REVIEW, never let the row read as the benign MERGE-CURR.
func TestDegradedRowNeverReadsBenign(t *testing.T) {
	reviewed := strings.Repeat("1", 40)
	head := strings.Repeat("2", 40)

	cases := []struct {
		name  string
		hooks forgeHookSet
	}{
		{
			name: "own-files read fails",
			hooks: forgeHookSet{
				getPR: func(string, int) (*deskkit.PullRequest, error) {
					return nil, errors.New("simulated: cannot read PR metadata")
				},
			},
		},
		{
			name: "compare read fails",
			hooks: forgeHookSet{
				compare: func(string, string, string) (*deskkit.RefComparison, error) {
					return nil, errors.New("simulated: cannot compare refs")
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			installBoardRoster(t, classDegradeRoster)
			h := c.hooks
			h.reviews = func(string, int) ([]deskkit.Review, error) {
				return []deskkit.Review{{
					Author: deskkit.Account{Login: "gl-reviewer", ID: 41987965},
					State:  "APPROVED", CommitID: reviewed,
					Body: "Verdict: approve", SubmittedAt: "2026-09-14T16:00:00Z",
				}}, nil
			}
			stubForgeHooks(t, h)

			out, err := classifyPR("example-org/tracker", greenRollupPR(60, head), false, nil, nil, nil, time.Now())
			if err != nil {
				t.Fatalf("classifyPR returned %v — a read failure on ONE PR's benign-merge check must "+
					"degrade that row, never fail the whole sweep", err)
			}
			if out.row == nil {
				t.Fatal("classifyPR produced no row — a degraded PR must still be classified, on the safe side")
			}
			if out.row.Action == actMergeCurr {
				t.Fatalf("action = %s — a row whose own-files/compare read FAILED must never be read as "+
					"the benign keep-current merge; that is exactly the permissive side the SPOF note forbids",
					out.row.Action)
			}
			if out.row.Action != actReReview {
				t.Fatalf("action = %s, want %s — the safe side for an unreadable benign-merge input",
					out.row.Action, actReReview)
			}
		})
	}
}

// TestDegradedRowCarriesItsReason resolves the SPOF note's second-layer claim against the
// REAL rendered output, not just a count: the row's own Note text — what an operator
// actually reads on the board — must carry the could-not-check reason that produced the
// degrade, not only a line on stderr nobody watching the board sees.
func TestDegradedRowCarriesItsReason(t *testing.T) {
	installBoardRoster(t, classDegradeRoster)
	head := strings.Repeat("3", 40)

	stubForgeHooks(t, forgeHookSet{
		reviews: func(string, int) ([]deskkit.Review, error) {
			return approvalWithNoSHA(), nil // the reviewed sha is never established
		},
		compare: func(_, base, head string) (*deskkit.RefComparison, error) {
			t.Fatalf("CompareRefs called with base=%q head=%q — no interval to compare", base, head)
			return nil, nil
		},
	})

	out, err := classifyPR("example-org/tracker", greenRollupPR(61, head), false, nil, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("classifyPR: %v", err)
	}
	if out.row == nil {
		t.Fatal("classifyPR produced no row")
	}
	if !strings.Contains(out.row.Note, "DEGRADED:") {
		t.Fatalf("row.Note = %q — the rendered row text must carry a DEGRADED marker so an operator "+
			"reading the board (not stderr) can see this row was affected", out.row.Note)
	}
	if !strings.Contains(out.row.Note, "reviewed sha") {
		t.Fatalf("row.Note = %q — the degraded reason must name WHICH field could not be established, "+
			"not just that something failed (a degrade that does not say which field sends its reader "+
			"to the wrong forge surface)", out.row.Note)
	}
}
