package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
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
//     (#4294), and on rule 3c's read of the last entry so is a one-line
//     bullet item whose neighbours are blank or one-line bullet items
//     (#6767) — or a non-blank line right after a results
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
//     cell; verifyrun's own passing Result cell "pass exit=N" at any
//     observed exit, heldWitnessPassRe, #5541), and no other cell of them or
//     of any unrecognised column holds visible text. A column that names an expectation and nothing else
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
//     (c) The latest run only (#4294, the maintainer's scope ruling): when
//     the last entry is a PASS run, nothing before its heading is read. A
//     PASS run here is a last entry that carries the live strict marker
//     heldEntryPass requires AND records Date/Runner rows (flipDateRunnerRows,
//     outside fenced code, comments removed) every one of which has both a
//     Date and a Runner cell (flipRowDated): a run with a missing Date or
//     Runner cell is not a PASS run, and an undated last entry leaves (a) and
//     (b) as the only ways. Every keyed results row of the last entry must
//     re-run clean as (a) reads it (heldLatestClean), with at least one such
//     row and no table in the entry the scan cannot key: a run whose own row
//     failed, could not run or records no outcome answers nothing before it.
//     Whether the run re-ran EVERY Verify row is not this scan's question:
//     verifyflip asks it of the run it credits (closureRunWitnesses), so 3c
//     applies only where that check runs. Only verifyflip and the reconcile
//     fold read it (flipHeldRefusal, latestRun true), and the model autoflip
//     once that same check has passed on the same run
//     (verifyPassHeldWitnessed, #6767); close-verify, the verify-gate card
//     and the sign-off act run no witness check and read rules 3a and 3b
//     alone. On this read only, a results row of the last entry whose
//     outcome cell opens on a clean outcome and carries a note reads clean
//     when a tool-written witness row of the same key in the same entry
//     already reads clean (heldCorroboratedNote, #6767): a verifier's
//     restatement of a witnessed row does not undo it. And 3c needs every Date/Runner row of
//     the run verifyflip credits to lie after the last entry heading
//     (heldWitnessInLatest), so the run it stops reading at is the run the
//     witness check reads. The last
//     entry's heading must be one the page surely draws (heldBoundaryDoubt,
//     local to that heading): no column-0 level-three heading in the entry
//     that the scan does not render live outside fenced code and HTML
//     comments, and, before the heading, no HTML block or tag left open
//     (heldHTMLUnclosed), no fence in a container (heldContainerFence) and
//     no element left open that strikes, hides or swallows what follows it
//     (heldCarryElement); any of those leaves (a) and (b) as the only ways.
//     A hold inside the last entry is read as always, and an earlier entry is
//     never edited or annotated to get here: it is accurate history, and the
//     scan simply does not read it.
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
//  5. Sourced definitions (#4536, #4200). A standalone or bulleted RISK-VALUE: DERIVED
//     record with a source path and line number may declare a quoted state
//     literal, explain a normative state relation, or map declared exit codes
//     to outcomes (heldRiskDefinition).
//     Only the occurrence inside the recognised phrase is excluded, never
//     the entire line, an unsourced note or a results-table cell. Unrecognised
//     wording and any independent hold remain read. The source locator is
//     syntax, not proof that the derivation or its claimed value is correct.
//
//  6. Structured signals only (#4721, ruling A). The scan reads witness
//     rows and lines that state a current hold; free prose naming a state
//     word is not a hold. A table line (any line carrying "|" or an HTML
//     table tag) is read as rules 1 to 5 say. On any other line an
//     occurrence is read only as one of three signals (heldProseSignal). A
//     HELD marker is a state word not written entirely in lower case (HELD,
//     Held, Could-Not-Check); the test is the word's own case, never a list
//     of spellings. A row result is a line that names a row (row 3, Row3, R3,
//     heldNamesRowNumRe), read at any case. A verdict line (RISK-VALUE: or
//     VERIFY:, heldVerdictLineRe) is read at any case, rule 5's sourced
//     definitions still excused. Lower-case
//     held or could-not-check in any other prose, a step said to be held
//     with no row, marker or verdict included, is not read: the witness row
//     carries its state.
//
//  7. Double-quoted output, flip read only (#4721). A verifier records what a
//     row's command printed — a mutated test's failure message, a diagnostic
//     — in double quotes as often as in inline code. On the flip read
//     (latestRun: verifyflip and the reconcile fold) an occurrence wholly
//     inside a double-quoted span of its cell is not read (heldQuotedOutput:
//     ASCII quotes paired in order, an odd count or an escaped quote in the
//     cell pairing nothing, a span holding only the marker word kept, no
//     struck span on the line). Those callers go on to require a passing
//     execution witness for every Verify row in the run they credit
//     (closureRunWitnesses), so a row that really held cannot flip on its
//     quoted words being skipped; the model autoflip reads it so only after
//     that check passes (verifyPassHeldWitnessed). The verify-gate card,
//     close-verify and the sign-off act run no witness check and read the
//     occurrence as before. An occurrence outside every quoted span on the same line is
//     read as always.
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
	latest  bool            // the superseding entry is a dated PASS run on a sure boundary: nothing before it is read (rule 3c)
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
	if s.latest {
		return true // rule 3c: only the last entry is read
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
//
// A one-line bullet item is the same (#6767): a line at column 0 opened by
// "-", "*" or "+" and one space (heldOneLineItem), whose neighbours are each
// blank or another such item line. Each item is its own container, a table
// needs two adjacent lines inside one container, and a list marker starts a
// new item, so no table can hold the line. No item text on the line or a
// neighbour may read as a delimiter row (only "|", ":", "-" and spaces), so
// the marker never has to be told from a delimiter. The item exemption is
// read only where items is set: the last entry on the latest-run read (rule
// 3c), whose rows the witness check and heldLatestClean read again. Every
// other read keeps a bullet holding "|" as a line it cannot key (#4294).
func heldOneLineProse(lines []string, inFence []bool, i int, items bool) bool {
	if i == 0 || i+1 >= len(lines) || inFence[i] {
		return false
	}
	l := lines[i]
	if items && heldOneLineItem(l) {
		near := func(n string) bool { return strings.Trim(n, " \t") == "" || heldOneLineItem(n) }
		return !inFence[i-1] && !inFence[i+1] && near(lines[i-1]) && near(lines[i+1])
	}
	if l == "" || strings.ContainsRune(l, '\r') || l[0] == '|' || heldContainerStrip(l) != l {
		return false
	}
	return strings.Trim(lines[i-1], " \t") == "" && strings.Trim(lines[i+1], " \t") == ""
}

// heldDelimiterishRe is text a GFM delimiter row could be: only "|", ":",
// "-" and spaces.
var heldDelimiterishRe = regexp.MustCompile(`^[|: \t-]*$`)

// heldOneLineItem reports whether l is a bullet item line at column 0 ("- ",
// "* " or "+ " then text) whose text cannot read as a delimiter row and that
// holds no carriage return (heldOneLineProse).
func heldOneLineItem(l string) bool {
	if len(l) < 3 || strings.IndexByte("-*+", l[0]) < 0 || l[1] != ' ' || strings.ContainsRune(l, '\r') {
		return false
	}
	return !heldDelimiterishRe.MatchString(l[2:])
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

// heldWitnessLines marks each source line of evidence that rule 7 must never
// skip: a line behind a row the witness check reads as a verifyrun witness
// row. The witness check reads rows only after stripRowComments removes the
// complete HTML comments (parseEvidenceRows), so membership is decided on
// that stripped text and mapped back to every source line the stripped row
// came from (flipStrippedSpans). A comment can then neither hide a witness
// shape from the line it sits on nor join two source lines into a witness
// row the scan did not see. The flip path strips the run twice (its run is
// already stripped, then parseEvidenceRows strips it again); when the second
// strip changes the text, or the mapping cannot be rebuilt, the two views
// can differ, so every line is marked: rule 7 is off, fail-closed.
func heldWitnessLines(evidence string) []bool {
	marks := make([]bool, strings.Count(evidence, "\n")+1)
	stripped, _ := stripRowComments(evidence)
	spans, ok := flipStrippedSpans(evidence)
	if again, _ := stripRowComments(stripped); !ok || again != stripped {
		for i := range marks {
			marks[i] = true
		}
		return marks
	}
	for k, row := range strings.Split(stripped, "\n") {
		if isWitnessRow(row) {
			for i := spans[k][0]; i <= spans[k][1]; i++ {
				marks[i] = true
			}
		}
	}
	return marks
}

// heldQuotedOutput reports whether the occurrence at clean[h[0]:h[1]] sits
// inside a double-quoted span (rule 7, #4721). clean is the line as the scan
// reads it: inline code already masked, struck spans cut (the caller passes
// only a line with none). row is true for an aligned results-table data row
// (heldLineRead.key set); alone is true when no other line of the line's
// paragraph (its run of non-blank lines) holds a '"'. The span is read
// FAIL-CLOSED, over every region a renderer could pair the quotes in:
//   - On an aligned results row the region is the cell: the text between the
//     unescaped "|" on either side of the occurrence, or the line's ends, so
//     a span never joins two cells. A table row does not wrap.
//   - On any other line the scan cannot tell a table row from prose holding
//     a "|", so the occurrence must read as inside a span both over the
//     whole line and over its cell. And a paragraph's quotes pair across its
//     soft-wrapped lines: a line that is not alone in its paragraph has no
//     span.
//   - heldQuotedIn reads one region: ASCII quotes only, an escaped or odd
//     quote pairs nothing, flanking, the bare marker word.
//   - A line inlineCodeScan does not mask (a possible link reference
//     definition, whose quoted title is not rendered text; a bare carriage
//     return, which a renderer splits) has no span.
func heldQuotedOutput(clean string, h []int, row, alone bool) bool {
	if hasBareCR(clean) || linkDefLine(clean) {
		return false
	}
	lo, hi := 0, len(clean)
	for i := 0; i < len(clean); i++ {
		if clean[i] == '\\' {
			i++
			continue
		}
		if clean[i] != '|' {
			continue
		}
		if i < h[0] {
			lo = i + 1
		} else if i >= h[1] {
			hi = i
			break
		}
	}
	if row {
		return heldQuotedIn(clean, h, lo, hi)
	}
	return alone && heldQuotedIn(clean, h, lo, hi) && heldQuotedIn(clean, h, 0, len(clean))
}

// heldQuotedIn reports whether the occurrence h lies inside a double-quoted
// span of clean[lo:hi], read fail-closed:
//   - Only the ASCII double quote '"' delimits. A quote inside masked inline
//     code is gone already. Any backslash-escaped quote in the region, or an
//     odd number of quotes in it, leaves the pairing uncertain: no span.
//   - Markup or a character a renderer may not show as a quotation mark
//     leaves it uncertain too: a "<" or "&" or link or image syntax ("](")
//     anywhere in the region, a non-ASCII byte beside a quote, or a quote
//     with white space (CommonMark's set, or the region's edge) on both
//     sides.
//   - Quotes pair in order, first with second, third with fourth. An opening
//     quote right after a letter or digit, or a closing quote right before
//     one (an inch mark: 5"), is not a quotation mark: no span.
//   - The occurrence must lie wholly inside one pair.
//   - A span holding nothing but the marker word (heldBareTokenRe: "HELD",
//     "**could-not-check:**") is the verifier's own status token in quotes,
//     not output, and is not a span here.
func heldQuotedIn(clean string, h []int, lo, hi int) bool {
	var quotes []int
	for i := lo; i < hi; i++ {
		if clean[i] == '\\' && i+1 < hi && clean[i+1] == '"' {
			return false
		}
		if clean[i] == '"' {
			quotes = append(quotes, i)
		}
	}
	if len(quotes)%2 != 0 {
		return false
	}
	// Markup can hide or supply a quote the source pairs: a "<" (an HTML
	// comment or tag around a quote) or a "&" (a character reference such
	// as &quot;) in the region leaves the pairing uncertain, as a "<"
	// leaves inlineCodeScan's.
	if strings.ContainsAny(clean[lo:hi], "<&") {
		return false
	}
	// Link or image syntax hides a quote too: a title in a link or image
	// ("[a](/u '\"')") or a bare destination ("[a](/u/\")") is not rendered
	// as text, so "](" in the region leaves the pairing uncertain.
	if strings.Contains(clean[lo:hi], "](") {
		return false
	}
	// White space is CommonMark's set: a form feed or vertical tab renders
	// as a space, so it flanks a mark as a space does.
	space := func(i int) bool {
		return i < lo || i >= hi || strings.IndexByte(" \t\n\v\f\r", clean[i]) >= 0
	}
	for _, q := range quotes {
		// A quote beside a non-ASCII byte (a no-break or other Unicode
		// space, a curly quote) or with white space on both sides (a spaced
		// ditto or inch mark) may not render as a quotation mark.
		if q > lo && clean[q-1] >= 0x80 || q+1 < hi && clean[q+1] >= 0x80 || space(q-1) && space(q+1) {
			return false
		}
		// An emphasis delimiter beside a quote renders as nothing: in
		// **5**" the quote follows a digit on the page, an inch mark, though
		// the source shows it after "*". A quote beside "*" or "_" gives no
		// span.
		if q > lo && strings.IndexByte("*_", clean[q-1]) >= 0 || q+1 < hi && strings.IndexByte("*_", clean[q+1]) >= 0 {
			return false
		}
	}
	alnum := func(i int) bool {
		if i < lo || i >= hi {
			return false
		}
		c := clean[i]
		return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	inside := false
	for k := 0; k < len(quotes); k += 2 {
		open, closeAt := quotes[k], quotes[k+1]
		if alnum(open-1) || alnum(closeAt+1) {
			return false
		}
		if h[0] > open && h[1] <= closeAt {
			inside = !heldBareTokenRe.MatchString(clean[open+1 : closeAt])
		}
	}
	return inside
}

// heldQuoteNeighbours reports, for each line, whether another line of its
// paragraph (its run of lines that are not blank after trimming) holds an
// ASCII double quote. The raw line is read, code spans and fences included:
// a quote it counts that a renderer would not only withholds a span
// (heldQuotedOutput), so the count errs toward reading the occurrence.
func heldQuoteNeighbours(lines []string) []bool {
	out := make([]bool, len(lines))
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		j, n := i, 0
		for ; j < len(lines) && strings.TrimSpace(lines[j]) != ""; j++ {
			if strings.Contains(lines[j], `"`) {
				n++
			}
		}
		for k := i; k < j; k++ {
			own := 0
			if strings.Contains(lines[k], `"`) {
				own = 1
			}
			out[k] = n-own > 0
		}
		i = j
	}
	return out
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
		regexp.MustCompile(`^Kubernetes-Observation: sha256:[0-9a-f]{64}$`),
		regexp.MustCompile(`^Module-Cache: sha256:[0-9a-f]{64}$`),
		regexp.MustCompile(`^Npm-Staging: sha256:[0-9a-f]{64}$`),
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
// under the rules documented above. Rule 3c is read only when latestRun is
// true, which only the flip-side callers pass (flipHeldRefusal).
func heldScanScope(evidence string, latestRun bool) heldScope {
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
			if !out[i].skip && out[i].key == "" && !heldOneLineProse(lines, inFence, i, latestRun && e == len(starts)-1) {
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
			if latestRun && heldLatestRule && aligned[len(starts)-1] && heldLatestClean(lines, live, out, tables, last) &&
				heldRunDated(lines[last:], inFence[last:]) && !heldBoundaryDoubt(lines, live, inFence, html, last, last, len(lines)) &&
				heldWitnessInLatest(evidence) {
				scope.from, scope.latest = last, true
			}
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

// heldWitnessInLatest reports whether every Date/Runner row of the run
// verifyflip credits for the latest PASS (flipLatestPass, the very selector
// the flip, the reconcile fold and the witness check read) lies after the
// last entry heading of that run. Rule 3c stops reading holds at the last
// entry heading, so it may do so only when no witness the flip counts comes
// from a line before it: a run that shares its date and sha with an earlier
// BLOCKED entry, or one run split under two headings with no verdict between
// them, keeps 3c off and is read by rules 3a and 3b. A refused selection
// keeps 3c off too. Every line that reads as an entry heading counts, live or
// not, fenced or not, indented or quoted, so a context the run text alone
// cannot see can only move the heading later and keep 3c off (fail closed).
//
// The gate reads with the readers it stands in for, never with narrower
// copies of them. p.Run is comment-stripped while 3c reads the rendered
// text, so a stripped span whose markers sit in code the page renders
// literally could hide 3c's heading (or its PASS) from p.Run: a stripped
// line 3c reads as live keeps 3c off (heldStripHidesLive). And the rows
// above the heading are tested with the witness check's own row reader
// (heldCreditedAbove), so a table the check credits on a looser header than
// Date and Runner keeps 3c off too.
// It is a variable only so TestHeldLatestScope can read 3c's other conditions
// with this one held true; the binary never reassigns it.
var heldWitnessInLatest = func(evidence string) bool {
	if heldStripHidesLive(evidence) {
		return false
	}
	p, err := flipLatestPass(evidence)
	if err != nil {
		return false
	}
	// The witness check strips the run's comments again (parseEvidenceRows).
	// When that strip would change the run, a first strip left a new comment
	// that can join text across the heading, so the gate cannot read the
	// prefix the way the witness check reads the run.
	if s, _ := stripRowComments(p.Run); s != p.Run {
		return false
	}
	run := strings.Split(p.Run, "\n")
	h := -1
	for i, l := range run {
		if heldEntryHeading(strings.TrimLeft(l, " \t>")) {
			h = i
		}
	}
	if h < 0 {
		return true
	}
	return !heldCreditedAbove(run[:h], evidence)
}

// heldStripHidesLive reports whether stripping the Evidence's comments
// (stripRowComments, the run reader's first step) removes text from a line
// the held scan reads as live: a span spread over lines whose markers the
// page does not read as a comment. An entry heading or a PASS can leave the
// run only that way, since a span on one line never holds a line start. A
// complete comment the page hides, such as the section template's note,
// touches no live line and changes nothing.
func heldStripHidesLive(evidence string) bool {
	lines := strings.Split(evidence, "\n")
	live := heldRender(lines)
	starts := make([]int, len(lines))
	for i, off := 1, 0; i < len(lines); i++ {
		off += len(lines[i-1]) + 1
		starts[i] = off
	}
	lineAt := func(off int) int {
		return sort.Search(len(starts), func(i int) bool { return starts[i] > off }) - 1
	}
	for _, span := range htmlCommentClosedRe.FindAllStringIndex(evidence, -1) {
		first, last := lineAt(span[0]), lineAt(span[1]-1)
		if first == last {
			continue
		}
		for i := first; i <= last; i++ {
			if live[i] {
				return true
			}
		}
	}
	return false
}

// heldCreditedAbove reports whether the lines above the run's last entry
// heading hold any row a witness reader the flip runs could credit: a
// Date/Runner row (flipDateRunnerRows), or any Evidence row the witness
// check reads as a witness (parseEvidenceRows, isWitnessRow), whatever its
// table's header says. A protected-table marker anywhere hands the witness
// check the full record (closureRunWitnesses); the protected audit then reads
// witnesses only from the run flipLatestPass selects, the run this gate reads,
// whenever the record holds a verdict (protectedSelectsRun). Only a protected
// record it would read whole keeps 3c off on the marker alone (#5541).
func heldCreditedAbove(above []string, evidence string) bool {
	if hasProtectedTableMarker(evidence) && !protectedSelectsRun(evidence) {
		return true
	}
	if rows, _ := flipDateRunnerRows(above); len(rows) > 0 {
		return true
	}
	for _, rows := range parseEvidenceRows(strings.Join(above, "\n")) {
		for _, r := range rows {
			if isWitnessRow(r.Text) {
				return true
			}
		}
	}
	return false
}

// heldLatestRule turns rule 3c on. It is always true in the binary; it is a
// variable only so the tests of rules 3a and 3b can read those rules alone, as
// they still decide every Evidence whose last entry 3c does not admit.
var heldLatestRule = true

// heldFenced reports, for each line, whether the scan reads it as fenced code:
// a fence toggle line (heldFenceToggle) or a line between two of them.
func heldFenced(lines []string) []bool {
	in := make([]bool, len(lines))
	fence := false
	for i, l := range lines {
		if heldFenceToggle(l) {
			fence = !fence
			in[i] = true
			continue
		}
		in[i] = fence
	}
	return in
}

// heldRunDated reports whether the run lines (one Evidence entry) record at
// least one Date/Runner row and every such row carries both cells
// (flipRowDated, the predicate verifyflip's run reader applies): a run with a
// missing Date or Runner cell is not a PASS run (rule 3c). Fenced code is not
// read, HTML comments are removed first, and an unterminated comment reads as
// undated.
func heldRunDated(lines []string, inFence []bool) bool {
	keep := make([]string, len(lines))
	for i, l := range lines {
		if !inFence[i] {
			keep[i] = l
		}
	}
	text, open := stripRowComments(strings.Join(keep, "\n"))
	if open >= 0 {
		return false
	}
	rows, _ := flipDateRunnerRows(strings.Split(text, "\n"))
	if len(rows) == 0 {
		return false
	}
	for _, r := range rows {
		if !flipRowDated(r) {
			return false
		}
	}
	return true
}

// heldBoundaryDoubt reports whether the entry heading on line at, a heading
// the scan renders live, may not start an entry on the page (rule 3c), with
// lines [lo, hi) the run that heading is to bound. It is local to that one
// heading: a column-0 level-three heading in [lo, hi) outside fenced code and
// HTML comments that the scan does not render live (a carriage return before
// it, an HTML block around it, a fence the scan and the page disagree on), or,
// in the lines before at, an HTML block or tag left open (heldHTMLUnclosed) or
// a fence inside a container (heldContainerFence), either of which may end
// where the scan does not and so swallow the heading. A column-0 heading is
// never a continuation of a list item or a blockquote, so it ends any
// container, and any HTML block inside one, before it: a tag start in a
// container (the rest of heldContainerDoubt) cannot reach it. Before at,
// an element that can strike, hide or swallow what follows it across blocks
// (heldCarryElement) must also be closed. Any other element left open
// earlier (the rest of heldElementDoubt) only nests the heading in the page's
// tree, where it still renders as a heading, so it is not read here.
func heldBoundaryDoubt(lines []string, live, inFence []bool, html []int, at, lo, hi int) bool {
	if at <= 0 || at >= len(lines) || !live[at] {
		return true
	}
	for i := max(lo, 1); i < hi && i < len(lines); i++ {
		if !live[i] && !inFence[i] && html[i] != heldHTMLComment && heldEntryHeading(lines[i]) {
			return true
		}
	}
	return heldHTMLUnclosed(lines[:at], inFence[:at]) || heldContainerFence(lines[:at]) ||
		heldCarryElement(lines[:at], inFence[:at])
}

// heldCarryTags are the elements whose effect the page carries past the block
// that opens them: the formatting elements an HTML parser reopens in every
// later block while they stay open (an unclosed <s> strikes everything after
// it, a PASS included), and the elements that collapse or drop their content.
var heldCarryTags = map[string]bool{}

func init() {
	for _, t := range strings.Fields(`a b big code em font i nobr s small strike
		strong tt u del ins details template`) {
		heldCarryTags[t] = true
	}
}

// heldCarryElement reports whether lines, outside top-level fenced code, open
// a heldCarryTags or heldRawTags element more often than they close it. It
// reads the lines as heldCarryView leaves them, which hides only the two
// places the page can never open such an element from (#7578): a certain
// inline code span, and a carried element inside a cell of a verifyrun
// witness table. Anywhere else an opener counts wherever it appears (in prose,
// in an uncertain code span, in a comment too), so this over-refuses by
// design; a placeholder such as <name> names no such element and is not read.
func heldCarryElement(lines []string, inFence []bool) bool {
	lines = heldCarryView(lines, inFence)
	open := map[string]int{}
	for i, l := range lines {
		if inFence[i] {
			continue
		}
		for p := 0; p+1 < len(l); p++ {
			if l[p] != '<' {
				continue
			}
			if isASCIILetter(l[p+1]) {
				if n := heldElementName(l[p+1:]); heldCarryTags[n] || heldRawTags[n] {
					open[n]++
				}
				continue
			}
			if n, ok := heldCloseTag(l[p:]); ok && open[n] > 0 {
				open[n]--
			}
		}
	}
	for _, c := range open {
		if c > 0 {
			return true
		}
	}
	return false
}

// heldCellEscapeTags are the elements whose tags, in a table cell, can carry
// what follows out of that cell: a table-structure tag closes the cell or the
// table early, so the rest of the row renders outside it; a template's end
// tag is ignored there, so the template swallows the rest of the page; a
// select switches the parser to its own mode; and object, applet and marquee
// each push their own active-formatting marker, so closing the cell clears
// the list only back to that inner marker and a formatting element opened
// before it in the cell is reopened after the table. A witness row holding
// any of them, as an opener or a closer, anywhere on the line, is not a
// contained cell (heldCarryView).
var heldCellEscapeTags = map[string]bool{}

// heldWitnessDelims maps each verifyrun witness table header line
// (heldWitnessHeaders) to the delimiter row verifyrun writes under it.
var heldWitnessDelims = map[string]string{}

func init() {
	for _, t := range strings.Fields(`table caption colgroup col tbody thead tfoot
		tr td th template select object applet marquee`) {
		heldCellEscapeTags[t] = true
	}
	for _, h := range []string{witnessHeader, witnessHeaderFailFirst} {
		parts := strings.SplitN(h, "\n", 2)
		heldWitnessDelims[parts[0]] = parts[1]
	}
}

// heldCarryView returns lines as heldCarryElement reads them (#7578). Fenced
// lines are returned as they are; heldCarryElement skips them itself. On
// every other line two things are hidden, each read fail-closed:
//   - A certain inline code span (inlineCodeScan, the masker the held scan
//     uses, driven over the same lines in the same order) is overwritten byte
//     for byte, on a line the page certainly renders as block text
//     (heldRender) only. Markdown renders a code span's text as text, never
//     as a tag, so "<a ...>" inside one opens nothing. A span the masker
//     cannot settle (an unpaired backtick, a "<" or link tail before it, a
//     paragraph left open by an earlier line) is not hidden, and a line the
//     page may read as part of an HTML block is read raw.
//   - In a row of a verifyrun witness table, identified exactly, the "<" of
//     each heldCarryTags opener and closer is hidden. Each cell renders as a
//     td. When a td closes, an HTML parser pops every element opened inside
//     it (details, del and ins included) and clears the active formatting
//     elements back to the last marker. That marker is the td's own unless
//     the cell opened an element that pushes one: td, th, caption, template,
//     object, applet or marquee, all in heldCellEscapeTags. In a row holding
//     none of them, no such element can reach past its cell, and a closer
//     there cannot close one opened outside it either. The table is exact: a
//     header line byte for byte in heldWitnessHeaders at the first line or
//     after a blank line, verifyrun's own delimiter row under it, and every
//     line from there to the row a witness row (isWitnessRow) with at least
//     as many cells as the header, starting "|" at column 0, rendered as
//     block text, with no carriage return. The page can end a table at a
//     line the scan cannot follow: a blank line, a line that starts another
//     block, a row with no cell ("|" alone), and, past a cap on cells padded
//     into short rows, a short row. So the first line that is not such a row
//     ends the exemption for the rest of the table; a row verifyrun did not
//     write is not skipped. A row holding any heldCellEscapeTags tag is not
//     exempt, and ends the exemption too. heldRawTags openers in a row still
//     count.
//
// Everything else, prose and table cells written by hand included, is read
// as before.
func heldCarryView(lines []string, inFence []bool) []string {
	out := make([]string, len(lines))
	live := heldRender(lines)
	var code inlineCodeScan
	// witness is 0 outside a verifyrun witness table, 1 on its delimiter
	// row, 2 on its data rows; cols is the table's header column count.
	witness, cols := 0, 0
	for i, l := range lines {
		out[i] = l
		if inFence[i] {
			if heldFenceToggle(l) {
				code.fenceLine(l)
			}
			witness = 0
			continue
		}
		if strings.TrimSpace(l) == "" {
			if isBlankLine(l) {
				code.blank()
			}
			witness = 0
			continue
		}
		masked := code.mask(l)
		if live[i] {
			out[i] = masked
		}
		if heldWitnessHeaders[l] && (i == 0 || isBlankLine(lines[i-1])) && i+1 < len(lines) &&
			!inFence[i+1] && lines[i+1] == heldWitnessDelims[l] && live[i] && live[i+1] {
			witness, cols = 1, strings.Count(l, "|")-1
			continue
		}
		if witness == 1 { // the delimiter row, matched with its header
			witness = 2
			continue
		}
		if witness == 0 || !live[i] || !strings.HasPrefix(l, "|") || hasBareCR(l) || heldCellEscape(l) {
			witness = 0
			continue
		}
		if c := witnessCells(l); c == nil || len(c) < cols {
			witness = 0 // the page may end the table at a row verifyrun did not write
			continue
		}
		b := []byte(out[i])
		for p := 0; p+1 < len(l); p++ {
			if l[p] != '<' {
				continue
			}
			if isASCIILetter(l[p+1]) && heldCarryTags[heldElementName(l[p+1:])] {
				b[p] = inlineCodeMask
			} else if n, ok := heldCloseTag(l[p:]); ok && heldCarryTags[n] {
				b[p] = inlineCodeMask
			}
		}
		out[i] = string(b)
	}
	return out
}

// heldCellEscape reports whether line holds an opener or a closer of a
// heldCellEscapeTags element anywhere, code spans included.
func heldCellEscape(line string) bool {
	for p := 0; p+1 < len(line); p++ {
		if line[p] != '<' {
			continue
		}
		if isASCIILetter(line[p+1]) && heldCellEscapeTags[heldElementName(line[p+1:])] {
			return true
		}
		if n, ok := heldCloseTag(line[p:]); ok && heldCellEscapeTags[n] {
			return true
		}
	}
	return false
}

// heldContainerFence reports whether any line that carries a blockquote or
// list-item marker or leading whitespace (heldContainerStrip) has a fence
// after them: the page ends such a fence with its container, where the scan's
// fence reading (heldFenced) may not, so the two may disagree on every fence
// after it.
func heldContainerFence(lines []string) bool {
	for _, l := range lines {
		if t := heldContainerStrip(l); t != l && heldFenceToggle(t) {
			return true
		}
	}
	return false
}

// heldRunFrom returns the first line of the run a verdict on line at closes,
// given lo, the first line after the verdict before it (verifyflip's run
// boundary): the latest live entry heading in [lo, at), so that an earlier
// entry's tail is never read as part of the latest run, or lo when no entry
// heading lies there. ok is false when a heading lies there but the boundary
// is ambiguous (heldBoundaryDoubt): the caller refuses rather than guess.
func heldRunFrom(lines []string, lo, at int) (from int, ok bool) {
	live := heldRender(lines)
	inFence := heldFenced(lines)
	html := heldHTMLLines(lines, inFence)
	from, inside, found := lo, false, false
	for i := max(lo, 1); i < at && i < len(lines); i++ {
		if inFence[i] || html[i] == heldHTMLComment || !heldEntryHeading(lines[i]) {
			continue
		}
		inside = true
		if live[i] {
			from, found = i, true
		}
	}
	if !inside {
		return lo, true
	}
	if !found || heldBoundaryDoubt(lines, live, inFence, html, from, lo, at) {
		return 0, false
	}
	return from, true
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
	clean, bad, ok := heldCoverRows(lines, live, out, tables, last)
	if !ok {
		return nil
	}
	covered := map[string]bool{}
	for k := range clean {
		if !bad[k] {
			covered[k] = true
		}
	}
	return covered
}

// heldLatestClean reports whether every keyed results row of the last entry
// (from line last on) reads clean by heldCovered's reading and there is at
// least one: a latest run with a row the cover cannot read as a pass, or a
// row it cannot align, is not a run that answers everything before it
// (rule 3c).
func heldLatestClean(lines []string, live []bool, out []heldLineRead, tables []heldTable, last int) bool {
	clean, bad, ok := heldCoverRowsAs(lines, live, out, tables, last, true)
	if !ok || len(clean) == 0 || len(bad) > 0 {
		return false
	}
	// The scan never reads a key cell, so a hold written into one ("3 (HELD)",
	// "3 (positive not run)") would go unseen once nothing before the entry
	// is read. Such a key keeps 3c off; it decides nothing else.
	for k := range clean {
		if heldLatestKeyHoldRe.MatchString(k) || heldObserveHoldRe.MatchString(k) {
			return false
		}
	}
	return true
}

// heldLatestKeyHoldRe is a hold word in a row key of the last entry
// (heldLatestClean), beside heldObserveHoldRe's words. It only turns rule 3c
// off, never refuses on its own, so it is not a held-scan site.
var heldLatestKeyHoldRe = regexp.MustCompile(`(?i)\b(?:held|hold\w*|could-not-\w+|deferr\w*)\b`)

// heldCoverRows sorts the keyed results rows of the last entry's tables into
// the keys that re-ran clean and the keys some row of which did not
// (heldCovered). ok is false when a row cannot be aligned.
func heldCoverRows(lines []string, live []bool, out []heldLineRead, tables []heldTable, last int) (clean, bad map[string]bool, ok bool) {
	return heldCoverRowsAs(lines, live, out, tables, last, false)
}

// heldCoverRowsAs is heldCoverRows; corroborate is true only for rule 3c
// (heldLatestClean), which reads a noted row as clean when a tool-written
// witness row of the same key, in the same entry, already reads clean
// (heldCorroboratedNote, #6767). Rules 3a and 3b never pass it.
func heldCoverRowsAs(lines []string, live []bool, out []heldLineRead, tables []heldTable, last int, corroborate bool) (clean, bad map[string]bool, ok bool) {
	clean, bad = map[string]bool{}, map[string]bool{}
	// witnessed: the keys with a clean row whose Result cell is the whole
	// tool-written pass (heldWitnessPassRe) and whose other outcome cells
	// read clean, in a table the page surely draws.
	witnessed := map[string]bool{}
	type noted struct {
		key  string
		ok   bool // drawn surely, and every outcome cell is empty, clean or a corroborable note
		seen bool // at least one such cell opens on a clean outcome
	}
	var notes []noted
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
				return nil, nil, false // a row that cannot be aligned (fail closed)
			}
			cells := heldCellRanges(lines[r])
			ran, other, tool := false, false, false
			nt := noted{key: out[r].key, ok: verified}
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
				cell := lines[r][rg[0]:rg[1]]
				st := heldOutcome(cell, role == heldColExit)
				if role == heldColOther && st != heldCellEmpty {
					st = heldCellOther
				}
				switch st {
				case heldCellPass:
					ran, nt.seen = true, true
					if role == heldColResult && heldWitnessPassRe.MatchString(strings.TrimSpace(cell)) {
						tool = true
					}
				case heldCellOther:
					other = true
					if role == heldColOther || !heldCorroboratedNote(cell) {
						nt.ok = false
					} else {
						nt.seen = true
					}
				}
			}
			if verified && ran && !other {
				clean[out[r].key] = true
				if tool {
					witnessed[out[r].key] = true
				}
			} else {
				bad[out[r].key] = true
				notes = append(notes, nt)
			}
		}
	}
	if !corroborate {
		return clean, bad, true
	}
	// A bad key turns clean only when every bad row of it is a corroborated
	// note and the same key has a clean tool-written witness row.
	fixed := map[string]bool{}
	for _, nt := range notes {
		if _, done := fixed[nt.key]; !done {
			fixed[nt.key] = true
		}
		fixed[nt.key] = fixed[nt.key] && nt.ok && nt.seen && witnessed[nt.key]
	}
	for k, v := range fixed {
		if v {
			delete(bad, k)
			clean[k] = true
		}
	}
	return clean, bad, true
}

// heldNoteContrastRe is a word in a noted outcome cell that reads as a
// failure, a doubt or a contrast (heldCorroboratedNote), beside
// heldObserveHoldRe's hold words and heldLatestKeyHoldRe's markers.
var heldNoteContrastRe = regexp.MustCompile(`(?i)\b(?:errors?|err|wrong|mismatch\w*|broken|break\w*|panic\w*|timeouts?|timed|regress\w*|missing|absent|differ\w*|unexpected\w*|flak\w*|but|however|except\w*|although|though|yet|not|no|never|none|without|only|instead|unless|warn\w*|refused|denied|crash\w*|abort\w*|exit[ \t]*[:=]?[ \t]*[1-9][0-9]*|rc[ \t]*[:=]?[ \t]*[1-9][0-9]*)\b|❌|✗|✘|⚠`)

// heldCorroboratedNote reports whether cell, an outcome cell the closed
// grammar (heldOutcome) reads as other, is a clean outcome with a note that
// rule 3c may read as clean once a tool-written witness row of the same key
// in the same entry reads clean (heldCoverRowsAs; #6767). A verifier's
// hand-written restatement of a witness row ("pass exit=0, prints --- PASS:
// TestX (sha256:…)") then no longer turns the run it restates into one that
// answers nothing. The cell must open on a recognised clean outcome
// (heldOutcomeRe) and hold no hold or fail word (heldObserveHoldRe,
// heldLatestKeyHoldRe, heldNoteContrastRe), no markup, entity, escape,
// strike or tilde that could render as something else or hide text, and no
// control or format character. A note column (heldColOther) is never read
// this way: it must stay empty, as rule 3a reads it.
func heldCorroboratedNote(cell string) bool {
	if strings.ContainsAny(cell, "<>&\\~") {
		return false
	}
	for _, r := range cell {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	n := strings.Trim(heldNorm(cell), "`*_ ")
	if heldOutcomeRe.FindString(n) == "" {
		return false
	}
	return !heldObserveHoldRe.MatchString(n) && !heldLatestKeyHoldRe.MatchString(n) && !heldNoteContrastRe.MatchString(n)
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
	// heldWitnessPassRe is the whole Result cell statusgen verifyrun writes
	// for a passing witness row (witness.row): the state token pass, the
	// observed exit code, and the optional sandbox token (#5541). The pass
	// is verifyrun's own verdict against the row's Expect cell, so an Expect
	// that names a non-zero exit ("exit 2") records "pass exit=2". The exit
	// code after pass is the observed code, not a second outcome, so the
	// cell reads as clean whatever the code is. A bare "pass" already reads
	// clean, so this admits no verdict the grammar did not. The cell is
	// compared as the tool writes it, trimmed of space only: "PASS exit=2",
	// "passed exit=2", a note after it, or "fail exit=0" stay outside it and
	// are read by heldOutcomeRe's closed grammar as before.
	heldWitnessPassRe = regexp.MustCompile(`^pass exit=[0-9]+(?: sandbox=(?:unshare|container-netns))?$`)
	// heldDigestRe is the Output cell statusgen verifyrun writes for every
	// witness row: the first 12 hex digits of the sha256 of the row's captured
	// output (hashOutput), never caller text.
	heldDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{12}$`)
	// heldBaseCellRe is the Base cell a fail-first verifyrun run writes for
	// every witness row (failFirstCell): the base SHA or "-", the base run's
	// state (an exit code only after red or green), the head tree as treeSHA
	// renders it and, after red or green, the delivering commit when the run
	// named one (#2721). It records the run at the merge-base, never the
	// head outcome, and none of it is caller text.
	heldBaseCellRe = regexp.MustCompile(`^base=(?:[0-9a-f]{7,40}|-) (?:(?:red|green) rc=(?:-|[0-9]+) · head=(?:[0-9a-f]{7,40}(?:\+dirty|\+unknown)?|no-git)(?: · delivering=[0-9a-f]{7,40})?(?: output=sha256:[0-9a-f]{64} log=row-[0-9]{6}\.log)?|(?:unproven|not-selected) · head=(?:[0-9a-f]{7,40}(?:\+dirty|\+unknown)?|no-git))$`)
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
	if !exit && heldWitnessPassRe.MatchString(strings.TrimSpace(cell)) {
		return heldCellPass
	}
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

// Source locators in a derivation can cite individual lines, ranges, or a
// second named source. They delimit the declaration and its rationale.
const heldRiskLine = `[1-9][0-9]*(?:-[1-9][0-9]*)?`
const heldRiskSource = `[a-zA-Z0-9_./-]+:` + heldRiskLine + `(?:, *` + heldRiskLine + `)*`
const heldRiskWord = `[a-z][a-z-]*`
const heldRiskCodePair = `[0-9]+ ` + heldRiskWord + `(?: or ` + heldRiskWord + `)?`

var (
	heldRiskRecordRe     = regexp.MustCompile(`^(?:[-*+] )?RISK-VALUE: DERIVED — ([^@|\r\n]+) @ ` + heldRiskSource + `(?: and [a-z]+(?: [a-z]+)* @ ` + heldRiskSource + `)* — `)
	heldRiskExitCodesRe  = regexp.MustCompile(`^exit codes = ([0-9]+(?:, *[0-9]+)+)$`)
	heldRiskPositionalRe = regexp.MustCompile(`^(?:the )?(?:spec|contract) maps (` + heldRiskWord + `(?:/` + heldRiskWord + `)+) respectively\b`)
	heldRiskNumberedRe   = regexp.MustCompile(`^(?:the )?(?:brief's |spec's |contract's )?(?:exit-code )?table: (` + heldRiskCodePair + `(?:, ` + heldRiskCodePair + `)+)(?:[.;]|$)`)
	heldRiskStateSetRe   = regexp.MustCompile(`^settled states = "` + heldRiskWord + `"(?:, "` + heldRiskWord + `")+$`)
	heldRiskStateListRe  = regexp.MustCompile(`\b` + heldRiskWord + `(?:, ` + heldRiskWord + `)* or ` + heldRiskWord + ` states stay presented\b`)
	heldRiskLiteralRe    = regexp.MustCompile(`^` + heldRiskStateDecl + `(?:(?:, | and )` + heldRiskStateDecl + `)*$`)
	heldRiskMeaningRe    = regexp.MustCompile(`(?i)\bmissing and could-not-check cannot release assessment\b|\ba timeout maps to could-not-check, never to permitted\b|\bretirement may remain held separately\b|\bcomplete/retirement-held (?:must|to) stay distinct from complete\b`)
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
	declaration := clean[record[2]:record[3]]
	if h[1] <= record[3] {
		// The whole declaration list must match: accepting a substring
		// would hide an observation before or beside a valid constant.
		return heldRiskLiteralRe.MatchString(declaration)
	}
	meaning := clean[record[1]:]
	if h[0] < record[1] || heldNamesRowRe.MatchString(meaning) {
		return false
	}
	start, end := h[0]-record[1], h[1]-record[1]
	if declared := heldRiskExitCodesRe.FindStringSubmatch(declaration); declared != nil &&
		heldRiskExitMeaning(strings.Split(declared[1], ","), meaning, start, end) {
		return true
	}
	return heldRiskPhrase(heldRiskMeaningRe, meaning, start, end) ||
		(heldRiskStateSetRe.MatchString(declaration) && heldRiskPhrase(heldRiskStateListRe, meaning, start, end))
}

func heldRiskPhrase(pattern *regexp.Regexp, meaning string, start, end int) bool {
	for _, span := range pattern.FindAllStringIndex(meaning, -1) {
		if span[0] <= start && end <= span[1] {
			return true
		}
	}
	return false
}

// An ordered list supplies one outcome per declared exit code. An explicit
// table must repeat those same codes in order. Only occurrences within the
// mapping itself are definitions; later observations still count as holds.
func heldRiskExitMeaning(codes []string, meaning string, start, end int) bool {
	if m := heldRiskPositionalRe.FindStringSubmatchIndex(meaning); m != nil {
		return m[2] <= start && end <= m[3] && strings.Count(meaning[m[2]:m[3]], "/")+1 == len(codes)
	}
	m := heldRiskNumberedRe.FindStringSubmatchIndex(meaning)
	if m == nil || start < m[2] || end > m[3] {
		return false
	}
	pairs := strings.Split(meaning[m[2]:m[3]], ", ")
	if len(pairs) != len(codes) {
		return false
	}
	for i, pair := range pairs {
		code, _, _ := strings.Cut(pair, " ")
		if code != strings.TrimSpace(codes[i]) {
			return false
		}
	}
	return true
}

// heldNamesRowNumRe matches a line naming a numbered Verify row: "row 3",
// "Row3", "rows 2", "R3" (rule 6, #4721 edge 2a). A bare "#3" is not a row
// name here: it is as often an issue or PR reference.
var heldNamesRowNumRe = regexp.MustCompile(`(?i)\b(?:rows?[ \t]*\d+|r\d+)\b`)

// heldVerdictLineRe matches a verdict line, a RISK-VALUE or VERIFY verdict
// (rule 6). A verdict states the entry's own disposition, so it is read at
// any case: "VERIFY: BLOCKED — 3/4 pass, 1 could-not-check" stays a hold.
var heldVerdictLineRe = regexp.MustCompile(`\b(?:RISK-VALUE|VERIFY):`)

// heldProseLine reports whether a line is prose for rule 6: neither a
// results-table row nor any other table line. A line carrying "|" anywhere,
// or an HTML table tag, is read as before (fail closed).
func heldProseLine(line string, read heldLineRead) bool {
	return read.key == "" && len(read.blank) == 0 && !strings.Contains(line, "|") && !heldHTMLTableRe.MatchString(line)
}

// heldProseSignal reports whether the state-word occurrence h of a prose
// line clean is a structured signal rule 6 reads (#4721, ruling A): a HELD
// marker (the word is not all lower case, edge 1a-i), a row result (the
// line names a row, edge 2a) or a verdict line (RISK-VALUE or VERIFY). Anything else,
// a step said to be held included (edge 3a), is free prose and not a hold.
func heldProseSignal(clean string, h []int) bool {
	if w := clean[h[0]:h[1]]; strings.ToLower(w) != w {
		return true
	}
	return heldNamesRowNumRe.MatchString(clean) || heldVerdictLineRe.MatchString(clean)
}
