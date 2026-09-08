package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// These tests exercise the operator-supplied dry-run home. They use a REAL git repo and a
// REAL `git worktree add` under a temp-relocated sanctioned prefix, because the validation's
// whole point is that a stated path is a genuine worktree of the item's own repo — a stub
// that merely echoes canned git output could not prove the checks reject a plain directory
// or a clone of another repo.

// ddGit runs a real git command for a fixture and fails the test on error.
func ddGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// ddRealRepo builds a scratch main checkout on `main`, one commit deep, whose origin URL
// parses to the allowed repo and whose refs/remotes/origin/main resolves locally.
func ddRealRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	ddGit(t, "", "init", "-b", "main", root)
	ddGit(t, root, "config", "user.email", "t@e.st")
	ddGit(t, root, "config", "user.name", "Test")
	ddGit(t, root, "config", "commit.gpgsign", "false")
	ddGit(t, root, "remote", "add", "origin", "git@github.com:medici-finance/assay.git")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ddGit(t, root, "add", "README.md")
	ddGit(t, root, "commit", "-m", "init")
	sha := ddGit(t, root, "rev-parse", "HEAD")
	ddGit(t, root, "update-ref", "refs/remotes/origin/main", sha)
	return root
}

// ddWorktreeEnv points HOME + roster at a private dir, relocates the sanctioned prefix at a
// temp dir, and installs a recording seam that DELEGATES to real git. It returns the
// relocated prefix and the recorded-argv slice.
func ddWorktreeEnv(t *testing.T) (tmpBase string, calls *[][]string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "deskdispatch-test")
	t.Setenv("CLAUDE_SESSION_ID", "deskdispatch-test")

	oldTmp := worktreeTmpBase
	tmpBase = filepath.Join(t.TempDir(), "wtbase")
	if err := os.MkdirAll(tmpBase, 0o755); err != nil {
		t.Fatalf("mkdir tmpBase: %v", err)
	}
	t.Cleanup(func() { worktreeTmpBase = oldTmp })
	worktreeTmpBase = tmpBase

	calls = &[][]string{}
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return old(name, args...)
	}
	t.Cleanup(func() { execCommand = old })
	return tmpBase, calls
}

// ddRunCapture captures stdout (the PLAN banner and, with no --prompt-file, the prompt),
// stderr (the refusal message run() prints), and the exit code of one run.
func ddRunCapture(t *testing.T, args []string) (out, errOut string, rc int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	rc = run(args)
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	bOut, _ := io.ReadAll(rOut)
	bErr, _ := io.ReadAll(rErr)
	return string(bOut), string(bErr), rc
}

// TestDryRunWorktreeRendersVerifiedPath — a real worktree of the item's own repo, under the
// sanctioned prefix, is rendered at BOTH placeholder sites, with no placeholder left, and
// the banner records that the path was verified rather than predicted.
func TestDryRunWorktreeRendersVerifiedPath(t *testing.T) {
	tmpBase, _ := ddWorktreeEnv(t)
	root := ddRealRepo(t)
	plantScripts(t, root)

	target := filepath.Join(tmpBase, "tracker-op")
	ddGit(t, root, "worktree", "add", "--detach", target, "HEAD")
	want := resolvePath(target)

	out, _, rc := ddRunCapture(t, []string{"item-1", "--root", root, "--repo", allowedRepo,
		"--dry-run", "--worktree", target})
	if rc != deskkit.ExitOK {
		t.Fatalf("dry-run --worktree rc = %d, want 0; out:\n%s", rc, out)
	}
	// Both placeholder sites (the home-worktree line and the recreate-worktree command) carry
	// the verified path.
	if n := strings.Count(out, want); n < 2 {
		t.Errorf("verified path %q appears %d time(s), want >= 2 (home line + recreate command)\n%s", want, n, out)
	}
	if strings.Contains(out, homeUnknown) {
		t.Errorf("the placeholder is still present though a verified path was supplied:\n%s", out)
	}
	if !strings.Contains(out, "operator-supplied, verified") {
		t.Errorf("the PLAN banner does not record the path as operator-supplied, verified:\n%s", out)
	}
}

// TestDryRunWorktreeRefusesUnverifiablePaths — the four NEGATIVE cases each exit 5 with their
// own reason and print no prompt. A stub could not prove these; real worktrees can.
func TestDryRunWorktreeRefusesUnverifiablePaths(t *testing.T) {
	tmpBase, _ := ddWorktreeEnv(t)
	root := ddRealRepo(t)
	plantScripts(t, root)

	// A second, unrelated real repo with its own worktree under the prefix — for the
	// "different repo" case.
	other := ddRealRepo(t)
	otherWT := filepath.Join(tmpBase, "tracker-other")
	ddGit(t, other, "worktree", "add", "--detach", otherWT, "HEAD")

	// A plain (non-worktree) directory under the prefix.
	plain := filepath.Join(tmpBase, "tracker-plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	// A directory outside every sanctioned prefix.
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		path       string
		wantReason string
	}{
		{"outside-prefix", outside, "outside the sanctioned worktree prefixes"},
		{"not-a-worktree", plain, "not a registered git worktree"},
		{"other-repo", otherWT, "worktree of a DIFFERENT repo"},
		{"shared-checkout", root, "the isolation floor"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, errOut, rc := ddRunCapture(t, []string{"item-1", "--root", root, "--repo", allowedRepo,
				"--dry-run", "--worktree", c.path})
			if rc != deskkit.ExitRefused {
				t.Fatalf("%s: rc = %d, want 5 (refused); out:\n%s\nstderr:\n%s", c.name, rc, out, errOut)
			}
			// Each case is refused for its OWN reason — this is what makes the row catch a
			// clone of another repo passing validation, or validation loosened to "directory
			// exists": both would refuse for a DIFFERENT reason, or not at all.
			if !strings.Contains(errOut, c.wantReason) {
				t.Errorf("%s: refusal did not name %q\nstderr:\n%s", c.name, c.wantReason, errOut)
			}
			if strings.Contains(out, "# Assignment") {
				t.Errorf("%s: a prompt was printed despite the refusal:\n%s", c.name, out)
			}
			if strings.Contains(out, homeUnknown) {
				t.Errorf("%s: the placeholder prompt was printed despite the refusal:\n%s", c.name, out)
			}
		})
	}
}

// TestWorktreeFlagRefusedOnRealDispatch — --worktree without --dry-run is refused (exit 5)
// and NOTHING ran: the flag must never reach a real dispatch, where the home is deskwt's to
// name.
func TestWorktreeFlagRefusedOnRealDispatch(t *testing.T) {
	tmpBase, calls := ddWorktreeEnv(t)
	root := ddRealRepo(t)
	plantScripts(t, root)
	target := filepath.Join(tmpBase, "tracker-op")
	ddGit(t, root, "worktree", "add", "--detach", target, "HEAD")
	// The fixture's own git ran; the recorder must show ZERO processes from the dispatch
	// itself, so reset after the fixture is built.
	*calls = nil

	out, _, rc := ddRunCapture(t, []string{"item-1", "--root", root, "--repo", allowedRepo,
		"--worktree", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("real dispatch --worktree rc = %d, want 5 (refused); out:\n%s", rc, out)
	}
	if len(*calls) != 0 {
		t.Fatalf("a real dispatch with --worktree ran %d child process(es): %v", len(*calls), *calls)
	}
	if strings.Contains(out, "# Assignment") {
		t.Errorf("a prompt was emitted for a refused real dispatch:\n%s", out)
	}
}
