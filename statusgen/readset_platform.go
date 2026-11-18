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
// go row, and the kind of each listed child. It flips only with the known
// limit brief 15 Task step 2 states accepted: the tracer reads clone3's
// clone_args and path arguments from tracee memory while sibling threads
// run, so an adversarial multi-threaded row can race those reads.
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

const reasonBriefOutsideTree = "the brief is outside the checkout being verified"

// Keep the host-only external-brief reason out of the protected container
// close check. Both grammars accept only the writer's exact token shapes.
var containerReadsPattern = `\(reads (?:sha256:[0-9a-f]{64}|could-not-check: (?:` +
	regexp.QuoteMeta(evaluation.ReasonNoRecorder) + `|recorder is Linux-only \([a-z0-9]+\)))\)`
var writerReadsPattern = `(?:` + containerReadsPattern + `|` + regexp.QuoteMeta(evaluation.ReadSetCellToken("", reasonBriefOutsideTree)) + `)`
var containerReadsAtRe = regexp.MustCompile(`^` + containerReadsPattern)
var writerReadsAtRe = regexp.MustCompile(`^` + writerReadsPattern)
var writerReadsSpanRe = regexp.MustCompile(` ` + writerReadsPattern)

// stripLeadingReadSetToken removes a Runner cell's read-set token only where
// and as verifyrun writes it (runnerCellWithReads): one space and a token of
// the caller's grammar, as the cell's FIRST parenthetical, right after
// `<runner> @ <tree>` and ahead of the on-behalf-of and source parentheticals.
// Any other token-shaped text (another position, a free-text reason, a
// token naming a tree or principal) stays in the cell, so a check comparing
// the stripped cell and the tree and principal readers, which read the whole
// cell, see the same identity, tree and principal (review S21).
func stripLeadingReadSetToken(cell string, allowed *regexp.Regexp) string {
	first := strings.IndexByte(cell, '(')
	if first < 1 || cell[first-1] != ' ' {
		return cell
	}
	if m := allowed.FindString(cell[first:]); m != "" {
		return cell[:first-1] + cell[first+len(m):]
	}
	return cell
}

// stripWriterReadSetTokens removes every writer-shaped token (one space and
// a token of writerReadsPattern) from a whole Evidence row's text. A token-shaped
// span with any other content stays in the text.
func stripWriterReadSetTokens(text string) string {
	return writerReadsSpanRe.ReplaceAllString(text, "")
}
