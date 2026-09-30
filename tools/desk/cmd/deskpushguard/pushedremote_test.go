package main

// pushedremote_test.go — the guard judges the remote git is ACTUALLY pushing to (#1201).
//
// git invokes the hook as `<hook> <remote-name> <remote-url>`. Every base-dependent check —
// the foreign-commit base, the register-id base, its sibling candidates and its liveness
// probe — must use that remote, and with no usable main on it say COULD-NOT-CHECK rather than
// quietly judging the push against `origin`.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// bareWithMain makes a bare repository whose main carries one commit, pushed from a
// throwaway seed. The label keeps the two repositories' histories visibly distinct.
func bareWithMain(t *testing.T, label string) (bare, seed string) {
	t.Helper()
	bare = t.TempDir()
	runGitT(t, bare, "init", "--bare", "-b", "main")
	seed = t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", label+"@test")
	runGitT(t, seed, "config", "user.name", label)
	runGitT(t, seed, "remote", "add", "origin", bare)
	commitEmpty(t, seed, "chore: "+label+" initial commit")
	runGitT(t, seed, "push", "origin", "main")
	return bare, seed
}

// pushSiblingFinding pushes, from seed, a sibling branch off main that adds a findings entry
// claiming id — an in-flight PR on that repository.
func pushSiblingFinding(t *testing.T, seed, branch, path, id string) {
	t.Helper()
	runGitT(t, seed, "checkout", "-q", "-b", branch, "main")
	regIDWriteFile(t, seed, path, "---\nid: "+id+"\ndate: \"2026-09-01\"\ntitle: \"sibling finding\"\n---\n\nBody.\n")
	runGitT(t, seed, "add", ".")
	runGitT(t, seed, "commit", "-q", "-m", "docs(findings): sibling finding")
	runGitT(t, seed, "push", "origin", branch)
	runGitT(t, seed, "checkout", "-q", "main")
}

// addOwnFinding commits, on the current branch of dir, a findings entry claiming id and
// returns the new head.
func addOwnFinding(t *testing.T, dir, path, id string) string {
	t.Helper()
	regIDWriteFile(t, dir, path, "---\nid: "+id+"\ndate: \"2026-09-01\"\ntitle: \"my finding\"\n---\n\nBody.\n")
	runGitT(t, dir, "add", ".")
	runGitT(t, dir, "commit", "-q", "-m", "docs(findings): my own finding")
	return runGitT(t, dir, "rev-parse", "HEAD")
}

// twoRemoteWorktree is #1201's shape: `origin` is repository A (whose main differs, and which
// carries an in-flight sibling claiming findings id F-1), while the push goes to a SECOND
// remote, "target", for repository B. It returns the worktree and the target's seed clone.
func twoRemoteWorktree(t *testing.T) (dir, targetSeed string) {
	t.Helper()
	wrongBare, wrongSeed := bareWithMain(t, "wrong")
	pushSiblingFinding(t, wrongSeed, "sib", "docs/streams/findings/2026-09-01-a-sibling.md", "F-1")
	targetBare, targetSeed := bareWithMain(t, "target")

	dir = t.TempDir()
	runGitT(t, dir, "init", "-b", "main")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "remote", "add", "origin", wrongBare)
	runGitT(t, dir, "fetch", "-q", "origin")
	runGitT(t, dir, "remote", "add", "target", targetBare)
	runGitT(t, dir, "fetch", "-q", "target")
	return dir, targetSeed
}

func pushLine(branch, sha string) string {
	return "refs/heads/" + branch + " " + sha + " refs/heads/" + branch + " 0000000000000000000000000000000000000000\n"
}

