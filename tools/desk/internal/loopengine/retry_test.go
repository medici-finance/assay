package loopengine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The corpus these tests encode is MEASURED, 2026-08-13, from live desk sessions:
//
//   - RETRYABLE: `API Error: 529 Overloaded` killed two worker sessions minutes apart (both
//     had uncommitted work intact and both completed after resume, so resume beat restart);
//     GitHub secondary rate limit 403 at ~16 concurrent ops on one token with core budget
//     remaining; gh/GitHub 5xx.
//   - NOT RETRYABLE, a decision: `deskpr create` rc=5 REFUSED on a writeguard false
//     positive; rc=5 on two bodycheck refusals; a guard-blocked `git push`. Retrying is the
//     bug; reaching for another tool to make the same write is the worse sibling.
//   - NOT RETRYABLE, deterministic: `deskpr create` rc=6 from `origin/remotes/origin/main`
//     (#911), hit by every worktree here every time; `deskclaim release --kind` rc=5, a
//     plain unknown-flag usage error.
//   - RETRY-AFTER, its own class: `deskfile` rc=4, filing budget exhausted until a named
//     wall-clock instant.
//   - COULD-NOT-CHECK: a PR with ZERO CI check-runs, whose App-authored anti-recursion
//     attribution is measurably false. "Wait" vs "escalate" is not derivable from it.

// testPolicy is a deterministic policy: no jitter, so a backoff schedule is an exact
// expectation rather than a range.
func testPolicy() RetryPolicy {
	p := DefaultRetryPolicy()
	p.Jitter = 0
	p.InitialInterval = 10 * time.Millisecond
	p.MaxInterval = 100 * time.Millisecond
	p.MaxElapsed = time.Second
	return p
}

// sleepRecorder captures the backoff schedule instead of waiting it out.
type sleepRecorder struct {
	mu   sync.Mutex
	naps []time.Duration
}

func (s *sleepRecorder) sleep(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.naps = append(s.naps, d)
}

func (s *sleepRecorder) schedule() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.naps))
	copy(out, s.naps)
	return out
}

// failNTimes returns a dispatchFn that fails the first n attempts with err, then succeeds.
func failNTimes(n int, err error) func(*fakeLoop, Item, Tier) (Handle, error) {
	var mu sync.Mutex
	calls := 0
	return func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		mu.Lock()
		calls++
		c := calls
		mu.Unlock()
		if c <= n {
			return nil, err
		}
		h := &fakeHandle{item: it, done: make(chan Result, 1)}
		h.done <- Result{Item: it, Verdict: VerdictPass, RunnerID: "local:test-runner"}
		return h, nil
	}
}

// TestRetryTransientIsRetriedWithBackoff is the retryable half of the corpus: a 529
// Overloaded is transport-class, so the engine holds the claim, backs off, and re-dispatches
// rather than dropping the item. It also pins the RESUME signal: from attempt 2 the Payload
// carries the attempt number, because the two 529-killed sessions both had their work intact
// and restarting from scratch would have burned it.
func TestRetryTransientIsRetriedWithBackoff(t *testing.T) {
	rec := &sleepRecorder{}
	pol := testPolicy()
	cfg := Config{RunnerID: "dispatcher-a", Progress: new(bytes.Buffer), Retry: &pol, Sleep: rec.sleep}

	var seen []map[string]string
	loop := &fakeLoop{name: "retrytest"}
	inner := failNTimes(2, errors.New("gh: API Error: 529 Overloaded"))
	loop.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		seen = append(seen, it.Payload)
		return inner(l, it, tier)
	}

	h, out := dispatchWithRetry(cfg, loop, Item{ID: "loop-engine/08"}, TierLocal)
	if out != nil {
		t.Fatalf("a transient 529 was not retried to success: %+v", out)
	}
	if h == nil {
		t.Fatal("no handle returned for a recovered dispatch")
	}
	if got := len(seen); got != 3 {
		t.Fatalf("dispatch attempts = %d; want 3 (two 529s then success)", got)
	}
	// Exact backoff: 10ms then 20ms (InitialInterval, BackoffCoefficient 2, no jitter).
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	got := rec.schedule()
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("backoff schedule = %v; want %v", got, want)
	}
	// Attempt 1 must be byte-identical to a pre-retry dispatch; attempts 2+ carry the
	// resume markers.
	if seen[0] != nil {
		t.Fatalf("attempt 1 Payload = %v; want the item's own Payload untouched (nil here)", seen[0])
	}
	if seen[1][PayloadRetryAttempt] != "2" || seen[2][PayloadRetryAttempt] != "3" {
		t.Fatalf("retry attempts did not carry the resume markers: %v / %v", seen[1], seen[2])
	}
	if seen[1][PayloadRetryOf] != fmt.Sprintf("%d", pol.MaxAttempts) {
		t.Fatalf("retry marker did not state the bound: %v", seen[1])
	}
}

