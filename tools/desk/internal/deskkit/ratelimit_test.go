package deskkit

import (
	"strings"
	"testing"
	"time"
)

const (
	testRepo = "medici-finance/assay"
	testPR   = 431
)

var testPRPtr = &[]int{testPR}[0]

// TestAllowWriteUnderBudget — a handful of recent writes stays under the limit.
func TestAllowWriteUnderBudget(t *testing.T) {
	dir := setup(t)
	for i := 0; i < 5; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "comment", Result: ResultOK})
	}
	if err := AllowWrite("deskpost", testRepo, testPR); err != nil {
		t.Fatalf("AllowWrite under budget = %v, want nil", err)
	}
}

// TestAllowWriteOverBudgetRefuses — the over-budget negative: 30 attempts in the past hour,
// 20 of them successful writes → AllowWrite REFUSES with the rate-limit exit code. The
// interleaved refusals no longer charge the budget (#209) — the 20 `ok`
// entries alone are twice the cap, so this test asserts the budget meter, not the mix.
func TestAllowWriteOverBudgetRefuses(t *testing.T) {
	dir := setup(t)
	for i := 0; i < 30; i++ {
		result := ResultOK
		if i%3 == 0 {
			result = ResultRefused
		}
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "comment", Result: result})
	}
	err := AllowWrite("deskpost", testRepo, testPR)
	if !IsRateLimited(err) {
		t.Fatalf("AllowWrite over budget = %v, want RateLimited (exit 4)", err)
	}
	if ExitCodeOf(err) != ExitRateLimited {
		t.Fatalf("ExitCodeOf = %d, want %d", ExitCodeOf(err), ExitRateLimited)
	}
}

// Entries older than the rolling hour do not count; a different tool's entries do not
// count against this tool.
//
// Driven from an INJECTED clock (AllowWriteAt) rather than wall time, matching the rest of
// the file. The stale entries sit just past the window edge — 61 minutes, not the two hours
// an earlier draft used. Two hours proves the same thing with an hour of slack, which means
// it would keep passing if the window arithmetic were off by up to an hour; anchored to the
// injected `now`, the margin is one minute and the test actually measures the edge.
func TestAllowWriteWindowAndGrouping(t *testing.T) {
	dir := setup(t)
	now := time.Now()
	old := now.Add(-rateWindow - time.Minute).UTC().Format(time.RFC3339)
	for i := 0; i < 20; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "comment", Result: ResultOK, TS: old})
	}
	for i := 0; i < 20; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpr", Verb: "pr-create", Result: ResultOK})
	}
	if err := AllowWriteAt("deskpost", testRepo, testPR, now); err != nil {
		t.Fatalf("AllowWriteAt = %v; old entries and other tools must not count", err)
	}
	// The same entries INSIDE the window must refuse — otherwise this test would pass on a
	// limiter that ignored the seeded lines entirely, for any reason.
	insideNow := now.Add(-rateWindow - time.Minute).Add(30 * time.Minute)
	if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, insideNow)); got != "pr-budget" {
		t.Fatalf("with the same entries inside the window the meter fired %s, want pr-budget — "+
			"the window edge, not blindness to the seed, must be what admits the write above", got)
	}
}

// A malformed audit line makes the rate-limit lookup Unverifiable — never a silent
// "assume under budget".
func TestAllowWriteCorruptLineUnverifiable(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "comment", Result: ResultOK})
	appendLine(t, dir, `{"ts":"broken`)
	if err := AllowWrite("deskpost", testRepo, testPR); !IsUnverifiable(err) {
		t.Fatalf("AllowWrite on corrupt line = %v, want Unverifiable (exit 6)", err)
	}
}

// A well-formed JSON line with an unparseable ts is also Unverifiable (the ts is a
// corrupt field for this lookup).
func TestAllowWriteBadTimestampUnverifiable(t *testing.T) {
	dir := setup(t)
	appendLine(t, dir, `{"ts":"not-a-timestamp","tool":"deskpost","result":"ok"}`)
	if err := AllowWrite("deskpost", testRepo, testPR); !IsUnverifiable(err) {
		t.Fatalf("AllowWrite with bad ts = %v, want Unverifiable", err)
	}
}

// --- #209: two meters, and neither may feed itself ---

// meterOf reports which of the two gates produced a refusal, so a test can assert the
// RIGHT one fired. Both return exit 4, so IsRateLimited alone cannot tell them apart and
// a test written against it would pass while the meter under test did nothing.
func meterOf(err error) string {
	switch {
	case err == nil:
		return "none"
	case !IsRateLimited(err):
		return "other:" + err.Error()
	case strings.Contains(err.Error(), "per-PR write budget"):
		return "pr-budget"
	case strings.Contains(err.Error(), "per-repo write budget"):
		return "repo-budget"
	case strings.Contains(err.Error(), "repo-wide write budget"):
		return "repo-wide"
	case strings.Contains(err.Error(), "tool-wide write budget"):
		return "tool-budget"
	case strings.Contains(err.Error(), "circuit breaker"):
		return "breaker"
	}
	return "unknown:" + err.Error()
}

// TestChargingAndBreakerClassByResult pins BOTH classifications for every audit result
// class in one table. Filling the window with a single result class makes the two meters
// separable: for a charging class the budget refuses; for a non-progress class the
// breaker refuses; for the stops' own output NEITHER fires however many lines pile up.
func TestChargingAndBreakerClassByResult(t *testing.T) {
	cases := []struct {
		name      string
		result    string
		n         int
		wantMeter string
	}{
		// Charged: the write reached, or may have reached, the remote.
		{"ok charges the per-PR budget", ResultOK, RateLimitPerPRPerHour, "pr-budget"},
		// unverifiable = sent, outcome unconfirmed. Not charging it is FAIL-OPEN: a
		// retry against a flaky API lands real duplicates while the meter reads zero.
		{"unverifiable charges the per-PR budget", ResultUnverifiable, RateLimitPerPRPerHour, "pr-budget"},
		// Not charged, but non-progress: the refusal loop, moved to the breaker.
		{"refused does not charge, it trips the breaker", ResultRefused, RateLimitPerPRPerHour, "breaker"},
		{"noop does not charge, it trips the breaker", ResultNoop, RateLimitPerPRPerHour, "breaker"},
		// #448: a precondition that could not be verified BEFORE any write
		// was attempted (a failed GET, a local CI/diff determination) must not be billed
		// as if it may have reached the remote -- it provably did not. Same treatment as
		// refused/noop: no budget charge, but it still trips the breaker so a repeated
		// failed-precondition loop is bounded by something.
		{"unwritten does not charge, it trips the breaker", ResultUnwritten, RateLimitPerPRPerHour, "breaker"},
		// The stops' own output: invisible to both meters, or the meters feed themselves.
		{"ratelimited feeds neither meter", ResultRateLimited, RateLimitPerPRPerHour, "none"},
		{"a 500-line ratelimited retry storm feeds neither meter", ResultRateLimited, 500, "none"},
		{"disabled feeds neither meter (the kill-switch drill is free)", ResultDisabled, RateLimitPerPRPerHour, "none"},
		// #214: a rehearsal writes nothing, so rehearsing must be free on BOTH
		// meters. The counts are deliberately past each meter's own trip point.
		{"dryrun feeds neither meter", ResultDryRun, RateLimitPerPRPerHour, "none"},
		{"a long rehearsal run feeds neither meter", ResultDryRun, BreakerTrip * 10, "none"},
		// Fail closed: an unclassified result must charge, so a future result class
		// cannot silently escape the budget by not being listed.
		{"an unrecognised result charges (fail closed)", "someNewResultClass", RateLimitPerPRPerHour, "pr-budget"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := setup(t)
			for i := 0; i < c.n; i++ {
				appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: c.result})
			}
			if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != c.wantMeter {
				t.Fatalf("after %d %q entries the %s meter fired, want %s", c.n, c.result, got, c.wantMeter)
			}
		})
	}
}

