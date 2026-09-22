package deskkit

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// countCreateMRPosts returns how many POSTs to the merge-request CREATE route the fake
// GitLab server recorded — the attempt count CreateDraftChange's bounded retry turns on.
func countCreateMRPosts(s *glServer) int {
	n := 0
	for _, r := range s.requests {
		if r.Method == http.MethodPost && lMRRoot.MatchString(r.Path) {
			n++
		}
	}
	return n
}

// noSleepForge is s.forge() with the retry wait replaced by a no-op, so a bounded-retry
// test spends no real time. It does NOT change the number of attempts or the branching —
// only the wall-clock wait between them.
func noSleepForge(s *glServer) *GitLabForge {
	f := s.forge()
	f.sleep = func(_ time.Duration) {}
	return f
}

// TestGitLabCreateDraftChangeRetriesTransientMissingBranch reproduces the exact reported
// failure of issue #1415: a successful push is followed immediately by a create-MR call
// that GitLab rejects with a transient "source branch does not exist" 400 while its
// post-push branch visibility has not yet converged, for a source branch whose name
// contains a "/". The bounded same-identity retry must ride out the one transient rejection
// and open the MR.
//
// FAIL-FIRST: on the unfixed CreateDraftChange (no retry) the first 400 returns
// immediately — countCreateMRPosts == 1 and err != nil — so this test is RED before the fix
// and GREEN after. The "/"-in-branch requirement is exercised directly: Head is "feat/one".
func TestGitLabCreateDraftChangeRetriesTransientMissingBranch(t *testing.T) {
	s := newGLServer(t)
	s.createMRTransientFails = 1 // fail once (transient), then succeed
	s.createMR = glMR(map[string]any{"iid": 31, "title": "Draft: t", "source_branch": "feat/one"})

	ref, err := noSleepForge(s).CreateDraftChange(glRepo,
		DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
	if err != nil {
		t.Fatalf("expected the transient missing-branch 400 to be retried to success, got error: %v", err)
	}
	if ref == nil || ref.Number != 31 {
		t.Fatalf("expected the MR opened on the retry (iid 31), got %+v", ref)
	}
	if got := countCreateMRPosts(s); got != 2 {
		t.Fatalf("expected exactly 2 create attempts (1 transient 400 + 1 success), got %d", got)
	}
}

// TestGitLabCreateDraftChangeRetryGivesUp proves the retry is BOUNDED: when GitLab keeps
// answering the transient "source branch does not exist" 400, CreateDraftChange stops after
// gitlabCreateMRAttempts and surfaces could-not-check — never loops forever, never rounds
// the un-confirmed create up to a pass.
func TestGitLabCreateDraftChangeRetryGivesUp(t *testing.T) {
	s := newGLServer(t)
	s.createMRTransientFails = 99 // always transient — never converges

	ref, err := noSleepForge(s).CreateDraftChange(glRepo,
		DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
	if err == nil {
		t.Fatalf("expected could-not-check after the bounded retry gave up, got nil error and ref %+v", ref)
	}
	if ref != nil {
		t.Fatalf("a create that never confirmed must not return a PullRef, got %+v", ref)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("give-up must be could-not-check (ExitUnverifiable=%d), got exit %d for %v",
			ExitUnverifiable, ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "could-not-check") ||
		!strings.Contains(err.Error(), "did not converge") {
		t.Fatalf("give-up error should name the exhausted convergence window, got: %v", err)
	}
	if got := countCreateMRPosts(s); got != gitlabCreateMRAttempts {
		t.Fatalf("expected exactly gitlabCreateMRAttempts=%d create attempts, got %d",
			gitlabCreateMRAttempts, got)
	}
}

// TestGitLabCreateDraftChangeUnrelated400NotRetried proves the guard is NARROW: a genuine
// validation 400 (here, a duplicate MR already open on the source branch — a body that
// names "source branch" but not "does not exist") is surfaced on the FIRST response with
// GitLab's own message preserved, and is NEVER retried. Retrying it would mask exactly the
// diagnostic the #1415 body-preservation fix exists to surface.
func TestGitLabCreateDraftChangeUnrelated400NotRetried(t *testing.T) {
	s := newGLServer(t)
	s.createMRStatus = http.StatusBadRequest
	s.createMRErrBody = map[string]any{"message": []string{
		"Another open merge request already exists for this source branch: !5"}}

	ref, err := noSleepForge(s).CreateDraftChange(glRepo,
		DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
	if err == nil {
		t.Fatalf("expected the validation 400 to be surfaced as an error, got ref %+v", ref)
	}
	if got := countCreateMRPosts(s); got != 1 {
		t.Fatalf("an unrelated 400 must NOT be retried: expected exactly 1 attempt, got %d", got)
	}
	// Part 1: GitLab's own message must be preserved verbatim in the surfaced error.
	if !strings.Contains(err.Error(), "Another open merge request already exists for this source branch: !5") {
		t.Fatalf("GitLab's structured error body must be preserved verbatim, got: %v", err)
	}
	// And the *ForgeAPIError classification survives errors.As, carrying the body.
	var ae *ForgeAPIError
	if !errors.As(err, &ae) || ae.Status != http.StatusBadRequest || ae.Body == "" {
		t.Fatalf("expected a *ForgeAPIError{Status:400, Body:non-empty} to survive errors.As, got: %v", err)
	}
	// The narrowness guard itself: this body must not be classified as the transient case.
	if gitlabIsTransientMissingSourceBranch(err) {
		t.Fatalf("a duplicate-MR 400 must not be classified as the transient missing-branch condition")
	}
}

// TestGitLabIsTransientMissingSourceBranch pins the classifier's exact surface: only a 400
// naming BOTH "source branch" and "does not exist" is the transient condition; every other
// status, and a 400 that names only one phrase or carries no body, is not.
func TestGitLabIsTransientMissingSourceBranch(t *testing.T) {
	mk := func(status int, body string) error {
		return Unverifiable("could-not-check",
			&ForgeAPIError{Status: status, Method: http.MethodPost, Path: "/p", Body: body})
	}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"transient lower", mk(400, `{message: [source branch "feat/x" does not exist]}`), true},
		{"transient mixed case", mk(400, `Source Branch "feat/x" Does Not Exist`), true},
		{"duplicate MR (source branch, no does-not-exist)", mk(400, "Another open merge request already exists for this source branch: !5"), false},
		{"validation, no phrases", mk(400, "Title can't be blank"), false},
		{"400 no body", mk(400, ""), false},
		{"404 with matching text", mk(404, `source branch "feat/x" does not exist`), false},
		{"409 with matching text", mk(409, `source branch "feat/x" does not exist`), false},
		{"nil error", nil, false},
		{"non-forge error", errors.New("source branch does not exist"), false},
	}
	for _, tc := range cases {
		if got := gitlabIsTransientMissingSourceBranch(tc.err); got != tc.want {
			t.Errorf("%s: gitlabIsTransientMissingSourceBranch = %v, want %v", tc.name, got, tc.want)
		}
	}
}
