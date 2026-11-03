package chat

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

// This file is the ONLY place in the console where a rendered figure is born,
// and it can only be reached with a TOOL RESULT in hand.
//
// The numbers rule itself is not written here. It is IMPORTED from
// github.com/medici-finance/assay/tools/desk/askassay, pinned in this module's
// go.mod to a merged commit and checksummed in go.sum. That is deliberate and
// it is a house rule: ONE binary of the rule, imported, never copied. A second
// copy would drift, and the copy that drifted would be the one rendering the
// number nobody checked.
//
// What this file does is the bridge: it turns an MCP read tool's declared
// probe into an [askassay.Question], hands the tool's value to
// [askassay.Computed], and lets the rule decide whether a digit renders. Every
// downgrade below is the imported rule's decision, not a local one:
//
//   - an incomplete source (no command / probe / window / limit)  → could-not-check
//   - no as-of stamp                                              → could-not-check
//   - a value at or above the source's declared cap (saturated)   → could-not-check
//   - a probe that ran blind, was throttled, or was delegated     → could-not-check
//
// and could-not-check never decays into a zero, because [askassay.Unavailable]
// cannot carry a value at all.

// Figure is one cited number in a chat answer: an [askassay.Answer] plus the
// query key that lets it click through to the Measures decomposition (§5.2).
//
// Both fields are unexported and there is no exported struct literal path to
// one — [FigureFromToolResult] is the only constructor, and it takes raw tool
// output. A caller therefore cannot write Figure{value: 142}: there is no such
// field to write, which is the same technique the imported rule uses one layer
// down.
type Figure struct {
	answer   askassay.Answer
	queryKey string
	scope    string
	tool     string
}

// Answer exposes the underlying rule-governed answer.
func (f Figure) Answer() askassay.Answer { return f.answer }

// QueryKey is the metrics key behind this figure. A figure with no query key
// does not render as a chip (§5.2) — the pane checks this, and the check is
// only meaningful because the key comes from the TOOL's own result rather than
// from anything the model said.
func (f Figure) QueryKey() string { return f.queryKey }

// Tool names the bound read tool whose result produced this figure.
func (f Figure) Tool() string { return f.tool }

// State reports the three-state result, straight off the imported rule.
func (f Figure) State() askassay.State { return f.answer.State() }

// Value returns the figure and whether one exists — false for could-not-check.
func (f Figure) Value() (int64, bool) { return f.answer.Value() }

// Reason reports why the figure is not a plain checked value, if it is not.
func (f Figure) Reason() string { return f.answer.Reason() }

// Render is the rule's own one-line rendering: figure, state, source, probe,
// window, limit, as-of. The pane shows it as the chip's title, so the source
// of every number is one hover away.
func (f Figure) Render() string { return f.answer.Render() }

// MeasuresHref is the §5.2 click-through: the Measures pane's decomposition
// for this figure's key, as a URL-addressable filter (spec §2.8). It returns
// "" when there is no query key, so a figure that cannot be decomposed cannot
// render as a link that goes nowhere.
func (f Figure) MeasuresHref() string {
	if f.queryKey == "" {
		return ""
	}
	q := url.Values{}
	q.Set("key", f.queryKey)
	if f.scope != "" {
		q.Set("scope", f.scope)
	}
	return "/measures?" + q.Encode()
}

// ---- the tool-result → figure bridge ---------------------------------------

// probeJSON mirrors the domain.Probe an MCP read tool attaches to its result.
// Its four fields are exactly the four an [askassay.Source] requires, which is
// not a coincidence: both are the same "one number, one declared source" rule,
// and this bridge is where the console's spelling of it meets the public
// module's.
type probeJSON struct {
	Source string `json:"source"`
	Method string `json:"method"`
	Window string `json:"window"`
	Limit  int64  `json:"limit"`
}

type assessmentJSON struct {
	Confidence string    `json:"confidence"`
	Probe      probeJSON `json:"probe"`
	Reason     string    `json:"reason"`
}

// metricsResultJSON is desk__metrics_query's result shape.
type metricsResultJSON struct {
	Scope           string          `json:"scope"`
	Metric          string          `json:"metric"`
	Value           json.RawMessage `json:"value"`
	Assessment      assessmentJSON  `json:"assessment"`
	ScopeAssessment assessmentJSON  `json:"scope_assessment"`
}

// confidenceCheckedClean is the one assessment value that admits a number.
// Anything else — could-not-check, checked-failed, or a value this build does
// not recognise — is not a pass.
const confidenceCheckedClean = "checked-clean"

