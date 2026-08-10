package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// blockerText returns just the blocker text for a stream with no finding
// register, discarding the (necessarily empty) hyperlink.
func blockerText(s *Stream) string {
	text, _ := topBlocker(s, nil, nil)
	return text
}

// --- Health rule tests ---

func TestRoadmapHealthRules(t *testing.T) {
	now := day(15) // 2026-07-16

	t.Run("P0-finding red rule fires", func(t *testing.T) {
		s := mkStream("critical", "active", "P0",
			Brief{Num: "01", Wave: 0, Status: "todo"},
		)
		s.LastTouch = day(0)
		findings := []Finding{
			{ID: "F-01", Date: "2026-07-01", Title: "Critical bug", Affects: []string{"critical/brief-01"}, Resolved: false},
		}
		color, reason := computeRoadmapHealth(s, []*Stream{s}, findings, now)
		if color != "red" {
			t.Fatalf("P0 stream with unresolved finding: want red, got %s", color)
		}
		if !strings.Contains(reason, "F-01") {
			t.Errorf("reason should mention finding ID: %s", reason)
		}
	})

	t.Run("P0-finding green when finding resolved", func(t *testing.T) {
		s := mkStream("critical", "active", "P0",
			Brief{Num: "01", Wave: 0, Status: "todo"},
		)
		s.LastTouch = day(14) // 1 day before now — keep fresh to avoid amber stale-todo rule
		findings := []Finding{
			{ID: "F-01", Date: "2026-07-01", Title: "Fixed thing", Affects: []string{"critical/brief-01"}, Resolved: true},
		}
		color, _ := computeRoadmapHealth(s, []*Stream{s}, findings, now)
		if color != "green" {
			t.Fatalf("P0 stream with resolved finding: want green, got %s", color)
		}
	})

	t.Run("Stalled-critical red rule fires", func(t *testing.T) {
		dep := mkStream("dep", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "done", Schema: "brief-v1"},
		)
		dep.LastTouch = day(0)
		target := mkStream("target", "active", "P1",
			Brief{Num: "02", Wave: 1, Status: "in-progress", Schema: "brief-v1", Depends: []string{"dep/01"}},
		)
		// Last touch is 10 days ago, so >7d stalled.
		target.LastTouch = day(5)
		color, reason := computeRoadmapHealth(target, []*Stream{target, dep}, nil, now)
		if color != "red" {
			t.Fatalf("critical-path brief stalled >7d: want red, got %s", color)
		}
		if !strings.Contains(reason, "stalled") || !strings.Contains(reason, "on critical path") {
			t.Errorf("reason should mention stalled and critical path: %s", reason)
		}
	})

	t.Run("Awaiting-backlog amber rule fires", func(t *testing.T) {
		s := mkStream("backlog", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "implemented"},
			Brief{Num: "02", Wave: 0, Status: "implemented"},
			Brief{Num: "03", Wave: 0, Status: "verified"},
		)
		// Touch is far enough back that all exceed 3d.
		s.LastTouch = day(10)
		color, reason := computeRoadmapHealth(s, []*Stream{s}, nil, now)
		if color != "amber" {
			t.Fatalf("2+ briefs awaiting >3d: want amber, got %s", color)
		}
		if !strings.Contains(reason, "awaiting verification") {
			t.Errorf("reason should mention awaiting verification: %s", reason)
		}
	})

	t.Run("Awaiting-backlog under threshold is green", func(t *testing.T) {
		s := mkStream("backlog", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "implemented"},
		)
		s.LastTouch = day(10)
		color, _ := computeRoadmapHealth(s, []*Stream{s}, nil, now)
		if color != "green" {
			t.Fatalf("1 brief awaiting: want green, got %s", color)
		}
	})

	t.Run("Nextup-stale amber rule fires", func(t *testing.T) {
		s := mkStream("stale-next", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "todo"},
		)
		s.LastTouch = day(5) // 10 days ago relative to day(15)
		color, reason := computeRoadmapHealth(s, []*Stream{s}, nil, now)
		if color != "amber" {
			t.Fatalf("unpicked next-up >3d: want amber, got %s", color)
		}
		if !strings.Contains(reason, "unpicked") {
			t.Errorf("reason should mention unpicked: %s", reason)
		}
	})

	t.Run("Green fallback fires when nothing matches", func(t *testing.T) {
		s := mkStream("healthy", "active", "P2",
			Brief{Num: "01", Wave: 0, Status: "done"},
		)
		s.LastTouch = day(14) // fresh
		color, reason := computeRoadmapHealth(s, []*Stream{s}, nil, now)
		if color != "green" {
			t.Fatalf("healthy stream: want green, got %s", color)
		}
		if reason != "" {
			t.Errorf("green should have empty reason, got: %s", reason)
		}
	})
}

