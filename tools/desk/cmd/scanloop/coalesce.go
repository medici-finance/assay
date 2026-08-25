package main

import (
	"fmt"
	"time"
)

// coalesce.go — the bounded coalesce window, and the body-regeneration rule it implies.
//
// THE CLASS THIS KILLS. The rule this replaces was unbounded: while a scan PR from this session was
// open, every new inbound appended to it. The PR therefore never reached a stable head — it could
// be approved, CI-green and mergeable-clean and STILL be a growing draft hours later, because each
// coalesce moved the head and re-opened the review cycle, pooling every placeholder in the session
// (including a hot human-filed request) behind one draft that could not land.
//
// THE BOUND. A scan PR YOUNGER than the window absorbs the batch; at or past the window a FRESH
// branch and PR are cut and the sealed one is left at a stable head for review. The window is
// config, not prose, so two sessions cannot hold two different numbers.
//
// THE THIRD STATE. A PR whose creation time cannot be established does NOT coalesce. Could-not-check
// takes the BOUNDED direction here, which is the deliberate asymmetry: an unnecessary extra PR is a
// review-queue cost, while a wrong coalesce re-opens the failure this window exists to close.

// DefaultCoalesceWindow is the shipped bound. It is the smallest window that still lets a burst of
// inbound arriving together share one PR, and small enough that a sealed PR is reviewable while the
// session is still live.
const DefaultCoalesceWindow = 20 * time.Minute

// CoalesceDecision is the three-state verdict.
type CoalesceDecision string

const (
	// CoalesceInto — push into the open scan PR's branch and REGENERATE its title and body.
	CoalesceInto CoalesceDecision = "COALESCE"
	// CoalesceFresh — cut a new branch and open a new draft PR.
	CoalesceFresh CoalesceDecision = "FRESH-PR"
	// CoalesceCouldNotCheck — the open PR's age is unknown. Renders as FRESH-PR at the action
	// level, but is reported distinctly: a bounded action taken for want of a reading is not the
	// same event as one taken on a reading, and collapsing them hides a broken probe.
	CoalesceCouldNotCheck CoalesceDecision = "COULD-NOT-CHECK"
)

// Act returns the action a decision resolves to. COULD-NOT-CHECK resolves to FRESH-PR — the
// bounded direction — while keeping its own identity in the report.
func (d CoalesceDecision) Act() CoalesceDecision {
	if d == CoalesceCouldNotCheck {
		return CoalesceFresh
	}
	return d
}

// OpenScanPR is this session's currently-open scan PR, if any.
type OpenScanPR struct {
	Number int
	Branch string
	// CreatedAt is the PR's own creation time. ZERO means unread — the could-not-check arm. It is
	// deliberately the PR's createdAt and not the branch's first commit: the review cycle the window
	// bounds starts when the PR opens.
	CreatedAt time.Time
}

// CoalescePolicy is the engine-side configuration of the window.
type CoalescePolicy struct {
	// Window is the maximum age of an open scan PR that may still absorb a batch. Zero means
	// DefaultCoalesceWindow. A NEGATIVE window disables coalescing entirely (every batch cuts a
	// fresh PR) — expressible on purpose, because "never coalesce" is a safe posture while "always
	// coalesce" is the bug.
	Window time.Duration
}

func (p CoalescePolicy) window() time.Duration {
	if p.Window == 0 {
		return DefaultCoalesceWindow
	}
	return p.Window
}

// Decide answers whether this batch joins the open scan PR or cuts a fresh one.
func (p CoalescePolicy) Decide(open *OpenScanPR, now time.Time) (CoalesceDecision, string) {
	w := p.window()
	if open == nil || open.Number <= 0 {
		return CoalesceFresh, "no open scan PR from this session — cut a fresh branch and draft PR"
	}
	if w < 0 {
		return CoalesceFresh, fmt.Sprintf("coalescing is disabled by configuration — #%d is left sealed at its current head", open.Number)
	}
	if open.CreatedAt.IsZero() {
		return CoalesceCouldNotCheck, fmt.Sprintf(
			"#%d is open but its creation time could not be read — an unmeasurable age never coalesces; "+
				"cutting a fresh PR is the bounded direction", open.Number)
	}
	age := now.Sub(open.CreatedAt)
	if age < 0 {
		return CoalesceCouldNotCheck, fmt.Sprintf(
			"#%d reports a creation time in the future (clock skew) — the age is not measurable, so it does not coalesce", open.Number)
	}
	if age < w {
		return CoalesceInto, fmt.Sprintf(
			"#%d is %s old, inside the %s window — push into its branch and regenerate its title and body",
			open.Number, roundAge(age), w)
	}
	return CoalesceFresh, fmt.Sprintf(
		"#%d is %s old, at or past the %s window — it stays sealed at a stable head so it can be reviewed and land; "+
			"this batch opens the next window's PR", open.Number, roundAge(age), w)
}

func roundAge(d time.Duration) time.Duration { return d.Round(time.Second) }

// PushAndRegenerate is the body-regeneration rule made STRUCTURAL.
//
// The scan PR's title and body state counts describing a diff that GROWS with every coalesced
// commit, so a body written once is wrong by the second push — a body still claiming the first
// push's counts against a much larger diff is a recurring review finding, not a hypothetical. The
// rule is therefore not "remember to regenerate": push and regenerate are ONE call that cannot
// perform half of itself, and the regeneration is derived from the branch's own diff rather than
// hand-edited.
//
// It returns the error of whichever half failed. A push that succeeded and a regeneration that did
// not is the dangerous state, so the regeneration failure is reported LOUDLY and never swallowed.
func PushAndRegenerate(push func() error, regenerate func() error) error {
	if push == nil || regenerate == nil {
		return fmt.Errorf("scan PR push: both the push and the title/body regeneration must be wired — " +
			"a push without a regeneration leaves the PR stating counts for a diff it no longer has")
	}
	if err := push(); err != nil {
		return fmt.Errorf("scan PR push: %w", err)
	}
	if err := regenerate(); err != nil {
		return fmt.Errorf("scan PR pushed but its title/body were NOT regenerated — the PR now states "+
			"counts for a smaller diff than it carries: %w", err)
	}
	return nil
}
