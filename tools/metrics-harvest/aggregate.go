// aggregate.go — the cross-domain aggregator.
//
// INPUT: the SAME run's metrics-snapshot-<grouping>-<run-id> artifacts (see
// snapshot.go). No committed raw tree, no cross-run Actions-API read.
//
// COMMITTED OUTPUT: the daily roll-up ONLY — reports/daily/<date>/<grouping>/
// rollup.json for each grouping, plus a top-level rollup.json and rollup.md.
//
// SCHEMA: prsOpenByState, issuesOpenByLabel, issuesOpenUnlabeled.
// prsOpenByReviewDecision is emitted null until the collector half (landing separately)
// lands — NEVER zero. Commits-to-main and throughput/lead-time are out of
// scope: they were re-derivable from the API at any later time, so
// snapshotting them into a nightly commit was storage of queryable data.
//
// ---------------------------------------------------------------------------
// THE GAP MODEL — the whole point of this file
// ---------------------------------------------------------------------------
//
// An earlier reducer had eight measured defects, all of one family: a
// COULD-NOT-CHECK RENDERED AS A MEASURED VALUE. An unreadable input published
// `0` at exit 0; an empty-config grouping and an all-unreadable grouping came
// out byte-identical; a duplicate config entry inflated five figures and
// reported clean. Every fix below is named against the defect it closes, and
// each has a regression test in aggregate_test.go that fails if the
// distinction is removed.
//
// Four rules, applied uniformly:
//
//  1. PER-FIGURE, NOT PER-REPO, ACCOUNTING. Each published figure carries its
//     OWN source sets (defect 3: one per-repo boolean could not express
//     "excluded from one figure, included in three", so a 43% undercount and
//     three complete figures shared one row and one gap label).
//
//  2. FOUR STATES, VISIBLE AT THE NUMBER — measured / withheld /
//     could-not-check / not-configured — in the headline table where the
//     figure is read, not only in a section below it (defects 1, 2).
//
//  3. A GAPPED OR TRUNCATED SOURCE REFUSES TO PUBLISH A COUNT rather than
//     publishing a smaller one (defects 1, 3, 8). The measured subtotal
//     survives in JSON as `measuredSubtotal`, explicitly not the figure, and
//     never appears in the markdown.
//
//  4. COMPARISONS REQUIRE COMPARABLE COVERAGE. A day-over-day delta is
//     computed only when both days measured that figure over the SAME source
//     set (defect 4); a prior roll-up that exists and fails to parse is
//     reported as a parse failure, never as "no prior day" (defect 5).
//
// Config-level refusals (duplicate spec, base-name collision — defects
// 6, 7) live in support.go's validateConfig and abort before anything is
// written.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Exit codes. The CI step distinguishes them: 3 means a roll-up WAS published
// and carries at least one non-measured figure — the markers are the day's
// evidence, so the record is still committed and the job then goes red.
const (
	exitOK        = 0
	exitError     = 1 // IO/internal failure; nothing trustworthy written
	exitRefused   = 2 // refusal (bad config, ambiguous input); nothing written
	exitPublished = 3 // published, but with a could-not-check / withheld figure
)

// Figure states, as they appear in JSON and (verbatim) in the markdown.
const (
	stateMeasured      = "measured"
	stateWithheld      = "withheld"
	stateCouldNotCheck = "could-not-check"
	stateNotConfigured = "not-configured"
)

// --- coverage -------------------------------------------------------------

