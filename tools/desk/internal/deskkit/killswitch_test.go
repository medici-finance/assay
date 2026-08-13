package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGuardKillSwitch exercises the kill switch. The function name contains
// "Guard" so a `-run Guard` verify step selects it; the disabled subtests log the
// refusal (which contains the word "disabled") so the grep in that step matches.
func TestGuardKillSwitch(t *testing.T) {
	t.Run("armed via env DESK_TOOLS_DISABLED=1", func(t *testing.T) {
		dir := setup(t)
		t.Setenv("DESK_TOOLS_DISABLED", "1")

		err := Guard()
		t.Logf("guard refused: %v", err) // prints "disabled" for Verify item 3
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled (exit 3)", err)
		}
		if ExitCodeOf(err) != ExitDisabled {
			t.Fatalf("ExitCodeOf = %d, want %d", ExitCodeOf(err), ExitDisabled)
		}
		// An audit line with result=disabled must have been written.
		entries, lerr := LoadEntries()
		if lerr != nil {
			t.Fatalf("LoadEntries: %v", lerr)
		}
		if len(entries) != 1 || entries[0].Result != ResultDisabled {
			t.Fatalf("expected 1 disabled audit line, got %+v", entries)
		}
		_ = dir
	})

	t.Run("armed via DISABLED file with reason", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir desk dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "DISABLED"), []byte("manual halt: runaway loop\n"), 0o600); err != nil {
			t.Fatalf("write DISABLED: %v", err)
		}
		err := Guard()
		t.Logf("guard refused: %v", err) // prints "disabled"
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled", err)
		}
		if got := err.Error(); !strings.Contains(got, "manual halt: runaway loop") {
			t.Fatalf("reason not surfaced: %q", got)
		}
	})

	t.Run("not armed → nil", func(t *testing.T) {
		setup(t)
		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v, want nil when not armed", err)
		}
	})

	t.Run("disarm transition is recorded once", func(t *testing.T) {
		dir := setup(t)
		// Seed history whose last line is a disabled state; DISABLED file absent now.
		appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "guard", Result: ResultDisabled, Detail: "was halted"})

		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v, want nil after disarm", err)
		}
		entries, err := LoadEntries()
		if err != nil {
			t.Fatalf("LoadEntries: %v", err)
		}
		last := entries[len(entries)-1]
		if last.Verb != "guard" || last.Result != ResultOK || !strings.Contains(last.Detail, "disarmed") {
			t.Fatalf("disarm transition not logged, last = %+v", last)
		}
	})

	t.Run("unreadable DISABLED → Unverifiable, never assume-enabled", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir desk dir: %v", err)
		}
		// A directory named DISABLED makes os.ReadFile fail with a non-NotExist error.
		if err := os.Mkdir(filepath.Join(dir, "DISABLED"), 0o700); err != nil {
			t.Fatalf("mkdir DISABLED: %v", err)
		}
		err := Guard()
		if !IsUnverifiable(err) {
			t.Fatalf("Guard() = %v, want Unverifiable (fail closed, exit 6)", err)
		}
	})
}

