package main

import (
	"regexp"
	"strconv"
	"strings"
)

// The read scope of the held/could-not-check scan (#1894).
//
// unroutedHeldLine used to read every line of a brief's whole Evidence
// section. Evidence accumulates: each verifier run appends its own `### `
// entry, and a run's table carries Command and Expect cells beside its
// result. So three kinds of text that are not a live row disposition kept
// refusing a clean pass:
//
//   - a row HELD/could-not-check in an EARLIER entry that a later run has
//     re-run and recorded with a strict PASS;
//   - an Expect (or Command) cell that names could-not-check as the
//     behaviour under test ("unreadable diff → could-not-check exit 6");
//   - prose beside the table that states what an exit code means
//     ("could-not-check→exit 6").
//
// heldScanScope decides what the scan reads. Every rule narrows only where
// the text is positively classified, and falls back to the old whole-text
// read where it is not:
//
//  1. Result cells. In an entry that holds an Evidence results table — a
//     header naming a row key ("#", "Row", "Command", exact names) and at
//     least one column that is not a key, Expect, Date or Runner column
//     (heldColumn) — each data row of that table is read in its result
//     cells only. A row whose cell count differs from its header's (an
//     unescaped "|" in a command) cannot be aligned and is read whole. A
//     table of any other shape is read whole.
//
//  2. Exit-code prose. In an entry that holds such a results table, a line
//     outside every table is still read, with one exception: an occurrence
//     written as an exit-code mapping — "could-not-check → exit 6",
//     "could-not-check = exit 6", "exit 6 (could-not-check)" — on a line
//     that names no row (heldExitMapping). It says what an exit code means,
//     not how a row came out: that is recorded in the row's own result cell,
//     which rule 1 reads. A mapping line that names a row ("row 2
//     could-not-check → exit 6") is read. In an entry with no results table
//     there is no result cell to fall back on, so every occurrence in its
//     prose is read. Any other prose occurrence — a verdict-line summary
//     "row 4 HELD", a note — is read.
//
//  3. Supersession. The Evidence body is split into entries at `### `
//     headings (level exactly three, column 0) that render as live headings
//     (heldRender); text before the first such heading is the first entry.
//     A held occurrence in an earlier entry is dropped in one of two ways,
//     and otherwise every entry is read. Both need the LAST entry to carry a
//     live strict **VERIFY: PASS** marker (heldEntryPass): at column 0,
//     opening its own paragraph, rendered live, and with no VERIFY token
//     after it anywhere in the entry, quoted or not.
//     (a) Row by row: every held occurrence in the earlier entries sits in
//     the result cell of an aligned row of a results table (rule 1) with a
//     non-empty row key (heldRowKey), and the last entry covers that key
//     (below).
//     (b) Whole entries (#3841): the last entry covers EVERY row key of
//     EVERY earlier entry's results tables — rows that held and rows that
//     did not — there is at least one such key, every data row of those
//     tables is aligned and keyed, no earlier entry records a row in a
//     table the scan cannot key, and no HTML block leaves the page inside a
//     tag, a quoted attribute value or a comment (heldHTMLUnclosed, which
//     lets that state cross a line end only inside a block the page is sure
//     to run on), which swallows what follows on the page. No line in a
//     container (a blockquote or list-item marker, or any leading
//     whitespace) holds a tag start or a fence (heldContainerDoubt): where
//     such a block ends on the page is more than the scan models, so this
//     over-refuses by design. Every element an entry opens outside fenced
//     code (void elements aside) is closed later in that entry, in nesting
//     order, by a close tag the page surely applies to it, and the last
//     entry holds none open across a line end (heldElementDoubt): an element
//     left open nests the later entries inside it, which may drop, collapse
//     or flatten them on the page; this too over-refuses. Any carriage return left after the brief
//     reader's single CRLF-to-LF rewrite ends a line on the page but not in
//     the scan's split, so from such a line on nothing is live (heldRender)
//     and no entry is superseded whole. Then the earlier entries are
//     superseded as wholes: their prose, an entry-level verdict such as VERIFY:
//     BLOCKED, a held or could-not-check note that names no row or names a
//     row absent from every keyed table, and a hold in fenced code or wholly
//     inside an HTML comment are all dropped. A hold on any other line of an
//     HTML block (heldHTMLLines) is still read. A table the scan cannot key
//     is any line of an earlier entry, outside fenced code (an HTML comment
//     is no exemption: the page may not read it as one), that carries an
//     HTML table tag (heldHTMLTableRe; a keyed row or its header included,
//     since a cell renders one live) or, other than a keyed results row or
//     its header, an unescaped "|" outside inline code — a table
//     of another shape, with no separator row, with no leading pipe, inside
//     a blockquote, or in HTML; a one-line paragraph that GFM can never read
//     as a table row (heldOneLineProse: column 0, no container marker, not
//     led by "|", blank lines on both sides) is prose, not such a line
//     (#4294) — or a non-blank line right after a results
//     table, which GFM may read as one more row of it. One earlier key the
//     last entry did not cover, one earlier row that cannot be aligned or
//     keyed, one such table line, or earlier entries with no keyed row at all
//     leave (a) as the only way, so a held occurrence anywhere else — prose,
//     a misaligned row, a table of another shape — leaves every entry read.
//     The last entry covers a key when it has live results tables
//     (heldCovered) with a row of the same key (heldRowKey: key-column name
//     and key text), and EVERY row of that key in the last entry is clean:
//     its Exit and Result-type columns (heldCoverColumn) hold at least one
//     cell that reads, as a whole, as a recognised clean outcome (heldOutcome:
//     ok, PASS, exit 0 or the like, followed only by neutral details such as
//     a non-zero test count or a duration; a bare 0 only as a whole Exit
//     cell), and no other cell of them or of any unrecognised column holds
//     visible text. A column that names an expectation and nothing else
//     (Expected exit, Pass criteria; heldExpectColumnRe, anchored) is never
//     read for the outcome; a header that only contains the word (Verdict vs
//     Expect, Matches expected?, Meets criteria) is an unrecognised column
//     and must be empty. A placeholder (—, n/a), a carry-forward
//     (same as Run 1), a note after the outcome (PASS (no re-run)), a zero
//     count (0 checks run), markup or a format character, or any text outside
//     that closed grammar is not clean. A later run that re-ran nothing,
//     re-ran other rows, or wrote anything but a clean outcome for a row
//     replaces nothing; so does a last entry with a row it cannot align.
//     Supersession is never inferred inside one entry: a later table under
//     the same heading does not clear an earlier row in it. It is
//     POSITIONAL, not dated: the entry written last in the file is the
//     latest, whatever date its heading carries, because Evidence is
//     appended and a heading's date is free text the scan does not trust.
//     The held occurrences of the last entry itself are always read.
//     A key cell that is a Verify row number with one parenthetical annotation
//     ("3 (Verify 3, negative)") keys as the bare number (heldCanonKeys,
//     #4294) when every number in the annotation is that number, and only
//     when, across the whole Evidence, the bare number under that header never
//     appears with two different annotations and no entry records it in two
//     forms ("3" beside "3 (negative)", "3 (positive)" beside "3 (negative)"):
//     those may be sub-rows of one Verify row, so they keep their written keys
//     and a bare later "3" covers neither. Only the entries before the last
//     are shortened: the last entry covers an earlier row under the bare
//     number or under the earlier row's own key exactly as written ("3
//     (negative)" re-ran as "3 (negative)"), never under an annotation the
//     earlier row did not carry. A lettered key ("3a"), nested parentheses,
//     text after the annotation or a number in it that names another row ("3
//     (Verify 4)", a date) is never shortened. Stated residual: an earlier
//     annotated row and a later bare number are one row, so a later bare "3"
//     that ran only one sub-case covers an earlier lone "3 (negative)"; the
//     format records no sub-case on a bare key that could tell them apart.
//
//  4. Observed output (#4294). An occurrence of "could-not-check" (never HELD)
//     in a Result-type cell (heldColResult) of an aligned, keyed row is what
//     the verified product printed, not the row's disposition, only when the
//     cell matches a closed grammar, every part of which must hold: the row's
//     table has an Exit column and every Exit cell of the row reads clean
//     (heldOutcome); the cell opens on PASS, passed or ok, optionally with a
//     positive or negative case label (heldObserveLeadRe); the words between
//     that lead and the marker, as written, are one named subject that is the
//     product under test (heldObserveSubject: "the planted brief", "the
//     product"), then one product-output verb, then only words of a small
//     object vocabulary (heldObserveObject: "its named diagnostic, a lint
//     PROBLEM and a"), so a cell with no subject, a subject that names the
//     row's own instrument (its command, tool, cli, binary, output or stdout:
//     the Command cell is the verify script), the verifier, its runner or
//     fixture, a case, a half or rows, a pronoun or a digit, or a second verb
//     after the first, leaves the row read; an article or possessive
//     comes right before the marker and, in the clause that holds it (since
//     the last ; . ! ? or dash), a verb of product output governs it
//     (heldObserveVerbRe: "prints", "yields", "lists"; never "reports" or
//     "shows", which a verifier also uses for its own instrument); the text
//     after the marker is a noun it qualifies (heldObserveHead: "mission
//     section", "banner") followed only by words of a small neutral vocabulary
//     (heldObserveTail; never "rows", "all" or "both", which count or name
//     Verify rows or cases), so any other word, a digit or a hold reason leaves
//     the row read; the line has no struck span; and no read cell of the row
//     holds a hold or fail word (heldObserveHoldRe, a second check behind the
//     grammar; a HELD there is read as its own occurrence). Wording the
//     grammar does not recognise is read as a hold.
//
//  5. Sourced definitions (#4536). A column-zero RISK-VALUE: DERIVED
//     record with a source path and line number may declare a quoted state
//     literal or explain a normative state relation (heldRiskDefinition).
//     Only the occurrence inside the recognised phrase is excluded, never
//     the entire line, an unsourced note or a results-table cell. Unrecognised
//     wording and any independent hold remain read. The source locator is
//     syntax, not proof that the derivation or its claimed value is correct.
//
//  6. Recounted prose (#4721). A line outside every table — prose, with no pipe
//     and no HTML table tag — may name a state word without recording a hold.
//     Exactly three shapes are excused (heldRecounted), each on the lower-case
//     occurrence alone: a zero tally closing its clause ("observed failed
//     obligations 0; could-not-check 0."), a verb that rounds the state to a
//     pass ("rounds could-not-check to pass"), and a past flip hold that
//     cites its tracker ("its board flip was held on #4536"). Prose stays
//     read for everything else, so a labelled disposition, a row, a reason, a
//     capital marker or a zero followed by a noun still holds. Table lines
//     are read under rules 1 to 5, never excused by this rule.
//
// The hygiene (fences, blockquotes, struck and inline code), the routing rule
// and the negation rule in unroutedHeldLine apply unchanged to what is read.
type heldLineRead struct {
	skip  bool     // the line is outside the scan's read scope
	prose bool     // a line outside every table, in an entry with a results table (rule 2)
	blank [][2]int // byte ranges of the line (cell contents) the scan must not read
	key   string   // the row key of an aligned results-table data row; "" for any other line
	whole bool     // a line of an entry that whole-entry supersession (rule 3b) may drop: any line outside a non-comment HTML block
	// observe are the byte ranges of the Result-type cells of an aligned
	// results row whose Exit cells all read clean (rule 4); nil otherwise.
	observe [][2]int
}

