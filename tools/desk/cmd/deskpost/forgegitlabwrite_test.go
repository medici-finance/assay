package main

// forgegitlabwrite_test.go — deskpost's verdict WRITE path on a GitLab-resolved repo
// (forge-gitlab brief 09 §1, Verify item 2). Before this wiring, `deskpost review` on a GitLab
// repo failed CLOSED in newGHClient with `deskpost has no gitlab write backend … exit 6` (the
// #772 follow-up); now newPostBackend routes a GitLab-resolved repo through the typed Forge
// surface, so the verb forms — and, on a real run, LANDS — the verdict through deskkit.Forge.
//
// The GitLab wire mapping itself (approve → approval + head-SHA note, request-changes →
// unapprove + note, the 403 → could-not-check surface) is proven at the Forge level in
// internal/deskkit/forge_gitlab_reviewwrite_test.go (Verify items 3 and 7). These tests prove
// the deskpost WIRING: that the verb reaches the Forge write path at all on GitLab, and that its
// preconditions resolve through the typed Forge reads rather than the GitHub-only client. The
// backend is a fake Forge injected through the forgeForReviewer seam, so no live GitLab instance
// is contacted (offline envelope).

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// glReviewFake is a fake deskkit.Forge scripted for the review dry-run/real-run precondition
// path. It embeds the interface so any op the path does NOT exercise panics if reached (a
// louder failure than a silent zero value). Every field defaults to a happy-path fixture.
type glReviewFake struct {
	deskkit.Forge // nil — unimplemented ops panic if reached

	head        string
	prAuthor    deskkit.Account
	prBody      string
	commitLogin string

	mu           sync.Mutex
	postedReview []deskkit.ReviewInput

	// Merge-hold fixture state (the forge-gitlab merge-hold brief, task 3). holdState
	// defaults to MergeHoldUnresolved ("" == unresolved) — the ordinary state of a freshly
	// opened change nobody has reviewed yet.
	holdState      string
	holdResolvedBy string
	holdHead       string
	holdReadErr    error
	holdSetErr     error
	setHoldCalls   []deskkit.MergeHoldUpdate
	// reportNote, when set, is handed to ReviewInput.Report on a successful PostReview — the
	// GitLab backend's "approval already stood" success-with-note (#1106).
	reportNote string
}

func (g *glReviewFake) GetPullRequest(_ deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error) {
	return &deskkit.PullRequest{
		Number: number, State: "open", Draft: true, NodeID: "gid://gitlab/MergeRequest/1",
		Author: g.prAuthor, HeadSHA: g.head, Body: g.prBody, ChangedFiles: 1,
	}, nil
}
func (g *glReviewFake) ReviewsAtHead(deskkit.ForgeRepo, int) ([]deskkit.Review, error) {
	return nil, nil // no prior verdict at head
}
func (g *glReviewFake) ListLabelEvents(deskkit.ForgeRepo, int) ([]deskkit.LabelEvent, error) {
	return nil, nil // unstamped → the model floor proceeds with a NOTICE
}
func (g *glReviewFake) MatchingRefs(deskkit.ForgeRepo, string) ([]string, error) {
	// Mirrors the real GitLab backend: CE cannot prefix-list custom refs, so the review-claim
	// family read is could-not-check → ClaimLivenessUnknown. The PR here is unstamped, so the
	// floor is already at its NOTICE outcome and the age-out changes nothing either way.
	return nil, deskkit.Unverifiable("gitlab: no ref-listing endpoint (fake mirrors the CE limit)", nil)
}
func (g *glReviewFake) ListChangedFiles(deskkit.ForgeRepo, int) ([]deskkit.ChangedFile, error) {
	// One non-risk path, matching GetPullRequest's ChangedFiles:1 (no short read). The
	// unstamped floor's ruling-3 risk overlay reads this on a NOTICE outcome: a private repo
	// with a docs-only diff is NOT risk-classed, so the verdict proceeds with its NOTICE.
	return []deskkit.ChangedFile{{Filename: "docs/desk-tools.md"}}, nil
}
func (g *glReviewFake) RepoVisibility(deskkit.ForgeRepo) (string, error) {
	return "private", nil // the public-repo +1 gate is a no-op on a private repo
}
func (g *glReviewFake) GetCommit(_ deskkit.ForgeRepo, sha string) (*deskkit.RepoCommit, error) {
	return &deskkit.RepoCommit{SHA: sha, AuthorLogin: g.commitLogin}, nil
}
func (g *glReviewFake) PostReview(_ deskkit.ForgeRepo, _ int, in deskkit.ReviewInput) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.postedReview = append(g.postedReview, in)
	if g.reportNote != "" && in.Report != nil {
		in.Report(g.reportNote)
	}
	return nil
}
func (g *glReviewFake) ReadMergeHold(deskkit.ForgeRepo, int) (*deskkit.MergeHold, error) {
	if g.holdReadErr != nil {
		return nil, g.holdReadErr
	}
	state := g.holdState
	if state == "" {
		state = deskkit.MergeHoldUnresolved
	}
	return &deskkit.MergeHold{State: state, ID: "disc-1", ResolvedBy: g.holdResolvedBy, Head: g.holdHead}, nil
}
func (g *glReviewFake) SetMergeHold(_ deskkit.ForgeRepo, _ int, in deskkit.MergeHoldUpdate) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.setHoldCalls = append(g.setHoldCalls, in)
	return g.holdSetErr
}

