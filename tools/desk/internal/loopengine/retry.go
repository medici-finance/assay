package loopengine

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Retry policy is DECLARED DATA, and its taxonomy has THREE outcomes, not two
// (docs/three-state-instrument-rule.md). Before this file the engine retried nothing: a
// dispatch that failed on a transient 5xx released the claim and dropped the item as if it
// had been refused, and every retry rule in the fleet lived as prose in five desk skills
// ("rerun once for a known flake", "retry the push", "never retry exit 4").
//
// # Why the third state is first-class
//
// A taxonomy with only retryable/non-retryable has to GUESS on a failure it does not
// recognise, and both guesses are defects:
//
//   - guess retryable  -> an unrecognised refusal is re-attempted forever (the
//     self-sustaining starvation, arrived at from a different direction);
//   - guess non-retryable -> a transient failure silently drops the item and the work is
//     lost with no operator ever told.
//
// The motivating measurement (2026-08-13): a PR with ZERO CI check-runs. It was attributed
// to an "App-authored PRs get no CI" anti-recursion rule; that attribution is FALSE — an
// App-authored PR in the same repo carries 19 check-runs, and three PRs sharing both PR
// author and commit author carry 21 / 8 / 0. The real state is COULD-NOT-CHECK: nothing
// observed says whether waiting would help. "Wait and retry" versus "escalate" is not
// derivable from that symptom, so the classifier must be allowed to say so.
//
// ClassUnclassified is therefore NOT a synonym for non-retryable. It is not retried (no
// loop) and it is not landed as a plain BLOCKED either (no silent drop): it lands as
// VerdictNeedsContext, which is the engine's existing route to a human.
//
// # exit 5 is never a retry trigger and never a fallback trigger
//
// Exit 5 is a deliberate safety stop — writeguard, bodycheck, a head-pin mismatch. Retrying
// it identically loops forever (the fix is upstream: reword the body, fix the guard), and
// "fall back to another tool" is the worse sibling — a blocked `git push` answered by
// reaching for the API is the same write with the guard removed. RetryPolicy.NonRetryable
// can only NARROW the retryable set, never widen it, so "retry on 5" is not expressible in
// this package's configuration surface at all. See Retryable.
//
// Note also that exit 5 is ALREADY OVERLOADED — `deskclaim release --kind` returns 5 for a
// plain unknown-flag usage error, which is a deterministic bug, not a safety refusal. This
// package does not add a third meaning to 5; it records that the engine cannot tell the two
// apart from the code alone (see ClassDeterministic).
//
// # Boundaries — cited, not absorbed
//
//   - The liveness path owns TIMEOUTS and heartbeat liveness. An HTTP transport timeout on
//     the dispatch CALL is transport-class here; a dispatched WORKER that stops making
//     progress is the liveness path's, and "context deadline exceeded" is deliberately left
//     unclassified below.
//   - The three-state WorkEvidence probe on Claim landed earlier. This file reuses
//     that shape rather than inventing a second one.
//   - The GitHub-durable refs/dispatch/<id> claim is elsewhere. Retry interacts with
//     claims by HOLDING one across attempts — see dispatchWithRetry.

// FailureClass is the classification of a dispatch failure. It is a string so the declared
// taxonomy table renders and diffs without a second name mapping.
type FailureClass string

const (
	// ClassTransient — the failure is in the transport, not in the work. Retry now, with
	// backoff.
	ClassTransient FailureClass = "transient"
	// ClassRetryAfter — retryable, but ONLY after a wall-clock instant the failure itself
	// stated. Distinct from ClassTransient because retrying it EARLY re-charges the budget
	// that refused it: the wait is data, not a guess.
	ClassRetryAfter FailureClass = "retry-after"
	// ClassRefusal — a guard said no. A refusal is a correct answer, not a failure; retrying
	// it is the bug.
	ClassRefusal FailureClass = "refusal"
	// ClassDeterministic — the call is malformed and will fail identically forever (a usage
	// error, an unknown flag, a wrong ref spelling). Same ACTION as a refusal, different
	// remediation: a refusal needs the input reworded, this needs the caller fixed.
	ClassDeterministic FailureClass = "deterministic"
	// ClassUnclassified — COULD-NOT-CHECK. Nothing observed decides whether waiting helps.
	// Neither retried nor silently dropped.
	ClassUnclassified FailureClass = "unclassified"
)