// heldScope is heldScanScope's decision: what to read of each line, and
// which earlier-entry rows the last entry re-ran (rule 3).
type heldScope struct {
	lines   []heldLineRead
	from    int             // the first line of the superseding last entry, or -1
	covered map[string]bool // the row keys the superseding entry re-ran
	whole   bool            // the superseding entry re-ran every row key of every earlier entry (rule 3b)
}

// supersedes reports whether the held occurrences found on the given lines
// (the scan's hits, in any order) that lie before the last entry are all
// replaced by it: every one is a keyed results-table row the last entry
// re-ran (rule 3a), or the last entry re-ran every earlier row and the hit is
// not in an HTML block other than a comment (rule 3b). A single hit it cannot classify
// keeps every hit read.
func (s heldScope) supersedes(hits []int) bool {
	if s.from < 0 {
		return false
	}
	for _, i := range hits {
		if i >= s.from {
			continue
		}
		read := s.lines[i]
		if read.key != "" && s.covered[read.key] {
			continue
		}
		if s.whole && read.whole {
			continue
		}
		return false
	}
	return true
}

// heldWholeCovered reports whether the last entry re-ran every earlier entry
// whole (rule 3b): every earlier entry's results-table rows are all aligned
// and keyed and it records no row in a table the scan cannot key, there is at
// least one such key, and covered holds every one.
func heldWholeCovered(keys [][]string, aligned []bool, covered map[string]bool) bool {
	n := 0
	for e := range keys {
		if !aligned[e] {
			return false
		}
		for _, k := range keys[e] {
			if !covered[k] {
				return false
			}
			n++
		}
	}
	return n > 0
}

// heldEntryHeading reports whether line opens an Evidence entry: an ATX
// heading of level exactly three at column 0. An indented heading is not an
// entry (it may sit in a list item or be indented code).
func heldEntryHeading(line string) bool {
	t := strings.TrimRight(line, "\r")
	return t == "###" || strings.HasPrefix(t, "### ") || strings.HasPrefix(t, "###\t")
}

// heldFenceToggle is the fence test unroutedHeldLine applies, so the scope
// and the scan agree on which lines are inside fenced code.
func heldFenceToggle(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// heldTableLine reports whether line is a table line: it starts with "|".
func heldTableLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

// heldHTMLTableRe is an HTML table tag anywhere on a line.
var heldHTMLTableRe = regexp.MustCompile(`(?i)</?(?:table|thead|tbody|tfoot|tr|td|th)\b`)

// heldHTMLUnclosed reports whether an HTML block of lines ends with the
// page's HTML tokenizer still inside a tag, a quoted attribute value or a
// comment. The page then swallows what follows, a later entry and its PASS
// included, while the scan reads it live, so no entry is superseded whole.
// The state may be open across a line end only inside a block the page is
// sure to run on past that line: one whose opener sits on its own at the top
// level and either names an end marker not yet met (heldHTMLOpenEnd) or a
// tag that interrupts a paragraph (heldHTMLKind6), up to that marker or the
// next blank line. Anywhere else (a block that starts as a paragraph or as a
// tag that cannot interrupt one, a block whose opener meets its end on its
// own line, a block the page may run on past a blank line to an end marker
// opened inside it) the state is checked at the end of every line. Lines in
// a container are left to heldContainerDoubt.
func heldHTMLUnclosed(lines []string, inFence []bool) bool {
	st, in, end, sure := 0, false, "", false
	for i, l := range lines {
		closed := false
		switch {
		case !in:
			h, e, c := heldHTMLOpenEnd(l)
			if inFence[i] || !h {
				continue
			}
			in, end, closed = true, e, c
			sure = e != "" || heldHTMLKind6(l) // a block closed on this line is checked here
		case end != "":
		case heldHTMLOpenEnds(l):
			_, end, _ = heldHTMLOpenEnd(l)
			sure = false // the page may run this block on to that end marker
		case !sure && heldHTMLKind6(l):
			sure = true // a block the page is sure of from here to the blank line
		}
		st = heldTagScan(st, l)
		switch {
		case closed:
			in = false
		case end != "":
			in = !strings.Contains(strings.ToLower(l), end)
		default:
			in = !isBlankLine(l)
		}
		if (!in || !sure) && st != 0 {
			return true
		}
	}
	return st != 0
}

// heldHTMLOpenEnds reports whether line opens an HTML block that runs on to
// an end marker it does not itself hold.
func heldHTMLOpenEnds(line string) bool {
	h, e, _ := heldHTMLOpenEnd(line)
	return h && e != ""
}

// heldKind6Tags are the tag names that open an HTML block able to interrupt
// a paragraph and running to the next blank line: the names CommonMark and
// GFM both list (CommonMark's source and search are left out, GFM lacks them).
var heldKind6Tags = map[string]bool{}

func init() {
	for _, t := range strings.Fields(`address article aside base basefont blockquote
		body caption center col colgroup dd details dialog dir div dl dt fieldset
		figcaption figure footer form frame frameset h1 h2 h3 h4 h5 h6 head header
		hr html iframe legend li link main menu menuitem nav noframes ol optgroup
		option p param section summary table tbody td tfoot th thead title tr
		track ul`) {
		heldKind6Tags[t] = true
	}
}

// heldHTMLKind6 reports whether line, at column 0, opens or closes one of
// heldKind6Tags, the name ended by a space, a tab, ">", "/>" or the line end.
func heldHTMLKind6(line string) bool {
	t, ok := strings.CutPrefix(line, "<")
	if !ok {
		return false
	}
	t = strings.TrimPrefix(t, "/")
	n := 0
	for n < len(t) && (isASCIILetter(t[n]) || t[n] >= '0' && t[n] <= '9') {
		n++
	}
	if !heldKind6Tags[strings.ToLower(t[:n])] {
		return false
	}
	r := t[n:]
	return r == "" || r[0] == ' ' || r[0] == '\t' || r[0] == '>' || strings.HasPrefix(r, "/>")
}

// heldTagStartRe is where an HTML tag, comment, declaration or processing
// instruction can start.
var heldTagStartRe = regexp.MustCompile(`<[A-Za-z!?/]`)

// heldContainerDoubt reports whether any line that carries a blockquote or
// list-item marker or leading whitespace (heldContainerStrip) has a fence
// after them or, outside top-level fenced code, a tag start. Where a container,
// a lazy continuation or an indented block ends on the page is more than the
// scan models, so an HTML block or fence there may end before the scan's
// read of it and leave what follows swallowed or live: whole supersession is
// off. This over-refuses (an inline tag in a list item, an indented code
// sample that holds a tag) by design.
func heldContainerDoubt(lines []string, inFence []bool) bool {
	for i, l := range lines {
		t := heldContainerStrip(l)
		if t == l {
			continue
		}
		if heldFenceToggle(t) || !inFence[i] && heldTagStartRe.MatchString(l) {
			return true
		}
	}
	return false
}

// heldContainerStrip drops the blockquote and list-item markers that open
// line, so an HTML block inside them is seen.
func heldContainerStrip(line string) string {
	for {
		t := strings.TrimLeft(line, " \t")
		switch {
		case strings.HasPrefix(t, ">"):
			line = t[1:]
			continue
		case len(t) > 1 && strings.IndexByte("-*+", t[0]) >= 0 && (t[1] == ' ' || t[1] == '\t'):
			line = t[2:]
			continue
		}
		n := 0
		for n < len(t) && n < 9 && t[n] >= '0' && t[n] <= '9' {
			n++
		}
		if n > 0 && n+1 < len(t) && (t[n] == '.' || t[n] == ')') && (t[n+1] == ' ' || t[n+1] == '\t') {
			line = t[n+2:]
			continue
		}
		return t
	}
}

// heldVoidTags are the elements the page never holds open: a start tag with
// one of these names has no content and no end tag.
var heldVoidTags = map[string]bool{}

// heldRawTags are the elements whose content the page does not read as
// ordinary HTML (raw text, escapable raw text, plaintext) or reads in a
// foreign namespace: a close tag inside them is not one, so any opener of
// one of these names refuses whole supersession, closed or not.
var heldRawTags = map[string]bool{}

func init() {
	for _, t := range strings.Fields(`area base br col embed hr img input link meta
		source track wbr`) {
		heldVoidTags[t] = true
	}
	for _, t := range strings.Fields(`script style textarea title xmp iframe noembed
		noframes noscript plaintext svg math`) {
		heldRawTags[t] = true
	}
}

// heldElement is an element an entry has opened and not yet closed: its
// name and the run (the lines between blank lines) its opener sits in.
type heldElement struct {
	name string
	run  int
	raw  bool
}

// heldElementDoubt reports whether an entry leaves an element open on the
// page (#3841). An earlier entry's element that is not closed in that entry
// nests the entries after it, the last one and its PASS included, inside
// that element, which may drop, collapse or flatten them while the scan
// reads them live; so whole supersession is off unless, in every entry,
// every element opened outside top-level fenced code is closed by a matching
// close tag later in the same entry, in nesting order, and the last entry
// holds no element open across a line end. Void elements are exempt. An
// opener is counted wherever it appears (in a code span, a comment or an
// attribute value too). A close tag counts only where the page surely reads
// it as one: complete on its line, not backslash-escaped, outside a tag and a
// comment (heldTagScan), in a run with no code span, link or image syntax,
// math or table, and either in the opener's own run or, when both runs are
// HTML blocks that run to a blank line (heldRawRun), in any later run; a
// close tag in a paragraph cannot end an element opened outside it. Any
// opener of heldRawTags, any close tag that does not match the innermost
// open element, and any other doubt refuses. This over-refuses by design.
func heldElementDoubt(lines []string, inFence []bool, starts []int) bool {
	st := 0
	for e, lo := range starts {
		hi := len(lines)
		if e+1 < len(starts) {
			hi = starts[e+1]
		}
		lastEntry := e == len(starts)-1
		var stack []heldElement
		run, raw, noClose := -1, false, false
		for i := lo; i < hi; i++ {
			l := lines[i]
			if isBlankLine(l) {
				run = -1
				continue
			}
			if run < 0 || i == lo {
				run = i
				raw = !inFence[i] && heldRawRun(l)
				noClose = heldNoCloseRun(lines[i:hi])
			}
			if inFence[i] {
				continue
			}
			p0, s0 := 0, st
			for p := 0; p+1 < len(l); p++ {
				if l[p] != '<' {
					continue
				}
				if isASCIILetter(l[p+1]) {
					n := heldElementName(l[p+1:])
					switch {
					case heldVoidTags[n]:
					case heldRawTags[n]:
						return true
					default:
						stack = append(stack, heldElement{n, run, raw})
					}
					continue
				}
				n, ok := heldCloseTag(l[p:])
				if !ok || heldVoidTags[n] || noClose || p > 0 && l[p-1] == '\\' {
					continue
				}
				s0, p0 = heldTagScan(s0, l[p0:p]), p
				if s0 != 0 {
					continue
				}
				if len(stack) == 0 {
					return true // a stray close tag
				}
				top := stack[len(stack)-1]
				if top.run != run && !(top.raw && raw) {
					continue // a close the page may not apply to that element
				}
				if top.name != n {
					return true
				}
				stack = stack[:len(stack)-1]
			}
			st = heldTagScan(st, l)
			if lastEntry && len(stack) > 0 {
				return true
			}
		}
		if len(stack) > 0 {
			return true
		}
	}
	return false
}

// heldElementCheck is heldElementDoubt as heldScanScope calls it. It is a
// variable only so the tests can turn it off and show that every earlier
// rule still refuses on its own: the rules are layers, not one check.
var heldElementCheck = heldElementDoubt

// heldElementName returns the tag name the page's HTML tokenizer reads at
// the start of s (after "<"), ASCII-lowercased: up to whitespace, "/" or ">".
func heldElementName(s string) string {
	b := []byte{}
	for i := 0; i < len(s) && strings.IndexByte(" \t\n\f\r/>", s[i]) < 0; i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}

// heldCloseTag reports whether s opens with a close tag CommonMark reads as
// one, complete on the line: "</", a tag name (an ASCII letter, then letters,
// digits and hyphens), optional spaces or tabs, then ">". It returns the
// name, ASCII-lowercased.
func heldCloseTag(s string) (string, bool) {
	if len(s) < 4 || s[1] != '/' || !isASCIILetter(s[2]) {
		return "", false
	}
	n := 3
	for n < len(s) && (isASCIILetter(s[n]) || s[n] >= '0' && s[n] <= '9' || s[n] == '-') {
		n++
	}
	r := strings.TrimLeft(s[n:], " \t")
	if !strings.HasPrefix(r, ">") {
		return "", false
	}
	return heldElementName(s[2:n]), true
}

// heldRawRun reports whether line, the first of a run, opens an HTML block
// that runs to the next blank line, so the page emits the whole run as raw
// HTML at the top level: a type 6 block (heldHTMLKind6), or a type 7 block
// read strictly — one open tag with plain attributes, or one close tag,
// alone on the line at column 0, its name not one that opens a type 1 block.
func heldRawRun(line string) bool {
	if heldHTMLKind6(line) {
		return true
	}
	t := strings.TrimRight(line, " \t")
	if n, ok := heldCloseTag(t); ok {
		return len(t) == strings.IndexByte(t, '>')+1 && !heldType1Name(n)
	}
	if len(t) < 3 || t[0] != '<' || !isASCIILetter(t[1]) || t[len(t)-1] != '>' {
		return false
	}
	n := 2
	for n < len(t) && (isASCIILetter(t[n]) || t[n] >= '0' && t[n] <= '9' || t[n] == '-') {
		n++
	}
	if heldType1Name(heldElementName(t[1:n])) {
		return false
	}
	return heldPlainAttrsRe.MatchString(t[n : len(t)-1])
}

// heldPlainAttrsRe is a run of attributes in the narrow shape heldRawRun
// accepts (a name, then optionally "=" and a value that is double-quoted
// without a double quote, or a plain word), then an optional "/".
var heldPlainAttrsRe = regexp.MustCompile(`^(?:[ \t]+[A-Za-z_:][A-Za-z0-9_.:-]*(?:="[^"<>]*"|=[A-Za-z0-9_.:/-]+)?)*[ \t]*/?$`)

// heldType1Name reports whether name opens a type 1 HTML block, one that
// runs to its own end marker rather than to a blank line.
func heldType1Name(name string) bool {
	return name == "pre" || name == "script" || name == "style" || name == "textarea"
}

// heldNoCloseRun reports whether the run that starts at lines[0] (up to the
// next blank line) holds syntax that may carry a close tag the page does not
// apply as one: a code span or fence, link or image syntax, a link
// reference or footnote, math, or a table.
func heldNoCloseRun(lines []string) bool {
	for _, l := range lines {
		if isBlankLine(l) {
			return false
		}
		for _, m := range []string{"`", "](", "]:", "][", "![", "[^", "$", "|", "~"} {
			if strings.Contains(l, m) {
				return true
			}
		}
	}
	return false
}

// heldTagScan advances the page's HTML tokenizer over line from state st:
// 0 text, 1 in a tag, 2 and 3 in a double- or single-quoted attribute value,
// 4 in a comment, 5 in a bogus comment ("<?", "<!x", "</ "). It returns the
// state at the line's end; a line break ends none of them.
func heldTagScan(st int, line string) int {
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch st {
		case 0:
			if c != '<' || i+1 >= len(line) {
				continue
			}
			switch n := line[i+1]; {
			case isASCIILetter(n) || n == '/' && i+2 < len(line) && isASCIILetter(line[i+2]):
				st = 1
			case strings.HasPrefix(line[i+1:], "!--"):
				st, i = 4, i+3
				if r := line[i+1:]; strings.HasPrefix(r, ">") || strings.HasPrefix(r, "->") {
					st = 0 // "<!-->" and "<!--->" close at once
				}
			case n == '!' || n == '?' || n == '/':
				st = 5
			}
		case 1:
			switch c {
			case '"':
				st = 2
			case '\'':
				st = 3
			case '>':
				st = 0
			}
		case 2:
			if c == '"' {
				st = 1
			}
		case 3:
			if c == '\'' {
				st = 1
			}
		case 4:
			if j, n := heldCommentEnd(line[i:]); j >= 0 {
				st, i = 0, i+j+n-1
			} else {
				i = len(line)
			}
		case 5:
			if c == '>' {
				st = 0
			}
		}
	}
	return st
}

