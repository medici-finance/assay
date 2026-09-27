package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forktest.go — the `### Fork test` block a `needs-decision` filing must carry: the options that can actually work, the default, the human-held
// gate that would catch a wrong guess on that default, and the search that showed the
// question was not already ruled. Pure parser: no I/O, no clock, no network — deskfile's
// `new` flow (deskfile.go) decides refuse / re-route / notice-lane / needs-decision from the
// typed result this file returns; nothing here writes an issue or reads one.
//
// The grammar is spelled out ONCE, in tools/desk/README.md ("The `### Fork test` block —
// the decision filing gate"); the desk skills point at that section rather than restating
// it, and so does this comment.

// forkTestHeadingRe matches the block's heading, any level (`#` through `######`), the same
// tolerant shape isEvidenceHeading uses above for "### Evidence" — a filer who writes
// `## Fork test` should not lose the whole gate to a `#`-count mismatch.
var forkTestHeadingRe = regexp.MustCompile(`(?i)^\s*#{1,6}\s*Fork test\s*$`)

// anyHeadingRe ends a section: any line opening with 1-6 `#` characters.
var anyHeadingRe = regexp.MustCompile(`^\s*#{1,6}(\s|$)`)

// forkOptionLineRe matches one `option:` line:
//
//	option: <letter> — <what it is> | works-because: <why> | consequence: <what follows>
//
// The separator between the letter and "what it is" is accepted as an em dash, en dash, or a
// plain hyphen — authors reach for whichever their editor produces. The pipe-delimited
// fields after it are pulled out separately (see parseForkTest) so a missing
// works-because/consequence segment is DETECTABLE rather than silently absorbed into "what
// it is".
var forkOptionLineRe = regexp.MustCompile(`(?i)^[ \t>*_-]*option:[ \t]*([A-Za-z0-9]+)[ \t]*[—–-][ \t]*(.*)$`)

// forkDefaultAnyRe recognises a `default:` line whatever its value, so a malformed value is
// reported as malformed, never as "no `default:` line found".
var forkDefaultAnyRe = regexp.MustCompile(`(?i)^[ \t>*_-]*default:[ \t]*(.*)$`)

// forkDefaultValueRe reads the option letter off a recognised default line's value: a bare
// letter, optionally followed by " — <text>" (the same letter-dash-text shape `option:` and
// `caught-by:` lines take), e.g. `default: A` or `default: A — keep the current default`.
var forkDefaultValueRe = regexp.MustCompile(`^([A-Za-z0-9]+)[ \t]*(?:[—–-][ \t]*.*)?$`)

// forkCaughtByAnyRe recognises a `caught-by:` line regardless of whether its value is one of
// the closed set, so an invalid value is reported as "invalid", never silently as "absent".
var forkCaughtByAnyRe = regexp.MustCompile(`(?i)^[ \t>*_-]*caught-by:[ \t]*(.*)$`)

// forkCaughtByValueRe splits a recognised caught-by line's value into its closed-set kind and
// the optional " — <detail>" that follows it.
var forkCaughtByValueRe = regexp.MustCompile(`(?i)^(draft-pr|flip|issue-close|nothing)[ \t]*(?:[—–-][ \t]*(.*))?$`)

// forkRuledCheckLineRe matches `ruled-check: <the search that was run> → <what it returned>`.
// The arrow is accepted as "→" or "->", for a filer whose editor cannot type the former.
var forkRuledCheckLineRe = regexp.MustCompile(`(?i)^[ \t>*_-]*ruled-check:[ \t]*(.+?)[ \t]*(?:\x{2192}|->)[ \t]*(.*)$`)