// TestGuardStopFlags exercises the loop stop-flags: STOP, STOP.<loop>,
// HEARTBEAT (fresh/stale/absent), and DESK_LOOP unset. Table-driven:
// each flag form alone; both; heartbeat fresh/stale/absent; DESK_LOOP unset
// (per-loop flags ignored, STOP still honored).
func TestGuardStopFlags(t *testing.T) {
	t.Run("STOP flag → armed (exit 3)", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "all loops halted by operator")
		err := Guard()
		t.Logf("guard refused: %v", err)
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled", err)
		}
		if !strings.Contains(err.Error(), "STOP") {
			t.Fatalf("reason should name STOP flag, got %q", err.Error())
		}
		if !strings.Contains(err.Error(), "all loops halted by operator") {
			t.Fatalf("reason should include flag content, got %q", err.Error())
		}
	})

	t.Run("STOP flag empty → armed with fallback reason", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "")
		err := Guard()
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled (empty STOP still halts)", err)
		}
		if !strings.Contains(err.Error(), "STOP") {
			t.Fatalf("reason should name STOP flag, got %q", err.Error())
		}
	})

	t.Run("STOP.<loop> when DESK_LOOP matches → armed", func(t *testing.T) {
		dir := setup(t)
		t.Setenv(loopEnv, "pr-review-desk")
		mkFlag(t, dir, "STOP.pr-review-desk", "pausing review loop")
		err := Guard()
		t.Logf("guard refused: %v", err)
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled", err)
		}
		if !strings.Contains(err.Error(), "STOP.pr-review-desk") {
			t.Fatalf("reason should name STOP.pr-review-desk flag, got %q", err.Error())
		}
	})

	t.Run("STOP.<loop> when DESK_LOOP is unset → ignored", func(t *testing.T) {
		dir := setup(t)
		t.Setenv(loopEnv, "")
		mkFlag(t, dir, "STOP.pr-review-desk", "pausing review loop")
		err := Guard()
		if err != nil {
			t.Fatalf("Guard() = %v, want nil — per-loop flag ignored when DESK_LOOP unset", err)
		}
	})

	t.Run("STOP.<loop> when DESK_LOOP is different → not armed", func(t *testing.T) {
		dir := setup(t)
		t.Setenv(loopEnv, "verify-desk")
		mkFlag(t, dir, "STOP.pr-review-desk", "pausing review loop")
		err := Guard()
		if err != nil {
			t.Fatalf("Guard() = %v, want nil — STOP.pr-review-desk should not affect verify-desk", err)
		}
	})

	t.Run("both STOP and STOP.<loop> → STOP takes precedence", func(t *testing.T) {
		dir := setup(t)
		t.Setenv(loopEnv, "pr-review-desk")
		mkFlag(t, dir, "STOP", "all loops down")
		mkFlag(t, dir, "STOP.pr-review-desk", "specific loop down")
		err := Guard()
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled", err)
		}
		if !strings.Contains(err.Error(), "STOP") && !strings.Contains(err.Error(), "all loops down") {
			t.Fatalf("STOP should surface, got %q", err.Error())
		}
	})

	t.Run("HEARTBEAT fresh → not armed", func(t *testing.T) {
		dir := setup(t)
		writeHeartbeat(t, dir)
		err := Guard()
		if err != nil {
			t.Fatalf("Guard() = %v, want nil when HEARTBEAT is fresh", err)
		}
	})

	t.Run("HEARTBEAT stale (>24h) → armed", func(t *testing.T) {
		dir := setup(t)
		writeStaleHeartbeat(t, dir, HeartbeatStaleDuration+time.Hour)
		err := Guard()
		t.Logf("guard refused: %v", err)
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled (stale HEARTBEAT)", err)
		}
		if !strings.Contains(err.Error(), "HEARTBEAT stale") {
			t.Fatalf("reason should name stale HEARTBEAT, got %q", err.Error())
		}
	})

	t.Run("HEARTBEAT absent → not armed", func(t *testing.T) {
		setup(t)
		err := Guard()
		if err != nil {
			t.Fatalf("Guard() = %v, want nil when HEARTBEAT absent", err)
		}
	})

	t.Run("HEARTBEAT exactly at boundary → not stale", func(t *testing.T) {
		dir := setup(t)
		writeStaleHeartbeat(t, dir, HeartbeatStaleDuration-time.Second)
		err := Guard()
		if err != nil {
			t.Fatalf("Guard() = %v, want nil — HEARTBEAT just inside window is still fresh", err)
		}
	})

	t.Run("HEARTBEAT exactly at boundary + 1ns → stale", func(t *testing.T) {
		dir := setup(t)
		writeStaleHeartbeat(t, dir, HeartbeatStaleDuration+time.Nanosecond)
		err := Guard()
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled — HEARTBEAT just over boundary is stale", err)
		}
	})

	t.Run("unreadable STOP → Unverifiable", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// A directory named STOP makes os.ReadFile fail with a non-NotExist error.
		if err := os.Mkdir(filepath.Join(dir, "STOP"), 0o700); err != nil {
			t.Fatalf("mkdir STOP: %v", err)
		}
		err := Guard()
		if !IsUnverifiable(err) {
			t.Fatalf("Guard() = %v, want Unverifiable (fail closed, exit 6)", err)
		}
	})

	t.Run("DISABLED takes precedence over STOP", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "all loops down")
		mkFlag(t, dir, "DISABLED", "kill switch armed")
		err := Guard()
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled", err)
		}
		if !strings.Contains(err.Error(), "kill switch armed") {
			t.Fatalf("DISABLED should take precedence, got %q", err.Error())
		}
	})

	t.Run("STOP takes precedence over HEARTBEAT stale", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "all loops down")
		writeStaleHeartbeat(t, dir, HeartbeatStaleDuration+time.Hour)
		err := Guard()
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled", err)
		}
		if !strings.Contains(err.Error(), "STOP") {
			t.Fatalf("STOP should take precedence over HEARTBEAT, got %q", err.Error())
		}
	})

	t.Run("audit line names the exact flag on stop", func(t *testing.T) {
		dir := setup(t)
		t.Setenv(loopEnv, "batch-fanout")
		mkFlag(t, dir, "STOP.batch-fanout", "fanout paused")
		Guard() // ignore error, check audit
		entries, lerr := LoadEntries()
		if lerr != nil {
			t.Fatalf("LoadEntries: %v", lerr)
		}
		if len(entries) == 0 {
			t.Fatalf("expected audit line, got none")
		}
		if !strings.Contains(entries[0].Detail, "STOP.batch-fanout") {
			t.Fatalf("audit detail should name the exact flag, got %q", entries[0].Detail)
		}
	})
}