// Classification is one classifier verdict: the class, the evidence that decided it, and —
// for ClassRetryAfter only — the stated wait.
type Classification struct {
	Class      FailureClass
	Why        string        // NAMES what decided it; never empty
	RetryAfter time.Duration // set only for ClassRetryAfter
}

// TransientError is the TYPED way an adapter says "this failure is transport-class" when it
// knows more than the classifier can see from an error string. Preferred over relying on
// the signature table below.
type TransientError struct {
	Msg string
	Err error
}

func (e *TransientError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *TransientError) Unwrap() error { return e.Err }

// Transient wraps a cause as an explicitly transport-class failure.
func Transient(msg string, err error) *TransientError { return &TransientError{Msg: msg, Err: err} }

// DeterministicError is the TYPED way an adapter says "this call is malformed and will fail
// identically forever". The engine CANNOT infer this from an exit code — every instance in
// the 2026-08-13 corpus (`deskpr create` rc=6 from the `origin/remotes/origin/main`
// spelling; `deskclaim release --kind` rc=5 unknown flag) arrived as a bare 5 or
// 6 and is classified ClassRefusal by the exit-code rule. Same action, so the safety
// property is identical; only the operator-facing remediation differs.
type DeterministicError struct {
	Msg string
	Err error
}

func (e *DeterministicError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *DeterministicError) Unwrap() error { return e.Err }

// Deterministic wraps a cause as a will-fail-identically-forever failure.
func Deterministic(msg string, err error) *DeterministicError {
	return &DeterministicError{Msg: msg, Err: err}
}

// transientSignature is one DECLARED substring that marks a transport-class failure, with
// the measurement behind it. Matching is case-insensitive on the error text.
//
// A signature table is a weak instrument and is treated as one: it can only ADD confidence
// that a failure is transient. Anything it does not match is ClassUnclassified, never
// "probably fine to retry".
type transientSignature struct {
	Match string
	Why   string
}

var transientSignatures = []transientSignature{
	{"529 overloaded", "model API 529 Overloaded — measured 2026-08-13: killed two worker sessions within minutes of each other; both had uncommitted work intact and both completed after resume"},
	{"api error: 529", "the same 529 Overloaded shape carrying the harness's `API Error:` prefix"},
	{"secondary rate limit", "GitHub secondary rate limit (403) — measured 2026-08-13 at ~16 concurrent operations on one shared token with core budget remaining"},
	{"500 internal server error", "GitHub/gh 5xx transport failure"},
	{"502 bad gateway", "GitHub/gh 5xx transport failure"},
	{"503 service unavailable", "GitHub/gh 5xx transport failure"},
	{"504 gateway timeout", "GitHub/gh 5xx transport failure on the dispatch CALL (a dispatched WORKER going quiet is the liveness path's heartbeat liveness, not this)"},
	{"connection reset by peer", "TCP reset mid-call"},
	{"tls handshake timeout", "TLS handshake did not complete"},
}

// UnclassifiedShapes is the DECLARED list of failure shapes this classifier deliberately
// does NOT decide. It is exported so the taxonomy doc renders from it — a taxonomy that
// does not publish its own blind spots is a silent cap.
func UnclassifiedShapes() []string {
	return []string{
		"a bare exit 4 carrying NO retry-after — the wait is unknown, and guessing it short is exactly the retry livelock; deskkit.RateLimited (no retry-after) produces this shape",
		"`context deadline exceeded` and every other timeout — the timeout taxonomy and heartbeat liveness live in liveness.go; classifying timeouts here would fork that decision across two homes",
		"`no such host` / DNS resolution failure — indistinguishable from a permanently wrong hostname without a second observation the engine does not have",
		"any error carrying no deskkit exit code and matching no declared transient signature — the default, and deliberately so",
		"a PR showing ZERO CI check-runs — not a dispatch error at all: it surfaces inside a worker's Result, its cause is could-not-check (the App-authored anti-recursion attribution is measurably false), and 'wait' vs 'escalate' is not derivable from the symptom",
		"a load-induced test flake inside a dispatched worker (a 5s fail-closed callout timeout reddening 3 tests under parallel `go test`) — the dispatch SUCCEEDED, so this is a Result verdict for Land, not a dispatch failure for this classifier",
	}
}

