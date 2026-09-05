// Package improve is the render layer behind the Improve pane
// (design §5.3 and §7.3): the four strips —
// Reports, Clusters, Retro queue, Did it work — expressed as types that
// cannot render a lie, rather than as a convention a reviewer has to police.
//
// THE RULE THIS PACKAGE EXISTS FOR
// --------------------------------
// The sibling answer layer made "a figure that cannot be computed renders
// could-not-check, never 0" structural. This package is the COLLECTION half
// of the same rule, and it is the half that goes wrong more quietly:
//
//	An empty strip and an underivable strip look identical on a screen.
//
// A "Retro queue" showing nothing because the probe failed is indistinguishable
// from a Retro queue showing nothing because there are no open proposals — and
// the first reads as "nothing to do" while meaning "you are blind". So:
//
//  1. [Strip] holds its rows in an UNEXPORTED field. [Strip.Render] — the
//     pane's only rendering path — reads the STATE, not the slice. A
//     could-not-check strip cannot render a row area at all, empty or
//     otherwise: it renders the [RowsField] token.
//
//  2. An empty row set is could-not-check BY DEFAULT. Rendering "no rows"
//     requires the strip definition to declare, in writing, why an empty
//     payload from ITS source is a real emptiness ([StripDef.EmptyMeansEmpty]
//     plus [StripDef.EmptyRationale]). An empty result is blind, not idle.
//
//  3. A strip whose cross-stream substrate has not landed CANNOT be derived at
//     all. [Derive] demands a [SubstrateReport] carrying its own source and
//     stamp, and refuses when the named brief is not at a landed status. This
//     is the anti-mock: you cannot fabricate a green strip for a tool that
//     does not exist, because the fabrication has to be spelled as a false
//     board reading with a declared source, which is a reviewable lie rather
//     than an absent-minded fixture.
//
// WHAT IS MEASURED, NOT ASSUMED
// -----------------------------
// Every guard here answers a failure observed on this repo's own data:
//
//   - `statusgen --bottleneck` is a READ-looking mode that WRITES
//     `docs/reports/factory-floor/<date>.md` as a side effect. Measured
//     2026-08-13: the file appeared in a pristine tree, the command exited 0,
//     and `pubmanifest check` then reported `1 unclassified` and FAILED,
//     which makes `pubmanifest stage` refuse to build a tree at all. See
//     sideeffect.go.
//   - `statusgen --trend` prints `insufficient history` and exits 0 when the
//     transition log is missing entirely, so a could-not-check is delivered
//     with the exit code of a measured no-data result (filed separately).
//     Classification here is therefore on CONTENT, never on exit code.
//   - The DORA emitter marks a partial metric `"computed": true`. Measured
//     2026-08-13: `change_failure_rate` reads
//     `"44% (partial: bug-issue signal only)"` with `computed: true`, while
//     three of the five metrics read `unknown` with `computed: false`. A
//     renderer keying on the boolean publishes a partial as a measurement.
//     See doraclass.go.
//
// READ-ONLY
// ---------
// This package runs NOTHING. It has no subprocess call site, opens no file
// and writes no file — it declares what a strip's source is and renders what
// a caller hands it. That is held true by TestImprovePaneHoldsNoWritePath and
// TestImprovePaneRunsNoSubprocess, which read this package's own source. The
// adopt action (§6.2's gold gate) is a routing value, not a write: see gate.go.
package improve

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

// nowFn is the clock seam. It exists so a refusal that has to invent its own
// stamp can still be given one in a test; it is never a source of data.
var nowFn = func() time.Time { return time.Now().UTC() }

// StripID names one of the four Improve-pane strips (§7.3). Panes are named,
// never numbered — so are their strips.
type StripID string

