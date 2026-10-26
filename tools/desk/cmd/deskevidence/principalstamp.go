package main

// On-behalf-of principal stamp for App-run Evidence rows.
//
// statusgen's attribution lint requires every App-authored Evidence row dated at or after
// its cutover to carry the `on-behalf-of human:<subject>` annotation in its Runner cell
// (statusgen/attribution.go, principalAttributionProblems). verifyrun renders that
// annotation when it writes a witness row, but a row that reaches this tool any other way
// (a transcribed table, a re-run assembled from several witnesses) can arrive without it —
// and the only remedy was to hand-edit the Runner cell, which is exactly the kind of
// model-written attribution the annotation exists to replace.
//
// So the tool stamps it at ingest. The principal comes from ONE place: deskkit's
// ResolvePrincipal, the same config-home roster source the commit trailer
// (OnBehalfOfCommitSuffix) and verifyrun's own annotation read. It is never taken from a
// flag, an environment variable, or the content being landed. When it cannot be resolved
// the landing refuses (exit 5) and nothing is written — a placeholder is never stamped.
//
// What is touched, and what never is:
//   - Only data rows of a `## Evidence` table that THIS landing adds (a line not already on
//     the branch). A row already on the branch stays byte-identical, stamped or not.
//   - Only a Runner cell that names an App identity (a leading `<slug>[bot]` identifier). A
//     human runner (`human:<name>`) needs no principal and is never read.
//   - An added App cell is classified with the LINT'S OWN reading of the annotation
//     (onBehalfOfPrincipalOf: the LAST `on-behalf-of human:<p>`, case-insensitive), against
//     the subject the roster resolves for this target and the same roster's human set:
//       - no `on-behalf-of` text at all: stamp;
//       - exactly one annotation, and it is the exact form this tool writes for the resolved
//         subject, byte for byte: left byte-identical;
//       - an annotation not in the `human:` form (a desk role, a bare name), one naming no
//         recognised roster human, more than one annotation, the SAME human in the other
//         form (the login on a public target, the neutral name on a private one), or the
//         subject written any other way (other case, extra words inside the parenthetical):
//         every on-behalf-of fragment is stripped and the resolved one stamped — the strip-
//         then-append rule deskkit.AppendOnBehalfOf applies to the commit trailer;
//       - any `human:` annotation naming a DIFFERENT recognised roster human: the landing
//         refuses (exit 5), names the row, and writes nothing. One human's attribution is
//         never overwritten with another's automatically.
//     So the cell content can never choose the principal: it is the roster's, or the
//     landing does not happen.
//   - An added App cell the tool cannot read the way the lint does refuses the landing (exit
//     5) before any classification: an HTML comment inside the cell or crossing a line break
//     on its line (the lint removes comments, joining lines), invalid UTF-8 or a character
//     whose lowercase form has a different byte length (the lint slices the cell at offsets
//     taken from a lowercased copy), or an invisible format character such as a zero-width
//     space (unreadableRunnerCell). After stamping, the landed section is read back the lint's
//     way and every App row not already on the branch must carry exactly the resolved stamp,
//     or the landing refuses.
//   - The stamp is always written in the exact form `(on-behalf-of human:<who>)` and nothing
//     broader, which is the only form a container witness bracket's digest drops from a
//     Runner cell (it then collapses the cell's spaces and tabs). A restamp puts it where the
//     last stripped fragment stood, with the one whitespace byte that fragment carried (or
//     none, if it was glued); a fresh stamp goes before a trailing runner-source tag
//     (`(forge-identity)`, `(ci-env)`, `(git-config)`), the order verifyrun renders, preceded
//     by one space (by none when the tag is glued to the text), or is appended after one
//     space. An unclosed
//     fragment ends before the next `(`, so it never takes a source tag with it. Every other
//     byte of the row is left as it was. So landing the stamp, or restamping one exact-form
//     annotation to another, never moves that digest; restamping a non-exact annotation (a
//     desk role, an empty principal) does, because the old annotation was inside the hash,
//     and the bracket then fails closed.
//
// Table reading mirrors statusgen's parseEvidenceRows (header = the row followed by a
// separator; Runner column by its header name, else the last cell; `\|` is cell content;
// closed HTML comments are not rows), so the row this stamps is the row the lint reads.

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// evidenceSeparatorRe matches a Markdown table separator row (statusgen's separatorRowRe).
var evidenceSeparatorRe = regexp.MustCompile(`^[\s:|-]+$`)

// evidenceCommentRe matches a CLOSED HTML comment (statusgen's htmlCommentClosedRe).
var evidenceCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// runnerSourceTags are the trailing runner-source parentheticals verifyrun renders after
// the on-behalf-of annotation (statusgen/verifyrun.go runnerSource* constants).
var runnerSourceTags = []string{"(forge-identity)", "(ci-env)", "(git-config)"}

const onBehalfOfCellMarker = "on-behalf-of"

// principalSubjectFn resolves the subject the annotation names for a landing into repo.
// A seam only so a test can count resolutions; production is resolveStampSubject.
var principalSubjectFn = resolveStampSubject

// resolveStampSubject resolves the annotation's subject from the roster — the neutral name
// on a public target, the login on a target the roster states private (deskkit.Principal.Subject) — or
// refuses, wrapping ResolvePrincipal's own refusal so the operator sees which step needed it.
func resolveStampSubject(repo string) (string, error) {
	p, err := deskkit.ResolvePrincipal("", repo)
	if err != nil {
		return "", deskkit.Refused("refused: an App-run Evidence row in this landing needs its on-behalf-of " +
			"principal stamped or checked and the tool cannot stamp one — nothing was written: " + err.Error())
	}
	subject := p.Subject()
	if subject == "" {
		return "", deskkit.Refused("refused: an App-run Evidence row in this landing needs its on-behalf-of " +
			"principal stamped or checked and the roster resolved an empty one, so the tool cannot stamp one — nothing was written")
	}
	return subject, nil
}