// FigureFromToolResult builds a [Figure] from a bound read tool's raw JSON
// result. It is the ONLY constructor of a Figure, and it takes no string the
// model produced: `raw` is what the tool returned, `tool` is the bound name
// the call was dispatched under.
//
// It is EXPORTED, and that costs the design nothing. The invariant this
// package holds is "a figure comes from a tool result", not "only this package
// may build one" — and the signature is what enforces it: there is no
// parameter here a caller could put model text into and get a digit out of.
// Exporting it lets the render layer's own tests build the figures they
// render, from tool results, rather than reaching for a looser back door.
//
// ok=false means this result carries no figure at all (a list read, a shape
// this build does not decode). That is different from a figure that renders
// could-not-check: the first is "no number was claimed here", the second is
// "a number was claimed and the rule refused it", and collapsing them would
// hide exactly the refusals this design exists to make visible.
func FigureFromToolResult(tool string, raw []byte, now time.Time) (Figure, bool) {
	if tool != "desk__metrics_query" {
		// Every other bound tool returns rows, not figures. Their content
		// reaches the model as DATA and reaches the transcript as escaped
		// text; neither path can mint a digit.
		return Figure{}, false
	}
	var res metricsResultJSON
	if err := json.Unmarshal(raw, &res); err != nil {
		return Figure{}, false
	}
	if strings.TrimSpace(res.Metric) == "" {
		return Figure{}, false
	}

	q := questionFromProbe(res.Metric, res.Scope, res.Assessment.Probe)
	stamp := askassay.ClockOnly(now,
		"live index read via "+tool+" — a server-side count with no tree behind it")
	fig := Figure{queryKey: res.Metric, scope: res.Scope, tool: tool}

	// The tool's OWN assessment governs first. A probe that ran blind, was
	// throttled, or was delegated to a substrate this build does not ingest
	// reports could-not-check, and an empty payload from such a probe is
	// BLIND, not idle — it must never become a zero here.
	if res.Assessment.Confidence != confidenceCheckedClean {
		reason := res.Assessment.Reason
		if reason == "" {
			reason = "the tool reported confidence " + res.Assessment.Confidence + " with no reason"
		}
		fig.answer = askassay.Unavailable(q, "the read tool did not return a clean measurement: "+reason, stamp)
		return fig, true
	}

	v, ok := scalarCount(res.Value)
	if !ok {
		// by_urgency and friends are maps: a breakdown is not a figure, and
		// flattening one into a single number would be an invention.
		fig.answer = askassay.Unavailable(q,
			"the result carries a breakdown rather than a single count, and a breakdown has no one figure to render", stamp)
		return fig, true
	}

	// Everything from here is the IMPORTED rule's decision. Computed
	// downgrades on an incomplete source, a missing stamp, a negative count,
	// or saturation of the declared cap.
	fig.answer = askassay.Computed(q, v, stamp)
	return fig, true
}

// questionFromProbe turns an MCP read tool's declared probe into the
// [askassay.Question] the rule validates against. It INVENTS nothing: every
// field is carried across from the tool's own declaration, and a tool that
// declared an incomplete probe produces an incomplete [askassay.Source], which
// the rule then refuses. That refusal is the point — an under-declared source
// must not be papered over here with a plausible default.
func questionFromProbe(metric, scope string, p probeJSON) askassay.Question {
	text := "what is " + metric
	if scope != "" {
		text += " at " + scope + " scope"
	}
	text += "?"
	return askassay.Question{
		ID:    metric,
		Text:  text,
		Class: askassay.ClassInventory,
		Source: askassay.Source{
			Cmd:    p.Source,
			Probe:  p.Method,
			Window: p.Window,
			Limit:  limitWords(p.Limit),
		},
		SaturatesAt: p.Limit,
	}
}

// limitWords renders a probe's row cap as the words [askassay.Source] requires.
// A cap of 0 means the probe imposes none — which is a CLAIM, so it is spelled
// out with its justification rather than left blank. An empty Limit is invalid
// to the rule by design: a source that cannot state its limit has a silent cap.
func limitWords(limit int64) string {
	if limit <= 0 {
		return "none — this read is a local index table scan with no server paging, so it cannot truncate; " +
			"its limit is one of CURRENCY (how fresh the index is), not of count"
	}
	return fmt.Sprintf("%d rows — a read that returns this many is indistinguishable from a truncated one, "+
		"so the rule refuses it rather than reporting it", limit)
}

// scalarCount extracts a single integral count from a tool result's value.
// It accepts a JSON number and nothing else: a string that looks like a
// number, an object, or an array is not a count, and coercing one would be the
// first step of exactly the fabrication this package is built to prevent.
func scalarCount(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false
	}
	v, err := n.Int64()
	if err != nil {
		return 0, false
	}
	return v, true
}
