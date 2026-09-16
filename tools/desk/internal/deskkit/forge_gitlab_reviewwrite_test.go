package deskkit

// forge_gitlab_reviewwrite_test.go — the reviewer verdict WRITE control on GitLab, proven as a
// round-trip against a STATEFUL fake instance (forge-gitlab brief 09, Verify items 3 and 7).
//
// The golden corpus (forge_gitlab_test.go) pins each write's WIRE footprint in isolation. These
// tests prove the load-bearing security property the golden cannot: that the surface a verdict
// WRITE produces is exactly the surface the read path (ReviewsAtHead) consumes AT HEAD — an
// approve and a request-changes are BOTH visible to the read path — and that a tier/permission
// 403 on a write is could-not-check, never a clean (or a laundered) verdict.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// glStatefulServer is a fake GitLab instance that PERSISTS the effects of the review write —
// a POSTed note is appended and readable by the notes GET, an /approve sets the approval and an
// /unapprove clears it — so a PostReview followed by ReviewsAtHead observes what the write did.
// This is the difference from the golden harness, whose routes return canned payloads and record
// only the request footprint.
type glStatefulServer struct {
	srv *httptest.Server

	mu       sync.Mutex
	notes    []map[string]any
	approved bool
	// approvers, when set, is served as approved_by INSTEAD of the state-derived list — the
	// #1106 "someone else approved, not me" fixture.
	approvers []map[string]any
	// status maps an escaped-path suffix to a BODYLESS status to answer instead of the route
	// (the shape GitLab's `unauthorized!` produces: no `message` field to read).
	status map[string]int
}

// glVersionTime is the current diff-version arrival time. A note is head-pinned by the backend
// only when its created_at is at/after this, so posted notes are stamped strictly after it.
const (
	glVersionTime = "2026-08-30T10:00:00Z"
	glNoteTime    = "2026-08-30T11:00:00Z"
	glHead        = "abc123"
)

func newGLStatefulServer(t *testing.T) *glStatefulServer {
	t.Helper()
	s := &glStatefulServer{status: map[string]int{}}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handler))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *glStatefulServer) forge() *GitLabForge {
	return &GitLabForge{Token: glTestToken, BaseURL: s.srv.URL, Client: s.srv.Client()}
}

func (s *glStatefulServer) approvedBy() []map[string]any {
	if s.approvers != nil {
		return s.approvers
	}
	if !s.approved {
		return []map[string]any{}
	}
	return []map[string]any{{"user": map[string]any{"id": 42, "username": "reviewer-bot"}}}
}

func (s *glStatefulServer) handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.EscapedPath()
	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	s.mu.Lock()
	defer s.mu.Unlock()

	for suffix, code := range s.status {
		if strings.HasSuffix(path, suffix) {
			w.WriteHeader(code)
			return
		}
	}

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/api/v4/user"):
		// The token's own identity — what classifyApprove401 compares against approved_by.
		enc(map[string]any{"id": 42, "username": "reviewer-bot"})
	case r.Method == http.MethodPost && lMRNotes.MatchString(path):
		var in struct {
			Body string `json:"body"`
		}
		body, _ := readAllCompact(r)
		_ = json.Unmarshal([]byte(body), &in)
		id := 1000 + len(s.notes)
		s.notes = append(s.notes, map[string]any{
			"id": id, "body": in.Body, "system": false, "created_at": glNoteTime,
			"author": map[string]any{"id": 42, "username": "reviewer-bot"},
		})
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": id})
	case r.Method == http.MethodGet && lMRNotes.MatchString(path):
		enc(s.notes)
	case r.Method == http.MethodPost && lMRApprove.MatchString(path):
		if s.approved {
			// Measured GitLab behaviour (#1106): an approver who ALREADY approved "cannot
			// approve", and the route answers that with `unauthorized!` — a bodyless 401,
			// byte-identical to a rejected credential.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.approved = true
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"approved_by": s.approvedBy()})
	case r.Method == http.MethodPost && lMRUnapprove.MatchString(path):
		s.approved = false
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodGet && lMRApprovals.MatchString(path):
		enc(map[string]any{"approved_by": s.approvedBy()})
	case r.Method == http.MethodGet && lMRVersions.MatchString(path):
		enc([]map[string]any{{"id": 3, "head_commit_sha": glHead, "created_at": glVersionTime}})
	case r.Method == http.MethodGet && lProjApproval.MatchString(path):
		// reset_approvals_on_push ON → an approval that exists was given at this head.
		enc(map[string]any{"reset_approvals_on_push": true})
	case r.Method == http.MethodGet && lMR.MatchString(path):
		enc(glMR(nil)) // sha=abc123, iid=7
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// findReview returns the first review satisfying pred, or nil.
func findReview(reviews []Review, pred func(Review) bool) *Review {
	for i := range reviews {
		if pred(reviews[i]) {
			return &reviews[i]
		}
	}
	return nil
}