// Classify maps a dispatch error onto the taxonomy. It NEVER returns a bare
// retryable/non-retryable boolean, and it NEVER falls through to "probably transient".
//
// Order is load-bearing:
//
//  1. the TYPED wrappers (TransientError, DeterministicError) — an adapter that knows wins;
//  2. AuthorIsRunnerError (exit 7) — a structural refusal;
//  3. a *deskkit.DeskError's exit code — 3/5/6 refusal, 4 retry-after IF it states one;
//  4. the declared transient signature table;
//  5. ClassUnclassified.
//
// Step 3 does NOT go through deskkit.ExitCodeOf: that helper fails closed by mapping every
// UNRECOGNISED error to 6, which would classify every transient transport failure in the
// corpus as a refusal. Fail-closed is right for an exit status and wrong for a classifier;
// here the fail-closed direction is ClassUnclassified at step 5.
func Classify(err error) Classification {
	if err == nil {
		return Classification{Class: ClassUnclassified, Why: "Classify called with a nil error — nothing to classify"}
	}

	var te *TransientError
	if errors.As(err, &te) {
		return Classification{Class: ClassTransient, Why: "adapter typed it TransientError: " + te.Error()}
	}
	var de *DeterministicError
	if errors.As(err, &de) {
		return Classification{Class: ClassDeterministic, Why: "adapter typed it DeterministicError: " + de.Error()}
	}
	var are *AuthorIsRunnerError
	if errors.As(err, &are) {
		return Classification{Class: ClassRefusal, Why: fmt.Sprintf("exit %d author==runner — a structural refusal, and re-dispatch cannot change who implemented the item", ExitAuthorIsRunner)}
	}

	var dk *deskkit.DeskError
	if errors.As(err, &dk) {
		switch dk.Code {
		case deskkit.ExitDisabled:
			return Classification{Class: ClassRefusal, Why: "exit 3 kill switch armed — the engine's own stop-flag path handles this; retrying would defeat it"}
		case deskkit.ExitRefused:
			return Classification{Class: ClassRefusal, Why: "exit 5 refused by a compiled-in constraint — a deliberate safety stop, never a retry trigger and never a fallback trigger"}
		case deskkit.ExitUnverifiable:
			return Classification{Class: ClassRefusal, Why: "exit 6 a precondition could not be positively verified — fail closed; the engine does not re-attempt a write it could not prove safe"}
		case deskkit.ExitRateLimited:
			if ra := deskkit.RetryAfterOf(err); ra > 0 {
				return Classification{
					Class:      ClassRetryAfter,
					Why:        fmt.Sprintf("exit 4 rate-limited with a STATED retry-after of %s — wait exactly that, once", ra),
					RetryAfter: ra,
				}
			}
			return Classification{Class: ClassUnclassified, Why: "exit 4 rate-limited but carrying NO retry-after — the wait is unknown, and a guessed-short wait is the livelock"}
		}
	}

	low := strings.ToLower(err.Error())
	for _, sig := range transientSignatures {
		if strings.Contains(low, sig.Match) {
			return Classification{Class: ClassTransient, Why: fmt.Sprintf("matched declared transient signature %q — %s", sig.Match, sig.Why)}
		}
	}

	return Classification{
		Class: ClassUnclassified,
		Why:   "no deskkit exit code and no declared transient signature matched — could-not-check, NOT assumed retryable and NOT assumed refused",
	}
}

