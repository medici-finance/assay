package gitcore

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// fetchscope_test.go — what an in-process Fetch may write, and what it must tolerate.
//
// Fetch writes the destinations of the refspecs it was handed and nothing else: no tag is
// followed, so an existing local tag is never replaced (git's own fetch never replaced one
// either). And a fetch from a local origin succeeds from a checkout holding commits the origin
// lacks — the ordinary state of a working checkout — because the local session ignores the
// client's "have" lines the origin cannot resolve, as git's upload-pack does.

// cloneFixture clones src into a fresh directory with the git binary and returns the clone as a
// fixture (deterministic identity and dates come from gittest's Git).
func cloneFixture(t *testing.T, src string) *gittest.Fixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", "-q", src, dir).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	f := &gittest.Fixture{Dir: dir}
	for _, kv := range [][2]string{{"user.name", "test"}, {"user.email", "test@example.invalid"}} {
		if _, err := f.Git("config", kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func mustFixtureGit(t *testing.T, f *gittest.Fixture, args ...string) string {
	t.Helper()
	out, err := f.Git(args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

// A local tag that differs from the upstream tag of the same name survives a fetch, and no
// upstream tag is written: with or without Force, over a wildcard tracking refspec (the shape
// whose default tag mode wrote every advertised tag whose object was present).
func TestFetchKeepsLocalTags(t *testing.T) {
	for _, force := range []bool{false, true} {
		origin := gittest.NewFixture(t)
		work := cloneFixture(t, origin.Dir)
		mustFixtureGit(t, work, "tag", "v1")
		localV1 := mustFixtureGit(t, work, "rev-parse", "refs/tags/v1")

		ahead := origin.CommitFile(t, "b.txt", "b\n", "ahead")
		mustFixtureGit(t, origin, "tag", "v1")
		mustFixtureGit(t, origin, "tag", "-a", "v2", "-m", "annotated")
		mustFixtureGit(t, origin, "tag", "v3")

		repo, err := Open(work.Dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{"+refs/heads/*:refs/remotes/origin/*"}, Force: force}); err != nil {
			t.Fatalf("Fetch(force=%v): %v", force, err)
		}
		if got := mustFixtureGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != ahead {
			t.Fatalf("force=%v: refs/remotes/origin/main = %s, want %s (the fetch must still land)", force, got, ahead)
		}
		if got := mustFixtureGit(t, work, "rev-parse", "refs/tags/v1"); got != localV1 {
			t.Errorf("force=%v: local tag v1 moved %s -> %s; a fetch must never replace a local tag", force, localV1, got)
		}
		if tags := mustFixtureGit(t, work, "for-each-ref", "--format=%(refname)", "refs/tags/"); tags != "refs/tags/v1" {
			t.Errorf("force=%v: tags after fetch = %q, want only the local refs/tags/v1 (no tag is followed)", force, tags)
		}
	}
}

// A fetch from a local origin into a checkout that holds a commit the origin does not have —
// on a side branch, and on the checked-out branch — lands the origin's new commit instead of
// failing on the unknown "have".
func TestLocalFetchWithUnpushedCommit(t *testing.T) {
	for _, onMain := range []bool{false, true} {
		origin := gittest.NewFixture(t)
		bare := filepath.Join(t.TempDir(), "origin.git")
		if out, err := exec.Command("git", "clone", "-q", "--bare", origin.Dir, bare).CombinedOutput(); err != nil {
			t.Fatalf("clone --bare: %v: %s", err, out)
		}
		work := cloneFixture(t, bare)
		if !onMain {
			mustFixtureGit(t, work, "checkout", "-q", "-b", "wip")
		}
		work.CommitFile(t, "local.txt", "only here\n", "unpushed")

		// Advance the origin by one commit the checkout does not have.
		pusher := cloneFixture(t, bare)
		ahead := pusher.CommitFile(t, "up.txt", "upstream\n", "upstream ahead")
		mustFixtureGit(t, pusher, "push", "-q", "origin", "main")

		for _, url := range []string{bare, "file://" + bare} {
			repo, err := Open(work.Dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Fetch(FetchOpts{URL: url, RefSpecs: []string{"+refs/heads/*:refs/remotes/origin/*"}}); err != nil {
				t.Fatalf("onMain=%v: Fetch(%s) from a checkout holding an unpushed commit: %v", onMain, url, err)
			}
			if got := mustFixtureGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != ahead {
				t.Fatalf("onMain=%v: refs/remotes/origin/main = %s, want %s", onMain, got, ahead)
			}
		}
	}
}

// CheckedOutBranches also holds the branch another worktree is in the middle of rebasing or
// bisecting (HEAD is detached there, so a HEAD-only read misses it), and the branches an
// in-progress `rebase --update-refs` will rewrite — git's own refusal set for fetch.
func TestCheckedOutBranchesInFlight(t *testing.T) {
	main := gittest.NewFixture(t)
	for i := 0; i < 3; i++ {
		main.CommitFile(t, "n.txt", strings.Repeat("x", i+1)+"\n", "c")
	}
	for _, b := range []string{"rebasing", "bisecting", "updating", "carried"} {
		mustFixtureGit(t, main, "branch", b)
	}
	// idle sits below every rebased range, so no in-flight operation names it.
	mustFixtureGit(t, main, "branch", "idle", "HEAD~3")
	addWT := func(name, branch string) *gittest.Fixture {
		dir := filepath.Join(t.TempDir(), name)
		mustFixtureGit(t, main, "worktree", "add", "-q", dir, branch)
		return &gittest.Fixture{Dir: dir}
	}

	// Stopped rebase: -x false stops after the first pick, HEAD detached.
	rb := addWT("rb", "rebasing")
	if _, err := rb.Git("rebase", "-q", "--force-rebase", "-x", "false", "HEAD~1"); err == nil {
		t.Fatal("the rebase fixture did not stop")
	}
	// Bisect: HEAD detached on a midpoint.
	bs := addWT("bs", "bisecting")
	mustFixtureGit(t, bs, "bisect", "start", "HEAD", "HEAD~3")
	for _, w := range []*gittest.Fixture{rb, bs} {
		if _, err := w.Git("symbolic-ref", "-q", "HEAD"); err == nil {
			t.Fatalf("fixture %s has a symbolic HEAD; the case needs a detached one", w.Dir)
		}
	}
	want := map[string]bool{"refs/heads/main": true, "refs/heads/rebasing": true, "refs/heads/bisecting": true}

	// rebase --update-refs (git 2.38+): the rebased branch and every branch it carries.
	up := addWT("up", "updating")
	mustFixtureGit(t, main, "branch", "-f", "carried", "updating~1")
	if out, err := up.Git("rebase", "-q", "--force-rebase", "--update-refs", "-x", "false", "HEAD~2"); err == nil {
		t.Fatal("the update-refs rebase fixture did not stop")
	} else if strings.Contains(err.Error(), "update-refs") && strings.Contains(err.Error(), "unknown") {
		t.Logf("git lacks rebase --update-refs (%s): that case is not exercised", out)
	} else {
		want["refs/heads/updating"] = true
		want["refs/heads/carried"] = true
	}

	for _, from := range []string{main.Dir, rb.Dir, bs.Dir} {
		got, err := CheckedOutBranches(from)
		if err != nil {
			t.Fatalf("CheckedOutBranches(from %s): %v", filepath.Base(from), err)
		}
		set := map[string]bool{}
		for _, r := range got {
			set[r] = true
		}
		for ref := range want {
			if !set[ref] {
				t.Errorf("CheckedOutBranches(from %s) = %v; missing %s", filepath.Base(from), got, ref)
			}
		}
		if set["refs/heads/idle"] {
			t.Errorf("CheckedOutBranches(from %s) = %v; holds refs/heads/idle, which no worktree uses", filepath.Base(from), got)
		}
	}
}
