package deskkit

// width.go — THE ONE PLACE a desk loop's agent-pool width is declared, and the bound
// that keeps a widened pool inside the budgets this package already enforces.
//
// THE DEFECT THIS CLOSES. Until now the width of every desk's agent pool lived as PROSE
// in a skill body: "a standing pool of N = 8 concurrent workers" and "a standing pool of
// N = 5 concurrent reviewer agents". Three consequences followed from a number that
// exists only as a sentence:
//
//  1. NOTHING COULD READ IT. A coordinator that notices one stage is the bottleneck had
//     no value to move and no value to read back — the pool was a fact about a model's
//     attention, not about the system.
//  2. NOTHING COULD BOUND IT. The sentence justifying N = 5 names the write budget and
//     the token's secondary-rate-limit trip, but nothing checked a width against either,
//     so "just run more reviewers" was one edit away from failing the board closed.
//  3. THE TWO NUMBERS COULD DISAGREE WITH THE CODE. The skill said 8; the engine's
//     Config comment said "batch: 8"; nothing held them together.
//
// So the numbers move here, once, and the skill bodies point at this table instead of
// restating a value that can drift from it.
//
// WHAT THIS FILE IS NOT. It is not a new budget and it does not raise an existing one.
// Every meter in ratelimit.go applies to a widened pool exactly as it applied before —
// this file only refuses a width those meters could not carry, which is strictly a
// narrowing. The safe direction to be wrong in here is LOW: a width refused too tightly
// costs throughput and says so in the refusal; a width admitted too loosely spends the
// repo's whole hourly budget on the first tick and fails the board closed.

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// EnvTokenConcurrencyTrip overrides the measured secondary-rate-limit trip point below.
// Unset is a COMPLETE configuration, not a degraded one: the shipped default is the
// house measurement, and a deployment that has not re-measured should not have to say so.
const EnvTokenConcurrencyTrip = "ASSAY_TOKEN_CONCURRENCY_TRIP"

const (
	// DefaultTokenConcurrencyTrip is the MEASURED point at which concurrent operations on
	// one shared App token draw GitHub's secondary-rate-limit 403 with core budget still
	// remaining: ~16, measured 2026-08-13. The measurement is already recorded in the
	// retry taxonomy's signature table (internal/loopengine/retry.go, "secondary rate
	// limit"); this constant is the same observation in a form a bound can read.
	//
	// It is a TRIP POINT, NOT AN OPERATING WIDTH. A pool sized at the measured trip is a
	// pool that fails closed, which is why tokenConcurrencyMargin below exists and why no
	// role's ceiling is ever this number.
	DefaultTokenConcurrencyTrip = 16

	// WidthTTL is how long a width a coordinator SET stays in force before the loop decays
	// back to its shipped default.
	//
	// A width has to expire, and the reason is the failure it prevents: a desk that widens a
	// pool and then dies would otherwise leave that pool wide forever, with no session left
	// that knows why. Expiry makes the default the resting state and a widening a thing
	// someone is actively holding open — the same shape as the session beacon it sits beside,
	// and 60 minutes is deliberately the same window the roster's own staleness readers use
	// (opmetrics' zombie threshold, deskwt's beacon-freshness window), so a width does not
	// outlive the session record that explains it.
	//
	// Those two readers each declare their own 60-minute constant today. This is the third,
	// which is one too many — but unifying them is a change to their reclaim semantics, not
	// to this feature, so it stays a named follow-up rather than a drive-by.
	WidthTTL = 60 * time.Minute

	// tokenConcurrencyMargin is the divisor between the measured trip point and the widest
	// pool a TOKEN-BOUND role may run (see widthPolicy.TokenBound). 2 holds such a role's
	// steady state at half the measured trip, leaving the other half for the desk's own
	// board sweeps, which run on the same token and are not counted by any pool.
	//
	// This is a DERIVED ceiling, not a re-measurement: nobody has measured the trip point
	// with a pool of N agents each idling between operations. Halving is the conservative
	// reading of a number whose failure mode is the whole board going closed at once.
	// Lowering it is the safe direction; raising it needs a new measurement, not an
	// argument.
	tokenConcurrencyMargin = 2
)

