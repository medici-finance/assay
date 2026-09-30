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
//
// ROUND 7 — a strict, fail-closed grammar replaces five rounds of shape-by-shape patches.
// Rounds 1-6 each closed one more way trailing or embedded content could be read as the
// block's declared subject (a fence, an indent, a setext heading, a `>`-quote, a second
// `subject:` line, a blank-then-key-line run-on...) by teaching extractForkSection one more
// boundary marker or teaching parseForkTest one more exclusion. Security review sec-1688-S1
// (round 6 residual) and the withheld variants in review-notes#169 kept finding the next shape
// the marker list had not enumerated (a plain trailing line after one blank line; an HTML
// comment hidden inside the block itself). The fix is not a seventh marker: it is dropping the
// whole "scan forward for a boundary marker" design in favour of three rules that need no
// enumeration —
//
//  1. Strip every fenced code block and every HTML comment out of the WHOLE body before any
//     of the rest of this runs (stripFencedBlocks, stripHTMLComments). Nothing inside either
//     can ever be read as a heading or a key line again, so a quoted example, a quoted heading,
//     or a subject hidden in a comment between two real key lines all vanish before parsing
//     starts, rather than needing their own boundary rule.
//  2. The heading must be followed DIRECTLY by the block — blank lines are fine (the grammar's
//     own well-formed shape has one), but any other text before the first key line is a
//     malformed block, not a silently-empty one (cor-1688-C11).
//  3. The block is the CONTIGUOUS run of key lines (forkKeyLineRe) starting there. It ends at
//     the first line that does not match — including a blank line — and nothing past that line
//     is EVER read as part of the block, whatever the rest of the body contains. A trailing
//     `subject:`-shaped line separated from the block by so much as one blank line is not a
//     boundary marker to special-case; it is simply not contiguous, so it was never a candidate
//     in the first place.
//
// `subject:` is read only from within that bounded run, and only when the run carries EXACTLY
// ONE such line (forkTestResult.HasSubject) — zero or more than one is the same fail-closed
// "no declared subject" outcome. A `>`-quoted, bulleted, or indented subject-shaped line can
// never enter the run at all: forkKeyLineRe requires column zero and a lowercase key name, so
// the round-5/6 quote and indent exclusions are now a CONSEQUENCE of the grammar rather than a
// bolt-on check parseForkTest had to run.

// forkTestHeadingRe matches the block's heading, any level (`#` through `######`), the same
// tolerant shape isEvidenceHeading uses above for "### Evidence" — a filer who writes
// `## Fork test` should not lose the whole gate to a `#`-count mismatch.
var forkTestHeadingRe = regexp.MustCompile(`(?i)^\s*#{1,6}\s*Fork test\s*$`)

// forkKeyLineRe is the grammar's ONE definition of a "key line": a lowercase key name (ASCII
// letters and hyphens only) at column zero, followed by ": " and at least one non-whitespace
// character of content. No leading whitespace, no bullet/blockquote/emphasis decoration, no
// indentation, no uppercase key name — a filer who wants a line recognised writes it exactly
// this way. extractForkSection uses this SAME test to both locate the block (the first key
// line after the heading) and to bound it (the contiguous run of lines that match); it says
// nothing about which key NAMES the grammar actually reads — parseForkTest's five field
// regexes below do that, over the bounded run only. An unrecognised key-shaped line (a typo, a
// future field) still counts toward CONTIGUITY even though parseForkTest reads nothing from
// it, which is the simpler, safer default: a stray line does not need to look like NOTHING the
// grammar recognises in order to keep the block open.
var forkKeyLineRe = regexp.MustCompile(`^[a-z][a-z-]*: \S`)

// forkOptionLineRe matches one `option:` line:
//
//	option: <letter> — <what it is> | works-because: <why> | consequence: <what follows>
//
// The separator between the letter and "what it is" is accepted as an em dash, en dash, or a
// plain hyphen — authors reach for whichever their editor produces. The pipe-delimited
// fields after it are pulled out separately (see parseForkTest) so a missing
// works-because/consequence segment is DETECTABLE rather than silently absorbed into "what
// it is". The key itself (`option: `) is exactly what forkKeyLineRe already required of any
// line in the bounded section — see the file comment.
var forkOptionLineRe = regexp.MustCompile(`^option: ([A-Za-z0-9]+)[ \t]*[—–-][ \t]*(.*)$`)

