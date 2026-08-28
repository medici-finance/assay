package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// chdir changes the process's working directory to dir for the duration of the test,
// returning a func that restores it. Safe here because this package's tests do not run
// t.Parallel().
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%s): %v", dir, err)
	}
	return func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore os.Chdir(%s): %v", old, err)
		}
	}
}

// runGitT runs git in dir and fails the test on error, returning trimmed stdout.
func runGitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (dir=%s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitEmpty makes an empty commit with the given subject and returns its full sha.
func commitEmpty(t *testing.T, dir, subject string) string {
	t.Helper()
	runGitT(t, dir, "commit", "--allow-empty", "-m", subject)
	return runGitT(t, dir, "rev-parse", "HEAD")
}

// newForeignCommitFixture builds a real three-repo scenario:
//
//   - remote.git (bare): the shared origin.
//   - a "sibling" branch pushed to origin with its own commits (an in-flight PR).
//   - a "victim" clone that reproduces the #22 bug: it forks its own branch off the
//     sibling's tip instead of origin/main, so the sibling's commits ride along, then adds
//     one genuine commit of its own.
//
// It returns the victim clone's path, the victim's own branch name, and the sha of the
// victim's own (non-foreign) commit.
func newForeignCommitFixture(t *testing.T) (victimDir, ownBranch, ownSHA string) {
	t.Helper()
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	// Sibling branch, pushed to origin — simulates PR #212, still open (not merged to main).
	runGitT(t, seed, "checkout", "-b", "sibling")
	commitEmpty(t, seed, "feat: sibling commit B")
	commitEmpty(t, seed, "feat: sibling commit C")
	runGitT(t, seed, "push", "origin", "sibling")

	// Victim clone: fetch everything, then reproduce the bug by branching off the
	// sibling's tip (as `git worktree add -b mine` off a HEAD sitting on sibling would)
	// instead of origin/main.
	victimDir = t.TempDir()
	runGitT(t, victimDir, "clone", remoteDir, ".")
	runGitT(t, victimDir, "config", "user.email", "victim@test")
	runGitT(t, victimDir, "config", "user.name", "victim")
	runGitT(t, victimDir, "checkout", "sibling")
	runGitT(t, victimDir, "checkout", "-b", "mine")
	ownSHA = commitEmpty(t, victimDir, "feat: my own genuine commit D")
	return victimDir, "mine", ownSHA
}

func TestCheckForeignCommits_DetectsLaunderedSiblingCommits(t *testing.T) {
	victimDir, ownBranch, ownSHA := newForeignCommitFixture(t)

	found, err := checkForeignCommits(victimDir, ownBranch, ownSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.masquerades) != 0 {
		t.Errorf("expected no merge masquerades, got %v", found.masquerades)
	}
	if len(found.foreign) != 2 {
		t.Fatalf("expected 2 foreign commits (sibling's B and C), got %d: %+v", len(found.foreign), found.foreign)
	}
	for _, f := range found.foreign {
		if f.sourceBranch != "origin/sibling" {
			t.Errorf("foreign commit %s: sourceBranch = %q, want origin/sibling", shortSHA(f.sha), f.sourceBranch)
		}
		if !strings.Contains(f.subject, "sibling commit") {
			t.Errorf("foreign commit %s: subject = %q, want a sibling-commit subject", shortSHA(f.sha), f.subject)
		}
	}
}

func TestCheckForeignCommits_CleanBranchFromOriginMainReportsNothing(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	// A sibling branch exists on origin (an unrelated open PR)...
	runGitT(t, seed, "checkout", "-b", "sibling")
	commitEmpty(t, seed, "feat: sibling commit B")
	runGitT(t, seed, "push", "origin", "sibling")

	// ...but this worker correctly cuts fresh from origin/main (the mandated recipe).
	goodDir := t.TempDir()
	runGitT(t, goodDir, "clone", remoteDir, ".")
	runGitT(t, goodDir, "config", "user.email", "good@test")
	runGitT(t, goodDir, "config", "user.name", "good")
	runGitT(t, goodDir, "checkout", "-b", "good-mine", "origin/main")
	ownSHA := commitEmpty(t, goodDir, "feat: my own genuine commit E")

	found, err := checkForeignCommits(goodDir, "good-mine", ownSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.foreign) != 0 {
		t.Errorf("expected 0 foreign commits on a clean origin/main-based branch, got %+v", found.foreign)
	}
	if len(found.masquerades) != 0 {
		t.Errorf("expected 0 merge masquerades, got %+v", found.masquerades)
	}
}

func TestCheckForeignCommits_DetectsSingleParentMergeMasquerade(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	dir := t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "checkout", "-b", "fake-merge", "origin/main")
	// tracker#259's failure shape: a single-parent commit whose SUBJECT claims to be a merge.
	fakeSHA := commitEmpty(t, dir, "merge: rebase onto origin/main")

	found, err := checkForeignCommits(dir, "fake-merge", fakeSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.foreign) != 0 {
		t.Errorf("expected 0 foreign commits, got %+v", found.foreign)
	}
	if len(found.masquerades) != 1 {
		t.Fatalf("expected 1 merge masquerade, got %d: %+v", len(found.masquerades), found.masquerades)
	}
	if found.masquerades[0].sha != fakeSHA {
		t.Errorf("masquerade sha = %s, want %s", found.masquerades[0].sha, fakeSHA)
	}
}

