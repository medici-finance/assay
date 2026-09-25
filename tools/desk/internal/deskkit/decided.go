package deskkit

// decided.go — the shared parse/render for the desk-decided block: the PR-body record of a
// reversible default a desk took without asking first (attention-budget/19). ONE shared
// implementation, because deskpr WRITES the block, deskflip READS it (label/block agreement,
// and the reviewer's undeclared-decision finding), and a future deskfile notice lane on
// issues (attention-budget/13, out of scope here) reads the SAME marker — a second
// hand-rolled parser is exactly how the two surfaces would drift.
//
// ONE TERM, ONE LABEL, ONE MARKER. The term is "desk-decided"; the label is
// DeskDecidedLabel (the counterpart of the existing `human-decided`, which is refused for
// every role — see desklabel's vocabulary); the machine marker is DeskDecidedMarker, the
// EXISTING `<!-- desk-r3-decision v1 -->` the weekly digest already reads on issue comments
// (deskdigest/collect.go's r3MarkerRe) — reused verbatim, not a second word.
//
// THE BLOCK'S SHAPE. A fixed heading, the marker, then a numbered list; each item carries
// exactly three fields — decision:, alternative:, cost: — one per line. The per-line
// `decision:` anchor is deliberately the SAME shape deskdigest's r3DecisionRe already
// expects (`^[ \t>*_-]*decision:[ \t]*(.+)$`), so a future reader over this surface is not a
// third format.
//
// WHAT THIS FILE DOES NOT DECIDE. It has no opinion on WHO may write the block, WHEN a
// missing block should refuse a flip, or how the weekly digest counts it — those are
// deskpr's, deskflip's and deskdigest's own call sites (attention-budget/17 for the last).
// This file only makes "parse a block" and "render a block" answerable once.

import (
	"fmt"
	"regexp"
	"strings"
)

// DeskDecidedHeading is the fixed PR-body heading the block is written and read back under.
const DeskDecidedHeading = "## Desk-decided"

// DeskDecidedMarker is the machine-readable marker — the existing `desk-r3-decision v1`
// marker deskdigest already reads on issue comments (attention-budget/13's notice lane),
// reused verbatim here so issues and PRs never carry two markers for the same term.
const DeskDecidedMarker = "<!-- desk-r3-decision v1 -->"

// DeskDecidedLabel is the PR label — the counterpart of the existing `human-decided` label
// (which desklabel's vocabulary refuses for every role, since it records a human act; this
// one records a DESK act, and deskpr applies it directly through LabelChange, the same
// create-if-missing path deskflip's queue-label swap and deskpost's verdict labels already
// use — it is not part of desklabel's closed per-role vocabulary).
const DeskDecidedLabel = "desk-decided"

// DeskDecidedLabelColor and DeskDecidedLabelDescription are the ONE spec the desk-decided
// label is created with on first use, by whichever writer reaches a repo first — deskpr on a
// PR, or deskfile's notice lane on an issue. Two specs would make the label's look depend on
// which tool happened to create it.
const (
	DeskDecidedLabelColor       = "5319e7"
	DeskDecidedLabelDescription = "A desk took a reversible default here — see the item's Desk-decided block"
)

// DecidedItem is one declared desk decision: what was chosen, the alternative not taken,
// and what reversing it costs the driver.
type DecidedItem struct {
	Decision    string
	Alternative string
	Cost        string
}

// decidedItemStart matches the start of one numbered item: "N. decision: <text>". Every
// other field of the item (alternative:, cost:) is read from the text between this item's
// start and the next one (or end of input).
var decidedItemStart = regexp.MustCompile(`(?m)^[ \t]*(\d+)\.[ \t]*decision:[ \t]*(.*)$`)

// decidedAltRe / decidedCostRe read the other two fields, anywhere in an item's chunk —
// the same indentation-tolerant, line-anchored shape decidedItemStart uses.
var (
	decidedAltRe  = regexp.MustCompile(`(?im)^[ \t]*alternative:[ \t]*(.*)$`)
	decidedCostRe = regexp.MustCompile(`(?im)^[ \t]*cost:[ \t]*(.*)$`)
)

