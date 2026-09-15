package main

// gitlabrequestchanges_test.go — the board half of "a GitLab request-changes verdict is a
// standing rejection the board can read" (issue #1124).
//
// The reviewer's write on GitLab already lands the right OBJECTS: a merge-request NOTE
// carrying the verdict line, plus an /unapprove. What the board does with them is the part
// under test here, and it is tested END TO END over a fake GitLab instance — the real
// deskkit.GitLabForge.ReviewsAtHead read, the real fetchReviews conversion, the real
// reduceReviews reduction and the real classify — because every reported failure of this
// contract has been a DISAGREEMENT between two of those layers, and a test that stubs the
// forge cannot see one.
//
// Two properties are pinned, each a separate way the standing rejection went missing:
//
//  1. A REQUEST_CHANGES whose body carries the SECURITY marker is still a request-changes.
//     `deskpost security-review --verdict fail` submits the REQUEST_CHANGES event and its
//     body may carry ONLY `Security-Review: fail` (deskpost refuses a correctness
//     `Verdict:` line in that lane). On GitHub that lands a CHANGES_REQUESTED review. On
//     GitLab it is a note, and a read that recognised only the CORRECTNESS verdict line
//     reduced it to COMMENTED — so the board reported "no bot APPROVED/CHANGES_REQUESTED
//     at head" and NEEDS-REVIEW over a live blocking verdict, which is the UNREVIEWED
//     alarm firing on a merge request the reviewer had already rejected.
//
//  2. The reviews come back in ASCENDING submitted order. GitLab's notes endpoint defaults
//     to `sort=desc` (newest first) where GitHub's reviews endpoint is chronological, and
//     every consumer of this read — reduceReviews here, deskpost's latestAppVerdict,
//     deskflip — documents "reviews arrive in ascending submitted order, so the last one
//     governs". Fed the reversed stream, the OLDEST verdict governs instead: an approval
//     at a newer head does not clear an older request-changes, and an ordinary
//     approve-then-reject at one head reads as the #37 forged no-op approval.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const boardRosterGitLabRC = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=reviewer=gitlab:gl-reviewer:41987965
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
`

// The current head, the moment it landed, and the note times either side of it. A note is
// head-pinned by the backend when it was created at or after the current diff version
// arrived — so a note at glRCNoteT1 (before) belongs to a SUPERSEDED head and carries no
// commit id, while glRCNoteT3 / glRCNoteT4 (after) are pinned to glRCHeadCur.
const (
	glRCHeadCur  = "2222222222222222222222222222222222222222"
	glRCNoteT1   = "2026-09-10T10:00:00Z"
	glRCArrived  = "2026-09-10T11:00:00Z"
	glRCNoteT3   = "2026-09-10T12:00:00Z"
	glRCNoteT4   = "2026-09-10T13:00:00Z"
	glRCReviewer = "gl-reviewer"
)

var (
	rcProjApprovals = regexp.MustCompile(`^/api/v4/projects/[^/]+/approvals$`)
	rcMRVersions    = regexp.MustCompile(`/merge_requests/[0-9]+/versions$`)
	rcMRApprovals   = regexp.MustCompile(`/merge_requests/[0-9]+/approvals$`)
	rcMRNotes       = regexp.MustCompile(`/merge_requests/[0-9]+/notes$`)
	rcMR            = regexp.MustCompile(`/merge_requests/[0-9]+$`)
)

// glNote is one merge-request note as the wire carries it.
type glNote struct {
	id        int
	body      string
	createdAt string
	system    bool
}

// serveGitLabReviews stands up a fake GitLab instance serving exactly the four reads
// ReviewsAtHead issues, and points the board's forgeFor at it. notes are given NEWEST
// FIRST, which is the order the real notes endpoint returns them in (`sort=desc` is its
// documented default) — handing the test an already-sorted stream would test nothing.
func serveGitLabReviews(t *testing.T, head string, notes []glNote, approved bool) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		path := r.URL.EscapedPath()
		switch {
		case rcProjApprovals.MatchString(path):
			// reset_approvals_on_push ON → an approval that exists is at the current head.
			enc(map[string]any{"reset_approvals_on_push": true})
		case rcMRVersions.MatchString(path):
			enc([]map[string]any{{"id": 9, "head_commit_sha": head, "created_at": glRCArrived}})
		case rcMRApprovals.MatchString(path):
			by := []map[string]any{}
			if approved {
				by = append(by, map[string]any{"user": map[string]any{"id": 42, "username": glRCReviewer}})
			}
			enc(map[string]any{"approved_by": by})
		case rcMRNotes.MatchString(path):
			out := make([]map[string]any, 0, len(notes))
			for _, n := range notes {
				out = append(out, map[string]any{
					"id": n.id, "body": n.body, "system": n.system, "created_at": n.createdAt,
					"author": map[string]any{"id": 42, "username": glRCReviewer},
				})
			}
			enc(out)
		case rcMR.MatchString(path):
			enc(map[string]any{"iid": 7, "sha": head, "web_url": "https://gitlab.example/x/y/-/merge_requests/7"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	prev := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return &deskkit.GitLabForge{
				Token:   "test-injected-token-0000",
				BaseURL: srv.URL,
				Client:  srv.Client(),
			},
			deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
	}
	t.Cleanup(func() { forgeFor = prev })
}

// boardVerdict runs the real board pipeline — forge read, conversion, reduction,
// classification — and returns the reduction and the action, so a failure names the layer.
func boardVerdict(t *testing.T, head string) (reviewState, string, string) {
	t.Helper()
	reviews, err := fetchReviews("example-org/tracker", 7)
	if err != nil {
		t.Fatalf("fetchReviews over the fake GitLab instance: %v", err)
	}
	st := reduceReviews(reviews, head)
	var p prBase
	p.Number = 7
	p.HeadRefOid = head
	// ciRequired=false makes CI vacuously green, so the assertion is about the REVIEW axis
	// and nothing else.
	action, note := classify(buildClassifyInput(p, st, false, ""))
	return st, action, note
}

// TestBoardReadsGitLabSecurityFailAsChangesRequested_1124 is property 1. A
// `Security-Review: fail` note is the ONLY body shape `deskpost security-review --verdict
// fail` can post, and that verb submits REQUEST_CHANGES — so on GitLab the note must read
// as CHANGES_REQUESTED, or the board reports a rejected merge request as never reviewed.
func TestBoardReadsGitLabSecurityFailAsChangesRequested_1124(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabRC)
	serveGitLabReviews(t, glRCHeadCur, []glNote{
		{id: 3, body: "## Findings\n\nSecurity-Review: fail\n", createdAt: glRCNoteT3},
	}, false)

	st, action, note := boardVerdict(t, glRCHeadCur)
	if !st.ever {
		t.Fatalf("reduceReviews: ever = false over a GitLab REQUEST_CHANGES verdict note at head — "+
			"the board reports %q, so the review desk re-dispatches a reviewer onto a merge request "+
			"its own reviewer has already rejected (#1124)", note)
	}
	if !st.atHead || !st.blocking {
		t.Fatalf("reduceReviews: atHead = %v, blocking = %v, want true/true — the security FAIL is the "+
			"standing rejection at %s", st.atHead, st.blocking, glRCHeadCur)
	}
	if action != actBlocked {
		t.Fatalf("classify = %s (%s), want %s", action, note, actBlocked)
	}
}