// TestForgeGitlabRequestChanges backs Verify item 3. An APPROVE verdict and then a
// REQUEST_CHANGES verdict are each posted through PostReview and read back by ReviewsAtHead:
//   - APPROVE lands as a head-SHA verdict note PLUS an /approve, so ReviewsAtHead sees BOTH a
//     standing APPROVED (from the approval) and the verdict note at head.
//   - REQUEST_CHANGES lands as unapprove PLUS a head-SHA verdict note, so ReviewsAtHead sees the
//     request-changes verdict note at head and NO standing APPROVED — the approval is revoked.
//
// Both verdicts are therefore visible to the read path at head (the approve↔request-changes
// symmetry), and the request-changes retraction is READABLE, not silently dropped.
func TestForgeGitlabRequestChanges(t *testing.T) {
	s := newGLStatefulServer(t)
	f := s.forge()

	// --- APPROVE round-trip ---
	const approveBody = "Verdict: correctness APPROVE — looks good"
	if err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "APPROVE", Body: approveBody}); err != nil {
		t.Fatalf("PostReview APPROVE: %v", err)
	}
	reviews, err := f.ReviewsAtHead(glRepo, 7)
	if err != nil {
		t.Fatalf("ReviewsAtHead after APPROVE: %v", err)
	}
	if got := findReview(reviews, func(r Review) bool { return r.State == "APPROVED" && r.CommitID == glHead }); got == nil {
		t.Fatalf("after APPROVE, ReviewsAtHead shows no standing APPROVED at head %s; reviews: %+v", glHead, reviews)
	}
	if got := findReview(reviews, func(r Review) bool {
		return strings.Contains(r.Body, "APPROVE") && r.CommitID == glHead
	}); got == nil {
		t.Fatalf("after APPROVE, the verdict NOTE is not readable at head %s; reviews: %+v", glHead, reviews)
	}

	// --- REQUEST_CHANGES round-trip ---
	const rejectBody = "Verdict: correctness REQUEST_CHANGES — needs work"
	if err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "REQUEST_CHANGES", Body: rejectBody}); err != nil {
		t.Fatalf("PostReview REQUEST_CHANGES: %v", err)
	}
	reviews, err = f.ReviewsAtHead(glRepo, 7)
	if err != nil {
		t.Fatalf("ReviewsAtHead after REQUEST_CHANGES: %v", err)
	}
	// The approval was revoked (unapprove): no standing APPROVED remains.
	if got := findReview(reviews, func(r Review) bool { return r.State == "APPROVED" }); got != nil {
		t.Fatalf("REQUEST_CHANGES must revoke the approval, but a standing APPROVED remains: %+v", *got)
	}
	// The request-changes verdict note is readable at head — the retraction is not dropped.
	if got := findReview(reviews, func(r Review) bool {
		return strings.Contains(r.Body, "REQUEST_CHANGES") && r.CommitID == glHead
	}); got == nil {
		t.Fatalf("after REQUEST_CHANGES, the verdict NOTE is not readable at head %s; reviews: %+v", glHead, reviews)
	}
}

