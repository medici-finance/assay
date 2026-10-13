package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Every hold the package tests exercise is also checked, move by move, against
// the whole-board evaluation: a divergence fails the run with an error instead
// of passing on the incremental answer (#2615).
func init() { holdCrossCheck = true }

// holdOutcome is one reconcileWritesWith run with its root made relative, so
// runs over two copies of one fixture compare byte for byte.
type holdOutcome struct {
	Rows  string
	Held  string
	Err   string
	Tree  map[string]string
	Stats holdStats
}

func runHold(t *testing.T, mk func(t *testing.T) (string, []BriefCell), write bool, opts holdOptions) holdOutcome {
	t.Helper()
	root, cells := mk(t)
	var st holdStats
	opts.stats = &st
	rows, held, err := reconcileWritesWith(root, cells, write, opts)
	rel := func(v any) string {
		b, jerr := json.Marshal(v)
		if jerr != nil {
			t.Fatal(jerr)
		}
		return strings.ReplaceAll(string(b), root, "<root>")
	}
	out := holdOutcome{Rows: rel(rows), Held: rel(held), Tree: map[string]string{}, Stats: st}
	if err != nil {
		out.Err = strings.ReplaceAll(err.Error(), root, "<root>")
	}
	werr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		r, _ := filepath.Rel(root, p)
		out.Tree[r] = string(b)
		return nil
	})
	if werr != nil {
		t.Fatal(werr)
	}
	return out
}

// assertHoldParity runs the fixture mk builds through the whole-board
// reference path (the hold as it was before #2615: every rule over the whole
// board for every move) and through the incremental path with the per-move
// cross-check on, in both modes, each on a fresh copy, and requires identical
// rows, holds, errors and resulting trees. It returns the incremental
// read-only outcome.
func assertHoldParity(t *testing.T, mk func(t *testing.T) (string, []BriefCell)) holdOutcome {
	t.Helper()
	var readOnly holdOutcome
	for _, write := range []bool{false, true} {
		ref := runHold(t, mk, write, holdOptions{reference: true})
		inc := runHold(t, mk, write, holdOptions{crossCheck: true})
		if strings.Contains(inc.Err, "lint hold self-check") {
			t.Fatalf("write=%v: %s", write, inc.Err)
		}
		if ref.Rows != inc.Rows {
			t.Fatalf("write=%v: rows differ from the whole-board reference:\nreference:   %s\nincremental: %s", write, ref.Rows, inc.Rows)
		}
		if ref.Held != inc.Held {
			t.Fatalf("write=%v: holds differ from the whole-board reference:\nreference:   %s\nincremental: %s", write, ref.Held, inc.Held)
		}
		if ref.Err != inc.Err {
			t.Fatalf("write=%v: errors differ:\nreference:   %q\nincremental: %q", write, ref.Err, inc.Err)
		}
		if !reflect.DeepEqual(ref.Tree, inc.Tree) {
			t.Fatalf("write=%v: the written tree differs from the whole-board reference's", write)
		}
		if !write {
			readOnly = inc
		}
	}
	return readOnly
}

// fixtureAllRows copies a committed fixture tree and witnesses EVERY brief row
// on it as implemented, so the hold judges every move the fixture allows.
func fixtureAllRows(dir string) func(t *testing.T) (string, []BriefCell) {
	return func(t *testing.T) (string, []BriefCell) {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS(dir)); err != nil {
			t.Fatal(err)
		}
		streams, _, err := loadStreams(root)
		if err != nil {
			return root, nil
		}
		var cells []BriefCell
		for _, s := range streams {
			for _, b := range s.Briefs {
				id := filepath.Base(s.Dir) + "/" + b.Num
				cells = append(cells, BriefCell{ID: id, Cell: "implemented", Source: "pr", Witness: "PR #1 " + id})
			}
		}
		return root, cells
	}
}

// TestHoldParityFixtures runs the parity over every committed fixture board.
func TestHoldParityFixtures(t *testing.T) {
	dirs, _ := filepath.Glob("testdata/*/docs/streams")
	if len(dirs) < 20 {
		t.Fatalf("fixture drift: want the committed fixture boards, found %d", len(dirs))
	}
	moved, heldAny := 0, 0
	for _, d := range dirs {
		fixture := filepath.Dir(filepath.Dir(d))
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			got := assertHoldParity(t, fixtureAllRows(fixture))
			t.Logf("moves=%d whole-board rule evaluations=%d", got.Stats.Moves, got.Stats.FullRuleEvals)
			if got.Rows != "null" {
				moved++
			}
			if got.Held != "null" {
				heldAny++
			}
		})
	}
	if moved == 0 || heldAny == 0 {
		t.Fatalf("fixture drift: the sweep must both admit and hold rows (fixtures with rows=%d, with holds=%d)", moved, heldAny)
	}
}

