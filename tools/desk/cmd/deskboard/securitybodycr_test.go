package main

// securitybodycr_test.go — the board's half of the security lane's marked body-only
// clearance (#5092). The board calls the same deskkit decision deskflip's gate does, so a
// row the gate would flip reads approved here, and a row it would refuse stays blocking.

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func sbdSecFail(marked bool, at string) review {
	body := "## Security review\n\nThe body claims an allowlist the diff does not add.\n\nSecurity-Review: fail"
	if marked {
		body += "\nBlocked-On-Body: " + beBoardFinding + " " + deskkit.PRBodyDigest(beBoardBodyBefore)
	}
	return beBoardReview("CHANGES_REQUESTED", body, at)
}

func sbdSecPass(at string) review {
	return beBoardReview("COMMENTED", "## Security review\n\nRe-read the live body.\n\nSecurity-Review: pass\n"+
		"Body-Reread-Digest: "+deskkit.PRBodyDigest(beBoardBodyAfter), at)
}

func sbdApprove(at string) review {
	return beBoardReview("APPROVED", "Correctness: looks right.", at)
}

// The #5090 shape, marked: fail, correctness APPROVE, body edit (18:10), then the pass.
//
// regression: #5092
func TestSecBodyBoardLifts(t *testing.T) {
	reviews := []review{sbdSecFail(true, "2026-07-14T18:03:22Z"), sbdApprove("2026-07-14T18:05:00Z"),
		sbdSecPass("2026-07-14T18:24:55Z")}
	st := beBoardReduce(reviews, beBoardBodyAfter)
	if !st.approved || st.blocking || !st.bodyEditReverified {
		t.Fatalf("marked security fail + later pass: approved=%v blocking=%v reverified=%v", st.approved, st.blocking,
			st.bodyEditReverified)
	}
}

// APPROVE first, then the fail, then the pass: the fail is the last decisive review, so the
// row is blocking but NOT suspectNoOp. The lift must still apply.
//
// regression: #5092
func TestSecBodyBoardApproveFirst(t *testing.T) {
	reviews := []review{sbdApprove("2026-07-14T18:00:00Z"), sbdSecFail(true, "2026-07-14T18:03:22Z"),
		sbdSecPass("2026-07-14T18:24:55Z")}
	st := beBoardReduce(reviews, beBoardBodyAfter)
	if !st.approved || st.blocking {
		t.Fatalf("APPROVE, marked fail, pass: approved=%v blocking=%v", st.approved, st.blocking)
	}
}

// regression: #5092
func TestSecBodyBoardStaysBlocked(t *testing.T) {
	cases := map[string][]review{
		"unmarked fail": {sbdSecFail(false, "2026-07-14T18:03:22Z"), sbdApprove("2026-07-14T18:05:00Z"),
			sbdSecPass("2026-07-14T18:24:55Z")},
		"no later pass":          {sbdSecFail(true, "2026-07-14T18:03:22Z"), sbdApprove("2026-07-14T18:05:00Z")},
		"no correctness APPROVE": {sbdSecFail(true, "2026-07-14T18:03:22Z"), sbdSecPass("2026-07-14T18:24:55Z")},
		"later unmarked fail": {sbdSecFail(true, "2026-07-14T18:03:22Z"), sbdApprove("2026-07-14T18:05:00Z"),
			sbdSecFail(false, "2026-07-14T18:20:00Z"), sbdSecPass("2026-07-14T18:24:55Z")},
		// A fully documented correctness body-edit APPROVE never clears a security fail.
		"correctness approve only": {sbdSecFail(true, "2026-07-14T18:03:22Z"), beBoardApprove()},
	}
	for name, reviews := range cases {
		t.Run(name, func(t *testing.T) {
			st := beBoardReduce(reviews, beBoardBodyAfter)
			if st.approved || !st.blocking || st.bodyEditReverified {
				t.Fatalf("approved=%v blocking=%v reverified=%v — want the row to stay blocking", st.approved,
					st.blocking, st.bodyEditReverified)
			}
		})
	}
}

// A security pass carrying the body-edit lines never clears a correctness CR.
//
// regression: #5092
func TestSecBodyBoardNoCrossLane(t *testing.T) {
	pass := beBoardReview("COMMENTED", beBoardApproveBody()+"\nSecurity-Review: pass", "2026-07-14T18:24:55Z")
	st := beBoardReduce([]review{beBoardCR(), pass}, beBoardBodyAfter)
	if st.approved || st.bodyEditReverified {
		t.Fatalf("a security pass cleared a correctness CR: approved=%v reverified=%v", st.approved, st.bodyEditReverified)
	}
}

// End to end through `deskboard actions`: the marked row no longer reads as blocked on the
// reviewer, and the unmarked twin still does.
//
// regression: #5092
func TestSecBodyBoardActions(t *testing.T) {
	runFor := func(t *testing.T, marked bool) actionRow {
		t.Helper()
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":17,"title":"example deck","body":`+jsonStr(beBoardBodyAfter)+`,"isDraft":true,`+
				`"lastEditedAt":`+jsonStr(beBoardEditedAt)+`,`+
				`"author":{"login":"ada"},"headRefOid":"`+beBoardHead+`","mergeStateStatus":"CLEAN",`+
				`"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"docs/example.md"}]`)
		var parts []string
		for _, r := range []review{sbdSecFail(marked, "2026-07-14T18:03:22Z"), sbdApprove("2026-07-14T18:05:00Z"),
			sbdSecPass("2026-07-14T18:24:55Z")} {
			parts = append(parts, `{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"`+r.State+`","commit_id":"`+
				beBoardHead+`","body":`+jsonStr(r.Body)+`,"submitted_at":"`+r.SubmittedAt+`"}`)
		}
		js := "["
		for i, p := range parts {
			if i > 0 {
				js += ","
			}
			js += p
		}
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", js+"]")
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
	marked := runFor(t, true)
	if marked.Action != actMergeNow {
		t.Fatalf("marked: action = %s (%s), want %s", marked.Action, marked.Note, actMergeNow)
	}
	unmarked := runFor(t, false)
	if unmarked.Action == actMergeNow {
		t.Fatalf("unmarked: action = %s (%s), want the row to stay blocked", unmarked.Action, unmarked.Note)
	}
	t.Logf("unmarked twin: %s (%s)", unmarked.Action, unmarked.Note)
}
