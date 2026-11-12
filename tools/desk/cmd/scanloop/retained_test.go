package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// retained_test.go — a coalesce whose carrier branch is still checked out in a scan worktree an
// earlier pass retained. Real git throughout; only statusgen, deskscanbody and deskpr are stubbed,
// and the stubbed deskpr create/update push the branch the way the real verb does.

const heldBranch = "chore/intake-scan-2026-08-24-1140"

// heldGitRun runs real git with a throwaway identity and returns git's error instead of failing the
// test, so a refused `worktree add` reaches the lane as the error it is.
func heldGitRun(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

type heldFixture struct {
	base, origin, root string
	lane               scanCarrierPRLane
	scans              int
}

// newHeldFixture opens the carrier the way a fresh pass does, then checks the carrier branch out in
// a second worktree named leaf, beside the target checkout: the worktree a failed pass retains.
func newHeldFixture(t *testing.T, leaf string) (*heldFixture, string) {
	t.Helper()
	f := &heldFixture{base: t.TempDir()}
	f.origin = filepath.Join(f.base, "origin.git")
	f.root = filepath.Join(f.base, "target")
	scanGit(t, f.base, "init", "--bare", "-b", "main", f.origin)
	scanGit(t, f.base, "clone", f.origin, f.root)
	scanGit(t, f.root, "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(f.root, "seed.txt"), []byte("seed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scanGit(t, f.root, "add", "seed.txt")
	scanGit(t, f.root, "commit", "-m", "seed")
	scanGit(t, f.root, "push", "origin", "main")

	f.lane = scanCarrierPRLane{write: func(string, string) error { return nil }, exec: func(dir, name string, args ...string) (string, error) {
		switch name {
		case "git":
			return heldGitRun(t, dir, args...)
		case "statusgen":
			f.scans++
			d := filepath.Join(dir, deskkit.ScanDir)
			if err := os.MkdirAll(d, 0o755); err != nil {
				return "", err
			}
			return "", os.WriteFile(filepath.Join(d, fmt.Sprintf("delta-%d.md", f.scans)), []byte("x\n"), 0o600)
		case "deskscanbody":
			return "derived\n", nil
		case "deskpr":
			if args[0] == "create" || args[0] == "update" {
				return heldGitRun(t, dir, "push", "origin", "HEAD:refs/heads/"+strings.TrimSpace(scanGit(t, dir, "branch", "--show-current")))
			}
			return "", nil
		}
		return "", fmt.Errorf("unexpected command %s", name)
	}}

	if _, err := f.lane.Execute(f.req(t, "tracker-intake-scan-fresh", nil)); err != nil {
		t.Fatalf("fresh pass: %v", err)
	}
	held := filepath.Join(f.base, leaf)
	scanGit(t, f.root, "worktree", "add", held, heldBranch)
	return f, held
}

func (f *heldFixture) req(t *testing.T, leaf string, open *OpenScanPR) LaneRequest {
	r := laneReq(t, scanItem("medici-finance/assay", string(LaneScanCarrierPR)), open)
	r.Root, r.Worktree, r.DryRun, r.Branch = f.root, filepath.Join(f.base, leaf), false, heldBranch
	return r
}

// coalesce runs one coalescing pass whose OWN worktree path is new, as worktreeFor's per-pass path
// always is.
func (f *heldFixture) coalesce(t *testing.T) (LaneOutcome, error) {
	return f.lane.Execute(f.req(t, "tracker-intake-scan-next", openPR(3*time.Minute)))
}

// regression: #6282
func TestCoalesceReusesCleanRetained(t *testing.T) {
	f, held := newHeldFixture(t, "tracker-intake-scan-retained")
	if _, err := f.coalesce(t); err != nil {
		t.Fatalf("a coalesce refused although the retained scan worktree holding %s is clean and at the "+
			"remote carrier tip: %v", heldBranch, err)
	}
	if got := scanGit(t, f.origin, "ls-tree", "-r", "--name-only", heldBranch); strings.Count(got, "delta-") != 2 {
		t.Fatalf("the carrier should hold the fresh delta plus the coalesced one:\n%s", got)
	}
	if _, err := os.Stat(held); !os.IsNotExist(err) {
		t.Fatalf("the reused worktree %s was not put back at the end of the pass (stat err %v)", held, err)
	}
	if _, err := os.Stat(filepath.Join(f.base, "tracker-intake-scan-next")); !os.IsNotExist(err) {
		t.Fatalf("the pass cut a second worktree although it reused the retained one (stat err %v)", err)
	}
}

// assertHeldUntouched pins that a refused coalesce left the retained worktree exactly as it was:
// still there, still holding the carrier branch, still carrying the marker file.
func assertHeldUntouched(t *testing.T, f *heldFixture, held, marker string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(held, marker)); err != nil {
		t.Fatalf("the refused coalesce disturbed the retained worktree's file %s: %v", marker, err)
	}
	if got := strings.TrimSpace(scanGit(t, held, "branch", "--show-current")); got != heldBranch {
		t.Fatalf("the retained worktree no longer holds %s (it holds %q)", heldBranch, got)
	}
}

// regression: #6282
func TestCoalesceRefusesDirtyHeld(t *testing.T) {
	f, held := newHeldFixture(t, "tracker-intake-scan-retained")
	if err := os.WriteFile(filepath.Join(held, "diagnosis.txt"), []byte("keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := f.coalesce(t)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "uncommitted or untracked") {
		t.Fatalf("err = %v, want a refusal naming the retained worktree's uncommitted changes", err)
	}
	assertHeldUntouched(t, f, held, "diagnosis.txt")
}

// regression: #6282
func TestCoalesceRefusesDivergedHeld(t *testing.T) {
	f, held := newHeldFixture(t, "tracker-intake-scan-retained")
	if err := os.WriteFile(filepath.Join(held, "local.txt"), []byte("unpushed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scanGit(t, held, "add", "local.txt")
	scanGit(t, held, "-c", "user.name=t", "-c", "user.email=t@example.test", "commit", "-m", "unpushed")
	_, err := f.coalesce(t)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "is not the remote carrier tip") {
		t.Fatalf("err = %v, want a refusal naming the retained worktree's diverged HEAD", err)
	}
	assertHeldUntouched(t, f, held, "local.txt")
}

// regression: #6282
func TestCoalesceRefusesForeignHeld(t *testing.T) {
	f, held := newHeldFixture(t, "someones-checkout")
	_, err := f.coalesce(t)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "not a scan worktree") {
		t.Fatalf("err = %v, want a refusal: a worktree this loop did not cut is never reused", err)
	}
	assertHeldUntouched(t, f, held, "seed.txt")
}
