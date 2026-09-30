package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The record-section exemption: a backticked path that no longer exists is NOT a
// PROBLEM inside a record section (`## Evidence`, `## Proof it can fail`), and
// still IS one everywhere else in the same brief. Every test here goes through
// linkProblems, the lint's own entry point, so it pins the behaviour rather than
// the helper. Fail-first: TestRecordSectionEvidenceTableNotExistenceChecked and
// TestRecordSectionProofRecordNotExistenceChecked are red on the pre-exemption
// code; the still-checked tests are pinned by recordsection-mutations.json.

const retiredPath = "tools/retired/gone.sh"

// recordBrief writes one brief under docs/streams/x and returns its path.
func recordBrief(t *testing.T, content string) (root, path string) {
	t.Helper()
	root = t.TempDir()
	sub := filepath.Join(root, "docs", "streams", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, writeTemp(t, sub, "brief-01-x.md", content)
}

// retiredPathProblems returns the backticked-path problems naming retiredPath.
func retiredPathProblems(root, path string) []string {
	var out []string
	for _, p := range linkProblems(root, []string{path}) {
		if strings.Contains(p, "backticked path") && strings.Contains(p, retiredPath) {
			out = append(out, p)
		}
	}
	return out
}

// briefWith builds a full brief whose ONLY occurrence of retiredPath is the one
// appended to section. Every other section is present, so a detector that
// exempted the whole file (or the wrong range) is visible.
func briefWith(section, line string) string {
	secs := []struct{ h, body string }{
		{"## Context", "Some context."},
		{"## Task", "Do the thing."},
		{"## Deliverables", "- `statusgen/x.go`"},
		{"## Verify (executable — no prose-only DoD items)", "| # | Command | Expected |\n|---|---|---|\n| 1 | `true` | exit 0 |"},
		{"## Evidence", "| # | Result |\n|---|---|\n| 1 | ran |\n\n### Non-implementer verifier run — VERIFY: PASS — 2026-09-01\n\nrow 1 exit 0"},
		{"## Proof it can fail (mutation against the real tree)", "| # | Mutation | Result |\n|---|---|---|\n| M1 | bent | caught |"},
		{"## Review", "Reviewed."},
	}
	var b strings.Builder
	b.WriteString("# Brief 01 — x\n\n")
	for _, s := range secs {
		b.WriteString(s.h + "\n\n" + s.body + "\n")
		if s.h == section {
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestRecordSectionEvidenceTableNotExistenceChecked(t *testing.T) {
	for name, line := range map[string]string{
		"evidence table row": "| 2 | `" + retiredPath + "` exit 0 |",
		"verifier-run line":  "- row 2 ran `" + retiredPath + "` → exit 0",
	} {
		t.Run(name, func(t *testing.T) {
			root, p := recordBrief(t, briefWith("## Evidence", line))
			if got := retiredPathProblems(root, p); len(got) != 0 {
				t.Fatalf("a retired path inside ## Evidence must not be existence-checked, got: %v", got)
			}
		})
	}
}

func TestRecordSectionProofRecordNotExistenceChecked(t *testing.T) {
	root, p := recordBrief(t, briefWith("## Proof it can fail (mutation against the real tree)",
		"| M2 | `"+retiredPath+"` — guard deleted | caught |"))
	if got := retiredPathProblems(root, p); len(got) != 0 {
		t.Fatalf("a retired path inside a ## Proof it can fail record must not be existence-checked, got: %v", got)
	}
	// The undecorated heading is the same section.
	root, p = recordBrief(t, "# B\n\n## Task\n\nx\n\n## Proof it can fail\n\n| M1 | `"+retiredPath+"` | caught |\n")
	if got := retiredPathProblems(root, p); len(got) != 0 {
		t.Fatalf("undecorated ## Proof it can fail must be a record section, got: %v", got)
	}
}

func TestRecordSectionNonRecordSectionsStillChecked(t *testing.T) {
	for _, tc := range []struct{ name, section, line string }{
		{"verify command cell", "## Verify (executable — no prose-only DoD items)", "| 2 | `" + retiredPath + "` | exit 0 |"},
		{"deliverables line", "## Deliverables", "- `" + retiredPath + "`"},
		{"task prose", "## Task", "Edit `" + retiredPath + "` to do it."},
		{"context prose", "## Context", "See `" + retiredPath + "`."},
		{"section after the records", "## Review", "Follow-up touches `" + retiredPath + "`."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, p := recordBrief(t, briefWith(tc.section, tc.line))
			if got := retiredPathProblems(root, p); len(got) != 1 {
				t.Fatalf("a retired path in %s must still be a PROBLEM (1), got %d: %v", tc.section, len(got), got)
			}
		})
	}
}

// A heading that merely contains the word "evidence" — decorated, a subsection,
// or a phrase — is not a record section: only the exact `## Evidence` heading is.
func TestRecordSectionEvidenceWordInOtherHeadingStillChecked(t *testing.T) {
	for _, h := range []string{
		"## Evidence (notes)",
		"## Evidence notes for the implementer",
		"### Evidence",
		"## Design deviation (read before the Evidence table)",
		"## Gather evidence",
		"## Proof it can failover", // the proof heading needs a space before any decoration
	} {
		t.Run(h, func(t *testing.T) {
			root, p := recordBrief(t, "# B\n\n## Task\n\nx\n\n"+h+"\n\nTouches `"+retiredPath+"`.\n\n## Review\n\nok\n")
			if got := retiredPathProblems(root, p); len(got) != 1 {
				t.Fatalf("heading %q is not a record section — the path must still be a PROBLEM (1), got %d: %v", h, len(got), got)
			}
		})
	}
}

// The exemption is for EXISTENCE only: a dead markdown link inside Evidence is
// still reported by the link check.
func TestRecordSectionDeadLinkStillReported(t *testing.T) {
	root, p := recordBrief(t, briefWith("## Evidence", "| 2 | see [log](./missing-log.md) |"))
	var dead int
	for _, pr := range linkProblems(root, []string{p}) {
		if strings.Contains(pr, "dead link") && strings.Contains(pr, "missing-log.md") {
			dead++
		}
	}
	if dead != 1 {
		t.Fatalf("a dead markdown link inside ## Evidence must still be reported once, got %d", dead)
	}
}

func TestRecordSectionRangesAndLineIndex(t *testing.T) {
	raw := "# B\n## Task\nx\n## Evidence\nrow\nrow\n## Review\nr\n## Proof it can fail — M\nm\n"
	rs := recordSectionRanges(raw)
	want := recordRanges{{5, 6}, {10, 11}}
	if len(rs) != len(want) || rs[0] != want[0] || rs[1] != want[1] {
		t.Fatalf("recordSectionRanges = %v, want %v", rs, want)
	}
	li := newLineIndex(raw)
	for off, line := range map[int]int{0: 1, 3: 1, 4: 2, strings.Index(raw, "row"): 5, len(raw) - 1: 10} {
		if got := li.line(off); got != line {
			t.Errorf("line(%d) = %d, want %d", off, got, line)
		}
	}
}
