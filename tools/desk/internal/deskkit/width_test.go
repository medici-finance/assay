package deskkit

import (
	"strconv"
	"strings"
	"testing"
)

// TestDefaultWidthsArePinned is the sibling of TestCapValuesArePinned: it hard-codes the
// shipped defaults so a silent bump goes red HERE, where the argument for each number
// lives, rather than showing up as a throughput change nobody attributed.
//
// The two numbers that matter most are the ones that used to be prose in a skill body.
// This test is what makes moving them out of prose an improvement rather than a relocation:
// prose could drift from the code, and a pinned constant cannot drift from itself.
func TestDefaultWidthsArePinned(t *testing.T) {
	want := map[string]int{
		"worker-desk":    8,
		"pr-review-desk": 5,
		"verify-desk":    1,
		"intake-desk":    1,
		"the-desk":       1,
	}
	for loop, n := range want {
		got, err := DefaultWidth(loop)
		if err != nil {
			t.Fatalf("DefaultWidth(%q): %v", loop, err)
		}
		if got != n {
			t.Errorf("DefaultWidth(%q) = %d, want %d. If this is a deliberate change, the doc "+
				"comment on that row in widthPolicies must carry the argument for the new number "+
				"— the whole point of the table is that a width and its justification move together.",
				loop, got, n)
		}
	}
	if len(widthPolicies) != len(want) {
		t.Errorf("widthPolicies has %d rows, this test pins %d — a new loop's width would otherwise "+
			"ship with nothing asserting its value", len(widthPolicies), len(want))
	}
}

// TestMaxWidth_IsBoundedByTheEnforcedBudget proves the budget arm is DERIVED from
// ratelimit.go rather than being a second copy of its numbers: recomputing the bound here
// from the same constants must reproduce the answer, so a change to a budget constant moves
// the width ceiling with it automatically.
func TestMaxWidth_IsBoundedByTheEnforcedBudget(t *testing.T) {
	// verify-desk is the row where the budget arm actually binds: deskevidence's unnumbered
	// bucket (30) at two charged writes per flip = 15, above its declared max of 6, so the
	// declared max wins — and the arithmetic must still be the arithmetic.
	p := widthPolicies["verify-desk"]
	if got := p.chargedCap(); got != UnnumberedCapFor("deskevidence") {
		t.Errorf("verify-desk chargedCap = %d, want deskevidence's effective unnumbered cap %d — "+
			"the table must READ ratelimit.go, never restate it", got, UnnumberedCapFor("deskevidence"))
	}
	if p.WritesPerItem != 2 {
		t.Errorf("verify-desk WritesPerItem = %d, want 2 (Evidence + README)", p.WritesPerItem)
	}

	// worker-desk's cap is deskpr's REPO-WIDE tier, which AllowWriteRepoWide holds at the
	// per-PR number (20), not the looser per-repo tier (100). Getting this wrong would let a
	// worker pool be sized against a budget five times the one that actually gates it.
	if got := widthPolicies["worker-desk"].chargedCap(); got != RateLimitPerPRPerHour {
		t.Errorf("worker-desk chargedCap = %d, want RateLimitPerPRPerHour %d — `deskpr create` "+
			"meters repo-wide at the PER-PR cap (AllowWriteRepoWide), not at the per-repo tier",
			got, RateLimitPerPRPerHour)
	}
}

// TestMaxWidth_IsTheMinimumOfEveryArm is what makes each ceiling a CHECKED claim rather
// than a comment. Every row's declared maximum happens to be the tighter number today, so a
// test that only compared final answers could not tell a computed-but-not-binding budget arm
// from a deleted one. This recomputes each arm from the same inputs and asserts both that
// MaxWidth equals their minimum and that it names the arm that produced it.
func TestMaxWidth_IsTheMinimumOfEveryArm(t *testing.T) {
	for _, loop := range WidthLoops() {
		p := widthPolicies[loop]
		arms, err := boundArms(loop, p)
		if err != nil {
			t.Fatalf("boundArms(%s): %v", loop, err)
		}
		seen := map[string]bool{}
		want, wantWhy := arms[0].cap, arms[0].why
		for _, a := range arms {
			seen[a.name] = true
			if a.cap < want {
				want, wantWhy = a.cap, a.why
			}
		}
		// Both non-declared arms must be PRESENT where they apply — a missing arm is a
		// ceiling nothing enforces.
		if !seen["budget"] {
			t.Errorf("%s has no budget arm; every role's width must be bounded by the writes it makes", loop)
		}
		if p.TokenBound && !seen["token"] {
			t.Errorf("%s is token-bound but has no token arm", loop)
		}
		got, why, err := MaxWidth(loop)
		if err != nil {
			t.Fatalf("MaxWidth(%s): %v", loop, err)
		}
		if got != want || why != wantWhy {
			t.Errorf("MaxWidth(%s) = %d bound by %q; the minimum of its arms is %d bound by %q",
				loop, got, why, want, wantWhy)
		}
	}
}

