package main

import (
	"bytes"
	"testing"
	"time"
)

// --- direct dereferencing tests for the shared FileFeatures assembly
// (features.go, authored by brief 09; this file is brief 08's own test
// coverage of it, since brief 09's own suite exercises it only indirectly
// through `check`). Reuses hotspotFixtureRepo/couplingFixtureRepo
// (check_test.go), mineFixture (instructionbrittle_test.go), and
// szzGit/commitFile/identifiedFix/openRepo (szz_test.go) — all in this
// package. ---

// TestFeatures_HotspotPercentileRanksChurnedAboveQuiet dereferences
// AssembleFileFeatures directly against the same known-answer fixture
// hotspot_test.go/check_test.go use: the churned, deeply-nested file must
// rank strictly above the quiet one, both measured.
func TestFeatures_HotspotPercentileRanksChurnedAboveQuiet(t *testing.T) {
	requireGit(t)
	dir := hotspotFixtureRepo(t)
	store := mineFixture(t, dir)

	hot, err := AssembleFileFeatures(store, "hot.go")
	if err != nil {
		t.Fatalf("AssembleFileFeatures(hot.go): %v", err)
	}
	quiet, err := AssembleFileFeatures(store, "quiet.go")
	if err != nil {
		t.Fatalf("AssembleFileFeatures(quiet.go): %v", err)
	}
	if hot.HotspotPercentile.State != StateMeasured || quiet.HotspotPercentile.State != StateMeasured {
		t.Fatalf("expected both measured, got hot=%+v quiet=%+v", hot.HotspotPercentile, quiet.HotspotPercentile)
	}
	if !(hot.HotspotPercentile.Value > quiet.HotspotPercentile.Value) {
		t.Fatalf("hot.go percentile %.3f must strictly exceed quiet.go's %.3f", hot.HotspotPercentile.Value, quiet.HotspotPercentile.Value)
	}
}

// TestFeatures_CouplingPartnersHistorical dereferences the coupling join
// directly: a.go's CouplingPartners must contain b.go from the fixture's
// three co-changed commits.
func TestFeatures_CouplingPartnersHistorical(t *testing.T) {
	requireGit(t)
	dir := couplingFixtureRepo(t)
	store := mineFixture(t, dir)

	a, err := AssembleFileFeatures(store, "a.go")
	if err != nil {
		t.Fatalf("AssembleFileFeatures(a.go): %v", err)
	}
	if !contains(a.CouplingPartners, "b.go") {
		t.Fatalf("a.go CouplingPartners must contain b.go, got %v", a.CouplingPartners)
	}
}

// TestFeatures_UnknownPathIsCouldNotMeasure dereferences the three-state
// floor: a path never seen by any mine run carries could-not-measure on
// every scalar family, an empty coupling list, and Unmeasured() true — never
// a silent zero standing in for absence.
func TestFeatures_UnknownPathIsCouldNotMeasure(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "seen.go", "package x\n")
	gitCmd(t, dir, "add", "seen.go")
	gitCmd(t, dir, "commit", "-q", "-m", "add seen.go")
	store := mineFixture(t, dir)

	f, err := AssembleFileFeatures(store, "never/seen.go")
	if err != nil {
		t.Fatalf("AssembleFileFeatures: %v", err)
	}
	if f.HotspotPercentile.State != StateCouldNotMeasure {
		t.Errorf("HotspotPercentile = %+v, want could-not-measure", f.HotspotPercentile)
	}
	if f.OwnershipTop.State != StateCouldNotMeasure {
		t.Errorf("OwnershipTop = %+v, want could-not-measure", f.OwnershipTop)
	}
	if f.DefectDensity.State != StateCouldNotMeasure {
		t.Errorf("DefectDensity = %+v, want could-not-measure", f.DefectDensity)
	}
	if len(f.CouplingPartners) != 0 {
		t.Errorf("CouplingPartners = %v, want empty", f.CouplingPartners)
	}
	if !f.Unmeasured() {
		t.Errorf("Unmeasured() = false, want true for a path with no mined signal at all")
	}
}

// TestFeatures_DefectDensityTravelsWithTraceRate dereferences the
// honest-claims pairing directly on the shared assembly: a traced defect on
// app.go produces a DefectDensity and DefectTraceRate that carry the SAME
// Measure state — density is never measured while its trace-rate is
// could-not-measure, or vice versa.
func TestFeatures_DefectDensityTravelsWithTraceRate(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	_ = commitFile(t, dir, "2020-01-01T00:00:00Z", "app.go", "package app\nreturn x / 0\ntail\n", "A: introduce defect")
	f := commitFile(t, dir, "2020-01-05T00:00:00Z", "app.go", "package app\nreturn x / y\ntail\n", "fix: guard divisor (#21)")

	repo := openRepo(t, dir)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 21, 21)}, nil)

	out := t.TempDir()
	if err := mine(dir, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine: %v", err)
	}
	store := NewStore(out)
	if err := corpus.WriteTo(store, 4, time.Now().UTC()); err != nil {
		t.Fatalf("write corpus: %v", err)
	}

	features, err := AssembleFileFeatures(store, "app.go")
	if err != nil {
		t.Fatalf("AssembleFileFeatures(app.go): %v", err)
	}
	if features.DefectDensity.State != features.DefectTraceRate.State {
		t.Fatalf("DefectDensity state %q and DefectTraceRate state %q must match (honest-claims: never one without the other)", features.DefectDensity.State, features.DefectTraceRate.State)
	}
	if features.DefectDensity.State != StateMeasured {
		t.Fatalf("expected a measured defect_density over the traced defect, got %+v", features.DefectDensity)
	}
}
