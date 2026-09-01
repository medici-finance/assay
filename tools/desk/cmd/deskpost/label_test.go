package main

import (
	"sort"
	"strings"
	"testing"
)

// These tests exercise the mechanical verdict-time labeler against the fake GitHub server.
// They cover the brief's Verify rows 2-5 in-process (rows requiring a "real merged PR" are
// exercised here through the fake, which is a faithful stand-in for the /files, /labels and
// /contents endpoints the labeler drives — no live infrastructure).

func strptr(s string) *string { return &s }

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}

func surfaceGlobsFixture() string {
	return ".github/workflows/**\ntools/desk/cmd/*guard*/**\n.claude/guardrails/**\n"
}

// Row 2 — a verdict on a PR whose diff touches .github/workflows/ gets surface:core + the
// correct size label, and a comment naming the matched glob. Driven through the full review
// verb so the WIRING (postVerdictReview → applyVerdictLabels) is exercised end to end.
func TestVerdictLabelsWorkflowPRGetsCoreAndSize(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.surfaceConfig = strptr(surfaceGlobsFixture())
	f.fileEntries = []prFile{
		{Filename: ".github/workflows/leaksweep-control.yml", Additions: 100, Deletions: 50}, // 150, counts
		{Filename: "docs/notes.md", Additions: 20, Deletions: 10},                            // 30, counts
	} // total 180 → size:M
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("review exit = %d, want 0", code)
	}
	if !hasLabel(f.prLabels, "size:M") {
		t.Errorf("labels %v missing size:M", f.prLabels)
	}
	if !hasLabel(f.prLabels, "surface:core") {
		t.Errorf("labels %v missing surface:core", f.prLabels)
	}
	if f.postedCmt != 1 {
		t.Errorf("postedCmt = %d, want 1 (surface:core dereferencing comment)", f.postedCmt)
	}
	// The comment must name the matched glob.
	if len(f.issueComments) != 1 {
		t.Fatalf("issueComments = %v, want 1", f.issueComments)
	}
	body, _ := f.issueComments[0]["body"].(string)
	if !strings.Contains(body, ".github/workflows/**") {
		t.Errorf("surface:core comment does not name the matched glob: %q", body)
	}
}

// Row 3 (NEGATIVE PATH) — a docs-only PR with the config present gets surface:std and NO
// core comment.
func TestVerdictLabelsDocsOnlyGetsStdNoComment(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.surfaceConfig = strptr(surfaceGlobsFixture())
	f.fileEntries = []prFile{
		{Filename: "docs/a.md", Additions: 30, Deletions: 10},
		{Filename: "README.md", Additions: 5, Deletions: 0},
	} // total 45 → size:S
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("review exit = %d, want 0", code)
	}
	if !hasLabel(f.prLabels, "size:S") {
		t.Errorf("labels %v missing size:S", f.prLabels)
	}
	if !hasLabel(f.prLabels, "surface:std") {
		t.Errorf("labels %v missing surface:std", f.prLabels)
	}
	if hasLabel(f.prLabels, "surface:core") {
		t.Errorf("labels %v must not carry surface:core for a docs-only PR", f.prLabels)
	}
	if f.postedCmt != 0 {
		t.Errorf("postedCmt = %d, want 0 (no core comment on the negative path)", f.postedCmt)
	}
}

// Row 4 — absent .assay-surfaces: size label only, NO surface label, exit 0. The surface
// family is left in the three-state Unknown, never guessed as std.
func TestVerdictLabelsAbsentConfigSizeOnly(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.surfaceConfig = nil // GET /contents/.assay-surfaces → 404
	f.fileEntries = []prFile{
		{Filename: ".github/workflows/ci.yml", Additions: 300, Deletions: 200}, // 500 → size:L
	}
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("review exit = %d, want 0", code)
	}
	if !hasLabel(f.prLabels, "size:L") {
		t.Errorf("labels %v missing size:L", f.prLabels)
	}
	for _, l := range f.prLabels {
		if len(l) >= len("surface:") && l[:len("surface:")] == "surface:" {
			t.Errorf("labels %v must carry NO surface label when .assay-surfaces is absent", f.prLabels)
		}
	}
	if f.postedCmt != 0 {
		t.Errorf("postedCmt = %d, want 0 (no config, no core comment)", f.postedCmt)
	}
}

