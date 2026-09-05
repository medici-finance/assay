package improve

import (
	"errors"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/askassay"
)

// THE WRITE-SIDE-EFFECT REGISTER
// ------------------------------
// The answer layer's read-only guard is a verb/mode allow-list, and its own
// header states the bound honestly: "if a permitted read verb acquires a write
// side effect, this guard will not notice."
//
// One already has. Measured 2026-08-13, in a pristine copy of this tree:
//
//	go run ./statusgen --root <tree> --bottleneck   → exit 0
//	<tree>/docs/reports/factory-floor/<date>.md      → CREATED
//
// The mode is in the answer layer's declared READ set. It is a report
// generator whose report is a file, and nothing about the invocation says so.
// The write itself is the hazard this guard exists for: a mode declared a READ
// must not create a tracked file in the tree it is pointed at, whatever
// downstream does or does not notice the new file.
//
// The original blast radius reached further. When this row was first measured,
// `docs/reports/` had NO row in the publication disposition manifest, so once
// the created file was tracked `pubmanifest check` went from PASS to
// `1 unclassified` and FAILED — which made `pubmanifest stage` refuse to build
// a tree at all: one read-looking probe from this pane could break the
// publication pipeline of the repo it was reporting on. That downstream
// consequence is now REMEDIATED elsewhere — a covering `docs/reports/**`
// withhold row (disposition `do-not-copy`) exists in the manifest, so the
// created file classifies and the check PASSES. The guard stays regardless:
// the write side effect is unchanged and still real, and a read that writes is
// refused here on its own merit, not on whether it happens to redden a
// downstream check today.
//
// So this package keeps a SECOND, narrower gate in front of the first: an
// argv must pass the answer layer's read-only guard AND must not appear here.
// The register is deliberately a hand-written list of MEASURED side effects
// rather than a heuristic — a heuristic that guessed which modes write would
// be wrong in both directions, and the point of the list is that each row is
// something somebody watched happen.

// ErrSideEffect is returned for an argv that reads but also writes.
var ErrSideEffect = errors.New("refused: the mode is a declared read but has a measured write side effect")

// SideEffect is one measured write performed by a read-looking invocation.
type SideEffect struct {
	// Binary and Mode identify the invocation.
	Binary string
	Mode   string
	// Writes is the path it creates or modifies, with the varying part in
	// angle brackets.
	Writes string
	// Measured says how the side effect was observed, so a reader can repeat
	// it rather than take it on trust.
	Measured string
	// Consequence says why it matters here, in this repo, today.
	Consequence string
}

func (s SideEffect) String() string {
	return fmt.Sprintf("%s %s writes %s (%s)", s.Binary, s.Mode, s.Writes, s.Consequence)
}

// declaredSideEffects is the register. One row today; a second row is a diff
// somebody reviews rather than a behaviour somebody discovers.
var declaredSideEffects = []SideEffect{
	{
		Binary: "statusgen",
		Mode:   "--bottleneck",
		Writes: "docs/reports/factory-floor/<YYYY-MM-DD>.md",
		Measured: "run against a pristine archive of this tree at a scratch root: the command exited 0, printed the report to stdout, AND created the dated file under the root it was pointed at. " +
			"Re-running it is how to reproduce this row; do not run it against a working tree you care about",
		Consequence: "The write itself is the hazard: a mode declared a READ created a tracked file in the tree it was pointed at. When first measured this also broke publication staging — `docs/reports/` resolved to NO manifest row, so the tracked file drove the manifest check from PASS to `1 unclassified` and a FAILED verdict, and the stager refuses to build a tree while any path is unclassified. That downstream break is now remediated: a covering `docs/reports/**` withhold row (disposition `do-not-copy`) exists in the publication disposition manifest, so the created file classifies and the check PASSES. The guard stays: the read-that-writes is refused on its own merit, independent of any downstream check",
	},
}

// DeclaredSideEffects returns the register.
func DeclaredSideEffects() []SideEffect { return append([]SideEffect(nil), declaredSideEffects...) }

// GuardNoSideEffect refuses an argv that matches a declared side effect.
func GuardNoSideEffect(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("%w: empty argv", ErrSideEffect)
	}
	for _, se := range declaredSideEffects {
		if argv[0] != se.Binary {
			continue
		}
		for _, a := range argv[1:] {
			name := a
			if i := strings.IndexByte(a, '='); i >= 0 {
				name = a[:i]
			}
			if name == se.Mode {
				return fmt.Errorf("%w: %s. Measured: %s", ErrSideEffect, se.String(), se.Measured)
			}
		}
	}
	return nil
}

// GuardStripSource is the gate this pane's sources pass. Both halves are
// required and the order is deliberate: the read-only allow-list first, so an
// outright write verb is refused as a write rather than as a side effect; the
// side-effect register second, so a mode the allow-list permits is still
// stopped when it has been measured writing.
func GuardStripSource(argv []string) error {
	if err := askassay.GuardReadOnly(argv); err != nil {
		return err
	}
	return GuardNoSideEffect(argv)
}

// CONTENT-CLASSIFIED METRIC READS
// -------------------------------
// Two measured cases where the exit code and the success flag both lie:
//
//   - `statusgen --trend` prints `insufficient history — need at least two
//     recorded transitions to form a trend` and exits 0 when the transition
//     log is ABSENT. A could-not-check is delivered wearing the exit code of a
//     measured no-data result. Filed separately.
//   - The flow-metric emitter marks a partial metric `"computed": true`.
//     Measured 2026-08-13 on this repo: the change-failure rate emitted
//     `"44% (partial: bug-issue signal only)"` with `computed: true`, while
//     three of the five metrics emitted `unknown` with `computed: false`.
//
// A did-it-work verdict built on either would be a judgement of a process
// change founded on a number nobody measured. So classification here is on
// CONTENT, and the boolean is treated as necessary but not sufficient.

// partialMarkers are the substrings that make a metric value unusable as one
// half of a before/after comparison, whatever flag accompanies it.
var partialMarkers = []string{
	"unknown",
	"partial",
	"insufficient history",
	"unavailable",
	"not automated",
	"requires",
	"no implemented",
	"could-not-check",
	"n/a",
	"—",
}

// ClassifyMetricValue turns an emitted metric value into a three-state verdict
// for the did-it-work strip. `computed` is the emitter's own flag; it is
// checked, and then the VALUE is checked, because the flag has been measured
// reading true over a partial figure.
//
// A metric that is partial is not half-usable here. The strip's whole job is
// to say whether an adopted change moved a number; half a numerator moving is
// not that, and reporting it as one is how a change gets credited with an
// improvement in the coverage of its own instrument.
func ClassifyMetricValue(value string, computed bool) (askassay.State, string) {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return askassay.CouldNotCheck, "the metric emitted an empty value — an empty figure is blind, not zero"
	}
	if !computed {
		return askassay.CouldNotCheck, fmt.Sprintf("the emitter reports this metric as not computed (value %q) — it is unwired, not flat", value)
	}
	for _, m := range partialMarkers {
		if strings.Contains(v, m) {
			return askassay.CouldNotCheck, fmt.Sprintf(
				"the emitter reports computed=true but the VALUE itself says %q (matched %q). A partial figure cannot serve as one half of a before/after: the strip would credit a process change with a movement that is really a change in how much of the metric is wired. Classified on content, not on the flag",
				value, m)
		}
	}
	return askassay.Checked, ""
}