func TestCheckForeignCommits_OrdinaryMergeMentionNotFlagged(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	dir := t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "checkout", "-b", "ordinary-work", "origin/main")
	// Single-parent commits that merely MENTION "merge" mid-subject — ordinary work
	// touching merge logic, not a commit claiming to itself BE a merge. None of these
	// should be refused (this PR's own review found the prior any-position regex flagged
	// exactly this shape, including its own fix commit).
	commitEmpty(t, dir, "resolve merge conflict in helper")
	commitEmpty(t, dir, "describe the merge workflow in README")
	headSHA := commitEmpty(t, dir, "simplify merge-base comparison")

	found, err := checkForeignCommits(dir, "ordinary-work", headSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.foreign) != 0 {
		t.Errorf("expected 0 foreign commits, got %+v", found.foreign)
	}
	if len(found.masquerades) != 0 {
		t.Errorf("expected ordinary merge-mentioning commits NOT to be flagged as masquerades, got %+v", found.masquerades)
	}
}

func TestCheckForeignCommits_RealMergeCommitNotFlagged(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	dir := t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "checkout", "-b", "real-merge", "origin/main")
	commitEmpty(t, dir, "feat: my own commit")
	// Advance a second local branch, then a REAL two-parent merge back into real-merge.
	runGitT(t, dir, "checkout", "-b", "topic")
	commitEmpty(t, dir, "feat: topic commit")
	runGitT(t, dir, "checkout", "real-merge")
	runGitT(t, dir, "merge", "--no-ff", "-m", "merge: bring in topic", "topic")
	headSHA := runGitT(t, dir, "rev-parse", "HEAD")

	found, err := checkForeignCommits(dir, "real-merge", headSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.foreign) != 0 {
		t.Errorf("expected 0 foreign commits, got %+v", found.foreign)
	}
	if len(found.masquerades) != 0 {
		t.Errorf("expected the real two-parent merge NOT to be flagged as a masquerade, got %+v", found.masquerades)
	}
}

// --- Fails-CLOSED reporting: "could not check" must never read as "base is fine" ----------
//
// Each of the three tests below asserts BOTH halves of the contract: the push is still
// allowed (fail-open, brief-10 — no findings manufactured out of ignorance) AND a
// could-not-check reason is recorded. Before this, all three returned an empty result that
// was byte-identical to a clean pass.

func TestCheckForeignCommits_FailsOpenOnNonSHA(t *testing.T) {
	// Existing deskpushguard tests feed placeholder stdin values ("x", "y") as the local
	// sha; checkForeignCommits must fail open (no findings, no error, no git invocation
	// against whatever happens to be the process's cwd) rather than error out.
	found, err := checkForeignCommits("", "test-branch", "x")
	if err != nil {
		t.Fatalf("expected nil error on a non-sha placeholder, got %v", err)
	}
	if len(found.foreign) != 0 || len(found.masquerades) != 0 || len(found.strayBases) != 0 {
		t.Errorf("expected no findings on a non-sha placeholder, got %+v", found)
	}
	if len(found.indeterminate) == 0 {
		t.Fatal("a non-sha local id means the base was never checked — that must be REPORTED " +
			"as could-not-check, not returned as an empty (clean-looking) result")
	}
	if !strings.Contains(found.indeterminate[0], "not a well-formed object id") {
		t.Errorf("could-not-check reason = %q, want it to name the malformed object id", found.indeterminate[0])
	}
}

