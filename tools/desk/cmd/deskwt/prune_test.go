package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// runCapOut runs `run(args)` capturing STDOUT (the interval/tick summary channel).
func runCapOut(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

// addWorktree runs `deskwt add <name>` and returns the created worktree path. A freshly
// added worktree is on a NEW branch tracking origin/main AT origin/main's commit, so it is
// tracked-clean AND merged (HEAD is an ancestor of origin/main) — the removable state.
func addWorktree(t *testing.T, name string) string {
	t.Helper()
	if rc := run([]string{"add", name}); rc != deskkit.ExitOK {
		t.Fatalf("add %q rc = %d, want 0", name, rc)
	}
	return filepath.Join(tmpBaseDir, "tracker-"+name)
}

// --- prune: merged+clean worktree is removed --------------------------------------

func TestPruneRemovesMergedCleanWorktree(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target := addWorktree(t, "merged")

	// Advance origin/main one commit so the fresh worktree's HEAD is no longer at
	// the tip — the fresh-worktree guard (headAtOriginMainTip) skips worktrees whose
	// HEAD == origin/main exactly, but this worktree should still be removed because
	// it landed no work. A real prune sweep only removes genuinely-stale worktrees
	// after the mainline has moved past them.
	writeFile(t, filepath.Join(work, "mainline.txt"), "marched forward\n")
	mustGit(t, work, "add", "mainline.txt")
	mustGit(t, work, "commit", "-m", "advance mainline")
	newMain := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", newMain)

	resetCalls(calls)
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("merged+clean worktree %s still exists after prune (err=%v)", target, err)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("worktree still registered after prune:\n%s", list)
	}
	if !strings.Contains(errout, "removed 1 merged+clean") {
		t.Fatalf("expected summary to report 1 removal; got:\n%s", errout)
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag during prune: %v", gitCalls(*calls))
	}
}

// --- prune: dirty tracked tree is skipped -----------------------------------------

func TestPruneSkipsDirtyTracked(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "dirty")
	// Modify an already-tracked file (unstaged) — a dirty tracked change.
	writeFile(t, filepath.Join(target, "README.md"), "seed\nmodified\n")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target) // MUST NOT have been removed
	if !strings.Contains(errout, "dirty") {
		t.Fatalf("expected a dirty skip reason; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed a dirty worktree; stderr:\n%s", errout)
	}
}

// --- prune: fresh-at-main worktree with untracked new file is LEFT --------------
// The core fix for the automatic-sweep untracked-work hole: a worktree whose HEAD is
// exactly origin/main (zero landed commits) may hold uncommitted new source files
// (untracked, never git-add'ed). The tracked-clean gate deliberately ignores untracked
// files (--untracked-files=no) so build artifacts don't block, but in prune's automatic
// sweep this creates a collision: a fresh worktree with new .go files is "clean AND
// merged" and would be wrongly removed. The fresh-worktree guard (headAtOriginMainTip)
// skips it. This test covers the exact scenario the reviewer flagged.
func TestPruneSkipsFreshAtMainWorktreeWithUntrackedFile(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "fresh")
	// A new source file, untracked (never git-add'ed) — a worker authoring new code
	// before their first commit. This is NOT a build artifact; it's real uncommitted work.
	writeFile(t, filepath.Join(target, "newmodule.go"), "package main\n")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target) // MUST NOT have been removed
	if !strings.Contains(errout, "fresh worktree at origin/main") {
		t.Fatalf("expected fresh-at-main skip reason; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed a fresh-at-main worktree with untracked new file; stderr:\n%s", errout)
	}
}

// --- prune: unpushed commit is skipped --------------------------------------------

func TestPruneSkipsUnpushedCommit(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "ahead")
	// A commit ahead of the upstream (origin/main) — unpushed AND not merged.
	writeFile(t, filepath.Join(target, "new.txt"), "new\n")
	mustGit(t, target, "add", "new.txt")
	mustGit(t, target, "commit", "-m", "unpushed work")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "unpushed") {
		t.Fatalf("expected an unpushed skip reason; got:\n%s", errout)
	}
}

// --- prune: clean but UNMERGED branch is skipped (active-worker protection) --------
// This is the most important case: an open PR in flight (commits pushed to the branch's
// own upstream but NOT yet merged to origin/main) must be LEFT untouched.
func TestPruneSkipsCleanUnmergedBranch(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "feature")
	// Commit work on the branch, then simulate it being PUSHED to its OWN upstream
	// (origin/feature) — so it is NOT "unpushed" — while still NOT merged to origin/main.
	writeFile(t, filepath.Join(target, "feat.txt"), "feature work\n")
	mustGit(t, target, "add", "feat.txt")
	mustGit(t, target, "commit", "-m", "feature work (open PR, unmerged)")
	featSHA := mustGit(t, target, "rev-parse", "HEAD")
	// Publish the branch to a remote-tracking ref and point the worktree's upstream at it.
	mustGit(t, work, "update-ref", "refs/remotes/origin/feature", featSHA)
	mustGit(t, target, "branch", "--set-upstream-to=origin/feature")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target) // active work MUST survive
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed an UNMERGED (active) worktree; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "unmerged") {
		t.Fatalf("expected an unmerged skip reason; got:\n%s", errout)
	}
	// It is clean and pushed to its own upstream, so it must NOT be labelled unpushed.
	if strings.Contains(errout, "unpushed") {
		t.Fatalf("clean pushed-but-unmerged worktree mislabelled unpushed; got:\n%s", errout)
	}
}