// TestForeignCommitCheckUsesThePushedRemotesMain is #1201's reproduction end to end: a branch
// correctly cut from refs/remotes/target/main, pushed to "target", in a worktree whose origin
// is an unrelated repository. Judged against origin's main every commit on the branch is
// "foreign" and its new findings id collides with origin's sibling; judged against the pushed
// remote's main the push is clean.
//
// FAIL-FIRST: on the unfixed guard (the base, the register-id candidates and the liveness
// probe all spelled `origin`) this push is refused.
func TestForeignCommitCheckUsesThePushedRemotesMain(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE")
	dir, _ := twoRemoteWorktree(t)
	runGitT(t, dir, "checkout", "-q", "-b", "mine", "refs/remotes/target/main")
	sha := addOwnFinding(t, dir, "docs/streams/findings/2026-09-01-mine.md", "F-1")

	defer chdir(t, dir)()
	var stderr strings.Builder
	rc := run([]string{"target", "https://github.com/example-org/example-repo.git"},
		stdinString(pushLine("mine", sha)), &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want %d: a branch cut from the pushed remote's main was judged "+
			"against origin's. stderr:\n%s", rc, deskkit.ExitOK, stderr.String())
	}
	for _, bad := range []string{"refusing", "foreign commit", "COULD-NOT-CHECK", "origin/sib"} {
		if strings.Contains(stderr.String(), bad) {
			t.Errorf("stderr carries %q — the push was judged against origin, not target:\n%s", bad, stderr.String())
		}
	}

	// Positive control: the same guard still refuses what IS wrong on the pushed remote — a
	// findings id already claimed by an in-flight sibling on target. Target's main also
	// carries an existing entry F-9 that the sibling re-claims at another path: F-9 is NOT a
	// claim of this push (it is already on the pushed remote's main), so only F-7 is named;
	// measured from origin's main instead, every file of target's main would read as new.
	t.Run("a collision on the pushed remote still refuses", func(t *testing.T) {
		dir2, seed2 := twoRemoteWorktree(t)
		regIDWriteFile(t, seed2, "docs/streams/findings/2026-08-01-on-main.md",
			"---\nid: F-9\ndate: \"2026-08-01\"\ntitle: \"already on main\"\n---\n\nBody.\n")
		runGitT(t, seed2, "add", ".")
		runGitT(t, seed2, "commit", "-q", "-m", "docs(findings): an entry already on main")
		runGitT(t, seed2, "push", "origin", "main")
		pushSiblingFinding(t, seed2, "tsib", "docs/streams/findings/2026-09-01-t-sibling.md", "F-7")
		runGitT(t, seed2, "checkout", "-q", "tsib")
		regIDWriteFile(t, seed2, "docs/streams/findings/2026-09-01-t-reclaim.md",
			"---\nid: F-9\ndate: \"2026-09-01\"\ntitle: \"re-claims F-9\"\n---\n\nBody.\n")
		runGitT(t, seed2, "add", ".")
		runGitT(t, seed2, "commit", "-q", "-m", "docs(findings): sibling re-claims F-9")
		runGitT(t, seed2, "push", "origin", "tsib")
		runGitT(t, dir2, "fetch", "-q", "target")
		runGitT(t, dir2, "checkout", "-q", "-b", "mine", "refs/remotes/target/main")
		sha2 := addOwnFinding(t, dir2, "docs/streams/findings/2026-09-01-mine.md", "F-7")

		defer chdir(t, dir2)()
		var stderr2 strings.Builder
		rc := run([]string{"target", "https://github.com/example-org/example-repo.git"},
			stdinString(pushLine("mine", sha2)), &stderr2)
		if rc != deskkit.ExitRefused {
			t.Fatalf("rc = %d, want %d: a sibling on the pushed remote claims F-7. stderr:\n%s",
				rc, deskkit.ExitRefused, stderr2.String())
		}
		if !strings.Contains(stderr2.String(), "F-7") || !strings.Contains(stderr2.String(), "target/tsib") {
			t.Errorf("refusal should name F-7 and the pushed remote's sibling target/tsib:\n%s", stderr2.String())
		}
		if strings.Contains(stderr2.String(), "F-9") {
			t.Errorf("F-9 is already on the pushed remote's main, so it is no claim of this push — "+
				"the register-id base was not the pushed remote's main:\n%s", stderr2.String())
		}
	})
}

// TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin is the negative path: the pushed remote has
// no main tracking ref, while origin has one against which this push WOULD be refused (it is
// cut from an origin sibling and claims an id another origin sibling holds). The answer is
// COULD-NOT-CHECK naming the pushed remote's main — never a silent fall-back to origin's.
func TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE")

	originBare, originSeed := bareWithMain(t, "origin")
	pushSiblingFinding(t, originSeed, "sib", "docs/streams/findings/2026-09-01-a-sibling.md", "F-3")
	runGitT(t, originSeed, "checkout", "-q", "-b", "other", "main")
	commitEmpty(t, originSeed, "feat: another in-flight sibling's work")
	runGitT(t, originSeed, "push", "origin", "other")

	// The pushed remote exists but has never been fetched: no refs/remotes/nomain/main.
	emptyBare := t.TempDir()
	runGitT(t, emptyBare, "init", "--bare", "-b", "main")

	dir := t.TempDir()
	runGitT(t, dir, "init", "-b", "main")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "remote", "add", "origin", originBare)
	runGitT(t, dir, "fetch", "-q", "origin")
	runGitT(t, dir, "remote", "add", "nomain", emptyBare)
	runGitT(t, dir, "checkout", "-q", "-b", "mine", "refs/remotes/origin/other")
	sha := addOwnFinding(t, dir, "docs/streams/findings/2026-09-01-mine.md", "F-3")

	defer chdir(t, dir)()
	var stderr strings.Builder
	rc := run([]string{"nomain", "https://github.com/example-org/example-repo.git"},
		stdinString(pushLine("mine", sha)), &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want %d (fail-open on an undeterminable base). stderr:\n%s",
			rc, deskkit.ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "COULD-NOT-CHECK the base of mine") ||
		!strings.Contains(stderr.String(), "refs/remotes/nomain/main") {
		t.Errorf("want COULD-NOT-CHECK naming refs/remotes/nomain/main, got:\n%s", stderr.String())
	}
	for _, bad := range []string{"refusing", "origin/other", "origin/sib"} {
		if strings.Contains(stderr.String(), bad) {
			t.Errorf("stderr carries %q — the guard fell back to origin's main:\n%s", bad, stderr.String())
		}
	}

	// No remote name at all is the same answer, not a guess at origin.
	t.Run("no remote name", func(t *testing.T) {
		var stderr strings.Builder
		rc := run([]string{"", "https://github.com/example-org/example-repo.git"},
			stdinString(pushLine("mine", sha)), &stderr)
		if rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want %d. stderr:\n%s", rc, deskkit.ExitOK, stderr.String())
		}
		if !strings.Contains(stderr.String(), "COULD-NOT-CHECK the base of mine") ||
			!strings.Contains(stderr.String(), "no fall-back to origin") {
			t.Errorf("want COULD-NOT-CHECK with no fall-back to origin, got:\n%s", stderr.String())
		}
		if strings.Contains(stderr.String(), "refusing") {
			t.Errorf("judged against origin with no remote named:\n%s", stderr.String())
		}
	})
}