func TestCheckForeignCommits_FailsOpenWhenOriginMainUnresolvable(t *testing.T) {
	dir := t.TempDir()
	runGitT(t, dir, "init", "-b", "main")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	sha := commitEmpty(t, dir, "chore: no origin remote configured at all")

	found, err := checkForeignCommits(dir, "main", sha)
	if err != nil {
		t.Fatalf("expected nil error when origin/main is unresolvable, got %v", err)
	}
	if len(found.foreign) != 0 || len(found.masquerades) != 0 || len(found.strayBases) != 0 {
		t.Errorf("expected no findings when origin/main can't resolve, got %+v", found)
	}
	if len(found.indeterminate) == 0 {
		t.Fatal("no origin remote means the base could not be determined — that must be " +
			"REPORTED as could-not-check, not returned as an empty (clean-looking) result")
	}
	if !strings.Contains(found.indeterminate[0], "refs/remotes/origin/main does not resolve") {
		t.Errorf("could-not-check reason = %q, want it to name the unresolvable base", found.indeterminate[0])
	}
}

// TestCheckForeignCommits_CouldNotCheckWhenOnlyTheStrayResolves is the sharpest fails-closed
// case, and the one a naive implementation gets WRONG rather than merely absent: the repo has
// a stray local `refs/heads/origin/main` but has never fetched, so `refs/remotes/origin/main`
// does not exist. A check that fell back to the bare spelling would resolve `origin/main`
// successfully — to the stray — and then confidently report a clean base computed against
// entirely the wrong ref. The contract is could-not-check, and the reason must say the stray
// was seen and deliberately not used.
func TestCheckForeignCommits_CouldNotCheckWhenOnlyTheStrayResolves(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	dir := t.TempDir()
	runGitT(t, dir, "init", "-b", "main")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "remote", "add", "origin", remoteDir) // remote configured but NEVER fetched
	commitEmpty(t, dir, "chore: initial")
	runGitT(t, dir, "branch", "origin/main", "HEAD") // the stray, and the ONLY thing named origin/main
	sha := commitEmpty(t, dir, "feat: work in an unfetched repo")

	// Fixture guard: prove the trap is really armed — bare resolves, qualified does not.
	if bare := runGitT(t, dir, "rev-parse", "--verify", "--quiet", "origin/main"); bare == "" {
		t.Fatal("fixture: bare origin/main should resolve (to the stray)")
	}
	if err := exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet",
		"refs/remotes/origin/main").Run(); err == nil {
		t.Fatal("fixture: refs/remotes/origin/main must NOT resolve in this repo")
	}

	found, err := checkForeignCommits(dir, "main", sha)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.foreign) != 0 || len(found.masquerades) != 0 || len(found.strayBases) != 0 {
		t.Errorf("expected no findings when the true base cannot resolve, got %+v", found)
	}
	if len(found.indeterminate) == 0 {
		t.Fatal("the true base does not resolve and only the stray does — that is could-not-check, " +
			"never a silent pass")
	}
	if !strings.Contains(found.indeterminate[0], "stray local branch refs/heads/origin/main") {
		t.Errorf("could-not-check reason = %q, want it to name the stray it refused to fall back to",
			found.indeterminate[0])
	}
}

// --- Stray-base cut: the failure actually happening in this checkout ----------------------

// newStrayBaseFixture builds a clone carrying a stale stray `refs/heads/origin/main`, and cuts
// a worktree from it using the EXACT recipe the worker-desk skill prescribes:
//
//	git worktree add <path> origin/main --detach
//
// With the stray present, that recipe resolves through refs/heads/ (rev-parse precedence puts
// it ahead of refs/remotes/) and checks out the STALE commit, printing only
// `warning: refname 'origin/main' is ambiguous.` — measured, not assumed: the fixture asserts
// the resulting HEAD is the stale sha before the test proceeds.
//
// Returns the worktree path, its branch name, its head sha, the stale (stray) sha and the true
// base sha.
func newStrayBaseFixture(t *testing.T) (wtDir, branch, headSHA, staleSHA, trueSHA string) {
	t.Helper()
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")
	staleSHA = runGitT(t, seed, "rev-parse", "HEAD")
	for _, s := range []string{"feat: main moves on 1", "feat: main moves on 2", "feat: main moves on 3"} {
		commitEmpty(t, seed, s)
	}
	runGitT(t, seed, "push", "origin", "main")
	trueSHA = runGitT(t, seed, "rev-parse", "HEAD")

	clone := t.TempDir()
	runGitT(t, clone, "clone", remoteDir, ".")
	runGitT(t, clone, "config", "user.email", "w@test")
	runGitT(t, clone, "config", "user.name", "w")
	runGitT(t, clone, "branch", "origin/main", staleSHA) // plant the stray

	wtDir = t.TempDir()
	// git worktree add refuses a non-empty dir; use a child path it creates itself.
	wtDir = wtDir + "/wt"
	runGitT(t, clone, "worktree", "add", "--detach", wtDir, "origin/main")

	if got := runGitT(t, wtDir, "rev-parse", "HEAD"); got != staleSHA {
		t.Fatalf("fixture: the skill's own `worktree add <path> origin/main --detach` produced "+
			"HEAD=%s, expected the STALE stray tip %s — the bug this test exists for did not reproduce",
			got, staleSHA)
	}
	runGitT(t, wtDir, "checkout", "-b", "mine")
	headSHA = commitEmpty(t, wtDir, "feat: my own work, on a stale base")
	return wtDir, "mine", headSHA, staleSHA, trueSHA
}