// heldPipeOutsideCode reports whether line holds a "|" that is neither
// backslash-escaped nor inside a code span. A backtick run with no closing run
// of the same length on the line is literal text, as CommonMark reads it.
func heldPipeOutsideCode(line string) bool {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case '|':
			return true
		case '`':
			n := 1
			for i+n < len(line) && line[i+n] == '`' {
				n++
			}
			if j := heldCloseRun(line, i+n, n); j >= 0 {
				i = j + n - 1
			} else {
				i += n - 1
			}
		}
	}
	return false
}

// heldOneLineProse reports whether line i is a paragraph of one line that GFM
// can never read as part of a table (#4294): outside fenced code, at column 0
// with no container marker (heldContainerStrip leaves it whole), not led by
// "|", holding no carriage return, and with a line holding only spaces or
// tabs straight before and after it. A GFM table needs a header row and a
// delimiter row on adjacent lines and ends at a blank line, so such a line is
// neither a header nor a delimiter nor a body row; its "|" is prose. A line
// with no line before or after it in the Evidence is not exempt.
func heldOneLineProse(lines []string, inFence []bool, i int) bool {
	if i == 0 || i+1 >= len(lines) || inFence[i] {
		return false
	}
	l := lines[i]
	if l == "" || strings.ContainsRune(l, '\r') || l[0] == '|' || heldContainerStrip(l) != l {
		return false
	}
	return strings.Trim(lines[i-1], " \t") == "" && strings.Trim(lines[i+1], " \t") == ""
}

// heldCloseRun returns where a backtick run of exactly n starts in line at or
// after from, or -1.
func heldCloseRun(line string, from, n int) int {
	for i := from; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		m := 1
		for i+m < len(line) && line[i+m] == '`' {
			m++
		}
		if m == n {
			return i
		}
		i += m
	}
	return -1
}

// heldCommentEnd returns the index and length of the first comment end in s:
// "-->", or "--!>", at which the page parser also ends a comment. It returns
// -1 when s holds neither. Searched from the opener's first byte, it also
// closes "<!--!>", which the page parser does not: the stricter choice.
func heldCommentEnd(s string) (int, int) {
	j, n := strings.Index(s, "-->"), 3
	if k := strings.Index(s, "--!>"); k >= 0 && (j < 0 || k < j) {
		j, n = k, 4
	}
	return j, n
}

// The kinds of line heldHTMLLines tells apart.
const (
	heldNotHTML     = iota
	heldHTMLComment // a line wholly inside an HTML comment
	heldHTMLOther   // any other line of an HTML block
)