// TestRetryRefusalIsNeverRetried is the positive control for the non-retryable half, and
// the one a mutation must redden: exit 5 is a deliberate safety stop, so it gets exactly ONE
// attempt. The sub-cases walk the whole refusal set (3 / 5 / 6 / 7).
func TestRetryRefusalIsNeverRetried(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"exit 5 writeguard refusal (deskpr create rc=5 on a false positive)", deskkit.Refused("refused: writeguard hit a PGP fingerprint on a context line")},
		{"exit 5 bodycheck refusal (heading read as a ruling)", deskkit.Refused("refused: bodycheck — heading reads as a ruling attributed to a named human")},
		{"exit 3 kill switch", deskkit.Disabled("refused: DISABLED is armed")},
		{"exit 6 unverifiable", deskkit.Unverifiable("could not verify head", errors.New("gh: 403"))},
		{"exit 7 author==runner", &AuthorIsRunnerError{ItemID: "loop-engine/08", Runner: "dispatcher-a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &sleepRecorder{}
			pol := testPolicy()
			var log bytes.Buffer
			cfg := Config{RunnerID: "dispatcher-a", Progress: &log, Retry: &pol, Sleep: rec.sleep}

			attempts := 0
			loop := &fakeLoop{name: "retrytest"}
			loop.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
				attempts++
				return nil, tc.err
			}

			h, out := dispatchWithRetry(cfg, loop, Item{ID: "loop-engine/08"}, TierLocal)
			if h != nil || out == nil {
				t.Fatalf("a refusal produced a handle instead of a terminal outcome (h=%v out=%v)", h, out)
			}
			if attempts != 1 {
				t.Fatalf("a refusal was dispatched %d times — retrying a deliberate safety stop is the bug this class exists to prevent", attempts)
			}
			if out.Class != ClassRefusal {
				t.Fatalf("class = %q; want %q", out.Class, ClassRefusal)
			}
			if out.Stop != "non-retryable" {
				t.Fatalf("stop reason = %q; want non-retryable", out.Stop)
			}
			if len(rec.schedule()) != 0 {
				t.Fatalf("the engine SLEPT before a refusal it never retried: %v", rec.schedule())
			}
			if out.Verdict() != VerdictBlocked {
				t.Fatalf("verdict = %q; want %q", out.Verdict(), VerdictBlocked)
			}
			if !strings.Contains(log.String(), "NOT retryable") {
				t.Fatalf("the refusal was not stated in the engine's own output; Progress=%q", log.String())
			}
		})
	}

	// A policy CANNOT widen the retryable set. Listing a refusal as retryable is not
	// expressible: NonRetryable only subtracts, and the base set is closed.
	pol := testPolicy()
	pol.NonRetryable = nil
	for _, c := range []FailureClass{ClassRefusal, ClassDeterministic, ClassUnclassified} {
		if pol.Retryable(c) {
			t.Fatalf("class %q became retryable with an empty NonRetryable list — 'retry on 5' must not be expressible in configuration", c)
		}
	}
	// And NonRetryable does narrow the classes that ARE retryable.
	pol.NonRetryable = []FailureClass{ClassTransient}
	if pol.Retryable(ClassTransient) {
		t.Fatal("NonRetryable did not narrow the retryable set")
	}
	if !pol.Retryable(ClassRetryAfter) {
		t.Fatal("NonRetryable narrowed a class it did not name")
	}
}

