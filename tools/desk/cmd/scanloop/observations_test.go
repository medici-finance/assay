package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unsetEnv clears a variable for the test and restores it afterwards.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

// replayPass seeds eight retained events for one repository, runs one retaining pass with
// an empty poll, and returns how many events the pass presented and the deferred lines.
func replayPass(t *testing.T) (int, []string) {
	t.Helper()
	isolatedDeskHome(t)
	dir := t.TempDir()
	var lines strings.Builder
	for n := 1; n <= 8; n++ {
		fmt.Fprintf(&lines, "medici-finance/assay#%d 2026-08-24T11:5%d:00Z\n", 200+n, n)
	}
	if err := os.WriteFile(filepath.Join(dir, "medici-finance__assay.state"), []byte(lines.String()), 0600); err != nil {
		t.Fatal(err)
	}
	loop := passLoop(t, "")
	loop.Monitor = retainingMonitor(dir, []string{"medici-finance/assay"}, func() (*MonitorReport, error) { return &MonitorReport{}, nil })
	if _, err := loop.SelectQueue(); err != nil {
		t.Fatal(err)
	}
	rep := loop.Report()
	if rep == nil {
		t.Fatal("no report")
	}
	return len(loop.Queued()), rep.Deferred
}

// regression: F-replay-own-knob — the replay bound had no knob of its own.
func TestReplayCapOwnKnob(t *testing.T) {
	t.Run("own knob sets the bound and the deferred line names it", func(t *testing.T) {
		t.Setenv("INTAKE_REPLAY_CAP", "3")
		unsetEnv(t, "INBOUND_MONITOR_BURST_CAP")
		got, deferred := replayPass(t)
		if got != 3 {
			t.Fatalf("presented %d retained events, want 3", got)
		}
		want := "medici-finance/assay 5 retained observation(s) deferred (replay cap 3 per repo; retained, oldest first)"
		if len(deferred) != 1 || deferred[0] != want {
			t.Fatalf("deferred = %q, want [%q]", deferred, want)
		}
	})

	t.Run("own knob wins over the burst knob", func(t *testing.T) {
		t.Setenv("INTAKE_REPLAY_CAP", "3")
		t.Setenv("INBOUND_MONITOR_BURST_CAP", "7")
		if got, _ := replayPass(t); got != 3 {
			t.Fatalf("presented %d, want 3 (own knob first)", got)
		}
	})

	t.Run("own knob above the burst bound enlarges the batch", func(t *testing.T) {
		t.Setenv("INTAKE_REPLAY_CAP", "6")
		t.Setenv("INBOUND_MONITOR_BURST_CAP", "2")
		got, deferred := replayPass(t)
		if got != 6 || len(deferred) != 1 || !strings.Contains(deferred[0], "replay cap 6 per repo") {
			t.Fatalf("presented %d, deferred %q; want 6 and cap 6", got, deferred)
		}
	})

	t.Run("own knob unset keeps the burst fallback", func(t *testing.T) {
		unsetEnv(t, "INTAKE_REPLAY_CAP")
		t.Setenv("INBOUND_MONITOR_BURST_CAP", "4")
		got, deferred := replayPass(t)
		if got != 4 || len(deferred) != 1 || !strings.Contains(deferred[0], "replay cap 4 per repo") {
			t.Fatalf("presented %d, deferred %q; want 4 and cap 4", got, deferred)
		}
	})

	for _, bad := range []string{"", "0", "-2", "many", "3.5"} {
		t.Run("own knob "+fmt.Sprintf("%q", bad)+" falls through to the burst knob", func(t *testing.T) {
			t.Setenv("INTAKE_REPLAY_CAP", bad)
			t.Setenv("INBOUND_MONITOR_BURST_CAP", "4")
			if got := replayCap(); got != 4 {
				t.Fatalf("replayCap() = %d, want 4", got)
			}
		})
	}

	t.Run("neither knob set keeps the default of 25", func(t *testing.T) {
		unsetEnv(t, "INTAKE_REPLAY_CAP")
		unsetEnv(t, "INBOUND_MONITOR_BURST_CAP")
		if got := replayCap(); got != 25 {
			t.Fatalf("replayCap() = %d, want 25", got)
		}
	})
}
