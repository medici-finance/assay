// Package report builds the Ask Assay PDF reports (desk-console-2/03, design
// doc §7.5): a weekly summary, a bottleneck report and a decision-latency
// report, each one a DETERMINISTIC function of the index at a stamp.
//
// THE §7.5 SPLIT, MADE STRUCTURAL
// -------------------------------
// "The generator is deterministic (index -> data -> render); the model adds
// narrative only." That split is worth nothing as a convention, because the
// failure it guards against — a total on a page with no query behind it —
// looks exactly like a correct total. So it is enforced:
//
//   - Every figure on every page comes from an [askassay.Answer]. There is no
//     exported numeric field anywhere in this package's document model.
//   - Narrative is prose, and [Doc.Build] REFUSES a document whose narrative
//     contains a numeral that is not one of the document's own rendered
//     figures (or an explicitly declared literal). The model may say "the
//     constraint moved to review"; it may not say "42".
//   - Every figure prints its source, probe, window and limit on the page,
//     not in a footnote and not in a tooltip. A cap that is not on the page
//     is a silent cap.
//
// COULD-NOT-CHECK ON PAPER
// ------------------------
// A figure with no value prints the literal token "could-not-check", its
// reason, and a hatched full-width track — never an empty track, because an
// empty track is a bar of length zero and a bar of length zero is a drawn
// claim that the value is 0. See pdf.go's "hatch" op.
//
// THE BOTTLENECK REPORT HAS A MEASURED PROBLEM, AND IT IS NOT WORKED AROUND
// ------------------------------------------------------------------------
// The registry's declared source for the constraint figure is
// `statusgen --root <root> --bottleneck`. That command WRITES:
// runBottleneck unconditionally calls writeBottleneckReport, which creates
// docs/reports/factory-floor/<date>.md. A read-only pane cannot run it, and a
// read-looking probe that writes an unmanifested path makes the publication
// stager refuse the whole tree.
//
// The inherited guard does not catch this — its statusgen read-mode
// allow-list lists --bottleneck among the read modes. [GuardReportProbe] is
// strictly narrower and refuses it, and TestInheritedGuardPermitsAWritingMode
// pins the difference so it cannot be lost silently. The consequence is
// carried honestly rather than routed around: [Bottleneck] takes the
// constraint figure from an ALREADY-WRITTEN report the caller read, and
// renders could-not-check with that exact reason when the caller has none.
package report

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/askassay"
	"github.com/medici-finance/assay/tools/desk/internal/askassay/chart"
)

// ErrRefused is returned for a document that cannot be built without breaking
// the numbers rule. It is a refusal, not a degraded render.
var ErrRefused = errors.New("report refused")

// Figure is one number on a page, bound to its answer.
type Figure struct {
	// Caption names what the number is, in words.
	Caption string
	// Answer is the ONE source of the number and its state.
	Answer askassay.Answer
}

// Section is one heading's worth of a report.
type Section struct {
	// Heading is the section title.
	Heading string
	// Narrative is the model's contribution: prose, and prose only. A
	// numeral here that is not one of this document's rendered figures is a
	// build refusal.
	Narrative []string
	// Figures are the section's numbers.
	Figures []Figure
}

// Doc is a whole report.
type Doc struct {
	// Title is the report name.
	Title string
	// AsOf is the stamp every figure was computed at, and the instant the
	// PDF's own /CreationDate is derived from. It is an input, never a clock
	// read: an artifact that changes because a second elapsed is not a
	// function of the index.
	AsOf askassay.Stamp
	// Sections are the body, in order.
	Sections []Section
	// AllowedLiterals are numeral-bearing tokens the narrative may contain
	// that are not figures — a date, a brief number. Each one is a declared
	// exception, so the exception list is reviewable.
	AllowedLiterals []string
}

// numeral finds digit runs in narrative prose.
var numeral = regexp.MustCompile(`\d[\d,.]*`)

