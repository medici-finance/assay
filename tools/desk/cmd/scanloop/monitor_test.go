package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const seededPoll = `MONITOR-ARMED: 41
`

const busyPoll = `INBOUND: example-org/tracker#101 2026-08-24T10:00:00Z
INBOUND: medici-finance/assay#7 2026-08-24T10:05:00Z
INBOUND-BURST: example-org/agents 40 over 25 — listing suppressed
MONITOR-DEGRADED: example-org/examples read FAILED (gh: boom) — keeping its previous 12 issue(s)
`

// TestParse_SeedCycleReportsNoInbound — the seeding pass must report the count and NOTHING
// enumerable. Replaying a whole backlog as new work on first sight is the phantom flood the
// poller's per-repo state exists to prevent, and a parser that invented items here would put it
// back.
func TestParse_SeedCycleReportsNoInbound(t *testing.T) {
	r := ParseMonitorOutput(seededPoll)
	if !r.Armed || r.ArmedTotal != 41 {
		t.Fatalf("armed=%v total=%d, want true/41", r.Armed, r.ArmedTotal)
	}
	if len(r.Inbound) != 0 {
		t.Fatalf("a seed cycle produced %d inbound item(s) — the backlog was replayed as new work", len(r.Inbound))
	}
}

func TestParse_EventsBurstsAndDegraded(t *testing.T) {
	r := ParseMonitorOutput(busyPoll)
	if len(r.Inbound) != 2 {
		t.Fatalf("inbound = %d, want 2", len(r.Inbound))
	}
	if got := r.Inbound[0].ID(); got != "example-org/tracker#101" {
		t.Fatalf("item id = %q", got)
	}
	if !r.Inbound[0].UpdatedAt.Equal(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("updatedAt = %v", r.Inbound[0].UpdatedAt)
	}
	if len(r.Bursts) != 1 || len(r.Degraded) != 1 {
		t.Fatalf("bursts=%d degraded=%d, want 1/1", len(r.Bursts), len(r.Degraded))
	}
	if !r.Blind() {
		t.Fatal("a cycle carrying a degraded repo AND a suppressed burst reported itself NOT blind — " +
			"rc would then be read as a complete sweep")
	}
}

// TestParse_UnknownLineIsSurfaced — a poller that grows a new line kind must not be silently
// half-read.
func TestParse_UnknownLineIsSurfaced(t *testing.T) {
	r := ParseMonitorOutput("MONITOR-SOMETHING-NEW: hello\n")
	if len(r.Unparsed) != 1 {
		t.Fatalf("unparsed = %v, want the unknown line surfaced", r.Unparsed)
	}
}

// TestParse_MalformedKeyIsDropped_MalformedTimeIsNot — the key is what gets claimed and routed, so
// an unusable key drops the row; a timestamp only ages it, so a bad one keeps the work.
func TestParse_MalformedKeyIsDropped_MalformedTimeIsNot(t *testing.T) {
	r := ParseMonitorOutput("INBOUND: notaslug 2026-08-24T10:00:00Z\nINBOUND: example-org/tracker#5 notatime\n")
	if len(r.Inbound) != 1 {
		t.Fatalf("inbound = %d, want only the one with a usable key", len(r.Inbound))
	}
	if !r.Inbound[0].UpdatedAt.IsZero() {
		t.Fatalf("unparsable timestamp produced %v, want the zero time", r.Inbound[0].UpdatedAt)
	}
	if len(r.Unparsed) != 1 {
		t.Fatalf("the unusable key was dropped without being surfaced: %v", r.Unparsed)
	}
}

// TestStateFileName_MatchesThePollersOwnMapping — the arming read is a filename convention shared
// with a shell script. If it drifts, every repo silently reports UNSEEDED and the plan tells an
// operator to arm a monitor that is already armed.
func TestStateFileName_MatchesThePollersOwnMapping(t *testing.T) {
	const slug = "example-org/tracker"
	if got := stateFileName(slug); got != "example-org__tracker.state" {
		t.Fatalf("stateFileName = %q", got)
	}
	if got := slugFromStateFile(stateFileName(slug)); got != slug {
		t.Fatalf("round trip = %q, want %q", got, slug)
	}
}