// TestMaxWidth_EveryDefaultIsAdmissible is the coherence check between the two halves of
// each row: a shipped default the bound would refuse is a loop that cannot start at its own
// documented width. It has no business shipping, and nothing else would catch it.
func TestMaxWidth_EveryDefaultIsAdmissible(t *testing.T) {
	for _, loop := range WidthLoops() {
		d, err := DefaultWidth(loop)
		if err != nil {
			t.Fatalf("DefaultWidth(%q): %v", loop, err)
		}
		if err := CheckWidth(loop, d); err != nil {
			max, why, _ := MaxWidth(loop)
			t.Errorf("%s's shipped default width %d is REFUSED by its own bound (max %d, bound by %s): %v",
				loop, d, max, why, err)
		}
	}
}

// TestCheckWidth_RefusesOverTheCeilingAndNamesIt is the exit-5 half of the deliverable. A
// refusal that does not state the accepted maximum is one the operator answers by guessing,
// so the number is asserted to be IN the message, not merely returned.
func TestCheckWidth_RefusesOverTheCeilingAndNamesIt(t *testing.T) {
	max, _, err := MaxWidth("pr-review-desk")
	if err != nil {
		t.Fatalf("MaxWidth: %v", err)
	}
	// The desk's own worked example — widening the review pool to 8 — must be ADMISSIBLE.
	// A bound that refuses the motivating case is a bound that will simply be routed around.
	if err := CheckWidth("pr-review-desk", 8); err != nil {
		t.Fatalf("widening pr-review-desk to 8 must be allowed (max %d): %v", max, err)
	}

	over := max + 1
	err = CheckWidth("pr-review-desk", over)
	if err == nil {
		t.Fatalf("CheckWidth(pr-review-desk, %d) = nil, want a refusal (max is %d)", over, max)
	}
	if !IsRefused(err) {
		t.Errorf("over-ceiling width must be Refused (exit %d), got exit %d: %v",
			ExitRefused, ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(max)) {
		t.Errorf("the refusal must NAME the accepted maximum (%d); got: %s", max, err.Error())
	}
}

// TestCheckWidth_ZeroAndNegativeAreRefused: narrowing to nothing is a STOP, and the control
// for stopping is the stop flag — one a human can see armed. A width of 0 would halt a loop
// through a knob nobody inspects when asking "why is this desk idle?".
func TestCheckWidth_ZeroAndNegativeAreRefused(t *testing.T) {
	for _, n := range []int{0, -1} {
		err := CheckWidth("worker-desk", n)
		if err == nil || !IsRefused(err) {
			t.Errorf("CheckWidth(worker-desk, %d) = %v, want a Refused", n, err)
		}
		if err != nil && !strings.Contains(err.Error(), "STOP.") {
			t.Errorf("the refusal should point at the stop flag as the real halt control; got: %s", err.Error())
		}
	}
}

// TestCheckWidth_UnknownLoopIsRefusedWithTheKnownSet: an unrecognised name must not resolve
// to a default. It is the loopnames.go rule applied to this table — a name the roster does
// not know is could-not-check about that loop, never "use the usual number".
func TestCheckWidth_UnknownLoopIsRefusedWithTheKnownSet(t *testing.T) {
	err := CheckWidth("btach-fanout", 4)
	if err == nil || !IsRefused(err) {
		t.Fatalf("CheckWidth on a typo'd loop = %v, want a Refused", err)
	}
	if !strings.Contains(err.Error(), "worker-desk") {
		t.Errorf("the refusal must show the real loop set so a typo is self-correcting; got: %s", err.Error())
	}
}

// TestWidth_RetiredLoopNameResolves proves the table is keyed through the SAME equivalence
// class the stop flag uses: a session still presenting the retired name gets its own width,
// not a refusal. A rename must not silently reset a desk's pool.
func TestWidth_RetiredLoopNameResolves(t *testing.T) {
	got, err := DefaultWidth("batch-fanout") // retired name for worker-desk
	if err != nil {
		t.Fatalf("DefaultWidth(batch-fanout): %v", err)
	}
	want, _ := DefaultWidth("worker-desk")
	if got != want {
		t.Errorf("retired name resolved to width %d, canonical to %d — a rename must not reset the pool", got, want)
	}
}

// TestMaxWidth_TokenArmBindsTheReviewPool pins WHICH arm binds pr-review-desk. The review
// pool has always been narrower than the worker pool because reviewers share one token, and
// that reason must survive as the reason — if the budget arm silently became the binding
// one, a later budget raise would widen the review pool for the wrong cause.
func TestMaxWidth_TokenArmBindsTheReviewPool(t *testing.T) {
	_, why, err := MaxWidth("pr-review-desk")
	if err != nil {
		t.Fatalf("MaxWidth: %v", err)
	}
	if !strings.Contains(why, "token") {
		t.Errorf("pr-review-desk's ceiling should be bound by the shared-token trip; bound by %q instead", why)
	}
}

// TestTokenConcurrencyTrip_BadOverrideIsUnverifiableNotADefault is the could-not-check rule
// on the ceiling itself. Falling back to the shipped default would run the WIDER ceiling on
// a deployment that was trying to declare a narrower one — a checker that cannot read its
// own bound must not come back green.
func TestTokenConcurrencyTrip_BadOverrideIsUnverifiableNotADefault(t *testing.T) {
	t.Setenv(EnvTokenConcurrencyTrip, "lots")
	_, _, err := MaxWidth("pr-review-desk")
	if err == nil {
		t.Fatal("an unparseable token-concurrency override must not silently fall back to the shipped default")
	}
	if !IsUnverifiable(err) {
		t.Errorf("bad override must be Unverifiable (exit %d), got exit %d: %v",
			ExitUnverifiable, ExitCodeOf(err), err)
	}
}

// TestTokenConcurrencyTrip_OverrideNarrows proves the override is live, and narrows.
func TestTokenConcurrencyTrip_OverrideNarrows(t *testing.T) {
	base, _, err := MaxWidth("pr-review-desk")
	if err != nil {
		t.Fatalf("MaxWidth: %v", err)
	}
	t.Setenv(EnvTokenConcurrencyTrip, "4")
	narrowed, why, err := MaxWidth("pr-review-desk")
	if err != nil {
		t.Fatalf("MaxWidth with override: %v", err)
	}
	if narrowed >= base {
		t.Errorf("override 4 gave max %d, was %d — a narrower measured trip must narrow the ceiling (%s)",
			narrowed, base, why)
	}
	if err := CheckWidth("pr-review-desk", base); err == nil {
		t.Errorf("with the trip overridden to 4, width %d must now be refused", base)
	}
}

// TestEffectiveWidth_OpenBreakerPinsAtActive is the breaker clause: widening is never a way
// through an open breaker. Pinning at the CURRENT ACTIVE COUNT — not at the default and not
// at 1 — is what makes it neither a growth nor a kill.
func TestEffectiveWidth_OpenBreakerPinsAtActive(t *testing.T) {
	got, err := EffectiveWidth("worker-desk", 12, 3, true)
	if err != nil {
		t.Fatalf("EffectiveWidth: %v", err)
	}
	if got != 3 {
		t.Errorf("open breaker with 3 in flight: width %d, want 3 — a tripped breaker pins the pool "+
			"at what is already running until it clears", got)
	}
	// And it must not be a disguised kill order either.
	if got < 3 {
		t.Errorf("width %d is below the 3 already in flight — the breaker clause must never shed running agents", got)
	}
}

// TestEffectiveWidth_ClampsToTheCeiling: a stored width that has become inadmissible (a
// budget constant was lowered under it) is clamped, not honoured. The knob is bounded at
// READ time as well as at write time, so a stale roster entry cannot outlive its bound.
func TestEffectiveWidth_ClampsToTheCeiling(t *testing.T) {
	max, _, err := MaxWidth("pr-review-desk")
	if err != nil {
		t.Fatalf("MaxWidth: %v", err)
	}
	got, err := EffectiveWidth("pr-review-desk", max+50, 1, false)
	if err != nil {
		t.Fatalf("EffectiveWidth: %v", err)
	}
	if got != max {
		t.Errorf("stored width %d resolved to %d, want the ceiling %d — a width stored before a "+
			"bound tightened must not survive it", max+50, got, max)
	}
}

// TestEffectiveWidth_UnsetFallsBackToTheDefault: "nobody has set a width" is the ordinary
// case, and it must produce the documented default rather than zero.
func TestEffectiveWidth_UnsetFallsBackToTheDefault(t *testing.T) {
	got, err := EffectiveWidth("worker-desk", 0, 0, false)
	if err != nil {
		t.Fatalf("EffectiveWidth: %v", err)
	}
	want, _ := DefaultWidth("worker-desk")
	if got != want {
		t.Errorf("unset width resolved to %d, want the default %d", got, want)
	}
}

// TestEffectiveWidth_UnknownLoopDoesNotWiden: an unresolvable loop returns the CURRENT
// active count with the error, so could-not-check can neither grow the pool nor shed it.
func TestEffectiveWidth_UnknownLoopDoesNotWiden(t *testing.T) {
	got, err := EffectiveWidth("no-such-desk", 99, 2, false)
	if err == nil {
		t.Fatal("an unknown loop must return an error, not a width")
	}
	if got != 2 {
		t.Errorf("unresolvable width returned %d with 2 in flight, want 2 — ignorance must not resize a pool", got)
	}
}