// forkSubjectLineRe matches an OPTIONAL `subject: <one line naming what is actually being
// decided>` line, capturing its prefix (group 1, everything before `subject:`) separately
// from its value (group 2) so the caller can tell a `>`-quoted line from a plain one — see
// below. It is the notice lane's ONLY source for its positive, content-bearing R-3 reversible
// signal (deskkit.NoticeLaneVerdict) — never the issue title, never body prose, never the
// `ruled-check:` line (security review sec-1688-S1, round 4). A title can and does carry more
// than one clause ("Tool default: let the desk commit to main when CI is green? Fix the
// help-text wording too." names a main-push governance question AND, in passing, a wording
// fix), so a scan over title+body admits on whichever clause happens to carry a reversible
// needle, not on what the filing is actually about. `subject:` asks the filer to name that in
// one line instead. Its ABSENCE is not an error (the fork-test gate's structural requirements
// are unchanged — see forkTestResult.Structural): a filing with no `subject:` line simply
// never admits the notice lane, which is the fail-closed default every other unrecognised
// shape already gets.
//
// A well-formed section is meant to declare the subject exactly once. Round 5 (security
// review sec-1688-S1) found two ways a second line crept in and still admitted: prose after
// the block with no heading in between (the section runs to the next heading or EOF) can
// itself carry what reads as a `subject:` line — including a `>`-quoted one, since the prefix
// class below matches a blockquote marker the same as a bullet — and when two or more
// `subject:` lines appeared, parseForkTest kept only the LAST one, so an honest first subject
// could be silently overridden by an incidental second line or a leftover template
// placeholder. parseForkTest below now reads the declared subject only when the section
// carries EXACTLY ONE `subject:` line and it is not `>`-quoted (group 1 contains no `>`);
// two or more — quoted, unquoted, or a mix — leaves r.Subject empty, the same fail-closed
// default as no `subject:` line at all. Bullet/emphasis prefixes (`*`, `_`, `-`, matching the
// grammar's other lines) are still accepted on the one subject line that counts.
var forkSubjectLineRe = regexp.MustCompile(`(?i)^([ \t>*_-]*)subject:[ \t]*(.*)$`)

// The closed set caught-by's first field must be one of.
const (
	caughtByDraftPR    = "draft-pr"
	caughtByFlip       = "flip"
	caughtByIssueClose = "issue-close"
	caughtByNothing    = "nothing"
)

// The three closed values --no-fork takes.
const (
	noForkBriefContradicts  = "brief-contradicts-artifact"
	noForkWrongRepo         = "wrong-repo"
	noForkToolFalsePositive = "tool-false-positive"
)

// deskDecidedLabel is the notice-lane label `deskfile new` applies itself (never a caller
// `--label` — cmdNew refuses one): it goes through deskkit's LabelChange ensure-exists path,
// the same one deskflip/deskpost's mechanical labels and deskpr's --decided use, so it is
// created on first use rather than requiring a human/admin label-create step first. It is
// deskkit's one DeskDecidedLabel, created with deskkit's one colour and description.
const deskDecidedLabel = deskkit.DeskDecidedLabel

// deskDecidedColor is the colour desk-decided is created with on first use.
const deskDecidedColor = deskkit.DeskDecidedLabelColor

// deskDecidedMarker is the SAME machine-readable marker deskdigest's r3MarkerRe reads
// (cmd/deskdigest/collect.go) and deskpr writes into a PR body — one marker, three writers:
// a desk taking an R-3 decision by hand posts it as a comment, deskpr --decided writes it
// into a PR body, and this notice lane writes it straight into the filed issue's own body,
// because the filing IS the decision (no separate comment to wait for).
const deskDecidedMarker = deskkit.DeskDecidedMarker

// repoShapeRe matches an `owner/repo`-shaped token, the minimum content check for
// --no-fork wrong-repo's "body must name the repo the work belongs in".
var repoShapeRe = regexp.MustCompile(`\b[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*\b`)