// Build validates the document against the numbers rule and returns it ready
// to render. Every refusal names the offending token.
func (d Doc) Build() (Doc, error) {
	if strings.TrimSpace(d.Title) == "" {
		return Doc{}, fmt.Errorf("%w: the document declares no title", ErrRefused)
	}
	if d.AsOf.Zero() {
		return Doc{}, fmt.Errorf("%w: the document carries no as-of stamp, and every figure in it moves", ErrRefused)
	}
	if len(d.Sections) == 0 {
		return Doc{}, fmt.Errorf("%w: the document has no sections, and a blank report is indistinguishable from a report of zeroes", ErrRefused)
	}

	allowed := map[string]bool{}
	for _, l := range d.AllowedLiterals {
		for _, m := range numeral.FindAllString(l, -1) {
			allowed[m] = true
		}
	}
	for _, s := range d.Sections {
		for _, f := range s.Figures {
			if v, ok := f.Answer.Value(); ok {
				allowed[strconv.FormatInt(v, 10)] = true
			}
		}
	}
	for _, m := range numeral.FindAllString(d.AsOf.String(), -1) {
		allowed[m] = true
	}

	for si, s := range d.Sections {
		if strings.TrimSpace(s.Heading) == "" {
			return Doc{}, fmt.Errorf("%w: section %d has no heading", ErrRefused, si)
		}
		if len(s.Figures) == 0 && len(s.Narrative) == 0 {
			return Doc{}, fmt.Errorf("%w: section %q is empty", ErrRefused, s.Heading)
		}
		for fi, f := range s.Figures {
			if strings.TrimSpace(f.Caption) == "" {
				return Doc{}, fmt.Errorf("%w: figure %d of section %q has no caption", ErrRefused, fi, s.Heading)
			}
			// A figure that CARRIES a value must have declared its source in
			// full. A could-not-check figure need not — it has no number to
			// justify — but it must say why.
			if _, ok := f.Answer.Value(); ok {
				if err := f.Answer.Source().Validate(); err != nil {
					return Doc{}, fmt.Errorf("%w: figure %q renders a number but %v", ErrRefused, f.Caption, err)
				}
			} else if strings.TrimSpace(f.Answer.Reason()) == "" {
				return Doc{}, fmt.Errorf("%w: figure %q is %s with no reason, which is not a finding", ErrRefused, f.Caption, askassay.CouldNotCheck)
			}
		}
		for _, line := range s.Narrative {
			for _, m := range numeral.FindAllString(line, -1) {
				if !allowed[m] {
					return Doc{}, fmt.Errorf(
						"%w: the narrative of section %q states the figure %q, which no query on this document produced. The model narrates; the index computes",
						ErrRefused, s.Heading, m)
				}
			}
		}
	}
	return d, nil
}

// PDF renders the document. It is a pure function of the receiver: two calls
// return identical bytes, which TestPDFIsByteIdenticalAcrossRenders asserts.
func (d Doc) PDF() ([]byte, error) {
	built, err := d.Build()
	if err != nil {
		return nil, err
	}
	return buildPDF(built.Title, built.AsOf.At, built.ops())
}

func (d Doc) ops() []op {
	var ops []op
	ops = append(ops, op{kind: "h1", text: d.Title})
	ops = append(ops, op{kind: "small", text: "as-of " + d.AsOf.String()})
	ops = append(ops, op{kind: "small", text:
	// The disclosure line is on the page, not in a covering note: a binary
	// artifact can carry text no reviewer reads.
	"Every figure below states its source, probe, window and limit. A figure that could not be measured prints \"" +
		string(askassay.CouldNotCheck) + "\" and never a zero."})
	ops = append(ops, op{kind: "rule"})

	for _, s := range d.Sections {
		ops = append(ops, op{kind: "h2", text: s.Heading})
		for _, line := range s.Narrative {
			ops = append(ops, op{kind: "body", text: line})
		}
		if len(s.Narrative) > 0 && len(s.Figures) > 0 {
			ops = append(ops, op{kind: "gap"})
		}
		max := int64(0)
		for _, f := range s.Figures {
			if v, ok := f.Answer.Value(); ok && v > max {
				max = v
			}
		}
		for i, f := range s.Figures {
			ops = append(ops, op{kind: "body", text: f.Caption})
			if v, ok := f.Answer.Value(); ok {
				frac := 0.0
				if max > 0 {
					frac = float64(v) / float64(max)
				}
				label := strconv.FormatInt(v, 10)
				if f.Answer.State() == askassay.CheckedFailed {
					label = "! " + label + " (" + string(askassay.CheckedFailed) + ")"
				}
				shade := 0.0
				if len(s.Figures) > 1 {
					shade = float64(i) / float64(len(s.Figures)-1)
				}
				ops = append(ops, op{kind: "bar", text: label, frac: frac, shade: shade})
			} else {
				ops = append(ops, op{kind: "hatch", text: string(askassay.CouldNotCheck)})
				ops = append(ops, op{kind: "small", text: "    reason: " + f.Answer.Reason()})
			}
			src := f.Answer.Source()
			ops = append(ops, op{kind: "small", text: fmt.Sprintf(
				"    source: %s | probe: %s", undeclared(src.Cmd), undeclared(src.Probe))})
			ops = append(ops, op{kind: "small", text: fmt.Sprintf(
				"    window: %s | limit: %s", undeclared(src.Window), undeclared(src.Limit))})
			for _, c := range f.Answer.Caveats() {
				ops = append(ops, op{kind: "small", text: "    caveat: " + c})
			}
			ops = append(ops, op{kind: "gap"})
		}
		ops = append(ops, op{kind: "rule"})
	}
	return ops
}

