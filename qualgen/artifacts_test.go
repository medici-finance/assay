package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestArtifacts_AppendOnly is Verify #6: a second emit run APPENDS to
// metrics.jsonl — the record count strictly increases and the prior records are
// left byte-for-byte unchanged — never rewriting the baseline.
func TestArtifacts_AppendOnly(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	rec := func(v float64, at time.Time) MetricRecord {
		return MetricRecord{
			Metric: MetricChurnRate, Grain: GrainRepo,
			Value: Measured(v), Basis: basisPublishedDefinitions, Note: honestClaimsNote,
			MinedAt: at,
		}
	}
	if err := store.Append(KindMetric, rec(0.03, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	path, _ := store.tablePath(KindMetric)
	firstBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	firstCount := len(readAll(t, store))

	// Second emit run: append a fresh snapshot.
	if err := store.Append(KindMetric, rec(0.02, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	secondCount := len(readAll(t, store))
	if secondCount <= firstCount {
		t.Fatalf("second emit must strictly increase record count: was %d, now %d", firstCount, secondCount)
	}

	// The prior record's bytes are a strict PREFIX of the new file — untouched.
	secondBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(secondBytes), string(firstBytes)) {
		t.Fatalf("append rewrote the prior baseline; prior bytes are not a prefix of the new file")
	}

	// A trend consumer reads the LATEST snapshot per key.
	latest := latestOf(readAll(t, store),
		func(m MetricRecord) string { return m.Metric + "|" + m.Grain + "|" + m.Key },
		func(m MetricRecord) time.Time { return m.MinedAt })
	got := latest[MetricChurnRate+"|"+GrainRepo+"|"]
	if got.Value.State != StateMeasured || got.Value.Value != 0.02 {
		t.Fatalf("latest snapshot should be the 0.02 record, got %+v", got.Value)
	}
}

func readAll(t *testing.T, store *Store) []MetricRecord {
	t.Helper()
	recs, err := store.ReadMetrics()
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

// TestArtifacts_ThreeStateField is Verify #7: a could-not-measure metric
// serializes with the unmeasured state (round-trips as could-not-measure, never
// as a silent zero) and renders as unmeasured — never `0`.
func TestArtifacts_ThreeStateField(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	// A could-not-measure duplicate-block rate at the repo grain.
	cnm := MetricRecord{
		Metric: MetricDuplicateBlockRate, Grain: GrainRepo,
		Value:   CouldNotMeasure[float64]("no classified added lines"),
		Basis:   basisPublishedDefinitions, Note: honestClaimsNote,
		MinedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	// Provide a measured copy/paste so the render has a non-trivial table.
	cp := MetricRecord{
		Metric: MetricCopyPasteRatio, Grain: GrainRepo,
		Value:   Measured(0.42),
		Basis:   basisPublishedDefinitions, Note: honestClaimsNote,
		MinedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, r := range []MetricRecord{cnm, cp} {
		if err := store.Append(KindMetric, r); err != nil {
			t.Fatal(err)
		}
	}

	// Round-trip: the could-not-measure state survives serialization.
	back, err := store.ReadMetrics()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range back {
		if m.Metric == MetricDuplicateBlockRate {
			found = true
			if m.Value.State != StateCouldNotMeasure {
				t.Fatalf("could-not-measure round-tripped as %q, not could-not-measure", m.Value.State)
			}
			if m.Value.Reason == "" {
				t.Fatalf("could-not-measure lost its reason on round-trip")
			}
		}
	}
	if !found {
		t.Fatal("could-not-measure record missing after round-trip")
	}

	// Render: the could-not-measure metric shows as unmeasured, never `0`.
	view, err := renderReport(store, BuiltinBaselines())
	if err != nil {
		t.Fatal(err)
	}
	// The duplicate-block row must carry a "not measured" local cell, not "| 0 |".
	dupRow := findRow(view, "Duplicate-block rate")
	if dupRow == "" {
		t.Fatalf("no duplicate-block row in view:\n%s", view)
	}
	if !strings.Contains(dupRow, "not measured") {
		t.Fatalf("could-not-measure metric should render 'not measured', got row: %q", dupRow)
	}
	if strings.Contains(dupRow, "| 0 |") {
		t.Fatalf("could-not-measure metric must NEVER render as 0; got row: %q", dupRow)
	}
}

func findRow(view, label string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, label) {
			return line
		}
	}
	return ""
}

// TestArtifacts_SchemaDeclaresDefectsAndAttribution is Verify #8: the
// defects.jsonl record (evidence tier + confidence) and the attribution/
// per-defect file shape are declared and round-trip, ready for quality/06–07
// and /10 to fill.
func TestArtifacts_SchemaDeclaresDefectsAndAttribution(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	// defects.jsonl: a DefectFix carrying evidence TIER and a three-state
	// CONFIDENCE round-trips through the append-only table.
	conf := Measured(0.8)
	fix := DefectFix{
		FixCommitSHA: "abc123",
		FixPRNumber:  42,
		ClosedIssue:  &IssueRef{Number: 7},
		Tier:         Tier1,
		Identified:   Measured(true),
		Confidence:   &conf,
	}
	if err := store.Append(KindDefect, fix); err != nil {
		t.Fatal(err)
	}
	backDefects, err := store.ReadDefects()
	if err != nil {
		t.Fatal(err)
	}
	if len(backDefects) != 1 {
		t.Fatalf("expected 1 defect record, got %d", len(backDefects))
	}
	got := backDefects[0]
	if got.Tier != Tier1 {
		t.Fatalf("evidence tier lost on round-trip: %q", got.Tier)
	}
	if got.Confidence == nil || got.Confidence.State != StateMeasured || got.Confidence.Value != 0.8 {
		t.Fatalf("confidence field lost/wrong on round-trip: %+v", got.Confidence)
	}

	// A record with NO confidence (quality/06's tier-only emit) round-trips with
	// the field omitted — the schema is additive, not a breaking change.
	noConf := DefectFix{FixCommitSHA: "def456", Tier: Tier2, Identified: Measured(true)}
	if err := store.Append(KindDefect, noConf); err != nil {
		t.Fatal(err)
	}
	backDefects, err = store.ReadDefects()
	if err != nil {
		t.Fatal(err)
	}
	if backDefects[1].Confidence != nil {
		t.Fatalf("tier-only defect should omit confidence, got %+v", backDefects[1].Confidence)
	}

	// attribution/: a per-defect dossier with a three-state stage + confidence
	// round-trips; a tombstone amendment appends without a silent overwrite.
	dossier := AttributionRecord{
		DefectID:          "DEF-001",
		FixCommitSHA:      "abc123",
		InducingCommitSHA: "old999",
		Stage:             Measured("review"),
		Confidence:        Measured(0.6),
	}
	if err := store.WriteAttribution(dossier); err != nil {
		t.Fatal(err)
	}
	// A second write to the SAME defect is REFUSED (never a silent overwrite).
	if err := store.WriteAttribution(dossier); err == nil {
		t.Fatal("second WriteAttribution to the same defect must be refused (tombstone-only)")
	}
	back, err := store.ReadAttribution("DEF-001")
	if err != nil || back == nil {
		t.Fatalf("read attribution: %v (nil=%v)", err, back == nil)
	}
	if back.Stage.State != StateMeasured || back.Stage.Value != "review" {
		t.Fatalf("attribution stage lost on round-trip: %+v", back.Stage)
	}
	if back.Confidence.State != StateMeasured || back.Confidence.Value != 0.6 {
		t.Fatalf("attribution confidence lost on round-trip: %+v", back.Confidence)
	}
	if back.SchemaVersion != attributionSchemaVersion {
		t.Fatalf("attribution schema version not stamped: %q", back.SchemaVersion)
	}

	// Tombstone amendment: append a correction; the trail records old→new.
	err = store.AmendAttribution("DEF-001",
		Amendment{Reason: "re-traced", Field: "stage", OldValue: "review", NewValue: "author"},
		func(r *AttributionRecord) { r.Stage = Measured("author") })
	if err != nil {
		t.Fatal(err)
	}
	amended, err := store.ReadAttribution("DEF-001")
	if err != nil {
		t.Fatal(err)
	}
	if amended.Stage.Value != "author" {
		t.Fatalf("amendment did not update the base stage: %+v", amended.Stage)
	}
	if len(amended.Amendments) != 1 || amended.Amendments[0].OldValue != "review" {
		t.Fatalf("amendment trail not recorded: %+v", amended.Amendments)
	}

	ids, err := store.ListAttribution()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "DEF-001" {
		t.Fatalf("ListAttribution = %v, want [DEF-001]", ids)
	}

	// An unsafe defect id (path escape) is refused, never written outside the dir.
	if err := store.WriteAttribution(AttributionRecord{DefectID: "../escape"}); err == nil {
		t.Fatal("unsafe attribution id must be refused")
	}
	if _, err := os.Stat(store.attributionDir()); err != nil {
		t.Fatalf("attribution dir should exist: %v", err)
	}
}
