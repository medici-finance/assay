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

// Run puts a `deskfile` that refuses every call at the front of PATH, then runs
// the package's tests. A test that needs a filing to succeed installs its own
// fake in front of the stub with t.Setenv("PATH", ...).
func Run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "deskfilestub")
	if err != nil {
		fmt.Fprintf(os.Stderr, "deskfilestub: cannot create the stub directory: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	name, body := "deskfile", "#!/bin/sh\necho \"refused: "+marker+"\" >&2\nexit 5\n"
	if runtime.GOOS == "windows" {
		name, body = "deskfile.bat", "@echo refused: "+marker+" 1>&2\r\n@exit /b 5\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "deskfilestub: cannot write the stub: %v\n", err)
		return 1
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		fmt.Fprintf(os.Stderr, "deskfilestub: cannot set PATH: %v\n", err)
		return 1
	}
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