// heldHTMLLines classifies each line by the HTML block it sits in (rule 3b):
// a comment line is one whose every byte lies inside the comment; a line of
// any other HTML block, or a comment's line with text after its end, is
// heldHTMLOther. A comment's end is the first "-->" or "--!>" counted from
// the opener's first byte (heldCommentEnd), so "<!-->" and "<!--->" close on
// their own line. The HTML block itself runs to the first line holding "-->",
// so the lines after a "--!>" end render live and are heldHTMLOther. An
// indented "<!--" (indented code on the page) is read here as a comment too;
// heldContainerDoubt turns whole supersession off for it. A block that ends
// on its own line is read as running to the next blank line unless it is a
// comment, and a block that ends at a blank line runs on to the end marker of
// any block a line in it opens: the stricter choices.
func heldHTMLLines(lines []string, inFence []bool) []int {
	kind := make([]int, len(lines))
	end, open, comment := "", false, false
	for i, l := range lines {
		lower := strings.ToLower(l)
		switch {
		case end != "":
			kind[i] = heldHTMLOther
			if comment {
				kind[i] = heldHTMLComment
				j, n := heldCommentEnd(lower)
				if j < 0 {
					continue
				}
				if strings.TrimSpace(lower[j+n:]) != "" {
					kind[i] = heldHTMLOther
				}
				comment = false // a "--!>" end leaves the block open, read
			}
			if strings.Contains(lower, end) {
				end = ""
			}
			continue
		case open:
			if isBlankLine(l) {
				open = false
				continue
			}
			kind[i] = heldHTMLOther
			if heldHTMLOpenEnds(l) {
				_, end = heldHTMLOpen(l)
				open = false // the page may run on to the end marker
			}
			continue
		case inFence[i]:
			continue
		}
		h, e := heldHTMLOpen(l)
		if !h {
			continue
		}
		kind[i] = heldHTMLOther
		if t := strings.TrimLeft(lower, " \t"); strings.HasPrefix(t, "<!--") {
			kind[i] = heldHTMLComment
			j, n := heldCommentEnd(t)
			if j < 0 {
				end, comment = "-->", true
				continue
			}
			if strings.TrimSpace(t[j+n:]) != "" {
				kind[i] = heldHTMLOther
			}
			if !strings.Contains(t, "-->") {
				end = "-->" // closed by "--!>": the rest of the block is read
			}
			continue
		}
		if e != "" {
			end = e
		} else {
			open = true
		}
	}
	return kind
}

// heldFence is an open CommonMark fence: its character and run length.
type heldFence struct {
	char byte
	n    int
}

// heldFenceCloses reports whether line closes the open fence f as CommonMark
// reads it: at most three spaces of indentation, a run of f's character at
// least as long as its opener, and nothing but spaces and tabs after.
func heldFenceCloses(f heldFence, line string) bool {
	t := strings.TrimRight(line, "\r")
	i := 0
	for i < len(t) && t[i] == ' ' {
		i++
	}
	if i > 3 {
		return false
	}
	n := 0
	for i+n < len(t) && t[i+n] == f.char {
		n++
	}
	return n >= f.n && strings.Trim(t[i+n:], " \t") == ""
}

// heldFenceOpens returns the fence line opens as CommonMark reads it, if any.
func heldFenceOpens(line string) (heldFence, bool) {
	t := strings.TrimRight(line, "\r")
	if !fenceLineValid(t) {
		return heldFence{}, false
	}
	t = strings.TrimLeft(t, " ")
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	return heldFence{t[0], n}, true
}

// heldHTMLOpen reports whether line starts an HTML block as CommonMark can
// read one (a "<" after leading whitespace, read here at ANY indentation —
// the stricter choice), and, for the block kinds that run across blank lines
// (htmlBlockEnds, a declaration), the end marker still to come: "" when the
// block ends at the next blank line or already ended on this line.
func heldHTMLOpen(line string) (html bool, end string) {
	html, end, _ = heldHTMLOpenEnd(line)
	return html, end
}

// heldHTMLOpenEnd is heldHTMLOpen, also reporting closed: the block is of a
// kind that runs to an end marker and the line already holds it, searched
// from the opener's first byte as the page does ("<!-->" ends at once). The
// page ends such a block on this line; heldHTMLOpen reads it as running on
// to the next blank line, the stricter choice for what it reads.
func heldHTMLOpenEnd(line string) (html bool, end string, closed bool) {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, "<") {
		return false, "", false
	}
	lower := strings.ToLower(t)
	for _, k := range htmlBlockEnds {
		if !strings.HasPrefix(lower, k.open) {
			continue
		}
		if k.end[1] == '/' { // type 1: the tag name must end here
			if n := len(k.open); n < len(t) && strings.IndexByte(" \t>\r", t[n]) < 0 {
				continue
			}
		}
		closed = strings.Contains(lower, k.end)
		if strings.Contains(lower[len(k.open):], k.end) {
			return true, "", closed
		}
		return true, k.end, closed
	}
	if len(t) > 2 && t[1] == '!' && isASCIILetter(t[2]) { // a declaration, type 4
		if strings.Contains(t[2:], ">") {
			return true, "", true
		}
		return true, ">", false
	}
	return true, "", false // types 6 and 7, or a paragraph that starts with "<": ends at a blank line
}

// heldRender classifies the lines of evidence for the structural reads rule 3
// makes (entry headings, the superseding marker, the covering tables): live[i]
// is true only where line i certainly renders as ordinary block text — outside
// fenced code and outside every HTML block. Every doubt is false:
//   - a fence is tracked twice, as CommonMark reads it (character, run length,
//     indentation, no info string on a closer) and as the scan reads it
//     (heldFenceToggle). From the first line where the two disagree, no line
//     is live, because the scan and the rendered page no longer agree on
//     what is code;
//   - a line that starts with "<" may open an HTML block: it, and every line
//     after it up to the next blank line, is not live; a block that runs
//     across blank lines (a comment, <pre>, …) is not live up to its end
//     marker, and a fence line inside any HTML block is a disagreement;
//   - a blank line is not live;
//   - a carriage return ends a line on the page that the scan's split on
//     "\n" does not see. The brief reader has already rewritten each CRLF to
//     LF exactly once, so any carriage return left, a trailing one included,
//     stood with no line feed after it in the file: from the first such line
//     no line is live;
//   - a block that ends at a blank line may hold a line that opens a block
//     running to an end marker (<pre>, a comment, …): the page may run on to
//     that marker past the blank line, so the lines up to it are not live.
//
// Paragraph-level contexts (a blockquote's lazy continuation, a code span or
// strike running across lines) are judged by the callers, which accept a
// marker only where it opens its own paragraph.
func heldRender(lines []string) []bool {
	live := make([]bool, len(lines))
	var (
		lax, doubt bool
		open       *heldFence
		htmlEnd    string
		html       bool
	)
	for i, l := range lines {
		toggle := heldFenceToggle(l)
		switch {
		case doubt:
			continue
		case strings.IndexByte(l, '\r') >= 0:
			doubt = true // the page ends a line here, the split does not
			continue
		case htmlEnd != "":
			if toggle {
				doubt = true
			} else if strings.Contains(strings.ToLower(l), htmlEnd) {
				htmlEnd = ""
			}
			continue
		case html:
			if isBlankLine(l) {
				html = false
			} else if toggle {
				doubt = true
			} else if heldHTMLOpenEnds(l) {
				_, htmlEnd = heldHTMLOpen(l)
				html = false // the page may run on to the end marker
			}
			continue
		}
		strict := false
		if open != nil {
			strict = heldFenceCloses(*open, l)
		} else if _, ok := heldFenceOpens(l); ok {
			strict = true
		}
		if toggle != strict {
			doubt = true
			continue
		}
		if toggle {
			if open != nil {
				open = nil
			} else {
				f, _ := heldFenceOpens(l)
				open = &f
			}
			lax = !lax
			continue
		}
		if lax || open != nil || isBlankLine(l) {
			continue
		}
		if h, end := heldHTMLOpen(l); h {
			html, htmlEnd = end == "", end
			continue
		}
		live[i] = true
	}
	return live
}

// heldVerdictTokenRe reads any VERIFY verdict token, including the ones
// verifyVerdictRe ignores (BLOCKED, PARTIAL, …).
var heldVerdictTokenRe = regexp.MustCompile(`VERIFY:[ \t]*[A-Za-z-]`)

// heldEntryPass reports whether the entry lines[lo:hi] (lo is its heading
// line) carries the live strict marker rule 3 requires: a line at column 0
// that begins with a strict **VERIFY: PASS…** marker (verifyVerdictBoldRe)
// holding none of "`", "<", "[", "\", "~" or "|", rendered live, and opening
// its own paragraph — the line before it is blank or the entry's heading, so
// it cannot continue a blockquote, a list item, a code span or a strike from
// the line above. No VERIFY token may follow the marker anywhere in the entry,
// in any context: a later verdict, even quoted, leaves the entry unsettled.
func heldEntryPass(lines []string, live []bool, lo, hi int) bool {
	at := -1
	for i := lo + 1; i < hi; i++ {
		m := verifyVerdictBoldRe.FindStringSubmatchIndex(lines[i])
		if m == nil || m[0] != 0 || lines[i][m[2]:m[3]] != "PASS" || !live[i] {
			continue
		}
		if strings.ContainsAny(lines[i][m[0]:m[1]], "`<[\\~|") {
			continue
		}
		if i-1 != lo && !isBlankLine(lines[i-1]) {
			continue
		}
		at = i
	}
	if at < 0 {
		return false
	}
	for i := at; i < hi; i++ {
		n := len(heldVerdictTokenRe.FindAllStringIndex(lines[i], -1))
		if i == at && n > 1 || i > at && n > 0 {
			return false
		}
	}
	return true
}

// heldCellRanges returns the byte range of each cell's content in a table
// line: the text between unescaped "|" delimiters (a `\|` is content, as
// splitRow reads it). The whitespace-only text before the first delimiter
// and after the last one is not a cell.
func heldCellRanges(line string) [][2]int {
	var pipes []int
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) && line[i+1] == '|' {
			i++
			continue
		}
		if line[i] == '|' {
			pipes = append(pipes, i)
		}
	}
	if len(pipes) == 0 {
		return nil
	}
	var out [][2]int
	if strings.TrimSpace(line[:pipes[0]]) != "" {
		out = append(out, [2]int{0, pipes[0]})
	}
	for k := 0; k+1 < len(pipes); k++ {
		out = append(out, [2]int{pipes[k] + 1, pipes[k+1]})
	}
	if last := pipes[len(pipes)-1]; strings.TrimSpace(line[last+1:]) != "" {
		out = append(out, [2]int{last + 1, len(line)})
	}
	return out
}

// heldNorm normalises a table cell for comparison: emphasis and surrounding
// space trimmed, lower case, inner whitespace collapsed.
func heldNorm(cell string) string {
	n := strings.ToLower(strings.Trim(strings.TrimSpace(cell), "*_ \t"))
	return strings.Join(strings.Fields(n), " ")
}

