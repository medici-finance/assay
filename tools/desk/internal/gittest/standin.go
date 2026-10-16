package gittest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The no-child harness: proves a local-origin fetch starts no process by OBSERVING THE
// PROCESS, not a seam of the code under test. go-git's stock local ("file") transport starts
// the git binary's upload-pack helper from PATH (falling back to `git` itself) with this
// process's whole environment; gitcore replaces that transport with an in-process one. A
// test stands those programs in first on PATH, sets an environment a child git would honour,
// runs the verb, and asserts the stand-in log is absent.

// StandInLocalTransport writes recording stand-ins for the programs go-git's stock local
// transport starts — the upload-pack helper always, and `git` itself when withGit is true —
// into a fresh directory put first on PATH, and returns the log each appends its argv and
// environment to. A stand-in exits 1, so a transport that starts one also fails, but the log
// is the assertion. A caller whose verb runs `git` legitimately outside the fetch transport
// (a kill-switch read at start-up, say) passes withGit=false.
func StandInLocalTransport(t *testing.T, withGit bool) (logPath string) {
	t.Helper()
	bin := t.TempDir()
	logPath = filepath.Join(t.TempDir(), "children.log")
	script := "#!/bin/sh\n{ echo \"STARTED $0 $*\"; env; } >>'" + logPath + "'\nexit 1\n"
	names := []string{"git-upload-pack"}
	if withGit {
		names = append(names, "git")
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// HostileGitEnv sets environment-supplied git configuration and a program-naming variable
// that a child git would honour, so a child that does start is started under the conditions
// the in-process transport exists to make irrelevant.
func HostileGitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.sshCommand")
	t.Setenv("GIT_CONFIG_VALUE_0", "/nonexistent/should-not-run")
	t.Setenv("GIT_SSH_COMMAND", "/nonexistent/should-not-run")
}

// AssertNoChild fails the test when any stand-in from StandInLocalTransport ran, naming the
// first start. Call it BEFORE checking the verb's own error: with a child started, the error
// is a symptom and the start is the finding.
func AssertNoChild(t *testing.T, logPath, what string) {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("%s: read stand-in log: %v", what, err)
	}
	first, _, _ := strings.Cut(string(b), "\n")
	t.Fatalf("%s started a child process (%s) — a local-origin fetch must run in-process", what, first)
}