// TestCheckForeignCommits_DetectsStrayLocalOriginMainBase is the fires-on-the-live-failure
// case. The branch drags in NO foreign commit (nothing was laundered — the stray is a plain
// ancestor of main), so the foreign-commit arm sees an empty range of suspects and the
// masquerade arm sees an ordinary subject. Only the stray-base arm can catch it.
//
// FAIL-FIRST: delete the checkStrayBase call from checkForeignCommits and this goes red with
// 0 findings while every other test in the file stays green.
func TestCheckForeignCommits_DetectsStrayLocalOriginMainBase(t *testing.T) {
	wtDir, branch, headSHA, staleSHA, trueSHA := newStrayBaseFixture(t)

	found, err := checkForeignCommits(wtDir, branch, headSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.indeterminate) != 0 {
		t.Fatalf("base was determinable here; unexpected could-not-check: %v", found.indeterminate)
	}
	if len(found.foreign) != 0 {
		t.Errorf("a stray-base cut launders nothing — expected 0 foreign commits, got %+v", found.foreign)
	}
	if len(found.strayBases) != 1 {
		t.Fatalf("expected exactly 1 stray-base finding for a worktree cut from the stray local "+
			"refs/heads/origin/main, got %d: %+v", len(found.strayBases), found.strayBases)
	}
	got := found.strayBases[0]
	if got.strayTip != staleSHA {
		t.Errorf("strayTip = %s, want the stale stray tip %s", got.strayTip, staleSHA)
	}
	if got.trueBase != trueSHA {
		t.Errorf("trueBase = %s, want the remote-tracking head %s", got.trueBase, trueSHA)
	}
	if got.behind != 3 {
		t.Errorf("behind = %d, want 3 (the commits main gained after the stray went stale)", got.behind)
	}
}

// TestCheckForeignCommits_StrayPresentButCutFromTrueBaseNotFlagged is the no-cry-wolf control
// for the arm above: the SAME hazardous repo (stray branch planted), but the worker cut
// correctly from refs/remotes/origin/main. Merely having a stray branch must not refuse a
// push.
func TestCheckForeignCommits_StrayPresentButCutFromTrueBaseNotFlagged(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")
	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")
	stale := runGitT(t, seed, "rev-parse", "HEAD")
	commitEmpty(t, seed, "feat: main moves on")
	runGitT(t, seed, "push", "origin", "main")

	dir := t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "branch", "origin/main", stale) // the stray IS present
	runGitT(t, dir, "checkout", "-b", "mine", "refs/remotes/origin/main")
	head := commitEmpty(t, dir, "feat: correctly based work")

	found, err := checkForeignCommits(dir, "mine", head)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.strayBases) != 0 {
		t.Errorf("a branch cut from the TRUE base must not be flagged just because a stray "+
			"refs/heads/origin/main exists in the repo; got %+v", found.strayBases)
	}
	if len(found.foreign) != 0 || len(found.masquerades) != 0 {
		t.Errorf("expected no other findings, got %+v", found)
	}
}

