package reflex

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/qualgen/attribution"
)

// ---------------------------------------------------------------------------
// Verify #3 (DEREFERENCING): a fixture lane with 8 pre-merge catches and 2
// escapes (per the planted review-escape overlay) yields catch-rate EXACTLY
// 0.8 and escape-rate EXACTLY 0.2 — specific asserted values, not presence.
// ---------------------------------------------------------------------------

func TestGateYieldFixture(t *testing.T) {
	// Two escapes: real attribution.LedgerEntry rows whose ReviewEscape
	// overlay names lane "security" — run through attribution.RollupOf,
	// brief-10's own tested aggregation, never re-derived here.
	entries := []attribution.LedgerEntry{
		{
			DefectID:     "pr-101",
			Stream:       "quality",
			Stage:        attribution.StageImplementation,
			ReviewEscape: attribution.ReviewEscape{Lanes: []string{"security"}},
			DossierHash:  "hash-101",
		},
		{
			DefectID:     "pr-102",
			Stream:       "quality",
			Stage:        attribution.StageBrief,
			ReviewEscape: attribution.ReviewEscape{Lanes: []string{"security"}},
			DossierHash:  "hash-102",
		},
		// A defect escaped through a DIFFERENT lane — must not pollute
		// security's counts.
		{
			DefectID:     "pr-103",
			Stream:       "quality",
			Stage:        attribution.StageImplementation,
			ReviewEscape: attribution.ReviewEscape{Lanes: []string{"style"}},
			DossierHash:  "hash-103",
		},
	}

	// 8 pre-merge catches: security flagged "qualgen/reflex" on 8 distinct
	// PRs, and a later-traced defect surface confirms that surface was
	// genuinely defect-prone.
	var findings []PreMergeFinding
	for i := 0; i < 8; i++ {
		findings = append(findings, PreMergeFinding{Lane: "security", Surface: []string{"qualgen/reflex"}})
	}
	// A finding with no matching later trace must NOT count as a catch.
	findings = append(findings, PreMergeFinding{Lane: "security", Surface: []string{"qualgen/nevertraced"}})

	defectSurfaces := []DefectSurface{
		{DefectID: "pr-201", Surface: []string{"qualgen/reflex", "qualgen/attribution"}},
	}

	in := GateYieldInput{
		Findings:       findings,
		LedgerEntries:  entries,
		DefectSurfaces: defectSurfaces,
	}

	lanes := ComputeGateYield(in)

	var security *LaneYield
	for i := range lanes {
		if lanes[i].Lane == "security" {
			security = &lanes[i]
		}
	}
	if security == nil {
		t.Fatalf("expected a %q lane in output, got %+v", "security", lanes)
	}
	if security.Catches != 8 {
		t.Fatalf("catches: want 8, got %d", security.Catches)
	}
	if security.Escapes != 2 {
		t.Fatalf("escapes: want 2, got %d", security.Escapes)
	}
	if security.CatchRate.State != StateMeasured || security.CatchRate.Value != 0.8 {
		t.Fatalf("catch-rate: want measured 0.8, got %+v", security.CatchRate)
	}
	if security.EscapeRate.State != StateMeasured || security.EscapeRate.Value != 0.2 {
		t.Fatalf("escape-rate: want measured 0.2, got %+v", security.EscapeRate)
	}

	// The "style" lane escape must not be attributed to security.
	var style *LaneYield
	for i := range lanes {
		if lanes[i].Lane == "style" {
			style = &lanes[i]
		}
	}
	if style == nil {
		t.Fatalf("expected a %q lane in output, got %+v", "style", lanes)
	}
	if style.Catches != 0 || style.Escapes != 1 {
		t.Fatalf("style lane: want 0 catches / 1 escape, got %+v", style)
	}
	if style.CatchRate.State != StateMeasured || style.CatchRate.Value != 0.0 {
		t.Fatalf("style catch-rate: want measured 0.0, got %+v", style.CatchRate)
	}
	if style.EscapeRate.State != StateMeasured || style.EscapeRate.Value != 1.0 {
		t.Fatalf("style escape-rate: want measured 1.0, got %+v", style.EscapeRate)
	}
}

