package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// ---- the GitLab arm of the dead-claim decay (#1111) -------------------------

// gitlabFakeDoer answers each request from a scripted function, recording what it
// was asked. It keeps the whole GitLab arm offline — no network, no live project.
// (ghfetch_test.go's fakeDoer is a URL→canned-body table; this one needs to vary
// by query parameter to exercise paging, so it takes a function.)
type gitlabFakeDoer struct {
	respond func(req *http.Request) (status int, body string)
	reqs    []*http.Request
}

func (f *gitlabFakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.reqs = append(f.reqs, req)
	status, body := f.respond(req)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}, nil
}

func stubGitLabDoer(t *testing.T, d httpDoer) {
	t.Helper()
	prev := gitlabHTTPDoer
	gitlabHTTPDoer = d
	t.Cleanup(func() { gitlabHTTPDoer = prev })
}

// TestDecayDeadClaimsDecaysOnGitLabOrigin is the headline regression for #1111:
// on a GitLab-hosted root, a branch whose MERGE REQUEST has already merged is a
// corpse and must be decayed out of the claim set, exactly as a merged PR is on
// GitHub. Before the fix the forge gate returned the branch set untouched with a
// "NOT APPLICABLE" notice, so this branch survived as a claim forever and the
// briefs behind it were silently held.
func TestDecayDeadClaimsDecaysOnGitLabOrigin(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "4242")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "job-token-value")

	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(*http.Request) (int, string) {
		return 200, `[
		  {"source_branch":"fix/issue-loop-01-live",  "state":"opened","source_project_id":4242,"target_project_id":4242},
		  {"source_branch":"fix/issue-loop-02-merged","state":"merged","source_project_id":4242,"target_project_id":4242},
		  {"source_branch":"fix/issue-loop-03-closed","state":"closed","source_project_id":4242,"target_project_id":4242},
		  {"source_branch":"fix/issue-loop-05-locked","state":"locked","source_project_id":4242,"target_project_id":4242}
		]`
	}})

	branches := []string{
		"main",
		"fix/issue-loop-01-live",
		"fix/issue-loop-02-merged",
		"fix/issue-loop-03-closed",
		"fix/issue-loop-04-nopr",
		"fix/issue-loop-05-locked",
	}
	var got []string
	var reason string
	stderr := captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })

	want := []string{"main", "fix/issue-loop-01-live", "fix/issue-loop-04-nopr", "fix/issue-loop-05-locked"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GitLab decay = %v, want %v (merged and closed MR branches dropped, open/locked/no-MR kept)", got, want)
	}
	if reason != "" {
		t.Errorf("a decay that RAN must report no could-not-check reason; got %q", reason)
	}
	if strings.Contains(stderr, "could-not-check") || strings.Contains(stderr, "NOT APPLICABLE") {
		t.Errorf("a GitLab decay that RAN must not report could-not-check or not-applicable; got:\n%s", stderr)
	}
}

// TestForkMRNameNeverDecaysLiveClaim is the end-to-end guard on
// the pass's load-bearing invariant: decay may only ever shrink the claim set to
// what it VERIFIED is dead.
//
// `GET /projects/:id/merge_requests` returns every merge request TARGETING the
// project, forks included, and a fork's source_branch is a name chosen inside the
// fork — it names nothing in the tracked project. Matching dead claims by bare
// source_branch therefore let anyone who can fork and open a merge request (the
// ordinary contribution bar — no elevated access, no write to this project) open a
// throwaway MR named after a live claim branch, close it, and have the decay drop
// that live claim: the brief goes back on the board and a second worker is
// dispatched onto work already in flight. This walks the whole pass, not just the
// reader, because the claim set is what the invariant is about.
func TestForkMRNameNeverDecaysLiveClaim(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "4242")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "pat")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")

	// Project 4242 is the tracked project. Its own "fix/issue-loop-02-merged" MR
	// really did merge and SHOULD decay. Fork 9999 opened and closed a merge
	// request whose source_branch collides with the tracked project's live claim
	// "fix/issue-loop-01-live"; that one must survive untouched.
	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(*http.Request) (int, string) {
		return 200, `[
		  {"source_branch":"fix/issue-loop-01-live",  "state":"closed","source_project_id":9999,"target_project_id":4242},
		  {"source_branch":"fix/issue-loop-02-merged","state":"merged","source_project_id":4242,"target_project_id":4242}
		]`
	}})

	branches := []string{"main", "fix/issue-loop-01-live", "fix/issue-loop-02-merged"}
	var got []string
	var reason string
	stderr := captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })

	want := []string{"main", "fix/issue-loop-01-live"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decay = %v, want %v — the fork MR must not decay the live claim it collides with, and the project's own merged MR must still decay", got, want)
	}
	if reason != "" {
		t.Errorf("a decay that ran must report no could-not-check reason; got %q", reason)
	}
	if strings.Contains(stderr, "could-not-check") {
		t.Errorf("a fully-attributed listing must report no could-not-check; got:\n%s", stderr)
	}
}

