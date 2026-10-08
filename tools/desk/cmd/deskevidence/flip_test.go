package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// flip_test.go — `deskevidence flip` (#2074). The statusgen half (stamp
// derivation and its refusals) is tested in statusgen/verifyflip_test.go; these
// tests pin the tree-level checks and the commit identity, against a real git
// checkout and the REAL publish-identity gate.

const (
	flReadme  = "docs/streams/vf/README.md"
	flRowImpl = "| 01 | [flip](brief-01-flip.md) | implemented |  | 2026-08-01 human:pat |"
	flStamp   = "2026-08-13 assay-verifier-app[bot] @ b988d175ab12"
	flRowVer  = "| 01 | [flip](brief-01-flip.md) | verified | " + flStamp + " | 2026-08-01 human:pat |"
)

func flReadmeBody(row string) string {
	return "# vf\n\n| # | Brief | Status | Verified | Reviewed |\n|---|---|---|---|---|\n" + row + "\n"
}

// flGit runs git in root, failing the test on error.
func flGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// flRoot builds a checkout whose origin/main carries the implemented row, with
// a flip branch checked out at it.
func flRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	flGit(t, root, "init", "-q", "-b", "main")
	flGit(t, root, "config", "commit.gpgsign", "false")
	flGit(t, root, "config", "user.email", "seed@example.org")
	flGit(t, root, "config", "user.name", "Seed")
	abs := filepath.Join(root, flReadme)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(flReadmeBody(flRowImpl)), 0o644); err != nil {
		t.Fatal(err)
	}
	flGit(t, root, "add", ".")
	flGit(t, root, "commit", "-q", "-m", "init")
	flGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	flGit(t, root, "checkout", "-q", "-b", "verify-flip")
	// The worktree's own user.* is NOT the verifier — the flip must not inherit it.
	flGit(t, root, "config", "user.email", issueLoopBotEmail)
	flGit(t, root, "config", "user.name", "Assay Issue Loop")
	return root
}

// flSetup stubs statusgen (verifyflip writes the one-line flip) and the
// closure check, and restores the real publish-identity gate.
func flSetup(t *testing.T) {
	t.Helper()
	setupFake(t)
	useRealPublishIdentityGate(t)
	oldFlip, oldClose, oldLint := verifyFlipFn, checkVerifiedFn, statusgenLintFn
	t.Cleanup(func() { verifyFlipFn, checkVerifiedFn, statusgenLintFn = oldFlip, oldClose, oldLint })
	verifyFlipFn = func(root, brief, sha, runner string, dry bool) (string, string, error) {
		if runner != "assay-verifier-app[bot]" {
			t.Errorf("verifyflip --runner = %q, want the verifier bot login", runner)
		}
		if !dry {
			if err := os.WriteFile(filepath.Join(root, flReadme), []byte(flReadmeBody(flRowVer)), 0o644); err != nil {
				return "", "", err
			}
		}
		return flReadme, flStamp, nil
	}
	checkVerifiedFn = func(string, string) (string, error) { return flReadme, nil }
	statusgenLintFn = func(string) ([]string, error) { return []string{"PROBLEM: pre-existing"}, nil }
}

func flRun(root string, extra ...string) int {
	return run(append([]string{"flip", "--root", root, "--brief", "vf/01", "--sha", "b988d175ab12"}, extra...))
}

// flUnchanged asserts nothing was committed and the README is the base row.
func flUnchanged(t *testing.T, root string) {
	t.Helper()
	if n := flGit(t, root, "rev-list", "--count", "refs/remotes/origin/main..HEAD"); n != "0" {
		t.Fatalf("a refused flip left %s commit(s) on the branch", n)
	}
	if st := flGit(t, root, "status", "--porcelain"); st != "" {
		t.Fatalf("a refused flip left the index/tree dirty:\n%s", st)
	}
	b, _ := os.ReadFile(filepath.Join(root, flReadme))
	if string(b) != flReadmeBody(flRowImpl) {
		t.Fatalf("a refused flip left the README changed:\n%s", b)
	}
}

func TestFlipCommitsAsVerifier(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	if code := flRun(root); code != deskkit.ExitOK {
		t.Fatalf("flip rc = %d, want 0", code)
	}
	id := flGit(t, root, "log", "-1", "--format=%an|%ae|%cn|%ce")
	want := "assay-verifier-app[bot]|" + verifierBotEmail + "|assay-verifier-app[bot]|" + verifierBotEmail
	if id != want {
		t.Fatalf("flip commit identity = %q, want %q", id, want)
	}
	if files := flGit(t, root, "diff", "--name-only", "refs/remotes/origin/main..HEAD"); files != flReadme {
		t.Fatalf("flip commit touches %q, want only %s", files, flReadme)
	}
	// The PR that carries it meets deskpr's gate UNCHANGED: the real check, role verifier.
	err := deskkit.PublishIdentityMatchesRole(deskkit.PublishIdentityInput{
		Dir: root, Base: "main", RemoteTip: "", Role: "verifier"})
	if err != nil {
		t.Fatalf("the flip commit fails the publish-identity gate: %v", err)
	}
}

