package main

// push_test.go — goldens for the in-process push (push.go), every one against the fixture's
// LOCAL bare remote: nothing here reaches a forge.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// pushFixture is newBaseFixture (work tree on feature/test-branch, push URL = the local
// bare) with git's global/system config sealed off, so no ambient core.hooksPath runs a
// hook the test did not plant. It returns the work tree and the bare's path.
func pushFixture(t *testing.T) (work, bare string) {
	t.Helper()
	hermeticGitConfig(t)
	work = newBaseFixture(t)
	return work, filepath.Join(filepath.Dir(work), "origin.git")
}

func branchSpec(work string) pushSpec {
	return pushSpec{dir: work, repo: "example-org/tracker", originURL: ghURL,
		srcRef: "refs/heads/feature/test-branch", dstBranch: "feature/test-branch"}
}

// TestPushLandsRef — a valid push lands the ref: the bare holds the branch at HEAD.
func TestPushLandsRef(t *testing.T) {
	work, bare := pushFixture(t)
	if err := pushBranch(branchSpec(work)); err != nil {
		t.Fatalf("push of a new branch: %v", err)
	}
	head := mustGit(t, work, "rev-parse", "HEAD")
	if got := mustGit(t, bare, "rev-parse", "refs/heads/feature/test-branch"); got != head {
		t.Fatalf("bare branch = %s, want HEAD %s", got, head)
	}
	// A fast-forward on top lands too.
	writeFile(t, filepath.Join(work, "more.txt"), "more\n")
	mustGit(t, work, "add", "more.txt")
	mustGit(t, work, "commit", "-m", "more")
	if err := pushBranch(branchSpec(work)); err != nil {
		t.Fatalf("fast-forward push: %v", err)
	}
	if got, want := mustGit(t, bare, "rev-parse", "refs/heads/feature/test-branch"), mustGit(t, work, "rev-parse", "HEAD"); got != want {
		t.Fatalf("bare branch after fast-forward = %s, want %s", got, want)
	}
}

// TestForcePushRejected — Verify row 3. A push that would need force (the local branch was
// rewritten after it was pushed) is REJECTED, and the remote branch keeps its old commit:
// nothing is silently forced. The refspec carries no "+" and PushOpts.Force is never set.
func TestForcePushRejected(t *testing.T) {
	work, bare := pushFixture(t)
	if err := pushBranch(branchSpec(work)); err != nil {
		t.Fatalf("seed push: %v", err)
	}
	pushed := mustGit(t, bare, "rev-parse", "refs/heads/feature/test-branch")

	// Rewrite: drop the pushed tip and commit something else in its place.
	mustGit(t, work, "reset", "--hard", "HEAD~1")
	writeFile(t, filepath.Join(work, "rewritten.txt"), "diverged\n")
	mustGit(t, work, "add", "rewritten.txt")
	mustGit(t, work, "commit", "-m", "rewritten history")

	err := pushBranch(branchSpec(work))
	if err == nil {
		t.Fatal("a push that needs force SUCCEEDED; it must be rejected")
	}
	var de *deskkit.DeskError
	if !errors.As(err, &de) || de.Code != deskkit.ExitUnverifiable {
		t.Fatalf("force-requiring push error = %v, want an exit-6 push failure", err)
	}
	if !strings.Contains(err.Error(), "non-fast-forward") {
		t.Fatalf("rejection %q does not say non-fast-forward", err)
	}
	if got := mustGit(t, bare, "rev-parse", "refs/heads/feature/test-branch"); got != pushed {
		t.Fatalf("remote branch moved to %s on a rejected push; want it still at %s", got, pushed)
	}

	// The same holds for the explicit-refspec shape (update --pr): HEAD onto a branch the
	// local history does not descend from.
	s := branchSpec(work)
	s.srcRef = "HEAD"
	if err := pushBranch(s); err == nil {
		t.Fatal("HEAD:<branch> push that needs force SUCCEEDED; it must be rejected")
	}
	if got := mustGit(t, bare, "rev-parse", "refs/heads/feature/test-branch"); got != pushed {
		t.Fatalf("remote branch moved to %s on a rejected HEAD push", got)
	}
}

