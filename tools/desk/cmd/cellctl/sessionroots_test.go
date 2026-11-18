package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func srGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// srWorld is a bare origin with one commit on main and a shared checkout of it that its
// operator has on a branch of their own, with an uncommitted change.
func srWorld(t *testing.T) (origin, shared string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	base := t.TempDir()
	origin = filepath.Join(base, "origin.git")
	seed := filepath.Join(base, "seed")
	srGit(t, base, "init", "-q", "--bare", "-b", "main", origin)
	srGit(t, base, "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srGit(t, seed, "add", "a.txt")
	srGit(t, seed, "commit", "-q", "-m", "one")
	srGit(t, seed, "remote", "add", "origin", origin)
	srGit(t, seed, "push", "-q", "origin", "main")
	shared = filepath.Join(base, "shared")
	srGit(t, base, "clone", "-q", origin, shared)
	srGit(t, shared, "checkout", "-q", "-b", "operator-work")
	if err := os.WriteFile(filepath.Join(shared, "a.txt"), []byte("operator edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return origin, shared
}

// srPushMain lands a new commit on origin's main from a scratch clone and returns its sha.
func srPushMain(t *testing.T, origin, msg string) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "pusher")
	srGit(t, filepath.Dir(tmp), "clone", "-q", origin, tmp)
	if err := os.WriteFile(filepath.Join(tmp, msg+".txt"), []byte(msg+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srGit(t, tmp, "add", ".")
	srGit(t, tmp, "commit", "-q", "-m", msg)
	srGit(t, tmp, "push", "-q", "origin", "main")
	return srGit(t, tmp, "rev-parse", "HEAD")
}

func srCell(t *testing.T) *Cell {
	t.Helper()
	return &Cell{Name: "example-cell", Dir: t.TempDir(), Kind: "house", Env: &Env{vals: map[string]string{}, set: map[string]bool{}, src: map[string]string{}}}
}

// regression: #6814
// A house verify-desk boot points DESK_ROOTS at a per-session tree at origin/main — never at
// the shared checkout — and leaves the shared checkout's branch and edits exactly as they were.
func TestSessionRootsCutAtOriginMain(t *testing.T) {
	origin, shared := srWorld(t)
	want := srPushMain(t, origin, "two") // the shared checkout has not fetched this yet
	c := srCell(t)
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	got := c.prepareSessionRoots("example-org/example-repo=" + shared + ",example-org/plain=" + plain)

	wt := c.sessionRootPath("example-org/example-repo")
	if wantRoots := "example-org/example-repo=" + wt + ",example-org/plain=" + plain; got != wantRoots {
		t.Fatalf("DESK_ROOTS = %q, want %q", got, wantRoots)
	}
	if head := srGit(t, wt, "rev-parse", "HEAD"); head != want {
		t.Errorf("session tree HEAD = %s, want origin/main %s", head, want)
	}
	if main := srGit(t, wt, "rev-parse", "refs/remotes/origin/main"); main != want {
		t.Errorf("refs/remotes/origin/main = %s, want %s", main, want)
	}
	if b := srGit(t, shared, "symbolic-ref", "--short", "HEAD"); b != "operator-work" {
		t.Errorf("shared checkout moved off its branch: %s", b)
	}
	if st := srGit(t, shared, "status", "--porcelain"); !strings.Contains(st, "a.txt") {
		t.Errorf("shared checkout lost its uncommitted change: %q", st)
	}
	if locked := srGit(t, shared, "worktree", "list", "--porcelain"); !strings.Contains(locked, "locked") {
		t.Errorf("session tree is not locked:\n%s", locked)
	}
}

// regression: #6814
// Before every cadence pass the session trees are moved to the freshly fetched origin/main, and
// a reboot reuses the tree instead of failing on it.
func TestSessionRootsRefreshBeforePass(t *testing.T) {
	origin, shared := srWorld(t)
	c := srCell(t)
	roots := "example-org/example-repo=" + shared
	c.prepareSessionRoots(roots)
	wt := c.sessionRootPath("example-org/example-repo")

	next := srPushMain(t, origin, "three")
	if err := c.refreshSessionRootsPass(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if head := srGit(t, wt, "rev-parse", "HEAD"); head != next {
		t.Errorf("after the pass refresh HEAD = %s, want %s", head, next)
	}

	again := srPushMain(t, origin, "four")
	c2 := srCell(t)
	c2.Dir = c.Dir
	c2.prepareSessionRoots(roots)
	if head := srGit(t, wt, "rev-parse", "HEAD"); head != again {
		t.Errorf("reboot left the tree at %s, want %s", head, again)
	}
}

// regression: #6814
// A tree that carries work is never moved: tracked edits and local commits are refused, named.
func TestSessionRootsRefuseLocalWork(t *testing.T) {
	_, shared := srWorld(t)
	c := srCell(t)
	c.prepareSessionRoots("example-org/example-repo=" + shared)
	r := c.sessionRoots[0]

	if err := os.WriteFile(filepath.Join(r.WT, "a.txt"), []byte("verifier edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := refreshSessionRoot(r); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Errorf("dirty tree: err = %v, want an uncommitted-changes refusal", err)
	}
	srGit(t, r.WT, "commit", "-q", "-am", "local")
	local := srGit(t, r.WT, "rev-parse", "HEAD")
	if _, err := refreshSessionRoot(r); err == nil || !strings.Contains(err.Error(), "does not contain") {
		t.Errorf("tree ahead of main: err = %v, want a refusal", err)
	}
	if head := srGit(t, r.WT, "rev-parse", "HEAD"); head != local {
		t.Errorf("refused tree was moved anyway: %s", head)
	}

	stray := srCell(t)
	if err := os.MkdirAll(stray.sessionRootPath("example-org/example-repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := refreshSessionRoot(sessionRoot{Name: "example-org/example-repo", Src: shared, WT: stray.sessionRootPath("example-org/example-repo")})
	if err == nil || !strings.Contains(err.Error(), "not a worktree") {
		t.Errorf("stray directory: err = %v, want a refusal", err)
	}
}

// regression: #6814
// Only a house verify-desk window gets session roots, and the cell can opt out.
func TestSessionRootsScope(t *testing.T) {
	c := srCell(t)
	if !c.wantsSessionRoots("verify-desk") {
		t.Error("house verify-desk: want session roots")
	}
	for _, role := range []string{"the-desk", "worker-desk", "pr-review-desk", "intake-desk"} {
		if c.wantsSessionRoots(role) {
			t.Errorf("%s: session roots are verify-desk only", role)
		}
	}
	c.Kind = "k8s"
	if c.wantsSessionRoots("verify-desk") {
		t.Error("k8s cell: session roots are house only")
	}
	c.Kind = "house"
	c.Env.Put(verifyRootsOptOut, "1")
	if c.wantsSessionRoots("verify-desk") {
		t.Errorf("%s=1 must opt out", verifyRootsOptOut)
	}
	if got := c.sessionRootsPlan("a/b=/x,c/d=/y"); got != "a/b="+c.sessionRootPath("a/b")+",c/d="+c.sessionRootPath("c/d") {
		t.Errorf("plan = %q", got)
	}
}

// regression: #6814
// A pass whose roots cannot be refreshed is could-not-check, and the harness is never started.
func TestCadencePassRefusesStaleRoots(t *testing.T) {
	c := srCell(t)
	c.sessionRoots = []sessionRoot{{Name: "example-org/example-repo", Src: filepath.Join(t.TempDir(), "gone"), WT: filepath.Join(t.TempDir(), "wt")}}
	res := c.executeCadencePass(context.Background(), "verify-desk", "claude", []string{"/nonexistent/harness"}, nil, t.TempDir())
	if res.Outcome != "could-not-check" || res.Err == nil || !strings.Contains(res.Err.Error(), "verify root") {
		t.Fatalf("result = %+v, want could-not-check naming the root", res)
	}
}