func TestFlipDryRunNoWrite(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	if code := flRun(root, "--dry-run"); code != deskkit.ExitOK {
		t.Fatalf("dry-run rc = %d, want 0", code)
	}
	flUnchanged(t, root)
}

func TestFlipRefusesMain(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	flGit(t, root, "checkout", "-q", "main")
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip on main rc = %d, want 5", code)
	}
	flUnchanged(t, root)
}

func TestFlipRefusesDetached(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	flGit(t, root, "checkout", "-q", "--detach")
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip on a detached HEAD rc = %d, want 5", code)
	}
	flUnchanged(t, root)
}

func TestFlipRefusesStaged(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flGit(t, root, "add", "other.txt")
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip with a staged change rc = %d, want 5", code)
	}
}

func TestFlipRefusesDirtyReadme(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	dirty := flReadmeBody(flRowImpl) + "\nlocal edit\n"
	abs := filepath.Join(root, flReadme)
	if err := os.WriteFile(abs, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip over a dirty README rc = %d, want 5", code)
	}
	if b, _ := os.ReadFile(abs); string(b) != dirty {
		t.Fatalf("a refused flip rewrote the dirty README")
	}
}

func TestFlipRefusesForeignBranch(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	if err := os.WriteFile(filepath.Join(root, "x.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flGit(t, root, "add", "x.txt")
	flGit(t, root, "commit", "-q", "-m", "foreign") // the worktree's issue-loop user.*
	inner, calls := verifyFlipFn, 0
	verifyFlipFn = func(r, b, s, u string, d bool) (string, string, error) {
		calls++
		return inner(r, b, s, u, d)
	}
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip on a branch carrying a foreign commit rc = %d, want 5", code)
	}
	if calls != 0 {
		t.Fatalf("the branch gate ran after statusgen (%d call(s)) — it must refuse before any write", calls)
	}
	if n := flGit(t, root, "rev-list", "--count", "refs/remotes/origin/main..HEAD"); n != "1" {
		t.Fatalf("a refused flip committed anyway (%s ahead)", n)
	}
}

func TestFlipStatusgenRefusal(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	verifyFlipFn = func(string, string, string, string, bool) (string, string, error) {
		return "", "", deskkit.Refused("flip: refusing: sha mismatch")
	}
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip after a statusgen refusal rc = %d, want 5", code)
	}
	flUnchanged(t, root)
}

func TestFlipRefusesLintProblem(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	// The after-lint sees the COMMITTED flip: a gain there takes the commit back.
	statusgenLintFn = func(r string) ([]string, error) {
		if flGit(t, r, "rev-list", "--count", "refs/remotes/origin/main..HEAD") == "0" {
			return []string{"PROBLEM: pre-existing"}, nil
		}
		return []string{"PROBLEM: pre-existing", "PROBLEM: vf/01 — Verified cell gained human:pat"}, nil
	}
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip introducing a PROBLEM rc = %d, want 5", code)
	}
	flUnchanged(t, root)
}

func TestFlipRefusesClosure(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	checkVerifiedFn = func(string, string) (string, error) {
		return "", deskkit.Refused("verified outcome refused: no witness")
	}
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip without a verified closure rc = %d, want 5", code)
	}
	flUnchanged(t, root)
}

func TestFlipRefusesExtraDiff(t *testing.T) {
	flSetup(t)
	root := flRoot(t)
	verifyFlipFn = func(r, _, _, _ string, dry bool) (string, string, error) {
		if !dry {
			body := strings.Replace(flReadmeBody(flRowVer), "# vf", "# vf edited", 1)
			if err := os.WriteFile(filepath.Join(r, flReadme), []byte(body), 0o644); err != nil {
				return "", "", err
			}
		}
		return flReadme, flStamp, nil
	}
	if code := flRun(root); code != deskkit.ExitRefused {
		t.Fatalf("flip whose write changes two lines rc = %d, want 5", code)
	}
	flUnchanged(t, root)
}

func TestFlipHelpExitsZero(t *testing.T) {
	flSetup(t)
	for _, args := range [][]string{{"flip", "--help"}, {"flip", "--root", ".", "-h"}} {
		if code := run(args); code != deskkit.ExitOK {
			t.Fatalf("%v rc = %d, want 0", args, code)
		}
	}
}