// TestRefusalsDoNotConsumeTheWriteBudget is the narrow assertion at the heart of #209,
// isolated from the breaker so it cannot pass for the wrong reason: a handful of
// body-check refusals must leave the write budget untouched.
//
// A burst of body-check refusals — not one of which made a network call — once emptied
// the whole hourly budget and stranded a completed verification artifact. Under the old
// "count everything" rule the 4 refusals plus 9 writes below are
// 13 attempts against a cap of 10 and this call refuses. It must now be admitted: only
// the 9 writes are chargeable, and 9 < 10.
func TestRefusalsDoNotConsumeTheWriteBudget(t *testing.T) {
	dir := setup(t)
	// Below BreakerTrip, so the breaker is provably not what is being measured here.
	for i := 0; i < BreakerTrip-1; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRefused})
	}
	for i := 0; i < RateLimitPerPRPerHour-1; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "none" {
		t.Fatalf("AllowWrite after %d local refusals + %d writes = %s, want none — a "+
			"refusal that never reached the network must not spend outward-write budget (#209)",
			BreakerTrip-1, RateLimitPerPRPerHour-1, got)
	}
	// One more real write DOES reach the cap: the budget still binds on actual writes.
	appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK})
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "pr-budget" {
		t.Fatalf("AllowWrite after %d writes = %s, want budget — the cap must still bind", RateLimitPerPRPerHour, got)
	}
}

// TestUnwrittenPreconditionFailuresDoNotConsumeTheWriteBudget is #448's core
// assertion, isolated from the breaker exactly as TestRefusalsDoNotConsumeTheWriteBudget
// isolates #209's: a run of PRE-WRITE precondition failures — a CI rollup read that 403'd,
// a check still pending, an empty rollup on a CI-required repo — must leave the write
// budget untouched, while genuine writes still bind it.
//
// This is deliberately NOT the same claim as "ResultUnverifiable never charges" — that
// direction is explicitly rejected in the issue as fail-open on a genuinely ambiguous
// POST. What must hold is narrower and is pinned by the second half of this test: a write
// that WAS sent and came back ambiguous (ResultUnverifiable) still charges, right up
// against the same cap, even while a pile of ResultUnwritten lines sits in the same
// window costing nothing.
func TestUnwrittenPreconditionFailuresDoNotConsumeTheWriteBudget(t *testing.T) {
	dir := setup(t)
	// Below BreakerTrip, so the breaker is provably not what is being measured here.
	for i := 0; i < BreakerTrip-1; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "ready", Result: ResultUnwritten})
	}
	for i := 0; i < RateLimitPerPRPerHour-1; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "ready", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "none" {
		t.Fatalf("AllowWrite after %d unwritten precondition failures + %d writes = %s, want none — "+
			"a precondition check that never reached the network must not spend outward-write budget (#448)",
			BreakerTrip-1, RateLimitPerPRPerHour-1, got)
	}
	// One more genuinely AMBIGUOUS write (sent, outcome unconfirmed) DOES reach the cap:
	// the split must not have widened into a blanket exemption for "unverifiable".
	appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "ready", Result: ResultUnverifiable})
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "pr-budget" {
		t.Fatalf("AllowWrite after a genuinely ambiguous unverifiable write = %s, want budget — "+
			"a call that may have reached the remote must still bind the cap", got)
	}
}

// TestBudgetLivelockRecovery is the #209 regression on meter 1: a backing-off caller
// must eventually be admitted. It replays the incident — the budget is spent by real
// posts, then review agents retry-loop and each retry appends a `ratelimited` audit line.
// Under the old rule those lines counted, so every retry pushed the rolling window's
// trailing edge forward and there was NO state in which a retrying caller could succeed.
// The middle assertion is the livelock proof: time served must count down 1:1.
func TestBudgetLivelockRecovery(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 7, 24, 22, 0, 0, 0, time.UTC)
	ts := func(d time.Duration) string { return base.Add(d).UTC().Format(time.RFC3339) }

	// 22:00 — the desk spends its whole budget on real, successful posts.
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK,
			TS: ts(time.Duration(i) * time.Second)})
	}

	// 22:30 — a reviewer attempts and is refused. It must be told WHEN to retry; a bare
	// exit 4 with no retry-after is what turned four agents into a livelock.
	at := base.Add(30 * time.Minute)
	err := AllowWriteAt("deskpost", testRepo, testPR, at)
	if got := meterOf(err); got != "pr-budget" {
		t.Fatalf("AllowWriteAt with a spent budget fired %s, want budget", got)
	}
	// The oldest charged entry is at 22:00:00 and leaves the (inclusive) window at
	// 23:00:01, so the budget frees 30m01s out. The extra second is the boundary epsilon.
	if want := 30*time.Minute + time.Second; RetryAfterOf(err) != want {
		t.Fatalf("RetryAfterOf = %v, want %v (oldest charged entry + 1h - now)", RetryAfterOf(err), want)
	}
	if !strings.Contains(err.Error(), "retry-after") {
		t.Fatalf("rate-limit message %q does not state a retry-after", err.Error())
	}

	// 22:30–22:50 — four agents retry-loop; every attempt appends a `ratelimited` line.
	// This is the exact input that used to make the window self-perpetuating.
	for i := 0; i < 40; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRateLimited,
			TS: ts(30*time.Minute + time.Duration(i*30)*time.Second)})
	}

	// 22:50 — still limited (the budget is genuinely spent), but the retry storm must NOT
	// have pushed the deadline out.
	err2 := AllowWriteAt("deskpost", testRepo, testPR, base.Add(50*time.Minute))
	if got := meterOf(err2); got != "pr-budget" {
		t.Fatalf("AllowWriteAt at 22:50 fired %s, want budget", got)
	}
	if want := 10*time.Minute + time.Second; RetryAfterOf(err2) != want {
		t.Fatalf("RetryAfterOf after a 40-retry storm = %v, want %v — retries pushed the window (LIVELOCK)",
			RetryAfterOf(err2), want)
	}

	// 23:00:01 — the charged entries have aged out. The caller is admitted. This is the
	// state the old limiter could never reach while any agent retried.
	if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, base.Add(time.Hour+time.Second))); got != "none" {
		t.Fatalf("AllowWriteAt after the window elapsed fired %s, want none — the limiter "+
			"must converge to allowing traffic when callers back off", got)
	}
}

