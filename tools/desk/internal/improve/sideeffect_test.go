package improve

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/askassay"
)

// TestImprovePaneRefusesTheMeasuredWriteSideEffect is the finding of this
// change made executable. `statusgen --bottleneck` was measured writing a dated
// report file, so it is not a read at any strength.
//
// At authoring time the answer layer's read-only allow-list PERMITTED it (the
// mode was named in its read set), so this package's second-layer guard was the
// only thing catching it. Since then that hole was closed upstream:
// `askassay.GuardReadOnly` now REFUSES `statusgen --bottleneck` by name. So the
// test asserts both halves refuse it — the upstream guard AND this package's
// guard. The second-layer guard is kept deliberately as defense-in-depth
// (belt-and-suspenders): if the answer layer ever regresses and re-permits the
// mode, this package still refuses it and says so.
//
// The mode's original downstream consequence — a tracked report file left
// `docs/reports/` unclassified and failed the publication manifest check — is
// now remediated: a covering `docs/reports/**` withhold row (disposition
// `do-not-copy`) exists in the publication disposition manifest, so the created
// file classifies and the check PASSES. The guard is NOT relaxed by that: the
// write side effect is unchanged and still real, and a read that writes a
// tracked file is refused here on its own merit, independent of any downstream
// check.
func TestImprovePaneRefusesTheMeasuredWriteSideEffect(t *testing.T) {
	argv := []string{"statusgen", "--root", ".", "--bottleneck"}

	// Upstream (answer layer): GuardReadOnly now refuses this mode by name.
	if err := askassay.GuardReadOnly(argv); err == nil {
		t.Fatalf("the upstream read-only guard PERMITTED %v — the upstream guard must refuse this measured write mode by name", argv)
	} else if !errors.Is(err, askassay.ErrRefused) {
		t.Fatalf("the upstream refusal of %v is not classed as a read-only refusal: %v", argv, err)
	}

	// Second-layer (this package): defense-in-depth, kept deliberately. Asserted
	// directly against GuardNoSideEffect because the composed GuardStripSource
	// now short-circuits on the upstream refusal above before this half runs —
	// so the second-layer guard's own refusal is proven in isolation here.
	err := GuardNoSideEffect(argv)
	if err == nil {
		t.Fatalf("GuardNoSideEffect PERMITTED %v — this mode was measured creating a tracked report file in the tree it is pointed at, which is a write a mode declared a READ must never perform", argv)
	}
	if !errors.Is(err, ErrSideEffect) {
		t.Fatalf("refusal is not classed as a side effect: %v", err)
	}
	for _, want := range []string{"factory-floor", "exited 0"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal must name what was measured; %q missing from %q", want, err.Error())
		}
	}

	// The `=` form of the same flag must not slip past the second-layer guard.
	if err := GuardNoSideEffect([]string{"statusgen", "--bottleneck=1"}); !errors.Is(err, ErrSideEffect) {
		t.Fatalf("the joined-flag form slipped past the side-effect guard: %v", err)
	}
}

// TestImprovePaneStripGuardStillPermitsRealReads is the control. A guard that
// refuses everything is not a guard, and the pane still has to be able to read
// the board.
func TestImprovePaneStripGuardStillPermitsRealReads(t *testing.T) {
	for _, argv := range [][]string{
		{"statusgen", "--root", ".", "--json"},
		{"statusgen", "--root", ".", "--dora", "--json"},
		{"statusgen", "--root", ".", "--alarms"},
		{"deskboard", "actions", "--json"},
		{"deskboard", "awaiting", "--json"},
		{"gh", "issue", "list", "--limit", "100"},
	} {
		if err := GuardStripSource(argv); err != nil {
			t.Fatalf("a real read was refused: %v → %v", argv, err)
		}
	}
	// And an outright write is refused as a WRITE, by the first guard, not as
	// a side effect — the order of the two halves is load-bearing.
	err := GuardStripSource([]string{"gh", "pr", "merge", "1"})
	if err == nil {
		t.Fatal("a write verb was permitted")
	}
	if errors.Is(err, ErrSideEffect) {
		t.Fatalf("a write verb was classed as a side effect rather than as a write: %v", err)
	}
}

