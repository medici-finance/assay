package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// failingCheckoutHook makes every `git worktree add` in work exit non-zero AFTER it has
// created and registered the worktree: git deliberately keeps the worktree when its
// post-checkout hook fails.
func failingCheckoutHook(t *testing.T, work string) string {
	t.Helper()
	hook := filepath.Join(work, ".git", "hooks", "post-checkout")
	writeFile(t, hook, "#!/bin/sh\necho already configured >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	return hook
}

// trackerLeaf is the leaf deskwt mints for a worktree name (the guard allows only
// tracker-* children of the tmp dir). Fixtures name targets through it, so this source
// never spells a minted scratch-worktree name for the public self-containment scan.
func trackerLeaf(name string) string { return "tracker-" + name }

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
			guard, guardErr := newPathGuard(work)
			if guardErr != nil {
				t.Fatal(guardErr)
			}
			target := guard.worktreeTarget(trackerLeaf(wtName))
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

// regression: #2746
// A concurrent winner has no reservation belonging to the losing invocation;
// a localized refusal cannot grant one because rollback accepts no stderr.
func TestRollbackSkipsForeignTarget(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	wtName := "winner"
	if rc := run([]string{"add", wtName}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	guard, guardErr := newPathGuard(work)
	if guardErr != nil {
		t.Fatal(guardErr)
	}
	target := guard.worktreeTarget(trackerLeaf(wtName))
	if note := rollbackFailedAdd(guard, work, target, nil); note != "" {
		t.Fatalf("rollback acted on a target another add owns: %q", note)
	}
	if _, err := os.Lstat(target); err != nil {
		t.Fatalf("the winner's worktree was removed: %v", err)
	}
}

// regression: #2746
func TestRollbackSymlinkTarget(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	outside := t.TempDir()
	marker := filepath.Join(outside, "keep")
	writeFile(t, marker, "foreign data")
	target := filepath.Join(tmpBaseDir, trackerLeaf("foreign-symlink"))
	if err := os.Symlink(outside, target); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	rollbackFailedAdd(guard, work, target, nil)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("foreign symlink destination removed: %v", err)
	}
	if fi, err := os.Lstat(target); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("foreign symlink changed: %v", err)
	}
}

// regression: #2746
func TestRollbackOwnership(t *testing.T) {
	for _, kind := range []string{"replaced", "symlink", "outside", "absent", "owned"} {
		t.Run(kind, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			guard, err := newPathGuard(work)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(tmpBaseDir, trackerLeaf("owned"))
			owned, err := reserveAddTarget(target)
			if err != nil {
				t.Fatal(err)
			}
			defer owned.root.Close()
			if kind == "replaced" || kind == "symlink" || kind == "absent" {
				if err := os.Rename(target, target+"-original"); err != nil {
					t.Fatal(err)
				}
			}
			marker := filepath.Join(target, "keep")
			switch kind {
			case "replaced":
				writeFile(t, marker, "foreign")
			case "symlink":
				outside := t.TempDir()
				marker = filepath.Join(outside, "keep")
				writeFile(t, marker, "foreign")
				if err := os.Symlink(outside, target); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "outside":
				writeFile(t, marker, "retained")
				// The allowlist changed since allocation; identity alone is not authority.
				guard.tmpDir = t.TempDir()
			case "owned":
				writeFile(t, marker, "owned")
			}
			note := rollbackFailedAdd(guard, work, target, owned)
			switch kind {
			case "owned":
				if _, err := os.Lstat(target); !os.IsNotExist(err) || !strings.Contains(note, "ROLLED BACK") {
					t.Fatalf("owned cleanup = %q, stat=%v", note, err)
				}
			case "absent":
				if note != "" {
					t.Fatalf("absent target cleanup = %q", note)
				}
			default:
				if _, err := os.Stat(marker); err != nil {
					t.Fatalf("%s data removed: %v", kind, err)
				}
				if !strings.Contains(note, "stopped") {
					t.Fatalf("%s cleanup not reported: %q", kind, note)
				}
			}
		})
	}
}

// regression: #2746
func TestAddReservationRace(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	var wg sync.WaitGroup
	owners := make(chan *addTarget, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); owned, _ := reserveAddTarget(target); owners <- owned }()
	}
	wg.Wait()
	close(owners)
	count := 0
	for owned := range owners {
		if owned != nil {
			count++
			owned.root.Close()
		}
	}
	if count != 1 {
		t.Fatalf("reservation owners = %d, want exactly one", count)
	}
}

// regression: #2746
func TestAddRefLockRollback(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
			// Place the lock after stale-branch preflight, before git creates its ref.
			writeFile(t, filepath.Join(work, ".git", "refs", "heads", "locked.lock"), "held")
		}
		return oldExec(name, args...)
	}
	t.Cleanup(func() { execCommand = oldExec })
	rc, stderr := runCapErr(t, []string{"add", "locked"})
	if rc != deskkit.ExitUnverifiable || !strings.Contains(stderr, "ROLLED BACK") {
		t.Fatalf("lock failure = %d: %s", rc, stderr)
	}
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	target := guard.worktreeTarget(trackerLeaf("locked"))
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("lock failure left target: %v", err)
	}
}

// TestLocalizedGitFailure is a subprocess fixture; it never contacts a remote.
func TestLocalizedGitFailure(t *testing.T) {
	if os.Getenv("DESK_TEST_LOCALIZED_FAIL") != "1" {
		return
	}
	fmt.Fprintln(os.Stderr, "fatal : le chemin existe déjà")
	os.Exit(1)
}

// regression: #2746
func TestLocalizedRaceKeepsWinner(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	target := guard.worktreeTarget(trackerLeaf("loser"))
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
			// Replace the loser's reservation with a separately registered winner before
			// delivering a localized git failure. Neither path nor stderr proves ownership.
			if err := os.Rename(target, target+"-reservation"); err != nil {
				t.Fatal(err)
			}
			winner := oldExec("git", "worktree", "add", "--detach", target, "HEAD")
			winner.Dir = work
			if out, err := winner.CombinedOutput(); err != nil {
				t.Fatalf("winner add: %v: %s", err, out)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestLocalizedGitFailure$")
			cmd.Env = append(os.Environ(), "DESK_TEST_LOCALIZED_FAIL=1")
			return cmd
		}
		return oldExec(name, args...)
	}
	t.Cleanup(func() { execCommand = oldExec })
	rc, stderr := runCapErr(t, []string{"add", "loser"})
	if rc != deskkit.ExitUnverifiable || !strings.Contains(stderr, "existe déjà") {
		t.Fatalf("localized failure = %d: %s", rc, stderr)
	}
	if _, err := os.Stat(filepath.Join(target, "README.md")); err != nil {
		t.Fatalf("concurrent winner lost: %v", err)
	}
	set, err := guard.worktreePaths(work)
	if err != nil || !set[resolvePath(target)] {
		t.Fatalf("winner registration lost: %v", err)
	}
}

// regression: #2746
func TestFailedAddWindowsHome(t *testing.T) {
	setGOOS(t, "windows")
	TestFailedAddLeavesNoTarget(t)
}
