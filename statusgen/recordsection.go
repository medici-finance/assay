package main

import (
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// record sections — the dated parts of a brief that record what was true when
// they were written, exempt from the backticked-path EXISTENCE check only.
//
// WHY IT EXISTS. The backticked-path check (linkProblems) fails any path in a
// brief that does not exist in the current tree. That is right for the parts of
// a brief that describe work still to be true — Task, Deliverables, Context, the
// Verify table's command cells, ordinary prose — and wrong for the parts that
// RECORD a run: an Evidence table, a verifier-run write-up, a mutation record.
// When a file such a record names is later moved or retired, the record is still
// accurate about the tree it ran against, yet the check goes red, and the only
// ways back to green were to rewrite the record (falsifying history) or to
// annotate every line of it.
//
// WHAT COUNTS AS A RECORD SECTION. Two headings, both located by the ONE section
// parser statusgen already uses for Evidence (sectionLineRange, behind
// evidenceLineRange) — the body runs from the heading to the next `## ` heading
// or EOF, and only the FIRST occurrence of each heading opens a section:
//
//   - `## Evidence` — exact after trimming, the same test every other Evidence
//     reader uses (isEvidenceHeading). Verifier-run write-ups (`### Non-
//     implementer verifier run …`), witness tables and fail-first/mutation
//     subsections live UNDER it and are covered by it. A decorated `## Evidence
//     (notes)`, a `### Evidence` subsection elsewhere, or any other heading that
//     merely contains the word is NOT a record section and stays checked.
//   - `## Proof it can fail` — exact, or followed by a space and a decoration
//     (`## Proof it can fail (mutation against the real tree)`): the top-level
//     mutation-record section some briefs carry beside their Evidence. Unlike
//     Evidence it has no brief-template contract fixing its spelling, and the
//     decorated form is the one in use, so a decoration after a space is
//     accepted; a heading that only contains the phrase mid-line is not.
//
// THE BOUNDARY. This exempts path EXISTENCE and nothing else: markdown links in
// a record section are still dead-link checked, the identifier-dereference check
// is unchanged, and every other lint that reads Evidence reads it exactly as
// before. Outside the two sections — Task, Deliverables, Context, the Verify
// table, prose — a missing backticked path is still a PROBLEM.
// ---------------------------------------------------------------------------

// isProofRecordHeading is the `## Proof it can fail` heading test.
func isProofRecordHeading(trimmed string) bool {
	const h = "## Proof it can fail"
	return trimmed == h || strings.HasPrefix(trimmed, h+" ")
}

// recordSectionHeadings is the complete record-section heading set.
var recordSectionHeadings = []func(trimmed string) bool{
	isEvidenceHeading,
	isProofRecordHeading,
}

// lineRange is a 1-based inclusive line range.
type lineRange struct{ start, end int }

// recordRanges is the set of record-section body ranges in one file.
type recordRanges []lineRange

// recordSectionRanges locates every record section of a raw brief file.
func recordSectionRanges(raw string) recordRanges {
	var rs recordRanges
	for _, isHeading := range recordSectionHeadings {
		if s, e, ok := sectionLineRange(raw, isHeading); ok {
			rs = append(rs, lineRange{start: s, end: e})
		}
	}
	return rs
}

// contains reports whether the 1-based line falls inside a record section.
func (rs recordRanges) contains(line int) bool {
	for _, r := range rs {
		if line >= r.start && line <= r.end {
			return true
		}
	}
	return false
}

// lineIndex maps a byte offset in a file to its 1-based line number, split on
// "\n" exactly as sectionLineRange splits, so offsets and ranges agree.
type lineIndex []int // byte offset of the start of each line

func newLineIndex(content string) lineIndex {
	idx := lineIndex{0}
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

// line returns the 1-based line number holding byte offset off.
func (li lineIndex) line(off int) int {
	return sort.Search(len(li), func(i int) bool { return li[i] > off })
}