// coverage is one figure's own account of what it covers. Every list is a
// set of "owner/repo" specs (or, when combining groupings, of scope labels),
// sorted for a stable diff.
type coverage struct {
	// Configured — the sources this figure claims to cover.
	Configured []string `json:"configured"`
	// Measured — the sources that actually contributed a value.
	Measured []string `json:"measured"`
	// Failed — sources that were checked and whose check failed (the
	// collector wrote the field as null and exited non-zero).
	Failed []string `json:"failed"`
	// Missing — sources with no snapshot entry, or whose entry never
	// mentioned this field. Could-not-check.
	Missing []string `json:"missing"`
	// Truncated — sources that DECLARED their upstream listing hit the API
	// cap, so their count is a floor, not a count.
	Truncated []string `json:"truncated"`
	// TruncationUndeclared — sources that said nothing either way about
	// truncation (defect 8: the collector's cap warning goes to stderr
	// only and does not cross the artifact boundary). The reducer could not
	// check, and marks the number rather than implying completeness.
	TruncationUndeclared []string `json:"truncationUndeclared"`
	// Unexpected — snapshot entries this figure does NOT claim to cover
	// (config drift). Never summed; always reported.
	Unexpected []string `json:"unexpected,omitempty"`
	// NotConfigured — contributing SCOPES (used only when combining
	// groupings) that configured no source at all. Without this, three
	// groupings of which one is empty would combine into a coverage that
	// looked complete.
	NotConfigured []string `json:"notConfigured,omitempty"`
}

func (c coverage) state() string {
	if len(c.Configured) == 0 {
		return stateNotConfigured
	}
	if len(c.Measured) == 0 {
		return stateCouldNotCheck
	}
	if len(c.Failed)+len(c.Missing)+len(c.Truncated)+len(c.NotConfigured) > 0 {
		return stateWithheld
	}
	return stateMeasured
}

func (c coverage) sortAll() coverage {
	for _, s := range [][]string{c.Configured, c.Measured, c.Failed, c.Missing,
		c.Truncated, c.TruncationUndeclared, c.Unexpected, c.NotConfigured} {
		sort.Strings(s)
	}
	return c
}

func (c *coverage) merge(o coverage) {
	c.Configured = append(c.Configured, o.Configured...)
	c.Measured = append(c.Measured, o.Measured...)
	c.Failed = append(c.Failed, o.Failed...)
	c.Missing = append(c.Missing, o.Missing...)
	c.Truncated = append(c.Truncated, o.Truncated...)
	c.TruncationUndeclared = append(c.TruncationUndeclared, o.TruncationUndeclared...)
	c.Unexpected = append(c.Unexpected, o.Unexpected...)
	c.NotConfigured = append(c.NotConfigured, o.NotConfigured...)
}

// --- published figures ----------------------------------------------------

// measure is one published integer figure. Value is nil for every state but
// `measured`: rule 3 — a gapped source refuses to publish a count.
type measure struct {
	Value *int     `json:"value"`
	State string   `json:"state"`
	Note  string   `json:"note,omitempty"`
	Cov   coverage `json:"coverage"`

	// MeasuredSubtotal is the sum over the measured subset when the figure is
	// withheld. It is a DIAGNOSTIC, not the figure — it is the very number
	// defect 3 measured being published as if it were the total, so it
	// is JSON-only and never rendered in the markdown.
	MeasuredSubtotal *int `json:"measuredSubtotal,omitempty"`
}

// labelMeasure is the open-issue label breakdown. Labels is nil unless the
// figure is measured — a partial label map is a wrong label map, not a small
// one. Note that a label count is an OCCURRENCE count (an issue with two
// labels contributes to two entries), so it is never summed into an issue
// count anywhere in this file.
type labelMeasure struct {
	Labels map[string]int `json:"labels"`
	State  string         `json:"state"`
	Note   string         `json:"note,omitempty"`
	Cov    coverage       `json:"coverage"`
}

// figures is the schema every scope publishes: a grouping, the all-products
// total, and `org`.
type figures struct {
	PRsOpenByState prsStateMeasures `json:"prsOpenByState"`

	// PRsOpenByReviewDecision is null until the collector emits it (landing separately).
	// Null, never zero — a section absent because nothing collects it must
	// not read as "no PRs are awaiting review".
	PRsOpenByReviewDecision     *labelMeasure `json:"prsOpenByReviewDecision"`
	PRsOpenByReviewDecisionNote string        `json:"prsOpenByReviewDecisionNote,omitempty"`

	IssuesOpenByLabel   labelMeasure `json:"issuesOpenByLabel"`
	IssuesOpenUnlabeled measure      `json:"issuesOpenUnlabeled"`
}

