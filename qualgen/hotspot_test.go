package main

import (
	"testing"
	"time"
)

// --- fixture helpers: synthetic Commit/FileDiff records, no real git repo
// needed — the hotspot aggregator consumes only quality/01's mined tables. ---

func addHunk(contents ...string) []Hunk {
	lines := make([]LineChange, 0, len(contents))
	for _, c := range contents {
		lines = append(lines, LineChange{Op: OpAdd, Content: c})
	}
	return []Hunk{{Lines: lines}}
}

func delHunk(contents ...string) []Hunk {
	lines := make([]LineChange, 0, len(contents))
	for _, c := range contents {
		lines = append(lines, LineChange{Op: OpDel, Content: c})
	}
	return []Hunk{{Lines: lines}}
}

func testCommit(sha string, when time.Time) Commit {
	return Commit{SHA: sha, AuthorEmail: "author@example.test", AuthorWhen: when}
}

func measuredDiff(sha, path string, hunks []Hunk) FileDiff {
	return FileDiff{CommitSHA: sha, NewPath: path, Kind: ChangeModified, Lines: Measured(hunks)}
}

func binaryDiff(sha, path string) FileDiff {
	return FileDiff{CommitSHA: sha, NewPath: path, Kind: ChangeModified, Binary: true,
		Lines: CouldNotMeasure[[]Hunk]("binary or unreadable blob: not line-diffable")}
}

// TestHotspotIsProductNotFactor is Verify row 2: a file high in BOTH decayed
// change-frequency and complexity outranks a file high in only one — the
// PRODUCT drives the score, not either factor alone.
func TestHotspotIsProductNotFactor(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	flat := []string{"x", "x", "x"}                           // zero indentation
	nested := []string{"        x", "        x", "        x"} // deep indentation

	commits := []Commit{
		testCommit("c1", now.AddDate(0, 0, -1)),
		testCommit("c2", now.AddDate(0, 0, -1)),
		testCommit("c3", now.AddDate(0, 0, -1)),
	}
	diffs := []FileDiff{
		// hot.go: touched 3 times recently AND deeply nested.
		measuredDiff("c1", "hot.go", addHunk(nested...)),
		measuredDiff("c2", "hot.go", addHunk(nested...)),
		measuredDiff("c3", "hot.go", addHunk(nested...)),
		// freq_only.go: touched 3 times recently but flat (complexity low).
		measuredDiff("c1", "freq_only.go", addHunk(flat...)),
		measuredDiff("c2", "freq_only.go", addHunk(flat...)),
		measuredDiff("c3", "freq_only.go", addHunk(flat...)),
		// complexity_only.go: touched once, deeply nested.
		measuredDiff("c1", "complexity_only.go", addHunk(nested...)),
	}

	recs := ComputeHotspots(commits, diffs, nil, HotspotParams{Now: now})
	byPath := map[string]HotspotRecord{}
	for _, r := range recs {
		byPath[r.Path] = r
	}

	hot, ok := byPath["hot.go"]
	if !ok || hot.Hotspot.State != StateMeasured {
		t.Fatalf("expected a measured hotspot for hot.go, got %+v", hot)
	}
	freqOnly := byPath["freq_only.go"]
	complexityOnly := byPath["complexity_only.go"]

	if !(hot.Hotspot.Value > freqOnly.Hotspot.Value) {
		t.Errorf("file high in both factors must outrank the frequency-only file: hot=%v freq_only=%v",
			hot.Hotspot.Value, freqOnly.Hotspot.Value)
	}
	if !(hot.Hotspot.Value > complexityOnly.Hotspot.Value) {
		t.Errorf("file high in both factors must outrank the complexity-only file: hot=%v complexity_only=%v",
			hot.Hotspot.Value, complexityOnly.Hotspot.Value)
	}
	// The two single-factor files must not themselves be ranked as high as
	// the dual-factor file — i.e. the ranking reflects the product, not
	// either input alone.
	if freqOnly.ComplexityProxy.Value >= hot.ComplexityProxy.Value {
		t.Fatalf("test fixture invalid: freq_only must have lower complexity than hot")
	}
	if complexityOnly.ChangeFrequency.Value >= hot.ChangeFrequency.Value {
		t.Fatalf("test fixture invalid: complexity_only must have lower change-frequency than hot")
	}
}