// stampEvidencePrincipal returns content with the on-behalf-of annotation stamped into every
// App-run Evidence row content adds relative to base (see the file comment), plus a map from
// each stamped or restamped row's trimmed original text to its trimmed stamped text. resolve
// is called at most once, and only when the landing adds some App-run row (every such row is
// stamped or checked against it); its error is returned unchanged. A row naming a different
// recognised human, or one the tool cannot read the way the lint does, refuses the whole
// landing (needsPrincipalStamp, unreadableRunnerCell, landedPrincipalProblem).
func stampEvidencePrincipal(content, base []byte, resolve func() (string, error)) ([]byte, map[string]string, error) {
	lines := strings.Split(string(content), "\n")
	if _, closed := briefBodyStart(lines); !closed && hasEvidenceHeading(lines) {
		return nil, nil, deskkit.Refused("refused: the landed brief opens a frontmatter block that never closes, so " +
			"the lint cannot read its Evidence section and this tool cannot check or stamp an on-behalf-of " +
			"principal in it — nothing was written. Close the frontmatter with a `---` line and land again")
	}
	onBranch := map[string]bool{}
	baseLines := strings.Split(string(base), "\n")
	for _, l := range baseLines {
		onBranch[l] = true
	}
	// A heading a renderer shows as Evidence, other than the one section the lint reads, shows
	// the rows under it as Evidence that neither the lint nor this tool ever reads. A landing
	// may neither add one nor add a line under one (a heading already on the branch with
	// nothing added under it does not block later landings).
	if at, under, ok := unreadEvidenceAddition(lines, baseLines, onBranch); ok {
		where := ""
		if under != at {
			where = fmt.Sprintf(" (the landing adds line %d under it)", under+1)
		}
		return nil, nil, deskkit.Refused(fmt.Sprintf("refused: line %d of the landed brief is a heading a renderer "+
			"shows as Evidence%s, but the lint reads only the first `## Evidence` section, so rows under that heading "+
			"would show as Evidence and never be checked or stamped — nothing was written. Land every row in that "+
			"one section, under no second Evidence heading: %q", at+1, where, strings.TrimSpace(lines[at])))
	}
	if i, r, ok := addedHeadingLetter(lines, onBranch); ok {
		return nil, nil, deskkit.Refused(fmt.Sprintf("refused: line %d of the landed brief is a heading holding the "+
			"non-ASCII letter or mark U+%04X, which can show as an ASCII one, so a reader can see a heading the lint "+
			"and this tool read as other text — nothing was written. Land the heading in ASCII letters: %q",
			i+1, r, strings.TrimSpace(lines[i])))
	}
	start, end, ok := evidenceSpan(lines)
	if !ok {
		// No section the lint reads: an App-run row anywhere in the body is still refused.
		if i, why := outsideEvidenceProblem(lines, -1, -1, onBranch); why != "" {
			return nil, nil, outsideRefusal(i, why, lines)
		}
		return content, nil, nil
	}
	// Every line this landing adds to the Evidence section, of any kind (data row, header,
	// separator, or a line that is not a table row at all), is checked for a control character
	// before any line is skipped below. A line counts as already on the branch here only when it
	// stands in the base's own Evidence section, so a line carried in from elsewhere in the base
	// file is checked like any other added line. The heading line that ends the section is
	// checked too: a lone CR in it can put a table the lint never reads right under it.
	onBranchEvidence := map[string]bool{}
	if bs, be, ok := evidenceSpan(baseLines); ok {
		for _, l := range baseLines[bs:min(be+1, len(baseLines))] {
			onBranchEvidence[l] = true
		}
	}
	for i := start; i < min(end+1, len(lines)); i++ {
		if onBranchEvidence[lines[i]] {
			continue
		}
		if r, ok := rowControlCharacter(lines[i]); ok {
			return nil, nil, deskkit.Refused(fmt.Sprintf("refused: the added Evidence line %d is one this tool "+
				"cannot read the way the lint does (it holds the control character U+%04X, which a renderer can show "+
				"as a line break, putting a row the lint never reads on screen) — nothing was written. Land "+
				"the line without it: %s", i+1, r, strings.TrimSpace(lines[i])))
		}
	}
	// Blank closed comments out byte-for-byte (newlines kept) so offsets and line numbers in
	// the blanked view are the original's. The lint REMOVES them instead, which joins the
	// lines on either side of a comment that crosses a line break; joined marks every line
	// such a comment touches, and an App row on one is refused (unreadableRunnerCell).
	section := strings.Join(lines[start:end], "\n")
	joined := map[int]bool{}
	for _, m := range evidenceCommentRe.FindAllStringIndex(section, -1) {
		first := strings.Count(section[:m[0]], "\n")
		last := first + strings.Count(section[m[0]:m[1]], "\n")
		for l := first; last > first && l <= last; l++ {
			joined[l] = true
		}
	}
	view := strings.Split(blankComments(section), "\n")

	subject := ""
	resolved := false
	subjectOf := func() (string, error) {
		if !resolved {
			s, err := resolve()
			if err != nil {
				return "", err
			}
			subject, resolved = s, true
		}
		return subject, nil
	}
	var stamped map[string]string
	runnerIdx := -1
	for i := 0; i < len(view); i++ {
		line := strings.TrimSpace(view[i])
		if !strings.HasPrefix(line, "|") {
			runnerIdx = -1
			continue
		}
		if evidenceSeparatorRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		spans := cellSpans(view[i])
		if i+1 < len(view) && evidenceSeparatorRe.MatchString(strings.Trim(strings.TrimSpace(view[i+1]), "|")) {
			// Checked whenever the landing adds a line to the table this header governs (the
			// header, its separator, or any row the lint reads under it), not only when the
			// header itself is added.
			if lintTableAdds(view, lines[start:end], i, onBranchEvidence) {
				if why := unreadableHeader(view[i], lines[start+i], spans); why != "" {
					return nil, nil, deskkit.Refused(fmt.Sprintf("refused: the added Evidence header at line %d is one "+
						"this tool cannot read the way the lint does (%s), so the column a reader takes as Runner may "+
						"not be the one the lint reads — nothing was written. Land the header with plain ASCII column "+
						"names and one `Runner` column: %s", start+i+1, why, strings.TrimSpace(lines[start+i])))
				}
			}
			runnerIdx = -1
			for j, sp := range spans {
				if strings.ToLower(strings.TrimSpace(view[i][sp[0]:sp[1]])) == "runner" {
					runnerIdx = j
				}
			}
			i++
			continue
		}
		if len(spans) == 0 {
			continue
		}
		orig := lines[start+i]
		if onBranch[orig] {
			continue
		}
		sp := spans[len(spans)-1]
		if runnerIdx >= 0 && runnerIdx < len(spans) {
			sp = spans[runnerIdx]
		}
		// Decide on the comment-blanked view (what the lint reads); insert into the original
		// bytes at the same offset, so everything else in the line is left exactly as it was.
		cell := view[i][sp[0]:sp[1]]
		if !isAppRunnerCell(strings.TrimSpace(cell)) {
			// A Runner cell the lint does not read as App-run must not show a reader one either:
			// markup the lint reads as text (an HTML tag, an entity, emphasis, strikethrough, a
			// code span, an escape, a comment marker in a code span, link or image syntax) is
			// stripped or reduced to its shown text before deciding what a renderer shows, with
			// a closed comment read both hidden and shown. Read on the cell's bytes as landed,
			// comments included, since a renderer can show a comment's text.
			if showsAppRun(orig[sp[0]:sp[1]]) {
				return nil, nil, deskkit.Refused(fmt.Sprintf("refused: the Evidence row at line %d has a Runner cell "+
					"this tool cannot read the way the lint does (it holds %s, and a renderer "+
					"shows it as an App-run Runner cell the lint does not read as one), so it cannot check or stamp "+
					"its on-behalf-of principal — nothing was written. Land the row with a plain Runner cell: %s",
					start+i+1, shownMarkup(orig[sp[0]:sp[1]]), strings.TrimSpace(orig)))
			}
			continue
		}
		if why := unreadableRunnerCell(cell, orig[sp[0]:sp[1]], joined[i]); why != "" {
			return nil, nil, deskkit.Refused(fmt.Sprintf("refused: the App-run Evidence row at line %d has a Runner cell "+
				"this tool cannot read the way the lint does (%s), so it cannot check or stamp its on-behalf-of "+
				"principal — nothing was written. Land the row with a plain Runner cell: %s",
				start+i+1, why, strings.TrimSpace(orig)))
		}
		subject, err := subjectOf()
		if err != nil {
			return nil, nil, err
		}
		verdict, other := needsPrincipalStamp(cell, subject)
		switch verdict {
		case stampLeave:
			continue
		case stampRefuse:
			return nil, nil, deskkit.Refused(fmt.Sprintf("refused: the App-run Evidence row at line %d names a different "+
				"recognised roster human (%s) than the principal the roster resolves for this target; "+
				"this tool never overwrites one human's attribution with another's — nothing was written. Leave the Runner "+
				"cell unannotated and land again, or have that human land it: %s",
				start+i+1, other, strings.TrimSpace(orig)))
		}
		// stampAdd / stampRestamp: strip every on-behalf-of fragment (none, for stampAdd) from
		// both the blanked view and the original at the same offsets, then insert the resolved
		// annotation where the last stripped fragment stood (with the separator it had), or,
		// with nothing stripped, before any trailing source tag.
		cellView, cellOrig := cell, orig[sp[0]:sp[1]]
		cuts := onBehalfOfFragments(cellView)
		for _, cut := range cuts {
			cellView = cellView[:cut[0]] + cellView[cut[1]:]
			cellOrig = cellOrig[:cut[0]] + cellOrig[cut[1]:]
		}
		pos, sep := stampPosition(cell, cellView, cuts)
		canonical := canonicalStamp(subject)
		annotation := sep + canonical
		stampedView := cellView[:pos] + annotation + cellView[pos:]
		// Defensive: the stamped cell must read, to the lint, as exactly one annotation naming
		// the resolved subject, written in the exact form. Anything else means the strip
		// mis-parsed the cell.
		if p, ok := onBehalfOfPrincipalOf(stampedView); !ok || p != strings.ToLower(subject) ||
			strings.Count(strings.ToLower(stampedView), onBehalfOfCellMarker) != 1 || strings.Count(stampedView, canonical) != 1 {
			return nil, nil, deskkit.Unverifiable(fmt.Sprintf("on-behalf-of stamp: the restamped Runner cell at line %d "+
				"does not read back as one annotation naming the resolved principal — refusing rather than land it: %s",
				start+i+1, strings.TrimSpace(orig)), nil)
		}
		newLine := orig[:sp[0]] + cellOrig[:pos] + annotation + cellOrig[pos:] + orig[sp[1]:]
		if stamped == nil {
			stamped = map[string]string{}
		}
		stamped[strings.TrimSpace(orig)] = strings.TrimSpace(newLine)
		lines[start+i] = newLine
	}
	// A line a renderer reads as a table row the lint does not read the same way (checked after
	// the rows, so a row the checks above refuse keeps its own, more specific refusal).
	if i, why := renderedRowMismatch(lines, start, end, onBranchEvidence); why != "" {
		return nil, nil, deskkit.Refused(fmt.Sprintf("refused: the added Evidence line %d is one this tool "+
			"cannot read the way the lint does (%s) — nothing was written. Begin every table row with `|`, and "+
			"put a blank line between a table and any other line: %s", i+1, why, strings.TrimSpace(lines[i])))
	}
	// Read the landed section back the way the lint does — closed comments REMOVED, lines
	// joined — and require every App-run row it reads that is not already a row of the
	// branch to carry exactly the resolved stamp. The checks above keep the two readings
	// aligned; this is the backstop for any shape where they still differ.
	landed := strings.Join(lines[start:end], "\n")
	baseSection := ""
	if bs, be, ok := evidenceSpan(baseLines); ok {
		baseSection = strings.Join(baseLines[bs:be], "\n")
	}
	// A row counts as already on the branch only when the base has the same line READ THE SAME
	// WAY — the same Runner cell under its own header — so a base line re-landed under a header
	// that moves the lint's Runner column is checked like any added row.
	onBranchRows := map[[2]string]bool{}
	for _, r := range lintEvidenceRunners(baseSection) {
		onBranchRows[[2]string{r.line, r.runner}] = true
	}
	for _, r := range lintEvidenceRunners(landed) {
		if onBranchRows[[2]string{r.line, r.runner}] || !isAppRunnerCell(r.runner) {
			continue
		}
		subject, err := subjectOf()
		if err != nil {
			return nil, nil, err
		}
		if why := landedPrincipalProblem(r.runner, subject); why != "" {
			return nil, nil, deskkit.Refused(fmt.Sprintf("refused: read the way the lint reads it (closed HTML comments "+
				"removed, which joins the lines around one), the landed Evidence section has an App-run row whose "+
				"Runner cell %s — nothing was written. Land the rows with plain Runner cells and no HTML comment "+
				"crossing a line break: %s", why, r.line))
		}
	}
	// The positive check: every line the landing adds to the section must be a blank line, a
	// row of a table both readings agree on, or a note line neither reads as a row.
	if i, why := evidenceLayoutProblem(lines, start, end, onBranchEvidence); why != "" {
		return nil, nil, deskkit.Refused(fmt.Sprintf("refused: the added Evidence line %d is one this tool "+
			"cannot read the way the lint does (%s) — nothing was written. Land Evidence as `|`-led rows under a "+
			"header and a `|---|` delimiter row, blank lines, and plain note lines: %s", i+1, why, strings.TrimSpace(lines[i])))
	}
	if i, why := outsideEvidenceProblem(lines, start, end, onBranch); why != "" {
		return nil, nil, outsideRefusal(i, why, lines)
	}
	if stamped == nil {
		return content, nil, nil
	}
	return []byte(strings.Join(lines, "\n")), stamped, nil
}

