package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// copyTestdataFixture copies the checked-in planted-content fixture
// testdata/instrbrittle/<name>/ into dstDir, so the planted references and
// the planted doc-drifts-from-code content are reviewable directly as files
// rather than as Go string literals; the test harness then drives the git
// history (commits, later code-only changes) on top of the copy.
func copyTestdataFixture(t *testing.T, name, dstDir string) {
	t.Helper()
	srcRoot := filepath.Join("testdata", "instrbrittle", name)
	err := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcRoot, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture %s: %v", name, err)
	}
}

// appendFile appends content to an existing file — how the staledoc fixture
// plants its "code changes N times, doc unchanged" history.
func appendFile(t *testing.T, dir, name, content string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s for append: %v", name, err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("append %s: %v", name, err)
	}
}

// mineFixture builds a Store by mining repoDir into a fresh tracking root.
func mineFixture(t *testing.T, repoDir string) *Store {
	t.Helper()
	out := t.TempDir()
	if err := mine(repoDir, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine: %v", err)
	}
	return NewStore(out)
}

// TestReferenceValidity_PlantedDeadRefs is Verify row 3: the pass reports
// EXACTLY the planted dead-reference count from testdata/instrbrittle/deadrefs
// (one dead file path, one dead symbol, one dead typed ID), the one live
// reference as valid, and the unclassifiable one as could-not-measure — a
// dereference of the known-answer fixture, not a presence check.
func TestReferenceValidity_PlantedDeadRefs(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "deadrefs", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "add planted reference fixture")

	store := mineFixture(t, dir)
	cfg := InstructionBrittleConfig{Globs: []string{"doc.md"}, WindowCount: 1}
	report, err := RunInstructionBrittleness(dir, store, cfg)
	if err != nil {
		t.Fatalf("RunInstructionBrittleness: %v", err)
	}
	if report.Configured.State != StateMeasured || !report.Configured.Value {
		t.Fatalf("Configured = %+v, want Measured(true)", report.Configured)
	}
	if len(report.Trend) != 1 {
		t.Fatalf("Trend has %d windows, want 1 (WindowCount=1)", len(report.Trend))
	}
	w := report.Trend[0]
	if w.Live != 1 {
		t.Errorf("Live = %d, want 1", w.Live)
	}
	if w.Dead != 3 {
		t.Errorf("Dead = %d, want 3", w.Dead)
	}
	if w.CouldNotMeasure != 1 {
		t.Errorf("CouldNotMeasure = %d, want 1", w.CouldNotMeasure)
	}

	// Dereference the mix BY KIND, not just the tally: exactly one dead
	// file-path, one dead symbol, one dead typed-ID, one live file-path, and
	// one could-not-classify reference.
	var deadPath, deadSymbol, deadTypedID, livePath, unclassifiable int
	for _, ref := range w.Refs {
		switch {
		case ref.Validity.State == StateCouldNotMeasure:
			unclassifiable++
		case ref.Validity.State == StateMeasured && !ref.Validity.Value && ref.Target.Kind == TargetFilePath:
			deadPath++
		case ref.Validity.State == StateMeasured && !ref.Validity.Value && ref.Target.Kind == TargetSymbol:
			deadSymbol++
		case ref.Validity.State == StateMeasured && !ref.Validity.Value && ref.Target.Kind == TargetTypedID:
			deadTypedID++
		case ref.Validity.State == StateMeasured && ref.Validity.Value && ref.Target.Kind == TargetFilePath:
			livePath++
		}
	}
	for name, got := range map[string]int{
		"dead file-path":     deadPath,
		"dead symbol":        deadSymbol,
		"dead typed-id":      deadTypedID,
		"live file-path":     livePath,
		"could-not-classify": unclassifiable,
	} {
		if got != 1 {
			t.Errorf("%s count = %d, want exactly 1 (got %d refs total)", name, got, len(w.Refs))
		}
	}
}