// TestBreakerTripsOnConsecutiveNonProgress — enforcing that a refusal loop MUST trip the limit,
// now enforced by the breaker rather than by the shared write budget. It must trip AT
// BreakerTrip and not before, so a reviewer reworking a refused body once or twice is not
// punished for it.
func TestBreakerTripsOnConsecutiveNonProgress(t *testing.T) {
	for _, result := range []string{ResultRefused, ResultNoop} {
		t.Run(result, func(t *testing.T) {
			dir := setup(t)
			base := time.Date(2026, 7, 30, 12, 31, 0, 0, time.UTC)
			for i := 0; i < BreakerTrip-1; i++ {
				appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: result,
					TS: base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)})
			}
			now := base.Add(time.Duration(BreakerTrip) * time.Second)
			if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, now)); got != "none" {
				t.Fatalf("after %d consecutive %q the %s meter fired, want none — the breaker "+
					"must not trip below BreakerTrip=%d", BreakerTrip-1, result, got, BreakerTrip)
			}
			// The BreakerTrip-th non-progress attempt opens it.
			appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: result,
				TS: now.UTC().Format(time.RFC3339)})
			err := AllowWriteAt("deskpost", testRepo, testPR, now.Add(time.Second))
			if got := meterOf(err); got != "breaker" {
				t.Fatalf("after %d consecutive %q the %s meter fired, want breaker", BreakerTrip, result, got)
			}
			if d := RetryAfterOf(err); d <= 0 || d > BreakerCooldown {
				t.Fatalf("breaker retry-after = %v, want a positive duration <= %v", d, BreakerCooldown)
			}
		})
	}
}

// TestBreakerDoesNotExtendItselfOnRetries is the SECOND livelock proof, and the reason
// the breaker exists as its own meter rather than as a rule bolted onto the counter. A
// breaker that counted its own refusals as non-progress would hold itself open forever —
// the identical failure to the one being fixed, re-entered through the new door.
func TestBreakerDoesNotExtendItselfOnRetries(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 7, 30, 12, 31, 0, 0, time.UTC)
	ts := func(d time.Duration) string { return base.Add(d).UTC().Format(time.RFC3339) }

	// Nine body-check refusals in 96 seconds — the real refusal-storm shape, verbatim.
	for i := 0; i < 9; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRefused,
			TS: ts(time.Duration(i*12) * time.Second)})
	}
	last := 96 * time.Second // the 9th refusal, at +96s
	t1 := base.Add(last + time.Second)
	err := AllowWriteAt("deskpost", testRepo, testPR, t1)
	if got := meterOf(err); got != "breaker" {
		t.Fatalf("after 9 consecutive refusals the %s meter fired, want breaker", got)
	}
	firstWait := RetryAfterOf(err)

	// The caller retry-loops for ten minutes; every attempt appends a `ratelimited` line.
	for i := 0; i < 60; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRateLimited,
			TS: ts(last + time.Duration(i*10)*time.Second)})
	}
	t2 := base.Add(last + 10*time.Minute)
	err2 := AllowWriteAt("deskpost", testRepo, testPR, t2)
	if got := meterOf(err2); got != "breaker" {
		t.Fatalf("ten minutes into a %v cooldown the %s meter fired, want breaker", BreakerCooldown, got)
	}
	// The deadline must count down 1:1 with the wall clock: whatever time elapsed between
	// the two probes must come off the wait, and the 60 retry lines must add nothing.
	if want := firstWait - t2.Sub(t1); RetryAfterOf(err2) != want {
		t.Fatalf("breaker retry-after after a 60-retry storm = %v, want %v — the retries "+
			"pushed the cooldown out (LIVELOCK in the second meter)", RetryAfterOf(err2), want)
	}

	// And it does clear: one attempt is admitted once the cooldown elapses.
	if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, base.Add(last+BreakerCooldown))); got != "none" {
		t.Fatalf("after the %v cooldown elapsed the %s meter fired, want none", BreakerCooldown, got)
	}
}

// TestBreakerResetsOnProgress — a successful (or possibly-successful) write clears the
// consecutive run. Otherwise refusals accumulated across a whole day of legitimate work
// would eventually trip a breaker that is meant to catch a LOOP.
func TestBreakerResetsOnProgress(t *testing.T) {
	for _, progress := range []string{ResultOK, ResultUnverifiable} {
		t.Run(progress, func(t *testing.T) {
			dir := setup(t)
			base := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
			at := func(d time.Duration) string { return base.Add(d).UTC().Format(time.RFC3339) }
			for i := 0; i < BreakerTrip*2; i++ {
				appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRefused,
					TS: at(time.Duration(i) * time.Second)})
			}
			appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: progress,
				TS: at(time.Duration(BreakerTrip*2) * time.Second)})
			if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, base.Add(time.Minute))); got != "none" {
				t.Fatalf("a %q after %d refusals left the %s meter firing, want none — "+
					"progress must reset the breaker", progress, BreakerTrip*2, got)
			}
		})
	}
}

// TestBreakerThrottlesRatherThanStops — a caller that keeps failing is admitted once per
// cooldown, and each failed probe re-opens the breaker. That is the loop stop (a spinning
// agent is held to 4 attempts an hour instead of hundreds) without the permanent denial
// the shared counter produced.
func TestBreakerThrottlesRatherThanStops(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return base.Add(d).UTC().Format(time.RFC3339) }
	for i := 0; i < BreakerTrip; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRefused,
			TS: at(time.Duration(i) * time.Second)})
	}
	last := time.Duration(BreakerTrip-1) * time.Second

	// The probe is admitted exactly at the cooldown boundary, not before.
	if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, base.Add(last+BreakerCooldown-time.Second))); got != "breaker" {
		t.Fatalf("one second before the cooldown elapsed the %s meter fired, want breaker", got)
	}
	if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, base.Add(last+BreakerCooldown))); got != "none" {
		t.Fatalf("at the cooldown boundary the %s meter fired, want none", got)
	}

	// That probe also fails → the breaker re-opens for another full cooldown.
	probe := last + BreakerCooldown
	appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRefused, TS: at(probe)})
	err := AllowWriteAt("deskpost", testRepo, testPR, base.Add(probe+time.Second))
	if got := meterOf(err); got != "breaker" {
		t.Fatalf("after the probe also failed the %s meter fired, want breaker", got)
	}
	if want := BreakerCooldown - time.Second; RetryAfterOf(err) != want {
		t.Fatalf("re-opened breaker retry-after = %v, want %v", RetryAfterOf(err), want)
	}
}