// --- prune: the current worktree is never removed ---------------------------------

func TestPruneSkipsCurrentWorktree(t *testing.T) {
	work := newRepo(t)
	// Add a worktree and make the tool RUN FROM it (cwd == that worktree). It is merged
	// and clean, so ONLY the current-worktree guard can keep it.
	withEnv(t, work)
	target := addWorktree(t, "self")
	// Re-point getwd at the worktree itself so it is the current dir.
	getwd = func() (string, error) { return target, nil }

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "current worktree") {
		t.Fatalf("expected a current-worktree skip; got:\n%s", errout)
	}
}

// --- prune: the shared checkout is refused by identity, never removed -------------

func TestPruneRefusesSharedCheckout(t *testing.T) {
	work := newRepo(t)
	mustGit(t, work, "branch", "--set-upstream-to=origin/main", "main")
	calls := withEnv(t, work)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, work) // shared checkout MUST survive
	if !strings.Contains(errout, "shared checkout") {
		t.Fatalf("expected the shared checkout to be listed as an identity skip; got:\n%s", errout)
	}
	// No worktree-remove path may run against the shared checkout.
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if !strings.Contains(list, work) {
		t.Fatalf("shared checkout no longer registered after prune:\n%s", list)
	}
	_ = calls
}

// --- prune: flag parsing --------------------------------------------------------

func TestPruneBadIntervalRefuses(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	for _, bad := range []string{"nope", "-5m", "0"} {
		if rc := run([]string{"prune", "--interval", bad}); rc != deskkit.ExitRefused {
			t.Fatalf("prune --interval %q rc = %d, want 5", bad, rc)
		}
	}
}

func TestPrunePositionalRefuses(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	if rc := run([]string{"prune", "stray"}); rc != deskkit.ExitRefused {
		t.Fatalf("prune with a positional arg rc = %d, want 5", rc)
	}
}

// --- prune: interval mode ticks then exits on stop --------------------------------

// TestPruneIntervalSingleTickThenStop drives runPruneLoop directly with a large interval
// and a pre-closed stop channel: the loop sweeps EXACTLY once (removing the merged+clean
// worktree), prints one tick line to stdout, then the stop wins the select → clean exit 0.
// No real sleep occurs (the ticker never fires within the test).
func TestPruneIntervalSingleTickThenStop(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "tick")

	// Advance origin/main so the fresh worktree is no longer at the tip (the
	// fresh-worktree guard blocks removal when HEAD == origin/main exactly).
	writeFile(t, filepath.Join(work, "mainline.txt"), "marched forward\n")
	mustGit(t, work, "add", "mainline.txt")
	mustGit(t, work, "commit", "-m", "advance mainline")
	newMain := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", newMain)

	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatalf("newPathGuard: %v", err)
	}
	cwd := resolvePath(mustAbsOrRaw(work))
	stop := make(chan struct{})
	close(stop) // stop is ready immediately; with a 1h interval the ticker never fires

	var loopErr error
	out := runCapOut(t, func() {
		loopErr = runPruneLoop(guard, work, cwd, time.Hour, stop, &auditCtx{verb: "prune"})
	})
	if loopErr != nil {
		t.Fatalf("runPruneLoop returned %v, want nil (clean stop)", loopErr)
	}
	if !strings.Contains(out, "prune tick 1:") {
		t.Fatalf("expected exactly one tick line on stdout; got:\n%s", out)
	}
	if strings.Contains(out, "tick 2:") {
		t.Fatalf("loop ticked more than once before stop; got:\n%s", out)
	}
	if _, serr := os.Stat(target); !os.IsNotExist(serr) {
		t.Fatalf("interval tick did not remove the merged+clean worktree %s", target)
	}
}

// TestPruneIntervalHaltsOnStopFlag proves the loop honors the deskkit STOP flag BETWEEN
// ticks: with STOP armed, Guard fires on the first iteration → the loop halts before any
// sweep, returning the exit-3 Disabled error (never a silent continue).
func TestPruneIntervalHaltsOnStopFlag(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "halted")

	// Arm the all-loops STOP flag under this test's fresh HOME.
	deskToolsDir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(deskToolsDir, 0o700); err != nil {
		t.Fatalf("mkdir desk-tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deskToolsDir, "STOP"), []byte("halt for test\n"), 0o600); err != nil {
		t.Fatalf("write STOP: %v", err)
	}

	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatalf("newPathGuard: %v", err)
	}
	cwd := resolvePath(mustAbsOrRaw(work))
	stop := make(chan struct{}) // never closed — only the STOP flag can halt the loop

	var loopErr error
	_ = runCapOut(t, func() {
		loopErr = runPruneLoop(guard, work, cwd, time.Hour, stop, &auditCtx{verb: "prune"})
	})
	if deskkit.ExitCodeOf(loopErr) != deskkit.ExitDisabled {
		t.Fatalf("runPruneLoop under STOP returned exit %d, want 3 (disabled)", deskkit.ExitCodeOf(loopErr))
	}
	assertExists(t, target) // halted before any sweep — worktree untouched
}