// blankComments returns section with every closed HTML comment blanked byte for byte (each
// byte other than a newline becomes a space), so the result has the same length, the same line
// breaks and the same byte offsets as section. Cell positions found in it slice the original
// lines at the same cells, whatever characters a comment holds.
func blankComments(section string) string {
	return evidenceCommentRe.ReplaceAllStringFunc(section, func(m string) string {
		b := []byte(m)
		for i := range b {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
		return string(b)
	})
}

// canonicalStamp is the one form the stamp writes: `(on-behalf-of human:<subject>)`, the exact
// parenthetical a container witness bracket's digest drops from a Runner cell.
func canonicalStamp(subject string) string {
	return "(" + onBehalfOfCellMarker + " human:" + subject + ")"
}

// stampPosition returns where, in the stripped cell, the stamp goes, and the separator written
// before it. cuts are the fragments stripped from cell (onBehalfOfFragments, descending).
//   - With fragments stripped, the stamp goes where the LAST of them stood, preceded by the
//     whitespace byte that fragment carried (none, if it was glued to the text before it). So
//     a restamp of one exact-form annotation to another changes only the annotation's bytes.
//     When fragments sit back to back, the separator is the one the first of them carried.
//   - With none, it goes before a trailing runner-source tag, ahead of any whitespace run
//     between the text and the tag, separated by one space; a tag glued to the text keeps the
//     stamp glued too. With no tag, it is appended after one space.
func stampPosition(cell, stripped string, cuts [][2]int) (int, string) {
	if len(cuts) > 0 {
		last := cuts[0]
		pos := last[0]
		for _, c := range cuts[1:] {
			pos -= c[1] - c[0]
		}
		// Fragments stripped back to back form one run; the separator is the one the run's
		// first fragment carried.
		head := last[0]
		for moved := true; moved; {
			moved = false
			for _, c := range cuts[1:] {
				if c[1] == head {
					head, moved = c[0], true
				}
			}
		}
		sep := ""
		if b := cell[head]; b == ' ' || b == '\t' {
			sep = string(b)
		}
		return pos, sep
	}
	text := strings.TrimRight(stripped, " \t")
	tagAt, ok := trailingSourceTag(text)
	if !ok {
		return len(text), " "
	}
	pos := tagAt
	for pos > 0 && (text[pos-1] == ' ' || text[pos-1] == '\t') {
		pos--
	}
	if pos == tagAt {
		return pos, ""
	}
	return pos, " "
}

// unreadableRunnerCell reports why an added App-run Runner cell cannot be read the way the
// lint reads it, or "" when it can. view is the cell with closed comments blanked, orig its
// bytes as landed, joined whether a comment crossing a line break touches the row's line.
//   - A comment crossing a line break: the lint removes it and so reads this line joined to
//     the next, which can put a different annotation in what it takes as this row.
//   - A comment inside the cell: the rendered row shows text the lint does not read.
//   - Invalid UTF-8, or a character whose lowercase form has a different byte length: the
//     lint matches the annotation in a lowercased copy and slices the original at the same
//     offsets, so the principal it reads would not be the text in the cell.
//   - An invisible format character (a zero-width space, a soft hyphen): the rendered marker
//     and the marker the lint matches are different text.
//   - Any other non-ASCII character (a combining mark, a homoglyph such as a Cyrillic letter in
//     the marker): the rendered text can show an annotation, or a name, the lint does not read.
//     An App-run Runner cell is an App login, a commit and ASCII tags, so nothing legitimate is
//     lost.
//   - Markdown link or image syntax (`[text](...)`, `[text][label]`, `![alt](...)`): a renderer
//     shows the link text, not the brackets and destination the lint reads. Brackets that open
//     no link (`[bot]`, a model tag such as `[1m]`) are left alone.
//
// A control character anywhere in an added Evidence line is refused separately
// (rowControlCharacter), since it can sit outside the Runner cell or outside any data row.
func unreadableRunnerCell(view, orig string, joined bool) string {
	if joined {
		return "an HTML comment on its line crosses a line break, and the lint reads the lines around it as one"
	}
	if view != orig {
		return "an HTML comment sits inside the cell"
	}
	for i := 0; i < len(orig); {
		r, size := utf8.DecodeRuneInString(orig[i:])
		i += size
		if r == utf8.RuneError && size == 1 {
			return "it is not valid UTF-8"
		}
		if unicode.Is(unicode.Cf, r) {
			return fmt.Sprintf("it holds the invisible format character U+%04X", r)
		}
		if utf8.RuneLen(unicode.ToLower(r)) != size {
			return fmt.Sprintf("it holds U+%04X, whose lowercase form has a different byte length, which shifts "+
				"the lint's case-insensitive reading of the annotation", r)
		}
	}
	for _, r := range orig {
		if r > unicode.MaxASCII {
			return fmt.Sprintf("it holds the non-ASCII character U+%04X, so the rendered text can differ from what "+
				"the lint reads", r)
		}
	}
	if c, ok := runnerMarkup(orig); ok {
		return fmt.Sprintf("it holds the markup character `%c`, which a renderer can read as HTML or Markdown "+
			"(a struck-through or hidden annotation, an entity or escape that shows another character), so the "+
			"rendered text can differ from what the lint reads", c)
	}
	if linkSyntax(orig) {
		return "it holds Markdown link or image syntax, which a renderer shows as its link text, not as written, so " +
			"the rendered text can differ from what the lint reads"
	}
	return ""
}

// runnerMarkup returns the first character in an App-run Runner cell that a renderer can read
// as markup changing the text it shows: `<` and `>` (an HTML tag: strikethrough, a title
// attribute, a hidden span), `&` (an entity), `~` (strikethrough) and `\` (an escape). An
// App-run Runner cell is an App login, a commit and plain tags, so none is legitimate there.
func runnerMarkup(cell string) (rune, bool) {
	if i := strings.IndexAny(cell, "<>&~\\"); i >= 0 {
		return rune(cell[i]), true
	}
	return 0, false
}

var (
	// htmlTagRe and runnerMarkupRe strip what a renderer does not show as text from a Runner
	// cell (HTML tags; emphasis, strikethrough, code and escape characters), to tell whether
	// it would SHOW as an App-run cell. commentMarkerRe matches the markers of an HTML
	// comment, which are removed so the comment's text counts as shown (inside a code span a
	// renderer shows it).
	htmlTagRe       = regexp.MustCompile(`<[^>]*>`)
	runnerMarkupRe  = regexp.MustCompile("[*_~`\\\\]")
	commentMarkerRe = regexp.MustCompile(`<!--|-->`)
	// rawTableTagRe matches a raw HTML table or heading tag, which a renderer shows as a table
	// or heading of its own that the lint never reads.
	rawTableTagRe = regexp.MustCompile(`(?i)<\s*/?\s*(?:table|thead|tbody|tfoot|tr|td|th|h[1-6])\b`)
	// codeSpanRe matches an inline code span, whose text a renderer shows as code, not markup.
	codeSpanRe = regexp.MustCompile("(`+)[^`]+?(`+)")
	// gfmDelimiterCellRe matches one cell of a delimiter row a GFM renderer accepts.
	gfmDelimiterCellRe = regexp.MustCompile(`^:?-+:?$`)
)

// commentReadings returns s read both ways a renderer can show a closed HTML comment: hidden
// (outside a code span) and with only its markers removed (inside one, where its text shows).
func commentReadings(s string) []string {
	return []string{evidenceCommentRe.ReplaceAllString(s, ""), commentMarkerRe.ReplaceAllString(s, "")}
}

// linkReadings returns s as written and with its Markdown link and image syntax reduced to the
// text a renderer shows: once with an image's alt text dropped (the image shows) and once with
// it kept (a reader can see the alt text where the image does not load).
func linkReadings(s string) []string {
	return []string{s, reduceLinks(s, false), reduceLinks(s, true)}
}

// shownTexts returns the texts a renderer can show for s, for deciding what a reader sees: each
// comment reading and each link reading, with HTML tags and emphasis, strikethrough, code and
// escape characters removed, and entities decoded, read once with the text of each element a
// sanitizer can drop kept and dropped, and apart from those, of each element a renderer hides
// by default kept and dropped two ways (see droppedElementRe, hiddenElementRe and dropHidden),
// each of those once more from each place a renderer starts a new line at (see lineBreakRe),
// and each of those once more for each math reading the tool takes (see mathReadings). Each is
// read once as it stands and once with every character that renders as nothing dropped (see
// rendersBlank) and the words that lead it and cannot be a login skipped (see pastLead), so a
// text's identity region (see identityRegion) starts at the first word a reader can take for a
// name. That last reading is taken once more with its leading words read as a renderer shows
// them (see pastShownLead).
// Every reading is added beside the others and none replaces one, so text later in a cell can
// add a reading but never take one away. A cell shows something when any of these does.
func shownTexts(s string) []string {
	var out []string
	for _, c := range commentReadings(s) {
		// The shortcut reading takes every balanced `[text]` as a reference link whose
		// definition stands somewhere in the brief (or on another branch), since a renderer
		// shows its text and not its brackets whenever a definition resolves; it is a reading
		// of what could be shown, never of link syntax (see linkSyntax).
		readings := append(linkReadings(c), reduceLinksMode(c, true, true))
		for _, r := range readings {
			// Elements a sanitizer drops and elements a renderer hides are read kept and dropped
			// apart from each other, since either can show while the other does not. A hidden
			// element is read dropped both to the first closing tag after it and to the closing
			// tag that matches it.
			sanitized := droppedElementRe.ReplaceAllString(r, "")
			shown := []string{r}
			for _, base := range []string{r, sanitized} {
				for _, e := range []string{base, hiddenElementRe.ReplaceAllString(base, ""), dropHidden(base)} {
					if !slices.Contains(shown, e) {
						shown = append(shown, e)
					}
				}
			}
			for i, n := 0, len(shown); i < n; i++ {
				for _, m := range lineBreakRe.FindAllStringIndex(shown[i], -1) {
					shown = append(shown, shown[i][m[1]:])
				}
			}
			for _, v := range shown {
				v = htmlTagRe.ReplaceAllString(v, "")
				for _, d := range mathDecodings(v) {
					t := strings.TrimSpace(runnerMarkupRe.ReplaceAllString(d, ""))
					out = append(out, t)
					blank := strings.Map(dropBlank, t)
					seen := pastLead(blank)
					if seen != t {
						out = append(out, seen)
					}
					// Only the leading words are read as a renderer shows them, each by itself, and
					// what follows them is taken as written.
					if lead := pastShownLead(blank); lead != seen && lead != t {
						out = append(out, lead)
					}
				}
			}
		}
	}
	return out
}

var (
	// glyphWordRe matches a word a renderer replaces with a glyph (`:name:`), in any case and
	// after emphasis and escape characters are stripped, so it matches more than a renderer
	// replaces. glyphPlaceholder is the one symbol it is read as, which holds no ASCII letter or
	// digit, as the glyph shown does not.
	glyphWordRe      = regexp.MustCompile(`(?i):[a-z0-9_+-]+:`)
	glyphPlaceholder = "\u25a1"
	// mathDelimRe matches a run of `$`, which a renderer can take as a math delimiter. Which runs
	// a renderer pairs is its own rule, which the tool does not hold: mathDelimiters leaves out
	// only the dollar signs a renderer shows as text, and mathReadings reads the text between
	// every two adjacent runs that are left.
	mathDelimRe = regexp.MustCompile(`\$+`)
	// mathMarkupRe matches a control word or symbol (a backslash and a name or one character), a
	// brace, a backtick or a sub- or superscript sign. In a math span a renderer typesets each as
	// no text of its own, except those mathGlyphs names. A control word can typeset its argument
	// as text, as blank space or as nothing, so mathReadings reads a span's text from each place
	// one ends.
	mathMarkupRe = regexp.MustCompile("\\\\(?:[A-Za-z]+|.)|[{}`^_]")
	// lineBreakRe matches a raw HTML line break or the opening or closing tag of an element a
	// renderer shows as a block of its own (a disclosure element among them), after which the
	// text a reader sees starts a new line.
	lineBreakRe = regexp.MustCompile(`(?i)<\s*/?\s*(?:br|hr|p|div|details|summary|blockquote|pre|ul|ol|li|dl|dt|dd|table|thead|tbody|tfoot|tr|td|th|caption|h[1-6]|section|article|aside|header|footer|nav|main|figure|figcaption|address|center|fieldset|legend|form)\b[^>]*>`)
	// droppedElementRe matches a raw HTML element a sanitizer can remove with its text, and so
	// show nothing for, from its opening tag to the next such closing tag or the end of the text.
	droppedElementRe = regexp.MustCompile(`(?is)<\s*(?:script|style|template|textarea|title|iframe|noembed|noframes|noscript|plaintext|xmp|math|svg)\b[^>]*>.*?(?:<\s*/\s*(?:script|style|template|textarea|title|iframe|noembed|noframes|noscript|plaintext|xmp|math|svg)\s*>|$)`)
	// hiddenOpenRe matches the opening tag of a raw HTML element a renderer hides by default with
	// its text: a ruby fallback, a data list or a dialog element (group 1 names it), or any
	// element that carries the hidden attribute (group 2 names it).
	hiddenOpenRe = regexp.MustCompile(`(?is)` + hiddenOpen)
	// hiddenElementRe matches such an element from its opening tag to the first closing tag of
	// any kind after it, or the end of the text, which is where a parser can end a ruby fallback
	// that the next one closes. dropHidden reads it to the closing tag that matches it.
	hiddenElementRe = regexp.MustCompile(`(?is)(?:` + hiddenOpen + `).*?(?:<\s*/[^>]*>|$)`)
	// elementTagRe matches an opening or closing HTML tag: group 1 holds a closing tag's slash,
	// and group 2 the element's name.
	elementTagRe = regexp.MustCompile(`(?i)<\s*(/?)\s*([a-z][a-z0-9-]*)\b[^>]*>`)
)

// hiddenOpen is hiddenOpenRe's pattern without its flags. A parser reads an attribute after
// white space, a slash or a closing quote.
const hiddenOpen = `<\s*(rp|datalist|dialog)\b[^>]*>|<\s*([a-z][a-z0-9-]*)\b[^>]*?[\s/"']hidden\b[^>]*>`

// dropHidden returns s without each element a renderer hides by default (see hiddenOpenRe),
// taken from its opening tag to the closing tag that matches it, counting the elements of the
// same name opened inside it, or to the end of the text.
func dropHidden(s string) string {
	var b strings.Builder
	for {
		m := hiddenOpenRe.FindStringSubmatchIndex(s)
		if m == nil {
			b.WriteString(s)
			return b.String()
		}
		g := 2
		if m[2] < 0 {
			g = 4
		}
		name := s[m[g]:m[g+1]]
		b.WriteString(s[:m[0]])
		s = s[m[1]:]
		end, depth := len(s), 1
		for _, t := range elementTagRe.FindAllStringSubmatchIndex(s, -1) {
			if !strings.EqualFold(s[t[4]:t[5]], name) {
				continue
			}
			if t[3] > t[2] {
				depth--
			} else {
				depth++
			}
			if depth == 0 {
				end = t[1]
				break
			}
		}
		s = s[end:]
	}
}

// pastLead returns t trimmed and without the words that lead it and hold no ASCII letter or
// digit, a word ending at white space. Such a word cannot be a login, so a reader takes the word
// after it for the name, whatever its characters are and whether or not they render. A word
// that holds an ASCII letter or digit ends the skip, and its characters are read as they stand.
// Every trim takes any Unicode white space, since a renderer shows a non-breaking or wide space
// at the edge of a word as white space too.
func pastLead(t string) string { return skipLead(t, false) }

// pastShownLead returns t as pastLead does, except that each leading word is read as a renderer
// shows it: a word a renderer shows as a glyph (see glyphWordRe) is read as one symbol, so a
// leading word that, read so, holds no ASCII letter or digit is skipped. Each word ends at white
// space and is read by itself, so only that word decides whether it is skipped. The word that
// ends the skip and everything after it are taken as written. Math spans are read by
// mathReadings, beside this reading and before it.
func pastShownLead(t string) string { return skipLead(t, true) }

// skipLead is pastLead, or pastShownLead when shown is set. Both readings walk the same loop,
// so a word is skipped, and the text trimmed, the same way in each.
func skipLead(t string, shown bool) string {
	t = strings.TrimSpace(t)
	for {
		end := strings.IndexFunc(t, unicode.IsSpace)
		if end <= 0 {
			return t
		}
		word := t[:end]
		if shown {
			word = glyphWordRe.ReplaceAllString(word, glyphPlaceholder)
		}
		if strings.IndexFunc(word, asciiAlnum) >= 0 {
			return t
		}
		t = strings.TrimSpace(t[end:])
	}
}

// mathDecodings returns each math reading of s (see mathReadings), each in each decoding of it
// (see decodings). The math readings are taken before entities are decoded, so a dollar sign
// written as an entity, which a renderer shows as text, is never read as a delimiter.
func mathDecodings(s string) []string {
	var out []string
	for _, m := range mathReadings(s) {
		out = append(out, decodings(m)...)
	}
	return out
}

// mathDelimiters returns the runs of `$` in s a renderer can take as math delimiters: each run
// without a dollar sign a backslash escapes, and none inside a code span.
func mathDelimiters(s string) [][]int {
	code := codeSpanRe.FindAllStringIndex(s, -1)
	var out [][]int
	for _, m := range mathDelimRe.FindAllStringIndex(s, -1) {
		n := 0
		for i := m[0] - 1; i >= 0 && s[i] == '\\'; i-- {
			n++
		}
		if n%2 == 1 {
			m[0]++
		}
		if m[0] == m[1] || slices.ContainsFunc(code, func(c []int) bool { return c[0] <= m[0] && m[0] < c[1] }) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// mathReadings returns d, and when d holds two or more runs a renderer can take as math
// delimiters (see mathDelimiters), d read by these rules, each beside the others and each with
// the text before the first run as written:
//   - once for each run, with everything from the first run up to and including it dropped, as
//     spans that show only a symbol or blank space;
//   - once with every run dropped and the text between the first and the last read as math
//     (see mathText), with its white space and without;
//   - once for each two adjacent runs and each place in the text between them where a control
//     word, a brace or white space ends, with everything from the first run up to that place
//     dropped, the text from there to the second run read as math, with its white space and
//     without, and the text after the second run as written.
//
// No reading depends on which runs before a span a renderer pairs. A renderer can typeset a
// span's text as text, so a name inside a span is read where a reader sees it, and a control
// word can show the text before such a place as blank space or as nothing, so each place is read
// as the start of what is shown. These are rules the tool holds, not a renderer's: a renderer
// can pair or typeset runs in a way none of them names.
func mathReadings(d string) []string {
	runs := mathDelimiters(d)
	if len(runs) < 2 {
		return []string{d}
	}
	out := []string{d}
	seen := map[string]bool{d: true}
	add := func(r string) {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	lead := d[:runs[0][0]]
	var all strings.Builder
	for i, r := range runs {
		add(lead + d[r[1]:])
		if i+1 < len(runs) {
			all.WriteString(d[r[1]:runs[i+1][0]])
		}
	}
	for _, collapse := range []bool{false, true} {
		add(lead + mathText(all.String(), collapse) + d[runs[len(runs)-1][1]:])
	}
	for i := 0; i+1 < len(runs); i++ {
		body, after := d[runs[i][1]:runs[i+1][0]], d[runs[i+1][1]:]
		starts := []int{0}
		for j, r := range body {
			if unicode.IsSpace(r) {
				starts = append(starts, j+utf8.RuneLen(r))
			}
		}
		for _, m := range mathMarkupRe.FindAllStringIndex(body, -1) {
			starts = append(starts, m[1])
		}
		for _, at := range starts {
			add(lead + mathText(body[at:], false) + after)
			add(lead + mathText(body[at:], true) + after)
		}
	}
	return out
}

// mathGlyphs are the control words and symbols a renderer typesets in math as a character a
// login or its tag can hold, each with that character.
var mathGlyphs = map[string]string{
	`\lbrack`: "[", `\rbrack`: "]", `\lbrace`: "{", `\rbrace`: "}", `\{`: "{", `\}`: "}",
	`\_`: "_", `\$`: "$", `\#`: "#", `\&`: "&", `\%`: "%",
	`\vert`: "|", `\lvert`: "|", `\rvert`: "|",
}

// mathText returns s as a renderer typesets it in math: each control word or symbol mathGlyphs
// names as its character, and every other match of mathMarkupRe as nothing. With collapse set,
// its entities are decoded and its white space dropped, as math drops the white space between
// the symbols it typesets.
func mathText(s string, collapse bool) string {
	if collapse {
		s = html.UnescapeString(s)
	}
	s = mathMarkupRe.ReplaceAllStringFunc(s, func(m string) string { return mathGlyphs[m] })
	if collapse {
		s = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, s)
	}
	return s
}

// asciiAlnum reports whether r is an ASCII letter or digit.
func asciiAlnum(r rune) bool {
	return r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

// c1RefRe matches a numeric character reference to a C1 control (U+0080 to U+009F).
var c1RefRe = regexp.MustCompile(`&#(?:[xX]0*[89][0-9a-fA-F]|0*(?:12[89]|1[3-5][0-9]));`)

// decodings returns s with its entities decoded as html.UnescapeString does, which reads a
// reference to a C1 control as the windows-1252 character a browser shows for it, and, when s
// holds such a reference, once more with it read as the control itself, which is what a
// CommonMark renderer emits for it and shows as nothing.
func decodings(s string) []string {
	d := html.UnescapeString(s)
	c := c1RefRe.ReplaceAllStringFunc(s, func(ref string) string {
		n, hex := strings.TrimSuffix(ref[2:], ";"), 10
		if n[0] == 'x' || n[0] == 'X' {
			n, hex = n[1:], 16
		}
		v, _ := strconv.ParseUint(n, hex, 32)
		return string(rune(v))
	})
	if c == s {
		return []string{d}
	}
	return []string{d, html.UnescapeString(c)}
}

// rendersBlank reports whether r renders as nothing and is not white space: a nonspacing or
// enclosing mark, a default-ignorable code point (the filler letters among them), the blank
// braille pattern, or a character in no letter, mark, number, punctuation, symbol or separator
// category, which is a control, format, private-use, noncharacter or unassigned code point.
// It is wider than rowattribution.go's stripInvisible, which reads an empty Runner: shownTexts
// keeps each text as it stands beside the text with these dropped, so a wider family only reads
// more of what a reader can be shown.
func rendersBlank(r rune) bool {
	if unicode.IsSpace(r) {
		return false
	}
	if r == '\u2800' || unicode.In(r, unicode.Mn, unicode.Me, unicode.Other_Default_Ignorable_Code_Point) {
		return true
	}
	return !unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Z)
}

// dropBlank is the strings.Map form of rendersBlank: it drops a character that renders as
// nothing and keeps every other.
func dropBlank(r rune) rune {
	if rendersBlank(r) {
		return -1
	}
	return r
}

var (
	// autolinkRe matches a GFM autolink at the start of a string (a URI or an email address in
	// angle brackets); group 1 is the text a renderer shows for it.
	autolinkRe = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9+.-]{1,31}:[^\x00-\x20<>]*|[A-Za-z0-9.!#$%&'*+/=?^_{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)>`)
)

// reduceLinks returns s with every inline link or image (`[text](dest "title")`), reference
// link or image (`[text][label]`, `[text][]`) and autolink (`<scheme:...>`) replaced by the
// text a renderer shows for it: the link text (itself reduced), an image's alt text when
// keepAlt is set and nothing otherwise, and an autolink's address. Backslash escapes and code
// spans are copied as written, since a renderer reads no link inside them. A bracket that opens
// no link is copied, and the text after it is read on.
func reduceLinks(s string, keepAlt bool) string {
	return reduceLinksMode(s, keepAlt, false)
}

// reduceLinksMode is reduceLinks that, with shortcuts set, also reduces a balanced `[text]`
// (or `![alt]`) that opens no inline or full reference link to its (reduced) text: a shortcut
// reference link, which a renderer shows as its text once a `[text]: destination` definition
// stands anywhere the renderer reads.
func reduceLinksMode(s string, keepAlt, shortcuts bool) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			b.WriteString(s[i : i+2])
			i += 2
			continue
		case c == '`':
			if end := codeSpanEnd(s, i); end > i {
				b.WriteString(s[i:end])
				i = end
				continue
			}
		case c == '<':
			if m := autolinkRe.FindStringSubmatchIndex(s[i:]); m != nil {
				b.WriteString(s[i+m[2] : i+m[3]])
				i += m[1]
				continue
			}
		case c == '[' || (c == '!' && i+1 < len(s) && s[i+1] == '['):
			open := i
			if c == '!' {
				open++
			}
			if text, end, ok := linkAt(s, open); ok {
				if c == '[' || keepAlt {
					b.WriteString(reduceLinksMode(text, keepAlt, shortcuts))
				}
				i = end
				continue
			}
			if shortcuts {
				if close := closingBracket(s, open); close > open {
					if c == '[' || keepAlt {
						b.WriteString(reduceLinksMode(s[open+1:close], keepAlt, shortcuts))
					}
					i = close + 1
					continue
				}
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// codeSpanEnd returns the index just past the code span that opens with the backtick run at
// i, or i when that run closes no span.
func codeSpanEnd(s string, i int) int {
	n := len(s[i:]) - len(strings.TrimLeft(s[i:], "`"))
	for j := i + n; j < len(s); {
		k := strings.IndexByte(s[j:], '`')
		if k < 0 {
			return i
		}
		j += k
		m := len(s[j:]) - len(strings.TrimLeft(s[j:], "`"))
		if m == n {
			return j + m
		}
		j += m
	}
	return i
}

// linkAt reads a link whose text opens with the `[` at open: its text, the index just past
// it, and whether the bracket opens one (an inline destination in parentheses, or a reference
// label in brackets, follows the text's closing bracket).
func linkAt(s string, open int) (text string, end int, ok bool) {
	close := closingBracket(s, open)
	if close < 0 || close+1 >= len(s) {
		return "", 0, false
	}
	switch s[close+1] {
	case '(':
		if e := inlineDestinationEnd(s, close+1); e > 0 {
			return s[open+1 : close], e, true
		}
	case '[':
		for j := close + 2; j < len(s); j++ {
			if s[j] == '\\' {
				j++
				continue
			}
			if s[j] == '[' {
				break
			}
			if s[j] == ']' {
				return s[open+1 : close], j + 1, true
			}
		}
	}
	return "", 0, false
}

// closingBracket returns the index of the `]` that closes the `[` at open (brackets nest;
// escapes and code spans are skipped), or -1.
func closingBracket(s string, open int) int {
	depth := 0
	for j := open; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '`':
			if e := codeSpanEnd(s, j); e > j {
				j = e - 1
			}
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				return j
			}
		}
	}
	return -1
}

// inlineDestinationEnd returns the index just past an inline link's `(destination "title")`
// opening at p, or -1 when no such destination closes there. The destination is either
// `<...>` or a run without spaces whose parentheses balance; the optional title is quoted with
// `"`, `'` or parentheses.
func inlineDestinationEnd(s string, p int) int {
	j := p + 1
	skipSpace := func() {
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
	}
	skipSpace()
	if j < len(s) && s[j] == '<' {
		for j++; j < len(s) && s[j] != '>'; j++ {
			if s[j] == '\\' {
				j++
			}
		}
		if j >= len(s) {
			return -1
		}
		j++
	} else {
		depth := 0
		for ; j < len(s) && s[j] != ' ' && s[j] != '\t'; j++ {
			if s[j] == '\\' {
				j++
				continue
			}
			if s[j] == '(' {
				depth++
			}
			if s[j] == ')' {
				if depth == 0 {
					break
				}
				depth--
			}
		}
	}
	skipSpace()
	if j < len(s) && (s[j] == '"' || s[j] == '\'' || s[j] == '(') {
		q := s[j]
		if q == '(' {
			q = ')'
		}
		for j++; j < len(s) && s[j] != q; j++ {
			if s[j] == '\\' {
				j++
			}
		}
		if j >= len(s) {
			return -1
		}
		j++
		skipSpace()
	}
	if j < len(s) && s[j] == ')' {
		return j + 1
	}
	return -1
}

// shownMarkup describes the first thing in s that shownTexts strips, decodes or reduces, or
// that keeps the lint from reading it as a reader does, for a refusal: an HTML tag or comment
// bracket, an entity, emphasis, strikethrough, code or escape character, then a character
// outside printable ASCII (a non-ASCII character or a tab), then a `[bot]` login followed by
// something other than a space or `(`, then link or image syntax.
func shownMarkup(s string) string {
	if i := strings.IndexAny(s, "<>&*_~`\\"); i >= 0 {
		return fmt.Sprintf("the markup character `%c`", s[i])
	}
	for _, r := range s {
		if r > unicode.MaxASCII || r == '\t' {
			return fmt.Sprintf("the character U+%04X", r)
		}
	}
	if asciiAppRunRe.MatchString(s) {
		return "a login ending `[bot]` followed by something other than a space or `(`"
	}
	if i := strings.IndexAny(s, "[]!"); i >= 0 {
		return fmt.Sprintf("the markup character `%c`", s[i])
	}
	return "markup"
}

// asciiAppRunRe matches text that opens with an ASCII App login directly followed by `[bot]`
// in any case, whatever follows, for a refusal's description.
var asciiAppRunRe = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9._-]*\[bot\]`)

// tagOpen and tagClose report whether r opens or closes a tag a reader may take for the
// brackets of a `[bot]` tag: an ASCII square or angle bracket, or any non-ASCII opening or
// closing bracket or quotation mark.
func tagOpen(r rune) bool {
	return r == '[' || r == '<' || r > unicode.MaxASCII && unicode.In(r, unicode.Ps, unicode.Pi)
}

func tagClose(r rune) bool {
	return r == ']' || r == '>' || r > unicode.MaxASCII && unicode.In(r, unicode.Pe, unicode.Pf)
}

// tagAfter reports whether a tag bracket that directly follows r may read as the tag of a
// login ending in r: r is an ASCII letter, digit, `.`, `_` or `-`, or any non-ASCII character
// other than a tag bracket. A bracket at the start of a text, or after another bracket or
// other ASCII punctuation, opens no tag.
func tagAfter(r rune) bool {
	return !tagOpen(r) && !tagClose(r) && (loginByte(r) || r > unicode.MaxASCII)
}

// identityRegion returns the part of a shown text that names who ran a row: the text up to
// the first ASCII space or `(`, as the lint splits a Runner cell, except that a space inside a
// tag (see tagAfter) does not end it when the tag closes before any `(` or other tag bracket.
// A tag that does not close so leaves the lint's cut.
func identityRegion(s string) string {
	cut := len(s)
	if i := strings.IndexAny(s, " ("); i >= 0 {
		cut = i
	}
	open, prev := false, rune(-1)
	for i, r := range s {
		after := prev
		prev = r
		switch {
		case open && tagClose(r):
			open = false
		case open && (r == '(' || tagOpen(r)):
			return s[:cut]
		case !open && tagAfter(after) && tagOpen(r):
			open = true
		case !open && (r == ' ' || r == '('):
			return s[:i]
		}
	}
	if open {
		return s[:cut]
	}
	return s
}

// loginByte reports whether r is in the alphabet an App login and its `[bot]` tag are written
// in: ASCII letters, digits, `.`, `_`, `-` and square brackets.
func loginByte(r rune) bool {
	return r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._-[]", r))
}

// cjkOnly reports whether every letter and digit in s, and at least one, is Han, Hiragana,
// Katakana or Hangul text that renders as itself. Such text cannot spell an App login.
func cjkOnly(s string) bool {
	seen := false
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if !unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) ||
			unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r) {
			return false
		}
		seen = true
	}
	return seen
}

// identityShowsTag reports whether a shown text's identity region (see identityRegion) shows
// a reader a login followed by a tag that the lint does not read as App-run. A tag is a tag
// bracket opened directly after a character a login may end in (see tagAfter) and closed
// later in the region. The region shows one when a tag holds exactly three letters or digits,
// as `[bot]` does, or when a tag holds three or more letters, digits or symbols and any
// character in the region is outside the login alphabet (see loginByte). Only a region whose
// letters are all CJK text (see cjkOnly) is exempt. No character is folded to or matched
// against another, so a character no check names still fails closed.
func identityShowsTag(s string) bool {
	ir := identityRegion(s)
	open, words, glyphs, prev := false, 0, 0, rune(-1)
	three, wide, foreign := false, false, false
	for _, r := range ir {
		if !loginByte(r) {
			foreign = true
		}
		after := prev
		prev = r
		switch {
		case !open && tagAfter(after) && tagOpen(r):
			open, words, glyphs = true, 0, 0
		case open && tagClose(r):
			three, wide, open = three || words == 3, wide || glyphs >= 3, false
		case open && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			words++
			glyphs++
		case open && unicode.IsSymbol(r):
			glyphs++
		}
	}
	return (three || wide && foreign) && !cjkOnly(ir)
}

// showsAppRun reports whether a cell shows a reader an App-run Runner cell in any of its
// shown texts, as the lint reads one or as a reader does (see identityShowsTag).
func showsAppRun(cell string) bool {
	for _, t := range shownTexts(cell) {
		if isAppRunnerCell(t) || identityShowsTag(t) {
			return true
		}
	}
	return false
}

// showsAttribution reports whether a line shows a reader App-run or on-behalf-of text in any
// of its shown texts: a `[bot]` tag or the on-behalf-of marker anywhere, or a word that shows
// as App-run (see identityShowsTag). A word starts the text or follows a space, a tab or a `(`,
// since identityRegion ends a region at a space or `(` and a word in parentheses shows as one.
func showsAttribution(line string) bool {
	for _, t := range shownTexts(line) {
		if l := strings.ToLower(t); strings.Contains(l, "[bot]") || strings.Contains(l, onBehalfOfCellMarker) {
			return true
		}
		for i := 0; i < len(t); i++ {
			if (i == 0 || t[i-1] == ' ' || t[i-1] == '\t' || t[i-1] == '(') && identityShowsTag(t[i:]) {
				return true
			}
		}
	}
	return false
}

// linkSyntax reports whether s holds Markdown link, image or autolink syntax a renderer shows
// as other text.
func linkSyntax(s string) bool {
	return reduceLinks(s, true) != s
}

// lintTableAdds reports whether a landing adds the lint-read header at view index h of a
// section, its separator, or any row the lint reads under it (the `|`-led lines after it).
func lintTableAdds(view, lines []string, h int, onBranch map[string]bool) bool {
	for j := h; j < len(view); j++ {
		if j > h+1 && !strings.HasPrefix(strings.TrimSpace(view[j]), "|") {
			break
		}
		if !onBranch[lines[j]] {
			return true
		}
	}
	return false
}

// headerCells returns a header row's cells as both readings compare them: trimmed, lowercased.
func headerCells(line string, spans [][2]int) []string {
	out := make([]string, len(spans))
	for j, sp := range spans {
		out[j] = strings.ToLower(strings.TrimSpace(line[sp[0]:sp[1]]))
	}
	return out
}

// gfmDelimiter reports whether line is a delimiter row a GFM renderer accepts under a header
// of n cells: every cell dashes with optional edge colons, and exactly n cells.
func gfmDelimiter(line string, n int) bool {
	spans := cellSpans(strings.TrimSpace(line))
	if len(spans) != n {
		return false
	}
	t := strings.TrimSpace(line)
	for _, sp := range spans {
		if !gfmDelimiterCellRe.MatchString(strings.TrimSpace(t[sp[0]:sp[1]])) {
			return false
		}
	}
	return true
}

// evidenceLayoutProblem is the positive check on the landed Evidence section lines[start:end]:
// it returns the first line the landing adds (one not in onBranch) that is not a blank line,
// a row of a table both readings agree on, or a note line neither reads as a row, and why,
// or "" when every added line is one of those. Both readings agree on a table when:
//   - its header is a `|`-led line the lint reads as a header (a separator line follows it)
//     and its delimiter row is one a GFM renderer accepts, with as many cells as the header
//     (checked whenever the landing adds any line of that table);
//   - every row is a `|`-led line, and no line in the middle of it reads to the lint as a
//     separator or as a new header (other than a repeat of the table's own header, which the
//     lint reads with the same Runner column);
//   - an added row is no wider than its header when the header has no `Runner` column (the
//     lint then reads the row's last cell, which a renderer drops), and only the cell the
//     lint reads as Runner shows as an App-run Runner cell.
//
// A note line is any other line outside a table (prose after a blank line, a heading), or a
// pipe-less line closing a table that shows no App-run or on-behalf-of text (a renderer shows
// it as a one-cell row). A `|`-led line under no header is shown as text and read by the lint
// with its last cell as Runner, so it is let through only when no other cell shows as an
// App-run Runner cell, and only when it is not a separator line (which the lint reads only
// directly under a header). An added line may also not hold a raw HTML table or heading tag,
// share its line with an HTML comment that crosses a line break, or be a table line inside a
// block quote. Lines a renderer reads as rows without a leading `|` are renderedRowMismatch's.
func evidenceLayoutProblem(lines []string, start, end int, onBranch map[string]bool) (int, string) {
	section := strings.Join(lines[start:end], "\n")
	joined := map[int]bool{}
	for _, m := range evidenceCommentRe.FindAllStringIndex(section, -1) {
		first := strings.Count(section[:m[0]], "\n")
		last := first + strings.Count(section[m[0]:m[1]], "\n")
		for l := first; last > first && l <= last; l++ {
			joined[l] = true
		}
	}
	view := strings.Split(blankComments(section), "\n")
	added := func(k int) bool { return !onBranch[lines[start+k]] }
	separator := func(k int) bool {
		return k < len(view) && evidenceSeparatorRe.MatchString(strings.Trim(strings.TrimSpace(view[k]), "|"))
	}
	// gfmTableAdds: whether the landing adds a line from k to the end of the table a renderer
	// reads (the next blank line or other block).
	gfmTableAdds := func(k int) bool {
		for j := k; j < len(view); j++ {
			if t := strings.TrimSpace(view[j]); t == "" || startsOtherBlock(t) {
				break
			}
			if added(j) {
				return true
			}
		}
		return false
	}
	inTable, split := false, false
	cols, runner := 0, -1
	var header []string
	for k := 0; k < len(view); k++ {
		i := start + k
		t := strings.TrimSpace(view[k])
		if added(k) {
			if rawTableTagRe.MatchString(lines[i]) {
				return i, "it holds a raw HTML table or heading tag, which a renderer shows as a table or heading the lint never reads"
			}
			if joined[k] && t != "" {
				return i, "an HTML comment that crosses a line break shares the line with other text, and the lint reads the lines around it as one"
			}
		}
		if t == "" || startsOtherBlock(t) {
			if added(k) && strings.HasPrefix(t, ">") && hasUnescapedPipe(t) {
				return i, "it is a table line inside a block quote, which a renderer shows as a table the lint never reads"
			}
			inTable = false
			continue
		}
		piped := strings.HasPrefix(t, "|")
		if !inTable {
			if !piped {
				continue
			}
			if separator(k) {
				if added(k) {
					return i, "the lint reads it as a table separator, but no table header stands directly above it"
				}
				continue
			}
			if !separator(k + 1) {
				// No header: a renderer shows the line as text, and the lint reads it as a row
				// whose Runner cell is its last cell, so only that cell may show as App-run.
				if spans := cellSpans(view[k]); added(k) {
					for j, sp := range spans[:max(len(spans)-1, 0)] {
						if showsAppRun(view[k][sp[0]:sp[1]]) || showsAppRun(lines[i][sp[0]:sp[1]]) {
							return i, fmt.Sprintf("it begins with `|` under no table header, so the lint reads its Runner "+
								"cell from its last cell, but its cell %d shows as an App-run Runner cell", j+1)
						}
					}
				}
				continue
			}
			spans := cellSpans(view[k])
			if (added(k) || gfmTableAdds(k+1)) && !gfmDelimiter(view[k+1], len(spans)) {
				return i + 1, fmt.Sprintf("the lint reads it as the delimiter row of the %d-column header above it, but a "+
					"renderer accepts a delimiter row only of dashes with optional edge colons, one cell per header cell, "+
					"so it shows no table there", len(spans))
			}
			inTable, split = true, false
			cols, runner, header = len(spans), -1, headerCells(view[k], spans)
			for j, sp := range spans {
				if strings.ToLower(strings.TrimSpace(view[k][sp[0]:sp[1]])) == "runner" {
					runner = j
				}
			}
			k++
			continue
		}
		if !piped {
			if added(k) && showsAttribution(lines[i]) {
				return i, "it closes a table without a blank line, so a renderer shows it as a one-cell row, and it shows App-run or on-behalf-of text the lint never reads as a row"
			}
			continue
		}
		if separator(k) {
			if added(k) {
				return i, "the lint reads it as a table separator, but it stands inside a table, where a renderer shows it as a row"
			}
			continue
		}
		if separator(k + 1) {
			// A repeat of the table's own header reads the same to both: the lint takes its
			// Runner column from the same cell a renderer shows rows under.
			if spans := cellSpans(view[k]); slices.Equal(headerCells(view[k], spans), header) {
				k++
				continue
			}
			if added(k) || added(k+1) {
				return i, "the lint reads it as a new table header, since a separator line follows it, but a renderer " +
					"shows it as a row of the table above, whose header differs; put a blank line above it to start a new table"
			}
			split = true
			continue
		}
		if !added(k) {
			continue
		}
		if split {
			return i, "the lint reads it under a header a renderer shows as a row, so the two read its Runner cell from different headers"
		}
		spans := cellSpans(view[k])
		if runner < 0 && len(spans) > cols {
			return i, fmt.Sprintf("it has %d cells under a %d-column header with no `Runner` column, so the lint reads "+
				"its Runner cell from its last cell, which a renderer drops", len(spans), cols)
		}
		r := len(spans) - 1
		if runner >= 0 && runner < len(spans) {
			r = runner
		}
		for j, sp := range spans {
			if j != r && (showsAppRun(view[k][sp[0]:sp[1]]) || showsAppRun(lines[i][sp[0]:sp[1]])) {
				return i, fmt.Sprintf("its cell %d shows as an App-run Runner cell, but the lint reads its Runner cell "+
					"from cell %d", j+1, r+1)
			}
		}
	}
	return 0, ""
}

// outsideEvidenceProblem returns the first body line outside the lint's Evidence section
// lines[start:end] (and its heading) that the landing adds (not in onBranch) and that shows
// a reader Evidence the lint never reads — a line holding a `|` with a cell that shows as an
// App-run Runner cell, or a raw HTML table or heading tag outside a code span — and why, or ""
// when there is none. Lines in fenced code show as code and are skipped. start and end are -1
// when the brief has no section the lint reads; every body line is then checked.
func outsideEvidenceProblem(lines []string, start, end int, onBranch map[string]bool) (int, string) {
	from, _ := briefBodyStart(lines)
	fence := ""
	for i := from; i < len(lines); i++ {
		// A fenced code block shows as code: no table, no heading.
		if f := codeFence(lines[i]); f != "" && (fence == "" || (f[0] == fence[0] && len(f) >= len(fence) &&
			strings.Trim(strings.TrimSpace(lines[i]), f[:1]) == "")) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
			continue
		}
		if fence != "" || (start >= 0 && i >= start-1 && i < end) || onBranch[lines[i]] {
			continue
		}
		if rawTableTagRe.MatchString(outsideCodeSpans(lines[i])) {
			return i, "it holds a raw HTML table or heading tag, which a renderer shows as a table or heading the lint never reads"
		}
		t := strings.TrimSpace(lines[i])
		if !hasUnescapedPipe(t) {
			continue
		}
		for _, sp := range cellSpans(t) {
			if showsAppRun(t[sp[0]:sp[1]]) {
				return i, "it shows an App-run Runner cell in a table row, and the lint reads Evidence only in the first `## Evidence` section"
			}
		}
	}
	return 0, ""
}

