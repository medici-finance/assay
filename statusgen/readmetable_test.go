package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// briefFrontmatter returns a minimal, clean brief-v1 file body that parseBriefFile
// accepts, with the given id/title/wave/effort.
func briefFrontmatter(id, title string, wave int, effort string) string {
	return "---\n" +
		"brief: " + id + "\n" +
		"title: " + title + "\n" +
		"wave: " + strconv.Itoa(wave) + "\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: " + effort + "\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-01-01 by test\n" +
		"sources: [\"teststream\"]\n" +
		"---\n\n# " + title + "\n"
}

const readmeTableFixtureBefore = "---\n" +
	"stream: teststream\n" +
	"status: active\n" +
	"priority: P1\n" +
	"track: platform\n" +
	"board: generated\n" +
	"---\n\n" +
	"# teststream\n\n" +
	"Prose ABOVE the table — must survive untouched.\n\n" +
	"## Briefs\n\n" +
	briefsMarkerBegin + "\n" +
	"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
	"|---|-------|------|--------|--------|----------|----------|\n" +
	"| 01 | [stale hand title](brief-01-first.md) | 0 | M | done | 2026-01-02 verifier | 2026-01-03 reviewer |\n" +
	"| 02 | [another stale title](brief-02-second.md) | 9 | L | todo | — | — |\n" +
	briefsMarkerEnd + "\n\n" +
	"Prose BELOW the table — must survive untouched.\n"