// forkDefaultAnyRe recognises a `default:` line whatever its value, so a malformed value is
// reported as malformed, never as "no `default:` line found".
var forkDefaultAnyRe = regexp.MustCompile(`^default: (.*)$`)

// forkDefaultValueRe reads the option letter off a recognised default line's value: a bare
// letter, optionally followed by " — <text>" (the same letter-dash-text shape `option:` and
// `caught-by:` lines take), e.g. `default: A` or `default: A — keep the current default`.
var forkDefaultValueRe = regexp.MustCompile(`^([A-Za-z0-9]+)[ \t]*(?:[—–-][ \t]*.*)?$`)

// forkCaughtByAnyRe recognises a `caught-by:` line regardless of whether its value is one of
// the closed set, so an invalid value is reported as "invalid", never silently as "absent".
var forkCaughtByAnyRe = regexp.MustCompile(`^caught-by: (.*)$`)

// forkCaughtByValueRe splits a recognised caught-by line's value into its closed-set kind and
// the optional " — <detail>" that follows it. The enum value itself stays case-insensitive
// (`Draft-PR` names the same kind as `draft-pr`) — only the `caught-by:` KEY is case-bound now,
// not its value.
var forkCaughtByValueRe = regexp.MustCompile(`(?i)^(draft-pr|flip|issue-close|nothing)[ \t]*(?:[—–-][ \t]*(.*))?$`)

// forkRuledCheckLineRe matches `ruled-check: <the search that was run> → <what it returned>`.
// The arrow is accepted as "→" or "->", for a filer whose editor cannot type the former.
var forkRuledCheckLineRe = regexp.MustCompile(`^ruled-check: (.+?)[ \t]*(?:\x{2192}|->)[ \t]*(.*)$`)