// RetryPolicy is the DECLARED retry policy — Temporal's activity-retry shape, with the
// bounds this fleet actually needs. Every field is a stated bound; there are no implicit
// caps anywhere in this package.
//
// NonRetryable can only NARROW the retryable set (see Retryable). There is deliberately no
// field that can make ClassRefusal, ClassDeterministic or ClassUnclassified retryable.
type RetryPolicy struct {
	// MaxAttempts is the TOTAL attempts for one item, first attempt included. 1 disables
	// retry entirely. Must be >= 1.
	MaxAttempts int
	// InitialInterval is the delay before the second attempt.
	InitialInterval time.Duration
	// BackoffCoefficient multiplies the interval after each attempt. Must be >= 1.
	BackoffCoefficient float64
	// MaxInterval is the BACKOFF CEILING — no single wait exceeds it, however many
	// attempts have run.
	MaxInterval time.Duration
	// MaxElapsed is the WALL-CLOCK CAP on all waiting for one item, summed across
	// attempts. Reaching it lands the item blocked rather than waiting longer, and it is
	// what keeps a retrying dispatcher's own claim from ageing past Config.StaleClaim
	// while it holds it (Run refuses a config where it could).
	MaxElapsed time.Duration
	// Jitter is the fraction (0..1) by which a computed backoff is randomised, +/-. 0 is
	// exact backoff (what the tests pin); the default spreads a fleet that failed together.
	Jitter float64
	// RetryAfterMaxAttempts caps attempts for a ClassRetryAfter failure specifically, and
	// is deliberately TIGHTER than MaxAttempts: deskkit's own refusal text says "sleep the
	// full retry-after and attempt ONCE". 2 = the original attempt plus that one.
	RetryAfterMaxAttempts int
	// NonRetryable narrows the retryable set further. Listing ClassTransient here disables
	// transport retry; listing an already-non-retryable class is a no-op, not a widening.
	NonRetryable []FailureClass
}

// DefaultRetryPolicy is the conservative default. Every number is stated in the rendered
// bounds table (RenderBoundsTable) so the doc cannot drift from the code.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:           3,
		InitialInterval:       2 * time.Second,
		BackoffCoefficient:    2.0,
		MaxInterval:           60 * time.Second,
		MaxElapsed:            5 * time.Minute,
		Jitter:                0.2,
		RetryAfterMaxAttempts: 2,
	}
}

// Retryable reports whether a class may be re-attempted at all.
//
// The base set is CLOSED: only ClassTransient and ClassRetryAfter are ever retryable, and
// no configuration can add to it. NonRetryable subtracts. This is the structural form of
// "exit 5 is never a retry trigger" — it is not a rule an operator can misconfigure away.
func (p RetryPolicy) Retryable(c FailureClass) bool {
	if c != ClassTransient && c != ClassRetryAfter {
		return false
	}
	return !slices.Contains(p.NonRetryable, c)
}

// attemptCap is the attempt ceiling for a class: ClassRetryAfter gets the tighter
// RetryAfterMaxAttempts, everything else gets MaxAttempts.
func (p RetryPolicy) attemptCap(c FailureClass) int {
	if c == ClassRetryAfter && p.RetryAfterMaxAttempts > 0 {
		return min(p.RetryAfterMaxAttempts, p.MaxAttempts)
	}
	return p.MaxAttempts
}

// Backoff returns the wait BEFORE attempt n+1, given that attempt n just failed (n is
// 1-based). It is exponential from InitialInterval, clamped to the MaxInterval ceiling,
// then jittered by +/- Jitter. It never returns a negative duration.
func (p RetryPolicy) Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := float64(p.InitialInterval)
	coef := p.BackoffCoefficient
	if coef < 1 {
		coef = 1
	}
	for i := 1; i < attempt; i++ {
		d *= coef
		if p.MaxInterval > 0 && d >= float64(p.MaxInterval) {
			d = float64(p.MaxInterval)
			break
		}
	}
	if p.MaxInterval > 0 && d > float64(p.MaxInterval) {
		d = float64(p.MaxInterval)
	}
	if p.Jitter > 0 {
		j := p.Jitter
		if j > 1 {
			j = 1
		}
		d *= 1 - j + 2*j*rand.Float64()
	}
	if d < 0 {
		return 0
	}
	return time.Duration(d)
}