func TestGateYieldFixture_NoJudgedItemsIsCouldNotMeasure(t *testing.T) {
	in := GateYieldInput{
		Findings: []PreMergeFinding{{Lane: "correctness", Surface: []string{"unmatched"}}},
	}
	lanes := ComputeGateYield(in)
	if len(lanes) != 1 {
		t.Fatalf("want 1 lane, got %d: %+v", len(lanes), lanes)
	}
	if lanes[0].CatchRate.State != StateCouldNotMeasure {
		t.Fatalf("a lane with zero catches and zero escapes must be could-not-measure, got %+v", lanes[0].CatchRate)
	}
	if lanes[0].EscapeRate.State != StateCouldNotMeasure {
		t.Fatalf("a lane with zero catches and zero escapes must be could-not-measure, got %+v", lanes[0].EscapeRate)
	}
}

func TestGateYieldLatencyAggregation(t *testing.T) {
	in := GateYieldInput{
		LedgerEntries: []attribution.LedgerEntry{
			{DefectID: "pr-1", ReviewEscape: attribution.ReviewEscape{Lanes: []string{"security"}}},
		},
		Latency: []LatencySample{
			{Lane: "security", Hours: Measured(2.0)},
			{Lane: "security", Hours: Measured(4.0)},
			{Lane: "style", Hours: CouldNotMeasure[float64]("no timestamp")},
		},
	}
	lanes := ComputeGateYield(in)
	byLane := map[string]LaneYield{}
	for _, l := range lanes {
		byLane[l.Lane] = l
	}
	if got := byLane["security"].LatencyCost; got.State != StateMeasured || got.Value != 3.0 {
		t.Fatalf("security latency: want measured 3.0, got %+v", got)
	}
	if got := byLane["style"].LatencyCost; got.State != StateCouldNotMeasure {
		t.Fatalf("style latency (samples present but unmeasured): want could-not-measure, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Verify #6: TestReviewEscapeOverlayThreeState — a defect with no
// review-escape overlay entry is emitted could-not-join, never attributed to
// a lane by guess.
// ---------------------------------------------------------------------------

func TestReviewEscapeOverlayThreeState(t *testing.T) {
	index := BuildLedgerIndex([]attribution.LedgerEntry{
		{DefectID: "pr-1", ReviewEscape: attribution.ReviewEscape{Lanes: []string{"security", "correctness"}}},
		{DefectID: "pr-2", ReviewEscape: attribution.ReviewEscape{Lanes: []string{}}},
	})

	// A defect PRESENT on the ledger with lanes: measured, real lanes.
	got := ReviewEscapeJoin("pr-1", index)
	if got.State != StateMeasured {
		t.Fatalf("pr-1: want measured, got %+v", got)
	}
	if len(got.Value) != 2 || got.Value[0] != "correctness" || got.Value[1] != "security" {
		t.Fatalf("pr-1 lanes: want [correctness security], got %v", got.Value)
	}

	// A defect present on the ledger with an EMPTY (but recorded) overlay:
	// still measured — an honest empty fact, not a gap.
	empty := ReviewEscapeJoin("pr-2", index)
	if empty.State != StateMeasured {
		t.Fatalf("pr-2 (empty overlay, recorded): want measured, got %+v", empty)
	}
	if len(empty.Value) != 0 {
		t.Fatalf("pr-2 lanes: want empty, got %v", empty.Value)
	}

	// A defect with NO ledger entry at all: could-not-join, never a guessed
	// empty-lanes escape.
	missing := ReviewEscapeJoin("pr-999-never-attributed", index)
	if missing.State != StateCouldNotMeasure {
		t.Fatalf("pr-999: want could-not-measure (could-not-join), got %+v", missing)
	}
	if !strings.Contains(missing.Reason, "could-not-join") {
		t.Fatalf("pr-999 reason should name could-not-join, got %q", missing.Reason)
	}
}

// ---------------------------------------------------------------------------
// Verify #7: TestNoNewMining — the join reads only M1/M2/M3 artifact
// fixtures; no git-history access path is invoked (mining seam is not
// called). Enforced structurally: this package's own non-test source files
// must never import a history-mining or ambient-IO capability.
// ---------------------------------------------------------------------------

func TestNoNewMining(t *testing.T) {
	forbidden := []string{"go-git", "os/exec", "net/http", `"net"`, "io/fs"}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read reflex package dir: %v", err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		checked++
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				bad = strings.Trim(bad, `"`)
				if strings.Contains(path, bad) {
					t.Errorf("%s imports %q — a mining/ambient-IO capability leaking into the M4 join layer (spec §7: no new mining)", name, path)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no reflex source files found to check — this test would pass vacuously")
	}
}
