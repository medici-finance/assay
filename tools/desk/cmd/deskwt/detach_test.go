package main

// detach_test.go — FAIL-FIRST coverage for `deskwt add --detach` (#1309 item 6, deskwt half).
//
// THE DEFECT. A verifier's worktree was cut on the brief's own `feat/<id>` branch, so a
// delivered brief whose feature branch still sat in a stale worker worktree made `deskwt add`
// refuse the verifier with "already exists and is CHECKED OUT" — although a verifier runs
// against merged origin/main and needs no branch at all. These pin that --detach cuts a
// detached-HEAD worktree at the base, creates and touches no branch, is NOT refused by a
// checked-out branch of the same name, and is removable afterwards (#851 detached-remove).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestAddDetachCutsDetachedHeadAndTouchesNoBranch(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	// A worker worktree already HOLDS the branch a same-named tracking add would collide with.
	if rc := run([]string{"add", "item-1", "--branch", "feat/item-1"}); rc != deskkit.ExitOK {
		t.Fatalf("worker add rc = %d, want 0", rc)
	}
	resetCalls(calls)

	// The verifier's add: detached, under its own name — the held branch is irrelevant.
	rc, stderr := runCapErr(t, []string{"add", "verify-item-1", "--detach", "--base", "refs/remotes/origin/main"})
	if rc != deskkit.ExitOK {
		t.Fatalf("add --detach rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-item-1")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("detached worktree %s not created: %v", target, err)
	}
	if head := mustGit(t, target, "rev-parse", "--abbrev-ref", "HEAD"); head != "HEAD" {
		t.Fatalf("worktree HEAD = %q, want a detached HEAD", head)
	}
	if got, want := mustGit(t, target, "rev-parse", "HEAD"), mustGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != want {
		t.Fatalf("detached HEAD %s is not the base %s", got, want)
	}
	// No branch was created for it, and the argv carried --detach with no -b.
	branches := mustGit(t, work, "branch", "--list", "verify-item-1", "tracker-verify-item-1")
	if strings.TrimSpace(branches) != "" {
		t.Fatalf("a detached add created a branch: %q", branches)
	}
	sawDetach := false
	for _, c := range gitCalls(*calls) {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, "worktree add") {
			if !strings.Contains(joined, "--detach") || strings.Contains(joined, " -b ") {
				t.Fatalf("worktree add argv is not the detached shape: %v", c)
			}
			sawDetach = true
		}
	}
	if !sawDetach || anyGitForce(*calls) {
		t.Fatalf("expected one `git worktree add --detach`, no force; git calls: %v", gitCalls(*calls))
	}

	// The lifecycle closes: a clean detached worktree at a remote-proven commit is removable.
	if rc := run([]string{"remove", target}); rc != deskkit.ExitOK {
		t.Fatalf("remove of the detached verifier worktree rc = %d, want 0", rc)
	}
}

func TestAddDetachAndBranchAreMutuallyExclusive(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	rc, stderr := runCapErr(t, []string{"add", "x", "--detach", "--branch", "feat/x"})
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("--detach with --branch rc = %d (want 5), stderr: %s", rc, stderr)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("git worktree add ran despite the refusal: %v", gitCalls(*calls))
	}
}