// TestPushAttachedHeadLands — HEAD as the source while HEAD is a BRANCH (symbolic) pushes
// that branch's commit. go-git silently skips a symbolic refspec source, so pushing the ref
// name would report success having sent nothing; pushBranch resolves it first.
func TestPushAttachedHeadLands(t *testing.T) {
	work, bare := pushFixture(t)
	s := branchSpec(work)
	s.srcRef, s.dstBranch = "HEAD", "feature/elsewhere"
	if err := pushBranch(s); err != nil {
		t.Fatalf("HEAD push: %v", err)
	}
	if got, want := mustGit(t, bare, "rev-parse", "refs/heads/feature/elsewhere"), mustGit(t, work, "rev-parse", "HEAD"); got != want {
		t.Fatalf("bare feature/elsewhere = %q, want HEAD %s", got, want)
	}
}

// TestPushRecordsUpstream — the create push leaves what `git push -u` left: the
// remote-tracking ref at the pushed commit and the branch's upstream config. It runs the same
// two reads `deskwt remove`'s pushed-commits guard runs (UpstreamRef, then AheadCount from the
// upstream to HEAD), which refuse a worktree with no upstream.
func TestPushRecordsUpstream(t *testing.T) {
	work, _ := pushFixture(t)
	s := branchSpec(work)
	s.setUpstream = true
	if err := pushBranch(s); err != nil {
		t.Fatalf("create push: %v", err)
	}
	head := mustGit(t, work, "rev-parse", "HEAD")
	if got := mustGit(t, work, "rev-parse", "refs/remotes/origin/feature/test-branch"); got != head {
		t.Fatalf("tracking ref = %q, want the pushed HEAD %s", got, head)
	}
	r, err := gitcore.Open(work)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	up, err := r.UpstreamRef()
	if err != nil {
		t.Fatalf("no upstream after the create push (deskwt remove would refuse): %v", err)
	}
	if up != "refs/remotes/origin/feature/test-branch" {
		t.Fatalf("upstream = %q, want refs/remotes/origin/feature/test-branch", up)
	}
	if n, aerr := r.AheadCount(up, "HEAD"); aerr != nil || n != 0 {
		t.Fatalf("ahead of upstream = %d (%v), want 0", n, aerr)
	}
}

// TestPushRecordsTrackingRef — the update push (no upstream write, as `git push origin
// <src>:<dst>` never set one) still moves the destination's remote-tracking ref to the pushed
// commit, which deskpr update's offline identity anchor reads. A rejected push records nothing.
func TestPushRecordsTrackingRef(t *testing.T) {
	work, _ := pushFixture(t)
	s := branchSpec(work)
	s.srcRef, s.dstBranch = "HEAD", "feature/pr-head"
	if err := pushBranch(s); err != nil {
		t.Fatalf("update push: %v", err)
	}
	head := mustGit(t, work, "rev-parse", "HEAD")
	if got := mustGit(t, work, "rev-parse", "refs/remotes/origin/feature/pr-head"); got != head {
		t.Fatalf("tracking ref = %q, want the pushed HEAD %s", got, head)
	}
	if out, _ := git(work, "config", "--get-regexp", `^branch\.`); strings.TrimSpace(out) != "" {
		t.Fatalf("update push wrote upstream config %q; only create sets one", out)
	}
	// A force-requiring push is rejected and leaves the tracking ref where it was.
	mustGit(t, work, "reset", "--hard", "HEAD~1")
	writeFile(t, filepath.Join(work, "diverged.txt"), "x\n")
	mustGit(t, work, "add", "diverged.txt")
	mustGit(t, work, "commit", "-m", "diverged")
	if err := pushBranch(s); err == nil {
		t.Fatal("force-requiring push succeeded")
	}
	if got := mustGit(t, work, "rev-parse", "refs/remotes/origin/feature/pr-head"); got != head {
		t.Fatalf("rejected push moved the tracking ref to %s", got)
	}
}

// TestPushHttpsUsesEndpoint — an https destination is pushed to the URL the endpoint
// resolver returns for the GATED repo (the forge's canonical URL + in-memory credential),
// never to the configured string; a resolver error stops the push.
func TestPushHttpsUsesEndpoint(t *testing.T) {
	work, bare := pushFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", "https://github.com/example-org/tracker.git")
	old := endpointFn
	t.Cleanup(func() { endpointFn = old })

	var gotRepo, gotOrigin string
	endpointFn = func(repo, originURL string) (deskkit.ForgeGitEndpoint, error) {
		gotRepo, gotOrigin = repo, originURL
		ep := deskkit.ForgeGitEndpoint{Kind: deskkit.ForgeGitHub}
		ep.Opts.URL = bare
		return ep, nil
	}
	if err := pushBranch(branchSpec(work)); err != nil {
		t.Fatalf("https push via the endpoint: %v", err)
	}
	if gotRepo != "example-org/tracker" || gotOrigin != ghURL {
		t.Fatalf("endpoint resolved for (%q, %q), want (example-org/tracker, %s)", gotRepo, gotOrigin, ghURL)
	}
	mustGit(t, bare, "rev-parse", "refs/heads/feature/test-branch")

	endpointFn = func(string, string) (deskkit.ForgeGitEndpoint, error) {
		return deskkit.ForgeGitEndpoint{}, deskkit.Unverifiable("no credential", nil)
	}
	writeFile(t, filepath.Join(work, "x.txt"), "x\n")
	mustGit(t, work, "add", "x.txt")
	mustGit(t, work, "commit", "-m", "x")
	if err := pushBranch(branchSpec(work)); err == nil {
		t.Fatal("push proceeded although the endpoint could not be resolved")
	}
}

