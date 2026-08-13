package deskkit

// verdictmarker.go is the CANONICAL reader for the `Security-Review:`
// verdict line — the one place that decides whether a review body carries a pass or a
// fail. It lives in deskkit, not in deskpost's bodycheck package, because deskkit is the
// only tree BOTH deskpost (cmd/deskpost/internal/bodycheck, a Go "internal" package that
// only code under cmd/deskpost/... may import) and deskboard (cmd/deskboard, a sibling
// tree) can reach.
//
// #408: before this file existed, deskboard/board.go carried its OWN third
// reader — `strings.TrimSpace(ln) == marker`, exact and case-SENSITIVE — that never went
// through bodycheck at all. #232 widened deskpost's read path to recover verdicts written
// with emphasis (`**Security-Review: fail**`); deskboard was not widened with it, so on
// the two live #1284 heads carrying an emphasised retraction, deskpost reported a
// standing retraction while deskboard reported "no security verdict — dispatch
// /security-review". Both answers blocked (deskboard's accepted set was always a strict
// SUBSET of deskpost's, so the divergence erred safe), but the board's stated REASON was
// wrong, and the reason is what a desk operator acts on.
//
// The fix is ONE reader with ONE accepted set: deskpost's bodycheck.HasSecurityReviewPass
// / HasSecurityReviewFail now delegate here, and deskboard calls these functions directly
// instead of carrying its own compare.
//
// The two directions are deliberately ASYMMETRIC about fenced code blocks (see
// HasSecurityReviewPass / HasSecurityReviewFail) — a caller that re-implements either
// half reopens the gap this file closes.

import (
	"regexp"
	"strings"
)

// FencePolicy says what the reader does with lines inside a fenced code block.
type FencePolicy bool

const (
	SkipFenced FencePolicy = true  // fenced lines are not candidate markers (GRANT path)
	ReadFenced FencePolicy = false // fenced lines are candidate markers (BLOCK path)
)

// HasSecurityReviewPass reports whether body carries the security-review PASS verdict
// line. Used by deskpost's ready gate and by deskboard's ACTION
// classifier: a risk-classed PR flips only when an App review at the CURRENT head carries
// this line.
//
// READ PATH — emphasis-tolerant (#232). See unwrapEmphasis.
//
// A marker inside a fenced code block does NOT count here. A fenced block is how the
// marker gets QUOTED in documentation, a skill, or a review that is explaining the format,
// and none of those is a grant. This is the GRANT direction, so the exclusion runs the
// blocking way; the FAIL direction deliberately does not have it (see
// HasSecurityReviewFail).
func HasSecurityReviewPass(body string) bool {
	return HasVerdictMarkerLine(body, secReviewPass, SkipFenced)
}

// HasSecurityReviewFail reports whether body carries the security-review FAIL verdict
// line. Without it a retraction is invisible: a reader that only ever
// looked for `pass` would read a `pass` followed by a `fail` at the SAME head as green
// (#216). Callers reduce pass/fail in submission order.
//
// READ PATH — emphasis-tolerant (#232). See unwrapEmphasis.
//
// UNLIKE the pass path, a fenced marker DOES count here. That asymmetry is the point.
// Both exclusions have to run in the blocking direction or a fence becomes a place to
// hide a verdict: skipping fenced lines on the pass path can only ever WITHHOLD a grant,
// while skipping them on the fail path would let a retraction go unread — and on a
// non-risk-classed path gate (e0) is the ONLY thing a fail has left to block with. The
// write path (deskpost's bodycheck.Review/VerdictKind) is not fence-aware, so a body whose
// only verdict line sits in a fence is postable; it must not become invisible to the
// reader that blocks.
func HasSecurityReviewFail(body string) bool {
	return HasVerdictMarkerLine(body, secReviewFail, ReadFenced)
}

// HasVerdictMarkerLine reports whether any line of body matches re once its surrounding
// Markdown emphasis has been unwrapped. It is the shared reduction under
// HasSecurityReviewPass/Fail (and deskpost's CorrectnessVerdictLine) — exported so every
// caller applies the same fence and emphasis handling rather than a hand-rolled compare.
//
// A fence DELIMITER line (``` or ~~~) is never a candidate under either policy: it is
// punctuation, and treating it as content is what let an inline triple-backtick span on
// its own line reduce to a bare marker.
func HasVerdictMarkerLine(body string, re *regexp.Regexp, fp FencePolicy) bool {
	inFence := false
	for _, ln := range strings.Split(body, "\n") {
		if isFenceDelimiter(ln) {
			inFence = !inFence
			continue
		}
		if inFence && fp == SkipFenced {
			continue
		}
		if re.MatchString(unwrapEmphasis(ln)) {
			return true
		}
	}
	return false
}

// fenceDelim matches a Markdown fenced-code-block delimiter line: three or more backticks
// or tildes, optionally indented, optionally followed by an info string.
//
// It is deliberately looser than CommonMark. CommonMark says an opening fence's info
// string may not contain a backtick, which would make ```` ```Security-Review: pass``` ````
// an inline code SPAN rather than a fence — and therefore, under a strict reading, a
// candidate marker line. Both readings of that line are punctuation-wrapped quotation, not
// a verdict, so the reader refuses to treat either as one.
var fenceDelim = regexp.MustCompile("^[ \t]*(```|~~~)")

