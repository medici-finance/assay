// Package askassay is the read-only answer layer behind the Ask Assay pane:
// the NUMBERS RULE (desk-console-2/01, design doc §7.5) expressed as code
// rather than as a convention a reviewer has to police.
//
// THE RULE
// --------
// "The model narrates, the index computes — every figure a chat or chart
// states is a cited query result, never an estimate."
//
// The whole of that rule reduces to three properties this package makes
// structural:
//
//  1. A figure cannot render without its ONE declared source. [Answer] holds
//     its value in an unexported field; the only constructor that can set it
//     is [Computed], and [Computed] DOWNGRADES to could-not-check when the
//     question's source is not fully declared (command, probe, window, limit)
//     or the stamp is empty. There is no path from "the model said 142" to a
//     rendered 142.
//
//  2. A number that cannot be computed renders COULD-NOT-CHECK, never zero.
//     This is the three-state rule from desk-hardening/01 and the single most
//     important property here: a pane that answers "0" for a probe that failed
//     is asserting a falsehood. [Unavailable] cannot carry a value at all —
//     the type makes the mistake unrepresentable rather than discouraged.
//
//  3. No silent caps. Every rendered answer states its probe method, its
//     window and its cap, because [Source] is invalid without all four. And a
//     count that reaches its source's cap is not a count: see
//     [Question.SaturatesAt], which turns a saturated list read into
//     could-not-check.
//
// WHY THE TYPE AND NOT A REVIEW CHECKLIST
// ---------------------------------------
// Every one of these failure modes was measured on this repo's own data, and
// none of them announces itself at the call site:
//
//   - Counts move while you read them. A live issue count was observed going
//     244 -> 242 inside 7.5 minutes. A figure with no as-of stamp is not
//     wrong later; it was never right.
//   - A list read returned exactly 500 rows against a true total of 958, and
//     reported no truncation of any kind. The caller saw a clean exit and a
//     full-looking slice. See caps.go for the instance of that same class
//     still live in this tree.
//   - An empty result is BLIND, not idle. Concurrent operations on one token
//     trip a secondary rate limit and return 403; a probe that comes back
//     empty because it was throttled looks exactly like a probe that came
//     back empty because the answer is zero.
//
// READ-ONLY
// ---------
// Nothing in this package writes. That is enforced structurally in probe.go
// (a closed verb/mode allow-list in front of the single subprocess call site
// in the package) and held true by TestPackageHoldsNoWritePath, which reads
// this package's own source. See probe.go's header for the bound on that
// claim.
package askassay

import (
	"fmt"
	"strings"
	"time"
)

// State is the three-state result from desk-hardening/01. There is no fourth
// state, and in particular there is no state that means "zero because the
// probe came back empty" — that is [CouldNotCheck] unless the question says
// otherwise in writing (see [Question.EmptyMeansZero]).
type State string

const (
	// Checked means the probe ran, the number was derived from its output,
	// and the number is inside the source's declared cap.
	Checked State = "checked"

	// CheckedFailed means the probe ran and reported a real negative — the
	// thing asked about is in a failed state. It carries a value.
	CheckedFailed State = "checked-failed"

	// CouldNotCheck means no number exists. The probe did not run, ran blind,
	// was rate-limited, saturated its cap, or the question's source was not
	// fully declared. It NEVER carries a value, and it never renders as 0.
	CouldNotCheck State = "could-not-check"
)

// Stamp is when a number was computed and against what. A figure without one
// is not an answer: the counts this package reports are all live and all move.
type Stamp struct {
	// At is the wall-clock instant the probe returned, in UTC.
	At time.Time
	// Ref names the tree the number was computed against: a commit SHA where
	// one exists, or an explicit "clock-only: <why>" where the number is a
	// live server-side count with no tree behind it. Empty is invalid.
	Ref string
}

// Zero reports whether the stamp is unusable.
func (s Stamp) Zero() bool { return s.At.IsZero() || strings.TrimSpace(s.Ref) == "" }

func (s Stamp) String() string {
	if s.Zero() {
		return "unstamped"
	}
	return s.At.UTC().Format(time.RFC3339) + "@" + s.Ref
}

// ClockOnly builds a stamp for a live server-side figure that has no tree
// behind it. The reason is mandatory so that "no SHA" is a statement rather
// than an omission.
func ClockOnly(at time.Time, why string) Stamp {
	if strings.TrimSpace(why) == "" {
		why = "unstated"
	}
	return Stamp{At: at, Ref: "clock-only: " + why}
}

// Answer is one rendered figure. Its value is unexported and unreachable
// except through [Answer.Value], which reports ok=false for every state but
// [Checked] and [CheckedFailed]. Callers that ignore ok get no number at all
// rather than a zero, because [Answer.Render] — the pane's only rendering
// path — reads the state, not the field.
type Answer struct {
	question string
	state    State
	value    int64
	source   Source
	stamp    Stamp
	reason   string
	caveats  []string
}

