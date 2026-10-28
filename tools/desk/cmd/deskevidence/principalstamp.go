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
	"regexp"
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
	start, end, ok := evidenceSpan(lines)
	if !ok {
		return content, nil, nil
	}
	onBranch := map[string]bool{}
	baseLines := strings.Split(string(base), "\n")
	for _, l := range baseLines {
		onBranch[l] = true
	}
	// Every line this landing adds to the Evidence section, of any kind (data row, header,
	// separator, or a line that is not a table row at all), is checked for a control character
	// before any line is skipped below. A line counts as already on the branch here only when it
	// stands in the base's own Evidence section, so a line carried in from elsewhere in the base
	// file is checked like any other added line.
	onBranchEvidence := map[string]bool{}
	if bs, be, ok := evidenceSpan(baseLines); ok {
		for _, l := range baseLines[bs:be] {
			onBranchEvidence[l] = true
		}
	}
	for i := start; i < end; i++ {
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
	blanked := evidenceCommentRe.ReplaceAllStringFunc(section, func(m string) string {
		b := []byte(m)
		for i := range b {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
		return string(b)
	})
	view := strings.Split(blanked, "\n")

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
	if stamped == nil {
		return content, nil, nil
	}
	return []byte(strings.Join(lines, "\n")), stamped, nil
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
	return ""
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