// heldKeyColumns and heldOtherColumns are the exact header names (heldNorm)
// rule 1 recognises: a row-key column, and a column that holds no row's
// result. Every other header — Exit, Result, Output, Observed, a note, and
// any longer spelling such as "Command (as run)" or "Runner verdict" — is a
// result column and is read.
var (
	heldKeyColumns   = map[string]bool{"#": true, "row": true, "no.": true, "no": true, "verify row": true, "command": true, "cmd": true}
	heldOtherColumns = map[string]bool{"expect": true, "expected": true, "date": true, "runner": true,
		"date / runner": true, "date/runner": true, "date · runner": true, "date runner": true}
)

// heldColumn classifies an Evidence table header cell. key is a row-key
// column; notResult is any column that is not a row's result (the key
// columns, Expect, Date, Runner).
func heldColumn(header string) (key, notResult bool) {
	n := heldNorm(header)
	switch {
	case heldKeyColumns[n]:
		return true, true
	case heldOtherColumns[n]:
		return false, true
	}
	return false, false
}

// heldRowKey reads a row key (rule 3): the key column's header name and the
// row's key cell, each normalised (heldNorm). Rows correspond only when both
// are equal, so a `#`-keyed row never matches a Command-keyed one, and two
// rows that render alike but are written differently never match. An empty
// key cell is not a key ("").
func heldRowKey(header, cell string) string {
	if k := heldNorm(cell); k != "" {
		return heldNorm(header) + "\x1f" + k
	}
	return ""
}

var (
	// heldAnnotatedKeyRe is a key cell (heldNorm) that is a Verify row number
	// with one parenthetical annotation and nothing after it:
	// "3 (verify 3, negative)".
	heldAnnotatedKeyRe = regexp.MustCompile(`^(\d+)[ \t]*\(([^()]*)\)$`)
	heldNumberRe       = regexp.MustCompile(`\d+`)
)

// heldKeyBase splits a row key (heldRowKey) into the key of its bare row
// number and its annotation (rule 3, #4294). ok is false — and base is the key
// itself — unless the key cell is a row number with an annotation in which
// every number is that row number.
func heldKeyBase(key string) (base, ann string, ok bool) {
	i := strings.IndexByte(key, '\x1f')
	if i < 0 {
		return key, "", false
	}
	m := heldAnnotatedKeyRe.FindStringSubmatch(key[i+1:])
	if m == nil {
		return key, "", false
	}
	for _, n := range heldNumberRe.FindAllString(m[2], -1) {
		if n != m[1] {
			return key, "", false
		}
	}
	return key[:i+1] + m[1], strings.TrimSpace(m[2]), true
}

// heldCanonKeys rewrites each annotated row key to the key of its bare row
// number (rule 3, #4294), in out and in keys alike, unless that number may
// have sub-rows: the bare number appears, anywhere in the Evidence, with two
// different annotations, or one entry records it in two forms (bare and
// annotated, or two annotations). Such keys stay as written.
//
// The rewrite is directional: it applies only to the entries before the last
// one (lines before last, keys of all but the final entry). The last entry's
// keys stay as written, so a row covers an earlier one under the earlier
// row's own key or under the bare number, never under an annotation the
// earlier row did not carry. An earlier key the last entry records exactly
// as written is left as written, so the raw keys still match.
func heldCanonKeys(out []heldLineRead, keys [][]string, last int) {
	lastRaw := map[string]bool{} // the last entry's keys, as written
	if len(keys) > 1 {
		for _, k := range keys[len(keys)-1] {
			lastRaw[k] = true
		}
	}
	forms := map[string]map[string]bool{} // bare key → the non-empty annotations seen
	split := map[string]bool{}            // bare key → two forms in one entry
	for e := range keys {
		seen := map[string]string{}
		for _, k := range keys[e] {
			base, ann, ok := heldKeyBase(k)
			if !ok {
				base, ann = k, ""
			}
			if prev, dup := seen[base]; dup && prev != ann {
				split[base] = true
			}
			seen[base] = ann
			if ann != "" {
				if forms[base] == nil {
					forms[base] = map[string]bool{}
				}
				forms[base][ann] = true
			}
		}
	}
	canon := func(k string) string {
		base, _, ok := heldKeyBase(k)
		if !ok || lastRaw[k] || split[base] || len(forms[base]) > 1 {
			return k
		}
		return base
	}
	for r := 0; r < last && r < len(out); r++ {
		if out[r].key != "" {
			out[r].key = canon(out[r].key)
		}
	}
	for e := 0; e+1 < len(keys); e++ {
		for j, k := range keys[e] {
			keys[e][j] = canon(k)
		}
	}
}

// heldObserveCells returns the Result-type cells (heldColResult) of an aligned
// results row, line, whose table, with header line hline, has an Exit column
// and whose every Exit cell reads clean (rule 4, #4294); nil otherwise.
func heldObserveCells(hline string, header [][2]int, line string, cells [][2]int, notResult []bool) [][2]int {
	exit := false
	var res [][2]int
	for c, rg := range cells {
		if notResult[c] {
			continue
		}
		switch heldCoverColumn(hline[header[c][0]:header[c][1]]) {
		case heldColExit:
			if heldOutcome(line[rg[0]:rg[1]], true) != heldCellPass {
				return nil
			}
			exit = true
		case heldColResult:
			res = append(res, rg)
		}
	}
	if !exit {
		return nil
	}
	return res
}

// heldObserveVerbs are the verbs of product output (rule 4).
const heldObserveVerbs = `prints?|printed|yields?|yielded|emits?|emitted|outputs?|lists?|listed|renders?|rendered`

var (
	// heldObserveLeadRe is a Result cell that opens on a passing outcome,
	// past space, quotes, emphasis and backticks: `"PASS: negative" …`.
	// A case label may follow it directly ("PASS: negative"): the label is
	// part of the lead, not a subject of the text after it.
	heldObserveLeadRe = regexp.MustCompile("(?i)^[\\s\"'“”‘’*_`]*(?:pass(?:ed)?|ok)\\b(?:[ \\t]*:[ \\t]*(?:positive|negative)\\b)?")
	// heldObserveBeforeRe is an article or possessive right before the marker.
	heldObserveBeforeRe = regexp.MustCompile(`(?i)(?:^|[^\pL\pN-])(?:a|an|the|its|their)[ \t]+$`)
	// heldObserveTailRe is a word of the text after the marker.
	heldObserveTailRe = regexp.MustCompile(`[\pL\pN]+(?:-[\pL\pN]+)*`)
	// heldObserveClauseRe is a clause break; the text after the last one,
	// up to the marker, is the clause that governs it.
	heldObserveClauseRe = regexp.MustCompile(`[;.!?—–]`)
	// heldObserveVerbRe is a verb of product output that must govern the
	// marker in its clause ("prints", "yields"): the row says what the product
	// wrote. Verbs a verifier also uses for its own instrument's disposition
	// (reports, shows, contains, includes, displays) are not among them.
	heldObserveVerbRe = regexp.MustCompile(`(?i)\b(?:` + heldObserveVerbs + `)\b`)
	// heldObserveVerbWordRe is one word that is such a verb.
	heldObserveVerbWordRe = regexp.MustCompile(`^(?:` + heldObserveVerbs + `)$`)
	// heldObserveSubject are the subjects, as lowercased words joined by one
	// space, that may stand between the lead and the output verb: each names
	// the product under test, never the verification. "planted brief" is an
	// input the product reads; a bare "brief" can name the brief whose Verify
	// table holds the row, so it is not one. The generic names of a program
	// or what it wrote (command, tool, cli, binary, lint, linter, output,
	// stdout) and "fixture" can name the row's own instrument, which the
	// Command cell runs, so none is a subject; nor is the verifier, its
	// runner or harness, a case, a half, rows, a pronoun or a digit. A cell
	// with no subject leaves the row held too: its implied subject is the
	// row's command.
	heldObserveSubject = map[string]bool{"the planted brief": true, "the product": true}
	// heldObserveObject are the words that may follow the output verb, up to
	// the marker: articles, "its", "and", and nouns of what the product wrote
	// ("its named diagnostic, a lint PROBLEM and a"). With no verb among them
	// none can open a clause of its own, so "lint" here only qualifies a
	// noun. "their" is not among them: the cell's one subject is singular,
	// so it would point outside the cell, at rows or cases.
	heldObserveObject = map[string]bool{"a": true, "an": true, "the": true, "its": true, "and": true,
		"named": true, "diagnostic": true, "diagnostics": true, "lint": true, "problem": true,
		"problems": true}
	// heldObserveHoldRe is a hold or fail word anywhere in a row's read cells.
	// It is a second check behind the closed grammar, never the only one.
	// HELD is not among them: every HELD occurrence is read on its own, and
	// rule 4 never excuses one.
	heldObserveHoldRe = regexp.MustCompile(`(?i)\b(?:blocked|fail\w*|skip\w*|unrun|deferred|pending|awaiting|awaits?|waiting|needs?|needed|still|open|remain\w*|outstanding|incomplete|partial\w*|offline|unavailable|unreachable|unverified|not[ \t]+(?:yet|exercised|executed|available|checked|re-?run|run)|no[ \t]+runner|cannot|can't|unable|todo)\b`)
	// heldObserveHead are the words that may follow the marker directly: the
	// noun it qualifies. heldObserveTail adds the neutral words that may
	// follow that noun. Any other word, or a digit, leaves the row held.
	heldObserveHead = map[string]bool{"section": true, "sections": true, "banner": true, "heading": true,
		"headings": true, "label": true, "labels": true, "mission": true}
	heldObserveTail = map[string]bool{"section": true, "sections": true, "banner": true, "heading": true,
		"headings": true, "label": true, "labels": true, "mission": true, "with": true, "no": true,
		"empty": true, "content": true, "once": true, "twice": true,
		"the": true, "a": true, "an": true, "its": true, "and": true, "as": true, "expected": true,
		"correctly": true, "in": true, "output": true, "text": true, "lists": true, "listing": true,
		"printed": true, "shown": true}
)

