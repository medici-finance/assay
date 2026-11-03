package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitcore"
)

// Count real work, not elapsed time: every candidate must be judged while the
// mainline is walked once and a repeated non-mainline proof is computed once.
// regression: #3187
func TestSweepReapReusesAncestry(t *testing.T) {
	work, roots := newPerfFixture(t, 40, 500)
	side := mustGit(t, work, "rev-parse", "refs/heads/side")
	behind := mustGit(t, work, "rev-parse", "refs/remotes/origin/main~1")
	mustGit(t, work, "update-ref", "refs/remotes/origin/retained", behind)
	for i, rt := range roots {
		branch := fmt.Sprintf("reuse-%d", i)
		mustGit(t, work, "update-ref", "refs/heads/"+branch, side)
		writeFile(t, filepath.Join(work, ".git", "worktrees", filepath.Base(rt), "HEAD"), "ref: refs/heads/"+branch+"\n")
		mustGit(t, work, "config", "branch."+branch+".remote", "origin")
		upstream := "main"
		if i%2 == 1 {
			upstream = "retained"
		}
		mustGit(t, work, "config", "branch."+branch+".merge", "refs/heads/"+upstream)
	}
	walks, queries := 0, 0
	oldLog, oldAncestor := logOriginMain, sweepIsAncestor
	logOriginMain = func(sc *sweepCtx, dir string) ([]string, error) { walks++; return oldLog(sc, dir) }
	var reader *gitcore.Repo
	sweepIsAncestor = func(repo *gitcore.Repo, a, d string) (bool, error) {
		queries++
		if reader != nil && reader != repo {
			t.Fatal("candidates rebuilt their common-store ancestry reader")
		}
		reader = repo
		return oldAncestor(repo, a, d)
	}
	t.Cleanup(func() { logOriginMain, sweepIsAncestor = oldLog, oldAncestor })
	sc := newSweepCtx(work)
	for _, rt := range roots {
		pushed, _, err := pushedState(sc, rt, false)
		if err != nil || pushed {
			t.Fatalf("unpushed candidate: pushed=%v err=%v", pushed, err)
		}
	}
	if walks != 1 || queries != 1 {
		t.Fatalf("40 candidates: mainline walks=%d non-mainline queries=%d, want 1 each", walks, queries)
	}
	// The same ref name moves. Reusing its old negative answer would be wrong.
	mustGit(t, work, "update-ref", "refs/remotes/origin/retained", side)
	if pushed, _, err := pushedState(sc, roots[1], false); err != nil || !pushed {
		t.Fatalf("moved upstream not observed: pushed=%v err=%v", pushed, err)
	}
	if queries != 2 {
		t.Fatalf("moved upstream used %d queries, want 2", queries)
	}
	mustGit(t, work, "update-ref", "refs/remotes/origin/retained", behind)
	if pushed, _, err := pushedState(sc, roots[1], false); err != nil || pushed {
		t.Fatalf("rewound upstream reused a stale positive answer: pushed=%v err=%v", pushed, err)
	}
	if queries != 2 {
		t.Fatal("immutable negative answer was recomputed")
	}
}

// regression: #3187
func TestSweepDetachedReviewsShareReachability(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	first, _ := openPRWorktree(t, work, "shared-review-first")
	head := mustGit(t, first, "rev-parse", "HEAD")
	mustGit(t, first, "checkout", "--detach", head)
	second := addWorktree(t, "shared-review-second")
	mustGit(t, second, "checkout", "--detach", head)
	old := sweepRefsContaining
	var reader *gitcore.Repo
	calls := 0
	sweepRefsContaining = func(repo *gitcore.Repo, head string) ([]string, error) {
		calls++
		if reader != nil && reader != repo {
			t.Fatal("detached candidates used different reachability indexes")
		}
		reader = repo
		return old(repo, head)
	}
	t.Cleanup(func() { sweepRefsContaining = old })
	sc := newSweepCtx(work)
	for _, rt := range []string{first, second} {
		if pushed, _, err := pushedState(sc, rt, true); err != nil || !pushed {
			t.Fatalf("retained review: pushed=%v err=%v", pushed, err)
		}
	}
	if calls != 2 || reader != sc.repos[work] {
		t.Fatal("both reviews must use the sweep's shared history handle")
	}
	// Ref names are NOT cached with the immutable graph nodes.
	refs := mustGit(t, work, "for-each-ref", "--format=%(refname)", "refs/remotes/")
	for _, ref := range strings.Fields(refs) {
		mustGit(t, work, "update-ref", "-d", ref)
	}
	if pushed, _, err := pushedState(sc, first, true); err != nil || pushed {
		t.Fatalf("deleted remote refs still authorized retention: pushed=%v err=%v", pushed, err)
	}
}