func isFenceDelimiter(line string) bool { return fenceDelim.MatchString(line) }

// emphasisChars are the Markdown emphasis/code delimiter characters.
const emphasisChars = "*_`"

// unwrapEmphasis removes the runs of Markdown emphasis/code delimiters that are acting as
// DELIMITERS from one line, and leaves every other occurrence in place.
//
// WHY (#232): the marker regexps are whole-line anchored, and #219 made the
// ready gate's security check order-sensitive on top of them. Every `Security-Review:`
// verdict written before that install was posted with emphasis — `**Security-Review:
// pass**`, `**Security-Review: fail**` — which an anchored regexp does not match. The
// gate therefore reported "no security verdict at head" on PRs that carry one. Across the
// live corpus of reviewer-App reviews on open PRs, many bodies carry a `Security-Review:`
// mention but only some match the strict whole-line form, and unwrapping changes the
// verdict only in the retraction direction — head-pinned `**Security-Review: fail**`
// RETRACTIONS (e.g. on #1284) that were invisible to both the pass check and the fail
// check. No emphasised PASS is recovered anywhere in the live corpus; the recovery is
// entirely in the retraction direction.
//
// A retraction that cannot be read is the worst direction of this defect: the earlier
// `pass` stays the visible state and the PR stays flippable, which is exactly the hole
// #216/#219 closed at the read path and this reopened at the parse.
//
// WHY THIS IS NOT A DELETE-EVERYWHERE STRIP. The first cut of this ran
// `strings.NewReplacer("*", "", "_", "", "`", "")` over the whole line. `*` is also
// Markdown's alternate BULLET character, so `* Security-Review: pass` — an ordinary list
// item in an ordinary correctness review — reduced to a bare marker and granted a security
// pass that had never been posted. Deleting emphasis characters from the MIDDLE of a word
// is equally unwanted: `Security-Rev*iew: pass`, `Security_Review: pass` and
// `Security-Review: pa`ss“ are not the format and must not read as it.
//
// So a run is dropped only where it is doing a delimiter's job (see isDelimiterRun), and
// the two rules that decide it are the two defects above:
//
//   - a run at the START of the line followed by whitespace is a BULLET, and is kept —
//     which is what stops `* Security-Review: pass` (and, by construction, `* **…**` and
//     “ * `…` “) from ever reducing to a marker;
//   - an INTERIOR run is kept whenever dropping it would JOIN two alphanumerics. That is
//     the general form of the rule, and it is why no amount of emphasis can MANUFACTURE a
//     keyword: unwrapping can never fuse `pa` and `ss` into `pass`, nor `Security-Re` and
//     `view` into the key, so a line that did not already contain the marker's words in
//     order cannot acquire them here. It also means `fail` can never reduce to `pass`.
//
// The rest of the anchor is untouched, so:
//
//   - the line must still reduce to exactly `Security-Review: pass|fail` with nothing else
//     on it;
//   - the documented `> ` quoting escape hatch survives, because `>` is not a delimiter:
//     `> Security-Review: pass` still does not match;
//   - a marker inside a table cell or a sentence still does not match, for the same
//     whole-line reason (`| Security-Review: pass |`, `…verdict: `Security-Review:
//     pass`.`).
//
// THIS IS THE READ PATH ONLY. deskpost's bodycheck.Review and VerdictKind (the WRITE gate)
// stay strictly anchored, so new artifacts must still be canonical and the kind stays
// non-caller-controllable.
func unwrapEmphasis(line string) string {
	s := strings.Trim(line, " \t\r")
	var b strings.Builder
	for i := 0; i < len(s); {
		if !isEmphasisByte(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && isEmphasisByte(s[j]) {
			j++
		}
		if !isDelimiterRun(s, i, j) {
			b.WriteString(s[i:j])
		}
		i = j
	}
	return b.String()
}

// isDelimiterRun reports whether s[i:j] — a maximal run of emphasis characters — is
// wrapping something rather than being part of it. See unwrapEmphasis for the rationale.
func isDelimiterRun(s string, i, j int) bool {
	atStart, atEnd := i == 0, j == len(s)
	switch {
	case atStart && atEnd:
		return false // the line is nothing but emphasis characters; there is no content
	case atStart:
		// `**Security-Review: pass**` opens with a delimiter; `* Security-Review: pass`
		// opens with a BULLET. The difference is the whitespace after the run.
		return !isSpaceByte(s[j])
	case atEnd:
		return !isSpaceByte(s[i-1])
	default:
		return !(isAlnumByte(s[i-1]) && isAlnumByte(s[j]))
	}
}

func isEmphasisByte(c byte) bool { return strings.IndexByte(emphasisChars, c) >= 0 }
func isSpaceByte(c byte) bool    { return c == ' ' || c == '\t' }
func isAlnumByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

var secReviewPass = regexp.MustCompile(`(?i)^[ \t]*Security-Review:[ \t]*pass[ \t\r]*$`)
var secReviewFail = regexp.MustCompile(`(?i)^[ \t]*Security-Review:[ \t]*fail[ \t\r]*$`)