// Validate reports the first stated bound that is incoherent. A policy is configuration a
// human wrote; an incoherent one fails closed at Run rather than silently behaving like
// some other policy.
func (p RetryPolicy) Validate() error {
	switch {
	case p.MaxAttempts < 1:
		return fmt.Errorf("loopengine: RetryPolicy.MaxAttempts must be >= 1, got %d", p.MaxAttempts)
	case p.MaxAttempts > 1 && p.InitialInterval <= 0:
		return fmt.Errorf("loopengine: RetryPolicy.InitialInterval must be > 0 when MaxAttempts > 1, got %s", p.InitialInterval)
	case p.BackoffCoefficient < 1:
		return fmt.Errorf("loopengine: RetryPolicy.BackoffCoefficient must be >= 1, got %v", p.BackoffCoefficient)
	case p.MaxAttempts > 1 && p.MaxInterval <= 0:
		return fmt.Errorf("loopengine: RetryPolicy.MaxInterval (the backoff ceiling) must be > 0 when MaxAttempts > 1 — an unbounded ceiling is a silent cap")
	case p.MaxAttempts > 1 && p.MaxElapsed <= 0:
		return fmt.Errorf("loopengine: RetryPolicy.MaxElapsed (the wall-clock cap) must be > 0 when MaxAttempts > 1 — an unbounded cap is a silent cap")
	case p.Jitter < 0 || p.Jitter > 1:
		return fmt.Errorf("loopengine: RetryPolicy.Jitter must be within [0,1], got %v", p.Jitter)
	case p.RetryAfterMaxAttempts < 0:
		return fmt.Errorf("loopengine: RetryPolicy.RetryAfterMaxAttempts must be >= 0, got %d", p.RetryAfterMaxAttempts)
	}
	return nil
}

// PayloadRetryAttempt / PayloadRetryOf are the Payload keys a retried dispatch carries.
//
// The Loop interface is FROZEN, so there is no Dispatch parameter for "this is attempt 2".
// Payload is the existing template-variable channel and needs no contract change, so the
// resume signal rides there — measured 2026-08-13, the two 529-killed worker sessions BOTH
// had their uncommitted work intact and resuming beat restarting, so an adapter that
// silently restarts from scratch on a retry is throwing away work the engine preserved.
// The engine sets these ONLY from attempt 2 onward: a first dispatch's Payload is
// byte-identical to what it was before this file existed.
const (
	PayloadRetryAttempt = "retry_attempt"
	PayloadRetryOf      = "retry_of"
)

// withAttempt returns a COPY of it whose Payload carries the attempt markers. The copy
// keeps the same ID, so the claim key — and therefore the held claim — is unchanged.
func withAttempt(it Item, attempt, attemptCap int) Item {
	out := it
	p := make(map[string]string, len(it.Payload)+2)
	for k, v := range it.Payload {
		p[k] = v
	}
	p[PayloadRetryAttempt] = fmt.Sprintf("%d", attempt)
	p[PayloadRetryOf] = fmt.Sprintf("%d", attemptCap)
	out.Payload = p
	return out
}

// RetryOutcome is the terminal record of a dispatch that never succeeded: how many attempts
// ran, what the last failure was classified as, and why the engine stopped.
type RetryOutcome struct {
	Attempts int
	Class    FailureClass
	Why      string        // the classifier's evidence
	Stop     string        // why retrying stopped: "non-retryable" | "attempts-exhausted" | "wall-clock-cap"
	Slept    time.Duration // total time actually waited
	Err      error         // the last dispatch error
}

// Verdict maps an outcome onto the engine's Result verdicts. This is where the third state
// earns its keep: an UNCLASSIFIED failure lands NEEDS_CONTEXT (routes to a human) rather
// than BLOCKED (a durable "we decided this is stuck"), because the engine did not decide
// anything — it could not check.
func (o *RetryOutcome) Verdict() string {
	if o.Class == ClassUnclassified {
		return VerdictNeedsContext
	}
	return VerdictBlocked
}