// TestRetryAfterIsSufficientAndUnpadded — waiting exactly the advertised retry-after must
// admit the caller, and no more than necessary. Under-reporting puts the caller straight
// back into a refusal (and another audit line), which is the loop this change ends;
// over-reporting wastes desk throughput.
//
// The FRACTIONAL rows are here because #1261's reviewer found its arithmetic held only
// by an interaction between two constants, never asserted: that version rounded the
// duration to the nearest second, which can round DOWN by up to 500ms, and every test
// asked at a whole-second offset. roundUpToSecond replaces that, and these rows pin it.
func TestRetryAfterIsSufficientAndUnpadded(t *testing.T) {
	base := time.Date(2026, 7, 24, 22, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		spacing time.Duration // gap between the charged writes
		now     time.Duration // offset from base at which we ask
	}{
		{"budget spent in a burst", time.Second, 30 * time.Minute},
		{"budget spread over 50 minutes", 5 * time.Minute, 55 * time.Minute},
		{"asked immediately after the burst", 0, time.Minute},
		{"asked 1ms into a second", time.Second, 30*time.Minute + time.Millisecond},
		{"asked 400ms into a second", time.Second, 30*time.Minute + 400*time.Millisecond},
		{"asked 499ms into a second", time.Second, 30*time.Minute + 499*time.Millisecond},
		{"asked 500ms into a second", time.Second, 30*time.Minute + 500*time.Millisecond},
		{"asked 501ms into a second", time.Second, 30*time.Minute + 501*time.Millisecond},
		{"asked 999ms into a second", time.Second, 30*time.Minute + 999*time.Millisecond},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := setup(t)
			for i := 0; i < RateLimitPerPRPerHour; i++ {
				appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK,
					TS: base.Add(time.Duration(i) * c.spacing).UTC().Format(time.RFC3339)})
			}
			now := base.Add(c.now)
			err := AllowWriteAt("deskpost", testRepo, testPR, now)
			if got := meterOf(err); got != "pr-budget" {
				t.Fatalf("AllowWriteAt fired %s, want budget", got)
			}
			d := RetryAfterOf(err)
			if d <= 0 {
				t.Fatalf("RetryAfterOf = %v, want positive", d)
			}
			if d%time.Second != 0 {
				t.Fatalf("RetryAfterOf = %v, want whole seconds (a caller sleeps this verbatim)", d)
			}
			// Sleeping the advertised duration must actually admit the caller.
			if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, now.Add(d))); got != "none" {
				t.Fatalf("still limited (%s) after waiting the advertised retry-after %v", got, d)
			}
			// And it must not be padded: two seconds earlier must still refuse.
			if got := meterOf(AllowWriteAt("deskpost", testRepo, testPR, now.Add(d-2*time.Second))); got != "pr-budget" {
				t.Fatalf("retry-after %v over-reports — the %s meter says free 2s earlier", d, got)
			}
		})
	}
}

// A tool that has never written is never rate-limited, and RetryAfterOf is 0 for
// non-rate-limit errors (it is only meaningful on exit 4).
func TestRetryAfterOfNonRateLimitErrors(t *testing.T) {
	setup(t)
	if d := RetryAfterOf(nil); d != 0 {
		t.Fatalf("RetryAfterOf(nil) = %v, want 0", d)
	}
	if d := RetryAfterOf(Refused("refused: nope")); d != 0 {
		t.Fatalf("RetryAfterOf(Refused) = %v, want 0", d)
	}
	if d := RetryAfterOf(Unverifiable("bad", nil)); d != 0 {
		t.Fatalf("RetryAfterOf(Unverifiable) = %v, want 0", d)
	}
	if err := AllowWrite("deskpost", testRepo, testPR); err != nil {
		t.Fatalf("AllowWrite with no history = %v, want nil", err)
	}
}

// --- #214: a dry run is side-effect free on both meters ---

// TestDryRunsNeverOpenTheBreaker is the #214 regression. `deskrelease cut --dry-run`
// used to audit `noop`, which the breaker counts as non-progress, so FIVE rehearsals
// opened a 15-minute breaker against the real cut: the act of checking whether a release
// was safe was enough to lock the release out.
//
// The loop below runs well past BreakerTrip and past RateLimitPerPRPerHour, so a failure here
// names which meter regressed rather than just "something refused".
func TestDryRunsNeverOpenTheBreaker(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	for i := 0; i < RateLimitPerPRPerHour*2; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskrelease", Verb: "cut:desk-tools/v0.1.3", Result: ResultDryRun,
			TS: base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)})
	}
	now := base.Add(time.Minute)
	if got := meterOf(AllowWriteAt("deskrelease", testRepo, 0, now)); got != "none" {
		t.Fatalf("after %d consecutive dry runs the %s meter fired, want none — rehearsing "+
			"a release must not consume budget or open a breaker against the real one (#214)",
			RateLimitPerPRPerHour*2, got)
	}
	// …and the real cut that follows is admitted, which is the property that actually
	// broke: the rehearsal must not have spent the act's allowance.
	appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskrelease", Verb: "cut:desk-tools/v0.1.3", Result: ResultOK,
		TS: now.UTC().Format(time.RFC3339)})
	if got := meterOf(AllowWriteAt("deskrelease", testRepo, 0, now.Add(time.Second))); got != "none" {
		t.Fatalf("the real cut after %d dry runs left the %s meter firing, want none",
			RateLimitPerPRPerHour*2, got)
	}
}

// TestDryRunsDoNotRESETTheBreakerEither is the fail-closed half, and the one a naive fix
// gets wrong. Making a dry run "not non-progress" is not enough: if it were treated as
// PROGRESS, checkBreaker's backward walk would stop at it and clear the consecutive run,
// so a spinning caller could interleave `--dry-run` between its refusals and keep the
// loop stop permanently disarmed with the very flag that was made free.
//
// A dry run must be INVISIBLE — it neither trips the breaker nor rescues a caller from
// it. Here BreakerTrip refusals straddle a dry run and the breaker must still be open.
func TestDryRunsDoNotRESETTheBreakerEither(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return base.Add(d).UTC().Format(time.RFC3339) }

	n := 0
	add := func(result string) {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskrelease", Verb: "cut:desk-tools/v0.1.3", Result: result,
			TS: at(time.Duration(n) * time.Second)})
		n++
	}
	// A refusal loop with a rehearsal spliced into the middle of it.
	for i := 0; i < BreakerTrip-1; i++ {
		add(ResultRefused)
	}
	add(ResultDryRun)
	add(ResultRefused)

	// Gate on the SAME (repo, pr) bucket the entries carry — the breaker is per-target
	// since #447; the property pinned here (an ignored result must not reset the run)
	// is scope-independent.
	err := AllowWriteAt("deskrelease", testRepo, testPR, base.Add(time.Duration(n)*time.Second))
	if got := meterOf(err); got != "breaker" {
		t.Fatalf("a dry run spliced into a %d-refusal loop left the %s meter firing, want "+
			"breaker — an ignored result must not RESET the consecutive run, or --dry-run "+
			"becomes a way to hold the loop stop open forever (#214)", BreakerTrip, got)
	}
	// The cooldown is measured from the last NON-PROGRESS attempt, not from the dry run.
	if d := RetryAfterOf(err); d <= 0 || d > BreakerCooldown {
		t.Fatalf("breaker retry-after = %v, want a positive duration <= %v", d, BreakerCooldown)
	}
}

