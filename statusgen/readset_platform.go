package main

// readset_platform.go — the platform decision for the read-set recorder
// (build-less-brittle/15). The recorder observes a row's file reads with
// ptrace and a seccomp filter, which only Linux offers; on every other OS a
// row's Runner cell carries a could-not-check token and no read-set file is
// written, so the witness keeps today's conservative scope.

import (
	"regexp"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// readSetRecorderShipped is THE switch for read-set judgement (review S20).
// It is false while this build carries no recorder: no caller observes a
// read set, the go floor never runs, and the library judges every
// `(reads sha256:…)` token could-not-check (evaluation.ReasonNoRecorder),
// so a row with one comes out exactly as the same row without it. The
// recorder slice flips it, once it also supplies what the judge then needs:
// a landing-identity binding on autoflip, a declared compile step for every
// go row, and the kind of each listed child.
const readSetRecorderShipped = false

// honourReadSets is what coverageOptionsFor reads. It is
// readSetRecorderShipped; only tests assign it (recordedReadSets), to drive
// the recorded path through the real callers.
var honourReadSets = readSetRecorderShipped

// readSetOffLinuxToken is the Runner-cell token a row gets on goos, an OS the
// recorder does not run on. readset_other.go calls it with runtime.GOOS.
func readSetOffLinuxToken(goos string) string {
	return evaluation.ReadSetCellToken("", "recorder is Linux-only ("+goos+")")
}

// writerReadsRe is the closed grammar of the read-set tokens verifyrun
// writes (review S21), whichever host wrote the row: a sha256 digest, the
// recorder-absent reason, or the off-Linux reason naming a GOOS. None of
// them can carry an `@`, a tree, a principal or a hand-written reason.
var writerReadsRe = regexp.MustCompile(`\(reads (?:sha256:[0-9a-f]{64}|could-not-check: (?:` +
	regexp.QuoteMeta(evaluation.ReasonNoRecorder) + `|recorder is Linux-only \([a-z0-9]+\)))\)`)

// writerReadsAtRe is writerReadsRe as the whole of a cell's first
// parenthetical, which is where runnerCellWithReads writes it.
var writerReadsAtRe = regexp.MustCompile(`^` + writerReadsRe.String())

// stripWriterReadSetToken removes a Runner cell's read-set token only where
// and as verifyrun writes it (runnerCellWithReads): one space and a token of
// writerReadsRe, as the cell's FIRST parenthetical, right after
// `<runner> @ <tree>` and ahead of the on-behalf-of and source parentheticals.
// Any other token-shaped text (another position, a free-text reason, a
// token naming a tree or principal) stays in the cell, so a check comparing
// the stripped cell and the tree and principal readers, which read the whole
// cell, see the same identity, tree and principal (review S21).
func stripWriterReadSetToken(cell string) string {
	first := strings.IndexByte(cell, '(')
	if first < 1 || cell[first-1] != ' ' {
		return cell
	}
	if m := writerReadsAtRe.FindString(cell[first:]); m != "" {
		return cell[:first-1] + cell[first+len(m):]
	}
	return cell
}

// stripWriterReadSetTokens removes every writer-shaped token (one space and
// a token of writerReadsRe) from a whole Evidence row's text. A token-shaped
// span with any other content stays in the text.
func stripWriterReadSetTokens(text string) string {
	return writerReadsSpanRe.ReplaceAllString(text, "")
}

var writerReadsSpanRe = regexp.MustCompile(` ` + writerReadsRe.String())