const (
	// StripReports is the good/bad/ugly stream, filterable by class, program
	// and epic; every report links its evidence.
	StripReports StripID = "reports"
	// StripClusters is the grouping of recurring signals into candidate
	// systemic issues, each one click from its members.
	StripClusters StripID = "clusters"
	// StripRetroQueue is the open proposals with their motivating evidence and
	// the human-gated adopt action.
	StripRetroQueue StripID = "retro-queue"
	// StripDidItWork is every adopted retro action with its target metric's
	// before/after. A change that moved nothing is visible — and, just as
	// importantly, a change whose metric cannot be read is NOT reported as
	// having moved nothing.
	StripDidItWork StripID = "did-it-work"
)

// RowsField is the token [Strip.Render] emits in the row-count position when
// no row set exists. It is a constant so a test can assert on it and a
// downstream renderer can key on it rather than pattern-match prose. It is
// deliberately NOT an empty string and NOT "0": both of those render as a
// pane with nothing in it, which is the defect this package exists to kill.
const RowsField = "could-not-check"

// MeasuredEmptyField is the token for a row set that was genuinely read and
// genuinely holds nothing. Reaching it requires a declared rationale.
const MeasuredEmptyField = "measured-empty"

// Strip is one rendered strip. Its rows are unexported and unreachable except
// through [Strip.Rows], which reports ok=false for every state but
// [askassay.Checked] and [askassay.CheckedFailed]. [Strip.Render] reads the
// state, so a strip that could not be derived renders neither rows nor an
// empty row area.
type Strip struct {
	id         StripID
	state      askassay.State
	rows       []Row
	unresolved []Unresolved
	source     askassay.Source
	stamp      askassay.Stamp
	reason     string
	caveats    []string
}

// ID reports which strip this is.
func (s Strip) ID() StripID { return s.id }

// State reports the three-state result.
func (s Strip) State() askassay.State { return s.state }

// Reason reports why the strip is not a plain checked render, if it is not.
func (s Strip) Reason() string { return s.reason }

// Source reports the one declared source behind the strip.
func (s Strip) Source() askassay.Source { return s.source }

// Caveats reports the standing qualifications on this strip.
func (s Strip) Caveats() []string { return append([]string(nil), s.caveats...) }

// Rows returns the row set and whether one exists. ok is false for
// [askassay.CouldNotCheck], and the returned slice is then nil — which is why
// [Strip.Render], not this method, is what a pane calls. A caller that ignores
// ok gets no rows rather than an empty list it might render as "nothing to do".
func (s Strip) Rows() ([]Row, bool) {
	if s.state == askassay.Checked || s.state == askassay.CheckedFailed {
		return append([]Row(nil), s.rows...), true
	}
	return nil, false
}

// Unresolved reports rows that referenced something the strip could not join
// to. They are reported, never dropped and never counted as clean.
func (s Strip) Unresolved() []Unresolved { return append([]Unresolved(nil), s.unresolved...) }

