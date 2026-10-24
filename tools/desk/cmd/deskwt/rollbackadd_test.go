package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// failingCheckoutHook makes every `git worktree add` in work exit non-zero AFTER it has
// created and registered the worktree: git deliberately keeps the worktree when its
// post-checkout hook fails.
func failingCheckoutHook(t *testing.T, work string) string {
	t.Helper()
	hook := filepath.Join(work, ".git", "hooks", "post-checkout")
	writeFile(t, hook, "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	return hook
}

// regression: #2746
//
// A failed `git worktree add` must not leave its target directory or its registration
// behind, in ANY of the three arms (new branch, detached, pinned commit): the next add of
// the same name would otherwise die with "target already exists".
func TestFailedAddLeavesNoTarget(t *testing.T) {
	for name, extra := range map[string][]string{
		"branch":   nil,
		"detached": {"--detach"},
		"pinned":   {"--upstream", "refs/remotes/origin/main"},
	} {
		t.Run(name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			hook := failingCheckoutHook(t, work)
			wtName := "failing"
			args := []string{"add", wtName}
			if name == "pinned" {
				args = append(args, "--base", mustGit(t, work, "rev-parse", "HEAD"))
			}
			args = append(args, extra...)

			rc, errOut := runCapErr(t, args)
			if rc != deskkit.ExitUnverifiable {
				t.Fatalf("add with a failing checkout hook rc = %d, want 6; stderr:\n%s", rc, errOut)
			}
			target := filepath.Join(tmpBaseDir, "tracker-"+wtName)
			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				t.Fatalf("failed add left %s behind (err=%v)", target, err)
			}
			if list := mustGit(t, work, "worktree", "list", "--porcelain"); strings.Contains(list, target) {
				t.Fatalf("failed add left %s registered:\n%s", target, list)
			}
			if !strings.Contains(errOut, "ROLLED BACK") {
				t.Errorf("the failure does not say the target was rolled back:\n%s", errOut)
			}

			// The corrected re-run of the SAME name succeeds: no "target already exists".
			if err := os.Remove(hook); err != nil {
				t.Fatal(err)
			}
			if rc, errOut := runCapErr(t, args); rc != deskkit.ExitOK {
				t.Fatalf("re-add of the same name rc = %d, want 0; stderr:\n%s", rc, errOut)
			}
		})
	}
}

// A concurrent add of the same name that won the race owns the target: git's own "already"
// refusal must never trigger a rollback of THAT worktree.
func TestRollbackSkipsForeignTarget(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	wtName := "winner"
	if rc := run([]string{"add", wtName}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-"+wtName)
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	if note := rollbackFailedAdd(guard, work, target, "fatal: '"+target+"' already exists"); note != "" {
		t.Fatalf("rollback acted on a target another add owns: %q", note)
	}
	if _, err := os.Lstat(target); err != nil {
		t.Fatalf("the winner's worktree was removed: %v", err)
	}
}