// briefIDShapeRe matches a `<stream>/<NN>`-shaped brief id (e.g. "example-stream/07") or the `assay:at:<stream>:<NN>` long form, the minimum content
// check for --no-fork brief-contradicts-artifact's "body must name the brief id".
var briefIDShapeRe = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:-[a-z0-9]+)*/[0-9]+\b|\bassay:at:[a-z0-9-]+:[0-9]+\b`)

// artifactPathShapeRe matches a backtick-quoted path with a file extension, the minimum
// content check for --no-fork brief-contradicts-artifact's "the artifact it contradicts".
var artifactPathShapeRe = regexp.MustCompile("`[^`\\n]*/[^`\\n]*\\.[A-Za-z0-9]+`")

// bodyHasFence reports whether body carries at least one fenced code block line (a line
// opening with three backticks), anywhere — the minimum content check for --no-fork
// tool-false-positive's "body must carry the tool's refusal text in a fence". Deliberately as
// undemanding as bodyHasEvidenceBlock's own fence test above: this tool cannot verify the
// fence quotes the ACTUAL refusal text without re-running the refused command itself, so it
// checks the shape a genuine quote takes and leaves the content judgement to review.
func bodyHasFence(body string) bool {
	for _, ln := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			return true
		}
	}
	return false
}

// renderDeskDecidedBlock composes the "## Desk-decided" block the notice lane appends to a
// filed issue's body (facts): the fixed heading, the shared r3 marker, then decision /
// alternative / cost. def is the resolved default option (never nil when this is called —
// the caller has already established Workable()).
func renderDeskDecidedBlock(def forkOption, others []forkOption, caughtByKind, caughtByDetail string) string {
	var alts []string
	for _, o := range others {
		if strings.EqualFold(o.Letter, def.Letter) {
			continue
		}
		alts = append(alts, o.Letter+" — "+o.What)
	}
	altLine := "(none)"
	if len(alts) > 0 {
		altLine = strings.Join(alts, "; ")
	}
	cost := caughtByKind
	if strings.TrimSpace(caughtByDetail) != "" {
		cost = caughtByKind + " — " + caughtByDetail
	}
	var b strings.Builder
	b.WriteString("\n\n## Desk-decided\n\n")
	b.WriteString(deskDecidedMarker + "\n")
	b.WriteString("decision: " + def.What + "\n")
	b.WriteString("alternative: " + altLine + "\n")
	b.WriteString("cost: " + cost + "\n")
	return b.String()
}

// forkOption is one parsed `option:` line.
type forkOption struct {
	Letter       string
	What         string
	WorksBecause string
	Consequence  string
}

// Counted reports whether this option counts toward the two-workable-options test. An
// option with an empty works-because or consequence is not counted (facts: "An option the
// filer believes cannot work is not written as an option; it goes in the prose as a
// rejected alternative").
func (o forkOption) Counted() bool {
	return strings.TrimSpace(o.WorksBecause) != "" && strings.TrimSpace(o.Consequence) != ""
}

// oneWayHay is the body EXCEPT any `ruled-check:` line, and ruledCheckLines is those lines.
// The ruled-check line is the grammar's record of the search for an existing ruling, so its
// natural wording ("no prior ruling found") trips the R-3 `ruling` needle on every filing
// whatever the item is about — but it is ALSO where a filer names the subject of that search,
// so it is never dropped from the one-way scan: filingOneWay reads it against every one-way
// list except the `ruling` needle (security review sec-1688-S1, round 2). The reversible
// half of the notice-lane verdict reads oneWayHay only, so the line can never ADMIT a filing.
func oneWayHay(body string) string {
	hay, _ := splitRuledCheck(body)
	return hay
}

func splitRuledCheck(body string) (rest, ruledCheck string) {
	lines := strings.Split(body, "\n")
	out := lines[:0:0]
	var rc []string
	for _, ln := range lines {
		if forkRuledCheckLineRe.MatchString(ln) {
			rc = append(rc, ln)
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n"), strings.Join(rc, "\n")
}

// rulingNeedle is the one HumanOnlySignals needle the ruled-check line is exempt from.
const rulingNeedle = "ruling"

// filingOneWay is the one-way check every off-queue route in `deskfile new` consults: the
// title, the body and the caller labels through deskkit.OneWay, with the ruled-check line
// read separately through deskkit.OneWayExempting(…, "ruling").
func filingOneWay(title, body string, labels []string) (deskkit.OneWayHit, bool) {
	rest, rc := splitRuledCheck(body)
	if hit, ok := deskkit.OneWay(title, rest, labels); ok {
		return hit, true
	}
	if rc != "" {
		if hit, ok := deskkit.OneWayExempting(rc, rulingNeedle); ok {
			return deskkit.OneWayHit{Match: hit.Match, Category: hit.Category + ", on the ruled-check line"}, true
		}
	}
	return deskkit.OneWayHit{}, false
}

// forkTestResult is everything parseForkTest read from a body. It is a REPORT, not a
// verdict — deskfile's cmdNew decides refuse / re-route / notice-lane / needs-decision from
// these fields; this type draws no conclusion of its own.
type forkTestResult struct {
	Found   bool         // a "### Fork test" heading (any level) was found at all
	Options []forkOption // every option: line that parsed, in document order

	HasDefault bool
	Default    string // the letter, as written
	// defaultRecognised is true when a `default:` line was found at all (valid or not);
	// DefaultRaw is its raw value, for the malformed-value message.
	defaultRecognised bool
	DefaultRaw        string

	HasCaughtBy        bool   // a caught-by: line with a value in the closed set
	CaughtByRaw        string // the raw value, whether or not it was in the closed set (for messages)
	CaughtByKind       string // one of the caughtBy* constants, set only when HasCaughtBy
	CaughtByDetail     string
	caughtByRecognised bool // a caught-by: line was found at all (valid or not)

	HasRuledCheck    bool
	RuledCheckSearch string
	RuledCheckResult string

	// HasSubject/Subject: the OPTIONAL `subject:` line (forkSubjectLineRe), set only when the
	// section carries EXACTLY ONE such line and it is not `>`-quoted (round 5 — see
	// forkSubjectLineRe). Never required — Structural() does not check it — but it is the
	// ONLY text deskkit.NoticeLaneVerdict reads for its positive reversible-signal test. Two
	// or more `subject:` lines, or a lone quoted one, leave HasSubject false and Subject
	// empty: the same fail-closed default as no `subject:` line at all, never a refusal (an
	// ambiguous subject is not a structural error — see Structural).
	HasSubject bool
	Subject    string

	// Errors names every structural problem found, in a stable order, for the refusal
	// message. Empty iff the block is structurally complete (found, at least one option
	// line, default/caught-by/ruled-check all present and well-formed, default names a
	// COUNTED option) — REGARDLESS of the option COUNT, which is the separate Workable()
	// test. A block can be perfectly well-formed and still name only one workable option.
	Errors []string
}

// CountedOptions returns the subset of Options that count toward the two-workable-options
// test.
func (r forkTestResult) CountedOptions() []forkOption {
	var out []forkOption
	for _, o := range r.Options {
		if o.Counted() {
			out = append(out, o)
		}
	}
	return out
}

// DefaultOption resolves r.Default to the parsed option it names (letter match,
// case-insensitive), or nil if it names nothing parsed.
func (r forkTestResult) DefaultOption() *forkOption {
	for i := range r.Options {
		if strings.EqualFold(r.Options[i].Letter, r.Default) {
			return &r.Options[i]
		}
	}
	return nil
}

// Structural reports whether the block parsed cleanly: found, well-formed, every required
// line present, and the default names a counted option. This is the gate for the "no block /
// unparseable block / missing line / default naming no counted option" refusal (facts) —
// independent of whether there are enough counted options to be workable (see Workable).
func (r forkTestResult) Structural() bool {
	return r.Found && len(r.Errors) == 0
}

// Workable reports whether the block counts two or more workable options. Only meaningful
// once Structural() is true; a non-structural block has already been refused for a
// different reason.
func (r forkTestResult) Workable() bool {
	return len(r.CountedOptions()) >= 2
}

// parseForkTest extracts the "### Fork test" section from body and validates its shape per
// the grammar in tools/desk/README.md. Pure: no I/O, no clock, no network.
func parseForkTest(body string) forkTestResult {
	var r forkTestResult
	section, found := extractForkSection(body)
	if !found {
		r.Errors = append(r.Errors,
			"no `### Fork test` section found (any heading level accepted) — see tools/desk/README.md")
		return r
	}
	r.Found = true

	seenLetter := map[string]bool{}
	var dupLetters []string
	subjectLines := 0     // every `subject:` line seen, quoted or not
	unquotedSubjects := 0 // the subset that is not `>`-quoted
	var lastUnquotedSubject string
	for _, ln := range strings.Split(section, "\n") {
		if m := forkOptionLineRe.FindStringSubmatch(ln); m != nil {
			if k := strings.ToLower(strings.TrimSpace(m[1])); seenLetter[k] {
				dupLetters = append(dupLetters, strings.TrimSpace(m[1]))
			} else {
				seenLetter[k] = true
			}
			opt := forkOption{Letter: strings.TrimSpace(m[1])}
			fields := strings.Split(m[2], "|")
			opt.What = strings.TrimSpace(fields[0])
			for _, f := range fields[1:] {
				f = strings.TrimSpace(f)
				low := strings.ToLower(f)
				switch {
				case strings.HasPrefix(low, "works-because:"):
					opt.WorksBecause = strings.TrimSpace(f[len("works-because:"):])
				case strings.HasPrefix(low, "consequence:"):
					opt.Consequence = strings.TrimSpace(f[len("consequence:"):])
				}
			}
			r.Options = append(r.Options, opt)
			continue
		}
		if m := forkDefaultAnyRe.FindStringSubmatch(ln); m != nil {
			r.defaultRecognised = true
			r.DefaultRaw = strings.TrimSpace(m[1])
			if vm := forkDefaultValueRe.FindStringSubmatch(r.DefaultRaw); vm != nil {
				r.HasDefault = true
				r.Default = vm[1]
			}
			continue
		}
		if m := forkCaughtByAnyRe.FindStringSubmatch(ln); m != nil {
			r.caughtByRecognised = true
			raw := strings.TrimSpace(m[1])
			r.CaughtByRaw = raw
			if vm := forkCaughtByValueRe.FindStringSubmatch(raw); vm != nil {
				r.HasCaughtBy = true
				r.CaughtByKind = strings.ToLower(vm[1])
				r.CaughtByDetail = strings.TrimSpace(vm[2])
			}
			continue
		}
		if m := forkRuledCheckLineRe.FindStringSubmatch(ln); m != nil {
			r.HasRuledCheck = true
			r.RuledCheckSearch = strings.TrimSpace(m[1])
			r.RuledCheckResult = strings.TrimSpace(m[2])
			continue
		}
		if m := forkSubjectLineRe.FindStringSubmatch(ln); m != nil {
			subjectLines++
			if !strings.Contains(m[1], ">") {
				unquotedSubjects++
				lastUnquotedSubject = strings.TrimSpace(m[2])
			}
			continue
		}
	}
	if subjectLines == 1 && unquotedSubjects == 1 {
		r.HasSubject = true
		r.Subject = lastUnquotedSubject
	}

	if len(r.Options) == 0 {
		r.Errors = append(r.Errors, "no `option:` lines found (need at least two counted options)")
	}
	for _, d := range dupLetters {
		r.Errors = append(r.Errors, fmt.Sprintf(
			"duplicate `option:` letter %q — each option needs its own letter; one option written twice is still one option", d))
	}
	switch {
	case !r.defaultRecognised:
		r.Errors = append(r.Errors, "no `default:` line found")
	case !r.HasDefault:
		r.Errors = append(r.Errors, fmt.Sprintf(
			"`default:` value %q must open with a bare option letter (`default: A` or `default: A — <text>`)", r.DefaultRaw))
	}
	switch {
	case !r.caughtByRecognised:
		r.Errors = append(r.Errors, "no `caught-by:` line found")
	case !r.HasCaughtBy:
		r.Errors = append(r.Errors, fmt.Sprintf(
			"`caught-by:` value %q is not one of draft-pr|flip|issue-close|nothing", r.CaughtByRaw))
	}
	if !r.HasRuledCheck {
		r.Errors = append(r.Errors, "no `ruled-check:` line found (the search for an existing ruling)")
	}
	if r.HasDefault {
		def := r.DefaultOption()
		switch {
		case def == nil:
			r.Errors = append(r.Errors, fmt.Sprintf("`default: %s` names no option in the block", r.Default))
		case !def.Counted():
			r.Errors = append(r.Errors, fmt.Sprintf(
				"`default: %s` names an option that is not counted (empty works-because or consequence)", r.Default))
		}
	}
	return r
}