// TestBoardGitLabApprovalAtNewerHeadClearsRequestChanges_1124 is property 2, in the
// direction that matters to a worker: the request-changes stands until the worker pushes
// and the reviewer approves the NEW head, and then it is cleared. Read in the wire's
// newest-first order the older rejection is processed LAST and governs, so the approval
// never clears anything and the row sits at RE-REVIEW forever.
func TestBoardGitLabApprovalAtNewerHeadClearsRequestChanges_1124(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabRC)
	// Wire order is newest-first, exactly as GitLab returns it.
	serveGitLabReviews(t, glRCHeadCur, []glNote{
		{id: 4, body: "## Review\n\nVerdict: approve\n", createdAt: glRCNoteT3},
		{id: 2, body: "## Review\n\nVerdict: request-changes\n", createdAt: glRCNoteT1},
	}, true)

	st, action, note := boardVerdict(t, glRCHeadCur)
	if !st.atHead || !st.approved {
		t.Fatalf("reduceReviews: atHead = %v, approved = %v, want true/true — the approve at the CURRENT "+
			"head is the newest verdict and governs; got action %s (%s)", st.atHead, st.approved, action, note)
	}
	if st.blocking {
		t.Fatalf("reduceReviews: blocking = true — the superseded request-changes at the OLD head is " +
			"still governing, so no push can ever clear it (#1124)")
	}
	if action == actNeedsReview || action == actReReview {
		t.Fatalf("classify = %s (%s) — an approval at the current head must clear the earlier "+
			"request-changes", action, note)
	}
}

