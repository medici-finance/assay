//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// The lifecycle verbs through the real binary: every state-changing invocation refuses unless
// stdin AND stdout are a terminal, and `drain --now` abandons claims only on a confirmed terminal.

// treeSnapshot lists every file under the roots with its size and mtime, so a refused invocation
// can be shown to have changed nothing a stop or a pause lives in.
func treeSnapshot(t *testing.T, roots ...string) string {
	t.Helper()
	var lines []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			lines = append(lines, p+" "+fi.ModTime().String())
			return nil
		})
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// runMixed runs the binary with stdin and stdout each either a pipe or the slave end of a PTY.
func runMixed(t *testing.T, w *cliWorld, stdinTTY, stdoutTTY bool, args ...string) (int, string) {
	t.Helper()
	master, slave, err := openPTY()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, e := master.Read(buf); e != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cellctlBinary(t), args...)
	cmd.Env = w.baseEnv()
	var out bytes.Buffer
	if stdinTTY {
		cmd.Stdin = slave
	} else {
		cmd.Stdin = strings.NewReader("")
	}
	if stdoutTTY {
		cmd.Stdout = slave
	} else {
		cmd.Stdout = &out
	}
	cmd.Stderr = &out
	err = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%v timed out", args)
	}
	if err == nil {
		return 0, out.String()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), out.String()
	}
	t.Fatalf("%v: %v", args, err)
	return 0, ""
}

// regression: lifecycle-off-tty
func TestPauseResumeDrainRefuseOffTTY(t *testing.T) {
	w := newCLIWorld(t, "")
	touch(t, filepath.Join(w.cfgDir, "STOP"))
	before := treeSnapshot(t, w.cfgDir, filepath.Join(w.cellDir, "run"))
	modes := []struct {
		name          string
		stdin, stdout bool
	}{{"pipes", false, false}, {"stdin terminal", true, false}, {"stdout terminal", false, true}}
	for _, args := range [][]string{
		{"pause", "example"},
		{"pause", "example", "worker-desk"},
		{"resume", "example"},
		{"resume", "example", "--foreign", "STOP"},
		{"drain", "example"},
		{"drain", "example", "--now"},
		{"down", "example", "worker-desk"},
		{"cycle", "example"},
		{"cycle", "example", "worker-desk", "--force-after-timeout"},
	} {
		for _, m := range modes {
			code, out := runMixed(t, w, m.stdin, m.stdout, args...)
			if code != 5 || !strings.Contains(out, cellop.ReasonNotInteractive) {
				t.Errorf("%v (%s): exit %d, want 5 %s\n%s", args, m.name, code, cellop.ReasonNotInteractive, out)
			}
		}
	}
	if after := treeSnapshot(t, w.cfgDir, filepath.Join(w.cellDir, "run")); after != before {
		t.Fatalf("a refused invocation changed the stops or the pause state:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// regression: drain-now-interactive
func TestDrainNowInteractiveOnly(t *testing.T) {
	w := newCLIWorld(t, "")
	stub := filepath.Join(w.deskToolsDir(), "desksupervise")
	snap := `{"schema":"desksupervise-status-v1","claims":[{"Key":"k1","Repo":"example-org/example-repo","Item":"i","State":"held","Holder":"example-worker-desk-20261103T000000Z"}]}`
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho '"+snap+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := w.run(t, nil, "drain", "example", "--now")
	if r.Code != 5 || !strings.Contains(r.Stdout+r.Stderr, cellop.ReasonNotInteractive) {
		t.Fatalf("drain --now off a terminal: exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	_, out := kubeTTY(t, w, 60*time.Second, "drain", "example", "--now", "--handoff-wait", "0s")
	if !strings.Contains(out, "abandon:k1") || !strings.Contains(out, "unknown outcome") {
		t.Fatalf("drain --now on a terminal must name the abandoned claim and its unknown outcome:\n%s", out)
	}
}