// outsideCodeSpans returns line without its inline code spans, whose text a renderer shows as
// code; a line holding a backslash (which can escape a backtick) is returned whole.
func outsideCodeSpans(line string) string {
	if strings.Contains(line, "\\") {
		return line
	}
	return codeSpanRe.ReplaceAllString(line, "")
}

// outsideRefusal is the refusal for an outsideEvidenceProblem line.
func outsideRefusal(i int, why string, lines []string) error {
	return deskkit.Refused(fmt.Sprintf("refused: the added line %d is outside the Evidence section the lint reads (%s), "+
		"so it would never be checked or stamped — nothing was written. Land every row in the one `## Evidence` "+
		"section: %s", i+1, why, strings.TrimSpace(lines[i])))
}

// unreadableHeader reports why an added Evidence header row cannot be read the way the lint
// reads it, or "" when it can. view is the header with closed comments blanked, orig its bytes
// as landed, spans its cells in view. The lint takes the Runner column from the last cell that
// trims and lowercases to exactly `runner`, and otherwise from the row's last cell; a header a
// reader sees differently can make the column shown as Runner not the one the lint reads.
//   - An invisible format character (a bidi isolate, a zero-width space): a cell can show as
//     `Runner` and read as something else.
//   - A non-ASCII letter or mark: a homoglyph can show as `Runner`. Non-ASCII punctuation and
//     symbols (`Date · Runner`) are left alone.
//   - A cell whose letters spell `runner` but that the lint does not read as exactly `runner`
//     (`**Runner**`, `Run<!-- -->ner`): a reader takes it as the Runner column; the lint does not.
//   - More than one `Runner` cell: the lint reads the last, a reader may take the first.
func unreadableHeader(view, orig string, spans [][2]int) string {
	for i := 0; i < len(orig); {
		r, size := utf8.DecodeRuneInString(orig[i:])
		i += size
		switch {
		case r == utf8.RuneError && size == 1:
			return "it is not valid UTF-8"
		case unicode.Is(unicode.Cf, r):
			return fmt.Sprintf("it holds the invisible format character U+%04X", r)
		case r > unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsMark(r)):
			return fmt.Sprintf("it holds the non-ASCII letter or mark U+%04X, which can show as an ASCII one", r)
		}
	}
	runners := 0
	for _, sp := range spans {
		cell := strings.ToLower(strings.TrimSpace(view[sp[0]:sp[1]]))
		if cell == "runner" {
			runners++
			continue
		}
		if lettersOf(cell) == "runner" {
			return fmt.Sprintf("its cell %q renders as Runner, but the lint does not read it as the Runner column",
				strings.TrimSpace(view[sp[0]:sp[1]]))
		}
	}
	if runners > 1 {
		return "it has more than one Runner column; the lint reads the last, and a reader may take the first"
	}
	return ""
}