// TestHoldParityUnitBoards runs the parity on the hand-built boards the
// existing hold and wouldApply tests use, with their own cells.
func TestHoldParityUnitBoards(t *testing.T) {
	assertHoldParity(t, func(t *testing.T) (string, []BriefCell) {
		return copyDesignGateFixture(t), []BriefCell{
			{ID: "dg/05", Cell: "implemented", Source: "pr", Witness: "PR #5 (merged 0000005)"},
			{ID: "dg/03", Cell: "implemented", Source: "pr", Witness: "PR #3 (merged 0000003)"},
		}
	})
	assertHoldParity(t, func(t *testing.T) (string, []BriefCell) {
		return writeParityBoard(t), []BriefCell{
			{ID: "apstream/01", Cell: "implemented", Source: "pr", Witness: "PR #10"},
			{ID: "cell:repo:apstream:01", Cell: "implemented", Source: "pr", Witness: "PR #10"},
			{ID: "apstream/02", Cell: "implemented", Source: "backfill", Witness: "PR #11 backfill"},
			{ID: "apstream/03", Cell: "implemented", Source: "pr", Witness: "PR #12"},
			{ID: "apstream/04", Cell: "todo", Source: "pr", Reason: "no PR"},
			{ID: "bpstream/01", Cell: "implemented", Source: "backfill", Witness: "PR #13 backfill"},
			{ID: "nostream/01", Cell: "implemented", Source: "pr", Witness: "PR #14"},
		}
	})
	// A row a DEPLOY record names: the deploy rule is re-evaluated in full.
	got := assertHoldParity(t, func(t *testing.T) (string, []BriefCell) {
		root, cells := fixtureAllRows("testdata/deploygate")(t)
		p := filepath.Join(root, "docs", "streams", "dp", "README.md")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		todo := strings.Replace(string(b), "| S | implemented | — | — |", "| S | todo | — | — |", 1)
		if todo == string(b) {
			t.Fatal("fixture shape changed: could not set dp/02 to todo")
		}
		if err := os.WriteFile(p, []byte(todo), 0o644); err != nil {
			t.Fatal(err)
		}
		return root, cells
	})
	if got.Stats.Moves != 1 || got.Stats.FullRuleEvals != len(statusKeyedLintRules)+1 {
		t.Fatalf("fixture drift: want dp/02's one move to re-run only the deploy rule in full, got %+v", got.Stats)
	}
}

