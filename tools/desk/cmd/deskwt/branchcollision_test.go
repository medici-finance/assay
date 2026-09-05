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

// --- prunable (directory-gone) holders --------------------------------------------
//
// The commonest way a dispatch dies is `rm -rf` of the worktree DIRECTORY without
// `git worktree remove`: git still lists the entry, now marked `prunable`, still "on" its
// branch. That bookkeeping entry is NOT a live owner, so it must not make the holder scan
// refuse. These tests fence the read-fix in branchHolders: a prunable entry is skipped as a
// holder (so the 0-ahead proof and the live-holder / ahead refusals do the deciding), and the
// prunable attribute never leaks forward into the next, live, entry.
//
// NOTE ON SCOPE (see PR body — the needs-decision recorded there): the read-fix lets the
// reclaim PROCEED, but it cannot make the subsequent `git worktree add -b <br>` SUCCEED while
// the prunable entry survives — git's own `-b` collision check consults every worktree
// registration (prunable included), so it still refuses ("… is already used by worktree at
// …" / "missing but already registered worktree") even after the branch ref is deleted. Only
// `git worktree prune` drops the entry, and this tool forbids `add` from running it. So these
// tests pin the reachable, correct behaviour of the read-fix; the brief's row-2 "second add
// exits 0" is escalated, not asserted here.

// plantPrunableHolder plants a local branch `br` checked out in a worktree whose directory is
// then removed WITHOUT `git worktree remove` — the state a dead dispatch leaves. `git worktree
// list --porcelain` still lists the entry, now carrying a `prunable` attribute line, still on
// `br`. Placed OUTSIDE the sanctioned tracker- prefix; returns the (now-gone) path.
func plantPrunableHolder(t *testing.T, work, br string) string {
	t.Helper()
	dead := filepath.Join(t.TempDir(), "dead-"+br)
	mustGit(t, work, "worktree", "add", "-b", br, dead, "refs/remotes/origin/main")
	if err := os.RemoveAll(dead); err != nil {
		t.Fatalf("rm -rf prunable holder dir: %v", err)
	}
	// Positive control: the entry really is prunable and still names the branch.
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if !strings.Contains(list, "prunable") || !strings.Contains(list, "branch refs/heads/"+br) {
		t.Fatalf("fixture void: no prunable entry on %s after rm -rf:\n%s", br, list)
	}
	return dead
}

// branchHolders must not count a prunable (directory-gone) entry as a holder, and the prunable
// attribute of one entry must not leak into the next. The prunable entry is listed BEFORE the
// live one, so a sticky attribute would wrongly skip the live holder — the exact reading that
// would delete a branch out from under a live worktree.
func TestBranchHoldersSkipsPrunableEntry(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	plantPrunableHolder(t, work, "ghost") // listed first
	live := branchHeld(t, work, "live")   // a live worktree standing on `live`, listed after

	holders, err := branchHolders(work)
	if err != nil {
		t.Fatalf("branchHolders: %v", err)
	}
	if got, ok := holders["refs/heads/ghost"]; ok {
		t.Errorf("a prunable (directory-gone) entry was counted as a holder of ghost: %q", got)
	}
	if got := holders["refs/heads/live"]; got != resolvePath(live) {
		t.Errorf("live holder of `live` = %q, want %s — the prunable attribute leaked across entries", got, resolvePath(live))
	}
}

// Verify row 3. A LIVE holder (directory present) is still a real owner: `add` refuses (5)
// naming the path. A prunable entry is listed BEFORE the live one, so this also proves the
// prunable attribute does not leak forward and skip the live holder.
func TestAddStillRefusesLiveHolder(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	plantPrunableHolder(t, work, "ghost") // listed before the live holder below
	held := branchHeld(t, work, "collide")
	resetCalls(calls)

	rc, errout := runCapErr(t, []string{"add", "collide"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("add over a LIVE holder rc = %d, want 5; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "CHECKED OUT in the worktree") || !strings.Contains(errout, held) {
		t.Fatalf("refusal did not name the live holding worktree %s:\n%s", held, errout)
	}
	if out := mustGit(t, work, "rev-parse", "--verify", "--quiet", "refs/heads/collide"); out == "" {
		t.Error("a refusal deleted the live-held branch")
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Errorf("git worktree add ran despite a live holder: %v", gitCalls(*calls))
	}
}

// Verify row 4 — the NEGATIVE control. A prunable entry holds a branch that is 1 commit AHEAD
// of its upstream: the prunable reading removes the HOLDER objection, but the 0-ahead proof
// must still refuse (5) naming the count, and the branch must survive. A "prunable ⇒ reclaim"
// implementation that dropped the proof along with the holder would delete unfinished work.
func TestAddStillRefusesAheadBranchEvenWhenPrunable(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	dead := filepath.Join(t.TempDir(), "dead-collide")
	mustGit(t, work, "worktree", "add", "-b", "collide", dead, "refs/remotes/origin/main")
	writeFile(t, filepath.Join(dead, "wip.txt"), "unpushed work\n")
	mustGit(t, dead, "add", "wip.txt")
	mustGit(t, dead, "commit", "-m", "unpushed work")
	if err := os.RemoveAll(dead); err != nil { // directory gone → the entry is now prunable
		t.Fatalf("rm -rf: %v", err)
	}
	resetCalls(calls)

	rc, errout := runCapErr(t, []string{"add", "collide"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("add over an AHEAD prunable-held branch rc = %d, want 5; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "1 commit(s) not in") || !strings.Contains(errout, "unfinished work") {
		t.Fatalf("refusal did not name the ahead count — the prunable read must remove only the "+
			"holder objection, never the 0-ahead proof:\n%s", errout)
	}
	if out := mustGit(t, work, "rev-parse", "--verify", "--quiet", "refs/heads/collide"); out == "" {
		t.Error("a refusal deleted the ahead branch")
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Errorf("git worktree add ran despite unfinished work: %v", gitCalls(*calls))
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
