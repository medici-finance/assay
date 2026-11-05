//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestVerifyrunDeadlineOwnsDescendants is a regression for a row deadline that
// killed only bash, leaving its ordinary child alive with the output pipe. It
// measures the actual executor, never a substitute timeout or a shorter check.
// regression: #4360
func TestVerifyrunDeadlineOwnsDescendants(t *testing.T) {
	root := t.TempDir()
	plan := resolveShellPlan()
	if !plan.ok {
		t.Skip("no runnable bash: " + plan.reason)
	}
	command := `printf 'started\n'; (sleep 2; printf 'late\n' > descendant-ran) & wait`
	rows := []verifyRow{
		{ID: "17", Command: command, Expect: "exit 0", Shell: rowShellSh},
		{ID: "23", Command: `printf 'next\n'`, Expect: "exit 0", Shell: rowShellSh},
	}
	began := time.Now()
	ws := runWitnessesWith(plan, root, rows, "test-runner", "git-config", "0123456789ab", "2026-10-24", 150*time.Millisecond, false)
	elapsed := time.Since(began)
	t.Logf("executor witness: rows=%d elapsed=%s deadline=150ms", len(ws), elapsed)
	if elapsed > time.Second {
		t.Errorf("row deadline overran: executor returned after %s; ordinary descendants retained the output pipe", elapsed)
	}
	if len(ws) != 2 {
		t.Fatalf("witness count=%d, want2", len(ws))
	}
	if ws[0].ID != "17" || ws[0].Command != command || ws[1].ID != "23" || ws[1].Command != rows[1].Command {
		t.Fatalf("row identity/order changed: %+v", ws)
	}
	if ws[0].State != stateCouldNotRun || ws[0].Exit != -1 || !strings.Contains(ws[0].Note, "timed out after 150ms") || ws[0].OutHash != hashOutput([]byte("started\n")) {
		t.Errorf("deadline witness changed: %+v", ws[0])
	}
	if ws[1].State != statePass || ws[1].Exit != 0 || ws[1].OutHash != hashOutput([]byte("next\n")) {
		t.Errorf("following row did not run independently: %+v", ws[1])
	}
	// Wait past the fixture child's finite work. A quick return alone could be a
	// pipe-close shortcut that leaves work running after the next row starts.
	if remaining := 2200*time.Millisecond - time.Since(began); remaining > 0 {
		time.Sleep(remaining)
	}
	if _, err := os.Stat(filepath.Join(root, "descendant-ran")); !os.IsNotExist(err) {
		t.Errorf("timed-out descendant performed delayed work; stat=%v", err)
	}
}

// TestVerifyrunDeadlineShellExit: the shell exits at once and leaves an
// ordinary child in its group holding the output pipe. The deadline must still
// kill that child; a kill wired only to the un-reaped shell never fires here.
// regression: #4360
func TestVerifyrunDeadlineShellExit(t *testing.T) {
	root := t.TempDir()
	command := `printf 'started\n'; (sleep 2; printf 'late\n' > descendant-ran) &`
	began := time.Now()
	got := runVerifyCommand(root, command, 150*time.Millisecond)
	elapsed := time.Since(began)
	t.Logf("executor returned after %s; deadline=150ms", elapsed)
	if elapsed > time.Second {
		t.Errorf("row deadline overran after the shell exited: executor returned after %s", elapsed)
	}
	if !got.couldNotRun || got.exit != -1 || string(got.output) != "started\n" || !strings.Contains(got.reason, "timed out after 150ms") {
		t.Errorf("deadline result changed: %+v output=%q", got, got.output)
	}
	if remaining := 2200*time.Millisecond - time.Since(began); remaining > 0 {
		time.Sleep(remaining)
	}
	if _, err := os.Stat(filepath.Join(root, "descendant-ran")); !os.IsNotExist(err) {
		t.Errorf("descendant outlived the deadline and did its delayed work; stat=%v", err)
	}
}

// Successful shells may leave a child producing output before the ORIGINAL row
// deadline. A fixed short WaitDelay would cut that valid output off after shell
// exit; cancellation must not turn this complete run into could-not-run.
// regression: #4360
func TestVerifyrunDeadlineKeepsSuccessfulBackgroundOutput(t *testing.T) {
	root := t.TempDir()
	command := `printf 'first\n'; (sleep 1.2; printf 'last\n') &`
	got := runVerifyCommand(root, command, 4*time.Second)
	if got.couldNotRun || got.exit != 0 || string(got.output) != "first\nlast\n" {
		t.Fatalf("normal background output changed: %+v output=%q", got, got.output)
	}
}

// regression: #4360
func TestVerifyrunDeadlineKeepsFailureAndTimeoutStates(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		timeout       time.Duration
		state         string
		exit          int
		output        string
	}{
		{"success", `printf 'ok\n'`, time.Second, statePass, 0, "ok\n"},
		{"nonzero", `printf 'failed\n'; exit 7`, time.Second, stateFail, 7, "failed\n"},
		{"pipeline", `false \| cat`, time.Second, stateFail, 1, ""},
		{"timeout", `printf 'started\n'; sleep 2`, 150 * time.Millisecond, stateCouldNotRun, -1, "started\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runVerifyCommand(t.TempDir(), tc.command, tc.timeout)
			state, _ := parseExpect("exit 0").verdict(got)
			if state != tc.state || got.exit != tc.exit || string(got.output) != tc.output {
				t.Errorf("state=%s result=%+v output=%q; wantstate=%s exit=%d output=%q", state, got, got.output, tc.state, tc.exit, tc.output)
			}
		})
	}
}

// The matched fixture benchmarks report executor work separately from the
// liveness regression's deliberate post-deadline observation window. Run each
// with -benchtime=1x in alternating fresh baseline/candidate processes. They do
// not estimate the duration of real verification commands or production runs.
func BenchmarkVerifyrunDeadlineTimeout(b *testing.B) {
	plan := resolveShellPlan()
	root := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		began := time.Now()
		got := runVerifyCommandWith(root, `printf 'started\n'; sleep 2 & wait`, 150*time.Millisecond, nil, plan, rowShellSh)
		elapsed := time.Since(began)
		if !got.couldNotRun || got.exit != -1 || string(got.output) != "started\n" {
			b.Fatalf("deadline fixture changed: %+v output=%q", got, got.output)
		}
		b.ReportMetric(float64(elapsed.Nanoseconds())/1e6, "executor-ms")
	}
}

func BenchmarkVerifyrunDeadlineNormalOutput(b *testing.B) {
	plan := resolveShellPlan()
	root := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		began := time.Now()
		got := runVerifyCommandWith(root, `printf 'first\n'; (sleep 1.2; printf 'last\n') &`, 4*time.Second, nil, plan, rowShellSh)
		elapsed := time.Since(began)
		if got.couldNotRun || got.exit != 0 || string(got.output) != "first\nlast\n" {
			b.Fatalf("normal fixture changed: %+v output=%q", got, got.output)
		}
		b.ReportMetric(float64(elapsed.Nanoseconds())/1e6, "executor-ms")
	}
}
