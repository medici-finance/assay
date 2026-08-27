package drainloop

import "time"

// Class is the retry taxonomy's THREE states — the honest alternative to a retry/no-retry
// pair. A pair has to guess on a failure it does not recognise, and both guesses are defects:
// loop forever on an unrecognised refusal, or silently drop a transient one. The third state,
// ClassUnknown, is held as itself and routed to a human instead of guessed.
type Class int

const (
	// ClassUnknown — the failure could not be classified. Route it to a human; never retry it
	// blindly and never silently drop it. This is the could-not-check state.
	ClassUnknown Class = iota
	// ClassTransient — a failure that may succeed on a retry (a flake, a rate limit, a
	// retry-after). Retry with backoff, up to a bound.
	ClassTransient
	// ClassDeterministic — a failure that will recur (a refusal, a compile error). Do not
	// retry; land it terminally.
	ClassDeterministic
)

// String renders a Class for logs.
func (c Class) String() string {
	switch c {
	case ClassTransient:
		return "transient"
	case ClassDeterministic:
		return "deterministic"
	default:
		return "unknown"
	}
}

// Classifier maps a failed Result to a Class. It is opt-in: a nil Classifier means every
// failure is ClassUnknown, i.e. routed to a human — the fail-safe default.
type Classifier func(Result) Class

// Decision is what a RetryPolicy decides to do with a failed item.
type Decision int

const (
	// DecisionRoute — hand the item to a human (an unclassifiable failure, or a transient one
	// that has exhausted its attempts). Never silently dropped.
	DecisionRoute Decision = iota
	// DecisionRetry — re-select and re-dispatch the item after Backoff.
	DecisionRetry
	// DecisionTerminal — land the failure as-is; it will not be retried.
	DecisionTerminal
)

// String renders a Decision for logs.
func (d Decision) String() string {
	switch d {
	case DecisionRetry:
		return "retry"
	case DecisionTerminal:
		return "terminal"
	default:
		return "route-to-human"
	}
}

// RetryPolicy is the deskkit-free retry layer: a classifier, an attempt bound, and a backoff.
// It is an OPTIONAL layer — a consumer wires it into its own Land / re-selection (or the
// house wires its deskkit exit-code mapping behind the Classifier). The engine's Run loop does
// not couple to it, keeping the six-method contract frozen-small; RetryPolicy is a facility
// the adapter consults, not a seventh method.
//
// The policy NARROWS only: an unrecognised failure is routed to a human (never retried into a
// loop), and a deterministic failure is terminal (never retried). Only a classified-transient
// failure with attempts remaining is retried.
type RetryPolicy struct {
	// Max is the maximum number of attempts for a transient failure before it is routed to a
	// human. Max <= 1 means "one attempt, then route".
	Max int
	// Classify maps a failed Result to a Class. nil ⇒ every failure is ClassUnknown.
	Classify Classifier
	// Backoff maps an attempt number (0-based: the delay BEFORE the next attempt) to a wait.
	// nil ⇒ no wait.
	Backoff func(attempt int) time.Duration
}

// Decide returns what to do with a result given how many attempts have already been made. A
// non-failing result is DecisionTerminal (nothing to retry). A failing result is classified:
// transient-with-attempts-left retries, transient-exhausted routes to a human, deterministic
// is terminal, and unclassifiable routes to a human.
func (p RetryPolicy) Decide(r Result, attempt int) Decision {
	if !r.failed() {
		return DecisionTerminal
	}
	class := ClassUnknown
	if p.Classify != nil {
		class = p.Classify(r)
	}
	switch class {
	case ClassTransient:
		if attempt+1 >= p.Max {
			return DecisionRoute // exhausted — route, never drop
		}
		return DecisionRetry
	case ClassDeterministic:
		return DecisionTerminal
	default: // ClassUnknown — the third state, never collapsed into a decision
		return DecisionRoute
	}
}

// Wait returns the backoff delay before the given attempt (0-based). Zero if no Backoff is
// set.
func (p RetryPolicy) Wait(attempt int) time.Duration {
	if p.Backoff == nil {
		return 0
	}
	return p.Backoff(attempt)
}

// ExponentialBackoff returns a Backoff that doubles from base each attempt (base, 2*base,
// 4*base, …). A cap of zero means uncapped.
func ExponentialBackoff(base, cap time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		d := base
		for i := 0; i < attempt; i++ {
			d *= 2
			if cap > 0 && d >= cap {
				return cap
			}
		}
		if cap > 0 && d > cap {
			return cap
		}
		return d
	}
}
