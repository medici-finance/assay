package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildRows builds roadmapStreamRow values for a set of streams the same way
// runRoadmap does, so page-generation tests exercise the real row set.
func buildRows(streams []*Stream, findings []Finding, now time.Time) []roadmapStreamRow {
	var rows []roadmapStreamRow
	for _, s := range streams {
		color, reason := computeRoadmapHealth(s, streams, findings, now)
		rows = append(rows, roadmapStreamRow{
			Stream:       s,
			HealthColor:  color,
			HealthReason: reason,
			StageCounts:  stageCounts(s),
			NextWave:     nextWave(s),
			TopBlocker:   blockerText(s),
		})
	}
	return rows
}

// TestStreamPageRowInvariant is the tested row<->page invariant: the overview's
// stream set is the page set — exactly one page per grid row, no row without a
// page and no page without a row.
func TestStreamPageRowInvariant(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	a := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "done"})
	a.Serves = "assay"
	b := mkStream("bravo", "active", "P2", Brief{Num: "01", Wave: 0, Status: "todo"})
	c := mkStream("charlie", "paused", "P2", Brief{Num: "01", Wave: 0, Status: "in-progress"})
	streams := []*Stream{a, b, c}
	rows := buildRows(streams, nil, now)

	pages := renderAllStreamPages(rows, streams, nil, nil, nil, nil, "sha1234", now, now)

	if len(pages) != len(rows) {
		t.Fatalf("page count %d != row count %d", len(pages), len(rows))
	}
	// Every row must have a page.
	for _, r := range rows {
		fn := streamPageFilename(r.Stream.Name)
		if _, ok := pages[fn]; !ok {
			t.Errorf("row %s has no page %s", r.Stream.Name, fn)
		}
	}
	// Every page must map back to a row.
	rowNames := map[string]bool{}
	for _, r := range rows {
		rowNames[streamPageFilename(r.Stream.Name)] = true
	}
	for fn := range pages {
		if !rowNames[fn] {
			t.Errorf("page %s has no grid row", fn)
		}
	}
}

// TestStreamPageDeterminism verifies a stream page renders byte-identically on
// re-run with the same inputs.
func TestStreamPageDeterminism(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	s := mkStream("alpha", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
		Brief{Num: "02", Wave: 1, Status: "in-progress", Schema: "brief-v1", Depends: []string{"alpha/01"}},
	)
	s.Serves = "assay"
	s.Owner = "alice"
	rows := buildRows([]*Stream{s}, nil, now)
	p1 := renderAllStreamPages(rows, []*Stream{s}, nil, nil, nil, nil, "sha1", now, now)
	p2 := renderAllStreamPages(rows, []*Stream{s}, nil, nil, nil, nil, "sha1", now, now)
	fn := streamPageFilename("alpha")
	if p1[fn] != p2[fn] {
		t.Fatal("stream page render is not deterministic")
	}
}

// TestStreamDeltaPanelFixture is the delta-panel fixture test: a historian log
// drives the rendered "since yesterday" panel.
func TestStreamDeltaPanelFixture(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	history := []HistoryEntry{
		// In window (last 24h): a transition, a merge to done, a new brief.
		{Ts: "2026-07-15T12:00:00Z", Brief: "alpha/01", From: "todo", To: "in-progress", SHA: "abcdef1234"},
		{Ts: "2026-07-15T13:00:00Z", Brief: "alpha/02", From: "implemented", To: "done", SHA: "9876543210"},
		{Ts: "2026-07-15T14:00:00Z", Brief: "alpha/03", From: "", To: "todo"},
		// Out of window.
		{Ts: "2026-07-10T14:00:00Z", Brief: "alpha/04", From: "todo", To: "done"},
		// Different stream — excluded.
		{Ts: "2026-07-15T15:00:00Z", Brief: "beta/01", From: "todo", To: "done"},
	}

	d := computeStreamDelta("alpha", history, nil, now)
	if len(d.Transitions) != 2 {
		t.Fatalf("want 2 transitions, got %d: %v", len(d.Transitions), d.Transitions)
	}
	if len(d.NewBriefs) != 1 || d.NewBriefs[0] != "03" {
		t.Fatalf("want new brief 03, got %v", d.NewBriefs)
	}
	// The done transition must carry the short merge SHA.
	joined := strings.Join(d.Transitions, " | ")
	if !strings.Contains(joined, "9876543") {
		t.Errorf("done transition should carry merge SHA: %v", d.Transitions)
	}
	if d.empty() {
		t.Error("delta should not be empty with in-window history")
	}

	// Empty stream renders the explicit empty state in the panel.
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "done"})
	quietNow := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) // history all out of window
	rows := buildRows([]*Stream{s}, nil, quietNow)
	pages := renderAllStreamPages(rows, []*Stream{s}, history, nil, nil, nil, "sha", quietNow, quietNow)
	page := pages[streamPageFilename("alpha")]
	if !strings.Contains(page, "no changes") {
		t.Error("quiet stream page should render the 'no changes' empty state")
	}
	if !strings.Contains(page, "since yesterday") {
		t.Error("every stream page must carry the lowercase 'since yesterday' marker (Verify #3)")
	}

	// A stream WITH changes still carries the lowercase marker AND the transition.
	rows2 := buildRows([]*Stream{s}, nil, now)
	pages2 := renderAllStreamPages(rows2, []*Stream{s}, history, nil, nil, nil, "sha", now, now)
	page2 := pages2[streamPageFilename("alpha")]
	if !strings.Contains(page2, "since yesterday") {
		t.Error("changed stream page must still carry 'since yesterday' marker")
	}
	if !strings.Contains(page2, "01: todo") {
		t.Error("changed stream page should render the transition from the historian")
	}
}