// TestDryRunIsNotIdempotencyDone — the second reason ResultDryRun is its own class. If a
// rehearsal counted as done, `--dry-run` would SUPPRESS the real write it was meant to
// preview: a silent skip, reported as success.
func TestDryRunIsNotIdempotencyDone(t *testing.T) {
	pr := 214
	head := "5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b"
	entries := []Entry{{
		Repo: "medici-finance/assay", Verb: "review:APPROVE",
		PR: &pr, HeadSHA: &head, Result: ResultDryRun,
	}}
	if AlreadyDoneIn(entries, "medici-finance/assay", pr, head, "review:APPROVE") {
		t.Fatal("a dryrun entry counted as already-done — a rehearsal would suppress the real write (#214)")
	}
	// The control: the same entry as a real write DOES count, so the test above is not
	// passing because the lookup is broken.
	entries[0].Result = ResultOK
	if !AlreadyDoneIn(entries, "medici-finance/assay", pr, head, "review:APPROVE") {
		t.Fatal("an ok entry did not count as already-done — the lookup itself is broken")
	}
}

// Both meters stay scoped to their own tool: one tool's refusal loop must not gate
// another tool, and the rolling window still expires.
func TestMetersStayScopedToTheirTool(t *testing.T) {
	dir := setup(t)
	for i := 0; i < BreakerTrip*3; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultRefused})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "breaker" {
		t.Fatalf("deskpost fired %s, want breaker", got)
	}
	if got := meterOf(AllowWrite("deskpr", testRepo, testPR)); got != "none" {
		t.Fatalf("deskpr fired %s, want none — deskpost's loop must not gate deskpr", got)
	}
}

// --- Per-PR and per-repo budget tiers ---

// TestPRBudgetIsolatesPRs: a full per-PR budget of writes on PR #1 does not gate PR #2.
func TestPRBudgetIsolatesPRs(t *testing.T) {
	pr1 := 431
	pr2 := 432
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &pr1, Tool: "deskpost", Verb: "review", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, pr2)); got != "none" {
		t.Fatalf("PR #2 fired %s, want none — per-PR budget must isolate PRs", got)
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, pr1)); got != "pr-budget" {
		t.Fatalf("PR #1 fired %s, want pr-budget — a full per-PR budget of writes must exhaust it", got)
	}
}

// TestRepoBudgetAcrossPRs: writes spread across many PRs trigger the per-repo cap.
func TestRepoBudgetAcrossPRs(t *testing.T) {
	dir := setup(t)
	for pr := 431; pr < 441; pr++ {
		p := pr
		for i := 0; i < 11; i++ {
			appendEntry(t, dir, Entry{Repo: testRepo, PR: &p, Tool: "deskpost", Verb: "review", Result: ResultOK})
		}
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, 500)); got != "repo-budget" {
		t.Fatalf("new PR fired %s, want repo-budget — 110 writes across 10 PRs must trip repo cap", got)
	}
}

// TestEmptyRepoFallsBackToToolBudget: a call site with NO repo context is held to the
// tool-wide budget at the per-PR cap — it does NOT escape the meter.
//
// This test previously asserted the opposite ("must skip per-PR and per-repo tiers") and
// seeded 150 writes to prove the skip. That pinned a bypass as intended behaviour:
// deskrelease was the only caller passing an empty repo, and the skip left it bounded
// solely by the breaker, which counts CONSECUTIVE non-progress and so never trips on a run
// of successful releases. The seed count is deliberately just over the per-PR cap rather
// than over the per-repo one — the point is that the FIRST tier binds.
func TestEmptyRepoFallsBackToToolBudget(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskrelease", Verb: "cut:v1.0", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskrelease", "", 0)); got != "tool-budget" {
		t.Fatalf("empty repo fired %s, want tool-budget — a scope-less call site must still be bounded", got)
	}
}

// TestEmptyRepoUnderToolBudgetIsAllowed is the other half: the fallback tier is a BUDGET,
// not a block. One write under the cap passes.
func TestEmptyRepoUnderToolBudgetIsAllowed(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour-1; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskrelease", Verb: "cut:v1.0", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskrelease", "", 0)); got != "none" {
		t.Fatalf("empty repo under budget fired %s, want none", got)
	}
}

// TestZeroPRUsesUnnumberedBucketNotRepoTier: pr=0 is a NARROWER scope, not an exemption.
//
// Seeding the repo's unnumbered bucket to the per-PR cap must refuse at the per-PR tier —
// well below the 100 the per-repo tier would need. Before #439's review,
// pr=0 skipped the per-PR check outright, so deskevidence and deskpr's create path ran at
// 100/hr instead of 10/hr with nothing announcing the 10× loosening.
func TestZeroPRUsesUnnumberedBucketNotRepoTier(t *testing.T) {
	dir := setup(t)
	// Seed to deskevidence's EFFECTIVE unnumbered cap (its override, 30) — still well below
	// the per-repo 100, so a refusal here proves the per-PR/unnumbered tier bound, not the
	// repo tier.
	cap := unnumberedCapFor("deskevidence")
	for i := 0; i < cap; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, Tool: "deskevidence", Verb: "commit", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskevidence", testRepo, 0)); got != "pr-budget" {
		t.Fatalf("zero pr fired %s, want pr-budget — the unnumbered bucket must bind at the per-PR cap", got)
	}
}

// TestUnnumberedBucketOverridePerTool pins the per-tool unnumbered-bucket override
// (unnumberedBucketCap): deskevidence's pr=0 bucket binds at its higher cap (30), NOT the
// base RateLimitPerPRPerHour (20), while a tool WITHOUT an override still binds at the base
// cap on the same bucket. Both directions matter — the override must lift the ceiling for the
// named tool AND leave every other tool untouched — and it must never fall through to the
// per-repo tier (the #439 fail-open shape).
func TestUnnumberedBucketOverridePerTool(t *testing.T) {
	// deskevidence at the BASE cap is still under its override — no refusal yet.
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, Tool: "deskevidence", Verb: "commit", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskevidence", testRepo, 0)); got != "none" {
		t.Fatalf("deskevidence at the base cap fired %s, want none — its override (%d) is higher than %d",
			got, unnumberedCapFor("deskevidence"), RateLimitPerPRPerHour)
	}
	// At its own override cap it refuses, on the per-PR (unnumbered) tier.
	dir = setup(t)
	for i := 0; i < deskevidenceUnnumberedCap; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, Tool: "deskevidence", Verb: "commit", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskevidence", testRepo, 0)); got != "pr-budget" {
		t.Fatalf("deskevidence at its override cap fired %s, want pr-budget", got)
	}
	// A tool with NO override still binds at the base cap on the same unnumbered bucket.
	if unnumberedCapFor("deskpost") != RateLimitPerPRPerHour {
		t.Fatalf("deskpost should have no override; unnumberedCapFor = %d, want %d",
			unnumberedCapFor("deskpost"), RateLimitPerPRPerHour)
	}
	dir = setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, Tool: "deskpost", Verb: "comment", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, 0)); got != "pr-budget" {
		t.Fatalf("deskpost (no override) at the base cap fired %s, want pr-budget", got)
	}
}