// TestDecaySkipsUnattributedMR pins the fail DIRECTION of the
// fork check. A merge request whose source/target project cannot be read is one
// this reader could not attribute — and an unattributable merge request is
// indistinguishable from a fork's, whose source_branch does not name a branch of
// this project at all. Treating "no ids" as "same project" would put the decay
// back on the wrong side of its own invariant (shrink only to what was VERIFIED
// dead) for exactly the responses we understand least. It is skipped, counted, and
// reported — under-decay, never over-decay.
func TestDecaySkipsUnattributedMR(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "4242")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "pat")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")
	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(*http.Request) (int, string) {
		return 200, `[{"source_branch":"stream/07","state":"merged"}]`
	}})

	var dead map[string]bool
	var err error
	stderr := captureStderr(t, func() { dead, err = listMergedClosedBranchesGitLab("/repo") })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dead["stream/07"] {
		t.Error("a merge request whose projects could not be read must NOT decay a claim — it was never attributed to this project")
	}
	if !strings.Contains(stderr, "could-not-check: dead-claim decay skipped 1 merge request(s)") {
		t.Errorf("skipping an unattributable merge request must be reported, not silent; got:\n%s", stderr)
	}
}

// TestDecaySkipsOtherProjectMR is the belt-and-braces arm: when the
// numeric project id is known, a row targeting some OTHER project is a response we
// do not understand and must draw no conclusion from, even though its source and
// target agree with each other.
func TestDecaySkipsOtherProjectMR(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "4242")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "pat")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")
	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(*http.Request) (int, string) {
		return 200, `[{"source_branch":"stream/07","state":"merged","source_project_id":5555,"target_project_id":5555}]`
	}})

	dead, err := listMergedClosedBranchesGitLab("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dead["stream/07"] {
		t.Error("a merge request between two OTHER projects must not decay this project's branch")
	}
}

// TestDecayPathAddressedProject proves the numeric-id check does
// not break the local/outside-CI path, where CI_PROJECT_ID is unset and the project
// is addressed by its escaped path: source == target is then the whole test, and an
// ordinary same-project merge request still decays.
func TestDecayPathAddressedProject(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "")
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "pat")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")
	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(*http.Request) (int, string) {
		return 200, `[
		  {"source_branch":"stream/07","state":"merged","source_project_id":4242,"target_project_id":4242},
		  {"source_branch":"stream/08","state":"closed","source_project_id":9999,"target_project_id":4242}
		]`
	}})

	dead, err := listMergedClosedBranchesGitLab("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dead["stream/07"] {
		t.Error("a same-project merged merge request must decay even when the project is addressed by path")
	}
	if dead["stream/08"] {
		t.Error("a fork-sourced merge request must not decay, path-addressed or not")
	}
}

// TestGitLabDecayRequestShape pins what the reader actually asks GitLab for: the
// v4 merge-request listing for the project, in EVERY state (an `opened`-only
// listing could never find a corpse), authenticated with the right header for the
// credential it found.
func TestGitLabDecayRequestShape(t *testing.T) {
	stubRemoteOriginURL(t, "git@gitlab.example.com:acme/group/board.git", nil)
	t.Setenv("CI_API_V4_URL", "")
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "pat-value")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")

	doer := &gitlabFakeDoer{respond: func(*http.Request) (int, string) { return 200, `[]` }}
	stubGitLabDoer(t, doer)

	if _, err := listMergedClosedBranchesGitLab("/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doer.reqs) != 1 {
		t.Fatalf("want exactly one request, got %d", len(doer.reqs))
	}
	req := doer.reqs[0]
	url := req.URL.String()
	// Base derived from the remote host when CI_API_V4_URL is unset, project
	// addressed by its URL-encoded full path when CI_PROJECT_ID is unset.
	for _, want := range []string{
		"https://gitlab.example.com/api/v4/projects/",
		"acme%2Fgroup%2Fboard",
		"/merge_requests?",
		"state=all",
	} {
		if !strings.Contains(url, want) {
			t.Errorf("request URL %q is missing %q", url, want)
		}
	}
	if got := req.Header.Get("PRIVATE-TOKEN"); got != "pat-value" {
		t.Errorf("an explicit token must travel as PRIVATE-TOKEN; got header %q", got)
	}
	if req.Header.Get("JOB-TOKEN") != "" {
		t.Error("an explicit token must not also be sent as JOB-TOKEN")
	}
}