// TestRetryAfterIsHonoredExactly pins the retry-after class: exit 4 that STATES its wait is
// slept for exactly that long, once — not the backoff schedule, and not a guess. Guessing
// short is the #209 livelock, and re-attempting early re-charges the budget that refused us.
func TestRetryAfterIsHonoredExactly(t *testing.T) {
	rec := &sleepRecorder{}
	pol := testPolicy()
	cfg := Config{RunnerID: "dispatcher-a", Progress: new(bytes.Buffer), Retry: &pol, Sleep: rec.sleep}

	const stated = 250 * time.Millisecond
	loop := &fakeLoop{name: "retrytest"}
	loop.dispatchFn = failNTimes(1, deskkit.RateLimitedAfter("refused: deskfile hit its filing budget", stated))

	h, out := dispatchWithRetry(cfg, loop, Item{ID: "loop-engine/08"}, TierLocal)
	if out != nil || h == nil {
		t.Fatalf("a stated retry-after was not honored to success: out=%+v", out)
	}
	got := rec.schedule()
	if len(got) != 1 || got[0] != stated {
		t.Fatalf("slept %v; want exactly one sleep of %v (the STATED wait, not the backoff schedule of %v)", got, stated, pol.InitialInterval)
	}

	// And the class is capped tighter than MaxAttempts: deskkit's own refusal text says
	// sleep the full retry-after and attempt ONCE.
	rec2 := &sleepRecorder{}
	cfg2 := Config{RunnerID: "dispatcher-a", Progress: new(bytes.Buffer), Retry: &pol, Sleep: rec2.sleep}
	attempts := 0
	loop2 := &fakeLoop{name: "retrytest"}
	loop2.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		attempts++
		return nil, deskkit.RateLimitedAfter("refused: still rate limited", stated)
	}
	_, out2 := dispatchWithRetry(cfg2, loop2, Item{ID: "loop-engine/08"}, TierLocal)
	if out2 == nil {
		t.Fatal("a persistently rate-limited dispatch succeeded")
	}
	if attempts != pol.RetryAfterMaxAttempts {
		t.Fatalf("retry-after attempts = %d; want RetryAfterMaxAttempts = %d (tighter than MaxAttempts %d)", attempts, pol.RetryAfterMaxAttempts, pol.MaxAttempts)
	}

	// A BARE exit 4 with no stated wait is could-not-check, NOT a guessed backoff.
	if c := Classify(deskkit.RateLimited("refused: rate limited")); c.Class != ClassUnclassified {
		t.Fatalf("a bare exit 4 with no retry-after classified as %q; want %q — guessing the wait short is the #209 livelock", c.Class, ClassUnclassified)
	}
}

// TestRetryAfterBeyondWallClockCapIsNotSlept is the deskfile case measured 2026-08-13: the
// filing budget was exhausted until a named instant almost a day out. A retry-after longer
// than the declared wall-clock cap is NOT slept — the engine hands the item back with the
// free-at instant stated, rather than holding a claim for a day.
func TestRetryAfterBeyondWallClockCapIsNotSlept(t *testing.T) {
	rec := &sleepRecorder{}
	pol := testPolicy() // MaxElapsed = 1s
	cfg := Config{RunnerID: "dispatcher-a", Progress: new(bytes.Buffer), Retry: &pol, Sleep: rec.sleep}

	loop := &fakeLoop{name: "retrytest"}
	loop.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		return nil, deskkit.RateLimitedAfter("refused: deskfile filing budget exhausted", 22*time.Hour)
	}

	h, out := dispatchWithRetry(cfg, loop, Item{ID: "loop-engine/08"}, TierLocal)
	if h != nil || out == nil {
		t.Fatal("a 22h retry-after produced a handle")
	}
	if len(rec.schedule()) != 0 {
		t.Fatalf("the engine slept %v against a wall-clock cap of %v", rec.schedule(), pol.MaxElapsed)
	}
	if out.Stop != "wall-clock-cap" {
		t.Fatalf("stop reason = %q; want wall-clock-cap", out.Stop)
	}
	if !strings.Contains(out.Why, "MaxElapsed") || !strings.Contains(out.Why, "free at") {
		t.Fatalf("the cap refusal did not state the bound and the free-at instant: %q", out.Why)
	}
	if out.Verdict() != VerdictBlocked {
		t.Fatalf("verdict = %q; want %q — the class was decided, only the wait was too long", out.Verdict(), VerdictBlocked)
	}
}