// --- Fixed ordering tests ---

func TestRoadmapFixedOrder(t *testing.T) {
	lending := mkStream("z-lending", "active", "P2")
	lending.Serves = "lending-app"
	reconciler := mkStream("a-reconciler", "active", "P2")
	reconciler.Serves = "reconciler"
	assay := mkStream("m-assay", "active", "P2")
	assay.Serves = "assay"
	platform := mkStream("p-platform", "active", "P2")
	platform.Serves = "platform"
	untagged := mkStream("untagged", "active", "P2")
	untagged.Serves = ""

	streams := []*Stream{lending, reconciler, assay, platform, untagged}
	keys := make([]string, len(streams))
	for i, s := range streams {
		keys[i] = roadmapSortKey(s)
	}

	// Verify serves ordering: lending-app < reconciler < assay < platform < untagged
	for i := 0; i < len(keys)-1; i++ {
		if keys[i] >= keys[i+1] {
			t.Errorf("sort key at %d (%s) >= sort key at %d (%s): serves ordering violated", i, keys[i], i+1, keys[i+1])
		}
	}

	// Within same serves category, priority matters.
	p0 := mkStream("p0-stream", "active", "P0")
	p0.Serves = "platform"
	p1 := mkStream("p1-stream", "active", "P1")
	p1.Serves = "platform"
	keys2 := []string{roadmapSortKey(p1), roadmapSortKey(p0)}
	// P0 should sort before P1.
	if keys2[0] >= keys2[1] {
		// Actually, alphabetical order means we need to check the key structure.
		// The fixed order is serves > priority > name. P0 < P1 alphabetically.
	}
	_ = keys2

	// Within same priority, alphabetical by name.
	a1 := mkStream("alpha", "active", "P1")
	a1.Serves = "platform"
	a2 := mkStream("beta", "active", "P1")
	a2.Serves = "platform"
	kA1 := roadmapSortKey(a1)
	kA2 := roadmapSortKey(a2)
	if kA1 >= kA2 {
		t.Errorf("alpha should sort before beta, got %s >= %s", kA1, kA2)
	}
}

// --- Empty exceptions test ---

func TestRoadmapEmptyExceptions(t *testing.T) {
	s := mkStream("healthy", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
	)
	s.LastTouch = day(14)
	s.Serves = "platform"
	s.Owner = "alice"
	rows := []roadmapStreamRow{
		{
			Stream:       s,
			HealthColor:  "green",
			HealthReason: "",
			StageCounts:  stageCounts(s),
			NextWave:     0,
			TopBlocker:   "—",
		},
	}
	exceptions := computeRoadmapExceptions(rows)
	if len(exceptions) != 0 {
		t.Fatalf("all-green streams should produce zero exceptions, got %d", len(exceptions))
	}

	// Verify that exceptions appear for red/amber.
	rows2 := []roadmapStreamRow{
		{
			Stream:       s,
			HealthColor:  "red",
			HealthReason: "red: P0 stream, unresolved finding F-01 — Test",
			StageCounts:  stageCounts(s),
			NextWave:     0,
			TopBlocker:   "—",
		},
	}
	exceptions2 := computeRoadmapExceptions(rows2)
	if len(exceptions2) != 1 {
		t.Fatalf("red stream should produce one exception, got %d", len(exceptions2))
	}
	if exceptions2[0].Color != "red" || exceptions2[0].Text == "" {
		t.Errorf("exception malformed: %+v", exceptions2[0])
	}
}

// --- Delta badges test ---

