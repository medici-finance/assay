package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cellSession marks this test process as a session of cell (ASSAY_CELL) running loop
// (DESK_LOOP and ASSAY_CELL_ROLE), the way cellctl stamps a launched desk.
func cellSession(t *testing.T, cell, loop string) {
	t.Helper()
	t.Setenv(CellMarkerEnv, cell)
	t.Setenv(CellRoleMarkerEnv, loop)
	t.Setenv(loopEnv, loop)
}

// regression: #2971
func TestGuardCellStop(t *testing.T) {
	t.Run("STOP.cell.<cell> stops every loop of the marked cell", func(t *testing.T) {
		for _, loop := range []string{"worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "the-desk"} {
			dir := setup(t)
			cellSession(t, "a", loop)
			mkFlag(t, dir, "STOP.cell.a", "draining cell a")
			err := Guard()
			if !IsDisabled(err) {
				t.Fatalf("loop %s: Guard() = %v, want Disabled (exit 3)", loop, err)
			}
			if ExitCodeOf(err) != ExitDisabled {
				t.Fatalf("loop %s: exit = %d, want %d", loop, ExitCodeOf(err), ExitDisabled)
			}
			if !strings.Contains(err.Error(), "STOP.cell.a") {
				t.Fatalf("loop %s: refusal should name STOP.cell.a, got %q", loop, err.Error())
			}
		}
	})

	t.Run("STOP.cell.<cell>.<loop> stops that loop only", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.cell.a.worker-desk", "pause worker of cell a")
		cellSession(t, "a", "worker-desk")
		err := Guard()
		t.Logf("worker-desk: %v", err)
		if !IsDisabled(err) || !strings.Contains(err.Error(), "STOP.cell.a.worker-desk") {
			t.Fatalf("worker-desk in cell a: Guard() = %v, want Disabled naming STOP.cell.a.worker-desk", err)
		}
		for _, other := range []string{"pr-review-desk", "verify-desk"} {
			cellSession(t, "a", other)
			if err := Guard(); err != nil {
				t.Fatalf("%s in cell a: Guard() = %v, want nil — the flag names worker-desk only", other, err)
			}
		}
	})

	t.Run("STOP.cell.<cell>.<loop> resolves the loop's name class", func(t *testing.T) {
		dir := setup(t)
		// batch-fanout is a registered alias of worker-desk (loopnames.go).
		mkFlag(t, dir, "STOP.cell.a.batch-fanout", "")
		cellSession(t, "a", "worker-desk")
		if err := Guard(); !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled — an alias-named cell flag must hold", err)
		}
	})

	t.Run("STOP.cell.<cell>.<role> holds via ASSAY_CELL_ROLE with DESK_LOOP unset", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.cell.a.verify-desk", "")
		t.Setenv(CellMarkerEnv, "a")
		t.Setenv(CellRoleMarkerEnv, "verify-desk")
		t.Setenv(loopEnv, "")
		if err := Guard(); !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled — the marker's role names verify-desk", err)
		}
	})

	// F2: the trailing DESK_LOOP check must hold on its own. cellSession sets
	// ASSAY_CELL_ROLE == DESK_LOOP, so the role-based check stops the session first and would
	// hide a removed DESK_LOOP path; here the role is unset, or names a different loop.
	t.Run("STOP.cell.<cell>.<loop> holds via DESK_LOOP alone", func(t *testing.T) {
		for _, tc := range []struct{ what, role string }{
			{"ASSAY_CELL_ROLE unset", ""},
			{"ASSAY_CELL_ROLE names another loop", "verify-desk"},
		} {
			for _, flag := range []string{"STOP.cell.a.worker-desk", "STOP.cell.a.batch-fanout"} {
				dir := setup(t)
				mkFlag(t, dir, flag, "")
				t.Setenv(CellMarkerEnv, "a")
				t.Setenv(CellRoleMarkerEnv, tc.role)
				t.Setenv(loopEnv, "worker-desk")
				err := Guard()
				if !IsDisabled(err) || !strings.Contains(err.Error(), flag) {
					t.Fatalf("%s, flag %s: Guard() = %v, want Disabled naming %s", tc.what, flag, err, flag)
				}
			}
		}
		// Control: the same session with no flag, and with another loop's flag, is not stopped.
		dir := setup(t)
		mkFlag(t, dir, "STOP.cell.a.verify-desk", "")
		t.Setenv(CellMarkerEnv, "a")
		t.Setenv(CellRoleMarkerEnv, "")
		t.Setenv(loopEnv, "worker-desk")
		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v, want nil — the flag names verify-desk, this session is worker-desk", err)
		}
	})

	t.Run("a different cell's flag does not stop", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.cell.b", "draining cell b")
		mkFlag(t, dir, "STOP.cell.b.worker-desk", "")
		cellSession(t, "a", "worker-desk")
		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v, want nil — STOP.cell.b must not stop a session of cell a", err)
		}
	})

	t.Run("no marker → no cell flag stops", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.cell.a", "")
		mkFlag(t, dir, "STOP.cell.a.worker-desk", "")
		t.Setenv(loopEnv, "worker-desk")
		// setup cleared ASSAY_CELL / ASSAY_CELL_ROLE.
		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v, want nil — an unmarked session is unaffected by cell flags", err)
		}
	})

	t.Run("DESK_CELL alone is not membership", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.cell.a", "")
		t.Setenv("DESK_CELL", "a")
		t.Setenv(loopEnv, "worker-desk")
		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v, want nil — DESK_CELL is comms-only and never read", err)
		}
	})

	t.Run("STOP.cell.<cell> wins over an unrecognised DESK_LOOP", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.cell.a", "")
		t.Setenv(CellMarkerEnv, "a")
		t.Setenv(loopEnv, "no-such-loop")
		if err := Guard(); !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled — the cell-wide flag precedes loop resolution", err)
		}
	})
}