// Summary is the one-line reason recorded on the landed Evidence row.
func (o *RetryOutcome) Summary(itemID string) string {
	return fmt.Sprintf("%s: dispatch failed after %d attempt(s) (waited %s) — class=%s stop=%s — %s",
		itemID, o.Attempts, o.Slept, o.Class, o.Stop, o.Why)
}

// Result renders the outcome as the landable Result the engine files-and-continues with.
func (o *RetryOutcome) Result(it Item, runnerID string) Result {
	return Result{
		Item:     it,
		Verdict:  o.Verdict(),
		RunnerID: runnerID,
		Rows: []EvidenceRow{{
			Command: "loopengine dispatch " + it.ID,
			Exit:    ExitCodeOf(o.Err),
			Output:  o.Summary(it.ID),
		}},
	}
}

// dispatchWithRetry runs Loop.Dispatch under the declared policy and returns either a live
// Handle (nil outcome) or a terminal *RetryOutcome (nil handle).
//
// # How retry interacts with the claim (the invariant that matters)
//
// The caller has ALREADY acquired the item's claim, and this function NEVER releases or
// re-acquires it between attempts. Every attempt re-dispatches the SAME item ID under the
// SAME claim file, so:
//
//   - no attempt can double-acquire — there is only ever one acquire per item per drain
//     pass, made before the first attempt;
//   - no attempt can orphan the claim — the claim outlives the whole retry sequence and the
//     caller releases it exactly once, after the terminal outcome is landed.
//
// A release-then-reclaim between attempts would reopen precisely the double-dispatch window
// claim.go's flock dance exists to close, because the gap between Release and Acquire is
// unlocked and another dispatcher sharing the claims dir would win it.
//
// The claim also AGES while it is held, which is why RetryPolicy.MaxElapsed exists and why
// Run refuses a config whose MaxElapsed could reach Config.StaleClaim: a dispatcher that
// waited past the stale threshold could have its own live claim reclaimed under it by
// another dispatcher that correctly judged it abandoned.
func dispatchWithRetry(cfg Config, loop Loop, it Item, tier Tier) (Handle, *RetryOutcome) {
	pol := cfg.retryPolicy(it)
	var slept time.Duration

	for attempt := 1; ; attempt++ {
		dispatched := it
		if attempt > 1 {
			dispatched = withAttempt(it, attempt, pol.MaxAttempts)
		}

		h, err := loop.Dispatch(dispatched, tier)
		if err == nil {
			if attempt > 1 {
				cfg.logf("retry-recovered: %s on attempt %d/%d (waited %s)", it.ID, attempt, pol.MaxAttempts, slept)
				journal(deskkit.ResultOK, fmt.Sprintf("%s: dispatch recovered on attempt %d/%d after %s of backoff", it.ID, attempt, pol.MaxAttempts, slept))
			}
			return h, nil
		}

		c := Classify(err)
		out := &RetryOutcome{Attempts: attempt, Class: c.Class, Why: c.Why, Slept: slept, Err: err}

		if !pol.Retryable(c.Class) {
			out.Stop = "non-retryable"
			cfg.logf("retry-stop: %s attempt %d — class=%s NOT retryable — %s", it.ID, attempt, c.Class, c.Why)
			journal(journalResult(c.Class), out.Summary(it.ID))
			return nil, out
		}

		attemptCap := pol.attemptCap(c.Class)
		if attempt >= attemptCap {
			out.Stop = "attempts-exhausted"
			cfg.logf("retry-stop: %s exhausted %d/%d attempts — class=%s — %s", it.ID, attempt, attemptCap, c.Class, c.Why)
			journal(deskkit.ResultNoop, out.Summary(it.ID))
			return nil, out
		}

		delay := pol.Backoff(attempt)
		if c.Class == ClassRetryAfter {
			// The failure STATED its wait. Honour it exactly — a shorter backoff re-charges
			// the budget that refused us, which is the whole lesson.
			delay = c.RetryAfter
		}

		if pol.MaxElapsed > 0 && slept+delay > pol.MaxElapsed {
			out.Stop = "wall-clock-cap"
			out.Why = fmt.Sprintf("%s; the next wait of %s would take total waiting to %s, past the declared MaxElapsed cap of %s (free at ~%s) — the engine hands the item back rather than hold its claim that long",
				c.Why, delay, slept+delay, pol.MaxElapsed, time.Now().Add(delay).UTC().Format(time.RFC3339))
			cfg.logf("retry-stop: %s wall-clock cap — %s", it.ID, out.Why)
			journal(deskkit.ResultNoop, out.Summary(it.ID))
			return nil, out
		}

		cfg.logf("retry: %s attempt %d/%d failed (class=%s) — waiting %s — %s", it.ID, attempt, attemptCap, c.Class, delay, c.Why)
		journal(deskkit.ResultNoop, fmt.Sprintf("%s: attempt %d/%d class=%s delay=%s — %s", it.ID, attempt, attemptCap, c.Class, delay, c.Why))
		cfg.sleep(delay)
		slept += delay
		out.Slept = slept
	}
}

