package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// lintProblemLines runs the FULL offline lint (run(root, "lint", ...), the same
// entry the hourly job's lint step calls) over root and returns its PROBLEM lines.
func lintProblemLines(t *testing.T, root string) []string {
	t.Helper()
	out := captureRun(t, func() int { return run(root, "lint", nil, nil, "") })
	var lines []string
	for _, l := range strings.Split(out.err, "\n") {
		if strings.HasPrefix(l, "PROBLEM:") {
			lines = append(lines, l)
		}
	}
	sort.Strings(lines)
	return lines
}

// newProblemLines returns the lines in after that before does not carry
// (multiset difference), so a pre-existing fixture PROBLEM never masks a new one.
func newProblemLines(before, after []string) []string {
	seen := map[string]int{}
	for _, l := range before {
		seen[l]++
	}
	var fresh []string
	for _, l := range after {
		if seen[l] > 0 {
			seen[l]--
			continue
		}
		fresh = append(fresh, l)
	}
	return fresh
}

// copyDesignGateFixture lays the committed design-gate fixture tree into a temp
// root: dg/05 is a risk-gated, post-cutover brief still at todo with NO design
// record; dg/03 is gate: model (scoped out of the design gate) at in-progress.
func copyDesignGateFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/designgate")); err != nil {
		t.Fatal(err)
	}
	return root
}

func readFixtureReadme(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "docs", "streams", "dg", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestApplyThenLintAddsNoProblem is the end-to-end guard for the defect class
// "a Status-cell writer moves a row into a state the lint rejects": whatever
// --apply writes, the FULL lint over the written tree must carry no PROBLEM it
// did not carry before the write. Before the hold, a trailer-witnessed dg/05
// (risk-gated, no design record) was flipped todo -> implemented and the lint
// then failed on "has no design: record" — exactly the hourly job's own --lint
// step failing on the tree it had just written.
func TestApplyThenLintAddsNoProblem(t *testing.T) {
	root := copyDesignGateFixture(t)
	before := lintProblemLines(t, root)

	cells := []BriefCell{
		{ID: "dg/05", Cell: "implemented", Source: "pr", Witness: "PR #5 (merged 0000005)"},
		{ID: "dg/03", Cell: "implemented", Source: "pr", Witness: "PR #3 (merged 0000003)"},
	}
	applied, held, err := applyReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}

	after := lintProblemLines(t, root)
	if fresh := newProblemLines(before, after); len(fresh) != 0 {
		t.Fatalf("--apply wrote a tree the full lint rejects; new PROBLEM line(s):\n%s\napplied: %+v", strings.Join(fresh, "\n"), applied)
	}

	// The lint-clean flip still lands: holding is per row, never all-or-nothing.
	if len(applied) != 1 || applied[0].ID != "dg/03" {
		t.Fatalf("want exactly dg/03 applied (gate: model, scoped out of the design gate), got %+v", applied)
	}
	// The held row is REPORTED, with the PROBLEM it would have caused — never
	// dropped in silence and never written.
	if len(held) != 1 || held[0].ID != "dg/05" {
		t.Fatalf("want dg/05 held, got %+v", held)
	}
	if !strings.Contains(strings.Join(held[0].Problems, "\n"), "no design: record") {
		t.Fatalf("the held row must carry the PROBLEM it would cause, got %+v", held[0].Problems)
	}
	if !strings.Contains(readFixtureReadme(t, root), "| 05 | [Risk-gated, todo, no record](./brief-05-todo.md) | 0 | S | todo |") {
		t.Fatalf("a held row must stay byte-for-byte at todo:\n%s", readFixtureReadme(t, root))
	}
}

// TestApplyHoldClearsWhenTheRecordExists proves the hold keys on the lint's
// verdict, not on the brief's risk class: the same risk-gated dg/05 WITH an
// approved design record flips normally — no row is held merely for being
// risk-gated, and no design record is ever invented by the writer.
func TestApplyHoldClearsWhenTheRecordExists(t *testing.T) {
	root := copyDesignGateFixture(t)
	bp := filepath.Join(root, "docs", "streams", "dg", "brief-05-todo.md")
	b, err := os.ReadFile(bp)
	if err != nil {
		t.Fatal(err)
	}
	withRecord := strings.Replace(string(b), "issues: []\n", "issues: []\ndesign: DR-fixture-approved\n", 1)
	if withRecord == string(b) {
		t.Fatal("fixture shape changed: could not add a design: line to dg/05")
	}
	if err := os.WriteFile(bp, []byte(withRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	before := lintProblemLines(t, root)

	applied, held, err := applyReconcileWrites(root, []BriefCell{
		{ID: "dg/05", Cell: "implemented", Source: "pr", Witness: "PR #5 (merged 0000005)"},
	})
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}
	if len(held) != 0 || len(applied) != 1 {
		t.Fatalf("a risk-gated brief WITH its design record must flip, got applied=%+v held=%+v", applied, held)
	}
	if fresh := newProblemLines(before, lintProblemLines(t, root)); len(fresh) != 0 {
		t.Fatalf("new PROBLEM line(s) after a record-backed flip:\n%s", strings.Join(fresh, "\n"))
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "streams", "decisions")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "docs", "streams", "decisions"))
	if len(entries) != 1 {
		t.Fatalf("the writer must never create a design record; decisions/ now holds %d entries", len(entries))
	}
}