// TestForgeGitlabWriteTierErrors backs Verify item 7. A 403 on the approval endpoint of a
// verdict WRITE surfaces could-not-check — never a clean write. A Premium-gated (or
// permission-gated) failure of the grant half is reported as an error the caller reads as
// could-not-check (exit 6), so a verdict that did NOT land is never mistaken for one that did.
func TestForgeGitlabWriteTierErrors(t *testing.T) {
	cases := []struct {
		name       string
		event      string
		forceRoute string // path suffix to 403
	}{
		// forceStatus keys on a path suffix; "/approve" matches only the approve route
		// (".../unapprove" ends in "napprove"), and "/unapprove" matches only unapprove.
		{"approve_grant_403", "APPROVE", "/approve"},
		{"request_changes_unapprove_403", "REQUEST_CHANGES", "/unapprove"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newGLServer(t)
			s.mr = glMR(nil)
			// The verdict note posts first and succeeds (201); the grant/revoke half 403s.
			s.forceStatus[tc.forceRoute] = http.StatusForbidden
			f := s.forge()

			err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: tc.event, Body: "reasoning"})
			if err == nil {
				t.Fatalf("a 403 on the %s write must surface could-not-check, not a clean write", tc.forceRoute)
			}
			if !strings.Contains(err.Error(), "could-not-check") {
				t.Fatalf("write-tier refusal must say could-not-check, got %q", err.Error())
			}
			var fae *ForgeAPIError
			if !errors.As(err, &fae) {
				t.Fatalf("write-tier 403 should carry a *ForgeAPIError, got %T (%v)", err, err)
			}
			if fae.Status != http.StatusForbidden {
				t.Fatalf("ForgeAPIError.Status = %d, want 403", fae.Status)
			}
			if code := ExitCodeOf(err); code != ExitUnverifiable {
				t.Fatalf("write-tier 403 should map to ExitUnverifiable (%d), got %d", ExitUnverifiable, code)
			}
			t.Logf("%s write 403 → could-not-check: %v", tc.event, err)
		})
	}
}