// writeReadmeTableFixture lays a teststream down under a temp root and returns the
// loaded Stream plus the README path. The brief frontmatter deliberately DISAGREES
// with the hand-written table (title/wave/effort), so a fresh render is a visible
// rewrite of the authoring columns while the lifecycle columns are preserved.
func writeReadmeTableFixture(t *testing.T, readme string) (*Stream, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-first.md"),
		[]byte(briefFrontmatter("teststream/01", "First thing derived", 0, "M")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-02-second.md"),
		[]byte(briefFrontmatter("teststream/02", "Second thing derived", 1, "S")), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parseStreamREADME(path)
	if err != nil {
		t.Fatalf("parseStreamREADME: %v", err)
	}
	s.Dir = dir
	return s, path
}

func TestReadmeTableRender(t *testing.T) {
	s, path := writeReadmeTableFixture(t, readmeTableFixtureBefore)
	_, region, _, ok := extractRegion(mustRead(t, path))
	if !ok {
		t.Fatal("fixture has no markers")
	}
	got := renderBriefsRegion(s, parsePreservedLifecycle(region))
	want := briefTableHead +
		"\n| 01 | [First thing derived](brief-01-first.md) | 0 | M | done | 2026-01-02 verifier | 2026-01-03 reviewer |" +
		"\n| 02 | [Second thing derived](brief-02-second.md) | 1 | S | todo | — | — |"
	if got != want {
		t.Fatalf("render mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestReadmeTableRewriteAndIdempotent(t *testing.T) {
	s, path := writeReadmeTableFixture(t, readmeTableFixtureBefore)

	changed, err := rewriteReadmeRegion(s, path)
	if err != nil {
		t.Fatalf("first rewrite: %v", err)
	}
	if !changed {
		t.Fatal("first rewrite should change the fixture (authoring columns disagree)")
	}
	after1 := mustRead(t, path)

	// Prose outside the markers is byte-preserved.
	if !strings.Contains(after1, "Prose ABOVE the table — must survive untouched.") ||
		!strings.Contains(after1, "Prose BELOW the table — must survive untouched.") {
		t.Fatal("rewrite disturbed prose outside the markers")
	}
	// Authoring columns now come from frontmatter; lifecycle preserved.
	if !strings.Contains(after1, "[First thing derived](brief-01-first.md)") {
		t.Fatal("authoring title not regenerated from frontmatter")
	}
	if !strings.Contains(after1, "| done | 2026-01-02 verifier | 2026-01-03 reviewer |") {
		t.Fatal("lifecycle cells were not preserved")
	}

	// Idempotency: a second rewrite changes nothing.
	changed2, err := rewriteReadmeRegion(s, path)
	if err != nil {
		t.Fatalf("second rewrite: %v", err)
	}
	if changed2 {
		t.Fatal("second rewrite must be a no-op (not idempotent)")
	}
	if after2 := mustRead(t, path); after2 != after1 {
		t.Fatal("second rewrite produced different bytes")
	}
}

func TestReadmeTableMarkersMissing(t *testing.T) {
	// board: generated but no markers — an error on rewrite, a PROBLEM on lint.
	noMarkers := strings.ReplaceAll(readmeTableFixtureBefore, briefsMarkerBegin+"\n", "")
	noMarkers = strings.ReplaceAll(noMarkers, briefsMarkerEnd+"\n", "")
	s, path := writeReadmeTableFixture(t, noMarkers)

	if _, err := rewriteReadmeRegion(s, path); err == nil {
		t.Fatal("rewrite must error when board: generated has no markers")
	}
	problems, _ := checkReadmeTables([]*Stream{s})
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "no "+briefsMarkerBegin) {
		t.Fatalf("expected a markers-missing PROBLEM, got %v", problems)
	}
}

// TestReadmeTableHandEditProblem is the fail-first guard for rule 47: a hand edit
// to an AUTHORING cell inside the markers is a PROBLEM naming the stream and row.
// It also proves the negative: an unedited region (regenerated) is clean, and a
// hand edit to a LIFECYCLE cell is NOT flagged offline (it is preserved).
func TestReadmeTableHandEditProblem(t *testing.T) {
	s, path := writeReadmeTableFixture(t, readmeTableFixtureBefore)
	// Bring the region to its canonical (clean) form first.
	if _, err := rewriteReadmeRegion(s, path); err != nil {
		t.Fatal(err)
	}
	clean, err := parseStreamREADME(path)
	if err != nil {
		t.Fatal(err)
	}
	clean.Dir = s.Dir
	if problems, _ := checkReadmeTables([]*Stream{clean}); len(problems) != 0 {
		t.Fatalf("a freshly regenerated region must be clean, got %v", problems)
	}

	// MUTATION: hand-edit an authoring cell (the title of row 01) inside the markers.
	edited := strings.Replace(mustRead(t, path), "[First thing derived]", "[HAND EDITED]", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	mut, err := parseStreamREADME(path)
	if err != nil {
		t.Fatal(err)
	}
	mut.Dir = s.Dir
	problems, _ := checkReadmeTables([]*Stream{mut})
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "hand edit to a generated table") {
		t.Fatalf("hand edit to an authoring cell must PROBLEM; got %v", problems)
	}
	if !strings.Contains(joined, "teststream") || !strings.Contains(joined, "row 01") {
		t.Fatalf("PROBLEM must name the stream and the row; got %v", problems)
	}

	// A hand edit to a LIFECYCLE cell (Status) is preserved, not flagged offline:
	// rewrite back to clean, then flip a status cell by hand.
	if _, err := rewriteReadmeRegion(s, path); err != nil {
		t.Fatal(err)
	}
	flipped := strings.Replace(mustRead(t, path), "| done | 2026-01-02 verifier", "| verified | 2026-01-02 verifier", 1)
	if err := os.WriteFile(path, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	lc, err := parseStreamREADME(path)
	if err != nil {
		t.Fatal(err)
	}
	lc.Dir = s.Dir
	if problems, _ := checkReadmeTables([]*Stream{lc}); len(problems) != 0 {
		t.Fatalf("a lifecycle-cell edit must NOT be an offline PROBLEM (it is preserved); got %v", problems)
	}
}

func TestReadmeTableDriftNotice(t *testing.T) {
	s := &Stream{
		Name: "teststream",
		Briefs: []Brief{
			{Num: "01", Status: "implemented"},
			{Num: "02", Status: "todo"},
			{Num: "03", Status: "done"},
		},
	}
	derived := []BriefCell{
		{ID: "teststream/01", Cell: "implemented", Witness: "PR #7 (merged abc1234)"}, // agrees → no notice
		{ID: "teststream/02", Cell: "in-progress", Witness: "PR #9 (draft def5678)"},  // drift → notice
		{ID: "teststream/03", Cell: "unknown", Reason: "offline"},                     // could-not-check → no notice
	}
	notices := assertedVsDerivedNotices(s, derived)
	if len(notices) != 1 {
		t.Fatalf("want exactly 1 drift NOTICE, got %d: %v", len(notices), notices)
	}
	n := notices[0]
	if !strings.Contains(n, "teststream/02") || !strings.Contains(n, "\"todo\"") || !strings.Contains(n, "\"in-progress\"") {
		t.Fatalf("drift NOTICE malformed: %q", n)
	}
	// The unknown row must never surface as a drift (three-state invariant).
	if strings.Contains(strings.Join(notices, "\n"), "teststream/03") {
		t.Fatalf("an unknown derived cell must not produce a drift NOTICE: %v", notices)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
