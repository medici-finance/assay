package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// buildMainHistoryFastImport builds a linear chain of n+1 empty-tree commits on
// refs/heads/main directly inside the bare repository at remoteDir, via a single
// `git fast-import` stream. See the call site's comment (in the test below) for why this
// replaces n+1 individual `git commit --allow-empty` calls followed by one `git push`.
func buildMainHistoryFastImport(t *testing.T, remoteDir string, n int) {
	t.Helper()

	const epoch = 1700000000 // arbitrary fixed base; only ordering/monotonicity matters here

	var b strings.Builder
	writeCommit := func(mark int, subject string, hasParent bool) {
		fmt.Fprintf(&b, "commit refs/heads/main\n")
		fmt.Fprintf(&b, "mark :%d\n", mark)
		fmt.Fprintf(&b, "author seed <seed@test> %d +0000\n", epoch+mark)
		fmt.Fprintf(&b, "committer seed <seed@test> %d +0000\n", epoch+mark)
		fmt.Fprintf(&b, "data <<COMMIT_MSG_EOF\n%s\nCOMMIT_MSG_EOF\n", subject)
		if hasParent {
			fmt.Fprintf(&b, "from :%d\n", mark-1)
		}
	}
	writeCommit(1, "chore: initial commit on main", false)
	for i := 0; i < n; i++ {
		writeCommit(i+2, fmt.Sprintf("chore: main history commit %d", i), true)
	}
	b.WriteString("done\n")

	cmd := exec.Command("git", "fast-import", "--quiet", "--done")
	cmd.Dir = remoteDir
	cmd.Stdin = strings.NewReader(b.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git fast-import (dir=%s) failed: %v\n%s", remoteDir, err, out)
	}
}

