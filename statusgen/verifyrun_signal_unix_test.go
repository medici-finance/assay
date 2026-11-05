//go:build unix

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestVerifyrunSignalChild is the subprocess half of
// TestVerifyrunSignalCancelsRow: it runs the real verifyrun entry point so the
// parent can signal it the way a terminal or supervisor would. Inert unless the
// parent set its environment.
func TestVerifyrunSignalChild(t *testing.T) {
	root := os.Getenv("ASSAY_SIGNAL_CHILD_ROOT")
	if root == "" {
		return
	}
	args := []string{"--brief", os.Getenv("ASSAY_SIGNAL_CHILD_BRIEF"), "--root", root, "--dry-run", "--timeout", "30s"}
	code := runVerifyrun(args, os.Stdout, os.Stderr)
	fmt.Printf("child-exit=%d\n", code)
	os.Exit(0)
}

// TestVerifyrunSignalRowNoVerdict: a row killed by the run's interrupt is
// could-not-run naming the signal — never the fail its killed shell's exit
// would otherwise read as, and never a pass.
// regression: #4360
func TestVerifyrunSignalRowNoVerdict(t *testing.T) {
	plan := resolveShellPlan()
	if !plan.ok {
		t.Skip("no runnable bash: " + plan.reason)
	}
	stop, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	plan.stop = stop
	time.AfterFunc(100*time.Millisecond, func() { cancel(runInterrupted{sig: syscall.SIGTERM}) })
	began := time.Now()
	got := runVerifyCommandWith(t.TempDir(), `printf 'started\n'; sleep 5`, 30*time.Second, nil, plan, rowShellSh)
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Errorf("the interrupt did not stop the row: returned after %s", elapsed)
	}
	state, note := parseExpect("exit 0").verdict(got)
	if state != stateCouldNotRun || got.exit != -1 || !strings.Contains(note, "interrupted by SIGTERM") || string(got.output) != "started\n" {
		t.Errorf("interrupted row: state=%s note=%q result=%+v output=%q", state, note, got, got.output)
	}
	// A row after the interrupt never starts.
	root := t.TempDir()
	ws := runWitnessesWith(plan, root, []verifyRow{{ID: "2", Command: "touch ran", Expect: "exit 0", Shell: rowShellSh}}, "r", "s", "t", "d", time.Second, false)
	if len(ws) != 1 || ws[0].State != stateCouldNotRun || !strings.Contains(ws[0].Note, "it was not run") {
		t.Errorf("row after the interrupt: %+v", ws)
	}
	if _, err := os.Stat(filepath.Join(root, "ran")); !os.IsNotExist(err) {
		t.Errorf("a row started after the interrupt; stat=%v", err)
	}
}

// TestVerifyrunSignalCancelsRow: every row runs in its own process group, so a
// stop signal sent to statusgen's group never reaches the row directly. The
// signal must still stop the row (its delayed work never happens), the row is
// could-not-run, the next row never starts, and no witness is written.
// regression: #4360
func TestVerifyrunSignalCancelsRow(t *testing.T) {
	for _, tc := range []struct {
		sig  syscall.Signal
		name string
	}{
		{syscall.SIGINT, "SIGINT"},
		{syscall.SIGTERM, "SIGTERM"},
		{syscall.SIGHUP, "SIGHUP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := admittedFixture(t, false)
			brief := filepath.Join(t.TempDir(), "brief.md")
			tick := string(rune(96))
			body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n" +
				"| 1 | " + tick + "touch started; sleep 1.5; touch escaped" + tick + " | exit 0 |\n" +
				"| 2 | " + tick + "touch second-ran" + tick + " | exit 0 |\n\n## Evidence\n\n"
			if err := os.WriteFile(brief, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestVerifyrunSignalChild$")
			cmd.Env = append(os.Environ(), "ASSAY_SIGNAL_CHILD_ROOT="+root, "ASSAY_SIGNAL_CHILD_BRIEF="+brief)
			// Its own group, as a terminal's foreground job or a supervised job is.
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var out bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &out
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			pgid := cmd.Process.Pid
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) }()

			deadline := time.Now().Add(20 * time.Second)
			for {
				if _, err := os.Stat(filepath.Join(root, "started")); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("row 1 never started; output:\n%s", out.String())
				}
				time.Sleep(20 * time.Millisecond)
			}
			signalled := time.Now()
			if err := syscall.Kill(-pgid, tc.sig); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatalf("statusgen did not exit after %s; output:\n%s", tc.name, out.String())
			}
			// Wait past row 1's delayed work before looking for it.
			if remaining := 2*time.Second - time.Since(signalled); remaining > 0 {
				time.Sleep(remaining)
			}
			if _, err := os.Stat(filepath.Join(root, "escaped")); !os.IsNotExist(err) {
				t.Errorf("row 1 outlived %s and did its delayed work; stat=%v", tc.name, err)
			}
			if _, err := os.Stat(filepath.Join(root, "second-ran")); !os.IsNotExist(err) {
				t.Errorf("row 2 started after %s; stat=%v", tc.name, err)
			}
			got := out.String()
			for _, want := range []string{
				"row 1: could-not-run",
				"interrupted by " + tc.name,
				"row 2: could-not-run",
				"no witness was written",
				"child-exit=2",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("output lacks %q:\n%s", want, got)
				}
			}
			for _, bad := range []string{"row 1: pass", "row 1: fail", "row 2: pass", "row 2: fail"} {
				if strings.Contains(got, bad) {
					t.Errorf("an interrupted row carries a verdict (%q):\n%s", bad, got)
				}
			}
		})
	}
}

