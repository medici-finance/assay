package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// writeFile writes content at a repo-relative path under root, creating parent dirs.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// scopedRow builds a BoardRow whose brief scopes are the given normalized prefixes (derivable).
func scopedRow(stream, num string, prefixes ...string) BoardRow {
	r := briefRow(stream, num, "M", "", "model", false)
	set := loopengine.WriteScopeSet{Derivable: true}
	for _, p := range prefixes {
		set.Scopes = append(set.Scopes, loopengine.WriteScope{Prefix: p})
	}
	r.WriteScopes = set
	return r
}

// TestRenderPlan_WriteOverlapWarns is the plan-level proof of Verify rows 2-4: a candidate whose
// scopes overlap an in-flight claim's scopes emits a WRITE-OVERLAP line naming both ids and the
// shared prefix, AFTER the queue rows; disjoint scopes print nothing; the queue is rendered in
// full regardless (advisory — the overlap never drops or reorders a row).
func TestRenderPlan_WriteOverlapWarns(t *testing.T) {
	setupDeskHome(t)
	board := []BoardRow{
		scopedRow("alpha", "01", "internal/loopengine/engine.go"),
		scopedRow("gamma", "03", "docs/site/"), // disjoint from any in-flight scope
	}
	inflight := []loopengine.Item{
		{ID: "beta/02", WriteScopes: loopengine.WriteScopeSet{Derivable: true, Scopes: []loopengine.WriteScope{{Prefix: "internal/loopengine/"}}}},
	}
	f := &FanoutLoop{
		Board:    func() ([]BoardRow, error) { return board, nil },
		InFlight: func() ([]loopengine.Item, error) { return inflight, nil },
		Rework:   noRework,
		Emit:     &bytes.Buffer{},
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()

	want := "WRITE-OVERLAP: alpha/01 ~ beta/02 on internal/loopengine/"
	if !strings.Contains(s, want) {
		t.Fatalf("missing overlap line %q in:\n%s", want, s)
	}
	if strings.Contains(s, "gamma/03 ~") {
		t.Fatalf("disjoint candidate gamma/03 must produce no overlap line:\n%s", s)
	}
	// The queue is rendered in full first: BOTH candidates still dispatch (advisory never drops a row).
	if !strings.Contains(s, "=== DISPATCH alpha/01") || !strings.Contains(s, "=== DISPATCH gamma/03") {
		t.Fatalf("advisory overlap must not drop a queue row:\n%s", s)
	}
	// The overlap warning comes AFTER the queue rows.
	if strings.Index(s, "=== DISPATCH") > strings.Index(s, "WRITE-OVERLAP") {
		t.Fatalf("WRITE-OVERLAP must follow the queue rows:\n%s", s)
	}
}

// TestRenderPlan_CouldNotDeriveReported proves the three-state honesty at the plan surface: a
// candidate whose scopes cannot be derived is named could-not-derive, never treated as clear,
// and it does not depend on the in-flight read (so it prints even with no in-flight claims).
func TestRenderPlan_CouldNotDeriveReported(t *testing.T) {
	setupDeskHome(t)
	board := []BoardRow{
		{Stream: "nofiles", Num: "01", Title: "t", Effort: "M", Gate: "model", BriefPath: "docs/streams/nofiles/x.md"}, // zero WriteScopes => could-not-derive
	}
	f := &FanoutLoop{
		Board:    func() ([]BoardRow, error) { return board, nil },
		InFlight: func() ([]loopengine.Item, error) { return nil, nil },
		Rework:   noRework,
		Emit:     &bytes.Buffer{},
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	if !strings.Contains(out.String(), "nofiles/01: scopes: could-not-derive") {
		t.Fatalf("could-not-derive not reported:\n%s", out.String())
	}
}

// TestRenderPlan_DisjointPrintsNothing proves Verify row 3 directly: disjoint candidate and
// in-flight scopes emit no WRITE-OVERLAP and no could-not-derive noise.
func TestRenderPlan_DisjointPrintsNothing(t *testing.T) {
	setupDeskHome(t)
	board := []BoardRow{scopedRow("aa", "01", "cmd/deskpost/")}
	inflight := []loopengine.Item{
		{ID: "bb/02", WriteScopes: loopengine.WriteScopeSet{Derivable: true, Scopes: []loopengine.WriteScope{{Prefix: "cmd/deskpr/"}}}},
	}
	f := &FanoutLoop{
		Board:    func() ([]BoardRow, error) { return board, nil },
		InFlight: func() ([]loopengine.Item, error) { return inflight, nil },
		Rework:   noRework,
		Emit:     &bytes.Buffer{},
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	if strings.Contains(out.String(), "WRITE-OVERLAP") || strings.Contains(out.String(), "could-not-derive") {
		t.Fatalf("disjoint scopes must print no advisory noise:\n%s", out.String())
	}
}

// TestReadNextUp_DerivesWriteScopes proves the board reader populates each row's write-scopes
// from the brief's Context files list (the data plan compares on).
func TestReadNextUp_DerivesWriteScopes(t *testing.T) {
	root := t.TempDir()
	status := "# STATUS\n\n## Next up\n\n| Stream | Brief | Wave | Score |\n|---|---|---|---|\n" +
		"| eng | 09 — scopes | 1 | 100 |\n"
	writeFile(t, root, "STATUS.md", status)
	brief := "---\nbrief: eng/09\ngate: model\neffort: M\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\n" +
		"## Context\n\nfiles:\n- `internal/loopengine/`\n\n## Task\ndo\n"
	writeFile(t, root, "docs/streams/eng/brief-09-scopes.md", brief)

	rows, err := readNextUp(root, "sha")
	if err != nil {
		t.Fatalf("readNextUp: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	ws := rows[0].WriteScopes
	if !ws.Derivable || len(ws.Scopes) != 1 || ws.Scopes[0].Prefix != "internal/loopengine/" {
		t.Fatalf("write-scopes not derived onto the row: %+v", ws)
	}
	if rows[0].toItem("sha").WriteScopes.Scopes[0].Prefix != "internal/loopengine/" {
		t.Fatal("toItem did not carry write-scopes onto the Item")
	}
}
