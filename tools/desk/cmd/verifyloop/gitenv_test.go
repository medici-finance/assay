package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitEnvOrigin is the origin URL every gitEnvRepo carries in its own config.
const gitEnvOrigin = "https://github.com/example-org/example-repo.git"

// gitEnvProbe runs real git in its cwd under the environment it was started with: a read
// of a key that is present, a --worktree write and a --list must all succeed. It then
// prints the read value, the declared variables, and the names of any variable a verifier
// launch must remove that is still set (empty or not).
const gitEnvProbe = `u=$(git config --get remote.origin.url) || { echo "get-failed"; exit 1; }
git config --worktree verify.probe ok || { echo "worktree-write-failed"; exit 1; }
git config --list >/dev/null || { echo "list-failed"; exit 1; }
leak=$(env | grep -E '^(GIT_CONFIG|GIT_CONFIG_PARAMETERS|GIT_DIR|GIT_WORK_TREE|GIT_COMMON_DIR|GIT_SSH_COMMAND|GIT_ASKPASS|SSH_ASKPASS)=' | cut -d= -f1 | sort | tr '\n' ' ')
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

// inheritBadGitEnv plants, in the environment a verifier launch inherits, every variable
// the launch must remove or override. Each one breaks gitEnvProbe or shows in its output
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