// lettersOf returns s's letters, lowercased, everything else dropped.
func lettersOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// renderedRowMismatch returns the first line of the Evidence section lines[start:end] that a
// Markdown renderer reads as a table row the lint does not read the same way, and why, or ""
// when there is none. Only lines this landing adds (not in onBranch) are refused. A renderer
// (GFM) reads a table from a header line holding a `|` followed by a delimiter row, through
// every following line until a blank line or the start of another block, whether or not a
// line begins with `|`; the lint reads only `|`-led lines, and forgets the table's header at
// the first line that is not one.
//   - A line in a table that does not begin with `|` but holds one renders as a row (or a
//     header, or a delimiter row) the lint never reads. A pipe-less note line ending a table
//     renders as a one-cell row with no Runner cell, and is left alone.
//   - A `|`-led line after such a line in the same table renders under the table's header,
//     but the lint has dropped that header and reads the row's last cell as its Runner cell.
//     A repeat of the table's own header (with a separator line under it) is the exception:
//     the lint reads its Runner column from the same cell a renderer shows rows under, so the
//     rows after it read the same to both (a witness table appended after an attestation line).
//     So is a row with exactly as many cells as the table's header when that header's Runner
//     column is its last cell, or it has none: both readings then take the row's last cell as
//     its Runner cell (rows appended without a header after an attestation line).
func renderedRowMismatch(lines []string, start, end int, onBranch map[string]bool) (int, string) {
	inTable, lost := false, false
	var header []string
	for i := start; i < end; i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || startsOtherBlock(t) {
			inTable, lost = false, false
			continue
		}
		piped := hasUnescapedPipe(t)
		if !inTable {
			if !piped || i+1 >= end || !isDelimiterRow(lines[i+1]) {
				continue
			}
			inTable, lost, header = true, false, headerCells(t, cellSpans(t))
		}
		added := !onBranch[lines[i]]
		if !strings.HasPrefix(t, "|") {
			if added && piped {
				return i, "a renderer reads it as a table row, but it does not begin with `|`, so the lint never reads it"
			}
			lost = true
			continue
		}
		if lost && i+1 < end && evidenceSeparatorRe.MatchString(strings.Trim(strings.TrimSpace(lines[i+1]), "|")) {
			if slices.Equal(headerCells(t, cellSpans(t)), header) {
				lost = false
				i++
				continue
			}
			// Refused when the landing adds this header, its separator, or any later line of the
			// table a renderer reads; a header already on the branch with nothing added under it
			// does not block other landings.
			for j := i; j < end; j++ {
				if u := strings.TrimSpace(lines[j]); u == "" || startsOtherBlock(u) {
					break
				}
				if !onBranch[lines[j]] {
					return i, "the lint reads it as a new table header, but no blank line separates it from the table " +
						"above, so a renderer shows it as a row under that table's different header; put a blank line above it"
				}
			}
		}
		if lost && added && !lastCellRunner(header, len(cellSpans(t))) {
			return i, "a renderer reads it as a table row under the table's header, but a line above it in the same " +
				"table does not begin with `|`, so the lint has dropped that header and reads the row's last cell as " +
				"its Runner cell; repeat the table's header and its delimiter row above the rows"
		}
	}
	return 0, ""
}