// TestForgeGitlabVerdictNoteState is the #798 regression: a GitLab correctness verdict must
// be visible to the read path with the review STATE the reviewer-approved gate reads, not
// merely as a COMMENTED note whose body happens to contain the word. The write side already
// lands the verdict as a `Verdict: approve|request-changes` NOTE (approve also POSTs an
// approval; request-changes has no native GitLab object), so unless ReviewsAtHead reduces
// that note to APPROVED / CHANGES_REQUESTED at head, deskflip reports "no APPROVED/
// CHANGES_REQUESTED correctness verdict" over a verdict that was really rendered — the write
// and the read disagree on the object.
//
// The decisive case is request-changes: after it there is NO approval object at all, so a
// CHANGES_REQUESTED at head can ONLY come from the note. A note-only write that regressed to
// COMMENTED would leave the retraction invisible again.
func TestForgeGitlabVerdictNoteState(t *testing.T) {
	// --- APPROVE: the verdict NOTE itself carries State APPROVED at head ---
	t.Run("approve_note_is_APPROVED_at_head", func(t *testing.T) {
		s := newGLStatefulServer(t)
		f := s.forge()
		const approveBody = "## Review\n\nVerdict: approve\n"
		if err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "APPROVE", Body: approveBody}); err != nil {
			t.Fatalf("PostReview APPROVE: %v", err)
		}
		reviews, err := f.ReviewsAtHead(glRepo, 7)
		if err != nil {
			t.Fatalf("ReviewsAtHead: %v", err)
		}
		// The NOTE (identified by its body) must itself read as APPROVED at head — not merely
		// the separate approval object. On GitLab CE the approval object is not head-pinned,
		// so the note is the channel the gate can always see.
		if got := findReview(reviews, func(r Review) bool {
			return strings.Contains(r.Body, "Verdict: approve") && r.State == "APPROVED" && r.CommitID == glHead
		}); got == nil {
			t.Fatalf("the approve verdict NOTE is not readable as APPROVED at head %s; reviews: %+v", glHead, reviews)
		}
	})

	// --- REQUEST_CHANGES: with no approval object, CHANGES_REQUESTED must come from the note ---
	t.Run("request_changes_note_is_CHANGES_REQUESTED_at_head", func(t *testing.T) {
		s := newGLStatefulServer(t)
		f := s.forge()
		const rejectBody = "## Review\n\nVerdict: request-changes\n"
		if err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "REQUEST_CHANGES", Body: rejectBody}); err != nil {
			t.Fatalf("PostReview REQUEST_CHANGES: %v", err)
		}
		reviews, err := f.ReviewsAtHead(glRepo, 7)
		if err != nil {
			t.Fatalf("ReviewsAtHead: %v", err)
		}
		// No standing APPROVED (unapprove revoked it / there was none).
		if got := findReview(reviews, func(r Review) bool { return r.State == "APPROVED" }); got != nil {
			t.Fatalf("request-changes must leave no standing APPROVED, got: %+v", *got)
		}
		// The verdict is VISIBLE as CHANGES_REQUESTED at head — the retraction is not a mere
		// COMMENTED note the gate cannot distinguish from a plain comment.
		if got := findReview(reviews, func(r Review) bool {
			return r.State == "CHANGES_REQUESTED" && r.CommitID == glHead
		}); got == nil {
			t.Fatalf("the request-changes verdict is not readable as CHANGES_REQUESTED at head %s; reviews: %+v", glHead, reviews)
		}
	})
}

// TestForgeGitlabApproveAlreadyApproved is the #1106 regression. GitLab's approve route
// answers a BODYLESS 401 when the acting user has already approved the merge request — the
// same bytes a rejected credential produces — and the backend reported it as "credential
// rejected ... re-mint". The approval was already in force. A second APPROVE at the same
// head must therefore SUCCEED, with a note (via ReviewInput.Report) naming the endpoint that
// answered 401 and the endpoint that proved the approval stands — never a refusal, and never
// a silent nil either.
func TestForgeGitlabApproveAlreadyApproved(t *testing.T) {
	s := newGLStatefulServer(t)
	f := s.forge()
	const body = "Verdict: correctness APPROVE — looks good"

	if err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "APPROVE", Body: body}); err != nil {
		t.Fatalf("first PostReview APPROVE: %v", err)
	}
	var note string
	err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "APPROVE", Body: body,
		Report: func(n string) { note = n }})
	if err != nil {
		t.Fatalf("second APPROVE by the same identity must be success-with-note, got a refusal: %v", err)
	}
	for _, want := range []string{"already approved by reviewer-bot (id 42)", "POST /projects/", "/merge_requests/7/approve",
		"HTTP 401", "GET /projects/", "/merge_requests/7/approvals", "credential is valid"} {
		if !strings.Contains(note, want) {
			t.Errorf("success note must carry %q; got %q", want, note)
		}
	}
	if strings.Contains(note, "could-not-check") || strings.Contains(note, "rejected") {
		t.Errorf("success note must not read like a refusal: %q", note)
	}
	// The approval still stands on the read path.
	reviews, rerr := f.ReviewsAtHead(glRepo, 7)
	if rerr != nil {
		t.Fatalf("ReviewsAtHead: %v", rerr)
	}
	if got := findReview(reviews, func(r Review) bool { return r.State == "APPROVED" && r.CommitID == glHead }); got == nil {
		t.Fatalf("the standing APPROVED at head %s vanished; reviews: %+v", glHead, reviews)
	}
	t.Logf("already-approved 401 → success-with-note: %s", note)
}