// TestRetryUnclassifiedLandsNeedsContextNotBlocked is the three-state invariant at the
// dispatch boundary (docs/three-state-instrument-rule.md). An unrecognised failure must NOT
// default to retryable (an infinite loop) and must NOT default to non-retryable (a silent
// work loss): it is could-not-check, it is landed NEEDS_CONTEXT, and it says so.
func TestRetryUnclassifiedLandsNeedsContextNotBlocked(t *testing.T) {
	rec := &sleepRecorder{}
	pol := testPolicy()
	var log bytes.Buffer
	cfg := Config{RunnerID: "dispatcher-a", Progress: &log, Retry: &pol, Sleep: rec.sleep}

	attempts := 0
	loop := &fakeLoop{name: "retrytest"}
	loop.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		attempts++
		// The 0-CI-check-runs shape: a symptom whose cause could not be established.
		return nil, errors.New("dispatch target shows 0 check-runs; cause not established")
	}

	h, out := dispatchWithRetry(cfg, loop, Item{ID: "loop-engine/08"}, TierLocal)
	if h != nil || out == nil {
		t.Fatal("an unclassified failure produced a handle")
	}
	if attempts != 1 {
		t.Fatalf("an unclassified failure was dispatched %d times — defaulting to retryable is the infinite-loop half of the two-state guess", attempts)
	}
	if out.Class != ClassUnclassified {
		t.Fatalf("class = %q; want %q", out.Class, ClassUnclassified)
	}
	if out.Verdict() != VerdictNeedsContext {
		t.Fatalf("verdict = %q; want %q — landing an undecided failure as BLOCKED is the silent-work-loss half of the two-state guess", out.Verdict(), VerdictNeedsContext)
	}
	if !strings.Contains(out.Why, "could-not-check") {
		t.Fatalf("the outcome did not say could-not-check: %q", out.Why)
	}

	// The blind spots are PUBLISHED, not implicit — no silent caps.
	shapes := UnclassifiedShapes()
	if len(shapes) < 5 {
		t.Fatalf("UnclassifiedShapes lists only %d shapes; the taxonomy must publish every failure shape it does not classify", len(shapes))
	}
	joined := strings.Join(shapes, "\n")
	for _, must := range []string{"exit 4", "check-runs", "loop-engine/09"} {
		if !strings.Contains(joined, must) {
			t.Fatalf("UnclassifiedShapes does not name %q", must)
		}
	}
}

// TestRetryExhaustionLandsBlockedAndDrainContinues runs the whole engine: one item whose
// dispatch always fails transiently, one that dispatches fine. Exhaustion must LAND the
// failed item (file-and-continue) and the drain must keep going — never a wedge.
func TestRetryExhaustionLandsBlockedAndDrainContinues(t *testing.T) {
	// DESK_LOOP must name a loop deskkit's roster RECOGNISES, not an ad-hoc fixture label:
	// once the loop-name roster lands (#977), Guard refuses an unresolvable DESK_LOOP as
	// Unverifiable, because "no STOP file found for a name I do not know" is could-not-check,
	// not clean. "retrydrain" below stays a human-readable fixture label, not a loop identity.
	deskDir := setupDeskHome(t, "verify-desk")
	rec := &sleepRecorder{}
	pol := testPolicy()

	loop := &fakeLoop{name: "retrydrain", remaining: []Item{
		{ID: "always-529"},
		{ID: "healthy"},
	}}
	attempts := map[string]int{}
	var mu sync.Mutex
	loop.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		mu.Lock()
		attempts[it.ID]++
		mu.Unlock()
		if it.ID == "always-529" {
			return nil, Transient("model API overloaded", errors.New("529"))
		}
		h := &fakeHandle{item: it, done: make(chan Result, 1)}
		h.done <- Result{Item: it, Verdict: VerdictPass, RunnerID: "local:test-runner"}
		return h, nil
	}
	loop.landFn = func(l *fakeLoop, r Result) error {
		l.recordLandAndDrain(r)
		return nil
	}

	cfg := Config{
		PoolSize:   2,
		IdlePoll:   3 * time.Millisecond,
		ClaimsDir:  t.TempDir(),
		StaleClaim: time.Hour,
		RunnerID:   "dispatcher-a",
		Progress:   os.Stdout,
		Retry:      &pol,
		Sleep:      rec.sleep,
	}
	if err := runUntil(t, cfg, loop, deskDir, func() bool { return len(loop.landedIDs()) == 2 }); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts["always-529"] != pol.MaxAttempts {
		t.Fatalf("the failing item was dispatched %d times; want MaxAttempts = %d", attempts["always-529"], pol.MaxAttempts)
	}
	if !contains(loop.landedIDs(), "always-529") {
		t.Fatalf("an exhausted item was DROPPED instead of landed blocked; landed=%v", loop.landedIDs())
	}
	if !contains(loop.landedIDs(), "healthy") {
		t.Fatalf("the drain did not continue past the exhausted item; landed=%v", loop.landedIDs())
	}
	var blocked *Result
	for i := range loop.landed {
		if loop.landed[i].Item.ID == "always-529" {
			blocked = &loop.landed[i]
		}
	}
	if blocked.Verdict != VerdictBlocked {
		t.Fatalf("exhausted item landed %q; want %q", blocked.Verdict, VerdictBlocked)
	}
	if len(blocked.Rows) != 1 || !strings.Contains(blocked.Rows[0].Output, "attempts-exhausted") {
		t.Fatalf("the landed Evidence row does not state why retrying stopped: %+v", blocked.Rows)
	}
}

