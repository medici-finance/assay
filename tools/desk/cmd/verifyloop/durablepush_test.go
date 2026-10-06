package main

// durablepush_test.go — goldens for verifyloop's in-process durable push (pushHeadToMain),
// against a LOCAL bare remote only.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// durableFixture: a bare remote holding main, and a clone of it to commit Evidence in.
func durableFixture(t *testing.T) (work, bare string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	seed, bare, work := filepath.Join(root, "seed"), filepath.Join(root, "origin.git"), filepath.Join(root, "work")
	gitT(t, root, "init", "-q", "-b", "main", seed)
	gitT(t, seed, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "seed")
	gitT(t, root, "clone", "-q", "--bare", seed, bare)
	gitT(t, root, "clone", "-q", bare, work)
	return work, bare
}

func commitIn(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", name)
	gitT(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", name)
}

func localEndpoint(bare string) func(string) (deskkit.ForgeGitEndpoint, error) {
	return func(string) (deskkit.ForgeGitEndpoint, error) {
		ep := deskkit.ForgeGitEndpoint{Kind: deskkit.ForgeGitHub}
		ep.Opts.URL = bare
		return ep, nil
	}
}

func execRun(args ...string) (string, error) {
	b, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	return string(b), err
}

// TestDurablePushLandsMain — a fast-forward Evidence commit lands on the remote's main.
func TestDurablePushLandsMain(t *testing.T) {
	work, bare := durableFixture(t)
	commitIn(t, work, "evidence.md")
	if err := pushHeadToMain(work, localEndpoint(bare), execRun); err != nil {
		t.Fatalf("durable push: %v", err)
	}
	if got, want := gitT(t, bare, "rev-parse", "refs/heads/main"), gitT(t, work, "rev-parse", "HEAD"); got != want {
		t.Fatalf("remote main = %s, want %s", got, want)
	}
}

// TestDurablePushRaceRejected — main moved under us: the push is REJECTED (the race the
// retry loop resolves), never forced over the other writer's commit.
func TestDurablePushRaceRejected(t *testing.T) {
	work, bare := durableFixture(t)
	other := filepath.Join(t.TempDir(), "other")
	gitT(t, filepath.Dir(other), "clone", "-q", bare, other)
	commitIn(t, other, "theirs.md")
	gitT(t, other, "push", "-q", "origin", "HEAD:main")
	theirs := gitT(t, bare, "rev-parse", "refs/heads/main")

	commitIn(t, work, "ours.md")
	err := pushHeadToMain(work, localEndpoint(bare), execRun)
	if err == nil || !strings.Contains(err.Error(), "non-fast-forward") {
		t.Fatalf("racing push = %v, want a non-fast-forward rejection", err)
	}
	if got := gitT(t, bare, "rev-parse", "refs/heads/main"); got != theirs {
		t.Fatalf("remote main moved to %s on a rejected push; the other writer's %s was overwritten", got, theirs)
	}
}

// TestDurablePushHookRefuses — the checkout's pre-push hook still guards the durable push.
func TestDurablePushHookRefuses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh hook fixture")
	}
	work, bare := durableFixture(t)
	before := gitT(t, bare, "rev-parse", "refs/heads/main")
	hook := filepath.Join(work, ".git", "hooks", "pre-push")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	commitIn(t, work, "evidence.md")
	if err := pushHeadToMain(work, localEndpoint(bare), execRun); err == nil {
		t.Fatal("a refusing pre-push hook did not stop the durable push")
	}
	if got := gitT(t, bare, "rev-parse", "refs/heads/main"); got != before {
		t.Fatalf("remote main moved past a refusing hook: %s", got)
	}
}
