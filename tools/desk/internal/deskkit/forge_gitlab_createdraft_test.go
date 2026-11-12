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
		// issue #6281: GitLab's field-keyed validation body, as the client library renders it.
		{"keyed source_branch", mk(400, `{message: {source_branch: [does not exist]}}`), true},
		{"keyed source_branch among others", mk(400, `{message: {source_branch: [is invalid, does not exist]}, {title: [is too long]}}`), true},
		{"keyed target_branch only", mk(400, `{message: {target_branch: [does not exist]}}`), false},
		{"keyed source invalid, target missing", mk(400, `{message: {source_branch: [is invalid]}, {target_branch: [does not exist]}}`), false},
		{"keyed source_branch 404", mk(404, `{message: {source_branch: [does not exist]}}`), false},
		{"nil error", nil, false},
		{"non-forge error", errors.New("source branch does not exist"), false},
	}
	for _, tc := range cases {
		if got := gitlabIsTransientMissingSourceBranch(tc.err); got != tc.want {
			t.Errorf("%s: gitlabIsTransientMissingSourceBranch = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// glKeyedMissingBody is the client library's rendering of GitLab's field-keyed validation
// body for the post-push race, exactly as the reporter of issue #6281 saw it.
const glKeyedMissingBody = "{message: {source_branch: [does not exist]}}"

// sleepLog is a no-op sleep that records every requested wait, so a test can pin the
// backoff schedule without spending real time.
func sleepLog(f *GitLabForge) *[]time.Duration {
	var waits []time.Duration
	f.sleep = func(d time.Duration) { waits = append(waits, d) }
	return &waits
}

// countMRListGets returns how many merge-request LIST reads (the duplicate guard) the fake
// recorded.
func countMRListGets(s *glServer) int {
	n := 0
	for _, r := range s.requests {
		if r.Method == http.MethodGet && lMRRoot.MatchString(r.Path) {
			n++
		}
	}
	return n
}

// TestGLCreateMRKeyedRetried reproduces issue #6281: GitLab rejects the create twice with
// the FIELD-KEYED "source_branch: does not exist" 400, then accepts it. The bounded retry
// must ride it out with a growing backoff, reading for an open merge request before each
// retry.
//
// FAIL-FIRST: before the fix the classifier knew only the sentence form, so this shape was
// surfaced on the first response — 1 create attempt and an error — RED.
//
// regression: #6281
func TestGLCreateMRKeyedRetried(t *testing.T) {
	s := newGLServer(t)
	s.createMRTransientFails = 2
	s.createMRTransientKeyed = true
	s.createMR = glMR(map[string]any{"iid": 31, "title": "Draft: t", "source_branch": "feat/one"})
	f := s.forge()
	waits := sleepLog(f)

	ref, err := f.CreateDraftChange(glRepo,
		DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
	if err != nil {
		t.Fatalf("expected the field-keyed missing-branch 400 to be retried to success, got: %v", err)
	}
	if ref == nil || ref.Number != 31 {
		t.Fatalf("expected the MR opened on the third attempt (iid 31), got %+v", ref)
	}
	if got := countCreateMRPosts(s); got != 3 {
		t.Fatalf("expected 3 create attempts (2 transient 400s + 1 success), got %d", got)
	}
	want := []time.Duration{2 * time.Second, 4 * time.Second}
	if len(*waits) != len(want) || (*waits)[0] != want[0] || (*waits)[1] != want[1] {
		t.Fatalf("expected backoff waits %v, got %v", want, *waits)
	}
	if got := countMRListGets(s); got != 2 {
		t.Fatalf("expected one open-MR read before each of the 2 retries, got %d", got)
	}
}

// TestGLCreateMRKeyedGivesUp pins the bound for issue #6281's shape: GitLab never sees the
// branch, so the create stops after gitlabCreateMRAttempts with the total wait at most 30s,
// and the failure carries the same exit code and the same GitLab error it reports without
// the retry.
//
// FAIL-FIRST: before the fix this shape was never retried — 1 create attempt, no waits — RED.
//
// regression: #6281
func TestGLCreateMRKeyedGivesUp(t *testing.T) {
	s := newGLServer(t)
	s.createMRTransientFails = 99
	s.createMRTransientKeyed = true
	f := s.forge()
	waits := sleepLog(f)

	ref, err := f.CreateDraftChange(glRepo,
		DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
	if err == nil || ref != nil {
		t.Fatalf("expected could-not-check and no ref after the bound ran out, got ref %+v err %v", ref, err)
	}
	if got := countCreateMRPosts(s); got != gitlabCreateMRAttempts {
		t.Fatalf("expected exactly gitlabCreateMRAttempts=%d create attempts, got %d", gitlabCreateMRAttempts, got)
	}
	var total time.Duration
	for i, d := range *waits {
		if want := gitlabCreateMRRetryDelay << i; d != want {
			t.Fatalf("wait %d = %s, want %s (doubling backoff); all waits %v", i, d, want, *waits)
		}
		total += d
	}
	if len(*waits) != gitlabCreateMRAttempts-1 {
		t.Fatalf("expected %d waits, got %v", gitlabCreateMRAttempts-1, *waits)
	}
	if total != gitlabCreateMRMaxWait || total > 30*time.Second {
		t.Fatalf("total backoff %s must equal gitlabCreateMRMaxWait (%s) and stay within 30s",
			total, gitlabCreateMRMaxWait)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("give-up must keep the could-not-check exit (%d), got %d: %v",
			ExitUnverifiable, ExitCodeOf(err), err)
	}
	var ae *ForgeAPIError
	if !errors.As(err, &ae) || ae.Status != http.StatusBadRequest || ae.Body != glKeyedMissingBody {
		t.Fatalf("expected GitLab's 400 with body %q to survive errors.As, got: %v", glKeyedMissingBody, err)
	}
	if !strings.Contains(err.Error(), "HTTP 400: "+ae.Error()) {
		t.Fatalf("the surfaced error must carry the unretried error text verbatim, got: %v", err)
	}
}

// TestGLCreateMRTargetMissingNoRetry proves the widened classifier stays narrow: a
// field-keyed 400 that names the TARGET branch as missing — alone, or beside a different
// source_branch complaint — is surfaced on the first response and never retried.
//
// regression: #6281
func TestGLCreateMRTargetMissingNoRetry(t *testing.T) {
	bodies := []map[string]any{
		{"message": map[string]any{"target_branch": []string{"does not exist"}}},
		{"message": map[string]any{
			"source_branch": []string{"is invalid"},
			"target_branch": []string{"does not exist"}}},
	}
	for _, b := range bodies {
		s := newGLServer(t)
		s.createMRStatus = http.StatusBadRequest
		s.createMRErrBody = b
		f := s.forge()
		waits := sleepLog(f)
		ref, err := f.CreateDraftChange(glRepo,
			DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
		if err == nil || ref != nil {
			t.Fatalf("body %v: expected the 400 surfaced as an error, got ref %+v", b, ref)
		}
		if got := countCreateMRPosts(s); got != 1 || len(*waits) != 0 {
			t.Fatalf("body %v: must not retry: %d attempts, waits %v", b, got, *waits)
		}
	}
}

// TestGLCreateMRNoRetryWhenOpen pins the duplicate guard: when a merge request is already
// open on the source branch by the time a retry is due, the backend does not POST again
// and names the open one, keeping the could-not-check exit and GitLab's 400.
//
// FAIL-FIRST: before the fix no open-MR read ran and the error named no open MR — RED.
//
// regression: #6281
func TestGLCreateMRNoRetryWhenOpen(t *testing.T) {
	s := newGLServer(t)
	s.createMRTransientFails = 99
	s.createMRTransientKeyed = true
	s.mrList = []map[string]any{glMR(map[string]any{"iid": 44, "source_branch": "feat/one"})}
	f := s.forge()
	_ = sleepLog(f)

	ref, err := f.CreateDraftChange(glRepo,
		DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
	if err == nil || ref != nil {
		t.Fatalf("expected a refusal to retry, got ref %+v err %v", ref, err)
	}
	if got := countCreateMRPosts(s); got != 1 {
		t.Fatalf("an open MR on the branch must stop the retry: expected 1 create attempt, got %d", got)
	}
	if got := countMRListGets(s); got != 1 {
		t.Fatalf("expected exactly 1 open-MR read before the would-be retry, got %d", got)
	}
	if !strings.Contains(err.Error(), "!44 is already open") || !strings.Contains(err.Error(), glKeyedMissingBody) {
		t.Fatalf("error must name the open MR and keep GitLab's body, got: %v", err)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("expected the could-not-check exit, got %d", ExitCodeOf(err))
	}
}

// TestGLCreateMRNoRetryOnListFail: when the open-MR read before a retry fails, "none open"
// is unconfirmed, so the backend does not POST again.
//
// FAIL-FIRST: before the fix no read ran, so the error never named the unconfirmed read — RED.
//
// regression: #6281
func TestGLCreateMRNoRetryOnListFail(t *testing.T) {
	s := newGLServer(t)
	s.createMRTransientFails = 99
	s.createMRTransientKeyed = true
	s.mrListStatus = http.StatusInternalServerError
	f := s.forge()
	_ = sleepLog(f)

	ref, err := f.CreateDraftChange(glRepo,
		DraftChangeInput{Title: "t", Body: "b", Head: "feat/one", Base: "main"})
	if err == nil || ref != nil {
		t.Fatalf("expected a refusal to retry, got ref %+v err %v", ref, err)
	}
	if got := countCreateMRPosts(s); got != 1 {
		t.Fatalf("a failed open-MR read must stop the retry: expected 1 create attempt, got %d", got)
	}
	if !strings.Contains(err.Error(), "could not confirm that no merge request is open") ||
		!strings.Contains(err.Error(), glKeyedMissingBody) {
		t.Fatalf("error must name the unconfirmed read and keep GitLab's body, got: %v", err)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("expected the could-not-check exit, got %d", ExitCodeOf(err))
	}
}