type prsStateMeasures struct {
	Draft measure `json:"draft"`
	Ready measure `json:"ready"`
	Total measure `json:"total"`
}

// all returns every integer figure with the JSON path a reader would name it
// by — one list, so the render, the trend and the loud-exit check can never
// drift out of sync with the schema.
func (f *figures) all() []struct {
	Name string
	M    *measure
} {
	return []struct {
		Name string
		M    *measure
	}{
		{"prsOpenByState.total", &f.PRsOpenByState.Total},
		{"prsOpenByState.draft", &f.PRsOpenByState.Draft},
		{"prsOpenByState.ready", &f.PRsOpenByState.Ready},
		{"issuesOpenUnlabeled", &f.IssuesOpenUnlabeled},
	}
}

// groupingRollup is reports/daily/<date>/<grouping>/rollup.json.
type groupingRollup struct {
	Grouping        string         `json:"grouping"`
	Date            string         `json:"date"`
	RunID           string         `json:"runID,omitempty"`
	Source          groupingSource `json:"source"`
	SourceNote      string         `json:"sourceNote,omitempty"`
	ReposConfigured []string       `json:"reposConfigured"`
	figures
}

// topRollup is reports/daily/<date>/rollup.json.
type topRollup struct {
	Date        string                     `json:"date"`
	RunID       string                     `json:"runID,omitempty"`
	Products    map[string]*groupingRollup `json:"products"`
	AllProducts figures                    `json:"allProducts"`
	Org         *groupingRollup            `json:"org"`
	Trend       *trendData                 `json:"trend"`
}

// --- accumulator ----------------------------------------------------------

// figureAcc accumulates ONE figure across a scope's sources, keeping that
// figure's own coverage (rule 1).
type figureAcc struct {
	sum    int
	labels map[string]int
	cov    coverage
}

func newFigureAcc(configured []string) *figureAcc {
	cfg := append([]string(nil), configured...)
	return &figureAcc{labels: map[string]int{}, cov: coverage{Configured: cfg}}
}

func (a *figureAcc) contribute(repo string, v int) {
	a.sum += v
	a.cov.Measured = append(a.cov.Measured, repo)
}

func (a *figureAcc) contributeLabels(repo string, m map[string]int) {
	for k, v := range m {
		a.labels[k] += v
	}
	a.cov.Measured = append(a.cov.Measured, repo)
}

func (a *figureAcc) failed(repo string)  { a.cov.Failed = append(a.cov.Failed, repo) }
func (a *figureAcc) missing(repo string) { a.cov.Missing = append(a.cov.Missing, repo) }
func (a *figureAcc) truncated(repo string) {
	a.cov.Truncated = append(a.cov.Truncated, repo)
}
func (a *figureAcc) truncationUndeclared(repo string) {
	a.cov.TruncationUndeclared = append(a.cov.TruncationUndeclared, repo)
}

const truncationUndeclaredNote = "truncation status was not declared by %d of %d measured " +
	"source(s) — the reducer could not check whether the upstream listing hit its `--limit` " +
	"cap, so this count may be a floor (collector half, landing separately, defect 8)"

func (a *figureAcc) measure() measure {
	cov := a.cov.sortAll()
	m := measure{State: cov.state(), Cov: cov}
	switch m.State {
	case stateMeasured:
		v := a.sum
		m.Value = &v
		if n := len(cov.TruncationUndeclared); n > 0 {
			m.Note = fmt.Sprintf(truncationUndeclaredNote, n, len(cov.Measured))
		}
	case stateWithheld:
		v := a.sum
		m.MeasuredSubtotal = &v
		unconfigured := ""
		if n := len(cov.NotConfigured); n > 0 {
			unconfigured = fmt.Sprintf(", plus %d contributing scope(s) with nothing configured: %v",
				n, cov.NotConfigured)
		}
		m.Note = fmt.Sprintf("withheld: measured %d of %d configured source(s) "+
			"(failed %d, missing %d, truncated %d%s) — a count is not published from a partial "+
			"source set", len(cov.Measured), len(cov.Configured),
			len(cov.Failed), len(cov.Missing), len(cov.Truncated), unconfigured)
	case stateCouldNotCheck:
		m.Note = fmt.Sprintf("could-not-check: none of %d configured source(s) produced a value "+
			"(failed %d, missing %d)", len(cov.Configured), len(cov.Failed), len(cov.Missing))
	case stateNotConfigured:
		m.Note = "not-configured: no repos are configured for this scope, so nothing was measured " +
			"— this is not a zero"
	}
	return m
}