// signalChild starts TestVerifyrunSignalChild on a brief holding rows, in its
// own process group, through /bin/sh running prelude and then exec-ing the test
// binary (so prelude can set the signal state the child starts with).
func signalChild(t *testing.T, prelude string, rows ...string) (root string, pgid int, out *bytes.Buffer, done chan error) {
	t.Helper()
	root, _ = admittedFixture(t, false)
	brief := filepath.Join(t.TempDir(), "brief.md")
	tick := string(rune(96))
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n"
	for i, r := range rows {
		body += fmt.Sprintf("| %d | %s%s%s | exit 0 |\n", i+1, tick, r, tick)
	}
	body += "\n## Evidence\n\n"
	if err := os.WriteFile(brief, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", prelude+` exec "$0" "$@"`, os.Args[0], "-test.run=^TestVerifyrunSignalChild$")
	cmd.Env = append(os.Environ(), "ASSAY_SIGNAL_CHILD_ROOT="+root, "ASSAY_SIGNAL_CHILD_BRIEF="+brief)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out = &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pgid = cmd.Process.Pid
	done = make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
	return root, pgid, out, done
}

// waitForFile polls for root/name, failing the test after limit.
func waitForFile(t *testing.T, root, name string, limit time.Duration, out *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never appeared; output:\n%s", name, out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestVerifyrunSignalIgnoredStays: a signal that was ignored when statusgen
// started (nohup, or a script's background job) stays ignored for the run.
// Catching it would turn a run its caller deliberately shielded into an
// interrupted, could-not-run one.
// regression: #4360
func TestVerifyrunSignalIgnoredStays(t *testing.T) {
	for _, tc := range []struct {
		sig  syscall.Signal
		trap string
	}{
		{syscall.SIGHUP, "HUP"},
		{syscall.SIGINT, "INT"},
	} {
		t.Run(tc.trap, func(t *testing.T) {
			root, pgid, out, done := signalChild(t, "trap '' "+tc.trap+";",
				"touch started; sleep 1; touch finished", "touch second-ran")
			waitForFile(t, root, "started", 20*time.Second, out)
			if err := syscall.Kill(-pgid, tc.sig); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				t.Fatalf("statusgen did not finish; output:\n%s", out.String())
			}
			got := out.String()
			for _, want := range []string{"row 1: pass", "row 2: pass", "child-exit=0"} {
				if !strings.Contains(got, want) {
					t.Errorf("ignored SIG%s changed the run: output lacks %q:\n%s", tc.trap, want, got)
				}
			}
			for _, name := range []string{"finished", "second-ran"} {
				if _, err := os.Stat(filepath.Join(root, name)); err != nil {
					t.Errorf("ignored SIG%s stopped the run: %s missing: %v", tc.trap, name, err)
				}
			}
		})
	}
}

// TestVerifyrunSignalSecondStops: after the first stop signal cancels the run,
// a second one takes its default effect. A row descendant that escaped its
// process group can hold the output pipe after the group kill; a second
// SIGTERM must still end statusgen instead of being caught and dropped.
// regression: #4360
func TestVerifyrunSignalSecondStops(t *testing.T) {
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("no perl to leave the row's process group")
	}
	root, pgid, out, done := signalChild(t, "",
		`perl -e 'setpgrp(0,0); open(F, ">", "escaped-pid"); print F $$; close F; sleep 6' & sleep 0.2; touch started; wait`)
	waitForFile(t, root, "started", 20*time.Second, out)
	t.Cleanup(func() {
		if b, err := os.ReadFile(filepath.Join(root, "escaped-pid")); err == nil {
			var pid int
			if _, err := fmt.Sscan(string(b), &pid); err == nil && pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	second := time.Now()
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if elapsed := time.Since(second); elapsed > 2*time.Second {
			t.Errorf("statusgen ended %s after the second SIGTERM; it was swallowed", elapsed)
		}
		var ee *exec.ExitError
		if !errors.As(err, &ee) || !ee.Sys().(syscall.WaitStatus).Signaled() {
			t.Errorf("second SIGTERM did not take its default effect: err=%v output:\n%s", err, out.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("statusgen did not exit after a second SIGTERM; output:\n%s", out.String())
	}
}
