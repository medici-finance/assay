package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitEnvOrigin is the origin URL every gitEnvRepo carries in its own config.
const gitEnvOrigin = "https://github.com/example-org/example-repo.git"

// gitEnvProbe runs real git in its cwd under the environment it was started with: a read
// of a key that is present, a --worktree write and a --list must all succeed. It then
// prints the read value, the declared variables, and the names of every GIT_* variable
// other than the four declared ones, and of SSH_ASKPASS, that is still set (empty or not).
const gitEnvProbe = `u=$(git config --get remote.origin.url) || { echo "get-failed"; exit 1; }
git config --worktree verify.probe ok || { echo "worktree-write-failed"; exit 1; }
git config --list >/dev/null || { echo "list-failed"; exit 1; }
leak=$(env | sed -nE 's/^([Gg][Ii][Tt]_[A-Za-z0-9_]*|SSH_ASKPASS)=.*/\1/p' | grep -vxE 'GIT_CONFIG_(GLOBAL|NOSYSTEM|COUNT)|GIT_TERMINAL_PROMPT' | sort | tr '\n' ' ')
printf '%s;%s;%s;%s;%s;[%s]' "$u" "$GIT_CONFIG_GLOBAL" "$GIT_CONFIG_NOSYSTEM" "$GIT_CONFIG_COUNT" "$GIT_TERMINAL_PROMPT" "$leak"`

// gitEnvWant is what gitEnvProbe prints under the declared verifier launch environment.
var gitEnvWant = gitEnvOrigin + ";" + os.DevNull + ";1;0;0;[]"

// gitEnvRepo makes a repository with one commit and gitEnvOrigin inline in its own config.
// It runs before inheritBadGitEnv, so its own git calls see a clean environment.
func gitEnvRepo(t *testing.T) (dir, head string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
			"-c", "commit.gpgsign=false", "-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	run("config", "remote.origin.url", gitEnvOrigin)
	run("commit", "-q", "--allow-empty", "-m", "fixture")
	return dir, run("rev-parse", "HEAD")
}

// inheritBadGitEnv plants, in the environment a verifier launch inherits, variables the
// launch must remove or override: the ones a named list once covered, the ones it missed,
// and an arbitrary GIT_FOO. Each one breaks gitEnvProbe or shows in its output
// if it survives: an empty GIT_CONFIG blinds `git config`, GIT_DIR and GIT_WORK_TREE point
// git away from the repository, GIT_CONFIG_PARAMETERS rewrites the origin URL, and the
// rest are reported by name or value.
func inheritBadGitEnv(t *testing.T) {
	t.Helper()
	nowhere := filepath.Join(t.TempDir(), "nowhere")
	t.Setenv("GIT_CONFIG", "")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'remote.origin.url'='https://github.com/foreign/repo.git'")
	t.Setenv("GIT_DIR", nowhere)
	t.Setenv("GIT_WORK_TREE", nowhere)
	t.Setenv("GIT_COMMON_DIR", nowhere)
	t.Setenv("GIT_SSH_COMMAND", "false")
	t.Setenv("GIT_ASKPASS", "false")
	t.Setenv("SSH_ASKPASS", "false")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "operator.gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	// Not on any named list: every inherited GIT_* variable is removed, so each of these is
	// reported by name if it survives, and the redirecting ones also break the probe.
	t.Setenv("GIT_OBJECT_DIRECTORY", nowhere)
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", nowhere)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(nowhere, "index"))
	t.Setenv("GIT_NAMESPACE", "example-ns")
	t.Setenv("GIT_SSH", "false")
	t.Setenv("GIT_EXEC_PATH", nowhere)
	t.Setenv("GIT_SSL_NO_VERIFY", "1")
	t.Setenv("GIT_FOO", "example")
}

// regression: #4823
// TestOwnGitCallsEnv: verifyloop's own git children (deriveRepo, deriveHead and the
// isolated worktree) run under the declared environment, not the inherited one.
func TestOwnGitCallsEnv(t *testing.T) {
	repo, head := gitEnvRepo(t)
	inheritBadGitEnv(t)
	if got := deriveRepo(repo); got != "example-org/example-repo" {
		t.Fatalf("deriveRepo = %q, want example-org/example-repo", got)
	}
	if got := deriveHead(repo); got != head {
		t.Fatalf("deriveHead = %q, want %q", got, head)
	}
	it := fixtureItem()
	it.TargetSHA = head
	it.Payload = map[string]string{"source_root": repo}
	dir, cleanup, err := gitDetachedWorktree(it)
	if err != nil {
		t.Fatalf("gitDetachedWorktree: %v", err)
	}
	defer cleanup()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("isolated worktree not created: %v", err)
	}
}

// regression: #4823
// TestLandingGitEnv: the Evidence landing's own children run under the declared verifier
// Git environment: the origin read, add and commit (through the landing's run), and the
// pre-push hook (its path read and the hook process). The push itself is in-process and is
// stubbed here; its local-path transport is fixture-only.
func TestLandingGitEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh hook fixture")
	}
	work, bare := durableFixture(t)
	gitT(t, work, "config", "user.name", "t")
	gitT(t, work, "config", "user.email", "t@example.invalid")
	seen := filepath.Join(t.TempDir(), "hook-env")
	hook := "#!/bin/sh\n" + gitEnvProbe + " >" + seen + "\n"
	if err := os.WriteFile(filepath.Join(work, ".git", "hooks", "pre-push"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "evidence.md"), []byte("evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inheritBadGitEnv(t)
	g := newGitDurable(work, "example-org/example-repo", "verifier")
	if got, err := g.run("git", "-C", work, "remote", "get-url", "origin"); err != nil || strings.TrimSpace(got) != bare {
		t.Fatalf("landing origin read = %q (%v), want %q", got, err, bare)
	}
	g.push = func() error { return nil }
	if err := g.commitPushRace([]string{"evidence.md"}, "verify(x): evidence"); err != nil {
		t.Fatalf("commitPushRace: %v", err)
	}
	if got := gitT(t, work, "show", "-s", "--format=%s", "HEAD"); got != "verify(x): evidence" {
		t.Fatalf("landing commit subject = %q, want the Evidence commit", got)
	}
	head := gitT(t, work, "rev-parse", "HEAD")
	if err := landingHook(work, g.run).Run("origin", bare, "HEAD", head, "refs/heads/main"); err != nil {
		t.Fatalf("pre-push hook: %v", err)
	}
	got, err := os.ReadFile(seen)
	if err != nil {
		t.Fatalf("pre-push hook left no record: %v", err)
	}
	if want := bare + ";" + os.DevNull + ";1;0;0;[]"; string(got) != want {
		t.Fatalf("pre-push hook Git env = %q, want %q", got, want)
	}
}