// journalResult maps a terminal class onto the audit `result` field. ClassUnclassified is
// ResultUnwritten — the could-not-check billing class: nothing reached a remote, and it is
// NOT ResultRefused because no refusal was ever determined.
func journalResult(c FailureClass) string {
	if c == ClassUnclassified {
		return deskkit.ResultUnwritten
	}
	return deskkit.ResultRefused
}

// journal writes one retry decision to the engine's audit stream (attempt number, class and
// delay all live in detail). This is the groundwork the crash-recovery journal replay reads.
func journal(result, detail string) {
	_ = deskkit.Log(deskkit.Entry{Tool: "loopengine", Verb: "retry", Result: result, Detail: detail})
}

// ---------------------------------------------------------------------------
// The declared taxonomy table — ONE source, rendered into the docs and CI-diffed.
// ---------------------------------------------------------------------------

// TaxonomyRow is one row of the declared taxonomy.
type TaxonomyRow struct {
	Class    FailureClass
	Action   string
	Bound    string
	Evidence string
}

// Taxonomy is the SINGLE DECLARED SOURCE of the retry taxonomy. README.md holds a rendered
// copy between markers and TestRetryTaxonomyDocIsDerivedNotDuplicated diffs it, so the two
// cannot drift (derive-or-diff). Adding a class here and not regenerating the doc is a red
// test, not a stale paragraph.
func Taxonomy() []TaxonomyRow {
	return []TaxonomyRow{
		{
			Class:  ClassTransient,
			Action: "retry, exponential backoff + jitter",
			Bound:  "`MaxAttempts` total attempts; each wait clamped to `MaxInterval`; all waiting summed under `MaxElapsed`",
			Evidence: "measured 2026-08-13: `API Error: 529 Overloaded` killed two worker sessions minutes apart (both resumed and completed); " +
				"GitHub secondary rate limit 403 at ~16 concurrent ops on one token with core budget remaining; gh/GitHub 5xx",
		},
		{
			Class:  ClassRetryAfter,
			Action: "wait the STATED instant, then attempt once",
			Bound:  "`RetryAfterMaxAttempts` (tighter than `MaxAttempts`); a stated wait longer than `MaxElapsed` is NOT slept — the item is landed blocked with the free-at instant",
			Evidence: "deskkit exit 4 carrying `RetryAfterOf`; measured 2026-08-13, `deskfile` rc=4 with its filing budget exhausted until a named wall-clock time. " +
				"Retrying early re-charges the budget that refused us",
		},
		{
			Class:  ClassRefusal,
			Action: "NEVER retry; land blocked",
			Bound:  "exactly one attempt — and `NonRetryable` can only narrow the retryable set, so `retry on 5` is not expressible in this package's config",
			Evidence: "deskkit exit 3 / 5 / 6 and loopengine exit 7. Measured 2026-08-13: `deskpr create` rc=5 on a writeguard false positive, " +
				"rc=5 on two bodycheck refusals, a guard-blocked `git push`. A refusal is a decision, not a failure — the fix is upstream (reword, or fix the guard), " +
				"and reaching for another tool to make the same write is the worse sibling of retrying",
		},
		{
			Class:  ClassDeterministic,
			Action: "NEVER retry; land blocked",
			Bound:  "exactly one attempt; reachable only via the typed `Deterministic()` wrapper — the engine CANNOT infer it from an exit code",
			Evidence: "measured 2026-08-13: `deskpr create` rc=6 from the `origin/remotes/origin/main` spelling (a known ref-spelling bug, hit by every worktree in this repo, every time) " +
				"and `deskclaim release --kind` rc=5, a plain unknown-flag usage error. Both arrive as a bare 5 or 6, so the classifier files them under refusal; " +
				"same action, different remediation",
		},
		{
			Class:  ClassUnclassified,
			Action: "could-not-check: NOT retried, NOT silently dropped — landed `NEEDS_CONTEXT`",
			Bound:  "zero retries and zero waiting; every shape that lands here is published by `UnclassifiedShapes()`",
			Evidence: "the third state of the instrument invariant. Motivating measurement 2026-08-13: a PR with ZERO CI check-runs, attributed to an App-authored " +
				"anti-recursion rule — an attribution that is measurably FALSE (an App-authored PR carries 19 check-runs; three PRs sharing PR author and commit author " +
				"carry 21 / 8 / 0). `wait and retry` versus `escalate` is not derivable from that symptom, and a two-state taxonomy would have guessed",
		},
	}
}

