package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureRoot is the known-value artifact set under testdata/report/.
const fixtureRoot = "testdata/report"

func renderFixture(t *testing.T) string {
	t.Helper()
	baselines, err := LoadBaselines(filepath.Join(fixtureRoot, "baselines.json"))
	if err != nil {
		t.Fatalf("load baselines: %v", err)
	}
	view, err := renderReport(NewStore(fixtureRoot), baselines)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return view
}

// TestReport_RendersKnownMetric is Verify #3: the render of testdata/report/
// dereferences a SPECIFIC known value — the copy/paste ratio 0.42 from the
// latest fixture snapshot — not merely a presence check. The 0.99 record in the
// older snapshot proves the latest-snapshot selection: the superseded value must
// NOT be what renders.
func TestReport_RendersKnownMetric(t *testing.T) {
	view := renderFixture(t)
	if !strings.Contains(view, "| Copy/paste ratio | 0.42 |") {
		t.Fatalf("expected copy/paste ratio 0.42 rendered on its row; view:\n%s", view)
	}
	if strings.Contains(view, "0.99") {
		t.Fatalf("superseded prior-snapshot value 0.99 must not render — latest snapshot only:\n%s", view)
	}
}

// TestReport_IndustryBesideLocal is Verify #4: a metric with a published
// baseline renders BOTH the local number and the industry-comparable number on
// the same row, and the honest-claims header reads "per GitClear's published
// definitions" (never "GitClear-equivalent").
func TestReport_IndustryBesideLocal(t *testing.T) {
	view := renderFixture(t)
	if !strings.Contains(view, "| Copy/paste ratio | 0.42 | 0.1 |") {
		t.Fatalf("expected local 0.42 beside industry 0.1 on the copy/paste row; view:\n%s", view)
	}
	if !strings.Contains(view, "per GitClear's published definitions") {
		t.Fatalf("expected honest-claims header 'per GitClear's published definitions'; view:\n%s", view)
	}
	if strings.Contains(view, "GitClear-equivalent") {
		t.Fatalf("must never claim 'GitClear-equivalent'; view:\n%s", view)
	}
}

// TestReport_LocalRunDiscards is Verify #5: a local (non-CI) `report --write`
// run does NOT write the committed QUALITY.md — the file on disk is
// byte-identical before and after — and the CI-writer guard is the only path
// that writes.
func TestReport_LocalRunDiscards(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := os.MkdirAll(store.dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed a committed metrics table so the render has something to read, and a
	// committed QUALITY.md with sentinel content that a local run must not touch.
	if err := store.Append(KindMetric, MetricRecord{
		Metric: MetricChurnRate, Grain: GrainRepo,
		Value: Measured(0.03), Basis: basisPublishedDefinitions, Note: honestClaimsNote,
		MinedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	viewPath := filepath.Join(store.dir(), qualityView)
	sentinel := []byte("SENTINEL — committed view, CI-written only\n")
	if err := os.WriteFile(viewPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}

	// Local --write with the guard env UNSET: MUST refuse (exit 2) and leave the
	// committed view byte-for-byte unchanged.
	t.Setenv(ciWriterEnv, "")
	var out, errb bytes.Buffer
	if rc := runReport([]string{"--out", root, "--write"}, &out, &errb); rc != 2 {
		t.Fatalf("local --write should be REFUSED with exit 2, got %d (stderr=%s)", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "REFUSED") {
		t.Fatalf("expected a REFUSED message on stderr, got %q", errb.String())
	}
	after, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("local run mutated the committed view:\n before=%q\n after =%q", before, after)
	}

	// The CI-writer guard is the ONLY path that writes: with the env set, the
	// committed view is written (and is no longer the sentinel).
	t.Setenv(ciWriterEnv, ciWriterToken)
	out.Reset()
	errb.Reset()
	if rc := runReport([]string{"--out", root, "--write"}, &out, &errb); rc != 0 {
		t.Fatalf("CI-writer --write should succeed, got %d (stderr=%s)", rc, errb.String())
	}
	written, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(written, sentinel) {
		t.Fatalf("CI-writer path did not write the rendered view")
	}
	if !strings.Contains(string(written), "# QUALITY.md") {
		t.Fatalf("CI-written view is not the rendered report:\n%s", written)
	}
}

// TestReport_LocalDefaultRendersToStdout pins that the DEFAULT (no --write) run
// renders to stdout and writes nothing under the tracking root.
func TestReport_LocalDefaultRendersToStdout(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Append(KindMetric, MetricRecord{
		Metric: MetricChurnRate, Grain: GrainRepo,
		Value: Measured(0.03), Basis: basisPublishedDefinitions, Note: honestClaimsNote,
		MinedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := runReport([]string{"--out", root}, &out, &errb); rc != 0 {
		t.Fatalf("default report exit %d (stderr=%s)", rc, errb.String())
	}
	if !strings.Contains(out.String(), "# QUALITY.md") {
		t.Fatalf("default run should render to stdout, got %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(store.dir(), qualityView)); !os.IsNotExist(err) {
		t.Fatalf("default run must not write the committed view (stat err=%v)", err)
	}
}
