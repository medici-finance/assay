package main

// readmetable_extras_test.go — the generated Briefs table must be TOTAL over the
// board it re-renders, not a truncation of it.
//
// A consumer board carried a ninth column, `What's landed`, holding cells like
// `implemented — PR #87 merged 2026-08-16 (reviewer-app APPROVED)`. Nothing in
// the tree derives that content, and the render emitted a fixed seven cells, so
// the migration DELETED the whole column. Keying the lifecycle columns by header
// name fixes where the canonical cells are READ from; it does not by itself stop
// a non-canonical column being dropped on the floor.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// extrasReviewedCell and extrasLandedCell are the two cells that must survive
// byte-identical: the reviewer sign-off in the canonical Reviewed column, and the
// board's own extra-column content.
const (
	extrasReviewedCell = "2026-08-07 assay-reviewer-app[bot] (APPROVED PR #59 @ 429282e)"
	extrasLandedCell   = "implemented — PR #87 merged 2026-08-16 (reviewer-app APPROVED)"
)

// extrasFixtureTree writes a brief-v1 tree whose Briefs table carries a trailing
// `What's landed` column, with or without a `Gate` column ahead of it. Both
// shapes are fixtures because a consumer who had ALREADY dropped Gate by hand
// still hit the truncation — it is not a corollary of the Gate case.
func extrasFixtureTree(t *testing.T, withGate bool) string {
	t.Helper()
	root := t.TempDir()
	streams := filepath.Join(root, "docs", "streams")
	svc := filepath.Join(streams, "svc")
	if err := os.MkdirAll(svc, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := "schema: graph-repos-v1\ncell: example\nself: app\nrepos:\n  app: {cell: example, repo: example-org/app}\n"
	if err := os.WriteFile(filepath.Join(streams, "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := "---\nbrief: svc/01\ntitle: A brief\nwave: 0\ndepends: []\nunblocks: []\neffort: M\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\nauthored: 2026-07-09 by test\nsources: [\"note\"]\n---\n\n# Brief 01\n\n## Context\nfiles: x\n"
	if err := os.WriteFile(filepath.Join(svc, "brief-01-a.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	head := "| # | Brief | Wave | Effort | Status | Verified | Reviewed | What's landed |\n" +
		"|---|-------|------|--------|--------|----------|----------|---------------|\n"
	row := "| 01 | [A brief](./brief-01-a.md) | 0 | M | implemented | 2026-08-08 ci | " +
		extrasReviewedCell + " | " + extrasLandedCell + " |\n"
	if withGate {
		head = "| # | Brief | Wave | Effort | Gate | Status | Verified | Reviewed | What's landed |\n" +
			"|---|-------|------|--------|------|--------|----------|----------|---------------|\n"
		row = "| 01 | [A brief](./brief-01-a.md) | 0 | M | model | implemented | 2026-08-08 ci | " +
			extrasReviewedCell + " | " + extrasLandedCell + " |\n"
	}
	readme := "---\nstream: svc\nstatus: active\npriority: P1\ntrack: product\n---\n\n# Svc\n\n## Briefs\n\n" + head + row
	if err := os.WriteFile(filepath.Join(svc, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// extrasRegionRow returns the trimmed generated-region data row for brief num.
func extrasRegionRow(t *testing.T, path, num string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, region, _, ok := extractRegion(string(raw))
	if !ok {
		t.Fatalf("%s: no generated region:\n%s", path, raw)
	}
	for _, line := range strings.Split(region, "\n") {
		cells := splitRow(line)
		if len(cells) > 1 && strings.TrimSpace(cells[0]) == num {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("%s: region has no row for brief %s:\n%s", path, num, region)
	return ""
}

// TestMigrateCarriesExtraColumnThrough is the guard against truncation, end to
// end through the migration. Both the reviewer sign-off and the extra column's
// cell must survive byte-identical, the extra column must still be in the header,
// and the result must be a fixed point of the regen path — a render that dropped
// the column again on the NEXT regen would be the same data loss, one run later.
//
// Fail-first: against renderBriefsRegion as it stood before this change (a fixed
// seven-cell fmt.Sprintf over briefTableHead), this fails at the first assertion —
// `What's landed` is absent from the migrated header entirely:
//
//	the extra column was truncated out of the header
func TestMigrateCarriesExtraColumnThrough(t *testing.T) {
	for _, c := range []struct {
		name     string
		withGate bool
	}{
		{"alongside a Gate column", true},
		{"with Gate already dropped by hand", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := extrasFixtureTree(t, c.withGate)
			readme := filepath.Join(root, "docs", "streams", "svc", "README.md")

			var out, errb bytes.Buffer
			if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &out, &errb); code != 0 {
				t.Fatalf("migrate exit=%d, want 0; stderr=%s", code, errb.String())
			}
			body := string(mustRead(t, readme))
			if !strings.Contains(body, "| What's landed |") {
				t.Fatalf("the extra column was truncated out of the header:\n%s", body)
			}
			if c.withGate && strings.Contains(body, "| Gate |") {
				t.Errorf("Gate is the one column the layout drops, and it survived:\n%s", body)
			}

			row := extrasRegionRow(t, readme, "01")
			cells := splitRow(row)
			if len(cells) != 8 {
				t.Fatalf("migrated row should carry the 7 generated columns + 1 extra, got %d: %q", len(cells), row)
			}
			if got := strings.TrimSpace(cells[4]); got != "implemented" {
				t.Errorf("Status cell = %q, want %q", got, "implemented")
			}
			if got := strings.TrimSpace(cells[6]); got != extrasReviewedCell {
				t.Errorf("Reviewed cell = %q, want it byte-identical as %q", got, extrasReviewedCell)
			}
			if got := strings.TrimSpace(cells[7]); got != extrasLandedCell {
				t.Errorf("extra-column cell = %q, want it byte-identical as %q", got, extrasLandedCell)
			}

			// Regen-stable and lint-clean on the migrated tree.
			boardRoot, found := findBoardRoot(root)
			if !found {
				t.Fatalf("migrated tree has no board root")
			}
			streams, _, err := loadStreams(boardRoot)
			if err != nil {
				t.Fatalf("loadStreams: %v", err)
			}
			for _, s := range streams {
				changed, err := rewriteReadmeRegion(s, s.Dir+"/README.md")
				if err != nil {
					t.Fatalf("regen --readmes: %v", err)
				}
				if changed {
					t.Errorf("regen --readmes rewrote %s on a freshly migrated tree", s.Name)
				}
			}
			if problems, _ := checkReadmeTables(streams); len(problems) != 0 {
				t.Errorf("--lint PROBLEMs on the migrated tree: %v", problems)
			}
			after := string(mustRead(t, readme))
			if !strings.Contains(after, extrasLandedCell) {
				t.Errorf("the extra column's cell did not survive the regen pass:\n%s", after)
			}
			if !strings.Contains(after, extrasReviewedCell) {
				t.Errorf("the reviewer sign-off did not survive the regen pass:\n%s", after)
			}
		})
	}
}

// TestPreservedRegionCollectsExtraColumns isolates the parse unit: which columns
// are extras, and which are deliberately dropped.
func TestPreservedRegionCollectsExtraColumns(t *testing.T) {
	cases := []struct {
		name       string
		region     string
		wantNames  []string
		wantCells  []string
		wantStatus string
	}{
		{
			name: "canonical seven columns have no extras",
			region: "\n" + briefTableHead +
				"\n| 01 | [x](brief-01-x.md) | 0 | S | implemented | — | " + extrasReviewedCell + " |\n",
			wantStatus: "implemented",
		},
		{
			name: "Gate is dropped, never carried as an extra",
			region: "\n| # | Brief | Wave | Effort | Gate | Status | Verified | Reviewed |\n" +
				"|---|-------|------|--------|------|--------|----------|----------|\n" +
				"| 01 | [x](brief-01-x.md) | 0 | S | model | implemented | — | " + extrasReviewedCell + " |\n",
			wantStatus: "implemented",
		},
		{
			name: "a trailing board column is an extra",
			region: "\n| # | Brief | Wave | Effort | Status | Verified | Reviewed | What's landed |\n" +
				"|---|-------|------|--------|--------|----------|----------|---------------|\n" +
				"| 01 | [x](brief-01-x.md) | 0 | S | implemented | — | " + extrasReviewedCell + " | " + extrasLandedCell + " |\n",
			wantNames: []string{"What's landed"}, wantCells: []string{extrasLandedCell},
			wantStatus: "implemented",
		},
		{
			name: "Gate and an extra together",
			region: "\n| # | Brief | Wave | Effort | Gate | Status | Verified | Reviewed | What's landed |\n" +
				"|---|-------|------|--------|------|--------|----------|----------|---------------|\n" +
				"| 01 | [x](brief-01-x.md) | 0 | S | model | implemented | — | " + extrasReviewedCell + " | " + extrasLandedCell + " |\n",
			wantNames: []string{"What's landed"}, wantCells: []string{extrasLandedCell},
			wantStatus: "implemented",
		},
		{
			name: "an extra column BEFORE the lifecycle columns is still carried",
			region: "\n| # | Brief | Wave | Effort | Owner | Status | Verified | Reviewed |\n" +
				"|---|-------|------|--------|-------|--------|----------|----------|\n" +
				"| 01 | [x](brief-01-x.md) | 0 | S | platform | implemented | — | " + extrasReviewedCell + " |\n",
			wantNames: []string{"Owner"}, wantCells: []string{"platform"},
			wantStatus: "implemented",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lc, extras := parsePreservedRegion(c.region)
			if got := lc["01"].status; got != c.wantStatus {
				t.Errorf("status = %q, want %q", got, c.wantStatus)
			}
			if lc["01"].reviewed != extrasReviewedCell {
				t.Errorf("reviewed = %q, want %q", lc["01"].reviewed, extrasReviewedCell)
			}
			if len(extras.names) != len(c.wantNames) {
				t.Fatalf("extra columns = %v, want %v", extras.names, c.wantNames)
			}
			for i := range c.wantNames {
				if extras.names[i] != c.wantNames[i] {
					t.Errorf("extra column %d = %q, want %q", i, extras.names[i], c.wantNames[i])
				}
				row := extras.byNum["01"]
				if i >= len(row) || row[i] != c.wantCells[i] {
					t.Errorf("extra cell %d = %v, want %q", i, row, c.wantCells[i])
				}
			}
		})
	}
}

// TestBriefTableHeadWithIsByteStable pins that a board with NO extra column
// re-renders byte-for-byte as before — the header must not churn every tree in
// the fleet — and that an extra column's separator lines up under its name.
func TestBriefTableHeadWithIsByteStable(t *testing.T) {
	if got := briefTableHeadWith(nil); got != briefTableHead {
		t.Errorf("no extras must render the canonical header verbatim:\n%s", got)
	}
	got := briefTableHeadWith([]string{"What's landed"})
	lines := strings.SplitN(got, "\n", 2)
	if !strings.HasSuffix(lines[0], "| What's landed |") {
		t.Errorf("extra column not appended after Reviewed: %q", lines[0])
	}
	if len(splitRow(lines[0])) != len(splitRow(lines[1])) {
		t.Errorf("header and separator disagree on column count:\n%s\n%s", lines[0], lines[1])
	}
}