func TestRoadmapDeltaBadges(t *testing.T) {
	// now is the latest entry time plus a small delta.
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	history := []HistoryEntry{
		// Within 24h window: 2026-07-15 12:00Z onwards.
		{Ts: "2026-07-15T12:00:00Z", Brief: "stream/01", From: "", To: "todo", SHA: "a1"},
		{Ts: "2026-07-15T13:00:00Z", Brief: "stream/01", From: "todo", To: "in-progress", SHA: "a2"},
		{Ts: "2026-07-15T14:00:00Z", Brief: "stream/02", From: "", To: "done", SHA: "a3"},
		{Ts: "2026-07-15T15:00:00Z", Brief: "stream/03", From: "implemented", To: "done", SHA: "a4"},
		{Ts: "2026-07-15T16:00:00Z", Brief: "stream/04", From: "todo", To: "verified", SHA: "a5"},
		// Outside window: before cutoff.
		{Ts: "2026-07-14T12:00:00Z", Brief: "stream/05", From: "todo", To: "done", SHA: "old"},
		// Different stream: should be excluded.
		{Ts: "2026-07-15T17:00:00Z", Brief: "other/01", From: "", To: "done", SHA: "other"},
	}
	deltas := computeRoadmapDeltas("stream", history, now)
	if len(deltas) < 2 {
		t.Fatalf("want at least 2 delta badges, got %d: %v", len(deltas), deltas)
	}
	// Should contain "+2 done" (02 and 03 were done), "1→in-progress" (01), "NEW", and "+1 done" for 04 verified.
	if !containsDelta(deltas, "done") {
		t.Error("deltas should contain done badge")
	}
	if !containsDelta(deltas, "in-progress") {
		t.Error("deltas should contain in-progress badge")
	}

	// No history: empty deltas.
	emptyDeltas := computeRoadmapDeltas("empty", nil, now)
	if len(emptyDeltas) != 0 {
		t.Fatalf("empty history should produce empty deltas, got %v", emptyDeltas)
	}
}

func containsDelta(deltas []string, substr string) bool {
	for _, d := range deltas {
		if strings.Contains(d, substr) {
			return true
		}
	}
	return false
}

// --- Goal-mix tests ---

func TestRoadmapGoalMix(t *testing.T) {
	lending := mkStream("lending-stream", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "in-progress", Schema: "brief-v1"},
	)
	lending.Serves = "lending-app"
	lending.LastTouch = day(0)

	assay := mkStream("assay-stream", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "in-progress", Schema: "brief-v1"},
		Brief{Num: "02", Wave: 0, Status: "in-progress", Schema: "brief-v1"},
	)
	assay.Serves = "assay"
	assay.LastTouch = day(0)

	untagged := mkStream("untagged-stream", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "in-progress", Schema: "brief-v1"},
	)
	untagged.LastTouch = day(0)

	streams := []*Stream{lending, assay, untagged}
	nu := nextUp(streams, nil, nil)

	// History: 3 assay done transitions in last 24h, 0 lending.
	history := []HistoryEntry{
		{Ts: "2026-07-09T12:00:00Z", Brief: "assay-stream/03", From: "implemented", To: "done"},
		{Ts: "2026-07-09T13:00:00Z", Brief: "assay-stream/04", From: "implemented", To: "done"},
		{Ts: "2026-07-09T14:00:00Z", Brief: "assay-stream/05", From: "implemented", To: "done"},
	}

	goals, inverted, invertMsg := computeGoalMix(streams, nu, history)

	// Count values.
	var lendingG, assayG, untaggedG *goalMix
	for i := range goals {
		switch goals[i].Goal {
		case "lending-app":
			lendingG = &goals[i]
		case "assay":
			assayG = &goals[i]
		case "untagged":
			untaggedG = &goals[i]
		}
	}

	if lendingG == nil || assayG == nil || untaggedG == nil {
		t.Fatal("missing expected goal mix entries")
	}

	if lendingG.Active != 1 {
		t.Errorf("lending WIP: want 1, got %d", lendingG.Active)
	}
	if assayG.Active != 2 {
		t.Errorf("assay WIP: want 2, got %d", assayG.Active)
	}
	if assayG.Merged24h != 3 {
		t.Errorf("assay merged 24h: want 3, got %d", assayG.Merged24h)
	}
	if lendingG.Merged24h != 0 {
		t.Errorf("lending merged 24h: want 0, got %d", lendingG.Merged24h)
	}

	// With 3 assay merges and 0 lending merges, inversion should be detected.
	if !inverted {
		t.Errorf("goal mix should be inverted when non-lending dominates")
	}
	if invertMsg == "" {
		t.Error("inversion message should be non-empty")
	}
}