// TestTransitionProblemsCannotSeeRow is the could-not-check arm of the hold:
// a row the lint's own stream model does not carry cannot be evaluated, so the
// evaluator says so (found=false) and applyReconcileWrites holds it rather than
// writing on trust.
func TestTransitionProblemsCannotSeeRow(t *testing.T) {
	root := copyDesignGateFixture(t)
	env, err := loadLintEnv(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := env.transitionProblems("dg", "99", "implemented"); found {
		t.Fatal("a row absent from the lint model must report found=false")
	}
	if _, found := env.transitionProblems("dg", "05", "implemented"); !found {
		t.Fatal("dg/05 is in the lint model and must be found")
	}
}

// TestHoldParityReadOnly is the parity check with a held row in the fixture:
// on the design-gate tree the read-only form reports exactly the rows and the
// holds --apply then produces, and writes nothing itself. Without the hold in
// the shared path, the read-only form lists dg/05 as a row --apply would write.
func TestHoldParityReadOnly(t *testing.T) {
	root := copyDesignGateFixture(t)
	cells := []BriefCell{
		{ID: "dg/05", Cell: "implemented", Source: "pr", Witness: "PR #5 (merged 0000005)"},
		{ID: "dg/03", Cell: "implemented", Source: "pr", Witness: "PR #3 (merged 0000003)"},
	}
	before := readFixtureReadme(t, root)
	planned, plannedHeld, err := planReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("planReconcileWrites: %v", err)
	}
	if got := readFixtureReadme(t, root); got != before {
		t.Fatalf("the read-only form wrote to the tree:\n%s", got)
	}
	applied, appliedHeld, err := applyReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}
	if len(appliedHeld) != 1 || appliedHeld[0].ID != "dg/05" || len(applied) != 1 {
		t.Fatalf("fixture drift: want dg/03 applied and dg/05 held, got applied=%+v held=%+v", applied, appliedHeld)
	}
	if !reflect.DeepEqual(planned, applied) {
		t.Fatalf("wouldApply differs from applied:\nplanned: %+v\napplied: %+v", planned, applied)
	}
	if !reflect.DeepEqual(plannedHeld, appliedHeld) {
		t.Fatalf("the read-only holds differ from --apply's:\nplanned: %+v\napplied: %+v", plannedHeld, appliedHeld)
	}
}

// TestHoldTreeRuleSeesStage pins the staging the hold decides by: a rule that
// re-reads the board from disk (as the drive-snapshot region check does) must
// see the move under evaluation in BOTH forms, while nothing is written. A
// planted rule of that shape flags dg/03 at implemented; dg/03 must be held by
// the read-only form and by --apply alike, the README byte-for-byte unchanged.
func TestHoldTreeRuleSeesStage(t *testing.T) {
	prev := statusKeyedLintRules
	statusKeyedLintRules = append(append([]statusKeyedLintRule(nil), prev...), statusKeyedLintRule{
		name: "plantedTreeRule",
		problems: func(e *lintEnv) []string {
			streams, _, err := loadStreams(e.root)
			if err != nil {
				return []string{"planted: " + err.Error()}
			}
			for _, s := range streams {
				for _, b := range s.Briefs {
					if s.Name == "dg" && b.Num == "03" && b.Status == "implemented" {
						return []string{"planted: dg/03 read from the tree at implemented"}
					}
				}
			}
			return nil
		},
	})
	t.Cleanup(func() { statusKeyedLintRules = prev })

	cells := []BriefCell{{ID: "dg/03", Cell: "implemented", Source: "pr", Witness: "PR #3 (merged 0000003)"}}
	for _, write := range []bool{false, true} {
		root := copyDesignGateFixture(t)
		before := readFixtureReadme(t, root)
		rows, held, err := reconcileWrites(root, cells, write)
		if err != nil {
			t.Fatalf("write=%v: %v", write, err)
		}
		if len(rows) != 0 || len(held) != 1 || !strings.Contains(strings.Join(held[0].Problems, "\n"), "planted: dg/03") {
			t.Fatalf("write=%v: a tree-reading rule must see the staged move and hold dg/03, got rows=%+v held=%+v", write, rows, held)
		}
		if got := readFixtureReadme(t, root); got != before {
			t.Fatalf("write=%v: a held row left the README changed:\n%s", write, got)
		}
	}
	stagedReadmes.RLock()
	left := len(stagedReadmes.m)
	stagedReadmes.RUnlock()
	if left != 0 {
		t.Fatalf("reconcileWrites left %d staged README(s) behind", left)
	}
}
