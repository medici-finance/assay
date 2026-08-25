package main

// transcribe-verdict write-scope guards (R-6 cl.4).
//
// R-6 cl.4 bounds what the verify-transcription lane may write: "modifications of
// docs/streams/<stream>/README.md status/Verified cells and appends to
// docs/streams/<stream>/brief-*.md `## Evidence` sections, and nothing else. For
// every touched brief the FRONTMATTER BLOCK and the `## Verify` SECTION are
// byte-identical before and after." The load-bearing reason (brief gate-why (c)):
// a Verify row is shell that verifyrun's scheduled main-rerun later EXECUTES from
// merged main, so letting a Verify-table edit ride this unattended lane would
// launder the F-verify-self-attest surface through it — and the write scope must
// not be able to exceed the cl.4 byte-bounds.
//
// This file adds the two cl.4 bounds the transcriber otherwise lacked:
//
//   - a per-entry BYTE BOUND on the exact Evidence markdown appended, plus an
//     eval-time refusal of Evidence that carries a Markdown section heading (a
//     `## Verify` table injection is the shape this forbids); and
//   - a POST-APPLY INVARIANT asserting the frontmatter block and the `## Verify`
//     section are byte-identical before and after — the backstop that keeps a
//     bug or a crafted append from ever mutating either, independent of the
//     eval-time checks.

import (
	"fmt"
	"strings"
)

// verdictMaxEvidenceBytes bounds the R-6 cl.4 write class per entry: the exact
// Evidence markdown a single verdict entry appends. A payload carrying a
// multi-kilobyte "evidence" string is refused as out-of-class, never committed.
const verdictMaxEvidenceBytes = 2048

// verdictEvidenceInjectsHeading reports whether an Evidence markdown string
// carries a Markdown ATX heading line (`# `, `## `, …) — the shape that would
// inject a new section (a `## Verify` table included) rather than append an
// Evidence row. cl.4.
func verdictEvidenceInjectsHeading(evidence string) bool {
	for _, ln := range strings.Split(evidence, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "#") {
			h := strings.TrimLeft(t, "#")
			if h != t && (h == "" || strings.HasPrefix(h, " ")) {
				return true
			}
		}
	}
	return false
}

// verdictFrontmatterBytes returns the raw text of the leading `---` … `---`
// frontmatter block (inclusive), or an error when there is none.
func verdictFrontmatterBytes(s string) (string, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", fmt.Errorf("no frontmatter")
	}
	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", fmt.Errorf("unterminated frontmatter")
	}
	return s[:len("---\n")+end+len("\n---")], nil
}

// verdictSectionBody returns the body of the `## <name>` section (prefix-matched
// heading, decorated headings allowed), between it and the next `## ` heading.
// Used only for the cl.4 invariant comparison.
func verdictSectionBody(s, name string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	head := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") && strings.HasPrefix(strings.TrimSpace(t[3:]), name) {
			head = i
			break
		}
	}
	if head < 0 {
		return ""
	}
	end := len(lines)
	for i := head + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[head+1:end], "\n")
}

// assertVerdictWriteScope enforces cl.4 as a post-apply backstop: a verify landing
// appends Evidence and flips a cell; it never edits the frontmatter block or the
// `## Verify` table. Both must be byte-identical before and after.
func assertVerdictWriteScope(before, after string) error {
	fb, ferr := verdictFrontmatterBytes(before)
	fa, ferr2 := verdictFrontmatterBytes(after)
	if ferr != nil || ferr2 != nil {
		return fmt.Errorf("could not isolate the frontmatter block for comparison")
	}
	if fb != fa {
		return fmt.Errorf("the frontmatter block changed")
	}
	if verdictSectionBody(before, "Verify") != verdictSectionBody(after, "Verify") {
		return fmt.Errorf("the `## Verify` section changed")
	}
	return nil
}