// TestCheckForeignCommits_OrdinaryBranchBehindMainNotFlagged is the second no-cry-wolf
// control, and the one that keeps the stray-base arm from degenerating into "refuse anything
// behind main". Every long-lived PR branch is behind main; none of them may be refused.
func TestCheckForeignCommits_OrdinaryBranchBehindMainNotFlagged(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")
	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")
	oldMain := runGitT(t, seed, "rev-parse", "HEAD")
	for _, s := range []string{"feat: main moves on 1", "feat: main moves on 2"} {
		commitEmpty(t, seed, s)
	}
	runGitT(t, seed, "push", "origin", "main")

	dir := t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "checkout", "-b", "mine", oldMain) // cut from an OLD main commit, no stray anywhere
	head := commitEmpty(t, dir, "feat: long-lived branch work")

	if behind := runGitT(t, dir, "rev-list", "--count", head+"..refs/remotes/origin/main"); behind != "2" {
		t.Fatalf("fixture: expected the branch to be 2 commits behind main, got %s", behind)
	}
	found, err := checkForeignCommits(dir, "mine", head)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.strayBases) != 0 {
		t.Errorf("being behind main is NOT a stray-base cut and must not be refused; got %+v", found.strayBases)
	}
	if len(found.foreign) != 0 || len(found.masquerades) != 0 || len(found.indeterminate) != 0 {
		t.Errorf("expected a wholly clean result for an ordinary behind-main branch, got %+v", found)
	}
}

// TestCheckForeignCommits_AmbiguousOriginMainRefUsesRemoteTracking is the positive control
// for resolveOriginMain's fully-qualified spelling.
//
// It builds the AMBIGUOUS-REF shape this tool's own subject produces: a checkout that
// carries a real local branch literally named `refs/heads/origin/main` (left behind by a
// `git branch origin/main` / `git worktree add ... origin/main`) alongside the
// remote-tracking `refs/remotes/origin/main`. Bare `origin/main` then resolves to the LOCAL
// branch — git's rev-parse precedence puts refs/heads/ first — and `--verify --quiet`
// swallows the ambiguity warning, so the wrong base is picked with no diagnostic.
//
// The fixture makes that mis-resolution VISIBLE as a false positive: `landed-feature`
// merged into main and its remote branch was not deleted (the ordinary case). Against the
// TRUE base its commits are behind main and out of range. Against the STALE local
// `origin/main` they are in range AND reachable from `origin/landed-feature`, which is not
// an ancestor of that stale base — so they are reported foreign and a correctly-cut branch
// is refused.
//
// FAIL-FIRST: revert resolveOriginMain to the bare `origin/main` spelling and this test goes
// red with 2 spurious foreign commits.
func TestCheckForeignCommits_AmbiguousOriginMainRefUsesRemoteTracking(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")
	staleBase := runGitT(t, seed, "rev-parse", "HEAD")

	// A feature branch that LANDS on main, whose remote branch is not deleted afterwards.
	runGitT(t, seed, "checkout", "-b", "landed-feature")
	commitEmpty(t, seed, "feat: landed feature work")
	runGitT(t, seed, "push", "origin", "landed-feature")
	runGitT(t, seed, "checkout", "main")
	runGitT(t, seed, "merge", "--no-ff", "-m", "Merge pull request #1 from landed-feature", "landed-feature")
	runGitT(t, seed, "push", "origin", "main")

	// A worker who cut CORRECTLY from origin/main and added one commit of their own.
	goodDir := t.TempDir()
	runGitT(t, goodDir, "clone", remoteDir, ".")
	runGitT(t, goodDir, "config", "user.email", "good@test")
	runGitT(t, goodDir, "config", "user.name", "good")
	runGitT(t, goodDir, "checkout", "-b", "good-mine", "origin/main")
	ownSHA := commitEmpty(t, goodDir, "feat: my own genuine commit")

	// Plant the stray local branch that makes bare `origin/main` ambiguous AND stale.
	runGitT(t, goodDir, "branch", "origin/main", staleBase)

	// Guard the fixture itself: assert the ambiguity is really present and that the bare
	// spelling really does resolve to the stale local branch. Without this the test could
	// pass because the shape was never built, not because the code is right.
	if got := runGitT(t, goodDir, "rev-parse", "refs/heads/origin/main"); got != staleBase {
		t.Fatalf("fixture: refs/heads/origin/main = %s, want the stale base %s", got, staleBase)
	}
	trueBase := runGitT(t, goodDir, "rev-parse", "refs/remotes/origin/main")
	if trueBase == staleBase {
		t.Fatalf("fixture: remote-tracking base %s must differ from the stale base", trueBase)
	}
	if bare := runGitT(t, goodDir, "rev-parse", "--verify", "--quiet", "origin/main"); bare != staleBase {
		t.Fatalf("fixture: bare origin/main resolved to %s, expected the stale local branch %s", bare, staleBase)
	}

	found, err := checkForeignCommits(goodDir, "good-mine", ownSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.foreign) != 0 {
		t.Errorf("a correctly-cut branch must report no foreign commits even when a stray "+
			"refs/heads/origin/main makes the bare ref ambiguous; got %+v", found.foreign)
	}
	if len(found.masquerades) != 0 {
		t.Errorf("expected 0 merge masquerades, got %+v", found.masquerades)
	}
}