func TestReadMonitorState_SeededUnseededAndForeign(t *testing.T) {
	dir := t.TempDir()
	write := func(slug string) {
		if err := os.WriteFile(filepath.Join(dir, stateFileName(slug)), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("example-org/tracker")
	write("example-org/retired")

	st, err := ReadMonitorState(dir, []string{"example-org/tracker", "medici-finance/assay"})
	if err != nil {
		t.Fatal(err)
	}
	if !st.Armed {
		t.Fatal("a dir with a baseline reported NOT ARMED")
	}
	if len(st.Seeded) != 1 || st.Seeded[0] != "example-org/tracker" {
		t.Fatalf("seeded = %v", st.Seeded)
	}
	if len(st.Unseeded) != 1 || st.Unseeded[0] != "medici-finance/assay" {
		t.Fatalf("unseeded = %v — a rostered repo with no baseline is a BLIND SPOT and must be named", st.Unseeded)
	}
	if len(st.Foreign) != 1 || st.Foreign[0] != "example-org/retired" {
		t.Fatalf("foreign = %v", st.Foreign)
	}
}

// TestReadMonitorState_MissingDirIsNotArmed_NotAnError — a checkout that never armed is an honest
// "not armed", not a crash.
func TestReadMonitorState_MissingDirIsNotArmed_NotAnError(t *testing.T) {
	st, err := ReadMonitorState(filepath.Join(t.TempDir(), "never-created"), []string{"example-org/tracker"})
	if err != nil {
		t.Fatalf("missing state dir errored: %v", err)
	}
	if st.Armed {
		t.Fatal("a missing state dir reported ARMED")
	}
}

func TestFindMonitorScript_SearchOrderAndSiblingFallback(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "target")
	sibling := filepath.Join(base, "plugins-home")
	plant := func(under string) string {
		p := filepath.Join(under, monitorScriptRelPath)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	want := plant(sibling)

	got, err := FindMonitorScript(root, "")
	if err != nil {
		t.Fatalf("sibling fallback failed: %v", err)
	}
	if got != want {
		t.Fatalf("found %q, want the sibling checkout's copy %q", got, want)
	}

	// The root's own copy wins over a sibling's.
	inRoot := plant(root)
	got, err = FindMonitorScript(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != inRoot {
		t.Fatalf("found %q, want the root's own copy %q", got, inRoot)
	}
}

// TestFindMonitorScript_NotFoundIsUnverifiable — a missing poller is COULD-NOT-CHECK, never a
// silent "run without an inbound surface".
func TestFindMonitorScript_NotFoundIsUnverifiable(t *testing.T) {
	_, err := FindMonitorScript(t.TempDir(), "")
	if err == nil {
		t.Fatal("a missing poller returned no error — the drain would run with no inbound surface")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (unverifiable)", got, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(err.Error(), "Searched:") {
		t.Fatalf("the refusal does not name where it looked: %v", err)
	}
}

// TestFindMonitorScript_ExplicitMissingIsRefused — an operator who named a path and got it wrong
// gets a refusal, not a silent fallback to a different script than the one they meant.
func TestFindMonitorScript_ExplicitMissingIsRefused(t *testing.T) {
	_, err := FindMonitorScript(t.TempDir(), filepath.Join(t.TempDir(), "nope.sh"))
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitRefused {
		t.Fatalf("exit code = %d, want %d (refused)", got, deskkit.ExitRefused)
	}
}

// TestRunMonitor_DegradedExitIsNotFatal — the poller's exit 2 means at least one repo retained its
// baseline. Those repos are could-not-check for this pass; the OTHERS still drain, so the run must
// carry the report rather than throw the whole cycle away.
func TestRunMonitor_DegradedExitIsNotFatal(t *testing.T) {
	rep, err := RunMonitor("/x/inbound-monitor.sh", t.TempDir(), []string{"example-org/tracker"},
		func(string, []string, ...string) (string, int, error) { return busyPoll, 2, nil })
	if err != nil {
		t.Fatalf("exit 2 was treated as fatal: %v", err)
	}
	if len(rep.Inbound) != 2 || len(rep.Degraded) != 1 {
		t.Fatalf("report = %+v", rep)
	}
}

// TestRunMonitor_PreconditionFailureIsUnverifiable — exit 1 is the poller saying it could not run
// at all.
func TestRunMonitor_PreconditionFailureIsUnverifiable(t *testing.T) {
	_, err := RunMonitor("/x/inbound-monitor.sh", t.TempDir(), []string{"example-org/tracker"},
		func(string, []string, ...string) (string, int, error) { return "inbound-monitor: jq not found", 1, nil })
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (unverifiable)", got, deskkit.ExitUnverifiable)
	}
}

// TestRunMonitor_EmptyScopeIsRefusedLoudly — an empty sweep is never reported as a clean, empty
// board.
func TestRunMonitor_EmptyScopeIsRefusedLoudly(t *testing.T) {
	_, err := RunMonitor("/x/inbound-monitor.sh", t.TempDir(), nil,
		func(string, []string, ...string) (string, int, error) { return "", 0, nil })
	if err == nil {
		t.Fatal("an empty scan scope produced a clean, empty sweep")
	}
}

// TestRunMonitor_PassesTheStateDirThrough — the state dir is how a session keeps its own baselines
// out of another session's. Passing it is not optional.
func TestRunMonitor_PassesTheStateDirThrough(t *testing.T) {
	dir := t.TempDir()
	var sawEnv []string
	var sawArgs []string
	_, err := RunMonitor("/x/inbound-monitor.sh", dir, []string{"example-org/tracker", "medici-finance/assay"},
		func(_ string, env []string, args ...string) (string, int, error) {
			sawEnv, sawArgs = env, args
			return seededPoll, 0, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range sawEnv {
		if e == EnvMonitorStateDir+"="+dir {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s was not passed to the poller", EnvMonitorStateDir)
	}
	if len(sawArgs) != 2 {
		t.Fatalf("repo args = %v, want the whole rostered scope in ONE invocation", sawArgs)
	}
}