// ParseDecidedItems parses a `--decided` input FILE (or the list portion of an already
// rendered block): a numbered list, each item carrying exactly three fields — decision:,
// alternative:, cost:. It is the one place that REFUSES an empty list or an item missing a
// field; the exit-5 contract lives at deskpr's call site, this only reports what is wrong.
func ParseDecidedItems(raw []byte) ([]DecidedItem, error) {
	text := string(raw)
	locs := decidedItemStart.FindAllStringSubmatchIndex(text, -1)
	if len(locs) == 0 {
		return nil, fmt.Errorf("no numbered decision items found — each item starts `N. decision: <text>` " +
			"(an empty --decided file is refused, not treated as \"no decision\")")
	}
	items := make([]DecidedItem, 0, len(locs))
	for i, loc := range locs {
		start := loc[0]
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		chunk := text[start:end]
		num := text[loc[2]:loc[3]]
		decision := strings.TrimSpace(text[loc[4]:loc[5]])

		altM := decidedAltRe.FindStringSubmatch(chunk)
		costM := decidedCostRe.FindStringSubmatch(chunk)

		var missing []string
		if decision == "" {
			missing = append(missing, "decision")
		}
		if altM == nil || strings.TrimSpace(altM[1]) == "" {
			missing = append(missing, "alternative")
		}
		if costM == nil || strings.TrimSpace(costM[1]) == "" {
			missing = append(missing, "cost")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("item %s is missing field(s): %s — every item needs decision:, "+
				"alternative: and cost:", num, strings.Join(missing, ", "))
		}
		items = append(items, DecidedItem{
			Decision:    decision,
			Alternative: strings.TrimSpace(altM[1]),
			Cost:        strings.TrimSpace(costM[1]),
		})
	}
	return items, nil
}