// TestRetryHoldsOneClaimAcrossAttempts is the claim invariant: a retry re-dispatches the
// SAME claim. It must not release-and-reclaim between attempts (that reopens the
// double-dispatch window claim.go's flock dance exists to close), and it must not orphan the
// claim on exhaustion.
func TestRetryHoldsOneClaimAcrossAttempts(t *testing.T) {
	// Registered loop name, not the fixture label — see the note in the exhaustion test above.
	deskDir := setupDeskHome(t, "verify-desk")
	claims := t.TempDir()
	rec := &sleepRecorder{}
	pol := testPolicy()

	it := Item{ID: "loop-engine/08"}
	loop := &fakeLoop{name: "retryclaim", remaining: []Item{it}}

	var mu sync.Mutex
	var claimSeen []string // the claim file's mtime-independent identity, per attempt
	attempts := 0
	loop.dispatchFn = func(l *fakeLoop, i Item, tier Tier) (Handle, error) {
		mu.Lock()
		attempts++
		// Read the claim as it stands DURING this attempt. A release-then-reclaim would
		// show a gap (missing file) or a changed record between attempts.
		b, err := os.ReadFile(claimPath(Config{ClaimsDir: claims}, i))
		if err != nil {
			claimSeen = append(claimSeen, "MISSING: "+err.Error())
		} else {
			claimSeen = append(claimSeen, string(b))
		}
		mu.Unlock()
		return nil, Transient("model API overloaded", errors.New("529"))
	}
	loop.landFn = func(l *fakeLoop, r Result) error {
		l.recordLandAndDrain(r)
		return nil
	}

	cfg := Config{
		PoolSize:   1,
		IdlePoll:   3 * time.Millisecond,
		ClaimsDir:  claims,
		StaleClaim: time.Hour,
		RunnerID:   "dispatcher-a",
		Progress:   new(bytes.Buffer),
		Retry:      &pol,
		Sleep:      rec.sleep,
	}
	if err := runUntil(t, cfg, loop, deskDir, func() bool { return len(loop.landedIDs()) == 1 }); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != pol.MaxAttempts {
		t.Fatalf("attempts = %d; want %d", attempts, pol.MaxAttempts)
	}
	for i, c := range claimSeen {
		if strings.HasPrefix(c, "MISSING") {
			t.Fatalf("attempt %d ran with NO claim held (%s) — a retry that releases its claim reopens the double-dispatch window", i+1, c)
		}
		if c != claimSeen[0] {
			t.Fatalf("the claim record CHANGED between attempts (%q -> %q) — a retry must re-dispatch the SAME claim, never re-acquire", claimSeen[0], c)
		}
	}
	// Exhaustion releases the claim exactly once — no orphan.
	if _, err := os.Stat(claimPath(cfg, it)); !os.IsNotExist(err) {
		t.Fatalf("the claim was ORPHANED after exhaustion (stat err=%v)", err)
	}
}