// lastCellRunner reports whether a row of n cells under a header with cells header (trimmed,
// lowercased) has its Runner cell at its last cell to both readings: the row is as wide as the
// header (a renderer pads a narrower row and drops a wider one's extra cells), and the column
// the lint takes from the header (its last `runner` cell, else its last cell) is the last one.
func lastCellRunner(header []string, n int) bool {
	if n != len(header) || n == 0 {
		return false
	}
	for j := len(header) - 1; j >= 0; j-- {
		if header[j] == "runner" {
			return j == n-1
		}
	}
	return true
}

// startsOtherBlock reports whether a trimmed line opens a block that ends a table: an ATX
// heading, a code fence or a block quote.
func startsOtherBlock(t string) bool {
	if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") || strings.HasPrefix(t, ">") {
		return true
	}
	return atxHeadingRe.MatchString(t)
}

// isDelimiterRow reports whether a line can be a table's delimiter row: dashes, colons, pipes
// and spaces only, with at least one dash.
func isDelimiterRow(line string) bool {
	t := strings.TrimSpace(line)
	return t != "" && strings.Contains(t, "-") && evidenceSeparatorRe.MatchString(t)
}

// hasUnescapedPipe reports whether s holds a `|` not escaped as `\|` (cellSpans' reading).
func hasUnescapedPipe(s string) bool {
	return len(cellSpans("x"+s+"x")) > 1
}

