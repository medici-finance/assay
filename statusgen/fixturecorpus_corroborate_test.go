package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMarker drops an empty fixture-corpus marker at dir (relative to root),
// creating parents. Returns nothing; fails the test on error.
func writeMarker(t *testing.T, root, dir string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(dir))
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	if err := os.WriteFile(filepath.Join(full, fixtureCorpusMarkerName), nil, 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

// TestCorroborateHonoursDeclaredFixtureMarker is the corroborate-side proof that a
// DECLARED fixture corpus (a subtree carrying the .statusgen-fixtures marker on
// disk) has its human:<name> stamps skipped, while an UNMARKED path with the SAME
// content is still corroborated — the fail-closed direction. Neither the marked
// corpus nor the unmarked path is the hardcoded education prefix, so this exercises
// the generalized marker mechanism, not the pre-existing carve-out.
//
// The literal token is spelled with the "HSTAMP" sentinel and expanded at runtime
// (as the sibling corroborate tests do), so this source file carries no bare
// human:<name> literal for the --corroborate self-scan gate to trip on.
func TestCorroborateHonoursDeclaredFixtureMarker(t *testing.T) {
	root := t.TempDir()
	// A declared fixture corpus: an eval/run-output corpus that opts out.
	writeMarker(t, root, "docs/streams/somestream/evalcorpus")

	diff := strings.ReplaceAll(`diff --git a/docs/streams/somestream/evalcorpus/run-01.md b/docs/streams/somestream/evalcorpus/run-01.md
--- a/docs/streams/somestream/evalcorpus/run-01.md
+++ b/docs/streams/somestream/evalcorpus/run-01.md
+| 01 | Captured run | 0 | S | done | 2026-09-01 opus | 2026-09-01 HSTAMPalex |
diff --git a/docs/streams/somestream/evalcorpus/nested/run-02.md b/docs/streams/somestream/evalcorpus/nested/run-02.md
--- a/docs/streams/somestream/evalcorpus/nested/run-02.md
+++ b/docs/streams/somestream/evalcorpus/nested/run-02.md
+| 02 | Deeper capture | 0 | S | done | 2026-09-01 opus | 2026-09-01 HSTAMPsam |
diff --git a/docs/streams/somestream/README.md b/docs/streams/somestream/README.md
--- a/docs/streams/somestream/README.md
+++ b/docs/streams/somestream/README.md
+| 03 | Real board row | 0 | S | done | 2026-09-01 opus | 2026-09-01 HSTAMPalex |
`, "HSTAMP", "human:")

	stamps := stampsInDiff(root, diff)

	// (a) MARKED-CORPUS DIRECTION: no stamp may come from under the declared corpus,
	// at any depth. Before the marker was honoured this reddens — the corpus is a
	// live-scanned subtree and every HSTAMP produces a stamp.
	for _, s := range stamps {
		if strings.HasPrefix(s.File, "docs/streams/somestream/evalcorpus/") {
			t.Errorf("stamp %+v came from a DECLARED fixture corpus — it must be skipped", s)
		}
	}

	// (b) FAIL-CLOSED DIRECTION: the unmarked board file, with identical stamp
	// content, is STILL corroborated. This is the load-bearing control: the marker
	// excludes only its own subtree, never a blanket relaxation. (An over-broad fix
	// that excluded, say, all of docs/streams/ once any marker existed would redden
	// this.)
	var sawBoard bool
	for _, s := range stamps {
		if s.File == "docs/streams/somestream/README.md" {
			sawBoard = true
		}
	}
	if !sawBoard {
		t.Errorf("stamp on the UNMARKED board file was dropped — a path outside any declared corpus must still require corroboration (fail-closed)")
	}

	// End-to-end: with no reviews/comments, the unmarked stamp reports MISSING, and
	// the excluded corpus contributes nothing to corroborate at all.
	results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1)
	var boardMissing bool
	for _, r := range results {
		if strings.HasPrefix(r.Stamp.File, "docs/streams/somestream/evalcorpus/") {
			t.Errorf("excluded fixture stamp reached corroboration: %+v", r)
		}
		if r.Stamp.File == "docs/streams/somestream/README.md" && r.Verdict == verdictMissing {
			boardMissing = true
		}
	}
	if !boardMissing {
		t.Errorf("unmarked board stamp did not report MISSING-CORROBORATION — a forged stamp outside a declared corpus must still fail")
	}
}

// TestCorroborateEducationPrefixStillExcluded pins that generalizing the exclusion
// did NOT remove the original hardcoded education carve-out: a stamp under the
// tutorial-skeleton prefix is still skipped even with NO marker on disk.
func TestCorroborateEducationPrefixStillExcluded(t *testing.T) {
	root := t.TempDir() // deliberately NO marker anywhere
	diff := strings.ReplaceAll(`diff --git a/docs/streams/education/assay-tutorial-skeleton/root/README.md b/docs/streams/education/assay-tutorial-skeleton/root/README.md
--- a/docs/streams/education/assay-tutorial-skeleton/root/README.md
+++ b/docs/streams/education/assay-tutorial-skeleton/root/README.md
+| 01 | Tutorial | 0 | S | done | 2026-09-01 opus | 2026-09-01 HSTAMPalex |
`, "HSTAMP", "human:")
	stamps := stampsInDiff(root, diff)
	for _, s := range stamps {
		if strings.HasPrefix(s.File, corroborateExcludedFixturePrefix) {
			t.Errorf("stamp %+v from the education prefix was corroborated — the hardcoded carve-out must survive generalization", s)
		}
	}
}