func (a *figureAcc) labelMeasure() labelMeasure {
	cov := a.cov.sortAll()
	lm := labelMeasure{State: cov.state(), Cov: cov}
	base := a.measure()
	lm.Note = base.Note
	if lm.State == stateMeasured {
		lm.Labels = a.labels
	}
	return lm
}

// --- reduction ------------------------------------------------------------

// aggregateGrouping reduces one grouping's snapshot into its published
// figures. Every branch here is a state, not a fallback: there is no path on
// which an unread source becomes a zero.
func aggregateGrouping(grouping string, configured []string, gs *groupingSnapshot,
	dateLabel, runID string) *groupingRollup {

	cfg := append([]string(nil), configured...)
	sort.Strings(cfg)

	draft := newFigureAcc(cfg)
	ready := newFigureAcc(cfg)
	total := newFigureAcc(cfg)
	labels := newFigureAcc(cfg)
	unlabeled := newFigureAcc(cfg)
	decision := newFigureAcc(cfg)
	anyDecisionDeclared := false

	for _, spec := range cfg {
		entry := gs.byRepo[spec]
		if entry == nil {
			// Either the whole grouping's artifact is absent/unreadable, or
			// this repo simply is not in it. Both are could-not-check for
			// every figure — never a zero (defects 1, 7).
			for _, a := range []*figureAcc{draft, ready, total, labels, unlabeled, decision} {
				a.missing(spec)
			}
			continue
		}

		// prsOpenByState — draft/ready/total share one source field, so they
		// share one verdict; they do NOT share it with the issue figures.
		switch entry.stateOf(fieldPRsOpenByState) {
		case fieldOK:
			if applyTruncation(entry, fieldPRsOpenByState, spec, draft, ready, total) {
				draft.contribute(spec, *entry.prsOpenByState.Draft)
				ready.contribute(spec, *entry.prsOpenByState.Ready)
				total.contribute(spec, *entry.prsOpenByState.Draft+*entry.prsOpenByState.Ready)
			}
		case fieldFailed:
			draft.failed(spec)
			ready.failed(spec)
			total.failed(spec)
		default:
			draft.missing(spec)
			ready.missing(spec)
			total.missing(spec)
		}

		switch entry.stateOf(fieldIssuesOpenByLabel) {
		case fieldOK:
			if applyTruncation(entry, fieldIssuesOpenByLabel, spec, labels) {
				labels.contributeLabels(spec, entry.issuesOpenByLabel)
			}
		case fieldFailed:
			labels.failed(spec)
		default:
			labels.missing(spec)
		}

		switch entry.stateOf(fieldIssuesOpenUnlabeled) {
		case fieldOK:
			if applyTruncation(entry, fieldIssuesOpenUnlabeled, spec, unlabeled) {
				unlabeled.contribute(spec, *entry.issuesOpenUnlabeled)
			}
		case fieldFailed:
			unlabeled.failed(spec)
		default:
			unlabeled.missing(spec)
		}

		switch entry.stateOf(fieldPRsOpenByReviewDecision) {
		case fieldOK:
			anyDecisionDeclared = true
			if applyTruncation(entry, fieldPRsOpenByReviewDecision, spec, decision) {
				decision.contributeLabels(spec, entry.reviewDecision)
			}
		case fieldFailed:
			anyDecisionDeclared = true
			decision.failed(spec)
		default:
			decision.missing(spec)
		}
	}

	unexpected := gs.unexpectedRepos(cfg)
	for _, a := range []*figureAcc{draft, ready, total, labels, unlabeled, decision} {
		a.cov.Unexpected = append(a.cov.Unexpected, unexpected...)
	}

	gr := &groupingRollup{
		Grouping:        grouping,
		Date:            dateLabel,
		RunID:           runID,
		Source:          gs.Source,
		SourceNote:      gs.Note,
		ReposConfigured: cfg,
	}
	gr.PRsOpenByState = prsStateMeasures{
		Draft: draft.measure(),
		Ready: ready.measure(),
		Total: total.measure(),
	}
	gr.IssuesOpenByLabel = labels.labelMeasure()
	gr.IssuesOpenUnlabeled = unlabeled.measure()
	if anyDecisionDeclared {
		lm := decision.labelMeasure()
		gr.PRsOpenByReviewDecision = &lm
	} else {
		gr.PRsOpenByReviewDecision = nil
		gr.PRsOpenByReviewDecisionNote = "No source declares prsOpenByReviewDecision yet " +
			"(collector half, landing separately) — the section is null, never zero: an absent collector must not read as " +
			"\"no PR is awaiting review\"."
	}
	return gr
}