// TestResolveOriginMain_PrefersRemoteTrackingOverStrayLocalBranch pins the resolution itself,
// independent of the range logic above, so a regression is attributed to the right function.
func TestResolveOriginMain_PrefersRemoteTrackingOverStrayLocalBranch(t *testing.T) {
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")
	staleBase := runGitT(t, seed, "rev-parse", "HEAD")
	commitEmpty(t, seed, "feat: main moves on")
	runGitT(t, seed, "push", "origin", "main")

	dir := t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "branch", "origin/main", staleBase) // the stray, stale local branch
	trueBase := runGitT(t, dir, "rev-parse", "refs/remotes/origin/main")

	got, err := resolveOriginMain(dir)
	if err != nil {
		t.Fatalf("resolveOriginMain: %v", err)
	}
	if got == staleBase {
		t.Fatalf("resolveOriginMain returned the STALE local refs/heads/origin/main (%s) — "+
			"the bare spelling's rev-parse precedence leaked through", got)
	}
	if got != trueBase {
		t.Errorf("resolveOriginMain = %s, want the remote-tracking base %s", got, trueBase)
	}
}

// --- Integration: run() end-to-end with a real foreign-commit fixture, no fake gh needed
// (the foreign-commit check is local-only) but withFakeGH is still used so the PR-state
// check (which DOES shell out to `gh`) doesn't hit the network in CI.