var (
	// atxHeadingRe matches an ATX heading line (trimmed); group 1 is its text with any closing
	// sequence removed.
	atxHeadingRe = regexp.MustCompile(`^#{1,6}(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$`)
	// setextUnderlineRe matches a setext heading's underline.
	setextUnderlineRe = regexp.MustCompile(`^ {0,3}(?:=+|-+)[ \t]*$`)
)

// htmlHeadingRe matches a raw HTML heading's opening tag; group 1 is its level, group 2 the
// rest of the line.
var htmlHeadingRe = regexp.MustCompile(`(?i)<h([1-6])\b[^>]*>(.*)`)

// bodyHeading is one heading in a brief's body: its line, its level, and its text as written.
type bodyHeading struct {
	line, level int
	text        string
}

// bodyHeadings returns every heading in the body: ATX, setext, and raw HTML `<h1>`–`<h6>`.
// Lines are also split on a lone CR, which a renderer can treat as a line break, so a heading
// the lint reads as part of another line counts too. A line inside a fenced code block, or
// indented four spaces or a tab (indented code), is shown as code, never as a heading.
func bodyHeadings(lines []string) []bodyHeading {
	from, _ := briefBodyStart(lines)
	type piece struct {
		line int
		text string
	}
	var pieces []piece
	for i := from; i < len(lines); i++ {
		for _, p := range strings.Split(lines[i], "\r") {
			pieces = append(pieces, piece{i, p})
		}
	}
	var out []bodyHeading
	fence := ""
	for k, p := range pieces {
		t := strings.TrimSpace(p.text)
		if f := codeFence(p.text); f != "" && (fence == "" || (f[0] == fence[0] && len(f) >= len(fence) && strings.Trim(t, f[:1]) == "")) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
			continue
		}
		if fence != "" || strings.HasPrefix(p.text, "    ") || strings.HasPrefix(p.text, "\t") {
			continue
		}
		if m := atxHeadingRe.FindStringSubmatch(t); m != nil {
			out = append(out, bodyHeading{p.line, len(t) - len(strings.TrimLeft(t, "#")), m[1]})
		} else if m := htmlHeadingRe.FindStringSubmatch(t); m != nil {
			out = append(out, bodyHeading{p.line, int(m[1][0] - '0'), m[2]})
		} else if k+1 < len(pieces) && t != "" && setextUnderlineRe.MatchString(pieces[k+1].text) {
			level := 2
			if strings.Contains(pieces[k+1].text, "=") {
				level = 1
			}
			out = append(out, bodyHeading{p.line, level, t})
		}
	}
	return out
}

// codeFence returns the backtick or tilde run that opens or closes a fenced code block on
// line (indented at most three spaces), or "" when line is not a fence.
func codeFence(line string) string {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 || (!strings.HasPrefix(t, "```") && !strings.HasPrefix(t, "~~~")) {
		return ""
	}
	n := len(t) - len(strings.TrimLeft(t, t[:1]))
	return t[:n]
}

// headingShowsEvidence reports whether a heading's text shows a reader `Evidence`: its letters
// spell `evidence` once entities are decoded, HTML tags removed and link or image syntax
// reduced to its shown text, with closed comments either removed or (as inside a code span)
// shown.
func headingShowsEvidence(text string) bool {
	for _, s := range commentReadings(text) {
		for _, v := range linkReadings(s) {
			if lettersOf(html.UnescapeString(htmlTagRe.ReplaceAllString(v, ""))) == "evidence" {
				return true
			}
		}
	}
	return false
}

// unreadEvidenceHeadings returns every body heading a renderer shows as Evidence — an ATX,
// setext or HTML heading of any level whose shown letters spell `evidence` — other than the
// `## Evidence` line the lint reads as the section.
func unreadEvidenceHeadings(lines []string) []bodyHeading {
	lintHeading := -1
	if start, _, ok := evidenceSpan(lines); ok {
		lintHeading = start - 1
	}
	var out []bodyHeading
	for _, h := range bodyHeadings(lines) {
		if h.line == lintHeading || (len(out) > 0 && out[len(out)-1].line == h.line) {
			continue
		}
		if headingShowsEvidence(h.text) {
			out = append(out, h)
		}
	}
	return out
}

// unreadEvidenceAddition reports a heading a renderer shows as Evidence, other than the lint's
// section, that the landing adds (more of that heading line than the base holds), or one under
// which the landing adds a non-blank line outside the lint's section: at is the heading's
// line, under the added line (at itself when the heading is the addition). A heading's lines
// run to the next heading of the same or a higher level.
func unreadEvidenceAddition(lines, baseLines []string, onBranch map[string]bool) (at, under int, ok bool) {
	baseCount := map[string]int{}
	for _, h := range unreadEvidenceHeadings(baseLines) {
		baseCount[baseLines[h.line]]++
	}
	start, end, hasSection := evidenceSpan(lines)
	all := bodyHeadings(lines)
	for _, h := range unreadEvidenceHeadings(lines) {
		if baseCount[lines[h.line]]--; baseCount[lines[h.line]] < 0 || !onBranch[lines[h.line]] {
			return h.line, h.line, true
		}
		stop := len(lines)
		for _, n := range all {
			if n.line > h.line && n.level <= h.level {
				stop = n.line
				break
			}
		}
		for j := h.line + 1; j < stop; j++ {
			if hasSection && j >= start && j < end {
				continue
			}
			if strings.TrimSpace(lines[j]) != "" && !onBranch[lines[j]] {
				return h.line, j, true
			}
		}
	}
	return 0, 0, false
}

// addedHeadingLetter returns the first body heading line the landing adds (not in onBranch)
// whose shown text holds a non-ASCII letter or mark, and that rune. A homoglyph can make a
// heading show as Evidence while its letters spell something else.
func addedHeadingLetter(lines []string, onBranch map[string]bool) (int, rune, bool) {
	for _, h := range bodyHeadings(lines) {
		if onBranch[lines[h.line]] {
			continue
		}
		for _, r := range html.UnescapeString(h.text) {
			if r > unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsMark(r)) {
				return h.line, r, true
			}
		}
	}
	return 0, 0, false
}

// rowControlCharacter returns the first control character (C0, DEL or C1, other than a tab,
// and a line's trailing CR) in an added line of the Evidence section, whatever kind of line it
// is. The lint splits lines on LF only, so a lone CR or other control character keeps what a
// renderer may show as two lines (a header that hides a data row, a non-table line that holds a
// whole table) in the one line the lint reads; a row shown that way would be visible and never
// checked.
func rowControlCharacter(line string) (rune, bool) {
	for _, r := range strings.TrimSuffix(line, "\r") {
		if r != '\t' && unicode.IsControl(r) {
			return r, true
		}
	}
	return 0, false
}

// landedPrincipalProblem reports how a landed App-run Runner cell, as the lint reads it, fails
// to carry exactly the resolved stamp, or "" when it does.
func landedPrincipalProblem(runner, subject string) string {
	p, ok := onBehalfOfPrincipalOf(runner)
	switch {
	case !ok:
		return "carries no on-behalf-of principal the lint reads"
	case p != strings.ToLower(subject):
		return fmt.Sprintf("names principal %q, not the one the roster resolves for this target", p)
	case strings.Count(strings.ToLower(runner), onBehalfOfCellMarker) != 1 || strings.Count(runner, canonicalStamp(subject)) != 1:
		return "does not carry exactly one annotation, in the exact form this tool writes"
	}
	return ""
}

type lintRunner struct {
	line   string // the row, trimmed, as the lint reads it
	runner string // its Runner cell, trimmed
}