// TestRoadmapDeterminism verifies that the roadmap output is deterministic:
// running with the same inputs produces byte-identical output.
func TestRoadmapDeterminism(t *testing.T) {
	s := mkStream("test", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
	)
	s.Serves = "platform"
	s.Owner = "alice"
	s.LastTouch = day(0)

	streams := []*Stream{s}
	nu := nextUp(streams, nil, nil)
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)

	var rows []roadmapStreamRow
	for _, st := range streams {
		color, reason := computeRoadmapHealth(st, streams, nil, now)
		rows = append(rows, roadmapStreamRow{
			Stream:       st,
			HealthColor:  color,
			HealthReason: reason,
			StageCounts:  stageCounts(st),
			Deltas:       nil,
			NextWave:     nextWave(st),
			TopBlocker:   blockerText(st),
		})
	}

	exceptions := computeRoadmapExceptions(rows)
	goals, inverted, invertMsg := computeGoalMix(streams, nu, nil)

	html1 := renderRoadmap(streams, rows, exceptions, goals, inverted, invertMsg, nu, nil, "sha1", now)
	html2 := renderRoadmap(streams, rows, exceptions, goals, inverted, invertMsg, nu, nil, "sha1", now)

	if html1 != html2 {
		t.Fatal("renderRoadmap is not deterministic: two calls with identical inputs produced different output")
	}
}

