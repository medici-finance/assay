package main

// bodyedit_test.go — the board's half of the documented body-edit re-verification class.
//
// The board must agree with deskflip's gate about what the class is: an admitted row reads
// APPROVED at head (and so takes the ordinary approved arms), and every near-miss keeps the #37
// SUSPECT-APPROVAL suppression exactly as before. The decision is deskkit's shared one; these
// tests pin that the board feeds it the same inputs the gate does.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	beBoardHead       = "deadbeefcafe0000111122223333444455556666"
	beBoardBodyBefore = "Summary: the example check FALSE-PASSES."
	beBoardBodyAfter  = "Summary: the example check passes on the corrected fixture."
	beBoardFinding    = "f-body-1"
)

func beBoardReview(state, body, at string) review {
	var r review
	r.User.Login = reviewerBotDisplay()
	r.State, r.CommitID, r.Body, r.SubmittedAt = state, beBoardHead, body, at
	return r
}

func beBoardCR() review {
	return beBoardReview("CHANGES_REQUESTED", "The PR body still asserts the retracted claim.\n\nBlocked-On-Body: "+
		beBoardFinding+" "+deskkit.PRBodyDigest(beBoardBodyBefore), "2026-07-14T18:03:22Z")
}

func beBoardApproveBody() string {
	return "Re-read the live PR body via the API.\n\nResolved-Body-Finding: " + beBoardFinding +
		"\nBody-Reread-Digest: " + deskkit.PRBodyDigest(beBoardBodyAfter) + "\nCI-Green-At: " + beBoardHead
}

func beBoardApprove() review {
	return beBoardReview("APPROVED", beBoardApproveBody(), "2026-07-14T18:24:55Z")
}

func beBoardReduce(reviews []review, liveBody string) reviewState {
	return applyBodyEditReverification(reduceReviews(reviews, beBoardHead), reviews, beBoardHead, liveBody)
}

func TestBodyEdit_BoardAdmitsDocumentedClass(t *testing.T) {
	st := beBoardReduce([]review{beBoardCR(), beBoardApprove()}, beBoardBodyAfter)
	if !st.approved || st.blocking || st.suspectNoOp || !st.bodyEditReverified {
		t.Fatalf("documented class: approved=%v blocking=%v suspect=%v reverified=%v — want approved, not suspect",
			st.approved, st.blocking, st.suspectNoOp, st.bodyEditReverified)
	}
	if st.approvedAt.IsZero() {
		t.Error("the governing APPROVE's time must drive approvedAt")
	}
}

func TestBodyEdit_BoardNearMissesStaySuspect(t *testing.T) {
	cases := map[string]struct {
		reviews []review
		live    string
	}{
		"undeclared CR (different class)": {
			[]review{beBoardReview("CHANGES_REQUESTED", "The PR body is wrong.", "2026-07-14T18:03:22Z"), beBoardApprove()},
			beBoardBodyAfter,
		},
		"check-only CR (different class)": {
			[]review{beBoardReview("CHANGES_REQUESTED", "Blocked-On-Check: changelog", "2026-07-14T18:03:22Z"), beBoardApprove()},
			beBoardBodyAfter,
		},
		"undocumented re-approve": {
			[]review{beBoardCR(), beBoardReview("APPROVED", "body fixed, looks good now", "2026-07-14T18:24:55Z")},
			beBoardBodyAfter,
		},
		"code-finding CR also standing at head": {
			[]review{
				beBoardReview("CHANGES_REQUESTED", "The retry loop never exits.", "2026-07-14T17:00:00Z"),
				beBoardCR(), beBoardApprove(),
			},
			beBoardBodyAfter,
		},
		"live body moved since the re-read": {
			[]review{beBoardCR(), beBoardApprove()},
			beBoardBodyAfter + "\nA new, unreviewed claim.",
		},
		"security-lane approve": {
			[]review{beBoardCR(), beBoardReview("APPROVED", beBoardApproveBody()+"\nSecurity-Review: pass", "2026-07-14T18:24:55Z")},
			beBoardBodyAfter,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			st := beBoardReduce(c.reviews, c.live)
			if st.approved || !st.blocking || !st.suspectNoOp || st.bodyEditReverified {
				t.Fatalf("near-miss lifted the suppression: approved=%v blocking=%v suspect=%v reverified=%v",
					st.approved, st.blocking, st.suspectNoOp, st.bodyEditReverified)
			}
		})
	}
}

// A later CR at the same head re-blocks: the lift never survives a fresh rejection.
func TestBodyEdit_BoardLaterCRReblocks(t *testing.T) {
	later := beBoardReview("CHANGES_REQUESTED", "New finding.", "2026-07-14T19:00:00Z")
	st := beBoardReduce([]review{beBoardCR(), beBoardApprove(), later}, beBoardBodyAfter)
	if st.approved || !st.blocking {
		t.Fatalf("a CR after the documented approve must block: approved=%v blocking=%v", st.approved, st.blocking)
	}
}

// End to end through `deskboard actions`: the admitted draft row takes the ordinary
// approved-at-head arm (approved + CI green + CLEAN → MERGE-NOW, "desk flips, then merge now")
// with the class named in its note; the undocumented twin still reads SUSPECT-APPROVAL.
func TestBodyEdit_BoardActionsEndToEnd(t *testing.T) {
	repo := "example-org/tracker"
	runFor := func(t *testing.T, approveBody string) actionRow {
		t.Helper()
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":17,"title":"example deck","body":`+jsonStr(beBoardBodyAfter)+`,"isDraft":true,`+
				`"author":{"login":"ada"},"headRefOid":"`+beBoardHead+`","mergeStateStatus":"CLEAN",`+
				`"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"docs/example.md"}]`)
		cr := beBoardCR()
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", `[`+
			`{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"CHANGES_REQUESTED","commit_id":"`+beBoardHead+`",`+
			`"body":`+jsonStr(cr.Body)+`,"submitted_at":"`+cr.SubmittedAt+`"},`+
			`{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+beBoardHead+`",`+
			`"body":`+jsonStr(approveBody)+`,"submitted_at":"2026-07-14T18:24:55Z"}]`)
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

	t.Run("documented — approved-at-head arm", func(t *testing.T) {
		row := runFor(t, beBoardApproveBody())
		if row.Action != actMergeNow {
			t.Fatalf("action = %s (%s), want %s", row.Action, row.Note, actMergeNow)
		}
		if !strings.Contains(row.Note, "body-edit re-verification") {
			t.Errorf("note does not name the class: %s", row.Note)
		}
	})
	t.Run("undocumented — SUSPECT-APPROVAL", func(t *testing.T) {
		row := runFor(t, "body fixed, looks good now")
		if row.Action != actSuspectApproval {
			t.Fatalf("action = %s, want %s", row.Action, actSuspectApproval)
		}
	})
}