// TestZeroPRBucketIsDisjointFromNumberedPRs: writes on a NUMBERED PR do not fill the
// unnumbered bucket, and vice versa. Without this the "narrower scope" claim above is
// untested — a bucket that silently merged with PR #431's would gate the wrong writer.
func TestZeroPRBucketIsDisjointFromNumberedPRs(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskevidence", Verb: "commit", Result: ResultOK})
	}
	// PR #431's budget is spent; the unnumbered bucket is untouched.
	if got := meterOf(AllowWrite("deskevidence", testRepo, 0)); got != "none" {
		t.Fatalf("unnumbered write fired %s, want none — a numbered PR must not fill the unnumbered bucket", got)
	}
	// And the converse.
	dir2 := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir2, Entry{Repo: testRepo, Tool: "deskevidence", Verb: "commit", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskevidence", testRepo, testPR)); got != "none" {
		t.Fatalf("numbered write fired %s, want none — the unnumbered bucket must not fill PR #%d's", got, testPR)
	}
}

// TestZeroPRBucketMatchesBothNilAndZeroEncodings: tools disagree on how "no PR" is
// recorded — deskevidence and deskrelease omit the field (nil), deskpost always writes a
// number and so records 0. The bucket must count both, or it is half-blind and a tool
// whose encoding it ignores writes unbounded.
func TestZeroPRBucketMatchesBothNilAndZeroEncodings(t *testing.T) {
	zero := 0
	for _, tc := range []struct {
		name string
		pr   *int
	}{
		{"nil-encoding", nil},
		{"zero-encoding", &zero},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setup(t)
			for i := 0; i < RateLimitPerPRPerHour; i++ {
				appendEntry(t, dir, Entry{Repo: testRepo, PR: tc.pr, Tool: "deskpost", Verb: "comment", Result: ResultOK})
			}
			if got := meterOf(AllowWrite("deskpost", testRepo, 0)); got != "pr-budget" {
				t.Fatalf("%s fired %s, want pr-budget", tc.name, got)
			}
		})
	}
}

// TestPerToolScopingPreserved: deskpost's cap does not gate deskpr on the same PR.
func TestPerToolScopingPreserved(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskpr", testRepo, testPR)); got != "none" {
		t.Fatalf("deskpr fired %s, want none — per-tool scoping must isolate deskpr from deskpost", got)
	}
}

// TestOldEntriesDoNotCount: entries outside the rolling window are invisible.
func TestOldEntriesDoNotCount(t *testing.T) {
	dir := setup(t)
	old := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK, TS: old})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "none" {
		t.Fatalf("fired %s, want none — entries outside the rolling hour must not count", got)
	}
}

// TestDifferentRepoDoesNotCount: writes on repo-A do not consume repo-B's budget.
func TestDifferentRepoDoesNotCount(t *testing.T) {
	dir := setup(t)
	otherRepo := "other-org/other-repo"
	for i := 0; i < RateLimitPerRepoPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: otherRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "none" {
		t.Fatalf("test repo fired %s, want none — writes on other repos must not consume this budget", got)
	}
}

// TestCapValuesArePinned hard-codes the two cap numbers.
//
// This closes the ONE mutation that survived the suite (#439 review, finding
// 5): raising RateLimitPerPRPerHour left every other assertion green, because every
// test seeds by reading the same constant the code reads. That is the
// self-referential-constant class — an assertion comparing a value against the value it
// was derived from is green for any value, so the suite measured the mechanism and never
// the policy.
//
// These caps ARE the policy. The per-PR cap was raised from 10 to 20 on 2026-08-14 to
// accelerate the verification-backlog drain (see the constant's doc comment: the drain is
// bursty, saturating the old 10/hr bucket in seconds then idling, so 20 is a supervised
// throughput loosening — 2× the busiest-hour figure of 10 measured in #1255's ledger
// extract — not a re-measurement of peak demand). 100 is the per-repo default. Changing
// either is a governance decision, not a refactor, so it must fail here and be changed
// deliberately with the argument updated in the constant's doc comment alongside.
func TestCapValuesArePinned(t *testing.T) {
	if RateLimitPerPRPerHour != 20 {
		t.Fatalf("RateLimitPerPRPerHour = %d, want 20 — this cap is policy (#1255, raised 2026-08-14); "+
			"changing it needs a throughput argument in the constant's doc comment, not a silent bump",
			RateLimitPerPRPerHour)
	}
	if RateLimitPerRepoPerHour != 100 {
		t.Fatalf("RateLimitPerRepoPerHour = %d, want 100 — this cap is policy; "+
			"changing it needs a throughput argument in the constant's doc comment, not a silent bump",
			RateLimitPerRepoPerHour)
	}
	if RateLimitPerRepoPerHour <= RateLimitPerPRPerHour {
		t.Fatalf("per-repo cap (%d) must exceed the per-PR cap (%d) — otherwise the per-PR tier "+
			"is unreachable and the two-tier design collapses to one",
			RateLimitPerRepoPerHour, RateLimitPerPRPerHour)
	}
}

// TestRefusalAtExactCapNotOneBelow pins the boundary independently of the constants'
// VALUES: cap-1 charged writes must pass and cap must refuse. Paired with
// TestCapValuesArePinned this covers both halves — the number, and the comparison.
func TestRefusalAtExactCapNotOneBelow(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour-1; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "none" {
		t.Fatalf("at cap-1 fired %s, want none — the cap-th write is the first refused one", got)
	}
	appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "review", Result: ResultOK})
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "pr-budget" {
		t.Fatalf("at cap fired %s, want pr-budget", got)
	}
}

// --- gate/audit alignment (#439, third review) -----------------------------