// TestPushOtherDestRefused — the push re-reads origin's push destination itself: an https
// URL naming another repo, or an SSH URL, is refused before anything is sent.
func TestPushOtherDestRefused(t *testing.T) {
	for _, dest := range []string{
		"https://github.com/someone-else/tracker.git",
		"ssh://git@example.invalid/example-org/tracker.git",
	} {
		work, bare := pushFixture(t)
		mustGit(t, work, "remote", "set-url", "--push", "origin", dest)
		err := pushBranch(branchSpec(work))
		var de *deskkit.DeskError
		if !errors.As(err, &de) || de.Code != deskkit.ExitRefused {
			t.Fatalf("push to %s = %v, want a refusal", dest, err)
		}
		if out := mustGit(t, bare, "for-each-ref", "refs/heads/"); out != "" {
			t.Fatalf("refused push to %s still landed: %s", dest, out)
		}
	}
}

// plantHook writes an executable pre-push hook into the work tree's hooks dir.
func plantHook(t *testing.T, work, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("sh hook fixture")
	}
	hooks := filepath.Join(work, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestPushRunsPrePushHook — the configured pre-push hook (the push guard, where installed)
// still runs before the in-process push, with git's contract: argv <name> <url>, stdin
// <local-ref> <local-sha> <remote-ref> <remote-sha>.
func TestPushRunsPrePushHook(t *testing.T) {
	work, bare := pushFixture(t)
	record := filepath.Join(t.TempDir(), "hook.txt")
	plantHook(t, work, "echo \"$1 $2\" > '"+record+"'\ncat >> '"+record+"'\n")
	if err := pushBranch(branchSpec(work)); err != nil {
		t.Fatalf("push with an accepting hook: %v", err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the pre-push hook never ran: %v", err)
	}
	head := mustGit(t, work, "rev-parse", "HEAD")
	want := "origin file://" + bare + "\nrefs/heads/feature/test-branch " + head +
		" refs/heads/feature/test-branch " + strings.Repeat("0", len(head)) + "\n"
	if string(raw) != want {
		t.Fatalf("hook saw:\n%q\nwant:\n%q", raw, want)
	}
}

// TestPushHookRefusalBlocks — a pre-push hook that exits non-zero stops the push: nothing
// lands, the push fails exit 6, and the hook's own output reaches stderr.
func TestPushHookRefusalBlocks(t *testing.T) {
	work, bare := pushFixture(t)
	plantHook(t, work, "echo 'guard: refused' >&2\nexit 1\n")
	stderr := withStderrCapture(t)
	err := pushBranch(branchSpec(work))
	var de *deskkit.DeskError
	if !errors.As(err, &de) || de.Code != deskkit.ExitUnverifiable {
		t.Fatalf("push past a refusing hook = %v, want an exit-6 failure", err)
	}
	if out := mustGit(t, bare, "for-each-ref", "refs/heads/"); out != "" {
		t.Fatalf("the hook refused but the push landed: %s", out)
	}
	if !strings.Contains(stderr.String(), "guard: refused") {
		t.Fatalf("the hook's output was swallowed; stderr: %q", stderr.String())
	}
}

// TestPushHookViaHooksPath — core.hooksPath (how the push guard is installed) is honoured.
func TestPushHookViaHooksPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh hook fixture")
	}
	work, bare := pushFixture(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pre-push"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "config", "core.hooksPath", dir)
	if err := pushBranch(branchSpec(work)); err == nil {
		t.Fatal("a refusing hook under core.hooksPath did not stop the push")
	}
	if out := mustGit(t, bare, "for-each-ref", "refs/heads/"); out != "" {
		t.Fatalf("the hooksPath hook refused but the push landed: %s", out)
	}
}