// Row 5 — idempotence: running the labeler twice on one PR yields identical labels with no
// duplicates, and posts the core comment only once. Called directly (a second full review
// at the same head no-ops before labeling), so the label apply itself is what is pinned.
func TestVerdictLabelsIdempotent(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.surfaceConfig = strptr(surfaceGlobsFixture())
	f.fileEntries = []prFile{
		{Filename: "tools/desk/cmd/writeguard/main.go", Additions: 120, Deletions: 40}, // 160 → size:M, core
	}
	c, err := newGHClient("example-org", "tracker")
	if err != nil {
		t.Fatalf("newGHClient: %v", err)
	}
	if _, err := applyVerdictLabels(c, 1, f.reportedChangedFiles()); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	first := append([]string(nil), f.prLabels...)
	if _, err := applyVerdictLabels(c, 1, f.reportedChangedFiles()); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	sort.Strings(first)
	second := append([]string(nil), f.prLabels...)
	sort.Strings(second)
	if len(second) != len(first) {
		t.Fatalf("label count changed across runs: %v -> %v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("labels differ across runs: %v vs %v", first, second)
		}
	}
	// No duplicates.
	seen := map[string]bool{}
	for _, l := range second {
		if seen[l] {
			t.Fatalf("duplicate label %q in %v", l, second)
		}
		seen[l] = true
	}
	if !hasLabel(second, "size:M") || !hasLabel(second, "surface:core") {
		t.Fatalf("expected size:M + surface:core, got %v", second)
	}
	if f.postedCmt != 1 {
		t.Errorf("postedCmt = %d, want 1 across two runs (marker dedup)", f.postedCmt)
	}
}

// Stale same-family labels are REPLACED, not stacked: a PR carrying an old size + old
// surface tier gets them removed when the current classification differs.
func TestVerdictLabelsReplaceStaleFamily(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.surfaceConfig = strptr(surfaceGlobsFixture())
	f.prLabels = []string{"size:L", "surface:std", "dispatched-model:opus-4.8"} // pre-existing
	f.fileEntries = []prFile{
		{Filename: ".github/workflows/ci.yml", Additions: 100, Deletions: 50}, // 150 → size:M, core
	}
	c, err := newGHClient("example-org", "tracker")
	if err != nil {
		t.Fatalf("newGHClient: %v", err)
	}
	if _, err := applyVerdictLabels(c, 1, f.reportedChangedFiles()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if hasLabel(f.prLabels, "size:L") {
		t.Errorf("stale size:L not removed: %v", f.prLabels)
	}
	if hasLabel(f.prLabels, "surface:std") {
		t.Errorf("stale surface:std not removed: %v", f.prLabels)
	}
	if !hasLabel(f.prLabels, "size:M") || !hasLabel(f.prLabels, "surface:core") {
		t.Errorf("current labels not applied: %v", f.prLabels)
	}
	// An UNRELATED family (the dispatcher model stamp) must be left untouched.
	if !hasLabel(f.prLabels, "dispatched-model:opus-4.8") {
		t.Errorf("labeler disturbed an unrelated-family label: %v", f.prLabels)
	}
}

// A short files read (fewer entries than GitHub's changed_files count) applies NEITHER
// label — the determination is could-not-check, never a label derived from a partial diff.
func TestVerdictLabelsShortReadSkips(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.surfaceConfig = strptr(surfaceGlobsFixture())
	f.fileEntries = []prFile{
		{Filename: ".github/workflows/ci.yml", Additions: 10, Deletions: 5},
	}
	f.changedFilesCount = 5 // GitHub reports 5, we read 1 → short read
	c, err := newGHClient("example-org", "tracker")
	if err != nil {
		t.Fatalf("newGHClient: %v", err)
	}
	out, err := applyVerdictLabels(c, 1, f.reportedChangedFiles())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(f.prLabels) != 0 {
		t.Errorf("short read must apply no labels, got %v", f.prLabels)
	}
	if len(out.notes) == 0 {
		t.Errorf("short read must record a could-not-check note")
	}
}
