package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A local branch of the derived name already exists in the SHARED refs store — the state an
// abandoned dispatch leaves behind, because `git worktree remove` does not delete the branch
// the worktree was on. Before the fix every shape of this arrived as a bare
// "git worktree add failed" plus git's own stderr, which through a wrapper reduced to the
// config echo; the operator had to re-run raw git to learn that a stray ref was the blocker.
//
// The three shapes are three different answers, and this table is the fence around that:
// a proven-empty leftover is RECLAIMED, a branch someone is standing on is REFUSED naming
// the worktree, and a branch carrying unpushed commits is REFUSED naming the count.

// branchStale plants a local branch at origin/main with an upstream — 0 ahead, no worktree.
func branchStale(t *testing.T, work, br string) {
	t.Helper()
	mustGit(t, work, "branch", "--track", br, "refs/remotes/origin/main")
}

// branchAhead plants a local branch carrying one commit its upstream does not have.
func branchAhead(t *testing.T, work, br string) {
	t.Helper()
	branchStale(t, work, br)
	// Commit onto the branch without checking it out: build a tree from the index of the
	// base commit, commit it against the branch tip, and move the ref. Keeps the fixture's
	// own HEAD (and thus the "no worktree holds it" property) untouched.
	base := mustGit(t, work, "rev-parse", "refs/heads/"+br)
	tree := mustGit(t, work, "rev-parse", base+"^{tree}")
	sha := mustGit(t, work, "-c", "user.email=t@e.st", "-c", "user.name=Test",
		"commit-tree", tree, "-p", base, "-m", "unpushed work")
	mustGit(t, work, "update-ref", "refs/heads/"+br, sha)
}

// branchHeld plants a local branch AND a second worktree standing on it.
func branchHeld(t *testing.T, work, br string) string {
	t.Helper()
	held := filepath.Join(t.TempDir(), "held-"+br)
	mustGit(t, work, "worktree", "add", "-b", br, held, "refs/remotes/origin/main")
	return held
}

func TestAddResolvesALocalBranchCollisionByName(t *testing.T) {
	cases := []struct {
		name string
		// plant returns the worktree path expected to be NAMED in the message, if any.
		plant   func(t *testing.T, work, br string) string
		wantRC  int
		wantOut []string // substrings the operator-facing message must carry
		// wantBranchGone: the collision was cleared by reclaiming the ref.
		wantWorktree bool
	}{
		{
			name: "stale 0-ahead leftover is reclaimed",
			plant: func(t *testing.T, work, br string) string {
				branchStale(t, work, br)
				return ""
			},
			wantRC:       deskkit.ExitOK,
			wantOut:      []string{"reclaimed stale local branch", "0 commits ahead of"},
			wantWorktree: true,
		},
		{
			name: "branch ahead of its upstream is refused, naming the count",
			plant: func(t *testing.T, work, br string) string {
				branchAhead(t, work, br)
				return ""
			},
			wantRC:  deskkit.ExitRefused,
			wantOut: []string{"already exists", "1 commit(s) not in", "unfinished work"},
		},
		{
			name: "branch held by another worktree is refused, naming the path",
			plant: func(t *testing.T, work, br string) string {
				return branchHeld(t, work, br)
			},
			wantRC:  deskkit.ExitRefused,
			wantOut: []string{"CHECKED OUT in the worktree", "deskwt remove "},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			work := newRepo(t)
			calls := withEnv(t, work)
			const name = "collide"
			heldPath := c.plant(t, work, name)

			rc, stderr := runCapErr(t, []string{"add", name})
			if rc != c.wantRC {
				t.Fatalf("add rc = %d, want %d; stderr:\n%s", rc, c.wantRC, stderr)
			}
			for _, want := range c.wantOut {
				if !strings.Contains(stderr, want) {
					t.Errorf("message does not name the cause: missing %q in:\n%s", want, stderr)
				}
			}
			if heldPath != "" && !strings.Contains(stderr, heldPath) {
				t.Errorf("refusal did not name the holding worktree %s:\n%s", heldPath, stderr)
			}
			// A refusal must never leave the branch — or anything else — mutated.
			if c.wantRC != deskkit.ExitOK {
				if out := mustGit(t, work, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); out == "" {
					t.Error("a refusal deleted the colliding branch")
				}
				if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-"+name)); !os.IsNotExist(err) {
					t.Errorf("a refusal created a worktree anyway (err=%v)", err)
				}
			}
			if c.wantWorktree {
				target := filepath.Join(tmpBaseDir, "tracker-"+name)
				if _, err := os.Stat(target); err != nil {
					t.Fatalf("reclaim did not produce the worktree %s: %v", target, err)
				}
				holders, herr := branchHolders(work)
				if herr != nil {
					t.Fatalf("branchHolders: %v", herr)
				}
				if got := holders["refs/heads/"+name]; got != resolvePath(target) {
					t.Errorf("branch %s is held by %q after reclaim, want %s", name, got, resolvePath(target))
				}
			}
			// No force verb reaches git on ANY of the three paths: the tool's own proof is
			// the gate, and the delete is a compare-and-delete on the proven sha.
			if anyGitForce(*calls) {
				t.Errorf("a git argv carried a force flag: %v", gitCalls(*calls))
			}
			for _, g := range gitCalls(*calls) {
				if len(g) > 2 && g[1] == "branch" && (g[2] == "-D" || g[2] == "--delete") {
					t.Errorf("the reclaim used `git branch -D`, not the compare-and-delete: %v", g)
				}
			}
		})
	}
}

// The compare-and-delete is what makes the reclaim safe under a race: the ref is deleted
// only if it still points where the 0-ahead proof was taken.
func TestReclaimDeletesTheBranchWithACompareAndDelete(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	branchStale(t, work, "cad")
	sha := mustGit(t, work, "rev-parse", "refs/heads/cad")

	if rc := run([]string{"add", "cad"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	var found bool
	for _, g := range gitCalls(*calls) {
		if len(g) >= 5 && g[1] == "update-ref" && g[2] == "-d" && g[3] == "refs/heads/cad" && g[4] == sha {
			found = true
		}
	}
	if !found {
		t.Fatalf("no `git update-ref -d refs/heads/cad %s` in: %v", sha, gitCalls(*calls))
	}
}

// An unreadable worktree list must fail CLOSED. Reading it as "nobody holds this branch"
// is the one reading that deletes a branch out from under a live worktree.
func TestBranchHoldersPropagatesAGitError(t *testing.T) {
	if _, err := branchHolders(t.TempDir()); err == nil {
		t.Fatal("branchHolders outside a repo returned no error")
	} else if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d", deskkit.ExitCodeOf(err), deskkit.ExitUnverifiable)
	}
}

// A branch whose upstream config points at a ref that no longer resolves is still
// MEASURABLE — against --base. Without the fallback such a branch would be unmeasurable,
// and an unmeasurable branch is a permanently stuck dispatch.
func TestComparisonRefFallsBackToBaseWhenTheUpstreamIsGone(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	branchStale(t, work, "lostup")
	mustGit(t, work, "config", "branch.lostup.merge", "refs/heads/vanished")

	if got := resolveComparisonRef(work, "refs/heads/lostup", "origin/main"); got != "origin/main" {
		t.Fatalf("comparison ref = %q, want the --base fallback origin/main", got)
	}
	if rc := run([]string{"add", "lostup"}); rc != deskkit.ExitOK {
		t.Fatalf("add over a branch with a dangling upstream rc = %d, want 0", rc)
	}
}