// TestGateReadsTheBucketItsWritesLandIn is the general invariant behind the third review's
// finding 1: FOR EVERY CALL SITE, THE WRITES IT PRODUCES MUST LAND IN THE BUCKET ITS OWN
// GATE READS.
//
// Narrowing a scope is not sufficient on its own. A gate can be aimed at a bucket the call
// site never fills, which reads like a tight cap and enforces nothing — `deskpr create`
// gated on the repo's unnumbered bucket while auditing every success with a fresh PR
// number, so it ran at 100/hr behind a comment claiming 10. Each row below seeds the audit
// shape a call site really writes, then asks that call site's real gate: it must refuse.
//
// A row fails if someone re-aims a gate at a bucket its writes cannot reach, which is the
// mutation that survived the previous pass on two call sites at once.
func TestGateReadsTheBucketItsWritesLandIn(t *testing.T) {
	numbered := func(n int) func(int) *int { return func(int) *int { return &n } }
	fresh := func(i int) *int { n := 6000 + i; return &n } // a new number every write
	none := func(int) *int { return nil }

	for _, tc := range []struct {
		name  string
		shape func(i int) *int // the PR field the call site's audit line carries
		gate  func() error     // the call site's real gate, verbatim
	}{
		{
			// deskpost review/comment/ready: gates on the number it was given, records it.
			name:  "deskpost/numbered-pr",
			shape: numbered(testPR),
			gate:  func() error { return AllowWrite("deskpost", testRepo, testPR) },
		},
		{
			// deskevidence: a branch commit, no PR anywhere — gate and audit agree on nil.
			name:  "deskevidence/unnumbered",
			shape: none,
			gate:  func() error { return AllowWrite("deskevidence", testRepo, 0) },
		},
		{
			// deskrelease: a cut records repoSlug and no PR (writeflow.go finishAudit).
			name:  "deskrelease/unnumbered",
			shape: none,
			gate:  func() error { return AllowWrite("deskrelease", testRepo, 0) },
		},
		{
			// deskpr create: records the number of the PR it just made — unknowable to the
			// gate, so the gate must be repo-wide. THIS is the row that was red.
			name:  "deskpr/create-fresh-number",
			shape: fresh,
			gate:  func() error { return AllowWriteRepoWide("deskpr", testRepo) },
		},
		{
			// deskpr update: resolves a real PR and records the same one.
			name:  "deskpr/update-numbered",
			shape: numbered(42),
			gate:  func() error { return AllowWrite("deskpr", testRepo, 42) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setup(t)
			tool := strings.SplitN(tc.name, "/", 2)[0]
			// Seed to the tool's EFFECTIVE cap so a tool with a raised unnumbered override
			// (deskevidence, 30) is filled to ITS ceiling, not the base 20 — otherwise the
			// gate legitimately admits another write and this test would false-fail.
			cap := unnumberedCapFor(tool)
			for i := 0; i < cap; i++ {
				appendEntry(t, dir, Entry{Repo: testRepo, PR: tc.shape(i), Tool: tool, Verb: "w", Result: ResultOK})
			}
			if err := tc.gate(); err == nil {
				t.Fatalf("%s: %d of this call site's OWN writes in the window and its gate still admits "+
					"another — the gate is reading a bucket these writes do not land in",
					tc.name, cap)
			}
		})
	}
}

// TestRepoWideCountsNumberedAndUnnumberedAlike pins the defining property of the repo-wide
// tier: it is a SUPERSET of every bucket a call site's writes could fall into, so no write
// it admits can escape the bucket it reads. Both encodings, and a mix, must fill it.
func TestRepoWideCountsNumberedAndUnnumberedAlike(t *testing.T) {
	for _, tc := range []struct {
		name  string
		shape func(i int) *int
	}{
		{"all-fresh-numbers", func(i int) *int { n := 6000 + i; return &n }},
		{"all-unnumbered", func(int) *int { return nil }},
		{"mixed", func(i int) *int {
			if i%2 == 0 {
				return nil
			}
			n := 6000 + i
			return &n
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setup(t)
			for i := 0; i < RateLimitPerPRPerHour; i++ {
				appendEntry(t, dir, Entry{Repo: testRepo, PR: tc.shape(i), Tool: "deskpr", Verb: "create", Result: ResultOK})
			}
			if got := meterOf(AllowWriteRepoWide("deskpr", testRepo)); got != "repo-wide" {
				t.Fatalf("%s fired %s, want repo-wide", tc.name, got)
			}
		})
	}
}

// TestRepoWideIsStricterThanPerRepo: the repo-wide tier must bind at the per-PR cap, well
// below the per-repo tier it subsumes. If it ever relaxes to the 100, PR creation silently
// returns to the 10x loosening this whole finding is about.
func TestRepoWideIsStricterThanPerRepo(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		n := 6000 + i
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &n, Tool: "deskpr", Verb: "create", Result: ResultOK})
	}
	if got := meterOf(AllowWriteRepoWide("deskpr", testRepo)); got != "repo-wide" {
		t.Fatalf("fired %s at the per-PR cap, want repo-wide — the repo-wide tier must not "+
			"wait for the per-repo cap of %d", got, RateLimitPerRepoPerHour)
	}
}

// TestRepoWideUnderBudgetIsAllowed — a budget, not a block.
func TestRepoWideUnderBudgetIsAllowed(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour-1; i++ {
		n := 6000 + i
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &n, Tool: "deskpr", Verb: "create", Result: ResultOK})
	}
	if got := meterOf(AllowWriteRepoWide("deskpr", testRepo)); got != "none" {
		t.Fatalf("fired %s at cap-1, want none", got)
	}
}

// TestRepoWideIsRepoScoped: another repo's writes must not consume this repo's budget.
func TestRepoWideIsRepoScoped(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour*2; i++ {
		n := 6000 + i
		appendEntry(t, dir, Entry{Repo: "medici-finance/elsewhere", PR: &n, Tool: "deskpr", Verb: "create", Result: ResultOK})
	}
	if got := meterOf(AllowWriteRepoWide("deskpr", testRepo)); got != "none" {
		t.Fatalf("fired %s, want none — writes on another repo must not consume this budget", got)
	}
}

// TestRepoWideWithNoRepoFallsBackToToolBudget: the same fail-closed fallback AllowWriteAt
// has. A scope-less caller is never ungated, whichever entry point it came through.
func TestRepoWideWithNoRepoFallsBackToToolBudget(t *testing.T) {
	dir := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, Tool: "deskpr", Verb: "create", Result: ResultOK})
	}
	if got := meterOf(AllowWriteRepoWide("deskpr", "")); got != "tool-budget" {
		t.Fatalf("fired %s, want tool-budget", got)
	}
}

// TestBreakerScopedToTarget pins the fix for #447: a
// refusal run on ONE PR opens the breaker for THAT PR only. Under the pre-fix global
// walk, five oversized-body refusals on #392 blocked every deskpost write on every
// repo — the budget beside the breaker had been scoped per-PR by #439 for exactly this
// reason. Observed failing against the global walk before the fix landed.
func TestBreakerScopedToTarget(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 8, 6, 15, 56, 0, 0, time.UTC)
	pr392 := 392
	for i := 0; i < BreakerTrip; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &pr392, Tool: "deskpost", Verb: "review", Result: ResultRefused,
			TS: base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)})
	}
	now := base.Add(time.Minute)
	// The offending target is blocked...
	if got := meterOf(AllowWriteAt("deskpost", testRepo, pr392, now)); got != "breaker" {
		t.Fatalf("offending PR: %s meter fired, want breaker", got)
	}
	// ...and names itself in the refusal so a blocked caller can attribute the stall.
	if err := AllowWriteAt("deskpost", testRepo, pr392, now); err == nil || !strings.Contains(err.Error(), "#392") {
		t.Fatalf("breaker refusal must name the tripped target, got: %v", err)
	}
	// A different PR in the same repo is NOT blocked.
	if got := meterOf(AllowWriteAt("deskpost", testRepo, 371, now)); got != "none" {
		t.Fatalf("sibling PR in same repo: %s meter fired, want none — breaker must be per-target", got)
	}
	// A different repo is NOT blocked.
	if got := meterOf(AllowWriteAt("deskpost", "example-org/example-reconciler", 52, now)); got != "none" {
		t.Fatalf("other repo: %s meter fired, want none — breaker must be per-target", got)
	}
	// The repo's unnumbered bucket is NOT blocked by a numbered PR's run.
	if got := meterOf(AllowWriteAt("deskpost", testRepo, 0, now)); got != "none" {
		t.Fatalf("unnumbered bucket: %s meter fired, want none", got)
	}
}