func undeclared(s string) string {
	if strings.TrimSpace(s) == "" {
		return "UNDECLARED"
	}
	return s
}

// Chart returns a chart over one section's figures, so the same numbers that
// print on the page can be drawn on screen from the same answers. The chart
// package's could-not-check treatment applies unchanged.
func (s Section) Chart(name, unit string, asOf askassay.Stamp) chart.Chart {
	c := chart.Chart{Title: s.Heading, AsOf: asOf,
		Series: chart.Series{Name: name, Unit: unit}}
	for _, f := range s.Figures {
		c.Series.Points = append(c.Series.Points, chart.Point{Label: f.Caption, Answer: f.Answer})
	}
	return c
}

// ---------------------------------------------------------------------------
// Index and the three generators
// ---------------------------------------------------------------------------

// Index is the answer set a report is built from. It is a SLICE, not a map:
// ranging a map is one of the classic sources of a non-deterministic render.
type Index struct {
	AsOf    askassay.Stamp
	Answers []askassay.Answer
}

// Get returns the answer for a question ID. A miss is not an omission — the
// caller turns it into a rendered could-not-check via [Index.Require], so a
// question the index could not answer still appears on the page.
func (ix Index) Get(id string) (askassay.Answer, bool) {
	for _, a := range ix.Answers {
		if a.Question() == id {
			return a, true
		}
	}
	return askassay.Answer{}, false
}

// Require returns the indexed answer, or a could-not-check naming the miss.
// A report never drops a figure it could not compute: a missing row is
// indistinguishable from a row whose value is zero, and the whole point of
// the three-state rule is that those are different findings.
func (ix Index) Require(id string) askassay.Answer {
	if a, ok := ix.Get(id); ok {
		return a
	}
	q, declared := askassay.Lookup(id)
	if !declared {
		return askassay.Unanswerable(id, id, ix.AsOf)
	}
	return askassay.Unavailable(q,
		"the index carries no answer for this question — the probe did not run in this pass, which is not the same as a result of zero", ix.AsOf)
}

// Weekly builds the weekly summary report.
func Weekly(ix Index, narrative map[string][]string) (Doc, error) {
	d := Doc{Title: "Weekly summary", AsOf: ix.AsOf}
	d.Sections = []Section{
		{
			Heading:   "Inventory",
			Narrative: narrativeFor(narrative, "Inventory"),
			Figures: []Figure{
				{Caption: "open issues", Answer: ix.Require("open-issue-count")},
				{Caption: "PRs awaiting a human", Answer: ix.Require("awaiting-count")},
				{Caption: "open PR actions", Answer: ix.Require("pr-action-count")},
			},
		},
		{
			Heading:   "Board",
			Narrative: narrativeFor(narrative, "Board"),
			Figures: []Figure{
				{Caption: "brief rows by status", Answer: ix.Require("brief-status-count")},
				{Caption: "rows at verified or done", Answer: ix.Require("completion-count")},
				{Caption: "active unresolved alarms", Answer: ix.Require("alarm-count")},
			},
		},
	}
	return d.Build()
}