// applyTruncation records the source's truncation declaration on every
// affected figure and reports whether the value may be used. A DECLARED
// truncation withholds the figure (rule 3); an UNDECLARED one is recorded and
// marked at the number, because withholding every figure until the collector
// half of defect 8 lands would publish nothing at all.
func applyTruncation(entry *snapshotRepo, field, spec string, accs ...*figureAcc) bool {
	declared, truncated := entry.truncationOf(field)
	switch {
	case declared && truncated:
		for _, a := range accs {
			a.truncated(spec)
		}
		return false
	case !declared:
		for _, a := range accs {
			a.truncationUndeclared(spec)
		}
	}
	return true
}

// combine folds several scopes' same-named figures into one. It re-runs the
// SAME state rule over the merged coverage, so an all-products total is
// `measured` only when every contributing grouping was — a grouping that
// configured nothing enters as NotConfigured and cannot vanish into a
// complete-looking union.
func combine(scopeNames []string, ms []measure) measure {
	agg := &figureAcc{labels: map[string]int{}}
	for i, m := range ms {
		agg.cov.merge(m.Cov)
		if m.Cov.state() == stateNotConfigured {
			agg.cov.NotConfigured = append(agg.cov.NotConfigured, scopeNames[i])
		}
		switch {
		case m.Value != nil:
			agg.sum += *m.Value
		case m.MeasuredSubtotal != nil:
			agg.sum += *m.MeasuredSubtotal
		}
	}
	return agg.measure()
}

func combineLabels(scopeNames []string, ms []labelMeasure) labelMeasure {
	agg := &figureAcc{labels: map[string]int{}}
	for i, m := range ms {
		agg.cov.merge(m.Cov)
		if m.Cov.state() == stateNotConfigured {
			agg.cov.NotConfigured = append(agg.cov.NotConfigured, scopeNames[i])
		}
		for k, v := range m.Labels {
			agg.labels[k] += v
		}
	}
	return agg.labelMeasure()
}