// forkFenceLineRe matches a fenced-code delimiter line (three or more backticks or tildes),
// allowed up to 3 leading spaces (a fence may be indented up to 3 columns and still open,
// per CommonMark). It is one of extractForkSection's hard section-boundary markers.
var forkFenceLineRe = regexp.MustCompile("^[ \t]{0,3}(```+|~~~+)")

// forkGrammarKeyLine reports whether ln is one of the fork-test grammar's five recognised key
// lines — option:/default:/caught-by:/ruled-check:/subject: — for the sole purpose of bounding
// the section in extractForkSection. It never parses ln's value (parseForkTest does that,
// over the bounded section only).
//
// A line indented 4 or more columns, or opening with a tab, is Markdown's own indented-code-
// block signal and is deliberately NEVER recognised here even when its text would otherwise
// match one of the five key-line patterns: security review sec-1688-S1 (round 6) found an
// indented subject-shaped line outside the real block was silently read as the declared
// subject, because the grammar's own prefix classes (`[ \t>*_-]*`) tolerate leading
// whitespace for bullet/quote formatting and cannot themselves tell "a bulleted subject line"
// from "an indented code block that happens to contain the word subject:".
func forkGrammarKeyLine(ln string) bool {
	if strings.HasPrefix(ln, "\t") || strings.HasPrefix(ln, "    ") {
		return false
	}
	return forkOptionLineRe.MatchString(ln) ||
		forkDefaultAnyRe.MatchString(ln) ||
		forkCaughtByAnyRe.MatchString(ln) ||
		forkRuledCheckLineRe.MatchString(ln) ||
		forkSubjectLineRe.MatchString(ln)
}