// TestStreamPageCollapsedDoneWave verifies that on a large synthetic stream a
// fully-done wave collapses to a single summary line while an incomplete wave
// renders its full row set.
func TestStreamPageCollapsedDoneWave(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	var briefs []Brief
	// Wave 0: 20 briefs, all done -> must collapse.
	for i := 1; i <= 20; i++ {
		briefs = append(briefs, Brief{Num: fmt.Sprintf("%02d", i), Wave: 0, Status: "done", Title: "done brief"})
	}
	// Wave 1: 5 briefs, mixed -> must render full rows.
	for i := 21; i <= 25; i++ {
		st := "todo"
		if i == 21 {
			st = "in-progress"
		}
		briefs = append(briefs, Brief{Num: fmt.Sprintf("%02d", i), Wave: 1, Status: st, Title: "open brief"})
	}
	s := mkStream("big", "active", "P1", briefs...)
	s.Serves = "assay"
	rows := buildRows([]*Stream{s}, nil, now)
	pages := renderAllStreamPages(rows, []*Stream{s}, nil, nil, nil, nil, "sha", now, now)
	page := pages[streamPageFilename("big")]

	if !strings.Contains(page, "wave-collapsed") {
		t.Error("fully-done wave 0 should collapse to one summary line")
	}
	if !strings.Contains(page, "Wave 0</span> 20 briefs — all complete") {
		t.Errorf("collapsed wave 0 summary missing or malformed")
	}
	// Wave 1 must render a real table with its incomplete rows, not collapse.
	if !strings.Contains(page, "Wave 1</div>") {
		t.Error("incomplete wave 1 should render a wave heading + table")
	}
	if strings.Contains(page, "Wave 1</span> 5 briefs — all complete") {
		t.Error("incomplete wave 1 must NOT collapse")
	}
	// A wave-1 brief id should appear as a row.
	if !strings.Contains(page, ">21<") && !strings.Contains(page, ">21</a>") {
		t.Error("wave 1 brief rows should be rendered")
	}
}

// TestStreamOutcomeParsing verifies the one-line outcome extraction from an H1
// em-dash tagline and the markdown-strip fallback.
func TestStreamOutcomeParsing(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"# Foo Stream — instrument the outcomes", "instrument the outcomes"},
		{"# Foo Stream — measure it. And more.", "measure it."},
		{"# Foo Stream — a [linked](url) phrase and `code`", "a linked phrase and code"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(c.in+"\n\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := streamOutcome(dir); got != c.want {
			t.Errorf("streamOutcome(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStreamBlockersComputed verifies the typed-graph blocker rows: an
// unsatisfied dependency and an unresolved finding both surface as
// issue->effect->action rows.
func TestStreamBlockersComputed(t *testing.T) {
	s := mkStream("alpha", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"beta/01"}},
	)
	allStatus := map[string]string{
		"alpha/02": "todo",
		"beta/01":  "in-progress", // not done -> blocks
	}
	findings := []Finding{
		{ID: "F-x", Title: "bad thing", Affects: []string{"alpha/brief-02"}, Resolved: false},
		{ID: "F-y", Title: "resolved thing", Affects: []string{"alpha"}, Resolved: true}, // excluded
	}
	rows := streamBlockers(s, allStatus, findings, map[string]string{"F-x": "2026-07-17-x.md"})
	if len(rows) != 2 {
		t.Fatalf("want 2 blocker rows (dep + finding), got %d: %+v", len(rows), rows)
	}
	var sawDep, sawFinding bool
	for _, r := range rows {
		if strings.Contains(r.Issue, "beta/01") && r.Action != "" && r.Effect != "" {
			sawDep = true
		}
		if strings.Contains(r.Issue, "F-x") && r.Finding == "2026-07-17-x.md" {
			sawFinding = true
		}
	}
	if !sawDep {
		t.Error("missing dependency blocker row with issue/effect/action")
	}
	if !sawFinding {
		t.Error("missing finding blocker row with entry-file link")
	}
}

// TestNextWaveGate verifies the depends-graph gate sentence.
func TestNextWaveGate(t *testing.T) {
	// Wave 1 brief depends on beta/01 which is not done -> gated.
	s := mkStream("alpha", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"beta/01"}},
	)
	allStatus := map[string]string{"alpha/01": "done", "alpha/02": "todo", "beta/01": "in-progress"}
	got := nextWaveGate(s, allStatus)
	if !strings.Contains(got, "Wave 1 unlocks when") || !strings.Contains(got, "beta/01") {
		t.Errorf("gated wave sentence wrong: %q", got)
	}

	// All done.
	s2 := mkStream("done", "active", "P1", Brief{Num: "01", Wave: 0, Status: "done"})
	if got := nextWaveGate(s2, map[string]string{"done/01": "done"}); got != "All waves complete." {
		t.Errorf("all-done gate: %q", got)
	}
}