// widthPolicy is one loop's declared pool policy. Every field except Why is an input to
// MaxWidth, so a value that looks wrong can be traced to the constraint it came from
// rather than to a number somebody picked.
type widthPolicy struct {
	// Default is the width the loop runs at with nobody steering it — the number the
	// skill bodies used to state in prose.
	Default int
	// DeclaredMax is the widest this role has an ARGUMENT for, independent of the
	// arithmetic below. It is the ceiling a human reasoned about; the budget and token
	// caps can only narrow it further, never widen past it.
	DeclaredMax int
	// WriteTool is the audit `tool` name whose bucket one item of this role's work
	// charges, and chargedCap is the enforced cap that bucket meters against. Together
	// they are what makes the budget arm of the bound a derivation from ratelimit.go
	// rather than a second copy of its numbers.
	WriteTool  string
	chargedCap func() int
	// WritesPerItem is how many charged outward writes finishing ONE item costs.
	WritesPerItem int
	// TokenBound marks a role whose agents hold a shared App token open for sustained
	// remote reads, so their count — not their write rate — is what approaches the
	// secondary-rate-limit trip. Reviewers are the motivating case and the reason the
	// review pool has always been narrower than the worker pool.
	TokenBound bool
	// DefaultReserve is the shipped per-class concurrency RESERVATION (example-stream/05):
	// a floor of slots held for resume/rework items so fresh dispatch cannot crowd them out
	// under a full pool — the inverse of Symphony's max_concurrent_agents_by_state, which
	// caps a state rather than floors it, because the failure this closes is fresh work
	// starving a resume, never the reverse. nil (the common case) means no reservation for
	// this loop; DefaultReserve(loop) below normalises that to an all-zero map so callers
	// never branch on nil.
	DefaultReserve map[string]int
	Why            string
}

// KnownReserveClasses is the fixed set of concurrency-reservation classes a width entry may
// declare, in DISPLAY order — resume before rework, matching the priority order the dispatch
// spec states (worker-desk SKILL.md §Sources of work rows 3 and 5: resuming started work
// outranks a fresh brief). Fixed rather than derived from any one loop's map so a `--reserve`
// flag and a printed summary always enumerate classes in the SAME order regardless of which
// are zero, and so an unrecognised class name is a REFUSAL (CheckReserve) rather than a typo
// that silently reserves nothing.
var KnownReserveClasses = []string{"resume", "rework"}

func isKnownReserveClass(c string) bool {
	for _, k := range KnownReserveClasses {
		if k == c {
			return true
		}
	}
	return false
}

// cloneReserve normalises a stored/declared reservation map onto the full KnownReserveClasses
// set (missing classes read as 0) and copies it, so a caller can range over the result without
// a nil check and without aliasing the table's own map.
func cloneReserve(in map[string]int) map[string]int {
	out := make(map[string]int, len(KnownReserveClasses))
	for _, c := range KnownReserveClasses {
		out[c] = in[c] // zero value for a class the map does not mention
	}
	return out
}

// FormatReserve renders a reservation map in KnownReserveClasses order as `resume:2,rework:0`
// — the ONE formatting function `deskroster width`, `deskboard throughput` and any future
// reader share, so the printed shape cannot drift between binaries.
func FormatReserve(m map[string]int) string {
	parts := make([]string, 0, len(KnownReserveClasses))
	for _, c := range KnownReserveClasses {
		parts = append(parts, fmt.Sprintf("%s:%d", c, m[c]))
	}
	return strings.Join(parts, ",")
}

// DefaultReserve is the shipped per-class reservation for `loop`, stored beside its width in
// the same widthPolicies row. A loop the table declares no reservation for returns an all-zero
// map (never nil), so a caller can sum or range over it unconditionally.
func DefaultReserve(loop string) (map[string]int, error) {
	_, p, err := policyFor(loop)
	if err != nil {
		return nil, err
	}
	return cloneReserve(p.DefaultReserve), nil
}