func combineFigures(scopeNames []string, parts []*groupingRollup) figures {
	pick := func(sel func(*groupingRollup) measure) measure {
		ms := make([]measure, 0, len(parts))
		for _, p := range parts {
			ms = append(ms, sel(p))
		}
		return combine(scopeNames, ms)
	}
	var f figures
	f.PRsOpenByState = prsStateMeasures{
		Draft: pick(func(g *groupingRollup) measure { return g.PRsOpenByState.Draft }),
		Ready: pick(func(g *groupingRollup) measure { return g.PRsOpenByState.Ready }),
		Total: pick(func(g *groupingRollup) measure { return g.PRsOpenByState.Total }),
	}
	lms := make([]labelMeasure, 0, len(parts))
	for _, p := range parts {
		lms = append(lms, p.IssuesOpenByLabel)
	}
	f.IssuesOpenByLabel = combineLabels(scopeNames, lms)
	f.IssuesOpenUnlabeled = pick(func(g *groupingRollup) measure { return g.IssuesOpenUnlabeled })

	anyDecision := false
	dms := make([]labelMeasure, 0, len(parts))
	for _, p := range parts {
		if p.PRsOpenByReviewDecision != nil {
			anyDecision = true
			dms = append(dms, *p.PRsOpenByReviewDecision)
		} else {
			dms = append(dms, labelMeasure{Cov: coverage{}})
		}
	}
	if anyDecision {
		lm := combineLabels(scopeNames, dms)
		f.PRsOpenByReviewDecision = &lm
	} else {
		f.PRsOpenByReviewDecisionNote = "No source declares prsOpenByReviewDecision yet " +
			"(collector half, landing separately) — the section is null, never zero."
	}
	return f
}

// --- trend ----------------------------------------------------------------

// deltaMeasure is one day-over-day delta. Withheld unless BOTH days measured
// the figure over the same source set (defect 4: deltas computed across
// differing gap sets fabricated movement that read as clean change).
type deltaMeasure struct {
	Value *int   `json:"value"`
	State string `json:"state"`
	Note  string `json:"note,omitempty"`
}

const (
	deltaComputed = "computed"
	deltaWithheld = "withheld"
)

type deltaSet struct {
	PRsOpenTotal        deltaMeasure `json:"prsOpenTotal"`
	PRsOpenDraft        deltaMeasure `json:"prsOpenDraft"`
	PRsOpenReady        deltaMeasure `json:"prsOpenReady"`
	IssuesOpenUnlabeled deltaMeasure `json:"issuesOpenUnlabeled"`
}

// Trend states.
const (
	trendComputed        = "computed"
	trendNoPriorDay      = "no-prior-day"
	trendPriorUnreadable = "prior-unreadable"
)

type trendData struct {
	PreviousDate string               `json:"previousDate"`
	PreviousPath string               `json:"previousPath"`
	State        string               `json:"state"`
	Note         string               `json:"note,omitempty"`
	AllProducts  *deltaSet            `json:"allProducts,omitempty"`
	Org          *deltaSet            `json:"org,omitempty"`
	Products     map[string]*deltaSet `json:"products,omitempty"`
}

func sameSources(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func deltaOf(cur, prev measure) deltaMeasure {
	if cur.State != stateMeasured || prev.State != stateMeasured {
		return deltaMeasure{State: deltaWithheld, Note: fmt.Sprintf(
			"withheld: the figure is %q today and %q on the prior day — a delta between a "+
				"measured and an unmeasured figure is movement in the instrument, not the world",
			cur.State, prev.State)}
	}
	if !sameSources(cur.Cov.Measured, prev.Cov.Measured) {
		return deltaMeasure{State: deltaWithheld, Note: fmt.Sprintf(
			"withheld: the two days measured different source sets (today %v, prior %v) — the "+
				"difference would be an artifact of the change in coverage (defect 4)",
			cur.Cov.Measured, prev.Cov.Measured)}
	}
	v := *cur.Value - *prev.Value
	return deltaMeasure{Value: &v, State: deltaComputed}
}

func deltaSetOf(cur, prev *figures) *deltaSet {
	return &deltaSet{
		PRsOpenTotal:        deltaOf(cur.PRsOpenByState.Total, prev.PRsOpenByState.Total),
		PRsOpenDraft:        deltaOf(cur.PRsOpenByState.Draft, prev.PRsOpenByState.Draft),
		PRsOpenReady:        deltaOf(cur.PRsOpenByState.Ready, prev.PRsOpenByState.Ready),
		IssuesOpenUnlabeled: deltaOf(cur.IssuesOpenUnlabeled, prev.IssuesOpenUnlabeled),
	}
}

// priorState is the three-state read of the prior day's roll-up. defect
// 5: the previous reducer collapsed "absent" and "present but unparseable"
// into one bool and then ASSERTED the absent one in prose, about a file that
// was sitting on disk.
type priorState int

const (
	priorFound priorState = iota
	priorAbsent
	priorUnreadable
)

func loadPriorRollup(path string) (*topRollup, priorState, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, priorAbsent, fmt.Sprintf("no file at %s", path)
		}
		return nil, priorUnreadable, fmt.Sprintf("%s exists but could not be read: %v", path, err)
	}
	var t topRollup
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, priorUnreadable, fmt.Sprintf(
			"%s exists (%d bytes) and did NOT parse: %v — this is a checked-failed, not an absence",
			path, len(data), err)
	}
	return &t, priorFound, ""
}