// TestRetryBoundsAreDeclaredNotSilent is the "no silent caps" rule as a test: every bound
// has a stated non-zero default, an incoherent policy is refused rather than quietly
// behaving like some other policy, and the wall-clock cap is refused if it could let a
// retrying dispatcher's own claim age past Config.StaleClaim and be reclaimed under it.
func TestRetryBoundsAreDeclaredNotSilent(t *testing.T) {
	d := DefaultRetryPolicy()
	if err := d.Validate(); err != nil {
		t.Fatalf("DefaultRetryPolicy is not self-consistent: %v", err)
	}
	for name, v := range map[string]int64{
		"MaxAttempts":           int64(d.MaxAttempts),
		"InitialInterval":       int64(d.InitialInterval),
		"MaxInterval":           int64(d.MaxInterval),
		"MaxElapsed":            int64(d.MaxElapsed),
		"RetryAfterMaxAttempts": int64(d.RetryAfterMaxAttempts),
	} {
		if v <= 0 {
			t.Fatalf("bound %s defaults to %d — an unbounded default is a silent cap", name, v)
		}
	}
	if d.RetryAfterMaxAttempts > d.MaxAttempts {
		t.Fatalf("RetryAfterMaxAttempts (%d) must not exceed MaxAttempts (%d)", d.RetryAfterMaxAttempts, d.MaxAttempts)
	}

	// The backoff CEILING actually binds.
	p := DefaultRetryPolicy()
	p.Jitter = 0
	if got := p.Backoff(20); got != p.MaxInterval {
		t.Fatalf("Backoff(20) = %s; want the MaxInterval ceiling %s", got, p.MaxInterval)
	}

	for _, bad := range []struct {
		name string
		mut  func(*RetryPolicy)
	}{
		{"MaxAttempts 0", func(p *RetryPolicy) { p.MaxAttempts = 0 }},
		{"no backoff ceiling", func(p *RetryPolicy) { p.MaxInterval = 0 }},
		{"no wall-clock cap", func(p *RetryPolicy) { p.MaxElapsed = 0 }},
		{"backoff coefficient below 1", func(p *RetryPolicy) { p.BackoffCoefficient = 0.5 }},
		{"jitter out of range", func(p *RetryPolicy) { p.Jitter = 2 }},
	} {
		p := DefaultRetryPolicy()
		bad.mut(&p)
		if err := p.Validate(); err == nil {
			t.Fatalf("Validate accepted an incoherent policy (%s)", bad.name)
		}
	}

	// Run refuses BOTH shapes of a bad policy before it dispatches anything: an incoherent
	// one, and one whose waiting could outlive the claim it holds.
	// Registered loop name deliberately: these assertions require Run to refuse for the
	// POLICY-bounds reason. Under the loop-name roster (#977) an ad-hoc DESK_LOOP would make
	// Run refuse for an unrelated reason and these checks would pass without testing bounds.
	deskDir := setupDeskHome(t, "verify-desk")

	incoherent := DefaultRetryPolicy()
	incoherent.MaxInterval = 0 // no backoff ceiling
	err := runRefusal(t, deskDir, Config{PoolSize: 1, IdlePoll: time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: time.Hour, Retry: &incoherent})
	if err == nil {
		t.Fatal("Run accepted an incoherent RetryPolicy instead of failing closed — a policy with an unbounded ceiling is a silent cap")
	}
	if !strings.Contains(err.Error(), "MaxInterval") {
		t.Fatalf("the refusal did not name the incoherent bound: %v", err)
	}

	pol := DefaultRetryPolicy()
	pol.MaxElapsed = 30 * time.Minute
	err = runRefusal(t, deskDir, Config{PoolSize: 1, IdlePoll: time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: 10 * time.Minute, Retry: &pol})
	if err == nil {
		t.Fatal("Run accepted MaxElapsed >= StaleClaim — a retrying dispatcher could have its own live claim reclaimed mid-retry")
	}
	if !strings.Contains(err.Error(), "StaleClaim") {
		t.Fatalf("the refusal did not name the interacting bound: %v", err)
	}
}

// runRefusal calls Run expecting it to REFUSE the config and return promptly. Run's normal
// state is an endless idle-poll, so a guard that stops refusing does not fail this test — it
// would WEDGE it. The deadline turns that into a fast, readable failure instead of a suite
// that hangs until the package timeout.
func runRefusal(t *testing.T, deskDir string, cfg Config) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- Run(cfg, &fakeLoop{name: "retrybounds"}) }()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
	}
	// Not refused: Run entered its endless idle-poll. Plant a STOP flag so the drain exits
	// instead of leaking a spinning goroutine into the rest of the suite, then report the
	// non-refusal to the caller.
	if err := os.MkdirAll(deskDir, 0o700); err == nil {
		_ = os.WriteFile(filepath.Join(deskDir, "STOP"), []byte("test stop\n"), 0o600)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run neither refused the config nor honoured a STOP flag")
	}
	_ = os.Remove(filepath.Join(deskDir, "STOP"))
	return nil
}