// CheckReserve is the gate behind `deskroster width --role <loop> --reserve resume=N,rework=M`:
// admissible only when every class named is a known one, no value is negative, and the SUM
// stays STRICTLY BELOW width. The strict inequality is the point (Verify row 4): a reservation
// that consumes the whole pool — or more — would starve fresh dispatch even while nothing
// reserved is waiting, which is the opposite of what a floor is for. Bound against the width
// explicitly passed in, never re-resolved here, so a set-time check and a set-time write agree
// on which width they judged.
func CheckReserve(loop string, reserve map[string]int, width int) error {
	sum := 0
	for class, n := range reserve {
		if !isKnownReserveClass(class) {
			return Refused(fmt.Sprintf(
				"refused: %q is not a reservation class this roster recognises. Known classes: %v",
				class, KnownReserveClasses))
		}
		if n < 0 {
			return Refused(fmt.Sprintf(
				"refused: --reserve %s=%d is negative; a reservation cannot hold a negative number of slots",
				class, n))
		}
		sum += n
	}
	if sum >= width {
		return Refused(fmt.Sprintf(
			"refused: a reservation summing to %d for %s would consume the whole width (%d) or more, "+
				"starving fresh dispatch even when nothing reserved is waiting — a reservation must never "+
				"idle a slot. The accepted maximum sum is %d.", sum, loop, width, width-1))
	}
	return nil
}

// widthPolicies is the compiled-in table, keyed by CANONICAL LOOP NAME (loopnames.go),
// not by App role. The two are deliberately different vocabularies — several loops can
// share one App identity — and a pool is a property of the LOOP window, which is also
// the thing a skill body describes and a stop flag halts.
//
// Adding a loop here is a PR, on purpose: the same choice loopnames.go made for the same
// reason. There is no widths.env whose corruption could resize every desk at once.
var widthPolicies = map[string]widthPolicy{
	"worker-desk": {
		Default:        8,
		DeclaredMax:    12,
		WriteTool:      "deskpr",
		chargedCap:     func() int { return RateLimitPerPRPerHour },
		WritesPerItem:  1,
		TokenBound:     false,
		DefaultReserve: map[string]int{"resume": 2},
		Why: "one worker item costs one `deskpr create`, which meters REPO-WIDE at the " +
			"per-PR cap (AllowWriteRepoWide). Workers hold their own worktrees and do most " +
			"of their work locally, so the binding constraint is that write budget, not the " +
			"token. The reserve holds 2 of the 8 slots for orphan-PR resumes so a full pool of " +
			"fresh briefs cannot leave a resume waiting (example-stream/05); rework starts at " +
			"0 because it shares row 5's priority with resume but has not yet needed its own " +
			"floor in practice — raise it the same way if it does.",
	},
	"pr-review-desk": {
		Default:       5,
		DeclaredMax:   10,
		WriteTool:     "deskpost",
		chargedCap:    func() int { return RateLimitPerRepoPerHour },
		WritesPerItem: 1,
		TokenBound:    true,
		Why: "a verdict is one charged write and each reviewer works a DIFFERENT PR, so " +
			"the per-PR tier never stacks and the aggregate repo tier is the budget arm. " +
			"Reviewers are gh-read-heavy on one shared token, so the token arm binds first " +
			"— which is the reason this pool has always been narrower than the worker pool.",
	},
	"verify-desk": {
		Default:       1,
		DeclaredMax:   6,
		WriteTool:     "deskevidence",
		chargedCap:    func() int { return UnnumberedCapFor("deskevidence") },
		WritesPerItem: 2,
		TokenBound:    false,
		Why: "a verified flip charges TWO writes (Evidence + README) into deskevidence's " +
			"unnumbered bucket, which is exactly the pacing constraint that bucket's raised " +
			"cap was sized around. Default 1 because this desk has always drained " +
			"sequentially; the ceiling is what a widened drain may reach.",
	},
	"intake-desk": {
		Default:       1,
		DeclaredMax:   1,
		WriteTool:     "deskpr",
		chargedCap:    func() int { return RateLimitPerPRPerHour },
		WritesPerItem: 1,
		TokenBound:    false,
		Why: "STRUCTURALLY ONE, not a throttle: the scan-carrier lane writes a single scan " +
			"branch and a single PR, so two concurrent lanes would race the same head. " +
			"scanloop states this and holds its in-flight cap at 1 deliberately rather than " +
			"as a knob; this row must not become the way that decision is reversed.",
	},
	"the-desk": {
		Default:       1,
		DeclaredMax:   1,
		WriteTool:     "deskpr",
		chargedCap:    func() int { return RateLimitPerPRPerHour },
		WritesPerItem: 1,
		TokenBound:    false,
		Why: "the coordinator is ONE window by construction — it is the arbiter across " +
			"streams, and a second one is a split brain, not more throughput. Listed rather " +
			"than omitted so `width --role the-desk` answers 1 instead of could-not-check.",
	},
}