// lintEvidenceRunners reads an Evidence section's data rows the way statusgen's
// parseEvidenceRows does — closed HTML comments REMOVED (stripRowComments), rows trimmed, a
// header being the row followed by a separator, the Runner column by its header name or else
// the last cell — and returns each row with its Runner cell.
func lintEvidenceRunners(section string) []lintRunner {
	var out []lintRunner
	lines := strings.Split(evidenceCommentRe.ReplaceAllString(section, ""), "\n")
	runnerIdx := -1
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			runnerIdx = -1
			continue
		}
		if evidenceSeparatorRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		spans := cellSpans(line)
		if i+1 < len(lines) && evidenceSeparatorRe.MatchString(strings.Trim(strings.TrimSpace(lines[i+1]), "|")) {
			runnerIdx = -1
			for j, sp := range spans {
				if strings.ToLower(strings.TrimSpace(line[sp[0]:sp[1]])) == "runner" {
					runnerIdx = j
				}
			}
			i++
			continue
		}
		if len(spans) == 0 {
			continue
		}
		sp := spans[len(spans)-1]
		if runnerIdx >= 0 && runnerIdx < len(spans) {
			sp = spans[runnerIdx]
		}
		out = append(out, lintRunner{line: line, runner: strings.TrimSpace(line[sp[0]:sp[1]])})
	}
	return out
}

// isAppRunnerCell reports whether a trimmed Runner cell names an App identity — a leading
// `<slug>[bot]` identifier — rather than a human runner (statusgen's isAppRunnerToken /
// isHumanRunnerToken reading).
func isAppRunnerCell(runner string) bool {
	if runner == "" || strings.HasPrefix(strings.ToLower(runner), "human:") {
		return false
	}
	id := runner
	if i := strings.IndexAny(id, " ("); i >= 0 {
		id = id[:i]
	}
	return strings.HasSuffix(id, "[bot]")
}

// stampVerdict is what the stamp does with one added App-run Runner cell.
type stampVerdict int

const (
	stampAdd     stampVerdict = iota // no on-behalf-of text: append the resolved annotation
	stampLeave                       // exactly the resolved exact-form annotation: byte-identical
	stampRestamp                     // a non-principal annotation: strip every fragment, then stamp
	stampRefuse                      // names a different recognised human: refuse, write nothing
)

// needsPrincipalStamp classifies an App-run Runner cell (comment-blanked) against the subject
// the roster resolved for this target — see the file comment for the rule. On stampRefuse it
// also returns the other human's principal as the cell names it.
//
// The annotation is read with the lint's own parse (onBehalfOfPrincipalOf: the LAST
// occurrence, case-insensitive). Recognition reads the same roster as the subject's
// resolution (deskkit.EffectiveConfig), so it never depends on the target's lint config. A
// cell is left only when its one annotation is, byte for byte, the exact form this tool
// writes; any other annotation naming the subject (other case, extra words inside the
// parenthetical) is restamped.
func needsPrincipalStamp(cell, subject string) (stampVerdict, string) {
	lower := strings.ToLower(cell)
	n := strings.Count(lower, onBehalfOfCellMarker)
	if n == 0 {
		return stampAdd, ""
	}
	subjectLogin := rosterHumanLogin(subject)
	// Every `human:` fragment in the cell, not only the one the lint reads: an earlier
	// fragment naming another recognised human is still that human's name, and stripping it
	// would be the overwrite this rule forbids.
	for _, p := range onBehalfOfPrincipals(cell) {
		if login := rosterHumanLogin(p); login != "" && login != subjectLogin {
			return stampRefuse, p
		}
	}
	if p, ok := onBehalfOfPrincipalOf(cell); ok && n == 1 && p == strings.ToLower(strings.TrimSpace(subject)) &&
		strings.Count(cell, canonicalStamp(subject)) == 1 {
		return stampLeave, ""
	}
	return stampRestamp, ""
}

// rosterHumanLogin returns the login a principal names when the roster recognises it as a
// human — a login in the human set, or a neutral name the human-login map carries for one —
// and "" otherwise. KEEP IN SYNC with statusgen's onBehalfOfPrincipalRecognised
// (statusgen/principal.go), which accepts exactly these two forms.
func rosterHumanLogin(principal string) string {
	p := strings.ToLower(strings.TrimSpace(principal))
	if p == "" {
		return ""
	}
	c := deskkit.EffectiveConfig()
	if _, ok := c.Humans[p]; ok {
		return p
	}
	if login, ok := c.HumanLogins[p]; ok {
		if _, human := c.Humans[login]; human {
			return login
		}
	}
	return ""
}

// onBehalfOfPrincipalOf is statusgen's parse of a cell's annotation (statusgen/principal.go,
// onBehalfOfPrincipalOf), copied so the stamp classifies a cell exactly as the lint reads it:
// the LAST case-insensitive `on-behalf-of human:` occurrence; the principal runs to `)`,
// whitespace or `|`, lowercased. KEEP IN SYNC — TestPrincipalStampLintParity runs the real
// lint against this reading.
func onBehalfOfPrincipalOf(cell string) (string, bool) {
	const marker = onBehalfOfCellMarker + " human:"
	idx := strings.LastIndex(strings.ToLower(cell), marker)
	if idx < 0 {
		return "", false
	}
	p := principalAt(cell[idx+len(marker):])
	return p, p != ""
}

// onBehalfOfPrincipals returns the principal of EVERY `on-behalf-of human:` occurrence in a
// cell, in order, with onBehalfOfPrincipalOf's per-occurrence parse.
func onBehalfOfPrincipals(cell string) []string {
	const marker = onBehalfOfCellMarker + " human:"
	var out []string
	lower := strings.ToLower(cell)
	for off := 0; ; {
		idx := strings.Index(lower[off:], marker)
		if idx < 0 {
			return out
		}
		at := off + idx + len(marker)
		if p := principalAt(cell[at:]); p != "" {
			out = append(out, p)
		}
		off = at
	}
}

func principalAt(rest string) string {
	end := 0
	for end < len(rest) {
		c := rest[end]
		if c == ')' || c == ' ' || c == '\t' || c == '\n' || c == '|' {
			break
		}
		end++
	}
	return strings.ToLower(strings.TrimSpace(rest[:end]))
}

// onBehalfOfFragments returns the [start,end) byte spans, in DESCENDING order (so they can
// be cut one after another without shifting each other), of every on-behalf-of fragment in a
// Runner cell, each with the one whitespace byte before it (if any):
//   - a fragment opened by `(` (whitespace allowed between) runs to the next `)` inclusive;
//     an unclosed one (no `)` before the next `(`) ends where a bare fragment would, so it
//     never reaches into a following parenthetical such as a runner-source tag;
//   - a bare fragment runs to the next `(` or `)` (trailing whitespace left in place), or to
//     the end of the cell's text.
//
// The cell's own trailing padding is never part of a fragment.
func onBehalfOfFragments(cell string) [][2]int {
	lower := strings.ToLower(cell)
	textEnd := len(strings.TrimRight(cell, " \t"))
	var spans [][2]int
	for off := 0; off < textEnd; {
		idx := strings.Index(lower[off:], onBehalfOfCellMarker)
		if idx < 0 {
			break
		}
		idx += off
		start, end := idx, textEnd
		j := idx - 1
		for j >= off && (cell[j] == ' ' || cell[j] == '\t') {
			j--
		}
		closeAt, openAt := strings.IndexByte(cell[idx:textEnd], ')'), strings.IndexByte(cell[idx:textEnd], '(')
		if j >= off && cell[j] == '(' && closeAt >= 0 && (openAt < 0 || closeAt < openAt) {
			start = j
			end = idx + closeAt + 1
		} else {
			if j >= off && cell[j] == '(' {
				start = j // unclosed: the `(` goes with it, the end is a bare fragment's
			}
			after := idx + len(onBehalfOfCellMarker)
			if k := strings.IndexAny(cell[after:textEnd], "()"); k >= 0 {
				end = after + k
			}
			for end > start && (cell[end-1] == ' ' || cell[end-1] == '\t') {
				end--
			}
		}
		// Exactly ONE whitespace byte before a fragment goes with it, and a restamp writes
		// that byte back before the new stamp (stampPosition). So stripping an old exact-form
		// stamp and inserting the new one leaves every other byte of the row as it was.
		if start > off && (cell[start-1] == ' ' || cell[start-1] == '\t') {
			start--
		}
		spans = append(spans, [2]int{start, end})
		if end <= idx {
			end = idx + len(onBehalfOfCellMarker)
		}
		off = end
	}
	for a, b := 0, len(spans)-1; a < b; a, b = a+1, b-1 {
		spans[a], spans[b] = spans[b], spans[a]
	}
	return spans
}

// trailingSourceTag reports where the runner-source tag a right-trimmed Runner cell ends with
// starts, if it ends with one — after a space, a tab or nothing, in any letter case. An added annotation goes
// before it — verifyrun's rendering order — so the tag is never rewritten.
func trailingSourceTag(runner string) (int, bool) {
	lower := strings.ToLower(runner)
	for _, tag := range runnerSourceTags {
		if len(lower) == len(runner) && strings.HasSuffix(lower, tag) {
			return len(runner) - len(tag), true
		}
	}
	return 0, false
}

// evidenceSpan locates the `## Evidence` section's lines: [start,end), end being the next
// `## ` heading or the end of content (statusgen's evidenceSectionSpan). Like the lint, it
// looks only in the BODY: a leading frontmatter block (briefBodyStart) is skipped, so a
// `## Evidence` line inside the frontmatter (a YAML comment there) is never taken as the
// section. A frontmatter block that never closes has no body the lint can read: ok is false.
func evidenceSpan(lines []string) (start, end int, ok bool) {
	from, closed := briefBodyStart(lines)
	if !closed {
		return 0, 0, false
	}
	start = -1
	for i := from; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "## Evidence" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	end = len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return start, end, true
}

// briefBodyStart returns the index of the first body line: the line after the closing `---`
// of a frontmatter block that opens on the first line (statusgen's splitFrontmatter), or 0
// for content with no frontmatter. closed is false when the first line opens a frontmatter
// block that never closes.
func briefBodyStart(lines []string) (from int, closed bool) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0, true
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 1, true
		}
	}
	return 0, false
}

// hasEvidenceHeading reports whether any line is the `## Evidence` heading, wherever it sits.
func hasEvidenceHeading(lines []string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			return true
		}
	}
	return false
}

// cellSpans returns each table cell's [start,end) byte offsets in line, splitting on `|` but
// not on an escaped `\|`, and dropping at most one blank cell at each end — the cell
// boundaries statusgen's splitRowEscaped reads.
func cellSpans(line string) [][2]int {
	var spans [][2]int
	cur := 0
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			i++
		case line[i] == '|':
			spans = append(spans, [2]int{cur, i})
			cur = i + 1
		}
	}
	spans = append(spans, [2]int{cur, len(line)})
	if len(spans) > 0 && strings.TrimSpace(line[spans[0][0]:spans[0][1]]) == "" {
		spans = spans[1:]
	}
	if n := len(spans); n > 0 && strings.TrimSpace(line[spans[n-1][0]:spans[n-1][1]]) == "" {
		spans = spans[:n-1]
	}
	return spans
}

// stampFragment applies stampEvidencePrincipal's line map to a --brief-path fragment, so the
// fragment merged into the brief is the stamped one and the block-level idempotency check
// compares like with like.
func stampFragment(fragment []byte, stamped map[string]string) []byte {
	lines := strings.Split(string(fragment), "\n")
	for i, l := range lines {
		key := strings.TrimSpace(l)
		if to, ok := stamped[key]; ok {
			lines[i] = strings.Replace(l, key, to, 1)
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