// TestRetryTaxonomyDocIsDerivedNotDuplicated is the derive-or-diff guard: retry.go's
// Taxonomy() is the ONE declared source, README.md carries a rendered copy between markers,
// and this test diffs them. A class added in code and not regenerated into the doc is a red
// test, not a stale paragraph.
func TestRetryTaxonomyDocIsDerivedNotDuplicated(t *testing.T) {
	raw, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	doc := string(raw)

	for _, blk := range []struct {
		begin, end, want string
	}{
		{"<!-- taxonomy:begin -->", "<!-- taxonomy:end -->", RenderTaxonomyTable()},
		{"<!-- bounds:begin -->", "<!-- bounds:end -->", RenderBoundsTable()},
		{"<!-- unclassified:begin -->", "<!-- unclassified:end -->", RenderUnclassifiedList()},
	} {
		got, err := blockBetween(doc, blk.begin, blk.end)
		if err != nil {
			t.Fatalf("%s: %v", blk.begin, err)
		}
		if got != strings.TrimSpace(blk.want) {
			t.Errorf("README.md block %s is STALE — regenerate it from retry.go (the declared source).\n--- README has ---\n%s\n--- code renders ---\n%s", blk.begin, got, strings.TrimSpace(blk.want))
		}
	}

	// The policy table has to be findable as the single home for the retry rules the desk
	// skills currently carry as prose.
	if !strings.Contains(strings.ToLower(doc), "non-retryable") {
		t.Error("README.md does not document the non-retryable taxonomy")
	}
}

// blockBetween returns the trimmed text between two markers.
func blockBetween(doc, begin, end string) (string, error) {
	i := strings.Index(doc, begin)
	if i < 0 {
		return "", fmt.Errorf("marker %q not found", begin)
	}
	rest := doc[i+len(begin):]
	j := strings.Index(rest, end)
	if j < 0 {
		return "", fmt.Errorf("marker %q not found after %q", end, begin)
	}
	return strings.TrimSpace(rest[:j]), nil
}

// TestRetryClassifyNeverGuesses walks the measured corpus one error at a time and pins the
// class each one lands in — including the ones that land in could-not-check.
func TestRetryClassifyNeverGuesses(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want FailureClass
	}{
		{"529 Overloaded (killed two worker sessions, 2026-08-13)", errors.New("API Error: 529 Overloaded"), ClassTransient},
		{"GitHub secondary rate limit 403", errors.New("HTTP 403: You have exceeded a secondary rate limit"), ClassTransient},
		{"gh 502", errors.New("gh: 502 Bad Gateway"), ClassTransient},
		{"typed transient", Transient("model overloaded", nil), ClassTransient},
		{"exit 4 with a stated wait (deskfile rc=4)", deskkit.RateLimitedAfter("budget exhausted", time.Hour), ClassRetryAfter},
		{"exit 4 with NO stated wait", deskkit.RateLimited("budget exhausted"), ClassUnclassified},
		{"exit 5 writeguard", deskkit.Refused("writeguard"), ClassRefusal},
		{"exit 3 disabled", deskkit.Disabled("kill switch"), ClassRefusal},
		{"exit 6 unverifiable (deskpr create rc=6, #911)", deskkit.Unverifiable("bad ref", errors.New("origin/remotes/origin/main")), ClassRefusal},
		{"exit 7 author==runner", &AuthorIsRunnerError{ItemID: "x", Runner: "y"}, ClassRefusal},
		{"typed deterministic (unknown flag)", Deterministic("deskclaim release: unknown flag --kind", nil), ClassDeterministic},
		{"timeout — loop-engine/09's, not ours", errors.New("context deadline exceeded"), ClassUnclassified},
		{"DNS failure", errors.New("dial tcp: lookup api.github.com: no such host"), ClassUnclassified},
		{"a bare unknown error", errors.New("something went wrong"), ClassUnclassified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.err)
			if got.Class != tc.want {
				t.Fatalf("Classify(%v) = %q; want %q", tc.err, got.Class, tc.want)
			}
			if strings.TrimSpace(got.Why) == "" {
				t.Fatal("the classification named no evidence — a verdict with no reason is not readable by the operator it is for")
			}
			if tc.want == ClassRetryAfter && got.RetryAfter <= 0 {
				t.Fatalf("a retry-after class carried no wait: %+v", got)
			}
			if tc.want != ClassRetryAfter && got.RetryAfter != 0 {
				t.Fatalf("a non-retry-after class carried a wait: %+v", got)
			}
		})
	}
}
