package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// maxIssueBodyLen is the forge's issue-body limit. A longer body makes
// `gh issue create` fail ("Body is too long (maximum is 65536 characters)"),
// which aborts the verify-gate-open batch at that brief and leaves every brief
// after it without a card (#4091). The title has its own cap (issuetitle.go).
const maxIssueBodyLen = 65536

// issueBodyCap is the length an emitted body is held to: the forge limit less
// a margin for what the workflow adds on the way (a trailing newline from
// `jq -r`) and for any difference in how the forge counts.
const issueBodyCap = maxIssueBodyLen - 5536

// issueBodyCutMark opens every cut notice. Tests and readers look for it.
const issueBodyCutMark = "**Cut here:**"

// forgeBodyLen counts s the way the forge could count it at its most
// expensive: in UTF-16 code units (an astral character such as an emoji is
// two), which is never fewer than the characters (code points) the limit
// names, and with each LF counted twice in case the body is stored with CRLF
// line endings. Never bytes: a byte count over-counts accented text and would
// cut bodies that fit, and a rune count alone under-counts astral characters.
func forgeBodyLen(s string) int {
	n := 0
	for _, r := range s {
		n += forgeRuneLen(r)
	}
	return n
}

func forgeRuneLen(r rune) int {
	if r == '\n' || r >= 0x10000 {
		return 2
	}
	return 1
}

// capIssueBody joins head, sections and tail into one issue body no longer
// than issueBodyCap (as forgeBodyLen counts). The head (the idempotency marker
// and the card header, which dedupe and the reader depend on) and the tail
// (the closing checklist) are never cut, with one exception: when they alone
// overrun the cap, the body itself is cut and only its first line, the marker,
// is kept whole. When the whole does not fit, the
// largest sections are cut first and to the same length (a small section such
// as Trade-offs stays whole while a huge Evidence table is cut), each at a line
// boundary where one exists; any code fence or HTML comment the cut leaves
// open is closed, and a notice states the cut and links the full text.
//
// link renders the full-text link for the notice. It is called only when a
// cut is needed, so a body that fits is returned byte-identical to the plain
// concatenation and costs no lookup.
func capIssueBody(head string, sections []string, tail string, link func() string) string {
	whole := head + strings.Join(sections, "") + tail
	if forgeBodyLen(whole) <= issueBodyCap {
		return whole
	}
	full := link()
	sizes := make([]int, len(sections))
	for i, s := range sections {
		sizes[i] = forgeBodyLen(s)
	}
	// Reserve the widest notice (and any fence/comment closer) for every
	// section that may be cut, so the cut sections plus their notices fit.
	reserve := forgeBodyLen(cutNotice("this section", full, issueBodyCap, issueBodyCap*100)) + cutCloseReserve
	budget := issueBodyCap - forgeBodyLen(head) - forgeBodyLen(tail) - len(sections)*reserve
	level := cutLevel(sizes, max(budget, 0))

	render := func(level int) string {
		var b strings.Builder
		b.WriteString(head)
		for i, s := range sections {
			if sizes[i] <= level {
				b.WriteString(s)
				continue
			}
			kept := cutAtLine(s, level)
			b.WriteString(kept)
			b.WriteString(closeOpenConstructs(kept))
			b.WriteString(cutNotice("this section", full, forgeBodyLen(kept), sizes[i]))
		}
		b.WriteString(tail)
		return b.String()
	}
	out := render(level)
	// A closer longer than cutCloseReserve (a fence run of more than a few dozen
	// characters) can overrun the cap. Lower the level by the overrun and cut
	// the sections again, so the tail survives whenever head, tail and the
	// notices fit. level strictly falls, so this ends; at level 0 every section
	// is already empty.
	for forgeBodyLen(out) > issueBodyCap && level > 0 {
		level = max(level-(forgeBodyLen(out)-issueBodyCap), 0)
		out = render(level)
	}
	if total := forgeBodyLen(whole); forgeBodyLen(out) > issueBodyCap {
		// Backstop: with every section cut to nothing the head, the tail and
		// the notices alone overrun the cap (an enormous title or rationale).
		// Cut the body itself, tail included. The marker line is a floor the
		// cut never goes above: dedupe reads it, and a card without it is filed
		// again on every run. Above the floor a closer longer than
		// cutCloseReserve shrinks the kept part by the overrun each time round.
		floor := forgeBodyLen(out[:strings.IndexByte(out, '\n')+1])
		notice := cutNotice("this issue body", full, issueBodyCap, total)
		limit := max(issueBodyCap-forgeBodyLen(notice)-cutCloseReserve, floor)
		for {
			kept := cutAtLine(out, limit)
			res := kept + closeOpenConstructs(kept) + cutNotice("this issue body", full, forgeBodyLen(kept), total)
			over := forgeBodyLen(res) - issueBodyCap
			if over <= 0 {
				out = res
				break
			}
			if limit <= floor {
				// Only the marker line is left; a closer for it would be
				// the overrun, so drop it.
				out = kept + cutNotice("this issue body", full, forgeBodyLen(kept), total)
				break
			}
			limit = max(limit-over, floor)
		}
	}
	return out
}

