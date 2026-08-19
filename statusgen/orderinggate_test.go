package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBrief drops a minimal-but-valid brief-v1 file into dir so parseBriefFile
// accepts it and orderingGateEdges can read its depends:/unblocks: edges.
func writeBrief(t *testing.T, dir, num string, depends, unblocks []string) {
	t.Helper()
	stream := filepath.Base(dir)
	q := func(xs []string) string {
		if len(xs) == 0 {
			return "[]"
		}
		var quoted []string
		for _, x := range xs {
			quoted = append(quoted, fmt.Sprintf("%q", x))
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	}
	body := fmt.Sprintf(`---
brief: %s/%s
title: fixture brief %s
wave: 0
depends: %s
unblocks: %s
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-17 by fixture
sources: ["fixture"]
---

# Brief %s
Body.
`, stream, num, num, q(depends), q(unblocks), num)
	if err := os.WriteFile(filepath.Join(dir, "brief-"+num+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestOrderingGateSilentDispatchIsFlagged is the #1250 regression: gate-shaped
// README prose whose ordering prerequisite is encoded in NO typed edge must
// surface as a NOTICE. Before this lint existed the same prose produced ZERO
// output — the silent premature-dispatch the issue reports — so this test fails
// on the old behaviour.
func TestOrderingGateSilentDispatchIsFlagged(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "pods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Briefs 05/06/07 exist but carry no edge encoding the §8 gate.
	writeBrief(t, dir, "05", nil, nil)
	writeBrief(t, dir, "06", nil, nil)
	writeBrief(t, dir, "07", nil, nil)

	// The exact desk-console prose shape: a range of briefs gated on a prose-only
	// prerequisite ("this lands" = a spec §, no ref).
	readme := "# pods\n\n" +
		"A pod loop that can still block is undebuggable; no CronJob\n" +
		"brief (05–07) starts before this lands.\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	notices := orderingGateNotices([]*Stream{{Name: "pods", Dir: dir}})
	if !hasProblem(notices, "ordering-gate", "README.md", "#1250") {
		t.Errorf("prose-only ordering gate must raise a NOTICE (the #1250 silent-dispatch class); got:\n%s", strings.Join(notices, "\n"))
	}
}

// TestOrderingGateEncodedEdgeIsClean: once the prerequisite IS a typed
// depends:/unblocks: edge between the two named briefs, the surviving caption
// prose is clean — the lint must not nag an edge that already exists.
func TestOrderingGateEncodedEdgeIsClean(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "pods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBrief(t, dir, "04", nil, nil)
	writeBrief(t, dir, "06", []string{"pods/04"}, nil) // 06 depends on 04 — the edge exists

	readme := "# pods\n\nBrief 06 is blocked on brief-04 until it lands.\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	notices := orderingGateNotices([]*Stream{{Name: "pods", Dir: dir}})
	for _, n := range notices {
		if strings.Contains(n, "README.md") {
			t.Errorf("an encoded depends: edge must leave the caption prose clean; got NOTICE: %s", n)
		}
	}
}

// TestOrderingGateUnencodedPairIsFlagged: two briefs named on opposite sides of
// the gate keyword with NO edge between them is the migrate-me case.
func TestOrderingGateUnencodedPairIsFlagged(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "pods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBrief(t, dir, "06", nil, nil)
	writeBrief(t, dir, "09", nil, nil)

	readme := "# pods\n\nBrief 06 is blocked on brief-09.\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	notices := orderingGateNotices([]*Stream{{Name: "pods", Dir: dir}})
	if !hasProblem(notices, "ordering-gate", "pods/06", "pods/09") {
		t.Errorf("an unencoded gate between two named briefs must be flagged; got:\n%s", strings.Join(notices, "\n"))
	}
}

// TestOrderingGateNegationSkipped: a line that DENIES a block ("never blocks
// brief-08") reads gate-shaped but asserts the opposite — a certain false
// positive that must be skipped (design §6.2).
func TestOrderingGateNegationSkipped(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "pods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBrief(t, dir, "08", nil, nil)

	readme := "# pods\n\nThis edge never blocks brief-08 until the sun burns out.\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	notices := orderingGateNotices([]*Stream{{Name: "pods", Dir: dir}})
	for _, n := range notices {
		if strings.Contains(n, "README.md") {
			t.Errorf("a negated non-gate line must not be flagged; got NOTICE: %s", n)
		}
	}
}

// TestOrderingGateWaiver: an explicit waiver with a reason-class silences the
// line; a bare `<!-- graph: -->` is itself a NOTICE.
func TestOrderingGateWaiver(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "pods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBrief(t, dir, "06", nil, nil)

	t.Run("reasoned waiver silences", func(t *testing.T) {
		readme := "# pods\n\nBrief 06 was blocked on the old design. <!-- graph: not-a-gate -->\n"
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
			t.Fatal(err)
		}
		notices := orderingGateNotices([]*Stream{{Name: "pods", Dir: dir}})
		for _, n := range notices {
			if strings.Contains(n, "README.md") {
				t.Errorf("a reasoned graph waiver must silence the line; got NOTICE: %s", n)
			}
		}
	})

	t.Run("bare waiver is itself a notice", func(t *testing.T) {
		readme := "# pods\n\nBrief 06 blocked on something. <!-- graph: -->\n"
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
			t.Fatal(err)
		}
		notices := orderingGateNotices([]*Stream{{Name: "pods", Dir: dir}})
		if !hasProblem(notices, "ordering-gate", "bare") {
			t.Errorf("a bare `<!-- graph: -->` waiver must raise a NOTICE; got:\n%s", strings.Join(notices, "\n"))
		}
	})
}