// RenderTaxonomyTable renders Taxonomy() as the markdown table the docs carry.
func RenderTaxonomyTable() string {
	var b strings.Builder
	b.WriteString("| Class | Engine action | Bound | Decided by (measured evidence) |\n")
	b.WriteString("|-------|---------------|-------|-------------------------------|\n")
	for _, r := range Taxonomy() {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", r.Class, r.Action, r.Bound, r.Evidence)
	}
	return b.String()
}

// RenderBoundsTable renders DefaultRetryPolicy's numbers. NO SILENT CAPS: every bound the
// engine applies is published here with what happens when it is reached.
func RenderBoundsTable() string {
	p := DefaultRetryPolicy()
	rows := []struct{ field, value, caps, atBound string }{
		{"`MaxAttempts`", fmt.Sprintf("%d", p.MaxAttempts), "total attempts for one item, first attempt included", "land blocked (`attempts-exhausted`), release the claim, drain continues"},
		{"`InitialInterval`", p.InitialInterval.String(), "the wait before attempt 2", "—"},
		{"`BackoffCoefficient`", fmt.Sprintf("%g", p.BackoffCoefficient), "growth per attempt", "—"},
		{"`MaxInterval`", p.MaxInterval.String(), "backoff CEILING — no single wait exceeds it", "waits stop growing; attempts continue to `MaxAttempts`"},
		{"`MaxElapsed`", p.MaxElapsed.String(), "wall-clock cap on all waiting for one item", "land blocked (`wall-clock-cap`) WITHOUT waiting; must stay under `Config.StaleClaim` or `Run` refuses to start"},
		{"`Jitter`", fmt.Sprintf("%g", p.Jitter), "+/- randomisation of each computed backoff", "—"},
		{"`RetryAfterMaxAttempts`", fmt.Sprintf("%d", p.RetryAfterMaxAttempts), "attempts for a `retry-after` failure specifically", "land blocked; deskkit's own text says sleep the full wait and attempt ONCE"},
	}
	var b strings.Builder
	b.WriteString("| Bound | Default | What it caps | At the bound |\n")
	b.WriteString("|-------|---------|--------------|--------------|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", r.field, r.value, r.caps, r.atBound)
	}
	return b.String()
}

// RenderUnclassifiedList renders UnclassifiedShapes() as the markdown list the docs carry.
func RenderUnclassifiedList() string {
	var b strings.Builder
	for _, s := range UnclassifiedShapes() {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	return b.String()
}