func TestRun_RefusesPushCarryingForeignCommits(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE") // no PR yet for this branch — isolate the new check
	victimDir, ownBranch, ownSHA := newForeignCommitFixture(t)

	oldWD := chdir(t, victimDir)
	defer oldWD()

	stdin := stdinString("refs/heads/" + ownBranch + " " + ownSHA + " refs/heads/" + ownBranch + " 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/example-repo.git"}, stdin, &stderr)
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (ExitRefused). stderr:\n%s", rc, deskkit.ExitRefused, stderr.String())
	}
	// Assert the laundered-commit arm SPECIFICALLY, not just any refusal: the distinctive
	// diagnostic phrase plus its issue citation. A bare rc-is-ExitRefused check would stay
	// green if a different guard (MERGED/CLOSED, register-id) did the refusing instead.
	if !strings.Contains(stderr.String(), "foreign commit dragged in from a sibling branch") {
		t.Errorf("expected stderr to carry the foreign-commit diagnostic, got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "(#22)") {
		t.Errorf("expected stderr to cite (#22), got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "origin/sibling") {
		t.Errorf("expected stderr to name origin/sibling as the source, got:\n%s", stderr.String())
	}
	// The remediation must not hand the worker the very incantation that causes the
	// stray-base cut: `git worktree add <path> origin/main --detach` resolves through
	// refs/heads/ when the stray exists.
	if strings.Contains(stderr.String(), "worktree add <path> origin/main") {
		t.Errorf("refusal advises the AMBIGUOUS bare spelling; it must say "+
			"refs/remotes/origin/main. stderr:\n%s", stderr.String())
	}
}

// --- Self-exclusion of a branch's OWN commits on a detached-HEAD push (#22) ----------------
//
// A resume/shepherd worker updates an existing open PR branch from an isolated worktree, which
// git spells as a detached-HEAD push: `git push origin HEAD:refs/heads/<branch>`. The branch's
// OWN already-published content commits are, by definition, reachable from origin/<branch> and
// are NOT yet on origin/main — so unless the self-exclusion recognises the real branch, they
// read exactly like an in-flight sibling's commits and the push is refused. The bug was that
// the branch name came from the LOCAL side of the refspec (`HEAD`), so the self-exclusion
// compared against origin/HEAD and never matched.

// newExistingPRBranchFixture builds an existing open PR branch (`mine`) whose own content
// commits are already published to origin, plus a SEPARATE clone standing in for the
// resume/shepherd worker that updates it. Returns that clone's path, the branch name, and the
// branch tip sha (present in the clone as a fetched remote object).
func newExistingPRBranchFixture(t *testing.T) (victimDir, branch, headSHA string) {
	t.Helper()
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	// An existing open PR branch with its own content commits, already published to origin.
	runGitT(t, seed, "checkout", "-b", "mine")
	commitEmpty(t, seed, "feat: my own content commit B")
	headSHA = commitEmpty(t, seed, "feat: my own content commit C")
	runGitT(t, seed, "push", "origin", "mine")

	// A separate clone that updates the existing PR branch (the resume-worker case). After
	// clone, origin/mine and origin/main are present as remote-tracking refs and headSHA is
	// a fetched object.
	victimDir = t.TempDir()
	runGitT(t, victimDir, "clone", remoteDir, ".")
	runGitT(t, victimDir, "config", "user.email", "victim@test")
	runGitT(t, victimDir, "config", "user.name", "victim")
	return victimDir, "mine", headSHA
}

// TestCheckForeignCommits_OwnAlreadyPublishedCommitsNotFlagged pins the self-exclusion at the
// detector boundary: given the CORRECT branch name, a branch's own commits — reachable from
// origin/<branch> and not yet on origin/main — must NOT be reported foreign. This is the
// invariant the parseRef fix exists to feed (it ensures the correct name reaches here on a
// detached-HEAD push).
func TestCheckForeignCommits_OwnAlreadyPublishedCommitsNotFlagged(t *testing.T) {
	victimDir, branch, headSHA := newExistingPRBranchFixture(t)

	found, err := checkForeignCommits(victimDir, branch, headSHA)
	if err != nil {
		t.Fatalf("checkForeignCommits error: %v", err)
	}
	if len(found.indeterminate) != 0 {
		t.Fatalf("base was determinable here; unexpected could-not-check: %v", found.indeterminate)
	}
	if len(found.foreign) != 0 {
		t.Errorf("a branch's OWN commits (reachable from origin/%s) must be self-excluded, not "+
			"reported foreign; got %+v", branch, found.foreign)
	}
	if len(found.masquerades) != 0 {
		t.Errorf("expected no masquerades, got %+v", found.masquerades)
	}
}

// TestRun_DetachedHeadPushToExistingPRBranchAllowed is the core self-exclusion case at the process
// boundary: a detached-HEAD update push (`HEAD:refs/heads/<branch>`) of a branch's OWN
// already-published commits must be ALLOWED. Before the fix, parseRef derived ownBranch="HEAD",
// the self-exclusion compared against origin/HEAD, and commits B and C were refused as foreign.
//
// FAIL-FIRST: revert parseRef to derive the branch from the LOCAL side (parts[0]) AND drop its
// branch=="HEAD" guard, and this goes red — ownBranch resolves to "HEAD", so origin/mine is not
// self-excluded and B/C are refused as foreign. (TestParseRef pins the derivation on its own.)
func TestRun_DetachedHeadPushToExistingPRBranchAllowed(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "OPEN") // an open PR exists for this branch
	victimDir, branch, headSHA := newExistingPRBranchFixture(t)

	oldWD := chdir(t, victimDir)
	defer oldWD()

	stdin := stdinString("HEAD " + headSHA + " refs/heads/" + branch + " 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/example-repo.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want %d (ExitOK) — a detached-HEAD update push of a branch's OWN "+
			"commits must be allowed. stderr:\n%s", rc, deskkit.ExitOK, stderr.String())
	}
	if strings.Contains(stderr.String(), "foreign commit") {
		t.Errorf("the branch's own already-published commits were misreported as foreign:\n%s", stderr.String())
	}
}

// TestRun_OnBranchResyncMergePushAllowed is the resync-merge shape: a branch that resyncs by
// merging origin/main and pushes the result via the detached-HEAD spelling. The own content
// commits (reachable from origin/<branch>) plus the new two-parent merge head must all pass.
func TestRun_OnBranchResyncMergePushAllowed(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "OPEN")

	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")
	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	// The PR branch with its own content, published to origin.
	runGitT(t, seed, "checkout", "-b", "mine")
	commitEmpty(t, seed, "feat: my own content commit B")
	commitEmpty(t, seed, "feat: my own content commit C")
	runGitT(t, seed, "push", "origin", "mine")

	// main advances after the PR was cut.
	runGitT(t, seed, "checkout", "main")
	commitEmpty(t, seed, "feat: main moves on D")
	runGitT(t, seed, "push", "origin", "main")

	// The resume/shepherd clone: on the PR branch, resync by merging origin/main.
	dir := t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "checkout", "mine")
	runGitT(t, dir, "merge", "origin/main", "-m",
		"Merge remote-tracking branch 'origin/main' into mine")
	head := runGitT(t, dir, "rev-parse", "HEAD")

	oldWD := chdir(t, dir)
	defer oldWD()

	stdin := stdinString("HEAD " + head + " refs/heads/mine 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/example-repo.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want %d (ExitOK) — an on-branch resync-merge push of a branch's own "+
			"content plus a real merge head must be allowed. stderr:\n%s", rc, deskkit.ExitOK, stderr.String())
	}
	if strings.Contains(stderr.String(), "foreign commit") {
		t.Errorf("the branch's own commits or its real merge head were misreported as foreign:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "masquerade") {
		t.Errorf("the real two-parent resync merge was misreported as a single-parent masquerade:\n%s", stderr.String())
	}
}