// TestActiveStopFlags exercises the diagnostic export used by deskboard.
func TestActiveStopFlags(t *testing.T) {
	t.Run("no flags → empty", func(t *testing.T) {
		setup(t)
		flags := ActiveStopFlags()
		if len(flags) != 0 {
			t.Fatalf("ActiveStopFlags = %+v, want empty", flags)
		}
	})

	t.Run("STOP flag → reported", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "all loops halted")
		flags := ActiveStopFlags()
		if len(flags) != 1 || flags[0].Name != "STOP" {
			t.Fatalf("ActiveStopFlags = %+v, want [STOP]", flags)
		}
		if flags[0].Reason != "all loops halted" {
			t.Fatalf("reason = %q, want 'all loops halted'", flags[0].Reason)
		}
	})

	t.Run("per-loop STOP flags → all reported", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.pr-review-desk", "review paused")
		mkFlag(t, dir, "STOP.verify-desk", "verify paused")
		flags := ActiveStopFlags()
		names := map[string]bool{}
		for _, f := range flags {
			names[f.Name] = true
		}
		if !names["STOP.pr-review-desk"] || !names["STOP.verify-desk"] {
			t.Fatalf("ActiveStopFlags = %+v, want both per-loop flags reported", flags)
		}
	})

	t.Run("stale HEARTBEAT → reported with stale=true", func(t *testing.T) {
		dir := setup(t)
		writeStaleHeartbeat(t, dir, HeartbeatStaleDuration+time.Hour)
		flags := ActiveStopFlags()
		found := false
		for _, f := range flags {
			if f.Name == "HEARTBEAT" && f.Stale {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("ActiveStopFlags = %+v, want HEARTBEAT with stale=true", flags)
		}
	})

	t.Run("fresh HEARTBEAT → not reported", func(t *testing.T) {
		dir := setup(t)
		writeHeartbeat(t, dir)
		flags := ActiveStopFlags()
		for _, f := range flags {
			if f.Name == "HEARTBEAT" {
				t.Fatalf("ActiveStopFlags = %+v, fresh HEARTBEAT should not be reported as active", flags)
			}
		}
	})

	t.Run("absent HEARTBEAT → not reported", func(t *testing.T) {
		setup(t)
		flags := ActiveStopFlags()
		for _, f := range flags {
			if f.Name == "HEARTBEAT" {
				t.Fatalf("ActiveStopFlags = %+v, absent HEARTBEAT should not be reported", flags)
			}
		}
	})
}

// --- test helpers for stop flags ---

func mkFlag(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func writeHeartbeat(t *testing.T, dir string) {
	t.Helper()
	mkFlag(t, dir, "HEARTBEAT", "ok") // fresh — created just now
}

func writeStaleHeartbeat(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "HEARTBEAT")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write HEARTBEAT: %v", err)
	}
	// Set mtime to age ago.
	staleTime := time.Now().Add(-age)
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes HEARTBEAT: %v", err)
	}
}