// extractForkSection returns the text between a "### Fork test" heading (any level) and the
// BOUNDED end of that section: the first heading, fence-delimiter line, or blank line
// encountered after the block's LAST recognised grammar key line (forkGrammarKeyLine) —
// never EOF, and never the section's own natural end-of-string if that runs past the last key
// line without one of those three markers (the fallback below cuts right after it instead).
// found is false when no such heading exists at all.
//
// Blank lines *before* the first key line, or a blank line immediately followed (after
// skipping further blank lines) by another key line, do not end the section — the grammar's
// own well-formed shape has a blank line between the heading and its first key line, and
// authors are free to group option:/default:/caught-by:/ruled-check:/subject: lines with a
// blank line between groups (e.g. a `subject:` line, then a blank line, then the option:
// lines). Only a blank run that is NOT followed by another key line — i.e. the block has
// genuinely ended — closes the section.
//
// Before this bound, the section ran to the next ATX heading or EOF, so prose after the block
// with no heading in between was still read as part of it: a `subject:`-shaped line inside a
// fenced or indented code block, in ordinary prose, or past a Markdown setext heading (none of
// which anyHeadingRe recognises as ending anything) could all be picked up as the declared
// subject (security review sec-1688-S1, round 6). Bounding the section this way, rather than
// tightening forkSubjectLineRe itself, keeps the fix in ONE place: any of the five grammar key
// lines is bound the same way, not just `subject:`.
func extractForkSection(body string) (section string, found bool) {
	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if forkTestHeadingRe.MatchString(ln) {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false
	}

	end := start + 1
	for i := start + 1; i < len(lines); i++ {
		ln := lines[i]
		if anyHeadingRe.MatchString(ln) || forkFenceLineRe.MatchString(ln) {
			return strings.Join(lines[start+1:i], "\n"), true
		}
		if strings.TrimSpace(ln) == "" {
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j >= len(lines) || !forkGrammarKeyLine(lines[j]) {
				return strings.Join(lines[start+1:i], "\n"), true
			}
			// A key line follows this blank run: the blank is part of the section (an
			// author's own grouping), not its end. Fall through and keep scanning.
		}
		end = i + 1
	}
	// Reached EOF without a heading/fence/blank boundary after the last key line: cut right
	// after it (end already reflects that — every line up to here either was a key line or a
	// blank run followed by one), never at physical EOF.
	return strings.Join(lines[start+1:end], "\n"), true
}

