package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// roleinitstale_test.go — #6115: role-init over its own registration whose directory a tmp
// sweep removed. git refuses `worktree add` at a registered path whose directory is missing
// ("is a missing but locked worktree"), so the boot stopped until someone unlocked and pruned
// the entry by hand.

// verifierPrefix is the verifier role's branch prefix in the fixture roster; role-init's
// target for a session is tmpBaseDir/tracker-<prefix>-<session>.
const verifierPrefix = "verify-desk"

func verifierTarget(session string) string {
	return filepath.Join(tmpBaseDir, "tracker-"+verifierPrefix+"-"+session)
}

// sweptRoleWorktree provisions a verifier role worktree for session, then deletes its
// directory the way a tmp sweep does, leaving the registration (and its lock) behind.
func sweptRoleWorktree(t *testing.T, session string) string {
	t.Helper()
	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", session}); rc != deskkit.ExitOK {
		t.Fatalf("first role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := verifierTarget(session)
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("simulate the tmp sweep: %v", err)
	}
	return target
}

// worktreeBlock returns the `git worktree list --porcelain` block for path ("" if absent).
func worktreeBlock(t *testing.T, work, path string) string {
	t.Helper()
	want := resolvePath(path)
	for _, b := range strings.Split(mustGit(t, work, "worktree", "list", "--porcelain"), "\n\n") {
		first := strings.SplitN(b, "\n", 2)[0]
		if resolvePath(strings.TrimPrefix(first, "worktree ")) == want {
			return b
		}
	}
	return ""
}

// regression: #6115
func TestRoleInitClearsOwnStaleLock(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target := sweptRoleWorktree(t, "boot1")

	// A second, unrelated registration whose directory is also gone. The recovery must touch
	// only role-init's own entry: a repo-wide `git worktree prune` would drop this one too.
	other := filepath.Join(tmpBaseDir, "unrelated")
	mustGit(t, work, "worktree", "add", "--detach", other, "HEAD")
	if err := os.RemoveAll(other); err != nil {
		t.Fatal(err)
	}
	resetCalls(calls)

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "boot1"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init over its own swept worktree rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "cleared its own stale registration") {
		t.Errorf("stderr does not report the recovery: %s", stderr)
	}
	assertExists(t, filepath.Join(target, ".git"))
	if br := mustGit(t, target, "rev-parse", "--abbrev-ref", "HEAD"); br != "verify-desk/boot1" {
		t.Fatalf("recovered worktree branch = %q, want verify-desk/boot1", br)
	}
	if b := worktreeBlock(t, work, target); !strings.Contains(b, "locked verify-desk live session (deskwt role-init session=boot1)") {
		t.Fatalf("recovered worktree is not re-locked with role-init's reason:\n%s", b)
	}
	if worktreeBlock(t, work, other) == "" {
		t.Fatalf("the unrelated missing registration %s was pruned; only role-init's own entry may be touched", other)
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag: %v", gitCalls(*calls))
	}
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("recovery ran a repo-wide `git worktree prune`: %v", gitCalls(*calls))
	}
}

// regression: #6115
func TestRoleInitKeepsForeignStale(t *testing.T) {
	cases := []struct {
		name   string
		reason string // "" = leave the registration unlocked
	}{
		{"other session", "verify-desk live session (deskwt role-init session=other)"},
		{"other role", "worker-desk live session (deskwt role-init session=keep)"},
		{"hand lock", "held by hand for inspection"},
		{"git add lock", gitInitLockReason},
		{"unlocked", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			work := newRepo(t)
			calls := withEnv(t, work)
			target := sweptRoleWorktree(t, "keep")
			mustGit(t, work, "worktree", "unlock", target)
			if tc.reason != "" {
				mustGit(t, work, "worktree", "lock", "--reason", tc.reason, target)
			}
			before := worktreeBlock(t, work, target)
			resetCalls(calls)

			rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "keep"})
			if rc != deskkit.ExitRefused {
				t.Fatalf("role-init over a foreign stale registration rc = %d, want 5 (refused); stderr: %s", rc, stderr)
			}
			if !strings.Contains(stderr, "clears only its own stale registration") {
				t.Errorf("refusal does not name the boundary: %s", stderr)
			}
			if after := worktreeBlock(t, work, target); after != before {
				t.Fatalf("the foreign registration changed:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			for _, verb := range []string{"unlock", "remove", "prune", "add"} {
				if hasWorktreeVerb(*calls, verb) {
					t.Fatalf("refusal ran `git worktree %s`: %v", verb, gitCalls(*calls))
				}
			}
		})
	}
}