// tokenConcurrencyTrip resolves the effective trip point: the env override when set and
// parseable, else the shipped measurement.
//
// An override that is present but unusable is UNVERIFIABLE, never a fall-back to the
// default. Falling back would silently run the WIDER shipped ceiling on a deployment that
// tried to declare a narrower one — a checker that could not read its own bound must not
// come back green.
func tokenConcurrencyTrip() (int, string, error) {
	raw, ok := os.LookupEnv(EnvTokenConcurrencyTrip)
	if !ok || raw == "" {
		return DefaultTokenConcurrencyTrip, "shipped default (measured)", nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < tokenConcurrencyMargin {
		return 0, "", Unverifiable(fmt.Sprintf(
			"could-not-check: %s=%q is not an integer >= %d, so the token-concurrency ceiling "+
				"cannot be established. Refusing rather than falling back to the shipped default "+
				"(%d), which is WIDER than any value this override would have set.",
			EnvTokenConcurrencyTrip, raw, tokenConcurrencyMargin, DefaultTokenConcurrencyTrip), nil)
	}
	return n, EnvTokenConcurrencyTrip, nil
}

// WidthLoops returns every loop name this table declares a width for, sorted. It backs
// the refusal message: an operator who named an unknown loop needs to see the real set.
func WidthLoops() []string {
	out := make([]string, 0, len(widthPolicies))
	for k := range widthPolicies {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// policyFor resolves raw to its canonical loop name and that loop's policy. It is the
// SINGLE reader of widthPolicies, so the bound, the default and any test cannot disagree
// about a loop's effective policy.
func policyFor(raw string) (canonical string, p widthPolicy, err error) {
	canonical, known := CanonicalLoopName(raw)
	if !known {
		return "", widthPolicy{}, Refused(fmt.Sprintf(
			"refused: %q is not a loop name this roster recognises. Known loop names: %v",
			raw, KnownLoopNames()))
	}
	p, ok := widthPolicies[canonical]
	if !ok {
		return "", widthPolicy{}, Refused(fmt.Sprintf(
			"refused: loop %q has no declared pool width. Loops with a declared width: %v",
			canonical, WidthLoops()))
	}
	return canonical, p, nil
}

// DefaultWidth is the width `loop` runs at when nobody has set one. It accepts any name
// the loop-name roster recognises, canonical or retired.
func DefaultWidth(loop string) (int, error) {
	_, p, err := policyFor(loop)
	if err != nil {
		return 0, err
	}
	return p.Default, nil
}

// boundArm is one ceiling a width has to survive, with the sentence that explains it.
type boundArm struct {
	name string
	cap  int
	why  string
}

// boundArms computes EVERY ceiling that applies to a loop, rather than folding them into a
// running minimum as they are computed.
//
// The shape is deliberate and it is a testability property, not a style preference. Folded
// into a running min, an arm that does not currently bind ANY row is indistinguishable from
// an arm that is not computed at all: deleting it changes no output, so no behavioural test
// can catch the deletion. Returning all three lets a test assert each arm's arithmetic
// directly and assert that MaxWidth is their minimum — which is what makes "the budget
// bounds the width" a checked claim rather than a comment, even while the declared maximum
// happens to be the tighter number for every row shipped today.
func boundArms(canonical string, p widthPolicy) ([]boundArm, error) {
	arms := []boundArm{{
		name: "declared",
		cap:  p.DeclaredMax,
		why:  fmt.Sprintf("%s's declared maximum", canonical),
	}}

	// Budget arm: how many items of this role's work the enforced write budget can carry in
	// one rolling hour. WritesPerItem is never 0 in the table; guard anyway so a future row
	// that forgets it cannot divide by zero into an unbounded width.
	if p.WritesPerItem > 0 {
		arms = append(arms, boundArm{
			name: "budget",
			cap:  p.chargedCap() / p.WritesPerItem,
			why: fmt.Sprintf("%s's write budget: %d charged writes per rolling hour / %d per item",
				p.WriteTool, p.chargedCap(), p.WritesPerItem),
		})
	}

	// Token arm — token-bound roles only. A worker mostly writes locally; a reviewer holds
	// the shared token open, and it is the COUNT of those that approaches the trip.
	if p.TokenBound {
		trip, source, err := tokenConcurrencyTrip()
		if err != nil {
			return nil, err
		}
		arms = append(arms, boundArm{
			name: "token",
			cap:  trip / tokenConcurrencyMargin,
			why: fmt.Sprintf("the shared App token's secondary-rate-limit trip (~%d concurrent ops, %s) "+
				"halved for margin", trip, source),
		})
	}
	return arms, nil
}

// MaxWidth is the widest pool `loop` may be set to right now, and the one-line reason
// that number and not a larger one. It is the CONJUNCTION of three ceilings — the
// declared maximum, what the role's write budget can carry, and (for a token-bound role)
// the secondary-rate-limit trip with its margin — because a width has to survive all
// three, exactly as ratelimit.go's own tiers are conjunctive.
func MaxWidth(loop string) (int, string, error) {
	canonical, p, err := policyFor(loop)
	if err != nil {
		return 0, "", err
	}

	arms, err := boundArms(canonical, p)
	if err != nil {
		return 0, "", err
	}
	max, why := arms[0].cap, arms[0].why
	for _, a := range arms[1:] {
		if a.cap < max {
			max, why = a.cap, a.why
		}
	}

	if max < 1 {
		// A ceiling below one would mean the role cannot run at all, which is a config
		// defect, not a throughput decision. Refuse rather than silently pin at zero.
		return 0, "", Refused(fmt.Sprintf(
			"refused: the computed maximum width for %s is %d — below one, so this loop could "+
				"not dispatch anything. That is a budget/ceiling misconfiguration, not a width to set.",
			canonical, max))
	}
	return max, why, nil
}

// CheckWidth is the gate behind `deskroster set --role <loop> --width N`. It returns nil
// when N is admissible and a Refused (exit 5) NAMING THE ACCEPTED MAXIMUM otherwise —
// naming it because a refusal that does not say what would be accepted is a refusal the
// operator answers by guessing.
func CheckWidth(loop string, n int) error {
	if n < 1 {
		return Refused(fmt.Sprintf(
			"refused: --width %d is not a pool width; the narrowest a running loop can be is 1. "+
				"To halt a loop use its stop flag (STOP.<name>), which is the control that exists "+
				"for stopping and the one a human can see armed.", n))
	}
	max, why, err := MaxWidth(loop)
	if err != nil {
		return err
	}
	if n > max {
		canonical, _ := CanonicalLoopName(loop)
		return Refused(fmt.Sprintf(
			"refused: --width %d exceeds what %s can carry. The accepted maximum is %d, bound by %s. "+
				"Widening is not a way to buy budget: every meter in the rate limiter applies to the "+
				"wider pool unchanged, so a pool above this number spends the hour's budget early and "+
				"fails the board closed instead of draining it faster.",
			n, canonical, max, why))
	}
	return nil
}

// EffectiveWidth is what a loop may actually run at on THIS tick: the requested width,
// clamped to the role's maximum, except that an OPEN CIRCUIT BREAKER pins the width at
// the count currently in flight until it clears.
//
// Two properties matter and both are about not making a bad tick worse:
//
//   - THE BREAKER IS NEVER WIDENED THROUGH. A breaker is open because attempts are
//     changing nothing; adding agents multiplies the non-progress that opened it. Pinning
//     at activeNow — rather than at the default, or at 1 — means the tick neither grows
//     the pool nor kills work already in flight.
//   - SHRINKING NEVER KILLS. A narrower width is a refusal to REFILL, not a signal to
//     terminate: an agent mid-item keeps its slot and the pool converges downward as
//     items land. That is why this returns a number the caller compares against occupancy
//     and never a set of agents to stop.
//
// A width that cannot be resolved at all (unknown loop, unreadable ceiling) returns
// activeNow with the error: could-not-check must not widen anything.
func EffectiveWidth(loop string, requested, activeNow int, breakerOpen bool) (int, error) {
	if breakerOpen {
		return activeNow, nil
	}
	max, _, err := MaxWidth(loop)
	if err != nil {
		return activeNow, err
	}
	if requested < 1 {
		d, derr := DefaultWidth(loop)
		if derr != nil {
			return activeNow, derr
		}
		requested = d
	}
	if requested > max {
		return max, nil
	}
	return requested, nil
}