// regression: #3187
func TestSweepRejectsChangedCommonStore(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	rt := addWorktree(t, "changed-store")
	sc := newSweepCtx(work)
	if _, err := sc.open(rt); err != nil {
		t.Fatal(err)
	}
	foreign := newRepo(t)
	writeFile(t, filepath.Join(rt, ".git"), "gitdir: "+filepath.Join(foreign, ".git")+"\n")
	if _, err := sc.open(rt); err == nil {
		t.Fatal("a cached candidate handle survived redirection to another common store")
	}
}

// regression: #3187
func TestSweepReapHoldsUnreadableHead(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	rt := addWorktree(t, "unreadable-head")
	sc := newSweepCtx(work)
	branch := headBranch(t, rt)
	writeFile(t, filepath.Join(work, ".git", "refs", "heads", branch), strings.Repeat("f", 40)+"\n")
	if pushed, _, err := pushedState(sc, rt, false); pushed || err == nil {
		t.Fatalf("missing HEAD object must be uncertainty: pushed=%v err=%v", pushed, err)
	}
	assertExists(t, rt)
}

// regression: #3187
func TestSweepAdminInventoryRefreshesBetweenSweeps(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	rt := addWorktree(t, "registry-refresh")
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(work, ".git", "worktrees", "unreadable", "gitdir")
	if err := os.MkdirAll(bad, 0755); err != nil {
		t.Fatal(err)
	}
	first := newSweepCtx(work)
	if _, err := first.adminDirFor(guard, rt); err == nil {
		t.Fatal("partial inventory accepted")
	}
	if err := os.RemoveAll(filepath.Dir(bad)); err != nil {
		t.Fatal(err)
	}
	second := newSweepCtx(work)
	if _, err := second.adminDirFor(guard, rt); err != nil {
		t.Fatalf("next sweep retained an obsolete error: %v", err)
	}
}

// regression: #3187
func TestSweepAdminInventoryScalesAcrossHusksAndAtTipTrees(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	head := mustGit(t, work, "rev-parse", "HEAD")
	roots := fabricateWorktrees(t, work, 80, head, head)
	old := time.Now().Add(-2 * freshWorktreeSafetyWindow)
	for i, rt := range roots {
		admin := filepath.Join(work, ".git", "worktrees", filepath.Base(rt))
		if i%2 == 0 {
			if err := os.Remove(filepath.Join(rt, ".git")); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(admin, old, old); err != nil {
			t.Fatal(err)
		}
	}
	assertGitSeesWorktrees(t, work, roots)
	reads, pointers := 0, 0
	oldDir, oldPointer := readAdminRegistry, readAdminPointer
	readAdminRegistry = func(path string) ([]os.DirEntry, error) { reads++; return oldDir(path) }
	readAdminPointer = func(path string) ([]byte, error) { pointers++; return oldPointer(path) }
	t.Cleanup(func() { readAdminRegistry, readAdminPointer = oldDir, oldPointer })
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	res, err := pruneSweep(guard, work, resolvePath(work), pruneOpts{dryRun: true, noBranches: true, noCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if reads != 1 || pointers > 2*len(roots) || pointers < len(roots) {
		t.Fatalf("80 candidates: registry reads=%d pointers=%d, want 1 registry and at most 160 pointers", reads, pointers)
	}
	if len(res.staleRegs) != len(roots)/2 {
		t.Fatalf("classified %d stale registrations, want %d", len(res.staleRegs), len(roots)/2)
	}
	for _, rt := range roots {
		assertExists(t, rt)
	}
}

// regression: #3187
func TestSweepAdminIdentityFailsClosed(t *testing.T) {
	for _, bad := range []string{"duplicate", "unreadable", "malformed", "moved", "changed-malformed", "removed"} {
		t.Run(bad, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			rt := addWorktree(t, "identity")
			guard, err := newPathGuard(work)
			if err != nil {
				t.Fatal(err)
			}
			sc := newSweepCtx(work)
			if bad == "moved" || bad == "changed-malformed" || bad == "removed" {
				if _, err := sc.adminDirFor(guard, rt); err != nil {
					t.Fatal(err)
				}
				pointer := filepath.Join(work, ".git", "worktrees", filepath.Base(rt), "gitdir")
				switch bad {
				case "moved":
					writeFile(t, pointer, filepath.Join(work, "different", ".git"))
				case "changed-malformed":
					writeFile(t, pointer, filepath.Join(rt, "not-git"))
				case "removed":
					if err := os.Remove(pointer); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				other := filepath.Join(work, ".git", "worktrees", "another")
				if err := os.MkdirAll(other, 0755); err != nil {
					t.Fatal(err)
				}
				switch bad {
				case "duplicate":
					writeFile(t, filepath.Join(other, "gitdir"), filepath.Join(rt, ".git"))
				case "unreadable":
					if err := os.Mkdir(filepath.Join(other, "gitdir"), 0755); err != nil {
						t.Fatal(err)
					}
				case "malformed":
					writeFile(t, filepath.Join(other, "gitdir"), "not-a-git-pointer")
				}
			}
			if _, err := sc.adminDirFor(guard, rt); err == nil {
				t.Fatal("uncertain admin identity was accepted")
			}
			assertExists(t, rt)
		})
	}
}