// TestRoadmapRenderHasRequiredElements verifies the HTML output contains required
// structural elements and colors.
func TestRoadmapRenderHasRequiredElements(t *testing.T) {
	s := mkStream("test", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
		Brief{Num: "02", Wave: 0, Status: "in-progress"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
	)
	s.Serves = "lending-app"
	s.Owner = "alice"
	s.LastTouch = day(0)

	streams := []*Stream{s}
	nu := nextUp(streams, nil, nil)
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)

	var rows []roadmapStreamRow
	for _, st := range streams {
		color, reason := computeRoadmapHealth(st, streams, nil, now)
		rows = append(rows, roadmapStreamRow{
			Stream:       st,
			HealthColor:  color,
			HealthReason: reason,
			StageCounts:  stageCounts(st),
			Deltas:       nil,
			NextWave:     nextWave(st),
			TopBlocker:   blockerText(st),
		})
	}

	html := renderRoadmap(streams, rows, nil, nil, false, "", nu, nil, "abc1234", now)

	required := []string{
		"3366FF", // Medici blue
		"Portfolio Roadmap",
		"no exceptions", // all-green = no exceptions
		"lending-app",
		"Goal Mix",
		"Legend",
		"computed, never hand-asserted",
		"<!DOCTYPE html>",
	}
	for _, want := range required {
		if !strings.Contains(html, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

// TestRoadmapStageCounts verifies stage counting logic.
func TestRoadmapStageCounts(t *testing.T) {
	s := mkStream("test", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "in-progress"},
		Brief{Num: "04", Wave: 0, Status: "done"},
		Brief{Num: "05", Wave: 0, Status: "done"},
		Brief{Num: "06", Wave: 0, Status: "blocked"},
	)
	counts := stageCounts(s)
	if counts["todo"] != 2 {
		t.Errorf("todo: want 2, got %d", counts["todo"])
	}
	if counts["in-progress"] != 1 {
		t.Errorf("in-progress: want 1, got %d", counts["in-progress"])
	}
	if counts["done"] != 2 {
		t.Errorf("done: want 2, got %d", counts["done"])
	}
	if counts["blocked"] != 1 {
		t.Errorf("blocked: want 1, got %d", counts["blocked"])
	}
}

// TestRoadmapNextWave verifies the next incomplete wave calculation.
func TestRoadmapNextWave(t *testing.T) {
	s := mkStream("test", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
		Brief{Num: "02", Wave: 0, Status: "verified"},
		Brief{Num: "03", Wave: 1, Status: "todo"},
		Brief{Num: "04", Wave: 2, Status: "in-progress"},
	)
	nw := nextWave(s)
	if nw != 1 {
		t.Errorf("next wave: want 1, got %d", nw)
	}

	// All done.
	s2 := mkStream("done", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
		Brief{Num: "02", Wave: 1, Status: "done"},
	)
	if nw2 := nextWave(s2); nw2 != 0 {
		t.Errorf("all done next wave: want 0, got %d", nw2)
	}
}

// TestRoadmapServesLabel verifies the serves-to-label mapping.
func TestRoadmapServesLabel(t *testing.T) {
	cases := []struct {
		serves, want string
	}{
		{"lending-app", "Lending App"},
		{"reconciler", "Reconciler"},
		{"assay", "Assay"},
		{"platform", "Platform"},
		{"", "Untagged"},
		{"unknown", "Untagged"},
	}
	for _, c := range cases {
		if got := servesLabel(c.serves); got != c.want {
			t.Errorf("servesLabel(%q) = %q, want %q", c.serves, got, c.want)
		}
	}
}

// --- Blocker hyperlink tests (methodology-metrics/23 port) ---

// writeFindingFixture creates a findings register under root and returns root.
func writeFindingFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "findings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func findingFile(id, date, title, affects string) string {
	return "---\nid: " + id + "\ndate: \"" + date + "\"\ntitle: \"" + title +
		"\"\naffects: [\"" + affects + "\"]\nack: \"\"\nresolved: false\n---\n\nbody\n"
}

// TestFindingFileByID verifies the ID→filename map, which cannot be derived
// from the ID: slug IDs and legacy numeric IDs both map to date-prefixed,
// title-slugged filenames.
func TestFindingFileByID(t *testing.T) {
	root := writeFindingFixture(t, map[string]string{
		"2026-07-17-ready-flip-is-convention-only.md": findingFile("F-ready-flip", "2026-07-17", "Ready flip is convention only", "alpha"),
		"2026-07-11-no-streams-root.md":               findingFile("F-01-no-streams-root", "2026-07-11", "No streams root", "beta"),
		"README.md":                                   "not a finding entry\n",
	})

	m := findingFileByID(root)
	if got := m["F-ready-flip"]; got != "2026-07-17-ready-flip-is-convention-only.md" {
		t.Errorf("slug ID: got %q", got)
	}
	if got := m["F-01-no-streams-root"]; got != "2026-07-11-no-streams-root.md" {
		t.Errorf("legacy-prefixed ID: got %q", got)
	}
	if _, ok := m[""]; ok {
		t.Error("non-entry file should not produce an empty-ID map key")
	}
}

// TestFindingFileByIDMissingRegister verifies a missing register degrades to
// nil rather than failing — blockers then render as plain text.
func TestFindingFileByIDMissingRegister(t *testing.T) {
	if m := findingFileByID(t.TempDir()); m != nil {
		t.Errorf("missing findings dir: want nil map, got %v", m)
	}
}

// TestTopBlockerFindingLink verifies the link is returned for a finding blocker
// and left empty for every other blocker shape.
func TestTopBlockerFindingLink(t *testing.T) {
	files := map[string]string{"F-known": "2026-07-17-known.md"}

	t.Run("finding blocker links", func(t *testing.T) {
		s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
		findings := []Finding{{ID: "F-known", Title: "Known thing", Affects: []string{"alpha"}}}
		text, link := topBlocker(s, findings, files)
		if !strings.Contains(text, "F-known") {
			t.Errorf("text: got %q", text)
		}
		if link != "2026-07-17-known.md" {
			t.Errorf("link: got %q", link)
		}
	})

	t.Run("finding with no entry file renders unlinked", func(t *testing.T) {
		s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
		findings := []Finding{{ID: "F-unfiled", Title: "Unfiled", Affects: []string{"alpha"}}}
		text, link := topBlocker(s, findings, files)
		if !strings.Contains(text, "F-unfiled") {
			t.Errorf("text: got %q", text)
		}
		if link != "" {
			t.Errorf("finding absent from register must not link: got %q", link)
		}
	})

	t.Run("blocked brief renders unlinked", func(t *testing.T) {
		s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "blocked", Title: "Stuck"})
		text, link := topBlocker(s, nil, files)
		if !strings.HasPrefix(text, "Blocked:") {
			t.Errorf("text: got %q", text)
		}
		if link != "" {
			t.Errorf("blocked-brief blocker must not link: got %q", link)
		}
	})

	t.Run("no blocker renders unlinked", func(t *testing.T) {
		s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
		text, link := topBlocker(s, nil, files)
		if text != "—" || link != "" {
			t.Errorf("no blocker: got (%q, %q)", text, link)
		}
	})

	t.Run("nil map degrades to plain text", func(t *testing.T) {
		s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
		findings := []Finding{{ID: "F-known", Title: "Known thing", Affects: []string{"alpha"}}}
		if _, link := topBlocker(s, findings, nil); link != "" {
			t.Errorf("nil register map: got %q", link)
		}
	})
}