// TestCheckRegisterIDCollisions_ManyDuplicateRemoteBranchesStaysBounded is the regression
// test for the duplicate-remote hang: deskpushguard pegged one CPU core indefinitely (never
// deciding) on a branch carrying a large main-catch-up merge, in a checkout with duplicate
// remotes pointing at the same upstream repo.
//
// Root cause (confirmed by reading gitcore.Repo.IsAncestor and go-git's
// object.Commit.IsAncestor, plumbing/object/merge_base.go @ v5.19.2): the old
// checkRegisterIDCollisions (registerid.go) — reached whenever the push touches a NEW id
// under a register directory (docs/streams/findings/, docs/streams/intake/) — looped over
// EVERY remote-tracking branch in the repository (remoteBranchNames, unfiltered by anything
// the pushed ref actually shares history with) and called
// gitcore.Repo.IsAncestor(b, originMain) for each one. That call resolves to go-git's native
// Commit.IsAncestor, which runs a fresh, UNMEMOIZED preorder walk of origin/main's ENTIRE
// history on every single invocation — no caching, no generation-number cut-off, nothing
// shared across calls (contrast gitcore's own RefsContaining, whose header comment in
// contains.go describes fixing exactly this class of bug for a different call site, or
// foreigncommit.go's sibling checkForeignCommits, fixed alongside this one).
//
// This loop's cost is therefore O(total remote branch count x origin/main's full history
// size), with NO dependency on how many commits the push itself is ahead by — a checkout
// with N literal-duplicate remotes for the same upstream (assay/pub/pubassay/public in the
// reported case) multiplies "total remote branch count" by N directly, and origin/main's
// history only grows over the life of a real repository (a 524-commit catch-up merge is one
// way it gets bigger, but the walk cost was already tied to main's CURRENT size, merge or
// not) — together this is what "pegged at 100-107% CPU, growing linearly, never deciding"
// looks like from outside, and it needs only ONE new register-directory id on the pushed
// branch to trigger, independent of the size of the push's own diff.
//
// This fixture reproduces the SHAPE, bounded: a few hundred commits on main (standing in
// for a real repository's accumulated history, of which the reported 524-commit catch-up
// merge is one contributor), a double-digit number of long-lived sibling branches that are
// genuinely NOT ancestors of main, each visible under several remote names (origin plus
// literal duplicates of its own URL, matching the reported `assay`/`pub`/`pubassay`/`public`
// shape), and a single new finding on the victim's own branch — the minimum that makes
// ownIDs non-empty and enters this loop at all.
//
// FAIL-FIRST: `git stash` this fix (restoring the per-branch gitcore.Repo.IsAncestor call in
// registerid.go, and branchIsAncestorOfMain in foreigncommit.go) and this test's wall-clock
// assertion goes red — see the PR's "## Fail-first" section for the observed timing on both
// sides of this repository's actual history size.
func TestCheckRegisterIDCollisions_ManyDuplicateRemoteBranchesStaysBounded(t *testing.T) {
	const (
		mainCommits  = 800 // stands in for a repository's accumulated origin/main history
		siblingCount = 20  // long-lived, never-merged sibling branches
		dupRemotes   = 4   // + origin = 5 remotes for the SAME upstream, matching #2527
	)

	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	// Build main's mainCommits+1-commit history directly in the bare remote via a single
	// `git fast-import` stream, rather than mainCommits+1 individual `git commit --allow-empty`
	// calls in a working tree followed by one `git push` of the whole history. The push-based
	// construction was observed to fail intermittently in CI ("remote unpack failed: eof
	// before pack header was fully read" / "Could not read <sha>") — a resource-pressure flake
	// from spawning ~1600 git subprocesses back to back and then transferring the resulting
	// ~800-object pack over the local push transport in one shot, immediately after. fast-import
	// writes the identical history (a linear chain of empty-tree commits) straight into the
	// bare repo's object store and ref in ONE process, with no push/unpack step to race —
	// faster, and immune to that class of transport flake.
	buildMainHistoryFastImport(t, remoteDir, mainCommits)

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	runGitT(t, seed, "fetch", "origin", "main")
	runGitT(t, seed, "checkout", "-B", "main", "origin/main")

	// Long-lived siblings, each genuinely NOT an ancestor of main (their own unique
	// commit), pushed to origin — never merged back, so an unmemoized ancestor walk
	// against ANY of them always runs to exhaustion of main's full history.
	for s := 0; s < siblingCount; s++ {
		branch := fmt.Sprintf("sibling-%d", s)
		runGitT(t, seed, "checkout", "-b", branch, "main")
		commitEmpty(t, seed, fmt.Sprintf("feat: %s commit", branch))
		runGitT(t, seed, "push", "origin", branch)
		runGitT(t, seed, "checkout", "main")
	}

	// Victim clone: dupRemotes EXTRA remotes pointing at the exact same URL as origin (the
	// literal "assay"/"pub"/"pubassay"/"public" shape from the issue), all fetched, so
	// every sibling branch above is visible under dupRemotes+1 distinct remote names —
	// multiplying the branch-scan loop's iteration count directly.
	victimDir := t.TempDir()
	runGitT(t, victimDir, "clone", remoteDir, ".")
	runGitT(t, victimDir, "config", "user.email", "victim@test")
	runGitT(t, victimDir, "config", "user.name", "victim")
	for d := 0; d < dupRemotes; d++ {
		name := fmt.Sprintf("dup%d", d)
		runGitT(t, victimDir, "remote", "add", name, remoteDir)
		runGitT(t, victimDir, "fetch", name)
	}

	// The ONLY thing that gets this push into the branch-scan loop at all: ONE new id
	// under a register directory that origin/main does not carry. No large diff needed —
	// the reported hang's dependency is on the REMOTE COUNT, not the push's own size.
	runGitT(t, victimDir, "checkout", "-b", "mine", "origin/main")
	regIDWriteFile(t, victimDir, "docs/streams/findings/2026-09-14-perf-fixture.md",
		"---\nid: F-2527-perf\ndate: \"2026-09-14\"\ntitle: \"perf fixture\"\n---\n\nBody.\n")
	runGitT(t, victimDir, "add", "docs/streams/findings/2026-09-14-perf-fixture.md")
	runGitT(t, victimDir, "commit", "-m", "docs(findings): add F-2527-perf")
	head := runGitT(t, victimDir, "rev-parse", "HEAD")

	start := time.Now()
	collisions, err := checkRegisterIDCollisions(victimDir, "mine", head)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("checkRegisterIDCollisions error: %v", err)
	}

	// The bound: comfortably above what the FIXED algorithm needs (one walk of main's
	// history plus O(1) lookups per branch — milliseconds), comfortably below what the OLD
	// algorithm needs (branch-count x unmemoized full-history walks, which at this
	// fixture's size already runs into multiple seconds — see the PR's Fail-first section).
	const bound = 5 * time.Second
	if elapsed > bound {
		t.Fatalf("checkRegisterIDCollisions took %s (> %s) scanning %d remote branches "+
			"(%d siblings x %d remote spellings, plus main's own %d copies) against a "+
			"%d-commit origin/main — duplicate-remote hang regression: the old per-branch "+
			"gitcore.Repo.IsAncestor call re-walks origin/main's whole history on every "+
			"call and does not finish in a bounded window as either dimension grows",
			elapsed, bound, siblingCount*(dupRemotes+1)+(dupRemotes+1), siblingCount, dupRemotes+1,
			dupRemotes+1, mainCommits+1)
	}
	t.Logf("checkRegisterIDCollisions: %s scanning %d remote branches against a %d-commit origin/main",
		elapsed, siblingCount*(dupRemotes+1)+(dupRemotes+1), mainCommits+1)

	if len(collisions) != 0 {
		t.Errorf("expected no id collisions (no sibling touches a register directory), got %+v", collisions)
	}
}