// heldObserveWords reports whether words, lowercased, are one product subject
// (heldObserveSubject), one output verb and then only object words
// (heldObserveObject), in that order (rule 4, #4294).
func heldObserveWords(words []string) bool {
	for i, w := range words {
		if !heldObserveVerbWordRe.MatchString(w) {
			continue
		}
		if !heldObserveSubject[strings.Join(words[:i], " ")] {
			return false
		}
		for _, o := range words[i+1:] {
			if !heldObserveObject[o] {
				return false
			}
		}
		return true
	}
	return false
}

// heldObservedOutput reports whether the occurrence at clean[h[0]:h[1]] is a
// row's record of what the product printed rather than its disposition (rule
// 4, #4294). line is the row with its non-result cells blanked, clean the same
// row as the scan reads it; the caller passes only a line with no struck span,
// so the offsets of both agree.
func heldObservedOutput(line, clean string, read heldLineRead, h []int) bool {
	if read.key == "" || !strings.EqualFold(clean[h[0]:h[1]], "could-not-check") || heldObserveHoldRe.MatchString(line) {
		return false
	}
	for _, rg := range read.observe {
		if h[0] < rg[0] || h[1] > rg[1] {
			continue
		}
		before, after := clean[rg[0]:h[0]], clean[h[1]:rg[1]]
		lead := heldObserveLeadRe.FindStringIndex(line[rg[0]:rg[1]])
		if lead == nil || !heldObserveBeforeRe.MatchString(before) || heldReasonAfterRe.MatchString(after) {
			return false
		}
		// The words between the lead and the marker, as the row is
		// written (inline code included): the subject of the verb is the
		// product, never the verification.
		if !heldObserveWords(heldObserveTailRe.FindAllString(strings.ToLower(line[rg[0]+lead[1]:h[0]]), -1)) {
			return false
		}
		if i := heldObserveClauseRe.FindAllStringIndex(before, -1); len(i) > 0 {
			before = before[i[len(i)-1][1]:]
		}
		if !heldObserveVerbRe.MatchString(before) {
			return false
		}
		words := heldObserveTailRe.FindAllString(strings.ToLower(after), -1)
		if len(words) == 0 || !heldObserveHead[words[0]] {
			return false
		}
		for _, w := range words {
			if !heldObserveTail[w] {
				return false
			}
		}
		return true
	}
	return false
}

// heldSeparatorRow reports whether line is a table's header separator.
func heldSeparatorRow(line string) bool {
	t := strings.Trim(strings.TrimSpace(line), "|")
	return strings.Contains(t, "-") && separatorRowRe.MatchString(t)
}

// heldTable is one results table the scope recognised (rule 1).
type heldTable struct {
	header, end int    // the header line, and the line after the last row
	keyCol      int    // the first row-key column
	notResult   []bool // per column: not a result column
}

var (
	// heldWitnessHeaders are the header lines statusgen verifyrun writes for
	// its execution witness table (witnessHeader, witnessHeaderFailFirst).
	heldWitnessHeaders = map[string]bool{
		strings.SplitN(witnessHeader, "\n", 2)[0]:          true,
		strings.SplitN(witnessHeaderFailFirst, "\n", 2)[0]: true,
	}
	// heldWitnessTrailerRes are the lines statusgen verifyrun and its
	// container bracket write directly under a witness table, with no blank
	// line between (containerWitnessLines, the admission receipt's
	// attestation, the bracket close), each matched field by field and
	// anchored at both ends. Most fields are a digest, a hex id or a
	// timestamp; the repo, brief= and model= fields carry names the run was
	// given (a repository, a brief path, a model id), so they may spell any
	// word their character class admits, a hold word such as HELD included.
	// That widens nothing: such a line is passed over only for alignment
	// (heldWitnessTrailerEnd), so an earlier entry's line is dropped only when
	// that entry is superseded whole, exactly as a prose line there already
	// is, and a trailer line in the latest entry is still read for holds like
	// any other line. A "|", "<" or ">" cannot appear in any of them, so such
	// a line is never a row GFM splits into cells nor markup.
	heldWitnessTrailerRes = []*regexp.Regexp{
		regexp.MustCompile(`^Tree-Digest: sha256:[0-9a-f]{64}$`),
		regexp.MustCompile(`^Container-Image: sha256:[0-9a-f]{64}$`),
		regexp.MustCompile(`^Verification-Attestation: [A-Za-z0-9._-]+/[A-Za-z0-9._-]+#[0-9]+ run=[0-9a-f]{64} (?:source-repo=[A-Za-z0-9._-]+/[A-Za-z0-9._-]+ )?source=[0-9a-f]{40} (?:tracking-source=[0-9a-f]{40} )?brief=[A-Za-z0-9._/-]+ model=[A-Za-z0-9._:\[\]-]+ tier=[a-z]+$`),
		regexp.MustCompile(`^Admission-Bracket: run=[0-9a-f]{64} source=[0-9a-f]{40} tree=sha256:[0-9a-f]{64} witness=sha256:[0-9a-f]{64} opened=[0-9TZ:+.-]+ closed=[0-9TZ:+.-]+ before=receipt after=check-verifier$`),
	}
)

// heldWitnessTrailerEnd returns the first line at or after j, the line after
// the table that opens at header, that is not a verifyrun trailer line
// (heldWitnessTrailerRes), or j when the table is not a verifyrun witness
// table (heldWitnessHeaders, matched byte for byte) or line j is no trailer
// (#4294). GFM reads such a line as one more row of the table above, but its
// shape is fixed field by field, so it is not a results row the scan fails to
// key: whole supersession may read past it, as long as the run of trailer
// lines then ends at a blank line or the entry's end. Any other line there
// still counts against the entry (heldScanScope).
func heldWitnessTrailerEnd(lines []string, inFence []bool, header, j, hi int) int {
	if !heldWitnessHeaders[lines[header]] {
		return j
	}
	t := j
	for ; t < hi && !inFence[t]; t++ {
		trailer := false
		for _, re := range heldWitnessTrailerRes {
			if re.MatchString(lines[t]) {
				trailer = true
				break
			}
		}
		if !trailer {
			break
		}
	}
	return t
}

// heldScanScope returns what the held scan reads of evidence (split on "\n"),
// under the rules documented above.
func heldScanScope(evidence string) heldScope {
	lines := strings.Split(evidence, "\n")
	out := make([]heldLineRead, len(lines))
	live := heldRender(lines)
	inFence := make([]bool, len(lines)) // as the scan reads it: a toggle or a line inside fenced code
	fence := false
	for i, l := range lines {
		if heldFenceToggle(l) {
			fence = !fence
			inFence[i] = true
			continue
		}
		inFence[i] = fence
	}
	html := heldHTMLLines(lines, inFence)

	// Entries, split at live level-3 headings.
	starts := []int{0}
	for i, l := range lines {
		if i > 0 && live[i] && heldEntryHeading(l) {
			starts = append(starts, i)
		}
	}

	var tables []heldTable
	// Per entry: its results tables' row keys, and whether every data row of
	// them is aligned and keyed (rule 3b).
	keys := make([][]string, len(starts))
	aligned := make([]bool, len(starts))
	for e := range starts {
		lo, hi := starts[e], len(lines)
		if e+1 < len(starts) {
			hi = starts[e+1]
		}
		aligned[e] = true
		// Table blocks: runs of table lines outside fenced code.
		hasResults := false
		for i := lo; i < hi; {
			if inFence[i] || !heldTableLine(lines[i]) {
				i++
				continue
			}
			j := i
			for j < hi && !inFence[j] && heldTableLine(lines[j]) {
				j++
			}
			if j-i >= 2 && heldSeparatorRow(lines[i+1]) {
				header := heldCellRanges(lines[i])
				tb := heldTable{header: i, end: j, keyCol: -1, notResult: make([]bool, len(header))}
				hasResult := false
				for c, r := range header {
					k, nr := heldColumn(lines[i][r[0]:r[1]])
					if k && tb.keyCol < 0 {
						tb.keyCol = c
					}
					tb.notResult[c] = nr
					hasResult = hasResult || !nr
				}
				if tb.keyCol >= 0 && hasResult {
					hasResults = true
					if t := heldWitnessTrailerEnd(lines, inFence, i, j, hi); t < hi && !isBlankLine(lines[t]) {
						aligned[e] = false // a line GFM may read as one more row of the table
					}
					tables = append(tables, tb)
					out[i].skip, out[i+1].skip = true, true // header and separator
					for r := i + 2; r < j; r++ {
						cells := heldCellRanges(lines[r])
						if len(cells) != len(header) {
							aligned[e] = false
							continue // cannot align: read whole, no key (fail closed)
						}
						for c, rg := range cells {
							if tb.notResult[c] {
								out[r].blank = append(out[r].blank, rg)
							}
						}
						kc := cells[tb.keyCol]
						hc := header[tb.keyCol]
						out[r].key = heldRowKey(lines[i][hc[0]:hc[1]], lines[r][kc[0]:kc[1]])
						if out[r].key == "" {
							aligned[e] = false
							continue
						}
						keys[e] = append(keys[e], out[r].key)
						out[r].observe = heldObserveCells(lines[i], header, lines[r], cells, tb.notResult)
					}
				}
			}
			i = j
		}
		for i := lo; i < hi; i++ {
			out[i].whole = html[i] != heldHTMLOther
			// An HTML table renders live in any cell, header and keyed rows
			// included; only a results table's own pipes are exempt.
			tableish := heldHTMLTableRe.MatchString(lines[i])
			if !out[i].skip && out[i].key == "" && !heldOneLineProse(lines, inFence, i) {
				tableish = tableish || heldPipeOutsideCode(lines[i])
			}
			if !inFence[i] && tableish {
				aligned[e] = false // a table the scan cannot key
			}
		}
		if !hasResults {
			continue
		}
		for i := lo; i < hi; i++ {
			if !inFence[i] && !heldTableLine(lines[i]) {
				out[i].prose = true
			}
		}
	}

	heldCanonKeys(out, keys, starts[len(starts)-1])

	scope := heldScope{lines: out, from: -1}
	if len(starts) > 1 {
		last := starts[len(starts)-1]
		if heldEntryPass(lines, live, last, len(lines)) {
			if covered := heldCovered(lines, live, out, tables, last); len(covered) > 0 {
				scope.from, scope.covered = last, covered
				scope.whole = heldWholeCovered(keys[:len(starts)-1], aligned[:len(starts)-1], covered) &&
					!heldHTMLUnclosed(lines, inFence) && !heldContainerDoubt(lines, inFence) &&
					!heldElementCheck(lines, inFence, starts)
			}
		}
	}
	return scope
}