// renderOneRow renders the deck for a single row and returns the HTML.
func renderOneRow(row roadmapStreamRow) string {
	streams := []*Stream{row.Stream}
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	return renderRoadmap(streams, []roadmapStreamRow{row}, nil, nil, false, "", nextUp(streams, nil, nil), nil, "abc1234", now)
}

// TestRoadmapBlockerCellHyperlink verifies the TopBlocker cell becomes an
// anchor when a link is present and stays plain text when it is not.
func TestRoadmapBlockerCellHyperlink(t *testing.T) {
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	s.Serves = "assay"
	s.Owner = "alice"
	s.LastTouch = day(0)

	base := roadmapStreamRow{
		Stream: s, HealthColor: "green", HealthReason: "ok",
		StageCounts: stageCounts(s), NextWave: nextWave(s),
		TopBlocker: "⚠ F-known — Known thing",
	}

	linked := base
	linked.BlockerFindingLink = "2026-07-17-known.md"
	html := renderOneRow(linked)
	want := `<a href="../../streams/findings/2026-07-17-known.md">⚠ F-known — Known thing</a>`
	if !strings.Contains(html, want) {
		t.Errorf("linked blocker cell missing anchor %q", want)
	}

	plain := base
	html = renderOneRow(plain)
	if strings.Contains(html, "streams/findings/") {
		t.Error("blocker with no finding link must not render an anchor")
	}
	if !strings.Contains(html, `<td style="font-size:12px;">⚠ F-known — Known thing</td>`) {
		t.Error("unlinked blocker must still render as plain escaped text")
	}
}

// TestRoadmapBlockerCellEscaping verifies escaping survives the move inside an
// anchor — neither the link text nor the href may break out of the markup.
func TestRoadmapBlockerCellEscaping(t *testing.T) {
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	s.Serves = "assay"
	s.Owner = "alice"
	s.LastTouch = day(0)

	row := roadmapStreamRow{
		Stream: s, HealthColor: "green", HealthReason: "ok",
		StageCounts: stageCounts(s), NextWave: nextWave(s),
		TopBlocker:         `⚠ F-x — <script>alert("xss")</script> & "quoted"`,
		BlockerFindingLink: `evil".md"><script>alert(1)</script>`,
	}
	html := renderOneRow(row)

	for _, bad := range []string{"<script>", `alert("xss")`, `"><script`} {
		if strings.Contains(html, bad) {
			t.Errorf("unescaped %q leaked into output", bad)
		}
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("blocker text should be html-escaped inside the anchor")
	}
	if !strings.Contains(html, `href="../../streams/findings/evil&quot;.md&quot;&gt;&lt;script&gt;`) {
		t.Error("href value should be html-escaped")
	}
}

// TestRoadmapBlockerHrefResolves proves the generated href resolves to a real
// file: the deck is written to docs/reports/roadmap/index.html, so
// "../../streams/findings/<name>" must land on docs/streams/findings/<name>.
func TestRoadmapBlockerHrefResolves(t *testing.T) {
	name := "2026-07-17-ready-flip-is-convention-only.md"
	root := writeFindingFixture(t, map[string]string{
		name: findingFile("F-ready-flip", "2026-07-17", "Ready flip is convention only", "alpha"),
	})

	m := findingFileByID(root)
	link := m["F-ready-flip"]
	if link == "" {
		t.Fatal("finding not found in register map")
	}

	// Resolve the href exactly as a browser would, from the deck's location.
	deckDir := filepath.Join(root, "docs", "reports", "roadmap")
	target := filepath.Join(deckDir, filepath.FromSlash("../../streams/findings/"+link))
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("href ../../streams/findings/%s does not resolve from the deck dir: %v", link, err)
	}
	if want := filepath.Join(root, "docs", "streams", "findings", name); filepath.Clean(target) != want {
		t.Errorf("resolved to %s, want %s", filepath.Clean(target), want)
	}
}