// heldManyRowBoard is manyRowBoard with every third stream's first brief made
// risk-gated (gate: human, irreversible) with no design record, so the design
// gate holds it while the rows around it move: later rows are judged against a
// board that does NOT carry the held row.
func heldManyRowBoard(t *testing.T) (string, []BriefCell) {
	root, cells := manyRowBoard(t, 12, 6, 3)
	dec := filepath.Join(root, "docs", "streams", "decisions")
	if err := os.MkdirAll(dec, 0o755); err != nil {
		t.Fatal(err)
	}
	dr, err := os.ReadFile("testdata/designgate/docs/streams/decisions/DR-fixture-approved.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dec, "DR-fixture-approved.md"), dr, 0o644); err != nil {
		t.Fatal(err)
	}
	for s := 0; s < 12; s += 3 {
		p := filepath.Join(root, "docs", "streams", fmt.Sprintf("mr%02d", s), "brief-01-b.md")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		gated := strings.Replace(string(b), "gate: model\nrisk: {regulatory: no, customer: no, irreversible: no,",
			"gate: human\ngate-why: touches a risk surface (fixture)\nrisk: {regulatory: no, customer: no, irreversible: yes,", 1)
		if gated == string(b) {
			t.Fatal("fixture shape changed: could not risk-gate brief 01")
		}
		if err := os.WriteFile(p, []byte(gated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Every fourth stream's second brief gets a scripted Verify row whose
	// script does not exist: harmless at todo, a stream-local PROBLEM once the
	// row moves past todo, so the row-class lint holds it.
	for s := 1; s < 12; s += 4 {
		name := fmt.Sprintf("mr%02d", s)
		p := filepath.Join(root, "docs", "streams", name, "brief-02-b.md")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		scripted := strings.Replace(string(b), "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | rc 0 |",
			"| # | Class | Command | Expect |\n|---|-------|---------|--------|\n| 1 | check:ci | docs/streams/"+name+"/verify.d/missing.sh | exit 0 |", 1)
		if scripted == string(b) {
			t.Fatal("fixture shape changed: could not script brief 02's Verify row")
		}
		if err := os.WriteFile(p, []byte(scripted), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, cells
}

// TestHoldParityManyRow is the parity on many rows across many streams, with
// cross-stream depends edges and holds from a whole-board rule (the design
// gate) and a stream-local one (the row-class lint) interleaved with admitted
// rows.
func TestHoldParityManyRow(t *testing.T) {
	got := assertHoldParity(t, heldManyRowBoard)
	var held []heldRow
	if err := json.Unmarshal([]byte(got.Held), &held); err != nil {
		t.Fatal(err)
	}
	var rows []appliedRow
	if err := json.Unmarshal([]byte(got.Rows), &rows); err != nil {
		t.Fatal(err)
	}
	if len(held) != 7 || len(rows) != 29 {
		t.Fatalf("fixture drift: want 7 held (4 risk-gated, 3 scripted) and 29 admitted, got held=%d rows=%d: %s", len(held), len(rows), got.Held)
	}
	for _, h := range held {
		p := strings.Join(h.Problems, "\n")
		designHeld := strings.HasSuffix(h.ID, "/01") && strings.Contains(p, "no design: record")
		scriptHeld := strings.HasSuffix(h.ID, "/02") && strings.Contains(p, "verify.d/missing.sh")
		if !designHeld && !scriptHeld {
			t.Fatalf("unexpected hold %+v", h)
		}
	}
	// One baseline pass of every rule, plus the decision-gate rule's whole-board
	// evaluation of each move of a gate: human brief's row (the fixture's four
	// risk-gated /01 briefs): that rule is the only one a move of such a row
	// reaches beyond its own stream.
	if bound := len(statusKeyedLintRules) + 4; got.Stats.FullRuleEvals > bound || got.Stats.Moves != 36 {
		t.Fatalf("36 moves took %d whole-board rule evaluations (want at most %d) over %d evaluated moves", got.Stats.FullRuleEvals, bound, got.Stats.Moves)
	}
}

// driveManyRowBoard is manyRowBoard with an active drive naming mr00 and a
// drive-plan page carrying that drive's snapshot region as rendered now: a
// move on mr00 changes the render (so the region drifts and the row is held),
// a move elsewhere does not.
func driveManyRowBoard(t *testing.T) (string, []BriefCell) {
	return driveBoard(t, false)
}

// drivePlaceholderBoard is driveManyRowBoard with a placeholder file in every
// stream, so each stream the hold mirrors carries a synthetic placeholder row
// after its README rows.
func drivePlaceholderBoard(t *testing.T) (string, []BriefCell) {
	return driveBoard(t, true)
}

func driveBoard(t *testing.T, placeholders bool) (string, []BriefCell) {
	root, cells := manyRowBoard(t, 4, 5, 2)
	if placeholders {
		for s := 0; s < 4; s++ {
			name := fmt.Sprintf("mr%02d", s)
			p := filepath.Join(root, "docs", "streams", name, fmt.Sprintf("issue-%d.md", 900+s))
			if err := os.WriteFile(p, []byte(placeholderFile(name, 900+s, "")), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeDrive(t, root, "d1", "declared-by: ian\n"+liveWindow+"intensity: focus\nstate: active\nwhy: fixture\nitems:\n  - stream: mr00\n  - stream: mr02\n")
	fresh, rc := driveSnapshotSection(root, "d1", driveTestNow)
	if rc != 0 {
		t.Fatalf("fixture drive d1 must render (rc %d)", rc)
	}
	writePlanMD(t, root, "d1", "# Drive d1\n\nprose\n\n"+driveSnapshotBeginPrefix+"d1 @deadbee 2026-08-15"+driveSnapshotMarkerSuffix+"\n"+fresh+driveSnapshotEndMarker+"\n")
	return root, cells
}

// TestHoldParityDriveRegion is the parity with the drive-snapshot region rule
// live: the one rule that re-renders from the tree, through its mirrored board.
func TestHoldParityDriveRegion(t *testing.T) {
	pinNow(t, driveTestNow)
	root, _ := driveManyRowBoard(t)
	if problems, _ := driveRegionLintProblems(root, driveTestNow); len(problems) != 0 {
		t.Fatalf("fixture drift: the fresh region must lint clean, got %v", problems)
	}
	got := assertHoldParity(t, driveManyRowBoard)
	if !strings.Contains(got.Held, "drive-region") {
		t.Fatalf("fixture drift: a move on a driven stream must drift the region and be held, got held=%s", got.Held)
	}
	if got.Rows == "null" {
		t.Fatal("fixture drift: moves on undriven streams must be admitted")
	}
	// The drive rule answered incrementally throughout: no whole-board
	// evaluation beyond the one baseline pass.
	if bound := len(statusKeyedLintRules); got.Stats.FullRuleEvals > bound {
		t.Fatalf("%d moves took %d whole-board rule evaluations; want at most %d", got.Stats.Moves, got.Stats.FullRuleEvals, bound)
	}
}

// TestHoldParityDrivePlaceholders is the drive-region parity on a board whose
// streams carry placeholder files: the hold must still answer the drive rule
// from its mirrored board, never fall back to a whole-board pass per move.
func TestHoldParityDrivePlaceholders(t *testing.T) {
	pinNow(t, driveTestNow)
	root, _ := drivePlaceholderBoard(t)
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range streams {
		if len(s.Placeholders) != 1 {
			t.Fatalf("fixture drift: stream %s must carry one placeholder, got %d", s.Name, len(s.Placeholders))
		}
	}
	if problems, _ := driveRegionLintProblems(root, driveTestNow); len(problems) != 0 {
		t.Fatalf("fixture drift: the fresh region must lint clean, got %v", problems)
	}
	got := assertHoldParity(t, drivePlaceholderBoard)
	if !strings.Contains(got.Held, "drive-region") || got.Rows == "null" {
		t.Fatalf("fixture drift: want driven moves held and undriven ones admitted, got held=%s rows=%s", got.Held, got.Rows)
	}
	if bound := len(statusKeyedLintRules); got.Stats.FullRuleEvals > bound {
		t.Fatalf("%d moves took %d whole-board rule evaluations; want at most %d — a placeholder row broke the drive rule's mirror", got.Stats.Moves, got.Stats.FullRuleEvals, bound)
	}
}

// TestHoldCrossCheckCatchesDrift proves the self-check the parity relies on
// can fail: a planted rule whose incremental form answers from a stale board
// while its whole-board form sees the move is an error, never a silent pass.
func TestHoldCrossCheckCatchesDrift(t *testing.T) {
	prev := statusKeyedLintRules
	planted := statusKeyedLintRule{
		name: "plantedStale",
		problems: func(e *lintEnv) []string {
			for _, s := range e.streams {
				for _, b := range s.Briefs {
					if s.Name == "dg" && b.Num == "03" && b.Status == "implemented" {
						return []string{"planted: dg/03 at implemented"}
					}
				}
			}
			return nil
		},
		incremental: statusInvariant, // a false claim: this rule does read Status
	}
	statusKeyedLintRules = append(append([]statusKeyedLintRule(nil), prev...), planted)
	t.Cleanup(func() { statusKeyedLintRules = prev })

	cells := []BriefCell{{ID: "dg/03", Cell: "implemented", Source: "pr", Witness: "PR #3 (merged 0000003)"}}
	_, _, err := reconcileWritesWith(copyDesignGateFixture(t), cells, false, holdOptions{crossCheck: true})
	if err == nil || !strings.Contains(err.Error(), "lint hold self-check") {
		t.Fatalf("the cross-check must reject a diverging incremental answer, got %v", err)
	}
	root := copyDesignGateFixture(t)
	before := readFixtureReadme(t, root)
	if _, _, err := reconcileWritesWith(root, cells, true, holdOptions{crossCheck: true}); err == nil {
		t.Fatal("the cross-check must fail --apply too")
	}
	if got := readFixtureReadme(t, root); got != before {
		t.Fatalf("a self-check failure must write nothing:\n%s", got)
	}
}