// TestBoardGitLabNewestVerdictGovernsAtOneHead_1124 is property 2 in the other direction,
// and it is the discriminating case: an ordinary approve-then-reject at ONE head. Read
// newest-first the pair arrives as reject-then-approve, which the #37 reduction reads as an
// APPROVED posted over a standing rejection with no push between — a SUSPECTED FORGED
// approval. The state is right by luck (still blocking) and the diagnosis is wrong, which
// routes a desk operator to a security incident that did not happen.
func TestBoardGitLabNewestVerdictGovernsAtOneHead_1124(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabRC)
	serveGitLabReviews(t, glRCHeadCur, []glNote{
		{id: 6, body: "## Review\n\nVerdict: request-changes\n", createdAt: glRCNoteT4},
		{id: 5, body: "## Review\n\nVerdict: approve\n", createdAt: glRCNoteT3},
	}, false)

	st, action, note := boardVerdict(t, glRCHeadCur)
	if !st.blocking {
		t.Fatalf("reduceReviews: blocking = false — the NEWEST verdict at head is the rejection; "+
			"got action %s (%s)", action, note)
	}
	if st.suspectNoOp {
		t.Fatalf("reduceReviews: suspectNoOp = true — an approve FOLLOWED BY a rejection is an ordinary " +
			"block; only the reverse order is the #37 no-op approval, and reading the stream backwards " +
			"is what manufactures the alarm")
	}
	if action != actBlocked {
		t.Fatalf("classify = %s (%s), want %s", action, note, actBlocked)
	}
}

// TestBoardGitLabReviewsAreAscending_1124 pins the ordering contract directly, at the seam
// where it is established. The three tests above each depend on it; this one names it, so a
// regression reports "the read came back newest-first" instead of three classification
// failures a reader has to work backwards from.
func TestBoardGitLabReviewsAreAscending_1124(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabRC)
	serveGitLabReviews(t, glRCHeadCur, []glNote{
		{id: 6, body: "third", createdAt: glRCNoteT4},
		{id: 5, body: "second", createdAt: glRCNoteT3},
		{id: 2, body: "first", createdAt: glRCNoteT1},
	}, false)

	reviews, err := fetchReviews("example-org/tracker", 7)
	if err != nil {
		t.Fatalf("fetchReviews: %v", err)
	}
	var got []string
	for _, r := range reviews {
		got = append(got, r.SubmittedAt)
	}
	want := []string{glRCNoteT1, glRCNoteT3, glRCNoteT4}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("submitted order = %v, want %v — every consumer of this read (reduceReviews here, "+
			"deskpost's latestAppVerdict, deskflip) reduces on 'the last verdict governs'", got, want)
	}
}