// TestDocCodeStaleness_PlantedDrift is Verify row 4: the doc↔code pass flags
// the planted stale doc (code changed N times, doc unchanged) as
// presumptively stale with co-change count = N.
func TestDocCodeStaleness_PlantedDrift(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "staledoc", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "establish doc-code coupling")
	establishSHA := gitCmd(t, dir, "rev-parse", "HEAD")

	const codeOnlyChanges = 5
	for i := 1; i <= codeOnlyChanges; i++ {
		appendFile(t, dir, "lib/thing.go", fmt.Sprintf("\n// change %d\n", i))
		gitCmd(t, dir, "add", "lib/thing.go")
		gitCmd(t, dir, "commit", "-q", "-m", fmt.Sprintf("code-only change %d", i))
	}

	store := mineFixture(t, dir)
	cfg := InstructionBrittleConfig{Globs: []string{"doc.md"}, StaleCoChangeThreshold: 2}
	report, err := RunInstructionBrittleness(dir, store, cfg)
	if err != nil {
		t.Fatalf("RunInstructionBrittleness: %v", err)
	}

	var found *StalenessResult
	for i := range report.Staleness {
		if report.Staleness[i].Pair.CodePath == "lib/thing.go" {
			found = &report.Staleness[i]
		}
	}
	if found == nil {
		t.Fatalf("no staleness result for lib/thing.go among %d results", len(report.Staleness))
	}
	if found.Pair.DocPath != "doc.md" {
		t.Errorf("DocPath = %q, want doc.md", found.Pair.DocPath)
	}
	if found.Pair.EstablishedAt != establishSHA {
		t.Errorf("EstablishedAt = %q, want %q", found.Pair.EstablishedAt, establishSHA)
	}
	if found.CodeOnlyChanges != codeOnlyChanges {
		t.Errorf("CodeOnlyChanges = %d, want %d (the planted N)", found.CodeOnlyChanges, codeOnlyChanges)
	}
	if !found.Stale {
		t.Error("expected Stale = true (CodeOnlyChanges exceeds the configured threshold)")
	}
}

// TestReferenceValidity_TrendedNotSnapshot is Verify row 5: reference-validity
// is emitted per window (>= 2 windows over the fixture history), proving it
// is trended rather than a single snapshot.
func TestReferenceValidity_TrendedNotSnapshot(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "staledoc", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "establish doc-code coupling")

	for i := 1; i <= 5; i++ {
		appendFile(t, dir, "lib/thing.go", fmt.Sprintf("\n// change %d\n", i))
		gitCmd(t, dir, "add", "lib/thing.go")
		gitCmd(t, dir, "commit", "-q", "-m", fmt.Sprintf("code-only change %d", i))
	}

	store := mineFixture(t, dir)
	cfg := InstructionBrittleConfig{Globs: []string{"doc.md"}, WindowCount: 3}
	report, err := RunInstructionBrittleness(dir, store, cfg)
	if err != nil {
		t.Fatalf("RunInstructionBrittleness: %v", err)
	}
	if len(report.Trend) < 2 {
		t.Fatalf("Trend has %d windows, want >= 2 — trended, not a single snapshot", len(report.Trend))
	}
	seen := map[string]bool{}
	for _, w := range report.Trend {
		if seen[w.AtSHA] {
			t.Fatalf("window %d repeats commit %s — not a real per-window trend", w.Index, w.AtSHA)
		}
		seen[w.AtSHA] = true
	}
	first, last := report.Trend[0], report.Trend[len(report.Trend)-1]
	if first.AtSHA == last.AtSHA {
		t.Fatal("first and last window resolved the same commit — not trended")
	}
}

// TestInstructionSet_Unconfigured_ThreeState is Verify row 7: an
// empty/unconfigured instruction-doc set yields could-not-measure, never
// measured-zero.
func TestInstructionSet_Unconfigured_ThreeState(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "deadrefs", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "add fixture")

	store := mineFixture(t, dir)
	report, err := RunInstructionBrittleness(dir, store, InstructionBrittleConfig{})
	if err != nil {
		t.Fatalf("RunInstructionBrittleness: %v", err)
	}
	if report.Configured.State != StateCouldNotMeasure {
		t.Fatalf("Configured.State = %q, want could-not-measure", report.Configured.State)
	}
	if report.Configured.Reason == "" {
		t.Error("a could-not-measure Configured must carry a reason")
	}
	if report.Trend != nil {
		t.Errorf("Trend = %v, want nil (unconfigured must never manufacture a measured-zero)", report.Trend)
	}
	if report.Staleness != nil {
		t.Errorf("Staleness = %v, want nil", report.Staleness)
	}
}

// TestInstructionBrittleness_EndToEndOnConfiguredFixture is part of Verify
// row 2's `-run InstructionBrittle` filter: a smoke test that a configured,
// mined fixture runs cleanly end to end (the exact-count dereferences live in
// the row-3/4/5/7 tests above).
func TestInstructionBrittleness_EndToEndOnConfiguredFixture(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	copyTestdataFixture(t, "deadrefs", dir)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "add fixture")

	store := mineFixture(t, dir)
	report, err := RunInstructionBrittleness(dir, store, InstructionBrittleConfig{Globs: []string{"doc.md"}})
	if err != nil {
		t.Fatalf("RunInstructionBrittleness: %v", err)
	}
	if report.Configured.State != StateMeasured || !report.Configured.Value {
		t.Fatalf("Configured = %+v, want Measured(true) on a configured run", report.Configured)
	}
	if len(report.Trend) == 0 {
		t.Fatal("expected at least one trend window on a configured, mined fixture")
	}
}
