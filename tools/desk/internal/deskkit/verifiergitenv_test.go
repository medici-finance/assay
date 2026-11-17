package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// plantedGitEnv is an inherited environment carrying every kind of Git variable a
// verifier launch must not pass on: the routing and config variables a named list once
// covered, the ones it missed (object store, index, namespace, ssh program, exec path, TLS
// verification), an arbitrary GIT_FOO, a lower-case git_* name, SSH_ASKPASS, and the four
// declared variables with operator values.
var plantedGitEnv = []string{
	"GIT_CONFIG=", "GIT_CONFIG=/operator", "GIT_CONFIG_PARAMETERS='a.b'='c'",
	"GIT_DIR=/x", "GIT_WORK_TREE=/x", "GIT_COMMON_DIR=/x", "GIT_SSH_COMMAND=ssh -i k",
	"GIT_ASKPASS=/p", "SSH_ASKPASS=/p",
	"GIT_OBJECT_DIRECTORY=/x", "GIT_ALTERNATE_OBJECT_DIRECTORIES=/x", "GIT_INDEX_FILE=/x/index",
	"GIT_NAMESPACE=example-ns", "GIT_SSH=/p", "GIT_EXEC_PATH=/x", "GIT_SSL_NO_VERIFY=1",
	"GIT_FOO=example", "git_lower=example",
	"GIT_CONFIG_GLOBAL=/operator", "GIT_CONFIG_NOSYSTEM=", "GIT_CONFIG_COUNT=2", "GIT_TERMINAL_PROMPT=1",
}

// declaredGitEnv is what a verifier launch declares, and so the only Git variables it may carry.
var declaredGitEnv = map[string]string{"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
	"GIT_CONFIG_COUNT": "0", "GIT_TERMINAL_PROMPT": "0"}

// strayGitVars returns, from a KEY=value environment, every GIT_* (case-blind) or
// SSH_ASKPASS entry other than exactly one declared value per declared key.
func strayGitVars(env []string) []string {
	var stray []string
	seen := map[string]int{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(strings.ToUpper(k), "GIT_") && k != "SSH_ASKPASS" {
			continue
		}
		if want, ok := declaredGitEnv[k]; ok && v == want && seen[k] == 0 {
			seen[k]++
			continue
		}
		stray = append(stray, kv)
	}
	for k := range declaredGitEnv {
		if seen[k] == 0 {
			stray = append(stray, k+" (declared value missing)")
		}
	}
	return stray
}

// regression: #4823
// TestVerifierGitEnvKeys: no inherited Git variable survives the verifier launch env (none
// is present with an empty value either), each declared variable appears exactly once with
// its declared value, and unrelated variables pass through.
func TestVerifierGitEnvKeys(t *testing.T) {
	got := VerifierGitEnv(append([]string{"PATH=/bin"}, plantedGitEnv...))
	if stray := strayGitVars(got); len(stray) > 0 {
		t.Errorf("the verifier launch env carries %q; only the declared values may remain", stray)
	}
	if got[0] != "PATH=/bin" {
		t.Errorf("PATH did not pass through: %q", got)
	}
}

// regression: #4823
// TestVerifierGitUnsetNames: the names a remove-only spawn API is given are every planted
// Git variable, so removing them leaves none behind.
func TestVerifierGitUnsetNames(t *testing.T) {
	base := append([]string{"PATH=/bin"}, plantedGitEnv...)
	drop := map[string]bool{}
	for _, k := range VerifierGitUnset(base) {
		drop[k] = true
	}
	var kept []string
	for _, kv := range base {
		if k, _, _ := strings.Cut(kv, "="); !drop[k] {
			kept = append(kept, kv)
		}
	}
	if len(kept) != 1 || kept[0] != "PATH=/bin" {
		t.Fatalf("removing VerifierGitUnset(base) keeps %q, want only PATH", kept)
	}
}

// regression: #4823
// TestVerifierGitShellRuns: a POSIX shell that inherits every planted variable and runs
// VerifierGitShell() hands its children the declared values and no other Git variable.
func TestVerifierGitShellRuns(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no POSIX sh")
	}
	cmd := exec.Command("sh", "-c", VerifierGitShell()+"\nenv")
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, plantedGitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sh: %v\n%s", err, out)
	}
	if stray := strayGitVars(strings.Split(strings.TrimSpace(string(out)), "\n")); len(stray) > 0 {
		t.Fatalf("after VerifierGitShell the shell still exports %q", stray)
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
		"GIT_OBJECT_DIRECTORY="+nowhere, "GIT_INDEX_FILE="+filepath.Join(nowhere, "index"), "GIT_EXEC_PATH="+nowhere,
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

// TestVerifierGitShell: the shell form removes every inherited Git variable by pattern,
// then declares the same variables VerifierGitLaunchEnv does.
func TestVerifierGitShell(t *testing.T) {
	want := `for v in $(env | sed -n 's/^\([Gg][Ii][Tt]_[A-Za-z0-9_]*\)=.*/\1/p') SSH_ASKPASS; do unset "$v"; done` +
		"\nexport GIT_CONFIG_GLOBAL=" + os.DevNull + " GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_COUNT=0 GIT_TERMINAL_PROMPT=0"
	if got := VerifierGitShell(); got != want {
		t.Fatalf("VerifierGitShell() =\n%s\nwant\n%s", got, want)
	}
}
