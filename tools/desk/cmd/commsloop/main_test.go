package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// TestIdlePollCadenceIsNotZero pins the exact regression this brief fixes:
// PoolSize:1 with NO IdlePoll set (the zero value) makes loopengine.Run's
// idle branch call time.Sleep(0) / time.After(0) every cycle the
// accepted-queue is empty — the steady state for this drain consumer — which
// is a CPU busy-spin plus a stderr "idle" flood. A future edit that deletes
// or zeroes idlePollCadence (used verbatim by run()'s loopengine.Config)
// fails this test immediately, with no timing involved.
func TestIdlePollCadenceIsNotZero(t *testing.T) {
	if idlePollCadence <= 0 {
		t.Fatalf("idlePollCadence = %v, want > 0 — a zero/negative IdlePoll makes loopengine.Run's idle branch busy-spin (time.Sleep(0)/time.After(0)) on the empty accepted-queue steady state", idlePollCadence)
	}
	// Follow cmd/scanloop/run.go's documented ~60-90s prod cadence (see
	// loopengine.Config.IdlePoll's doc) rather than a merely-nonzero value
	// that would still flood at, say, 1ms.
	if idlePollCadence < 30*time.Second {
		t.Fatalf("idlePollCadence = %v, want a real steady-state cadence (~60-90s, matching cmd/scanloop/run.go's IdlePoll: time.Minute), not a near-zero value that still floods stderr/CPU", idlePollCadence)
	}
}

// setupCommsloopDeskHome points deskkit's state dir (via HOME) at a fresh
// temp dir so this test controls the kill-switch deterministically, mirroring
// internal/loopengine's own setupDeskHome test helper.
//
// DESK_LOOP is set to a name deskkit/loopnames.go actually KNOWS ("worker-desk")
// rather than this package's own Loop.Name() ("commsloop", which is not a
// registered loop name) — exactly the house convention (CLAUDE.md: `export
// DESK_LOOP=worker-desk` before invoking a drain binary) that makes the
// per-loop STOP.<name> check resolvable in production. Were DESK_LOOP left
// unset, loopengine.Run would set it to "commsloop" itself (Run's own
// os.Getenv("DESK_LOOP") == "" fallback), which deskkit's stopFlagState then
// refuses as an unknown loop name — a separate, out-of-scope registration
// gap this test deliberately routes around rather than papering over.
func setupCommsloopDeskHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "") // "" is not "1" => not armed
	t.Setenv("CLAUDE_SESSION_ID", "commsloop-test")
	t.Setenv("DESK_LOOP", "worker-desk")
	return filepath.Join(home, ".config", "assay")
}

// syncBuf is a concurrency-safe io.Writer so the Progress line count can be
// read from the test goroutine while loopengine.Run writes from its own.
type syncBuf struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuf) idleLines() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Count(b.buf.String(), "idle\n")
}

// TestRunDoesNotBusySpinOnEmptyQueue drives the REAL loopengine.Run with a
// commsloop Loop wired exactly like run() wires it (empty accepted-queue —
// the steady state), and proves the idle path PACES at its configured
// cadence instead of busy-spinning: with a nonzero (if test-scaled) IdlePoll,
// landing N idle cycles must take at least roughly N-1 cadences of wall
// time. Before this brief's fix (IdlePoll defaulting to 0), the same number
// of idle cycles would land near-instantly — a busy CPU spin invisible to
// this assertion's floor, and a stderr flood the "idle" line count alone
// does not catch either, which is why the timing floor is the load-bearing
// check here, not just counting.
func TestRunDoesNotBusySpinOnEmptyQueue(t *testing.T) {
	deskDir := setupCommsloopDeskHome(t)
	root := t.TempDir() // empty accepted-queue for the whole test

	acl := comms.Compiled()
	loop := &Loop{
		Root: root,
		Mon:  DirMonitor{Root: root},
		ACL:  &acl,
		// Filer left nil deliberately: an empty queue never quarantines
		// anything, so no filing call is ever exercised in this test.
	}

	const idlePoll = 25 * time.Millisecond
	const wantIdleCycles = 4

	progress := &syncBuf{}
	cfg := loopengine.Config{
		PoolSize:   1,
		IdlePoll:   idlePoll,
		ClaimsDir:  filepath.Join(t.TempDir(), "claims"),
		StaleClaim: time.Hour,
		Progress:   progress,
	}

	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- loopengine.Run(cfg, loop) }()

	deadline := time.After(5 * time.Second)
	planted := false
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()

	var elapsed time.Duration
	var runErr error
loop:
	for {
		select {
		case runErr = <-done:
			elapsed = time.Since(start)
			break loop
		case <-tick.C:
			if !planted && progress.idleLines() >= wantIdleCycles {
				planted = true
				if err := os.MkdirAll(deskDir, 0o700); err != nil {
					t.Fatalf("mkdir deskdir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(deskDir, "STOP"), []byte("test stop\n"), 0o600); err != nil {
					t.Fatalf("write STOP: %v", err)
				}
			}
		case <-deadline:
			t.Fatal("loopengine.Run did not stop within deadline (possible wedge)")
		}
	}

	if runErr != nil {
		t.Fatalf("loopengine.Run: %v", runErr)
	}
	if n := progress.idleLines(); n < wantIdleCycles {
		t.Fatalf("observed %d idle cycles, want >= %d — the run stopped before proving pacing", n, wantIdleCycles)
	}

	// The pacing floor: landing wantIdleCycles-1 SLEEPS between idle cycles
	// at idlePoll cadence takes at least that many cadences of wall time. A
	// busy-spin (IdlePoll == 0, this brief's regression) would land the same
	// cycle count near-instantly, well under this floor.
	minElapsed := time.Duration(wantIdleCycles-1) * idlePoll / 2
	if elapsed < minElapsed {
		t.Fatalf("idle loop paced %d cycles in %v, want >= %v for an IdlePoll of %v — this is too fast to be sleeping between cycles and looks like a busy spin instead",
			wantIdleCycles, elapsed, minElapsed, idlePoll)
	}
}