// Bottleneck builds the bottleneck report.
//
// Its constraint figure does NOT come from running the declared source. That
// source writes (see the package header), so this generator takes whatever
// the caller read from an already-written report and otherwise renders
// could-not-check with that reason on the page.
func Bottleneck(ix Index, narrative map[string][]string) (Doc, error) {
	stage := ix.Require("bottleneck-stage")
	if _, ok := stage.Value(); !ok {
		if q, declared := askassay.Lookup("bottleneck-stage"); declared {
			stage = askassay.Unavailable(q, bottleneckWriteReason, ix.AsOf)
		}
	}
	d := Doc{Title: "Bottleneck report", AsOf: ix.AsOf}
	d.Sections = []Section{
		{
			Heading:   "The constraint",
			Narrative: narrativeFor(narrative, "The constraint"),
			Figures: []Figure{
				{Caption: "WIP at the likely constraint stage", Answer: stage},
				{Caption: "throughput over the window", Answer: ix.Require("flow-throughput")},
			},
		},
		{
			Heading:   "Load",
			Narrative: narrativeFor(narrative, "Load"),
			Figures: []Figure{
				{Caption: "open PR actions", Answer: ix.Require("pr-action-count")},
				{Caption: "PRs awaiting a human", Answer: ix.Require("awaiting-count")},
			},
		},
	}
	return d.Build()
}

// bottleneckWriteReason is the measured reason, quoted on the page.
const bottleneckWriteReason = "the only declared source for this figure writes a dated report file as a side effect of being read, so a read-only pane cannot run it; supply the figure from an already-written report instead"

// DecisionLatency builds the decision-latency report.
func DecisionLatency(ix Index, narrative map[string][]string) (Doc, error) {
	d := Doc{Title: "Decision latency", AsOf: ix.AsOf}
	d.Sections = []Section{
		{
			Heading:   "Waiting on a human",
			Narrative: narrativeFor(narrative, "Waiting on a human"),
			Figures: []Figure{
				{Caption: "PRs awaiting a human", Answer: ix.Require("awaiting-count")},
				{Caption: "open PR actions", Answer: ix.Require("pr-action-count")},
				{Caption: "PR inventory in scope", Answer: ix.Require("pr-inventory-count")},
			},
		},
		{
			Heading:   "Movement",
			Narrative: narrativeFor(narrative, "Movement"),
			Figures: []Figure{
				{Caption: "throughput over the window", Answer: ix.Require("flow-throughput")},
				{Caption: "gate score", Answer: ix.Require("gate-score")},
			},
		},
	}
	return d.Build()
}

func narrativeFor(m map[string][]string, heading string) []string {
	// Indexing a map is deterministic; RANGING one is not. Only indexing
	// happens here, and the result is copied so a caller cannot mutate a
	// built document out from under a render.
	return append([]string(nil), m[heading]...)
}

// ---------------------------------------------------------------------------
// The probe guard
// ---------------------------------------------------------------------------

// writeSideEffectModes are statusgen modes that LOOK like reads and write.
// Each one is here because the write was found in the tool's source, not
// because the flag name suggested it.
var writeSideEffectModes = [][2]string{
	{"--bottleneck", "runBottleneck unconditionally calls writeBottleneckReport, creating docs/reports/factory-floor/<date>.md; that path has no publication-manifest row, so one read-looking probe makes the publication stager refuse the tree"},
}

// GuardReportProbe is the read-only guard a report generator must pass an
// argv through. It is the inherited pane guard PLUS a refusal of the modes
// whose read has a write side effect.
//
// It is deliberately a NARROWING, not a replacement: anything the inherited
// guard refuses stays refused, and this adds to the refusal set. A guard that
// can widen the inherited one is not a guard.
func GuardReportProbe(argv []string) error {
	if err := askassay.GuardReadOnly(argv); err != nil {
		return err
	}
	for _, a := range argv {
		for _, m := range writeSideEffectModes {
			if a == m[0] {
				return fmt.Errorf("%w: %s writes — %s", askassay.ErrRefused, m[0], m[1])
			}
		}
	}
	return nil
}

// WriteSideEffectModes reports the declared write-side-effect modes, so a
// caller and a test can enumerate them without reading this file.
func WriteSideEffectModes() [][2]string {
	return append([][2]string(nil), writeSideEffectModes...)
}