// setupGitLabReview plants a roster binding a GitLab repo, injects the fake Forge through the
// forgeForReviewer seam, and wires the audit/stdout/stderr scaffolding the verb needs — the
// GitLab twin of setupFake, minus the GitHub fake server (the forge backend never touches it).
func setupGitLabReview(t *testing.T, f *glReviewFake) *bytes.Buffer {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	const roster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,reviewer=assay-reviewer-app:300000004,worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=gl-group/gl-repo:ci:private
ASSAY_REPO_FORGES=gl-group/gl-repo=gitlab
`
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir config home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)

	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "deskpost-gitlab-test")
	// Outward verbs refuse with $DESK_LOOP unset; the review path is exercised past that gate.
	t.Setenv("DESK_LOOP", "pr-review-desk")

	oldFor := forgeForReviewer
	forgeForReviewer = func(deskkit.ForgeRepo) (deskkit.Forge, error) { return f, nil }
	t.Cleanup(func() { forgeForReviewer = oldFor })

	var errBuf bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout = &bytes.Buffer{}
	stderr = &errBuf
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })

	return &errBuf
}

func newGLReviewFake() *glReviewFake {
	return &glReviewFake{
		head:        testHead,
		prAuthor:    deskkit.Account{Login: "shared-agent", ID: 2002}, // trusted → trust gate passes
		prBody:      "does the thing\n",                               // no claim trailer → claimLiveness Unknown
		commitLogin: "shared-agent",                                   // != the reviewer bot → non-author check OK
	}
}

const glReviewRepo = "gl-group/gl-repo"

// Verify item 2: `deskpost review --dry-run` on a GitLab-resolved repo forms an APPROVE verdict
// through the Forge write path — exit 0, and the output does NOT carry the exit-6 fail-closed.
func TestGitLabReviewDryRunFormsVerdict(t *testing.T) {
	f := newGLReviewFake()
	errBuf := setupGitLabReview(t, f)
	bf := writeBody(t, "rev.md", okReviewBody)

	args := append(reviewArgs(glReviewRepo, "1", "approve", testHead, bf), "--dry-run")
	code := run(args)
	if code != 0 {
		t.Fatalf("dry-run review on a GitLab repo exit = %d, want 0\nstderr:\n%s", code, errBuf.String())
	}
	// The whole point of the brief: the exit-6 fail-closed is GONE on the GitLab review path.
	if strings.Contains(errBuf.String(), "no gitlab write backend") {
		t.Fatalf("output still carries the #772 fail-closed string:\n%s", errBuf.String())
	}
	if len(f.postedReview) != 0 {
		t.Fatalf("a dry run must not POST a verdict, but PostReview was called %d time(s)", len(f.postedReview))
	}
	if e := lastAudit(t); e.Result != deskkit.ResultDryRun || e.Verb != "review:correctness:approve" {
		t.Fatalf("audit = %+v, want verb=review:correctness:approve result=dryrun", e)
	}
}

// The real (non-dry-run) run LANDS the verdict through the typed Forge op — PostReview is the
// shipping consumer the §6 freeze rule requires (the verifier's row-2 corollary: the op landed
// with no non-test consumer). It maps the correctness approve to an APPROVE event pinned to the
// reviewed head, carrying the review body.
func TestGitLabReviewRealRunLandsVerdictThroughForge(t *testing.T) {
	f := newGLReviewFake()
	_ = setupGitLabReview(t, f)
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(glReviewRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("real review on a GitLab repo exit = %d, want 0", code)
	}
	if len(f.postedReview) != 1 {
		t.Fatalf("PostReview called %d time(s), want 1 — the verdict must land through the Forge op", len(f.postedReview))
	}
	got := f.postedReview[0]
	if got.Event != "APPROVE" {
		t.Errorf("PostReview event = %q, want APPROVE", got.Event)
	}
	if got.HeadSHA != testHead {
		t.Errorf("PostReview head = %q, want the reviewed head %q", got.HeadSHA, testHead)
	}
	if !strings.Contains(got.Body, "Verdict: approve") {
		t.Errorf("PostReview body did not carry the verdict text; got %q", got.Body)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultOK {
		t.Fatalf("audit result = %q, want ok", e.Result)
	}
	// task 3: an approve at head RELEASES the merge-hold, naming that head.
	if len(f.setHoldCalls) != 1 {
		t.Fatalf("SetMergeHold called %d time(s), want exactly 1", len(f.setHoldCalls))
	}
	if h := f.setHoldCalls[0]; !h.Resolved || h.Head != testHead {
		t.Errorf("SetMergeHold = %+v, want a release naming head %q", h, testHead)
	}
}

// task 3: releasing the merge-hold when it is resolved at a STALE head re-arms FIRST (naming
// the new head), then applies the verdict's own release — two writes, in that order.
func TestGitLabReviewApproveAtNewHeadRearmsStaleHoldFirst(t *testing.T) {
	f := newGLReviewFake()
	f.holdState = deskkit.MergeHoldResolved
	f.holdResolvedBy = "assay-reviewer-app"
	f.holdHead = "000000000000000000000000000000000000dead" // stale — not testHead
	_ = setupGitLabReview(t, f)
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(glReviewRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("approve review on a GitLab repo exit = %d, want 0", code)
	}
	if len(f.setHoldCalls) != 2 {
		t.Fatalf("SetMergeHold called %d time(s), want 2 (re-arm the stale resolve, then release at head): %+v",
			len(f.setHoldCalls), f.setHoldCalls)
	}
	if h := f.setHoldCalls[0]; h.Resolved || !strings.Contains(h.Reason, "new head") {
		t.Errorf("first SetMergeHold = %+v, want a re-arm naming the new head", h)
	}
	if h := f.setHoldCalls[1]; !h.Resolved || h.Head != testHead {
		t.Errorf("second SetMergeHold = %+v, want a release at %q", h, testHead)
	}
}

// task 3: a verdict that posts but whose hold write fails exits non-zero, naming the failure —
// never a silent success that leaves the recorded verdict and the server-side gate out of step.
func TestGitLabReviewApproveMergeHoldWriteFailureIsLoud(t *testing.T) {
	f := newGLReviewFake()
	f.holdSetErr = errors.New("503 the instance is unavailable")
	errBuf := setupGitLabReview(t, f)
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs(glReviewRepo, "1", "approve", testHead, bf))
	if code == 0 {
		t.Fatalf("expected a non-zero exit when releasing the merge-hold fails; stderr:\n%s", errBuf.String())
	}
	if len(f.postedReview) != 1 {
		t.Fatalf("the verdict itself must still have posted before the hold-release failure, got %d posts",
			len(f.postedReview))
	}
}

// A request-changes verdict on GitLab reaches the same Forge write path with the
// REQUEST_CHANGES event — the symmetric mapping the GitLab backend turns into unapprove + a
// head-SHA verdict note (proven at the Forge level in forge_gitlab_reviewwrite_test.go).
func TestGitLabReviewRequestChangesRoutesThroughForge(t *testing.T) {
	f := newGLReviewFake()
	_ = setupGitLabReview(t, f)
	bf := writeBody(t, "rev.md", "## Review\n\nBlocking issue found.\n\nVerdict: request-changes\n")

	if code := run(reviewArgs(glReviewRepo, "1", "request-changes", testHead, bf)); code != 0 {
		t.Fatalf("request-changes review on a GitLab repo exit = %d, want 0", code)
	}
	if len(f.postedReview) != 1 || f.postedReview[0].Event != "REQUEST_CHANGES" {
		t.Fatalf("PostReview = %+v, want one REQUEST_CHANGES verdict", f.postedReview)
	}
	// task 3: request-changes RE-ARMS the merge-hold, naming the reason.
	if len(f.setHoldCalls) != 1 {
		t.Fatalf("SetMergeHold called %d time(s), want exactly 1", len(f.setHoldCalls))
	}
	if h := f.setHoldCalls[0]; h.Resolved || h.Reason != "request-changes" {
		t.Errorf("SetMergeHold = %+v, want a re-arm with reason \"request-changes\"", h)
	}
}

// TestGitLabReviewAlreadyApprovedNoteIsReported (#1106): when the backend lands the verdict
// by a route other than the plain POST — GitLab's approve route answered 401 because the
// App's approval already stood — the verb SUCCEEDS (exit 0, audit ok, so the idempotency
// store records the body) and REPORTS which route it saw: the note reaches stderr and the
// audit detail, naming the endpoint. A success that swallowed the note would leave the
// operator unable to tell "posted" from "already in force", and a refusal is the defect.
func TestGitLabReviewAlreadyApprovedNoteIsReported(t *testing.T) {
	f := newGLReviewFake()
	f.reportNote = "already approved by example-bot (id 42) — POST /projects/example-org%2Fexample-project/merge_requests/1/approve answered HTTP 401 because this identity's approval already stands"
	errBuf := setupGitLabReview(t, f)
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(glReviewRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("an already-standing approval must be success, exit = %d, want 0\nstderr:\n%s", code, errBuf.String())
	}
	if len(f.postedReview) != 1 || f.postedReview[0].Report == nil {
		t.Fatalf("PostReview must be called once with a Report sink wired; got %+v", f.postedReview)
	}
	if !strings.Contains(errBuf.String(), "deskpost: NOTE: "+f.reportNote) {
		t.Errorf("the backend's note must reach stderr; stderr:\n%s", errBuf.String())
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultOK {
		t.Fatalf("audit result = %q, want ok (the verdict is in force)", e.Result)
	}
	if !strings.Contains(e.Detail, f.reportNote) {
		t.Errorf("audit detail must carry the note; got %q", e.Detail)
	}
}