// cutCloseReserve covers what closeOpenConstructs usually appends: a fence
// run of up to a few dozen characters, "\n-->" and a couple of end tags.
// A longer closer is caught by the level loop and the backstop loop in
// capIssueBody, not by this.
const cutCloseReserve = 64

// cutNotice renders the statement placed where content was cut.
func cutNotice(what, full string, kept, total int) string {
	return fmt.Sprintf("\n\n> %s %s was cut to %d of its %d characters to keep this issue under the forge's %d-character body limit. The full text is in the brief at this commit: %s\n\n",
		issueBodyCutMark, what, kept, total, maxIssueBodyLen, full)
}

// cutLevel returns the largest length L such that cutting every section longer
// than L down to L keeps the sum of sizes within budget (water-filling): the
// sections that fit at that level are kept whole.
func cutLevel(sizes []int, budget int) int {
	sorted := append([]int(nil), sizes...)
	sort.Ints(sorted)
	remaining := budget
	for i, s := range sorted {
		share := remaining / (len(sorted) - i)
		if s > share {
			return share
		}
		remaining -= s
	}
	return remaining + 1 // everything fits
}

// cutAtLine returns the longest prefix of s within limit (as forgeBodyLen
// counts) that ends at a line boundary, or, when even the first line is over
// the limit, the longest prefix that ends on a whole character.
func cutAtLine(s string, limit int) string {
	n, lastLine := 0, -1
	for i, r := range s {
		w := forgeRuneLen(r)
		if n+w > limit {
			if lastLine > 0 {
				return s[:lastLine]
			}
			return s[:i]
		}
		n += w
		if r == '\n' {
			lastLine = i + 1
		}
	}
	return s
}

// blockTags are the HTML block tags a cut can leave open. details folds the
// rest of the card into a collapsed block; pre, script, style and textarea
// start a CommonMark type-1 raw HTML block, which runs to its end tag or to the
// end of the document. Listed innermost first, the order their closers follow.
var blockTags = []string{"pre", "script", "style", "textarea", "details"}

var (
	blockOpenRe  = regexp.MustCompile(`(?i)<(pre|script|style|textarea|details)(?:[\s/>]|$)`)
	blockCloseRe = regexp.MustCompile(`(?i)</(pre|script|style|textarea|details)\s*>`)
)

// maxTagClosers bounds the end tags closeOpenConstructs appends. A tag costs
// about 8 characters in the kept text and its closer 10 to 12, so an unbounded
// closer could outgrow what it closes; past this many the rest stay open.
const maxTagClosers = 16

// closeOpenConstructs returns what must follow kept so the notice after it
// renders as a notice: a closing fence when kept ends inside a fenced code
// block, a comment closer when it ends inside an HTML comment, and an end tag
// for each details, pre, script, style or textarea block left open, at most
// maxTagClosers of them (any of these would swallow the notice and the closing
// checklist after it, or fold them into a collapsed or raw block).
//
// The comment closer is "\n-->", never " -->": the verify-gate marker pattern
// (verifyMarkerRe) needs a space before its closing dashes, so a cut that falls
// inside a marker-shaped comment cannot be completed into a second marker that
// names another brief. The leading newline puts the closer on a line of its
// own, so the character before the dashes is never a space.
func closeOpenConstructs(kept string) string {
	var close string
	open := ""
	balance := map[string]int{}
	for _, line := range strings.Split(kept, "\n") {
		c, run, rest, ok := fenceRun(line)
		switch {
		case !ok:
			if open == "" {
				for _, m := range blockOpenRe.FindAllStringSubmatch(line, -1) {
					balance[strings.ToLower(m[1])]++
				}
				for _, m := range blockCloseRe.FindAllStringSubmatch(line, -1) {
					balance[strings.ToLower(m[1])]--
				}
			}
		case open == "" && fenceLineValid(line):
			open = strings.Repeat(string(c), run)
		case open != "" && c == open[0] && run >= len(open) && strings.Trim(rest, " \t\r") == "":
			open = ""
		}
	}
	if open != "" {
		close += "\n" + open + "\n"
	}
	if strings.LastIndex(kept, "<!--") > strings.LastIndex(kept, "-->") {
		close += "\n-->"
	}
	left := maxTagClosers
	for _, tag := range blockTags {
		for ; balance[tag] > 0 && left > 0; balance[tag]-- {
			close += "\n</" + tag + ">"
			left--
		}
	}
	return close
}

// fenceRun splits a line indented at most three spaces into its leading run of
// backticks or tildes (the character and the run length) and what follows it.
func fenceRun(line string) (c byte, run int, rest string, ok bool) {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	if i > 3 || i >= len(line) || line[i] != '`' && line[i] != '~' {
		return 0, 0, "", false
	}
	c = line[i]
	for i+run < len(line) && line[i+run] == c {
		run++
	}
	if run < 3 {
		return 0, 0, "", false
	}
	return c, run, line[i+run:], true
}
