//go:build darwin || linux

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// regression: #6455
func TestShellOracleParity(t *testing.T) {
	t.Parallel()
	suite, err := filepath.Abs("../../../cellctl/tests/parity.test.sh")
	if err != nil {
		t.Fatal(err)
	}
	oracle, err := filepath.Abs("../../../cellctl/testdata/cellctl-shell-oracle.sh")
	if err != nil {
		t.Fatal(err)
	}
	binary := cellctlBinary(t)
	run := func(t *testing.T, impl, only, want string, code int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", suite)
		cmd.Dir = t.TempDir()
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "CELLCTL_A=" + oracle, "CELLCTL_B=" + impl, "PARITY_ONLY=" + only, "KUBECONFIG=/dev/null"}
		out, err := cmd.CombinedOutput()
		got := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				got = e.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if got != code || !strings.Contains(string(out), want) {
			t.Fatalf("parity exit=%d want=%d: %v\n%s", got, code, err, out)
		}
		t.Log(want)
	}
	t.Run("matrix", func(t *testing.T) { run(t, binary, "", "parity: 200 cells, 0 divergent", 0) })
	// A second site or extension must not inherit an exemption from the known labels row.
	// These wrappers change the real binary's output, never the expected oracle output.
	for _, tc := range []struct{ name, only, mutation string }{
		{"unknown-label-text", "house/claude/tmux/check", `sed 's/labels: the cell/labels: unexpected the cell/'`},
		{"label-state", "house/claude/tmux/check", `sed 's/?     could-not-check: labels:/MISS  labels:/'`},
		{"false-clean-summary", "house/claude/tmux/check", `sed 's/^\[check\] every checked.*/[check] all preconditions met/'`},
		{"new-label-state", "house/github/-/new", `sed 's/labels: NOT ENSURED/labels: ensured/'`},
		{"other-diagnostic", "house/claude/tmux/check", `cat; printf 'unexpected diagnostic\n'`},
		{"exit-status", "house/claude/tmux/check", `cat`},
		{"unknown-set-notice", "k8s/codex/tmux/set", `cat; printf '[set] notice: unexpected extension\n'`},
		{"second-site", "container/claude/tmux/status", `cat; printf '  ?     could-not-check: labels: unexpected second site\n'`},
		{"plan-bytes", "scrubbed/claude/tmux/desk", `awk '$0 == "[plan] env KUBECONFIG=/dev/null" { $0 = "[plan] env KUBECONFIG=wrong" } { print }'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			impl := filepath.Join(t.TempDir(), "cellctl-mutant")
			script := "#!/bin/bash\nout=$(" + strconv.Quote(binary) + " \"$@\" 2>&1); rc=$?\nprintf '%s\\n' \"$out\" | { " + tc.mutation + "; }\n"
			if tc.name == "exit-status" {
				script += "exit 7\n"
			} else {
				script += "exit \"$rc\"\n"
			}
			control := "#!/bin/bash\nexec " + strconv.Quote(binary) + " \"$@\"\n"
			if err := os.WriteFile(impl, []byte(control), 0o700); err != nil {
				t.Fatal(err)
			}
			run(t, impl, tc.only, "parity: 1 cells, 0 divergent", 0)
			if err := os.WriteFile(impl, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			run(t, impl, tc.only, "DIVERGENT  "+tc.only, 1)
		})
	}
}

// Shared expectations belong to transcript comparisons, not package startup:
// helper subprocesses re-execute this binary from arbitrary fixture directories.
// regression: #6455
func TestParityFixtureInit(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^$")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH"), "KUBECONFIG=/dev/null"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("test-binary startup outside its package: %v\n%s", err, out)
	}
}