// TestIndentationComplexityProxy is Verify row 3: deeper indentation nesting
// yields a strictly higher proxy on the same line count (language-agnostic).
func TestIndentationComplexityProxy(t *testing.T) {
	shallow := []string{"  a", "  b", "  c"}
	deep := []string{"        a", "        b", "        c"}

	shallowM := indentationComplexity(shallow)
	deepM := indentationComplexity(deep)

	if shallowM.State != StateMeasured || deepM.State != StateMeasured {
		t.Fatalf("expected both measured, got shallow=%v deep=%v", shallowM.State, deepM.State)
	}
	if !(deepM.Value > shallowM.Value) {
		t.Errorf("deeper nesting must yield a strictly higher proxy on the same line count: shallow=%v deep=%v",
			shallowM.Value, deepM.Value)
	}

	// A tab is treated as a deeper indent than a couple of spaces.
	tabbed := indentationComplexity([]string{"\tx"})
	spaced := indentationComplexity([]string{"  x"})
	if !(tabbed.Value > spaced.Value) {
		t.Errorf("a tab must count as deeper than two spaces: tab=%v spaced=%v", tabbed.Value, spaced.Value)
	}
}

// TestComplexityUnmeasurableIsThreeState is Verify row 6: a binary/unreadable
// file emits could-not-measure complexity (with a reason); an unchanged text
// file emits measured-zero change-frequency. The two are never conflated.
func TestComplexityUnmeasurableIsThreeState(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	commits := []Commit{testCommit("c1", now.AddDate(0, 0, -5))}
	diffs := []FileDiff{
		binaryDiff("c1", "logo.png"),
	}
	// unchanged.txt is present at the tip but never appears in any diff.
	allPaths := []string{"logo.png", "unchanged.txt"}

	recs := ComputeHotspots(commits, diffs, allPaths, HotspotParams{Now: now})
	byPath := map[string]HotspotRecord{}
	for _, r := range recs {
		byPath[r.Path] = r
	}

	bin, ok := byPath["logo.png"]
	if !ok {
		t.Fatalf("expected a hotspot row for logo.png")
	}
	if bin.ComplexityProxy.State != StateCouldNotMeasure {
		t.Errorf("binary file complexity must be could-not-measure, got %q", bin.ComplexityProxy.State)
	}
	if bin.ComplexityProxy.Reason == "" {
		t.Error("could-not-measure complexity must carry a reason")
	}
	if bin.ChangeFrequency.State != StateMeasured {
		t.Errorf("a file that WAS touched must have a measured change-frequency, got %q", bin.ChangeFrequency.State)
	}

	unchanged, ok := byPath["unchanged.txt"]
	if !ok {
		t.Fatalf("expected a hotspot row for unchanged.txt")
	}
	if unchanged.ChangeFrequency.State != StateMeasuredZero {
		t.Errorf("an untouched file's change-frequency must be measured-zero, got %q", unchanged.ChangeFrequency.State)
	}
	if unchanged.ChangeFrequency.Reason != "" {
		t.Errorf("measured-zero must carry no reason, got %q", unchanged.ChangeFrequency.Reason)
	}

	if bin.ComplexityProxy.State == unchanged.ChangeFrequency.State {
		t.Fatalf("could-not-measure complexity and measured-zero change-frequency must never share a state")
	}
	// The overall hotspot for the untouched file floors to measured-zero
	// (zero frequency), never could-not-measure, even though its complexity
	// could also not be assessed.
	if unchanged.Hotspot.State != StateMeasuredZero {
		t.Errorf("hotspot for a never-touched file must be measured-zero (frequency floors it), got %q", unchanged.Hotspot.State)
	}
}

// TestChangeFrequencyDecay pins the exponential-decay shape: a touch at
// exactly one half-life ago contributes half the weight of one today.
func TestChangeFrequencyDecay(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	halfLife := 30.0

	recent := changeFrequency([]time.Time{now}, now, halfLife)
	aged := changeFrequency([]time.Time{now.AddDate(0, 0, -30)}, now, halfLife)

	if recent.State != StateMeasured || aged.State != StateMeasured {
		t.Fatalf("expected both measured, got recent=%v aged=%v", recent.State, aged.State)
	}
	if diff := recent.Value - 2*aged.Value; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("a touch one half-life old must weigh half of one today: recent=%v aged=%v", recent.Value, aged.Value)
	}

	zero := changeFrequency(nil, now, halfLife)
	if zero.State != StateMeasuredZero {
		t.Errorf("no touches must be measured-zero, got %q", zero.State)
	}
}