func buildTrend(rootAbs, dateLabel string, top *topRollup) (*trendData, error) {
	prevDate, err := previousDate(dateLabel)
	if err != nil {
		return nil, err
	}
	prevPath := filepath.Join(rootAbs, "reports", "daily", prevDate, "rollup.json")
	prev, st, note := loadPriorRollup(prevPath)

	td := &trendData{PreviousDate: prevDate, PreviousPath: prevPath}
	switch st {
	case priorAbsent:
		td.State = trendNoPriorDay
		td.Note = note + " — checked, and it is genuinely not there; no day-over-day delta is computable"
		return td, nil
	case priorUnreadable:
		td.State = trendPriorUnreadable
		td.Note = note
		return td, nil
	}

	td.State = trendComputed
	td.AllProducts = deltaSetOf(&top.AllProducts, &prev.AllProducts)
	if top.Org != nil && prev.Org != nil {
		td.Org = deltaSetOf(&top.Org.figures, &prev.Org.figures)
	}
	td.Products = map[string]*deltaSet{}
	for _, g := range productGroupings {
		cur, okc := top.Products[g]
		pv, okp := prev.Products[g]
		if !okc || !okp {
			continue
		}
		td.Products[g] = deltaSetOf(&cur.figures, &pv.figures)
	}
	return td, nil
}

// --- entry point ----------------------------------------------------------

func runAggregate(args []string) int {
	fs := flag.NewFlagSet("metrics-harvest aggregate", flag.ContinueOnError)
	dateStr := fs.String("date", "", "date YYYY-MM-DD (default: yesterday, UTC)")
	root := fs.String("root", "", "repo root the reports/daily/ output is written under (default: derived from --config)")
	configPath := fs.String("config", "", "path to the domains config file (default: domains.yaml alongside this source)")
	snapshots := fs.String("snapshots", "snapshots", "directory holding this run's downloaded metrics-snapshot-*.json artifacts")
	runID := fs.String("run-id", "", "the CI run id whose snapshot artifacts these are (recorded in the roll-up)")
	if err := fs.Parse(args); err != nil {
		return exitRefused
	}

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitRefused
	}

	r := *root
	if r == "" {
		r = defaultRoot(cfgPath)
	}
	rAbs, err := filepath.Abs(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	dateLabel, err := resolveDate(*dateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitRefused
	}

	snapDir, err := filepath.Abs(*snapshots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	fmt.Fprintf(os.Stderr, "metrics-harvest aggregate: root=%s config=%s snapshots=%s date=%s\n",
		rAbs, cfgPath, snapDir, dateLabel)

	top, err := aggregateForDate(rAbs, cfg, snapDir, dateLabel, *runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitRefused
	}

	if err := writeRollups(rAbs, dateLabel, top); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	problems := loudProblems(top)
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "metrics-harvest aggregate: published %s with %d unmeasured "+
			"figure(s)/comparison(s):\n", dateLabel, len(problems))
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", p)
		}
		return exitPublished
	}
	fmt.Fprintln(os.Stderr, "metrics-harvest aggregate: done, every figure measured")
	return exitOK
}