// TestForgeGitlabApprove401Classification pins the other three outcomes of a 401 on the
// approve route (#1106): a credential that is genuinely rejected — proven by a 401 on the
// identity read or on the approvals read — stays the fail-closed refusal it always was, with
// the confirming endpoint named; a valid credential whose user is simply not an eligible
// approver is a refusal that names ELIGIBILITY and the acting identity, not the credential;
// and a classification read that fails some other way leaves the 401 a could-not-check that
// says it could not be classified. None of the three ever reads as success.
func TestForgeGitlabApprove401Classification(t *testing.T) {
	const approvals = "/merge_requests/7/approvals"
	cases := []struct {
		name    string
		setup   func(s *glStatefulServer)
		want    []string // substrings the refusal must carry
		forbid  []string // substrings it must not carry
		wantSts int      // the *ForgeAPIError status the refusal must still unwrap to
	}{
		{
			name:    "credential_confirmed_by_user_read",
			setup:   func(s *glStatefulServer) { s.status["/api/v4/user"] = http.StatusUnauthorized },
			want:    []string{"could-not-check", "credential rejected (HTTP 401)", "/merge_requests/7/approve", "confirmed: GET /user also returned HTTP 401"},
			forbid:  []string{"eligible approver"},
			wantSts: http.StatusUnauthorized,
		},
		{
			name:    "credential_confirmed_by_approvals_read",
			setup:   func(s *glStatefulServer) { s.status[approvals] = http.StatusUnauthorized },
			want:    []string{"could-not-check", "credential rejected (HTTP 401)", "/merge_requests/7/approve", "confirmed: GET /projects/", approvals + " also returned HTTP 401"},
			forbid:  []string{"eligible approver"},
			wantSts: http.StatusUnauthorized,
		},
		{
			name: "valid_credential_not_an_eligible_approver",
			setup: func(s *glStatefulServer) {
				s.approvers = []map[string]any{{"user": map[string]any{"id": 99, "username": "someone-else"}}}
			},
			want:    []string{"could-not-check", "not an eligible approver (HTTP 401)", "/merge_requests/7/approve", "GET /user answered 200 as reviewer-bot (id 42)", "rotating the token will not change this"},
			forbid:  []string{"credential rejected"},
			wantSts: http.StatusUnauthorized,
		},
		{
			name:    "classification_read_failed_otherwise",
			setup:   func(s *glStatefulServer) { s.status["/api/v4/user"] = http.StatusForbidden },
			want:    []string{"could-not-check", "/merge_requests/7/approve", "returned HTTP 401", "could not be classified", "GET /user", "HTTP 403"},
			forbid:  []string{"credential rejected", "eligible approver"},
			wantSts: http.StatusUnauthorized,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newGLStatefulServer(t)
			s.approved = true // every case starts from "the route answers 401" (see the approve handler)
			tc.setup(s)
			f := s.forge()
			reported := false
			err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "APPROVE", Body: "reasoning",
				Report: func(string) { reported = true }})
			if err == nil {
				t.Fatalf("a 401 that is not a standing approval by this identity must refuse, got nil")
			}
			if reported {
				t.Fatalf("Report must never fire on an error path")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("refusal must carry %q; got %q", w, err.Error())
				}
			}
			for _, fb := range tc.forbid {
				if strings.Contains(err.Error(), fb) {
					t.Errorf("refusal must not carry %q; got %q", fb, err.Error())
				}
			}
			var fae *ForgeAPIError
			if !errors.As(err, &fae) || fae.Status != tc.wantSts || fae.Method != http.MethodPost {
				t.Fatalf("refusal must unwrap to the approve route's *ForgeAPIError (POST, %d); got %T %v", tc.wantSts, err, err)
			}
			if code := ExitCodeOf(err); code != ExitUnverifiable {
				t.Fatalf("exit = %d, want ExitUnverifiable (%d)", code, ExitUnverifiable)
			}
			t.Logf("%s → %v", tc.name, err)
		})
	}
}