// TestImprovePaneSideEffectRegisterIsHeldByAHandWrittenRoster keeps the
// register from quietly emptying out. The roster below is written out by hand,
// element for element, and must equal the production list.
//
// This shape exists because of a measured failure on the sibling change: a
// mutation that removed one entry from a list left the suite GREEN, because
// two entries happened to cover the same fixture. A count is not enough; an
// element-for-element roster is.
func TestImprovePaneSideEffectRegisterIsHeldByAHandWrittenRoster(t *testing.T) {
	want := []struct{ binary, mode, writes string }{
		{"statusgen", "--bottleneck", "docs/reports/factory-floor/<YYYY-MM-DD>.md"},
	}
	got := DeclaredSideEffects()
	if len(got) != len(want) {
		t.Fatalf("the side-effect register holds %d row(s), this roster declares %d — a row added or removed without a matching edit here is a side effect the guard can no longer see", len(got), len(want))
	}
	for i := range want {
		if got[i].Binary != want[i].binary || got[i].Mode != want[i].mode || got[i].Writes != want[i].writes {
			t.Fatalf("register row %d = %s %s -> %s, roster says %s %s -> %s",
				i, got[i].Binary, got[i].Mode, got[i].Writes, want[i].binary, want[i].mode, want[i].writes)
		}
		if strings.TrimSpace(got[i].Measured) == "" {
			t.Fatalf("register row %d carries no measurement — a side-effect row nobody watched happen is a guess", i)
		}
		if strings.TrimSpace(got[i].Consequence) == "" {
			t.Fatalf("register row %d does not say why it matters", i)
		}
	}
}

// TestImprovePanePartialMetricIsNotAMeasurement — classification on content,
// because the emitter's own `computed` flag was measured reading true over a
// value that says "partial" in words.
func TestImprovePanePartialMetricIsNotAMeasurement(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		computed bool
		want     askassay.State
		reason   string
	}{
		{"the measured partial, verbatim", "44% (partial: bug-issue signal only)", true, askassay.CouldNotCheck, "partial"},
		{"unknown with the flag down", "unknown", false, askassay.CouldNotCheck, "not computed"},
		{"unknown with the flag UP", "unknown", true, askassay.CouldNotCheck, "unknown"},
		{"the trend emitter's own words", "insufficient history — need at least two recorded transitions to form a trend", true, askassay.CouldNotCheck, "insufficient history"},
		{"empty value", "", true, askassay.CouldNotCheck, "empty value"},
		{"an em-dash placeholder", "—", true, askassay.CouldNotCheck, "—"},
		{"a real measurement", "66.04 commits/day", true, askassay.Checked, ""},
		{"a real zero", "0", true, askassay.Checked, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, reason := ClassifyMetricValue(tc.value, tc.computed)
			if state != tc.want {
				t.Fatalf("state = %q, want %q (reason: %s)", state, tc.want, reason)
			}
			if tc.reason != "" && !strings.Contains(reason, tc.reason) {
				t.Fatalf("reason %q does not name %q", reason, tc.reason)
			}
			if tc.want == askassay.Checked && reason != "" {
				t.Fatalf("a clean measurement should carry no refusal reason, got %q", reason)
			}
		})
	}
}

// TestImprovePanePartialMarkerRosterIsHeldByHand — same shape as the
// side-effect roster, and for the same measured reason: several of these
// markers would each catch the same fixture, so a count-only check would let a
// removal through.
func TestImprovePanePartialMarkerRosterIsHeldByHand(t *testing.T) {
	want := []string{
		"unknown", "partial", "insufficient history", "unavailable",
		"not automated", "requires", "no implemented", "could-not-check", "n/a", "—",
	}
	if len(partialMarkers) != len(want) {
		t.Fatalf("the marker list holds %d entries, this roster declares %d", len(partialMarkers), len(want))
	}
	for i := range want {
		if partialMarkers[i] != want[i] {
			t.Fatalf("marker %d = %q, roster says %q", i, partialMarkers[i], want[i])
		}
		// Each marker must be individually load-bearing: a value containing
		// ONLY that marker must classify as could-not-check.
		state, _ := ClassifyMetricValue("7 "+want[i], true)
		if state != askassay.CouldNotCheck {
			t.Fatalf("marker %q does not actually cause a refusal — it is decoration, not a check", want[i])
		}
	}
}
