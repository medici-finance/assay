package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #4823
// TestVerifierGitEnvKeys: every variable a verifier launch removes is absent from the
// result (never present with an empty value), each declared variable appears exactly once
// with its declared value, and unrelated variables pass through.
func TestVerifierGitEnvKeys(t *testing.T) {
	base := []string{
		"PATH=/bin", "GIT_CONFIG=", "GIT_CONFIG=/operator", "GIT_CONFIG_PARAMETERS='a.b'='c'",
		"GIT_DIR=/x", "GIT_WORK_TREE=/x", "GIT_COMMON_DIR=/x", "GIT_SSH_COMMAND=ssh -i k",
		"GIT_ASKPASS=/p", "SSH_ASKPASS=/p", "GIT_CONFIG_GLOBAL=/operator", "GIT_CONFIG_NOSYSTEM=",
		"GIT_CONFIG_COUNT=2", "GIT_TERMINAL_PROMPT=1",
	}
	got := VerifierGitEnv(base)
	seen := map[string][]string{}
	for _, kv := range got {
		k, v, _ := strings.Cut(kv, "=")
		seen[k] = append(seen[k], v)
	}
	for _, k := range []string{"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_DIR", "GIT_WORK_TREE",
		"GIT_COMMON_DIR", "GIT_SSH_COMMAND", "GIT_ASKPASS", "SSH_ASKPASS"} {
		if v, ok := seen[k]; ok {
			t.Errorf("%s survives the verifier launch env as %q; it must be removed", k, v)
		}
	}
	for k, v := range map[string]string{"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_COUNT": "0", "GIT_TERMINAL_PROMPT": "0", "PATH": "/bin"} {
		if len(seen[k]) != 1 || seen[k][0] != v {
			t.Errorf("%s = %q, want exactly [%q]", k, seen[k], v)
		}
	}
}

// regression: #4823
// TestVerifierGitEnvRealGit: under VerifierGitEnv of an environment that plants every
// variable the launch removes, real git reads a key that is present in the repository's
// own config, writes --worktree config, and lists the configuration.
func TestVerifierGitEnvRealGit(t *testing.T) {
	const origin = "https://github.com/example-org/example-repo.git"
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "remote.origin.url", origin}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	nowhere := filepath.Join(t.TempDir(), "nowhere")
	base := append(os.Environ(), "GIT_CONFIG=", "GIT_DIR="+nowhere, "GIT_WORK_TREE="+nowhere,
		"GIT_CONFIG_PARAMETERS='remote.origin.url'='https://github.com/foreign/repo.git'")
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = VerifierGitEnv(base)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	if got, err := git("config", "--get", "remote.origin.url"); err != nil || got != origin {
		t.Fatalf("git config --get remote.origin.url = %q (%v), want %q", got, err, origin)
	}
	if out, err := git("config", "--worktree", "verify.probe", "ok"); err != nil {
		t.Fatalf("git config --worktree: %v: %s", err, out)
	}
	if out, err := git("config", "--list"); err != nil || !strings.Contains(out, "verify.probe=ok") {
		t.Fatalf("git config --list = %q (%v)", out, err)
	}
}

// TestVerifierGitShell: the shell form removes, then declares, the same variables.
func TestVerifierGitShell(t *testing.T) {
	want := "unset GIT_CONFIG GIT_CONFIG_PARAMETERS GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_SSH_COMMAND " +
		"GIT_ASKPASS SSH_ASKPASS\nexport GIT_CONFIG_GLOBAL=" + os.DevNull +
		" GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_COUNT=0 GIT_TERMINAL_PROMPT=0"
	if got := VerifierGitShell(); got != want {
		t.Fatalf("VerifierGitShell() =\n%s\nwant\n%s", got, want)
	}
}