// heldCovered returns the row keys the last entry (from line last on)
// re-ran clean. A key is covered only when EVERY keyed row of it in the last
// entry's results tables is clean, so a Verify row that ran two commands, or
// a second table that lists the same key, cannot cover a row the other line
// says was skipped. A row is clean only in a table that opens its own block
// (the line before it is blank or the entry heading, so it cannot continue a
// blockquote or a list item) with every line at column 0 and rendered live;
// a keyed row of any other results table in the entry counts as not clean.
// One row in the entry's results tables that cannot be aligned with its
// header leaves nothing covered: its key cannot be read, so it cannot be ruled
// out as a second line of a held row.
//
// A row is clean when its outcome columns (heldCoverColumn) carry at least
// one recognised clean outcome and nothing else that is visible (heldOutcome).
// An expectation column ("Expected exit", "Pass criteria") is never read for
// the outcome. Any other result column must be empty: a note, a check name or
// any text the cover cannot classify keeps the row from covering.
func heldCovered(lines []string, live []bool, out []heldLineRead, tables []heldTable, last int) map[string]bool {
	clean, bad := map[string]bool{}, map[string]bool{}
	for _, tb := range tables {
		if tb.header <= last {
			continue
		}
		verified := true
		if prev := tb.header - 1; prev != last && !isBlankLine(lines[prev]) {
			verified = false
		}
		for i := tb.header; i < tb.end; i++ {
			verified = verified && live[i] && strings.HasPrefix(lines[i], "|")
		}
		header := heldCellRanges(lines[tb.header])
		for r := tb.header + 2; r < tb.end; r++ {
			if out[r].key == "" {
				return nil // a row that cannot be aligned (fail closed)
			}
			cells := heldCellRanges(lines[r])
			ran, other := false, false
			for c, rg := range cells {
				if tb.notResult[c] {
					continue
				}
				role := heldCoverColumn(lines[tb.header][header[c][0]:header[c][1]])
				if role == heldColExpect || heldOutputDigest(lines[tb.header][header[c][0]:header[c][1]], lines[r][rg[0]:rg[1]]) {
					continue
				}
				if heldFailFirstBase(lines[tb.header][header[c][0]:header[c][1]], lines[r][rg[0]:rg[1]]) {
					continue
				}
				st := heldOutcome(lines[r][rg[0]:rg[1]], role == heldColExit)
				if role == heldColOther && st != heldCellEmpty {
					st = heldCellOther
				}
				switch st {
				case heldCellPass:
					ran = true
				case heldCellOther:
					other = true
				}
			}
			if verified && ran && !other {
				clean[out[r].key] = true
			} else {
				bad[out[r].key] = true
			}
		}
	}
	covered := map[string]bool{}
	for k := range clean {
		if !bad[k] {
			covered[k] = true
		}
	}
	return covered
}

// heldColRole is what a result column of a covering table holds
// (heldCoverColumn).
type heldColRole int

const (
	heldColOther  heldColRole = iota // a note, a check name, any header the cover does not recognise
	heldColExit                      // an exit code: Exit, Exit code, rc
	heldColResult                    // the row's actual result: Result, Observed, Output, Outcome, Status
	heldColExpect                    // an expectation or criterion, never the outcome
)

var (
	// heldExitColumns and heldResultColumns are the exact header names
	// (heldNorm) the cover reads a row's outcome from.
	heldExitColumns   = map[string]bool{"exit": true, "exit code": true, "exit status": true, "rc": true}
	heldResultColumns = map[string]bool{"result": true, "results": true, "observed": true, "output": true,
		"outcome": true, "status": true, "actual": true, "actual result": true, "observed result": true, "verdict": true}
	// heldExpectColumnRe is a header (heldNorm) that names an expectation or a
	// criterion and nothing else: Expect, Expected exit, Expected result, Pass
	// criteria. It is anchored on both ends. A header that merely CONTAINS the
	// word (Verdict vs Expect, Matches expected?, Meets criteria, Criteria
	// met?) records an actual verdict, so it stays heldColOther and must be
	// empty.
	heldExpectColumnRe = regexp.MustCompile(`^(expect(ed)?( (exit( code| status)?|outcome|result|output|status))?|(pass |acceptance )?criteri(a|on))$`)
)

// heldCoverColumn classifies a result-column header for the cover.
func heldCoverColumn(header string) heldColRole {
	n := heldNorm(header)
	switch {
	case heldExitColumns[n]:
		return heldColExit
	case heldResultColumns[n]:
		return heldColResult
	case heldExpectColumnRe.MatchString(n):
		return heldColExpect
	}
	return heldColOther
}

// heldCellState is what a covering row's result cell says (heldOutcome).
type heldCellState int

const (
	heldCellEmpty heldCellState = iota // renders as nothing a reader could see
	heldCellPass                       // a recognised clean outcome
	heldCellOther                      // anything else: a placeholder, a carry-forward, markup, unrecognised text
)

var (
	// heldOutcomeRe is a recognised clean outcome at the start of what is
	// left of a result cell: "ok", "PASS", "passed", "green", "exit 0",
	// "rc=0", a check mark. A bare "0" is an outcome only as a whole Exit
	// cell (heldOutcome).
	heldOutcomeRe = regexp.MustCompile(`^(?:ok|pass(?:ed|es)?|green|clean|success(?:ful)?|exit(?:[ \t]+code)?[ \t]*[:=]?[ \t]*0|rc[ \t]*[:=]?[ \t]*0|✓|✔|✅)`)
	// heldDetailRes are the neutral details a clean outcome may carry after
	// it, the closed set heldDetailsClean accepts: a count of things that ran
	// (its numbers checked by heldDetailCounts), a count of zero failures, a
	// duration, or the clean outcome again ("exit 0, ok").
	heldDetailRes = []*regexp.Regexp{
		regexp.MustCompile(`^(\d+)[ \t]+(?:tests?|subtests?|checks?|cases?|files?|rows?|assertions?|packages?)(?:[ \t]+(?:passed|passing|ok|green|run|ran))?`),
		regexp.MustCompile(`^(\d+)[ \t]+(?:passed|passing)`),
		regexp.MustCompile(`^(\d+)[ \t]*/[ \t]*(\d+)(?:[ \t]+(?:passed|passing|ok|green))?`),
		regexp.MustCompile(`^(\d+)[ \t]+of[ \t]+(\d+)(?:[ \t]+(?:tests?|subtests?|checks?|cases?|rows?))?[ \t]+(?:passed|passing|ok|green)`),
		regexp.MustCompile(`^(?:0[ \t]+(?:failures?|failed|fails|errors?|findings?)|no[ \t]+(?:failures|errors|findings))`),
		regexp.MustCompile(`^(?:in[ \t]+)?\d+(?:\.\d+)?[ \t]*(?:minutes?|mins?|ms|m|seconds?|secs?|s)`),
		// The sandbox token statusgen verifyrun writes after `exit=` (#2350):
		// it names WHICH network-off sandbox ran the row, one of the two
		// compile-time modes (verifysandbox.go), never caller text. The mode
		// is a whole word: it ends the cell or a space, comma or semicolon
		// follows it, so "sandbox=unshare-ok" is not the unshare mode.
		regexp.MustCompile(`^sandbox=(?:unshare|container-netns)(?:[ \t,;]|$)`),
		heldOutcomeRe,
	}
	// heldDigestRe is the Output cell statusgen verifyrun writes for every
	// witness row: the first 12 hex digits of the sha256 of the row's captured
	// output (hashOutput), never caller text.
	heldDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{12}$`)
	// heldBaseCellRe is the Base cell a fail-first verifyrun run writes for
	// every witness row (failFirstCell): the merge-base SHA or "-", the base
	// run's state (an exit code only after red or green), and the head tree
	// as treeSHA renders it. It records the run at the merge-base, never the
	// head outcome, and none of it is caller text.
	heldBaseCellRe = regexp.MustCompile(`^base=(?:[0-9a-f]{7,40}|-) (?:(?:red|green) rc=(?:-|[0-9]+)|unproven|not-selected) · head=(?:[0-9a-f]{7,40}(?:\+dirty|\+unknown)?|no-git)$`)
	// heldStrikeRe is a struck span as GFM renders one: the text between the
	// tildes neither starts nor ends with a space. "~~ HELD ~~" is not a
	// strike on GitHub, so the cover keeps it.
	heldStrikeRe = regexp.MustCompile(`~~[^~ \t](?:[^~]*[^~ \t])?~~`)
	// heldDetailSepRe is the punctuation, emphasis and space between details.
	heldDetailSepRe = regexp.MustCompile("^[ \t,;:.()\\[\\]·—–*_`-]+")
)

// heldOutcome classifies one outcome cell of a covering row by what it renders
// as; exit says the cell is in an Exit column. Struck spans that render as a
// strike (heldStrikeRe) are removed first.
// The WHOLE rest of the cell must then read as a clean outcome: a bare "0" as
// a whole Exit cell, or a recognised outcome (heldOutcomeRe) followed only by
// neutral details (heldDetailsClean). The grammar is closed — letters, digits,
// spaces, a few punctuation marks and the check marks — so anything else is
// heldCellOther: a placeholder (—, n/a), a carry-forward (same as Run 1), a
// note after the outcome ("PASS (no re-run)"), a zero count ("0 checks run"),
// unrecognised text, and markup or an invisible format character that may
// render as nothing or hide text (<!-- -->, <br>, &nbsp;, a backslash escape,
// a zero-width space), none of which the grammar admits.
func heldOutcome(cell string, exit bool) heldCellState {
	cell = heldStrikeRe.ReplaceAllString(cell, "")
	n := strings.Trim(heldNorm(cell), "`*_ ")
	if n == "" {
		return heldCellEmpty
	}
	if exit && n == "0" {
		return heldCellPass
	}
	m := heldOutcomeRe.FindString(n)
	if m == "" || !heldDetailsClean(n[len(m):]) {
		return heldCellOther
	}
	return heldCellPass
}