// regression: #2971
func TestGuardCellStopNeverClears(t *testing.T) {
	t.Run("STOP still stops a session whose marker names another cell", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "all loops halted")
		mkFlag(t, dir, "STOP.cell.b", "")
		cellSession(t, "a", "worker-desk")
		err := Guard()
		if !IsDisabled(err) || !strings.Contains(err.Error(), "STOP: all loops halted") {
			t.Fatalf("Guard() = %v, want Disabled naming STOP", err)
		}
	})

	t.Run("STOP.<loop> still stops a session whose marker names another cell", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP.worker-desk", "")
		mkFlag(t, dir, "STOP.cell.b.worker-desk", "")
		cellSession(t, "a", "worker-desk")
		err := Guard()
		if !IsDisabled(err) || !strings.Contains(err.Error(), "STOP.worker-desk") {
			t.Fatalf("Guard() = %v, want Disabled naming STOP.worker-desk", err)
		}
	})

	t.Run("DISABLED still wins over a marked session", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "DISABLED", "kill switch")
		cellSession(t, "a", "worker-desk")
		err := Guard()
		if !IsDisabled(err) || !strings.Contains(err.Error(), "kill switch") {
			t.Fatalf("Guard() = %v, want Disabled naming the kill switch", err)
		}
	})

	t.Run("STOP wins over an invalid marker", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "")
		t.Setenv(CellMarkerEnv, "../x")
		if err := Guard(); !IsDisabled(err) {
			t.Fatalf("Guard() = %v, want Disabled — STOP is resolved before the marker", err)
		}
	})
}

// regression: #2971
func TestCellNameRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"../x", "a/b", "A", "a.b", "a_b", ".."} {
		t.Run(bad, func(t *testing.T) {
			dir := setup(t)
			// Plant a flag at exactly the path a naive join would build, so a
			// path-building implementation would refuse exit 3 instead of exit 6.
			naive := filepath.Join(dir, "STOP.cell."+bad)
			if err := os.MkdirAll(filepath.Dir(naive), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(naive, []byte("planted"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(CellMarkerEnv, bad)
			t.Setenv(loopEnv, "worker-desk")
			err := Guard()
			t.Logf("marker %q: %v", bad, err)
			if !IsUnverifiable(err) || ExitCodeOf(err) != ExitUnverifiable {
				t.Fatalf("marker %q: Guard() = %v (exit %d), want Unverifiable exit %d", bad, err, ExitCodeOf(err), ExitUnverifiable)
			}
			if strings.Contains(err.Error(), "planted") {
				t.Fatalf("marker %q: a path was built from the invalid name: %q", bad, err.Error())
			}
		})
	}
	for _, good := range []string{"a", "house", "cell-2"} {
		if !ValidCellName(good) {
			t.Fatalf("ValidCellName(%q) = false, want true", good)
		}
	}
}

// regression: #2971
func TestActiveStopFlagsCellInert(t *testing.T) {
	dir := setup(t)
	root := t.TempDir()
	t.Setenv("CELLS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "cell.env"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	mkFlag(t, dir, "STOP.cell.a", "drain a")
	mkFlag(t, dir, "STOP.cell.a.worker-desk", "")
	mkFlag(t, dir, "STOP.cell.a.no-such-loop", "")
	mkFlag(t, dir, "STOP.cell.ghost", "")
	mkFlag(t, dir, "STOP.cell.Bad_Name", "")

	want := map[string]bool{ // name → inert
		"STOP.cell.a":              false,
		"STOP.cell.a.worker-desk":  false,
		"STOP.cell.a.no-such-loop": true,
		"STOP.cell.ghost":          true,
		"STOP.cell.Bad_Name":       true,
	}
	got := map[string]ActiveStopFlag{}
	for _, f := range ActiveStopFlags() {
		got[f.Name] = f
	}
	for name, inert := range want {
		f, ok := got[name]
		if !ok {
			t.Fatalf("ActiveStopFlags() missing %s; got %+v", name, got)
		}
		if f.Inert != inert {
			t.Fatalf("%s: Inert = %v, want %v (reason %q)", name, f.Inert, inert, f.Reason)
		}
		if inert && !strings.Contains(f.Reason, "INERT") {
			t.Fatalf("%s: inert flag reason should say INERT, got %q", name, f.Reason)
		}
	}
	// F3: the unregistered case names the root it looked in and does not claim the flag
	// "halts nothing" — the Guard honours it for a session of a cell registered elsewhere.
	ghost := got["STOP.cell.ghost"].Reason
	if !strings.Contains(ghost, "no registration for cell ghost under "+root) || strings.Contains(ghost, "halts nothing") {
		t.Fatalf("unknown cell reason = %q", ghost)
	}
	if !strings.Contains(got["STOP.cell.a.no-such-loop"].Reason, "halts nothing") {
		t.Fatalf("unknown-loop reason = %q", got["STOP.cell.a.no-such-loop"].Reason)
	}
}
