package terminal

import (
	"context"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// fourLoopCfg is the row-count-invariant fixture: 4 configured loops, no
// peers, no `master:` (so dispatch is the default master) — the shape the
// negative-path test exercises with every handle unreachable.
func fourLoopCfg() *schema.File {
	return &schema.File{Cells: []schema.Cell{{
		Name: "assay",
		Loops: []schema.Loop{
			{Role: schema.RoleReview, App: "assay-reviewer-app", Target: &schema.TargetSpec{Kind: schema.TargetContainer, Container: "c1"}},
			{Role: schema.RoleVerify, App: "assay-verifier-app", Target: &schema.TargetSpec{Kind: schema.TargetContainer, Container: "c2"}},
			{Role: schema.RoleDispatch, App: "assay-desk-app", Target: &schema.TargetSpec{Kind: schema.TargetContainer, Container: "c3"}},
			{Role: schema.RoleMetrics, App: "metrics-harvest-app", Target: &schema.TargetSpec{Kind: schema.TargetContainer, Container: "c4"}},
		},
	}}}
}

// TestDeskRowsNeverDropUnreachable (Verify row 2, NEGATIVE PATH, SPOF layer a)
// — a fixture of 4 loops, every handle ErrUnreachable, must still produce
// exactly 4 rows, all could-not-check. A reach failure may only change a
// row's STATE; it must never remove the row (the silent-cell failure mode,
// design §10).
func TestDeskRowsNeverDropUnreachable(t *testing.T) {
	cfg := fourLoopCfg()
	r := NewRegistry(cfg)
	cell := &cfg.Cells[0]
	for _, l := range cell.Loops {
		id := DeskID(cell.Name, string(l.Role))
		if err := r.Attach(context.Background(), id, &fakeTarget{unreachable: true}, Size{Cols: 80, Rows: 24}); err != nil {
			t.Fatalf("Attach(%s) with an unreachable target returned a hard error: %v", id, err)
		}
	}

	rows := r.DeskRows(cell)
	if len(rows) != len(cell.Loops) {
		t.Fatalf("DeskRows returned %d rows, want %d (one per configured loop, unreachable never drops a row)", len(rows), len(cell.Loops))
	}
	for _, row := range rows {
		if row.State != string(StateCouldNotCheck) {
			t.Errorf("row %s state = %q, want could-not-check (every handle here is unreachable)", row.ID, row.State)
		}
	}
}

// TestDeskRowsMasterGroupDefaultsToDispatch (Verify row 4) — with no
// `master:` key, the default master role is dispatch, the-desk's identity in
// both dogfood cells; the default is by ROLE, never "the first loop".
func TestDeskRowsMasterGroupDefaultsToDispatch(t *testing.T) {
	cfg := fourLoopCfg()
	r := NewRegistry(cfg)
	rows := r.DeskRows(&cfg.Cells[0])
	var masters []string
	for _, row := range rows {
		if row.Group == "master" {
			masters = append(masters, row.Role)
		}
	}
	if len(masters) != 1 || masters[0] != "dispatch" {
		t.Fatalf("master group = %v, want exactly one row with role dispatch", masters)
	}
}

// TestDeskRowsMasterGroupHonorsExplicitRole — an explicit `master:` picks the
// NAMED role, never the first configured loop (a cell listing review first
// must not silently make review the master).
func TestDeskRowsMasterGroupHonorsExplicitRole(t *testing.T) {
	cfg := fourLoopCfg()
	cfg.Cells[0].Master = "verify"
	r := NewRegistry(cfg)
	rows := r.DeskRows(&cfg.Cells[0])
	for _, row := range rows {
		wantGroup := "desk"
		if row.Role == "verify" {
			wantGroup = "master"
		}
		if row.Group != wantGroup {
			t.Errorf("row %s group = %q, want %q", row.ID, row.Group, wantGroup)
		}
	}
}

// TestDeskRowsAppIsWriteAppOrDefault — a row's app is the loop's EFFECTIVE
// write identity (WriteAppOrDefault), not the desk-tools roster and not the
// bare App field when a write_app override is set.
func TestDeskRowsAppIsWriteAppOrDefault(t *testing.T) {
	writeApp := "assay-cell-issues"
	cfg := &schema.File{Cells: []schema.Cell{{
		Name: "assay",
		Loops: []schema.Loop{
			{Role: schema.RoleReview, App: "assay-reviewer-app", WriteApp: &writeApp},
		},
	}}}
	r := NewRegistry(cfg)
	rows := r.DeskRows(&cfg.Cells[0])
	if len(rows) != 1 || rows[0].App != writeApp {
		t.Fatalf("row app = %+v, want the write_app override %q", rows, writeApp)
	}
}

// TestDeskRowsPeersAreLast — configured desks first, peers appended after,
// each a could-not-check row carrying its url (this brief lists peers; it
// does not dereference them — peer reachability makes them reachable).
func TestDeskRowsPeersAreLast(t *testing.T) {
	cfg := fourLoopCfg()
	cfg.Cells[0].Peers = []schema.Peer{{Cell: "tracker", URL: "https://tracker.example.com"}}
	r := NewRegistry(cfg)
	cell := &cfg.Cells[0]
	rows := r.DeskRows(cell)
	if len(rows) != len(cell.Loops)+1 {
		t.Fatalf("DeskRows = %d rows, want %d (loops + 1 peer)", len(rows), len(cell.Loops)+1)
	}
	last := rows[len(rows)-1]
	if last.Group != "peer" || last.Cell != "tracker" || last.URL != "https://tracker.example.com" {
		t.Fatalf("last row = %+v, want the peer row (group peer, cell tracker, its url)", last)
	}
	if last.State != string(StateCouldNotCheck) {
		t.Errorf("peer row state = %q, want could-not-check (no cross-deskd read in this brief)", last.State)
	}
	for _, row := range rows[:len(cell.Loops)] {
		if row.Group == "peer" {
			t.Errorf("a configured-desk row must not carry group peer: %+v", row)
		}
	}
}

// TestDeskRowsNilRegistryStillProducesRows — a nil *Registry (no terminal
// registry wired on this build) must still return one could-not-check row per
// configured loop, never a crash and never an empty list.
func TestDeskRowsNilRegistryStillProducesRows(t *testing.T) {
	cfg := fourLoopCfg()
	var r *Registry
	rows := r.DeskRows(&cfg.Cells[0])
	if len(rows) != len(cfg.Cells[0].Loops) {
		t.Fatalf("nil-registry DeskRows = %d rows, want %d", len(rows), len(cfg.Cells[0].Loops))
	}
	for _, row := range rows {
		if row.State != string(StateCouldNotCheck) {
			t.Errorf("row %s state = %q, want could-not-check with no registry wired", row.ID, row.State)
		}
	}
}

// TestDeskRowsNilCellIsEmpty — a nil cell argument is an empty row set, not a
// panic.
func TestDeskRowsNilCellIsEmpty(t *testing.T) {
	r := NewRegistry(fourLoopCfg())
	if rows := r.DeskRows(nil); rows != nil {
		t.Fatalf("DeskRows(nil) = %v, want nil", rows)
	}
}