// TestBreakerCrossTargetRefusalDoesNotExtendCooldown pins the outage-extender: with the
// global walk, a genuine refusal on example-reconciler#52 reset the cooldown clock of a
// breaker tripped by #392 — any
// refusal anywhere, once per cooldown, held production down indefinitely. Per-target,
// each run carries its own clock.
func TestBreakerCrossTargetRefusalDoesNotExtendCooldown(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 8, 6, 16, 0, 0, 0, time.UTC)
	pr392, pr52 := 392, 52
	for i := 0; i < BreakerTrip; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &pr392, Tool: "deskpost", Verb: "review", Result: ResultRefused,
			TS: base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)})
	}
	// Ten minutes later an UNRELATED target takes a single genuine refusal.
	appendEntry(t, dir, Entry{Repo: "example-org/example-reconciler", PR: &pr52, Tool: "deskpost", Verb: "review", Result: ResultRefused,
		TS: base.Add(10 * time.Minute).UTC().Format(time.RFC3339)})
	// #392's cooldown runs from ITS OWN last refusal (base+4s), so at base+16m it has
	// cooled and the probe is admitted; under the global walk the example-reconciler refusal
	// pushed free-at out to base+25m and this fired the breaker.
	if got := meterOf(AllowWriteAt("deskpost", testRepo, pr392, base.Add(16*time.Minute))); got != "none" {
		t.Fatalf("cross-target refusal extended the cooldown: %s meter fired at +16m, want none", got)
	}
	// One refusal does not block the unrelated target either.
	if got := meterOf(AllowWriteAt("deskpost", "example-org/example-reconciler", pr52, base.Add(11*time.Minute))); got != "none" {
		t.Fatalf("single refusal on unrelated target: %s meter fired, want none", got)
	}
}

// TestBreakerBackstopTripsAcrossTargets — scoping the breaker per-target opens a gap the
// global walk covered: a refusal storm spread thin (a couple of non-progress attempts on
// each of many targets) never forms a per-target run. The tool-wide backstop closes it,
// tripping at BreakerBackstopTrip consecutive non-progress attempts across ALL targets,
// and its refusal names the run's members because the blocked caller's own input is
// usually fine (#447's misdirected-advice finding).
func TestBreakerBackstopTripsAcrossTargets(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	// BreakerBackstopTrip refusals, two per target, so no per-target run reaches
	// BreakerTrip.
	n := BreakerBackstopTrip
	for i := 0; i < n; i++ {
		pr := 100 + i/2
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &pr, Tool: "deskpost", Verb: "review", Result: ResultRefused,
			TS: base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)})
	}
	now := base.Add(time.Duration(n) * time.Second)
	err := AllowWriteAt("deskpost", testRepo, 999, now)
	if got := meterOf(err); got != "breaker" {
		t.Fatalf("after %d cross-target refusals the %s meter fired, want breaker (backstop)", n, got)
	}
	if !strings.Contains(err.Error(), "across") || !strings.Contains(err.Error(), "#100") {
		t.Fatalf("backstop refusal must say it is cross-target and name run members, got: %v", err)
	}
	// One fewer stays open.
	dir2 := setup(t)
	for i := 0; i < n-1; i++ {
		pr := 100 + i/2
		appendEntry(t, dir2, Entry{Repo: testRepo, PR: &pr, Tool: "deskpost", Verb: "review", Result: ResultRefused,
			TS: base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)})
	}
	if got := meterOf(AllowWriteAt("deskpost", testRepo, 999, base.Add(time.Duration(n)*time.Second))); got != "none" {
		t.Fatalf("below the backstop trip the %s meter fired, want none", got)
	}
	// Pin the constant's VALUE, not just its floor: the fixtures above derive from
	// BreakerBackstopTrip, so raising it would silently self-adjust every count in
	// this test (#454 review). The margin question is live — observed per-tool runs
	// have come close to this trip (see the constant's comment) — so a raise must
	// arrive as a deliberate edit here, with that comment updated alongside.
	if BreakerBackstopTrip != 20 {
		t.Fatalf("BreakerBackstopTrip = %d, want 20 — update this pin AND the constant's margin comment together", BreakerBackstopTrip)
	}
}

// TestBreakerRepoWideCallSite covers checkBreakerRepo through its only caller
// (AllowWriteRepoWideAt): a refusal run SPREAD ACROSS PRs of one repo must stop the
// repo-wide call site (deskpr create — it cannot name a PR bucket, so its breaker is
// the whole repo), while another repo stays untouched (#454 review).
func TestBreakerRepoWideCallSite(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < BreakerTrip; i++ {
		pr := 200 + i // five refusals, five DIFFERENT PRs — no per-target run forms
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &pr, Tool: "deskpr", Verb: "create", Result: ResultRefused,
			TS: base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)})
	}
	now := base.Add(time.Minute)
	if got := meterOf(AllowWriteRepoWideAt("deskpr", testRepo, now)); got != "breaker" {
		t.Fatalf("repo-wide caller after a %d-refusal cross-PR run on the repo: %s meter fired, want breaker", BreakerTrip, got)
	}
	// The repo scope is load-bearing: a different repo's repo-wide caller is untouched.
	if got := meterOf(AllowWriteRepoWideAt("deskpr", "example-org/example-reconciler", now)); got != "none" {
		t.Fatalf("other repo's repo-wide caller: %s meter fired, want none", got)
	}
}

// TestBreakerOutOfScopeEntriesAreInvisibleNotProgress pins breakerRun's treatment of
// out-of-scope entries: they neither count toward a target's run NOR reset it. This is
// the property that makes per-target scoping work on a busy desk — successes on OTHER
// targets land continuously between one loop's refusals, and if they reset the run the
// per-target breaker would essentially never fire in production (#454 review).
func TestBreakerOutOfScopeEntriesAreInvisibleNotProgress(t *testing.T) {
	dir := setup(t)
	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	pr392 := 392
	n := 0
	at := func() string { s := base.Add(time.Duration(n) * time.Second).UTC().Format(time.RFC3339); n++; return s }
	for i := 0; i < BreakerTrip; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &pr392, Tool: "deskpost", Verb: "review", Result: ResultRefused, TS: at()})
		// A success on a DIFFERENT target lands between every pair of refusals.
		other := 300 + i
		appendEntry(t, dir, Entry{Repo: testRepo, PR: &other, Tool: "deskpost", Verb: "review", Result: ResultOK, TS: at()})
	}
	now := base.Add(time.Minute)
	if got := meterOf(AllowWriteAt("deskpost", testRepo, pr392, now)); got != "breaker" {
		t.Fatalf("interleaved out-of-scope successes broke the run: %s meter fired for the spinning target, want breaker", got)
	}
	// And the targets that were succeeding stay open.
	if got := meterOf(AllowWriteAt("deskpost", testRepo, 300, now)); got != "none" {
		t.Fatalf("healthy sibling target: %s meter fired, want none", got)
	}
}