// forkSubjectLineRe matches an OPTIONAL `subject: <one line naming what is actually being
// decided>` line. It is the notice lane's ONLY source for its positive, content-bearing R-3
// reversible signal (deskkit.NoticeLaneVerdict) — never the issue title, never body prose,
// never the `ruled-check:` line (security review sec-1688-S1, round 4). A title can and does
// carry more than one clause ("Tool default: let the desk commit to main when CI is green? Fix
// the help-text wording too." names a main-push governance question AND, in passing, a wording
// fix), so a scan over title+body admits on whichever clause happens to carry a reversible
// needle, not on what the filing is actually about. `subject:` asks the filer to name that in
// one line instead. Its ABSENCE is not an error (the fork-test gate's structural requirements
// are unchanged — see forkTestResult.Structural): a filing with no `subject:` line simply
// never admits the notice lane, which is the fail-closed default every other unrecognised
// shape already gets.
//
// A well-formed section is meant to declare the subject exactly once: parseForkTest reads a
// declared subject only when the BOUNDED, CONTIGUOUS run (extractForkSection) carries EXACTLY
// ONE line matching this pattern — zero or more than one leaves r.Subject empty, the same
// fail-closed default as no `subject:` line at all (round 7 folds rounds 5-6's separate
// `>`-quote exclusion and blank-run/fence/heading boundary tracking into these two rules:
// forkKeyLineRe's column-zero, no-decoration shape already keeps a quoted or indented line out
// of the run entirely, and the run's own contiguity already keeps anything past a blank line,
// a fence, or a heading out of it — see the file comment).
var forkSubjectLineRe = regexp.MustCompile(`^subject: (.*)$`)

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
	// bounded section carries EXACTLY ONE such line (see forkSubjectLineRe). Never required —
	// Structural() does not check it — but it is the ONLY text deskkit.NoticeLaneVerdict reads
	// for its positive reversible-signal test. Zero or more than one `subject:` line leaves
	// HasSubject false and Subject empty: the same fail-closed default as no `subject:` line at
	// all, never a refusal (an ambiguous subject is not a structural error — see Structural).
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
	section, found, malformed := extractForkSection(body)
	if !found {
		r.Errors = append(r.Errors,
			"no `### Fork test` section found (any heading level accepted) — see tools/desk/README.md")
		return r
	}
	r.Found = true
	if malformed != "" {
		r.Errors = append(r.Errors, malformed)
		return r
	}

	seenLetter := map[string]bool{}
	var dupLetters []string
	var subjects []string // every `subject:` line in the bounded run
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
			subjects = append(subjects, strings.TrimSpace(m[1]))
			continue
		}
	}
	if len(subjects) == 1 {
		r.HasSubject = true
		r.Subject = subjects[0]
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
// allowed up to 3 leading spaces (a fence may be indented up to 3 columns and still open, per
// CommonMark). Used by stripFencedBlocks (round 7) to remove every fenced block, delimiters and
// content both, from the whole body before any of the rest of this file runs.
var forkFenceLineRe = regexp.MustCompile("^[ \t]{0,3}(```+|~~~+)")

// stripFencedBlocks removes every fenced code block in body — the opening delimiter, every
// line inside it, and the closing delimiter — entirely, by toggling in/out of "fence" state on
// each line matching forkFenceLineRe and dropping every line seen while in that state
// (delimiters included). Round 7: this is what closes cor-1688-C12/sec-1688-S5's fence-arm gap
// and the sibling case the security review noted (a `### Fork test` heading quoted inside an
// EARLIER fenced example is now gone before the heading search in extractForkSection ever
// runs) — a fence's contents are unreadable to the parser at all, rather than the parser
// trying to notice where a fence starts and stops while also scanning for key lines.
//
// An UNTERMINATED fence (no closing delimiter before EOF) drops everything after it to the end
// of the body. That is the fail-closed reading of a malformed fence: the alternative, treating
// an unterminated fence as if it were never opened, would let whatever comes after it — options,
// defaults, a subject — parse as if the stray ``` had never been typed, which is the wrong
// direction to fail in a gate whose whole job is refusing to guess.
func stripFencedBlocks(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for _, ln := range lines {
		if forkFenceLineRe.MatchString(ln) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// htmlCommentRe matches one HTML comment, `<!--` through the nearest following `-->`, across
// any number of lines. RE2 (Go's regexp package) has no catastrophic-backtracking failure mode
// regardless of the non-greedy `.*?`, so this is linear even on an adversarial body.
var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// stripHTMLComments removes every HTML comment in body entirely, delimiters included. Round 7:
// this is what closes the withheld review-notes#169 variant of a `subject:` line hidden inside
// an HTML comment BETWEEN two of the block's own real key lines — invisible in the rendered
// issue, but previously still read as a key line by parseForkTest, which never cared what
// Markdown construct a key-shaped line sat inside. An UNTERMINATED `<!--` (no `-->` anywhere in
// the rest of the body) is left as plain text: it was never actually hidden from a renderer
// either, so there is nothing to fail closed about.
func stripHTMLComments(body string) string {
	return htmlCommentRe.ReplaceAllString(body, "")
}

// extractForkSection locates the "### Fork test" block per the round-7 grammar (see the file
// comment and tools/desk/README.md):
//
//  1. Fences and HTML comments are stripped from the WHOLE body first (stripFencedBlocks,
//     stripHTMLComments) — this is a local copy inside this function; it never touches the
//     body any other gate (evidence, marker-claim, one-way) reads.
//  2. The heading (any level) is found. found is false only when no heading exists at all.
//  3. The heading must be followed DIRECTLY by the block: any number of blank lines are
//     tolerated (the grammar's own well-formed shape has one), but the first non-blank line
//     after the heading must be a key line (forkKeyLineRe) — anything else (prose, an empty
//     heading with nothing following it) makes malformed non-empty and section empty
//     (cor-1688-C11: a heading was found, but no block could be located after it).
//  4. From that first key line, the section is the CONTIGUOUS run of lines matching
//     forkKeyLineRe. It ends at the first line that does not match, blank or not — nothing
//     past that line is ever part of the section, however the rest of the body reads.
func extractForkSection(body string) (section string, found bool, malformed string) {
	body = stripHTMLComments(stripFencedBlocks(body))
	lines := strings.Split(body, "\n")

	start := -1
	for i, ln := range lines {
		if forkTestHeadingRe.MatchString(ln) {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false, ""
	}

	i := start + 1
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !forkKeyLineRe.MatchString(lines[i]) {
		return "", true, "the `### Fork test` heading is not followed directly by the block: " +
			"blank lines are fine, but no other text may come between the heading and the " +
			"first `option:`/`default:`/`caught-by:`/`ruled-check:`/`subject:` line — see " +
			"tools/desk/README.md"
	}

	end := i
	for end < len(lines) && forkKeyLineRe.MatchString(lines[end]) {
		end++
	}
	return strings.Join(lines[i:end], "\n"), true, ""
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
