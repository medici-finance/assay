package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// NOTE on naming: as of this writing #746 (foreigncommit_test.go) is not yet merged to
// main, so this file uses its own regID*-prefixed git-fixture helpers instead of sharing
// names like runGitT/commitEmpty that #746 also introduces. Once #746 lands, a follow-up can
// collapse the duplication; until then the two PRs stay mechanically non-conflicting.

// regIDRunGitT runs git in dir and fails the test on error, returning trimmed stdout.
func regIDRunGitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (dir=%s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// regIDWriteFile writes content to relPath under dir, creating parent directories as
// needed, then returns the absolute path written.
func regIDWriteFile(t *testing.T, dir, relPath, content string) string {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", full, err)
	}
	return full
}

// newRegisterIDCollisionFixture reproduces #72's second unique failure shape verbatim:
// tracker#566 reused `id: F-39`. Two workers, EACH correctly cutting a worktree from
// origin/main (neither branch contains the other's commits — this is not #22's
// wrong-base bug), independently add a findings-register entry and happen to pick the same
// id. It returns the SECOND worker's clone (the one about to push and collide) plus its own
// branch name and head sha.
func newRegisterIDCollisionFixture(t *testing.T, firstID, secondID string) (dir, ownBranch, ownSHA string) {
	t.Helper()
	remoteDir := t.TempDir()
	regIDRunGitT(t, remoteDir, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	regIDRunGitT(t, seed, "init", "-b", "main")
	regIDRunGitT(t, seed, "config", "user.email", "seed@test")
	regIDRunGitT(t, seed, "config", "user.name", "seed")
	regIDRunGitT(t, seed, "remote", "add", "origin", remoteDir)
	regIDWriteFile(t, seed, "README.md", "# repo\n")
	regIDRunGitT(t, seed, "add", ".")
	regIDRunGitT(t, seed, "commit", "-m", "chore: initial commit on main")
	regIDRunGitT(t, seed, "push", "origin", "main")

	// Worker A: cuts fresh from origin/main, adds a findings entry claiming firstID, pushes
	// its branch to origin (an in-flight sibling PR — not yet merged to origin/main).
	workerA := t.TempDir()
	regIDRunGitT(t, workerA, "clone", remoteDir, ".")
	regIDRunGitT(t, workerA, "config", "user.email", "a@test")
	regIDRunGitT(t, workerA, "config", "user.name", "worker-a")
	regIDRunGitT(t, workerA, "checkout", "-b", "worker-a-branch", "origin/main")
	regIDWriteFile(t, workerA, "docs/streams/findings/2026-08-01-worker-a-finding.md",
		"---\nid: "+firstID+"\ndate: \"2026-08-01\"\ntitle: \"worker A's finding\"\n---\n\nBody.\n")
	regIDRunGitT(t, workerA, "add", ".")
	regIDRunGitT(t, workerA, "commit", "-m", "docs(findings): worker A's finding")
	regIDRunGitT(t, workerA, "push", "origin", "worker-a-branch")

	// Worker B: ALSO cuts fresh from origin/main (same point — worker A's branch is not yet
	// merged, so origin/main hasn't moved), adds its OWN findings entry, and — the bug —
	// independently picks the same id.
	workerB := t.TempDir()
	regIDRunGitT(t, workerB, "clone", remoteDir, ".")
	regIDRunGitT(t, workerB, "config", "user.email", "b@test")
	regIDRunGitT(t, workerB, "config", "user.name", "worker-b")
	regIDRunGitT(t, workerB, "checkout", "-b", "worker-b-branch", "origin/main")
	regIDWriteFile(t, workerB, "docs/streams/findings/2026-08-01-worker-b-finding.md",
		"---\nid: "+secondID+"\ndate: \"2026-08-01\"\ntitle: \"worker B's finding\"\n---\n\nBody.\n")
	regIDRunGitT(t, workerB, "add", ".")
	regIDRunGitT(t, workerB, "commit", "-m", "docs(findings): worker B's finding")
	headSHA := regIDRunGitT(t, workerB, "rev-parse", "HEAD")

	// Worker B hasn't pushed yet — but worker B's clone already has worker A's branch
	// visible as origin/worker-a-branch (fetched at clone time, since worker A pushed
	// before worker B cloned), exactly as a real pre-push hook would see it.
	return workerB, "worker-b-branch", headSHA
}

func TestCheckRegisterIDCollisions_DetectsOit566StyleCollision(t *testing.T) {
	dir, ownBranch, ownSHA := newRegisterIDCollisionFixture(t, "F-39", "F-39")

	// First: confirm the ALREADY-COVERED failure shape (foreign/laundered commits) reports
	// NOTHING here — this is the gap #746 leaves. Worker B's branch is cut correctly from
	// origin/main and carries no commit reachable from worker A's branch, so an
	// ancestry-based check has nothing to find.
	if hasAnyCommitReachableFromOtherBranch(t, dir, ownBranch, ownSHA) {
		t.Fatalf("fixture invariant broken: worker B's commit should not be reachable from any sibling branch")
	}

	collisions, err := checkRegisterIDCollisions(dir, ownBranch, ownSHA)
	if err != nil {
		t.Fatalf("checkRegisterIDCollisions error: %v", err)
	}
	if len(collisions) != 1 {
		t.Fatalf("expected exactly 1 collision, got %d: %+v", len(collisions), collisions)
	}
	c := collisions[0]
	if c.id != "F-39" {
		t.Errorf("collision id = %q, want F-39", c.id)
	}
	if !strings.Contains(c.ownPath, "worker-b-finding.md") {
		t.Errorf("collision ownPath = %q, want worker-b-finding.md", c.ownPath)
	}
	if c.sourceBranch != "origin/worker-a-branch" {
		t.Errorf("collision sourceBranch = %q, want origin/worker-a-branch", c.sourceBranch)
	}
	if !strings.Contains(c.sourcePath, "worker-a-finding.md") {
		t.Errorf("collision sourcePath = %q, want worker-a-finding.md", c.sourcePath)
	}
}

func TestCheckRegisterIDCollisions_DifferentIDsReportNothing(t *testing.T) {
	dir, ownBranch, ownSHA := newRegisterIDCollisionFixture(t, "F-legit-alpha", "F-legit-beta")

	collisions, err := checkRegisterIDCollisions(dir, ownBranch, ownSHA)
	if err != nil {
		t.Fatalf("checkRegisterIDCollisions error: %v", err)
	}
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions with distinct ids, got %+v", collisions)
	}
}