// RenderDecidedBlock renders the full PR-body section: the fixed heading, the marker, then
// the numbered list — the ONE shape deskpr writes and deskflip reads back. Callers pass
// items already validated by ParseDecidedItems; this never refuses.
func RenderDecidedBlock(items []DecidedItem) string {
	var b strings.Builder
	b.WriteString(DeskDecidedHeading)
	b.WriteString("\n")
	b.WriteString(DeskDecidedMarker)
	b.WriteString("\n\n")
	for i, it := range items {
		fmt.Fprintf(&b, "%d. decision: %s\n", i+1, it.Decision)
		fmt.Fprintf(&b, "   alternative: %s\n", it.Alternative)
		fmt.Fprintf(&b, "   cost: %s\n", it.Cost)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// headingLineRe matches ANY Markdown ATX heading line (one to six `#`s, a space, then
// text) — the boundary findDeskDecidedSection stops at.
var headingLineRe = regexp.MustCompile(`^#{1,6}[ \t]`)

// findDeskDecidedSection locates the "## Desk-decided" section within body: from the start
// of the heading LINE through, but excluding, the next heading line of any level, or end of
// body. found=false means no such heading line exists at all (matched as a whole trimmed
// line, so a heading that merely CONTAINS the text — "### Not Desk-decided really" — never
// matches).
//
// Lines inside a fenced code block (``` or ~~~, the same delimiter test the verdict-marker
// readers use) are never a heading in either role: a PR body that QUOTES an example block is
// documentation, not a declaration, so a fenced `## Desk-decided` neither starts a section nor
// a fenced `#` line ends one (review advisory N1 on attention-budget/19's PR). Skipping the
// fence is safe in this direction: the block is a DECLARATION (a grant-shaped record), and
// every refusal deskflip derives from it — malformed, or label/block disagreement — reads the
// same real, unfenced section it always did.
func findDeskDecidedSection(body string) (start, end int, found bool) {
	lines := strings.SplitAfter(body, "\n")
	pos := 0
	headingStart := -1
	inFence := false
	for _, ln := range lines {
		trimmed := strings.TrimRight(ln, "\n\r")
		if isFenceDelimiter(trimmed) {
			inFence = !inFence
			pos += len(ln)
			continue
		}
		if inFence {
			pos += len(ln)
			continue
		}
		if headingStart < 0 {
			if strings.TrimSpace(trimmed) == DeskDecidedHeading {
				headingStart = pos
			}
		} else if headingLineRe.MatchString(trimmed) {
			return headingStart, pos, true
		}
		pos += len(ln)
	}
	if headingStart >= 0 {
		return headingStart, len(body), true
	}
	return 0, 0, false
}

// HasDeskDecidedHeading reports whether body already carries a "## Desk-decided" heading
// line anywhere. `deskpr create` refuses a --decided write onto a body that already carries
// one (a hand-written block, or a copy of the retired interim comment form pasted into the
// body) rather than silently duplicating or shadowing it.
func HasDeskDecidedHeading(body string) bool {
	_, _, found := findDeskDecidedSection(body)
	return found
}

// ReplaceOrAppendDeskDecidedBlock returns body with its "## Desk-decided" section replaced
// by block, IN PLACE, when one is present — `deskpr edit --decided`'s path — or block
// appended at the end (preceded by a blank line) when absent — `deskpr create --decided`'s
// path, once the create-time refusal above has already ruled out a hand-written one.
func ReplaceOrAppendDeskDecidedBlock(body, block string) string {
	block = strings.TrimRight(block, "\n") + "\n"
	start, end, found := findDeskDecidedSection(body)
	if found {
		before := body[:start]
		after := body[end:]
		before = strings.TrimRight(before, "\n")
		if before != "" {
			before += "\n\n"
		}
		if after != "" && !strings.HasPrefix(after, "\n") {
			after = "\n" + after
		}
		return before + block + after
	}
	trimmed := strings.TrimRight(body, "\n")
	if trimmed == "" {
		return block
	}
	return trimmed + "\n\n" + block
}

// ParseDeskDecidedBlockInBody looks for a "## Desk-decided" section in body and parses it.
// found=false means no such section exists at all — the "no block, no finding" case deskflip
// must never refuse on by itself. When found, a non-nil err means the section exists but
// does NOT parse as well-formed (missing marker, empty list, or an item missing a field) —
// deskflip's mechanical check refuses on THIS, distinctly from "no block at all".
func ParseDeskDecidedBlockInBody(body string) (items []DecidedItem, found bool, err error) {
	start, end, ok := findDeskDecidedSection(body)
	if !ok {
		return nil, false, nil
	}
	section := body[start:end]
	afterHeading := section
	if i := strings.IndexByte(afterHeading, '\n'); i >= 0 {
		afterHeading = afterHeading[i+1:]
	} else {
		afterHeading = ""
	}
	markerIdx := strings.Index(afterHeading, DeskDecidedMarker)
	if markerIdx < 0 {
		return nil, true, fmt.Errorf("%s section is missing the %s marker", DeskDecidedHeading, DeskDecidedMarker)
	}
	listPart := afterHeading[markerIdx+len(DeskDecidedMarker):]
	its, perr := ParseDecidedItems([]byte(listPart))
	if perr != nil {
		return nil, true, fmt.Errorf("%s section is malformed: %w", DeskDecidedHeading, perr)
	}
	return its, true, nil
}

// undeclaredDeskDecisionRe matches the reviewer-kit's fixed finding line:
// `Undeclared-desk-decision: <one line>`. It is a BLOCK-direction marker — like
// `Security-Review: fail` and the correctness `Verdict: request-changes` line — so it is
// read with ReadFenced: a finding must not be hideable inside a code fence.
var undeclaredDeskDecisionRe = regexp.MustCompile(`(?im)^[ \t]*Undeclared-desk-decision:[ \t]*(.+?)[ \t\r]*$`)

// UndeclaredDeskDecisionLines returns every `Undeclared-desk-decision: <one line>` value a
// review body carries, fence- and emphasis-handled through the same shared reduction every
// other verdict marker uses (VerdictMarkerValues) — never a second hand-rolled scan.
func UndeclaredDeskDecisionLines(body string) []string {
	return VerdictMarkerValues(body, undeclaredDeskDecisionRe, ReadFenced)
}
