// Package deskfilestub keeps a package's tests from reaching the real
// `deskfile`. The comms tools file issues by running `deskfile new` from PATH,
// and a test run from a desk shell has the real verb, a role token and a
// default repo to hand, so an unguarded test that drives a filing path would
// open a real issue.
package deskfilestub

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// marker is in every refusal the stub prints, so a test can tell the stub from
// the real verb.
const marker = "deskfilestub: a test reached deskfile without installing its own fake"

// Install puts a `deskfile` that refuses every call at the front of PATH and
// returns a function that restores PATH and removes the stub. A test that needs
// a filing to succeed installs its own fake in front of the stub with
// t.Setenv("PATH", ...).
func Install() (restore func(), err error) {
	dir, err := os.MkdirTemp("", "deskfilestub")
	if err != nil {
		return nil, err
	}
	name, body := "deskfile", "#!/bin/sh\necho \"refused: "+marker+"\" >&2\nexit 5\n"
	if runtime.GOOS == "windows" {
		name, body = "deskfile.bat", "@echo refused: "+marker+" 1>&2\r\n@exit /b 5\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	old, had := os.LookupEnv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+old); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return func() {
		if had {
			_ = os.Setenv("PATH", old)
		} else {
			_ = os.Unsetenv("PATH")
		}
		_ = os.RemoveAll(dir)
	}, nil
}

// Run installs the stub, runs the package's tests, and restores PATH. A
// package's TestMain calls it in place of m.Run and hands the result to
// os.Exit.
func Run(m *testing.M) int {
	restore, err := Install()
	if err != nil {
		fmt.Fprintf(os.Stderr, "deskfilestub: cannot install the deskfile stub: %v\n", err)
		return 1
	}
	defer restore()
	return m.Run()
}

// AssertStubbed fails t unless `deskfile` on PATH is the stub Run installed.
// One call per guarded package pins that its TestMain goes through Run.
func AssertStubbed(t *testing.T) {
	t.Helper()
	out, err := exec.Command("deskfile", "new").CombinedOutput()
	if err == nil || !strings.Contains(string(out), marker) {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		t.Fatalf("deskfile on PATH is not the test stub (err=%v, last output line %q): route this package's TestMain through deskfilestub.Run", err, lines[len(lines)-1])
	}
}