func TestCheckRegisterIDCollisions_FailsOpenOnNonSHA(t *testing.T) {
	collisions, err := checkRegisterIDCollisions(t.TempDir(), "some-branch", "not-a-sha")
	if err != nil {
		t.Fatalf("expected fail-open (nil error), got %v", err)
	}
	if collisions != nil {
		t.Errorf("expected nil collisions on non-SHA input, got %+v", collisions)
	}
}

func TestCheckRegisterIDCollisions_FailsOpenWhenOriginMainUnresolvable(t *testing.T) {
	dir := t.TempDir()
	regIDRunGitT(t, dir, "init", "-b", "main")
	regIDRunGitT(t, dir, "config", "user.email", "x@test")
	regIDRunGitT(t, dir, "config", "user.name", "x")
	regIDRunGitT(t, dir, "commit", "--allow-empty", "-m", "chore: only commit, no origin remote at all")
	sha := regIDRunGitT(t, dir, "rev-parse", "HEAD")

	collisions, err := checkRegisterIDCollisions(dir, "main", sha)
	if err != nil {
		t.Fatalf("expected fail-open (nil error), got %v", err)
	}
	if collisions != nil {
		t.Errorf("expected nil collisions when origin/main is unresolvable, got %+v", collisions)
	}
}

// hasAnyCommitReachableFromOtherBranch is a tiny local re-implementation of the "is this
// commit reachable from some other remote branch" ancestry test that foreigncommit.go's
// checkForeignCommits performs (that file doesn't exist on this branch yet — the change
// that adds it is not merged as of this writing) — used here only to make this
// test's "the existing ancestry-based approach finds nothing" claim self-verifying rather
// than asserted in a comment.
func hasAnyCommitReachableFromOtherBranch(t *testing.T, dir, ownBranch, sha string) bool {
	t.Helper()
	out := regIDRunGitT(t, dir, "branch", "-r", "--contains", sha)
	ownRemote := "origin/" + ownBranch
	for _, raw := range strings.Split(out, "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "* "))
		if b == "" || b == ownRemote {
			continue
		}
		return true
	}
	return false
}

// TestRun_RefusesPushCarryingRegisterIDCollision is the end-to-end proof: run() itself,
// invoked exactly as the real pre-push hook would be (stdin ref line, DESKPUSHGUARD
// env), refuses worker B's push and cites the collision rule in stderr, with cwd set to
// worker B's worktree (dir="" in checkRegisterIDCollisions resolves to process cwd, matching
// main.go's own call site).
func TestRun_RefusesPushCarryingRegisterIDCollision(t *testing.T) {
	dir, ownBranch, ownSHA := newRegisterIDCollisionFixture(t, "F-39", "F-39")

	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore os.Chdir(%s): %v", old, err)
		}
	})

	// No PR exists yet for worker-b-branch — DESKPUSHGUARD_FAKE_STATE=NONE mirrors the real
	// "no pull requests found" gh response, so the MERGED/CLOSED half of the guard allows
	// the push and the register-id collision check is the only thing that can refuse it.
	t.Setenv("DESKPUSHGUARD_FAKE_STATE", "NONE")

	stdin := strings.NewReader("refs/heads/" + ownBranch + " " + ownSHA + " refs/heads/" + ownBranch + " 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/medici-finance/assay.git"}, stdin, &stderr)

	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want ExitRefused(%d). stderr:\n%s", rc, deskkit.ExitRefused, stderr.String())
	}
	if !strings.Contains(stderr.String(), "F-39") {
		t.Errorf("expected stderr to name the colliding id F-39, got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "rename/re-derive") {
		t.Errorf("expected stderr to cite the collision rule (rename/re-derive), got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "worker-a-branch") {
		t.Errorf("expected stderr to name the sibling source branch, got:\n%s", stderr.String())
	}
}
