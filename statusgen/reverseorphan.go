package main

import (
	"fmt"
	"os"
	"sort"
)

// reverseorphan.go — the REVERSE-ORPHAN lint (distribution/13 Task E-a).
//
// THE GAP THIS CLOSES. A stream README's brief table is authoritative for what
// the board shows: every row is a first-class brief in Next-up, scoring, the
// generated STATUS.md, and the append-only history. checkBriefFiles already
// guards the FORWARD direction — a brief FILE whose README row is absent is a
// PROBLEM ("no row NN in the README brief table"). But the REVERSE was
// unguarded: a README ROW whose brief FILE does not exist conjures a PHANTOM
// brief. Every file-iterating check is blind to it — checkBriefFiles,
// verifySectionProblems and attributionProblems all walk briefFilePaths(s), a
// glob over files that EXIST, so a row with no file behind it is invisible to
// all of them (main.go says exactly this at the off-board-classification note).
// Only the dead-link lint catches a phantom row today, and only INCIDENTALLY —
// when the row's Brief cell happens to be a Markdown link to the missing file.
// A row whose brief NUMBER simply has no brief-NN.md behind it slips through.
//
// This lint is the direct reverse of the forward check: for every real README
// brief row, assert a brief-NN[-slug].md file exists in the stream directory.
//
// THREE-STATE (docs/three-state-instrument-rule.md): checked-clean (a row with
// its file — silent), checked-failed (a phantom row — a hard PROBLEM), and
// could-not-check (the stream directory is unreadable — a NOTICE naming the
// stream, never a silent pass).
//
// EXEMPTIONS, both principled:
//   - placeholder-v1 SYNTHETIC rows. attachPlaceholders appends one Brief per
//     issue-NN.md placeholder file (Schema "placeholder-v1", Num like
//     "issue-300"); those are backed by an issue-*.md file, not a brief-*.md, so
//     the brief-file existence test does not apply to them.
//   - a stream with ZERO brief files. Such a stream is table-only (a legacy or
//     not-yet-split board that keeps its briefs inline in the README, with no
//     per-brief files at all). The convention this lint enforces — "a row is
//     backed by a brief file" — was never adopted there, so asserting it would
//     red an entire legitimate board rather than catch a stray phantom. Once a
//     stream has even one brief file it has opted into the file-backed
//     convention, and every row is then held to it.
func reverseOrphanProblems(streams []*Stream) (problems, notices []string) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	notice := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	for _, s := range streams {
		// could-not-check: read the directory directly so an unreadable stream
		// dir is reported by name, not silently treated as "no phantom rows".
		entries, err := os.ReadDir(s.Dir)
		if err != nil {
			notice("%s: could not read stream directory to check for phantom README rows (%v) — reverse-orphan lint could-not-check", s.Name, err)
			continue
		}
		fileNums := map[string]bool{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if _, num, ok := expectedBriefID(e.Name()); ok {
				fileNums[num] = true
			}
		}
		// The file-backed opt-in: a stream with no brief files at all is
		// table-only and exempt (see the header).
		if len(fileNums) == 0 {
			continue
		}
		for i := range s.Briefs {
			b := &s.Briefs[i]
			if b.Schema == "placeholder-v1" {
				continue // synthetic placeholder row — backed by issue-NN.md, not brief-NN.md
			}
			if !fileNums[b.Num] {
				add("%s: README brief row %q has no brief-%s[-slug].md file behind it — a phantom row conjures a brief that does not exist (reverse-orphan: the reverse of the file→row check). Add the brief file, or remove the row.", s.Name, b.Num, b.Num)
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}
