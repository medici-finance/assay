package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPerfProcessCoverage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixtures; raw runner is platform-independent")
	}
	perfFixture(t)
	perfResetCounters()
	raw, err := OutputProcess(exec.Command("/bin/sh", "-c", "printf '  raw bytes \\n';printf 'child diagnosis\\n' >&2;exit 5"))
	var exit *exec.ExitError
	if string(raw) != "  raw bytes \n" || !errors.As(err, &exit) || exit.ExitCode() != 5 || string(exit.Stderr) != "child diagnosis\n" {
		t.Fatalf("Output contract changed: %q %#v", raw, err)
	}
	r := Run(ToolCall{Name: "/bin/sh", Args: []string{"-c", "printf ok;exit 6"}})
	if r.ExitCode != 6 || r.Stdout != "ok" || !errors.As(r.Err, &exit) {
		t.Fatalf("shared runner changed: %+v", r)
	}
	if err := RunProcess(exec.Command(filepath.Join(t.TempDir(), "absent"))); err == nil {
		t.Fatal("missing child became success")
	}
	if _, err := verifierGitBytes(t.TempDir(), "--version"); err != nil {
		t.Fatal(err)
	}
	if ran, err := (Hooks{BeforeRun: "exit 0"}).Run(HookBeforeRun, HookEnv{}); !ran || err != nil {
		t.Fatal(ran, err)
	}
	// Real production minter (no credential): a PATH shim only prints a fixture path.
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "desktoken"), []byte("#!/bin/sh\nprintf '/fixture-token-path\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", path+string(os.PathListSeparator)+os.Getenv("PATH"))
	token, _, err := tokenMinter("worker", "example-org/tracker")
	if err != nil || token != "/fixture-token-path" {
		t.Fatalf("minter behavior changed %q %v", token, err)
	}
	children, wall, _ := PerfCounters()
	if children != 6 || wall <= 0 {
		t.Fatalf("child attempts=%d wall=%v, want exactly six (including failed start)", children, wall)
	}
}