// TestGitLabDecayCIJobTokenUsesJobHeader pins the other credential shape: the
// pipeline's own CI_JOB_TOKEN is sent as JOB-TOKEN, which is the only header
// GitLab accepts it under.
func TestGitLabDecayCIJobTokenUsesJobHeader(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "7")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "ci-job-value")

	doer := &gitlabFakeDoer{respond: func(*http.Request) (int, string) { return 200, `[]` }}
	stubGitLabDoer(t, doer)
	if _, err := listMergedClosedBranchesGitLab("/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := doer.reqs[0].Header.Get("JOB-TOKEN"); got != "ci-job-value" {
		t.Errorf("CI_JOB_TOKEN must travel as JOB-TOKEN; got %q", got)
	}
}

// TestGitLabDecayPagesThroughEveryPage proves the listing is not truncated at one
// page: a corpse that only appears on the second page must still be decayed.
func TestGitLabDecayPagesThroughEveryPage(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "7")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "pat")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")

	// Page 1 is FULL (per_page entries) so the loop must ask for page 2; the
	// corpse lives only on page 2.
	full := make([]string, 0, gitlabMRPerPage)
	for i := 0; i < gitlabMRPerPage; i++ {
		full = append(full, fmt.Sprintf(`{"source_branch":"feat/filler-%d","state":"opened","source_project_id":7,"target_project_id":7}`, i))
	}
	page1 := "[" + strings.Join(full, ",") + "]"
	page2 := `[{"source_branch":"fix/late-corpse","state":"merged","source_project_id":7,"target_project_id":7}]`
	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(req *http.Request) (int, string) {
		if req.URL.Query().Get("page") == "1" {
			return 200, page1
		}
		return 200, page2
	}})

	dead, err := listMergedClosedBranchesGitLab("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dead["fix/late-corpse"] {
		t.Error("a merged merge request on page 2 must still be decayed — the listing must page")
	}
}

// TestGitLabDecayHTTPErrorIsNotAnEmptySet is the three-state property at the
// reader's own boundary: a refused listing must be an ERROR naming the status,
// never an empty set — an empty set means "looked, nothing is dead", which is
// indistinguishable from a clean run and is exactly the silent could-not-check
// this pass exists to prevent.
func TestGitLabDecayHTTPErrorIsNotAnEmptySet(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "7")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "bad")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")
	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(*http.Request) (int, string) {
		return 401, `{"message":"401 Unauthorized"}`
	}})

	dead, err := listMergedClosedBranchesGitLab("/repo")
	if err == nil {
		t.Fatalf("a 401 must be an error, not a clean empty set; got %v", dead)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("the error must name the HTTP status; got %v", err)
	}
}

// TestGitLabDecayRefusesWithoutAToken pins that an absent credential is a named
// could-not-check rather than an unauthenticated attempt: GitLab answers an
// unauthenticated listing of a private project with 404, whose empty body would
// decode to "nothing is dead".
func TestGitLabDecayRefusesWithoutAToken(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("CI_PROJECT_ID", "7")
	t.Setenv("STATUSGEN_GITLAB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")
	stubGitLabDoer(t, &gitlabFakeDoer{respond: func(*http.Request) (int, string) {
		t.Error("no request may be sent without a credential")
		return 200, `[]`
	}})

	if _, err := listMergedClosedBranchesGitLab("/repo"); err == nil {
		t.Fatal("an absent GitLab token must be an error, not an unauthenticated attempt")
	} else if !strings.Contains(err.Error(), "STATUSGEN_GITLAB_TOKEN") {
		t.Errorf("the refusal must name the variable an adopter can set; got %v", err)
	}
}