// aggregateForDate reduces one day. It returns an error only for a refusal —
// ambiguous input the reducer must not guess about.
func aggregateForDate(rootAbs string, cfg domainsConfig, snapDir, dateLabel, runID string) (*topRollup, error) {
	snaps, err := loadSnapshots(snapDir, dateLabel)
	if err != nil {
		return nil, err
	}

	rollups := map[string]*groupingRollup{}
	for _, g := range groupingOrder {
		rollups[g] = aggregateGrouping(g, cfg.Groupings[g], snaps[g], dateLabel, runID)
	}

	top := &topRollup{
		Date:     dateLabel,
		RunID:    runID,
		Products: map[string]*groupingRollup{},
	}
	if orgGrouping != "" {
		top.Org = rollups[orgGrouping]
	}
	parts := make([]*groupingRollup, 0, len(productGroupings))
	for _, g := range productGroupings {
		top.Products[g] = rollups[g]
		parts = append(parts, rollups[g])
	}
	top.AllProducts = combineFigures(productGroupings, parts)

	trend, err := buildTrend(rootAbs, dateLabel, top)
	if err != nil {
		return nil, err
	}
	top.Trend = trend
	return top, nil
}

// loudProblems enumerates everything that should make the CI job red AFTER
// the roll-up is committed: the markers are the day's evidence, so the record
// lands and the run then reports the gap.
func loudProblems(top *topRollup) []string {
	var out []string
	scopes := []struct {
		name string
		f    *figures
	}{{"allProducts", &top.AllProducts}}
	for _, g := range productGroupings {
		scopes = append(scopes, struct {
			name string
			f    *figures
		}{g, &top.Products[g].figures})
	}
	if top.Org != nil {
		scopes = append(scopes, struct {
			name string
			f    *figures
		}{"org", &top.Org.figures})
	}
	for _, s := range scopes {
		for _, fg := range s.f.all() {
			// An UNDECLARED truncation status is marked at the number (see
			// render.go) but is deliberately not loud here: no collector
			// emits the declaration yet (collector lane), so making it red would
			// paint every run red for a condition no run can currently
			// clear, which is how a signal stops being read.
			if fg.M.State != stateMeasured {
				out = append(out, fmt.Sprintf("%s.%s: %s", s.name, fg.Name, fg.M.State))
			}
			if len(fg.M.Cov.Unexpected) > 0 {
				out = append(out, fmt.Sprintf("%s.%s: %d unconfigured source(s) in the snapshot: %v",
					s.name, fg.Name, len(fg.M.Cov.Unexpected), fg.M.Cov.Unexpected))
			}
		}
		if s.f.IssuesOpenByLabel.State != stateMeasured {
			out = append(out, fmt.Sprintf("%s.issuesOpenByLabel: %s", s.name, s.f.IssuesOpenByLabel.State))
		}
	}
	if top.Trend != nil && top.Trend.State == trendPriorUnreadable {
		out = append(out, "trend: the prior day's roll-up exists and did not parse — "+top.Trend.Note)
	}
	return out
}

// --- output ---------------------------------------------------------------

// writeRollups writes the day's SIX files and nothing else: one rollup.json
// per grouping plus the top-level rollup.json/rollup.md. No raw snapshot is
// ever written into the tree.
func writeRollups(rootAbs, dateLabel string, top *topRollup) error {
	for _, g := range groupingOrder {
		gr := top.Products[g]
		if orgGrouping != "" && g == orgGrouping {
			gr = top.Org
		}
		dir := filepath.Join(rootAbs, "reports", "daily", dateLabel, g)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
		if err := writeJSON(filepath.Join(dir, "rollup.json"), gr); err != nil {
			return err
		}
	}
	topDir := filepath.Join(rootAbs, "reports", "daily", dateLabel)
	if err := os.MkdirAll(topDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", topDir, err)
	}
	if err := writeJSON(filepath.Join(topDir, "rollup.json"), top); err != nil {
		return err
	}
	md := renderRollupMarkdown(top)
	if err := os.WriteFile(filepath.Join(topDir, "rollup.md"), []byte(md), 0o644); err != nil {
		return fmt.Errorf("write rollup.md: %w", err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