// TestRoleInitReusesLeftBranch is the issue's "possible follow-on": once the stale entry is
// cleared, does role-init take the `-b <branch>` arm and die on "branch already exists"? It
// does not: role-init checks refs/heads/<branch> first and attaches an existing branch. The
// fixture is the reported shape: the stale entry has an unrelated branch checked out while
// the role branch is left behind locally (a failed `worktree add --track -b` creates the
// branch before git refuses the path).
//
// regression: #6115
func TestRoleInitReusesLeftBranch(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "left"}); rc != deskkit.ExitOK {
		t.Fatalf("first role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := verifierTarget("left")
	mustGit(t, target, "checkout", "-q", "-b", "fixture/side")
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "rev-parse", "--verify", "refs/heads/verify-desk/left")
	resetCalls(calls)

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "left"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init with the role branch left behind rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if br := mustGit(t, target, "rev-parse", "--abbrev-ref", "HEAD"); br != "verify-desk/left" {
		t.Fatalf("worktree branch = %q, want verify-desk/left", br)
	}
	for _, c := range gitCalls(*calls) {
		if strings.Contains(strings.Join(c, " "), "worktree add --track -b") {
			t.Fatalf("role-init took the -b arm with the branch already present: %v", c)
		}
	}
	// The unrelated branch the stale entry held survives the clearing.
	mustGit(t, work, "rev-parse", "--verify", "refs/heads/fixture/side")
}

// TestGitFailCarriesFatalLine pins the error text half of #6115 on the one git seam every
// deskwt verb uses: git prints progress on stderr ahead of its `fatal:` line, and the
// surfaced message used to stop at the progress line. Both role-init arms are checked (the
// existing-branch arm prints `Preparing worktree`; the -b arm adds a `set up to track` line).
//
// regression: #6115
func TestGitFailCarriesFatalLine(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := sweptRoleWorktree(t, "said")
	mustGit(t, work, "branch", "fixture/attach", "HEAD")
	for _, args := range [][]string{
		{"worktree", "add", target, "fixture/attach"},
		{"worktree", "add", "--track", "-b", "fixture/fresh", target, "origin/main"},
	} {
		_, err := runGit(work, args...)
		if err == nil {
			t.Fatalf("git %v over a missing-but-locked path succeeded; want git's refusal", args)
		}
		msg := err.Error()
		if !strings.Contains(msg, "Preparing worktree") {
			t.Fatalf("fixture drift: git's progress line is not in the message: %q", msg)
		}
		if !strings.Contains(msg, "fatal:") || !strings.Contains(msg, "missing but locked worktree") {
			t.Errorf("git %v: the message drops git's fatal line: %q", args, msg)
		}
		if strings.Contains(msg, "\n") {
			t.Errorf("git %v: the message is not one line: %q", args, msg)
		}
	}
}

// TestAddFailCarriesFatalLine is the defect class's second instance, planted on a different
// verb: `deskwt add` over a registered path whose directory is gone reaches the same git
// refusal through the same seam, and its message must name git's cause too, not the
// `Preparing worktree` progress line ahead of it. add is not taught to recover here: its
// target is not session-derived, so it has no own-lock proof to match.
//
// regression: #6115
func TestAddFailCarriesFatalLine(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	const name = "planted"
	target := filepath.Join(tmpBaseDir, "tracker-"+name) // deskwt add's target for name
	mustGit(t, work, "worktree", "add", "--detach", target, "HEAD")
	mustGit(t, work, "worktree", "lock", "--reason", "fixture lock", target)
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}

	rc, stderr := runCapErr(t, []string{"add", name})
	if rc == deskkit.ExitOK {
		t.Fatalf("add over a missing-but-locked registration succeeded; stderr: %s", stderr)
	}
	if !strings.Contains(stderr, "missing but locked worktree") {
		t.Fatalf("add's refusal drops git's fatal line:\n%s", stderr)
	}
}
