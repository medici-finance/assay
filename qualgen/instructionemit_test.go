package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// mineFixtureWithInstruction mines repoDir into a fresh tracking root with the
// instruction-brittleness pass configured over globs, so the emitted metrics
// table carries the reference-validity + doc↔code staleness families. It is the
// production path runMine drives (mineWithConfig), not the pass called in
// isolation — this is what pins that the miner actually EMITS the numbers.
func mineFixtureWithInstruction(t *testing.T, repoDir string, cfg InstructionBrittleConfig) *Store {
	t.Helper()
	out := t.TempDir()
	m1 := DefaultM1Config()
	m1.Instruction = cfg
	if err := mineWithConfig(repoDir, out, &bytes.Buffer{}, m1); err != nil {
		t.Fatalf("mineWithConfig: %v", err)
	}
	return NewStore(out)
}

// TestInstructionBrittleness_EmittedToMetrics pins the FOLLOW-UP: the miner
// EMITS the instruction reference-validity numbers into the append-only metrics
// table (not just computes them in-process). It dereferences the deadrefs
// fixture's known answer — Live=1, Dead=3, could-not-measure=1, validity 0.25 —
// off the committed artifacts, the same known-answer the brief-04 pass test
// asserts in-process.
func TestInstructionBrittleness_EmittedToMetrics(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "deadrefs", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "add planted reference fixture")

	store := mineFixtureWithInstruction(t, dir, InstructionBrittleConfig{Globs: []string{"doc.md"}, WindowCount: 1})
	recs, err := store.ReadReferenceValidity()
	if err != nil {
		t.Fatalf("ReadReferenceValidity: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("emitted %d reference-validity records, want 1 window (WindowCount=1); recs=%+v", len(recs), recs)
	}
	w := recs[0]
	if w.Live != 1 || w.Dead != 3 || w.CouldNotMeasure != 1 {
		t.Fatalf("emitted counts Live=%d Dead=%d CNM=%d, want 1/3/1 (deadrefs planted answer)", w.Live, w.Dead, w.CouldNotMeasure)
	}
	if w.Validity.State != StateMeasured {
		t.Fatalf("Validity state = %q, want measured; %+v", w.Validity.State, w.Validity)
	}
	if got := w.Validity.Value; got != 0.25 {
		t.Fatalf("emitted reference-validity = %v, want 0.25 (1 live / (1 live + 3 dead))", got)
	}
	if w.AtSHA == "" {
		t.Fatal("emitted window record has empty AtSHA; a measured window must carry its representative commit")
	}
}

// TestReport_RendersReferenceValidityTrend is the render half of the follow-up:
// QUALITY.md now carries the REAL reference-validity trend where it used to carry
// the "not yet emitted to the metrics table" placeholder. It dereferences the
// emitted 0.25 into the rendered view, and pins that the stale placeholder text
// is gone.
func TestReport_RendersReferenceValidityTrend(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "deadrefs", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "add planted reference fixture")

	store := mineFixtureWithInstruction(t, dir, InstructionBrittleConfig{Globs: []string{"doc.md"}, WindowCount: 1})
	view, err := renderReport(store, BuiltinBaselines())
	if err != nil {
		t.Fatalf("renderReport: %v", err)
	}
	if strings.Contains(view, "not yet emitted to the metrics table") {
		t.Fatalf("stale placeholder still rendered — the brittleness family must now render real numbers:\n%s", view)
	}
	if !strings.Contains(view, "| Window | At commit | Live refs | Dead refs | Unmeasured | Reference-validity |") {
		t.Fatalf("reference-validity trend table header missing:\n%s", view)
	}
	// The dereference: the emitted validity 0.25 renders in the window row.
	if !strings.Contains(view, "| 1 | 3 | 1 | 0.25 |") {
		t.Fatalf("expected the window row to carry Live=1 Dead=3 Unmeasured=1 validity=0.25; view:\n%s", view)
	}
}

// TestInstructionBrittleness_UnconfiguredEmitsMarker pins three-state honesty
// (spec §4.6 fact 1): a mine with NO instruction-doc glob set emits a single
// could-not-measure reference-validity marker — never a silent zero — and the
// report renders it as unmeasured.
func TestInstructionBrittleness_UnconfiguredEmitsMarker(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "deadrefs", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "add planted reference fixture")

	// Default mine (no instruction globs configured).
	store := mineFixture(t, dir)
	recs, err := store.ReadReferenceValidity()
	if err != nil {
		t.Fatalf("ReadReferenceValidity: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("unconfigured mine emitted %d records, want 1 could-not-measure marker; recs=%+v", len(recs), recs)
	}
	if recs[0].Validity.State != StateCouldNotMeasure {
		t.Fatalf("unconfigured marker state = %q, want could-not-measure (never a silent zero)", recs[0].Validity.State)
	}
	if recs[0].AtSHA != "" {
		t.Fatalf("unconfigured marker should carry no window (empty AtSHA), got %q", recs[0].AtSHA)
	}

	view, err := renderReport(store, BuiltinBaselines())
	if err != nil {
		t.Fatalf("renderReport: %v", err)
	}
	if !strings.Contains(view, "## Instruction reference-validity trend\n\nnot measured — ") {
		t.Fatalf("unconfigured brittleness should render as 'not measured — <reason>'; view:\n%s", view)
	}
}

// TestDocCodeStaleness_EmittedAndRenderedAlarm pins that the doc↔code staleness
// family is emitted to the metrics table and its stale pairs render as alarms in
// QUALITY.md (the planted staledoc history: code moves 5 times, doc unchanged).
func TestDocCodeStaleness_EmittedAndRenderedAlarm(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "staledoc", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "establish doc-code coupling")
	const codeOnlyChanges = 5
	for i := 1; i <= codeOnlyChanges; i++ {
		appendFile(t, dir, "lib/thing.go", fmt.Sprintf("\n// change %d\n", i))
		gitCmd(t, dir, "add", "lib/thing.go")
		gitCmd(t, dir, "commit", "-q", "-m", fmt.Sprintf("code-only change %d", i))
	}

	store := mineFixtureWithInstruction(t, dir, InstructionBrittleConfig{Globs: []string{"doc.md"}, StaleCoChangeThreshold: 2})
	recs, err := store.ReadDocCodeStaleness()
	if err != nil {
		t.Fatalf("ReadDocCodeStaleness: %v", err)
	}
	var found *DocCodeStalenessRecord
	for i := range recs {
		if recs[i].CodePath == "lib/thing.go" {
			found = &recs[i]
		}
	}
	if found == nil {
		t.Fatalf("no emitted staleness record for lib/thing.go among %d records", len(recs))
	}
	if !found.Stale || found.CodeOnlyChanges != codeOnlyChanges {
		t.Fatalf("emitted staleness = {Stale:%v CodeOnlyChanges:%d}, want {true %d}", found.Stale, found.CodeOnlyChanges, codeOnlyChanges)
	}

	view, err := renderReport(store, BuiltinBaselines())
	if err != nil {
		t.Fatalf("renderReport: %v", err)
	}
	if !strings.Contains(view, "### Doc↔code staleness alarms") {
		t.Fatalf("staleness alarm section missing:\n%s", view)
	}
	if !strings.Contains(view, "| doc.md | lib/thing.go | 5 |") {
		t.Fatalf("expected the stale doc↔code pair rendered as an alarm row; view:\n%s", view)
	}
}