// TestRemoteProjectPath pins the project-path extraction across the remote URL
// forms an adopter can have configured, including a nested subgroup.
func TestRemoteProjectPath(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"https://gitlab.example.com/acme/board.git", "acme/board"},
		{"https://gitlab.example.com/acme/board", "acme/board"},
		{"git@gitlab.example.com:acme/group/board.git", "acme/group/board"},
		{"ssh://git@gitlab.example.com:2222/acme/board.git", "acme/board"},
		{"", ""},
	} {
		if got := remoteProjectPath(tc.raw); got != tc.want {
			t.Errorf("remoteProjectPath(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// ---- the loud failure, on ANY forge ----------------------------------------

// TestDecayUnavailableIsLoudOnGitLabToo is the other half of #1111: when the
// GitLab read cannot look, the pass must be LOUD and must keep the full branch
// set. Six days passed with this reported as a NOTICE on stderr while the board
// it wrote read clean; the marker is now could-not-check and the reason travels
// back to the caller so the artifact can wear it too.
func TestDecayUnavailableIsLoudOnGitLabToo(t *testing.T) {
	stubRemoteOriginURL(t, "https://gitlab.example.com/acme/board.git", nil)
	stubMergedClosedBranchesGitLab(t, nil, errors.New("GitLab API HTTP 403 listing merge requests"))

	branches := []string{"main", "fix/issue-loop-02-merged"}
	var got []string
	var reason string
	stderr := captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })

	if !reflect.DeepEqual(got, branches) {
		t.Fatalf("an unavailable decay must keep the full set (it may only SHRINK claims); got %v", got)
	}
	if !strings.Contains(stderr, "could-not-check: claims not decayed") {
		t.Errorf("the unavailable line must name could-not-check and that claims were not decayed; got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "HTTP 403") {
		t.Errorf("the unavailable line must carry the reason; got:\n%s", stderr)
	}
	if !strings.Contains(reason, "merge-request state over the GitLab REST v4 API") {
		t.Errorf("the reason must name WHICH read could not look; got %q", reason)
	}
}

// TestClaimSourceCarriesDecayCouldNotCheck pins that the could-not-check reaches
// BOTH surfaces a reader actually sees — the run's own output and the emitted
// board — and that it is not confused with the pre-existing claim-read
// degradation, which points the opposite way (superset vs subset).
func TestClaimSourceCarriesDecayCouldNotCheck(t *testing.T) {
	ran := ClaimSource{Known: true}
	if ran.DecayNotice() != "" || ran.DecayBanner() != "" {
		t.Error("a decay that ran must produce no could-not-check notice or banner")
	}

	blind := ClaimSource{Known: true, DecayReason: "PR state through `gh pr list` could not be read: gh: not found"}
	notice, banner := blind.DecayNotice(), blind.DecayBanner()
	if !strings.Contains(notice, "could-not-check: claims not decayed") {
		t.Errorf("DecayNotice must be marked could-not-check; got %q", notice)
	}
	if !strings.Contains(notice, "gh: not found") {
		t.Errorf("DecayNotice must carry the reason; got %q", notice)
	}
	if !strings.Contains(banner, "COULD-NOT-CHECK") || !strings.Contains(banner, "dead-claim decay did not run") {
		t.Errorf("DecayBanner must announce the could-not-check in the artifact; got %q", banner)
	}
	// Direction matters: this one says the board is a SUBSET. The claim-read
	// degradation says SUPERSET. Confusing them inverts the reader's conclusion.
	if !strings.Contains(banner, "subset") {
		t.Errorf("DecayBanner must say the board is a subset; got %q", banner)
	}
	if strings.Contains(banner, "unfiltered superset") {
		t.Errorf("DecayBanner must not reuse the claim-read degradation's superset wording; got %q", banner)
	}
}

// TestDecayBannerReachesTheEmittedBoard proves the banner is not merely
// renderable but actually rendered into STATUS.md's Next up section — the
// difference between a three-state instrument and a two-state one is whether the
// ARTIFACT wears it.
func TestDecayBannerReachesTheEmittedBoard(t *testing.T) {
	streams := []*Stream{mkStream("issue-loop", "active", "P0", Brief{Num: "01", Title: "One", Status: "todo"})}
	view := ClaimView{
		Claimed: map[string]bool{},
		Source:  ClaimSource{Known: true, DecayReason: "merge-request state over the GitLab REST v4 API could not be read: no GitLab API token in the environment"},
	}
	nu := nextUp(streams, view, nil)
	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "COULD-NOT-CHECK — dead-claim decay did not run") {
		t.Fatalf("the emitted board must carry the could-not-check banner; Next-up section:\n%s", out)
	}
}