// Computed is the ONLY constructor that can attach a number to an answer, and
// it refuses to do so unless the numbers rule is satisfied:
//
//   - the question declares a complete source (command, probe, window, limit);
//   - the stamp names a time and a ref;
//   - the value has not saturated the source's declared cap.
//
// Every refusal returns a could-not-check answer naming the reason. It never
// returns an error the caller can drop on the floor, because a dropped error
// at a decision surface renders as a confident zero.
func Computed(q Question, v int64, st Stamp) Answer {
	if err := q.Source.Validate(); err != nil {
		return Unavailable(q, "the numbers rule refuses this figure: "+err.Error(), st)
	}
	if st.Zero() {
		return Unavailable(q,
			"the numbers rule refuses this figure: it carries no as-of stamp, and every count this pane reports moves",
			Stamp{At: time.Now().UTC(), Ref: "clock-only: stamp missing at construction"})
	}
	if v < 0 {
		return Unavailable(q, fmt.Sprintf("the probe yielded a negative count (%d), which no source here can legitimately produce", v), st)
	}
	if q.SaturatesAt > 0 && v >= q.SaturatesAt {
		return Unavailable(q, fmt.Sprintf(
			"the read returned %d against a declared cap of %d — a saturated list read is indistinguishable from a truncated one, so there is no count here. Cap: %s",
			v, q.SaturatesAt, q.Source.Limit), st)
	}
	return Answer{question: q.ID, state: Checked, value: v, source: q.Source, stamp: st, caveats: q.allCaveats()}
}

// Failed records a real negative: the probe ran, and what it measured is bad.
// It carries a value because the number is real.
func Failed(q Question, v int64, reason string, st Stamp) Answer {
	a := Computed(q, v, st)
	if a.state != Checked {
		return a
	}
	a.state = CheckedFailed
	a.reason = reason
	return a
}

// Unavailable is the constructor for every case where no number exists. It
// takes no value and there is no way to add one afterwards: could-not-check
// cannot decay into a zero.
func Unavailable(q Question, reason string, st Stamp) Answer {
	if strings.TrimSpace(reason) == "" {
		reason = "unstated — a could-not-check with no reason is not a finding"
	}
	return Answer{question: q.ID, state: CouldNotCheck, source: q.Source, stamp: st, reason: reason, caveats: q.allCaveats()}
}

// Value returns the figure and whether one exists. ok is false for
// [CouldNotCheck], and the returned int64 is then meaningless — which is why
// [Answer.Render], not this method, is what the pane calls.
func (a Answer) Value() (int64, bool) {
	if a.state == Checked || a.state == CheckedFailed {
		return a.value, true
	}
	return 0, false
}

// State reports the three-state result.
func (a Answer) State() State { return a.state }

// Question reports the registry ID this answer came from.
func (a Answer) Question() string { return a.question }

// Reason reports why the answer is not a plain [Checked], if it is not.
func (a Answer) Reason() string { return a.reason }

// Source reports the one declared source behind the figure.
func (a Answer) Source() Source { return a.source }

// Caveats reports the standing qualifications attached to this answer's class.
func (a Answer) Caveats() []string { return append([]string(nil), a.caveats...) }

// FigureField is the token [Answer.Render] emits in the figure position when
// no number exists. It is a constant so that a test can assert on it and a
// downstream renderer can key on it instead of pattern-matching prose.
const FigureField = "could-not-check"

// Render is the pane's single rendering path. It reads the STATE, never the
// value field, so an answer that could not be computed cannot render a digit
// in the figure position — the property this whole package exists to hold.
//
// The separator is " · " and not "|": these lines are pasted into markdown
// Evidence tables, and a raw pipe shreds the table it lands in.
func (a Answer) Render() string {
	figure := FigureField
	if v, ok := a.Value(); ok {
		figure = fmt.Sprintf("%d", v)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: figure=%s · state=%s · source=%s · probe=%s · window=%s · limit=%s · as-of=%s",
		a.question, figure, a.state,
		fallback(a.source.Cmd, "UNDECLARED"),
		fallback(a.source.Probe, "UNDECLARED"),
		fallback(a.source.Window, "UNDECLARED"),
		fallback(a.source.Limit, "UNDECLARED"),
		a.stamp.String())
	if a.reason != "" {
		fmt.Fprintf(&b, "\n  reason: %s", a.reason)
	}
	for _, c := range a.caveats {
		fmt.Fprintf(&b, "\n  caveat: %s", c)
	}
	return b.String()
}

func fallback(s, alt string) string {
	if strings.TrimSpace(s) == "" {
		return alt
	}
	return s
}