// Render is the pane's single rendering path. It reads the STATE, never the
// row slice, so a strip that could not be derived cannot render an empty list.
//
// The separator is " · " and not "|": these lines are pasted into markdown
// Evidence tables, and a raw pipe shreds the table it lands in.
func (s Strip) Render() string {
	rowsField := RowsField
	if rows, ok := s.Rows(); ok {
		if len(rows) == 0 {
			rowsField = MeasuredEmptyField
		} else {
			rowsField = fmt.Sprintf("%d", len(rows))
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: rows=%s · state=%s · source=%s · probe=%s · window=%s · limit=%s · as-of=%s",
		s.id, rowsField, s.state,
		fallback(s.source.Cmd, "UNDECLARED"),
		fallback(s.source.Probe, "UNDECLARED"),
		fallback(s.source.Window, "UNDECLARED"),
		fallback(s.source.Limit, "UNDECLARED"),
		s.stamp.String())
	if s.reason != "" {
		fmt.Fprintf(&b, "\n  reason: %s", s.reason)
	}
	for _, u := range s.unresolved {
		fmt.Fprintf(&b, "\n  UNRESOLVED: %s", u.String())
	}
	if rows, ok := s.Rows(); ok {
		for _, r := range rows {
			fmt.Fprintf(&b, "\n  row: %s", r.Render())
		}
	}
	for _, c := range s.caveats {
		fmt.Fprintf(&b, "\n  caveat: %s", c)
	}
	return b.String()
}

// Derive is the ONLY constructor that can attach rows to a strip, and it
// refuses to do so unless every condition below holds. Each refusal returns a
// could-not-check strip naming the reason; none returns an error a caller can
// drop, because a dropped error at a render surface becomes an empty pane.
//
//   - the definition declares a complete source (command, probe, window, limit);
//   - the stamp names a time and a ref;
//   - the substrate the strip reads has been MEASURED to have landed;
//   - every row validates;
//   - the row count has not saturated the source's declared cap;
//   - the row set is non-empty, OR the definition declares in writing why an
//     empty payload from its source is a real emptiness.
func Derive(def StripDef, sub SubstrateReport, rows []Row, st askassay.Stamp) Strip {
	if err := def.Validate(); err != nil {
		return Undecidable(def, "the strip definition cannot back a render: "+err.Error(), st)
	}
	if st.Zero() {
		return Undecidable(def,
			"no as-of stamp — every row set this pane renders is derived from live sources that move",
			askassay.ClockOnly(nowFn(), "stamp missing at construction"))
	}
	if err := def.checkSubstrate(sub); err != nil {
		return Undecidable(def, err.Error(), st)
	}
	for i, r := range rows {
		if r == nil {
			return Undecidable(def, fmt.Sprintf("row %d is nil — a hole in a row set renders as a shorter list, which is a silent loss", i), st)
		}
		if err := r.Validate(); err != nil {
			return Undecidable(def, fmt.Sprintf("row %d (%s) cannot render: %s", i, r.RowID(), err.Error()), st)
		}
	}
	if def.SaturatesAt > 0 && len(rows) >= def.SaturatesAt {
		return Undecidable(def, fmt.Sprintf(
			"the read returned %d rows against a declared cap of %d — a saturated list read is indistinguishable from a truncated one, so this is not a row set. Cap: %s",
			len(rows), def.SaturatesAt, def.Source.Limit), st)
	}
	if len(rows) == 0 && !def.EmptyMeansEmpty {
		return Undecidable(def, fmt.Sprintf(
			"the source returned no rows and %s does not declare an empty result to be a real emptiness. An empty strip is BLIND, not idle — rendering it as an empty list would assert that there is nothing to do", def.ID), st)
	}
	s := Strip{
		id: def.ID, state: askassay.Checked, rows: append([]Row(nil), rows...),
		source: def.Source, stamp: st, caveats: def.allCaveats(),
	}
	if len(rows) == 0 {
		s.reason = "measured empty: " + def.EmptyRationale
	}
	return s
}

// Undecidable is the constructor for every case where no row set exists. It
// takes no rows and there is no way to add any afterwards: a could-not-check
// strip cannot decay into an empty list.
func Undecidable(def StripDef, reason string, st askassay.Stamp) Strip {
	if strings.TrimSpace(reason) == "" {
		reason = "unstated — a could-not-check with no reason is not a finding"
	}
	return Strip{
		id: def.ID, state: askassay.CouldNotCheck, source: def.Source,
		stamp: st, reason: reason, caveats: def.allCaveats(),
	}
}

// WithUnresolved attaches join failures to a strip. Unresolved references are
// carried alongside the rows, never subtracted from them: a cluster that lost
// two of its five members is a cluster of five with two unresolved, not a
// tidy cluster of three.
func (s Strip) WithUnresolved(u []Unresolved) Strip {
	s.unresolved = append([]Unresolved(nil), u...)
	return s
}

func fallback(s, alt string) string {
	if strings.TrimSpace(s) == "" {
		return alt
	}
	return s
}