// heldOutputDigest reports whether cell, under header, is a verifyrun witness
// row's output digest (heldDigestRe in an Output column; #4294). A digest says
// nothing about the outcome — the row's Result cell does — so the cover reads
// it as neither a clean outcome nor a note: such a row is clean only on what
// its other outcome cells say, and a digest alone never makes a row clean.
// The cell is compared as the tool writes it, trimmed of space only: a digest
// in emphasis or in upper case is not the tool's text, so it is a note.
func heldOutputDigest(header, cell string) bool {
	return heldNorm(header) == "output" && heldDigestRe.MatchString(strings.TrimSpace(cell))
}

// heldFailFirstBase reports whether cell, under header, is a fail-first
// witness row's Base cell (heldBaseCellRe in a Base column; #4294). Like the
// output digest it says nothing about the head outcome, so the cover reads it
// as neither a clean outcome nor a note, and a Base cell alone never makes a
// row clean. Any other text in a Base column, or a Base cell in any other
// column, is still a note.
func heldFailFirstBase(header, cell string) bool {
	return heldNorm(header) == "base" && heldBaseCellRe.MatchString(strings.TrimSpace(cell))
}

// heldDetailsClean reports whether t, the text after a clean outcome, is
// nothing but neutral details (heldDetailRes), optionally separated by
// punctuation, emphasis and space. Whatever is left after the last detail
// must itself be a detail, so text outside the closed set never passes.
func heldDetailsClean(t string) bool {
	for {
		t = heldDetailSepRe.ReplaceAllString(t, "")
		if t == "" {
			return true
		}
		matched := false
		for _, re := range heldDetailRes {
			m := re.FindStringSubmatch(t)
			if m == nil {
				continue
			}
			if !heldDetailCounts(m[1:]) {
				return false
			}
			t, matched = t[len(m[0]):], true
			break
		}
		if !matched {
			return false
		}
	}
}

// heldDetailCounts checks the numbers a count detail carries: every count of
// things that ran is above zero, and a ratio ("3/3", "3 of 3 passed") passed
// all it ran.
func heldDetailCounts(nums []string) bool {
	for _, s := range nums {
		if n, err := strconv.Atoi(s); err != nil || n == 0 {
			return false
		}
	}
	return len(nums) < 2 || nums[0] == nums[1]
}

var (
	// heldExitAfterRe is an exit-code mapping right after the marker:
	// "could-not-check → exit 6", "-> exit code 2", "= exit 6".
	heldExitAfterRe = regexp.MustCompile(`(?i)^[ \t*_]*(?:→|->|=>|=)[ \t*_]*exit(?:[ \t]+code)?[ \t]*\d`)
	// heldExitBeforeRe is an exit code whose meaning the marker names in
	// parentheses, right before it: "exit 6 (could-not-check".
	heldExitBeforeRe = regexp.MustCompile(`(?i)\bexit(?:[ \t]+code)?[ \t]*\d+[ \t]*\([ \t*_]*$`)
	// heldNamesRowRe is a line that names a row: "row 2", "rows", "#4".
	heldNamesRowRe = regexp.MustCompile(`(?i)\brows?\b|#[ \t]*\d`)
)

// heldExitMapping reports whether the occurrence at clean[h[0]:h[1]] is
// written as an exit-code mapping on a line that names no row (rule 2) rather
// than a disposition.
func heldExitMapping(clean string, h []int) bool {
	if heldNamesRowRe.MatchString(clean) {
		return false
	}
	return heldExitAfterRe.MatchString(clean[h[1]:]) || heldExitBeforeRe.MatchString(clean[:h[0]])
}

// heldBlankRanges overwrites the given byte ranges of s with spaces. The
// length is unchanged, so offsets into s still agree with the line's.
func heldBlankRanges(s string, rs [][2]int) string {
	if len(rs) == 0 {
		return s
	}
	b := []byte(s)
	for _, r := range rs {
		for k := r[0]; k < r[1] && k < len(b); k++ {
			b[k] = ' '
		}
	}
	return string(b)
}

// Sourced RISK-VALUE derivations describe constants and domain transitions,
// not execution observations. The prefix alone is never an exemption: only
// marker occurrences within these closed declaration/semantic phrases are
// excluded. A second, unrecognised occurrence on the same line is still read.
// No entry supersession or passing witness is needed to recognise a definition.
// Only the declared product fact-state vocabulary is recognised. Arbitrary
// identifiers, even in assignment syntax, may be row/run observations.
const heldRiskStateDecl = `(?:Present = "present"|Missing = "missing"|(?:CouldNotCheck|Unavailable) = "could-not-check")`

var (
	heldRiskRecordRe  = regexp.MustCompile(`^RISK-VALUE: DERIVED — ([^@|\r\n]+) @ [a-zA-Z0-9_./-]+:[1-9][0-9]*(?:-[1-9][0-9]*)? — `)
	heldRiskLiteralRe = regexp.MustCompile(`^` + heldRiskStateDecl + `(?:(?:, | and )` + heldRiskStateDecl + `)*$`)
	heldRiskMeaningRe = regexp.MustCompile(`(?i)\bmissing and could-not-check cannot release assessment\b|\ba timeout maps to could-not-check, never to permitted\b|\bretirement may remain held separately\b|\bcomplete/retirement-held (?:must|to) stay distinct from complete\b`)
)

// heldRiskDefinition reports whether this occurrence is a declared literal
// before the source locator, or part of a normative state relation after it.
// This deliberately leaves other wording unresolved rather than treating
// arbitrary prose carrying RISK-VALUE as a cleared acceptance obligation.
func heldRiskDefinition(clean string, h []int) bool {
	record := heldRiskRecordRe.FindStringSubmatchIndex(clean)
	if record == nil {
		return false
	}
	var spans [][]int
	if h[1] <= record[3] {
		// The whole declaration list must match: accepting a substring
		// would hide an observation before or beside a valid constant.
		return heldRiskLiteralRe.MatchString(clean[record[2]:record[3]])
	} else if h[0] >= record[1] && !heldNamesRowRe.MatchString(clean[record[1]:]) {
		for _, span := range heldRiskMeaningRe.FindAllStringIndex(clean[record[1]:], -1) {
			spans = append(spans, []int{record[1] + span[0], record[1] + span[1]})
		}
	}
	for _, span := range spans {
		if span[0] <= h[0] && h[1] <= span[1] {
			return true
		}
	}
	return false
}

// Recounted prose (rule 6, #4721).
//
// Prose that only recounts a state word is not a disposition: a tally that
// closes on a zero, a sentence about what a mutation does, a past flip hold
// the entry cites by its tracker. Each of the three shapes below is a closed
// excuse written to fail closed, like the negation rule it sits beside: it
// applies to the LOWER-CASE occurrence alone (a capital HELD or COULD-NOT-CHECK
// is the disposition token a verifier writes, and Held or Could-Not-Check is
// not on any list), reads only that occurrence's own surroundings, and leaves
// every other occurrence on the line to the rules that already read it.
//
//   - Zero tally: the state word is followed by a bare 0 that ends its clause
//     (";", ",", ")", a sentence stop or the end of the line), optionally after
//     a colon, and what precedes the word is the start of the line, a clause
//     break (";", ",", "(") or a tally label ("accounting:", "summary:"). A noun
//     after the zero ("could-not-check 0 clusters reachable"), a row before the
//     word ("row 3 could-not-check: 0.") or a ratio on the line that did not
//     pass everything it counts ("1 of 2 rows passed") is not a tally.
//   - Mutation sentence: the word is the object of a verb that rounds a state up
//     ("rounds", "coerces", "converts", "promotes", "upgrades") and is followed
//     at once by "to pass" or a synonym ("rounds could-not-check to pass").
//   - Past flip hold: "flip was held on <tracker ref>" or "flip had been held on
//     <tracker ref>", where the tracker is #N or owner/repo#N and the reference
//     ends its clause. A present-tense flip hold, one that names no tracker, or
//     one that goes on to give a reason or a further clause is read.
var (
	heldTallyAfterRe  = regexp.MustCompile(`^[ \t]*:?[ \t]*0[ \t]*(?:[;,)]|\.(?:[ \t]|$)|$)`)
	heldTallyBeforeRe = regexp.MustCompile(`(?i)(?:^|[;,(]|\b(?:accounting|summary|totals?|counts?|tally):)[ \t]*$`)
	heldRoundBeforeRe = regexp.MustCompile(`(?i)\b(?:rounds?|rounded|coerces?|coerced|converts?|converted|promotes?|promoted|upgrades?|upgraded)[ \t]+$`)
	heldRoundAfterRe  = regexp.MustCompile(`(?i)^[ \t]+(?:to|into|as)[ \t]+(?:a[ \t]+)?(?:pass(?:es|ed)?|green|ok|success)\b`)
	heldFlipBeforeRe  = regexp.MustCompile(`(?i)\bflip[ \t]+(?:was|had[ \t]+been)[ \t]+$`)
	heldFlipAfterRe   = regexp.MustCompile(`^[ \t]+on[ \t]+(?:[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)?#[1-9][0-9]*(?:[;,)]|\.(?:[ \t]|$)|[ \t]*$)`)
	// heldRatioRe is a ratio on the line ("1 of 2 rows passed", "3/4").
	heldRatioRe = regexp.MustCompile(`(?i)\b(\d+)(?:[ \t]+of[ \t]+|/)(\d+)\b`)
)

// heldRecounted reports whether the prose occurrence at clean[h[0]:h[1]] is one
// of the three recounted shapes above (rule 6, #4721). The caller passes only
// lines outside every table, with no struck span.
func heldRecounted(clean string, h []int) bool {
	word, before, after := clean[h[0]:h[1]], clean[:h[0]], clean[h[1]:]
	if word != "held" && word != "could-not-check" {
		return false // a capital or mixed-case occurrence is read
	}
	if heldTallyAfterRe.MatchString(after) && heldTallyBeforeRe.MatchString(before) && !heldShortRatio(clean) {
		return true
	}
	if word == "could-not-check" {
		return heldRoundBeforeRe.MatchString(before) && heldRoundAfterRe.MatchString(after)
	}
	return heldFlipBeforeRe.MatchString(before) && heldFlipAfterRe.MatchString(after)
}

// heldShortRatio reports whether clean carries a ratio that did not pass
// everything it counts ("1 of 2 rows passed", "3/4", "0 of 0"): beside one, a
// zero tally after the state word is no evidence that nothing is held.
func heldShortRatio(clean string) bool {
	for _, m := range heldRatioRe.FindAllStringSubmatch(clean, -1) {
		if !heldDetailCounts(m[1:]) {
			return true
		}
	}
	return false
}