// TestRun_DetachedHeadPushStillRefusesForeignSiblingCommit is the SECURITY FLOOR for the fix:
// even with the branch name now correctly derived from the remote side of a detached-HEAD
// refspec, a commit reachable ONLY from a DIFFERENT sibling branch (not main, not this branch's
// own origin ref) must STILL be refused. The fix narrows the self-exclusion to origin/<own
// branch>; it must not blanket-exclude everything on a detached push.
//
// FAIL-FIRST: broaden the self-exclusion to skip the foreign-commit check whenever the local
// ref is HEAD and this goes green (wrongly), letting a laundered sibling commit through.
func TestRun_DetachedHeadPushStillRefusesForeignSiblingCommit(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE") // no PR yet — isolate the foreign-commit arm
	victimDir, ownBranch, ownSHA := newForeignCommitFixture(t)

	oldWD := chdir(t, victimDir)
	defer oldWD()

	// Same detached-HEAD spelling as the allowed cases above — but here the pushed range
	// carries the sibling's laundered commits, reachable from origin/sibling (≠ origin/mine,
	// and not an ancestor of main).
	stdin := stdinString("HEAD " + ownSHA + " refs/heads/" + ownBranch + " 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/example-repo.git"}, stdin, &stderr)
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (ExitRefused) — a genuinely foreign sibling commit must still "+
			"be refused on a detached-HEAD push. stderr:\n%s", rc, deskkit.ExitRefused, stderr.String())
	}
	if !strings.Contains(stderr.String(), "foreign commit dragged in from a sibling branch") {
		t.Errorf("expected the foreign-commit diagnostic, got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "origin/sibling") {
		t.Errorf("expected origin/sibling named as the source, got:\n%s", stderr.String())
	}
}

// TestRun_RefusesStrayBaseCut drives run() end-to-end on a worktree cut from the stray local
// `origin/main` and asserts the process-level refusal, not just the detector's return value.
func TestRun_RefusesStrayBaseCut(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE")
	wtDir, branch, headSHA, _, _ := newStrayBaseFixture(t)

	oldWD := chdir(t, wtDir)
	defer oldWD()

	stdin := stdinString("refs/heads/" + branch + " " + headSHA + " refs/heads/" + branch + " 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/example-repo.git"}, stdin, &stderr)
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (ExitRefused). stderr:\n%s", rc, deskkit.ExitRefused, stderr.String())
	}
	// Assert the stray-base arm SPECIFICALLY — a bare rc check would stay green if some
	// other guard did the refusing.
	if !strings.Contains(stderr.String(), "STRAY LOCAL branch refs/heads/origin/main") {
		t.Errorf("expected the stray-base diagnostic, got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "refs/remotes/origin/main --detach") {
		t.Errorf("expected the refusal to prescribe the UNAMBIGUOUS re-cut, got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "3 commit(s) behind main") {
		t.Errorf("expected the refusal to quantify how far behind the stale base is, got:\n%s", stderr.String())
	}
}

// TestRun_AnnouncesCouldNotCheck proves the fails-closed REPORTING contract at the process
// boundary: when the base cannot be determined the push is still allowed (ExitOK, brief-10),
// but stderr must say so in terms no one can mistake for a clean bill of health. Silence here
// was the defect — an unverifiable base and a verified-clean base printed identically.
func TestRun_AnnouncesCouldNotCheck(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE")

	dir := t.TempDir()
	runGitT(t, dir, "init", "-b", "main")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	sha := commitEmpty(t, dir, "chore: a repo with no origin remote at all")

	oldWD := chdir(t, dir)
	defer oldWD()

	stdin := stdinString("refs/heads/main " + sha + " refs/heads/main 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/example-repo.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want %d (ExitOK — indeterminacy must not wedge a push)", rc, deskkit.ExitOK)
	}
	if !strings.Contains(stderr.String(), "COULD-NOT-CHECK the base of main") {
		t.Fatalf("an unverifiable base must be ANNOUNCED, not passed over in silence. stderr:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "NOT a clean bill of health") {
		t.Errorf("the could-not-check line must state it is not an all-clear, got:\n%s", stderr.String())
	}
}
