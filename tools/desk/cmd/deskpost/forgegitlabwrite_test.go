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
	return nil
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
}