// forkTestErrorMessage renders parseForkTest's Errors as the refused-filing message body,
// naming each missing/invalid line — the facts' "the message names each missing line".
func forkTestErrorMessage(r forkTestResult) string {
	var b strings.Builder
	b.WriteString("refused: a `needs-decision` filing must carry a well-formed `### Fork test` " +
		"block (grammar: tools/desk/README.md). Problems found:\n")
	for _, e := range r.Errors {
		b.WriteString("  - " + e + "\n")
	}
	b.WriteString("The `--force-new --reason` bypass is the only escape hatch, as for every " +
		"refusal in this tool.")
	return b.String()
}

// forkTestRerouteMessage renders the "fewer than two counted options" refusal, naming the
// three --no-fork re-routes (facts) so the filer has a way forward without forcing a
// needs-decision filing over a question that never had a fork.
func forkTestRerouteMessage(r forkTestResult) string {
	n := len(r.CountedOptions())
	return fmt.Sprintf(
		"refused: the `### Fork test` block counts %d workable option(s) — fewer than the two a "+
			"decision needs. An item with one workable option is not a decision. Re-run with exactly "+
			"one of:\n"+
			"  --no-fork brief-contradicts-artifact   (title prefix `amend brief:`, addressed --to desk; "+
			"body must name the brief id and the artifact it contradicts)\n"+
			"  --no-fork wrong-repo                   (title prefix `re-dispatch:`, addressed --to "+
			"worker; body must name the repo the work belongs in)\n"+
			"  --no-fork tool-false-positive           (label `bug`; body must carry the tool's refusal "+
			"text in a fence)\n"+
			"Override with --force-new --reason only if a second workable option genuinely exists and "+
			"the block under-counted it.", n)
}

// forkTestOneWayMessage renders the "fewer than two counted options" refusal for a ONE-WAY
// item. Every --no-fork re-route files off the driver's queue, so none is offered: a one-way
// item with one workable option is still the driver's call (typically an act only they can
// take), and the only way forward is the audited --force-new --reason, which files it under
// needs-decision.
func forkTestOneWayMessage(r forkTestResult, hit deskkit.OneWayHit) string {
	return fmt.Sprintf(
		"refused: the `### Fork test` block counts %d workable option(s) — fewer than the two a "+
			"decision needs — but this item is one-way (%s), so it is NOT re-routed off the driver's "+
			"queue: no --no-fork re-route applies. Whether the one workable option is an act only the "+
			"driver can take or anything else, file it as that ask by re-running with --force-new "+
			"--reason \"<why this has one option>\": that files it under needs-decision (on the driver's "+
			"queue), audited, without the fork-test block.", len(r.CountedOptions()), hit.String())
}
