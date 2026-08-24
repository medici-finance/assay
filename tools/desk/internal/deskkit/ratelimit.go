package deskkit

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// RateLimitPerPRPerHour is the outward-write budget per tool per PR/issue
	// per rolling hour. It is also the cap on the repo's UNNUMBERED bucket
	// (pr=0), where deskevidence's Evidence commits land — so this number is the
	// verify-desk's Evidence-drain ceiling as well as the per-PR cap.
	//
	// 10 was the original value. It was the busiest-hour figure measured from the
	// ledger (#1255's extract — the busiest rolling hour on record landed only a
	// minority of its attempts as real posts), carried forward from the retired
	// RateLimitPerHour as the blast-radius cap: one runaway agent's write ceiling
	// on a single target.
	//
	// Raised to 20 on 2026-08-14 to accelerate the verification-backlog drain.
	// Empirical basis, measured this date from ~/.config/assay/audit.jsonl: the
	// deskevidence drain is BURSTY, not steady — it saturates the 10/hr bucket in
	// seconds and then idles. 19 landed writes across a 3.7h span fell in only 6
	// distinct minutes of actual writing; one observed burst hit exactly 10 and
	// was then followed by 205 minutes of silence. Only ONE `ratelimited` refusal
	// appears in 24h, and that undercounts true demand rather than bounding it:
	// the skill PAUSES to avoid the cap instead of attempting into it, so the
	// refusal it would otherwise record never happens.
	//
	// So 20 is a deliberate throughput LOOSENING for a supervised drain — 2× the
	// measured peak — NOT a new measurement of busiest-hour demand. Stated as
	// plainly as RateLimitPerRepoPerHour below states that it is "not measured":
	// the busiest-hour figure on record remains 10; 20 buys drain headroom above
	// it and nothing here re-measures the peak.
	//
	// Loop-safety is unaffected by this number. The circuit breaker (BreakerTrip
	// consecutive non-progress attempts — an independent meter) still bounds a
	// runaway loop regardless of the budget, and RateLimitPerRepoPerHour = 100 is
	// unchanged, so the cross-PR aggregate ceiling is untouched.
	RateLimitPerPRPerHour = 20

	// RateLimitPerRepoPerHour is the outward-write budget per tool per repo
	// per rolling hour. It caps aggregate writes across all PRs/issues in one
	// repo, so a fleet of agents on many PRs cannot collectively overwhelm a
	// single repo.
	//
	// UNLIKE THE PER-PR CAP, THIS NUMBER IS NOT MEASURED — stating that plainly
	// because the two constants sit side by side and read as equally grounded, and
	// they are not. The per-PR cap above is anchored on an observed busiest-hour
	// figure (10); 100 here is a derived ceiling: 5× the current per-PR cap of 20,
	// i.e. "five PRs simultaneously at their full individual budget", which is
	// well above any fan-out the desk has run.
	// It is deliberately loose. This tier exists to bound a tool-level runaway that
	// spreads ACROSS PRs — the shape the per-PR tier cannot see — not to shape
	// normal throughput, so an over-generous value still closes the hole while a
	// tight one would refuse legitimate work with no incident to justify it.
	//
	// What would settle it: charged deskpost/deskreply writes per repo per rolling
	// hour at the busiest observed fan-out, read off audit.jsonl the way #1255's
	// extract produced the per-PR number. Until someone runs that extract, treat
	// this as an upper bound with an argument, not a measurement — and note that
	// lowering it is the safe direction to be wrong in, raising it is not.
	RateLimitPerRepoPerHour = 100

	rateWindow = time.Hour

	// deskevidenceUnnumberedCap is the override cap below (kept as a named constant so the
	// number and its justification sit together). See unnumberedBucketCap.
	deskevidenceUnnumberedCap = 30

	// BreakerTrip is how many CONSECUTIVE non-progress attempts (see nonProgress) open
	// the circuit breaker. This is the "a refusal loop MUST trip the limit" rule, moved
	// off the write budget and onto its own meter. 5 is above any plausible legitimate
	// run — a reviewer reworks a refused body once or twice — and well under the
	// body-check refusal burst that once emptied the budget within seconds.
	BreakerTrip = 5
	// BreakerCooldown is how long the breaker stays open, measured from the most recent
	// non-progress attempt. Short enough that a human-paced retry is not punished, long
	// enough that a spinning agent is throttled to 4/hour instead of hundreds.
	BreakerCooldown = 15 * time.Minute

	// BreakerBackstopTrip is the tool-wide consecutive-non-progress trip. The primary
	// breaker is scoped per-target (#447: oversized-body refusals concentrated on ONE
	// PR halted every desk write on other repos), which opens a gap: a refusal storm
	// spread thin across many targets never forms a per-target run. This backstop
	// closes that gap.
	//
	// 20 = 4× the per-target trip. NOT MEASURED as a design input (same honesty as
	// the per-repo budget constant above), and BE PRECISE about what this meter
	// counts: it walks ALL of the tool's entries, so its runs INCLUDE single-target
	// runs — it is per-tool global, not cross-target-only. Observed per-tool runs have
	// come close to this trip in practice — a long unbroken run of refusals plus noops
	// in a single tool's unnumbered bucket has approached it, so the margin against
	// this trip can be as little as ONE EVENT, not "well under": one more refusal makes
	// 20, after which that tool's writes to ANY repo stop until progress or cooldown —
	// by design (the per-target breaker throttles that bucket identically), but an
	// editor lowering this constant should know the meter can already be touching it.
	// Lowering re-approaches the machine-global starvation #447 describes; raising
	// weakens the last fleet-wide stop — neither direction is free.
	BreakerBackstopTrip = 4 * BreakerTrip
)

// unnumberedBucketCap overrides RateLimitPerPRPerHour for a specific tool's UNNUMBERED
// (pr=0) bucket — the repo bucket where writes carrying no PR number land (deskevidence's
// Evidence/status commits, deskrelease's cuts). The default (RateLimitPerPRPerHour, 20) was
// measured from deskpost COMMENT spam (#1255), a different verb class from an Evidence/status
// commit: deskevidence charges TWO writes per flip (Evidence + README), so 20 paces the
// whole verify-desk at ten flips an hour, which is the binding constraint on the verification
// drain.
//
// 30 is a throughput-derived CEILING, not a re-measurement of peak demand (same honesty as
// the per-repo constant above). It lifts deskevidence's pacing wall without EXEMPTING it —
// the crucial #439 property: an unscoped exemption fails OPEN, so this stays a scoped, still-
// bounded cap. The circuit breaker, the per-repo tier (100), and the fail-closed unnumbered-
// bucket accounting all still apply, so a runaway deskevidence loop is stopped exactly as
// before; only the steady-state throughput ceiling moved. Lowering it is the safe direction;
// raising it needs the kind of argument the cap constants above require of themselves.
var unnumberedBucketCap = map[string]int{
	"deskevidence": deskevidenceUnnumberedCap,
}

// unnumberedCapFor returns the cap for tool's UNNUMBERED (pr=0) bucket: its override if it
// has one, else RateLimitPerPRPerHour. It is the SINGLE reader of unnumberedBucketCap, so the
// gate and any test cannot disagree about a tool's effective cap.
func unnumberedCapFor(tool string) int {
	if c, ok := unnumberedBucketCap[tool]; ok {
		return c
	}
	return RateLimitPerPRPerHour
}

// UnnumberedCapFor is the exported reader of a tool's UNNUMBERED (pr=0) effective cap. It
// exists so a cross-package pin — e.g. deskevidence's last-write serialisation test, which
// must seed the ledger to the tool's EFFECTIVE cap, not the base RateLimitPerPRPerHour — can
// derive that cap from the same single source the gate uses (unnumberedCapFor), and so can
// never drift from an override the gate honours.
func UnnumberedCapFor(tool string) int {
	return unnumberedCapFor(tool)
}

// Verb classes and the rate limit. The rolling-hour budget below governs
// OUTWARD-WRITE verbs only — those that hit GitHub or another remote (deskpr's push +
// `gh pr create`, deskpost's review/comment/ready). LOCAL-ONLY verbs — ones whose only
// side effect is on this machine's filesystem or git worktree state and that make no
// outward call (deskwt's `add`/`remove`, i.e. `git worktree add`/`remove` + prune) —
// are NOT rate-limited and MUST NOT call AllowWrite: the budget exists to cap runaway
// remote amplification (the PR-flood class), which a local-only verb cannot
// cause. Local-only verbs still take the full audit line and the kill switch;
// they just skip this one gate.

// TWO METERS, NOT ONE (#209).
//
// The original design ran one counter over every audit line "regardless of result — a
// refusal loop MUST trip the limit". That conflated two controls measured in
// different units, and the conflation was self-sustaining:
//
//	AllowWrite refuses → the caller audits result="ratelimited" → the next AllowWrite
//	counts that line → refuses again.
//
// Every retry appended a line inside the rolling window, so the window's trailing edge
// advanced as fast as callers retried and the count could never decay below the limit.
// In one observed window every deskpost entry in the trailing hour was `ratelimited` and
// none were writes. Completed, head-verified verdicts could not be published for roughly
// an hour, and some were eventually posted through the raw `gh` path under the wrong
// identity — carrying no gate authority. The same starvation had already happened from a
// different trigger (#1255): a burst of body-check refusals, none of which made a network
// call, emptied the whole budget.
//
// So the two controls are separated here:
//
//   - THE WRITE BUDGET (RateLimitPerPRPerHour per PR, RateLimitPerRepoPerHour per repo,
//     each per rolling hour) caps remote amplification. Only attempts that may have
//     REACHED the remote charge it — see chargesBudget.
//   - THE CIRCUIT BREAKER (BreakerTrip consecutive non-progress attempts → open for
//     BreakerCooldown) stops a spinning caller. See nonProgress.
//
// The refusal-loop intent survives in full: a refusal loop is still stopped, and stopped harder
// and sooner than a shared counter stopped it (5 attempts, not 10). What no longer
// happens is a refusal loop stopping everyone ELSE, permanently.
//
// Both meters are IMMUNE TO THEIR OWN OUTPUT, which is the property the old design
// lacked and the one to preserve in any future change: a `ratelimited` line is ignored
// by both, so retrying appends only invisible lines, both clocks run down monotonically,
// and a backing-off caller is guaranteed admission. Adding a result class that this
// function's own refusals can produce, and then counting it, re-creates the livelock.
//
// A third class is invisible to both for a different reason: ResultDryRun (#214). A
// rehearsal writes nothing, so it charges no budget; and it is IGNORED by the breaker
// rather than counted as progress, so it can neither open the breaker nor reset one.
// Note the asymmetry with the gates themselves: a dry run is still SUBJECT to both
// meters (runOutward calls AllowWrite before it knows the verb's shape), it just does
// not FEED them. That is the fail-closed direction — a rehearsal is free, not privileged.

// chargesBudget reports whether an audit result consumes outward-write budget.
//
// The rule is "may have reached the remote", not "succeeded", and the difference is
// load-bearing:
//
//   - ResultOK charges: the write landed.
//   - ResultUnverifiable charges: the call was SENT and its outcome could not be
//     confirmed — a timeout, or a GitHub 5xx returned after the comment actually
//     posted. Not charging it is FAIL-OPEN: an agent retrying against a flaky API
//     lands real duplicate writes while every meter reads zero. Rare in practice but
//     argued from consequence, not frequency.
//   - ResultUnwritten does NOT charge (#448): a precondition could not be
//     positively verified, but the failure happened BEFORE any outward write was
//     attempted — a GET that 403'd or errored, a trust-gate read, a local CI/diff
//     determination that came back pending or short. This is the class
//     ResultUnverifiable used to absorb: measured on deskpost's live audit log, nearly
//     all `unverifiable` lines were exactly this shape (failed GETs and local
//     pending/empty CI determinations) and only a small remainder were a genuinely
//     ambiguous POST. The
//     two must be billed differently because they answer different questions — "did we
//     maybe reach the remote" (charge) versus "did we ever try" (we provably did not,
//     so don't) — and conflating them is what let a repo's read errors exhaust the same
//     budget its writes draw from. Do NOT fold this back into ResultUnverifiable's
//     charging rule: that direction is fail-open on the genuinely ambiguous POST case,
//     which is the one the budget exists to cover.
//   - ResultRefused does NOT charge: a body-check, guard or head-pin refusal is a
//     LOCAL reject. The rationale is capping remote amplification, and there was no
//     amplification to cap. The breaker below is what stops a refusal loop.
//   - ResultNoop does NOT charge: idempotency short-circuited and no outward write left
//     the machine. Charging it means re-running one completed command ten times — zero
//     remote writes — exhausts the budget, which is the same self-starvation.
//   - ResultRateLimited and ResultDisabled do NOT charge: they are this limiter's and
//     the kill switch's own output. Neither reached the network, and counting
//     ResultRateLimited is precisely the livelock. (Counting ResultDisabled also made
//     the sanctioned kill-switch drill — running the binaries under
//     DESK_TOOLS_DISABLED=1 to prove Guard fires — spend outward-write budget.)
//   - ResultDryRun does NOT charge (#214): the invocation stopped before the write by
//     construction. Rehearsing an act is not performing it, and a rehearsal that spent
//     the real act's budget would punish exactly the caution the flag exists to allow.
//
// An UNRECOGNISED result charges, failing closed: a new result class must be
// classified deliberately here, and until it is, the safe default is to count it.
func chargesBudget(result string) bool {
	switch result {
	case ResultRefused, ResultNoop, ResultRateLimited, ResultDisabled, ResultDryRun, ResultUnwritten:
		return false
	default:
		return true // ResultOK, ResultUnverifiable, and anything unclassified
	}
}

// nonProgress reports whether an audit result represents an attempt that RAN and changed
// nothing at the remote — the fuel of a spinning caller, and what the circuit breaker
// counts.
//
// ResultRefused and ResultNoop qualify. So does ResultUnwritten (#448): a
// precondition that could not be verified before any write was attempted is, to a caller
// looping on the same input, indistinguishable in shape from a refusal — nothing reached
// the remote, and it must count toward the breaker or a repeated failed-precondition loop
// would be bounded by NEITHER meter (chargesBudget already excludes it from the budget).
// ResultRateLimited, ResultDisabled and
// ResultDryRun deliberately do NOT: the first two are the stops' own output, and counting
// them would let the breaker hold itself open forever off its own refusals — the exact
// livelock this file exists to remove, re-entered through the second meter. ResultOK and
// ResultUnverifiable are progress and RESET the breaker — a genuinely ambiguous POST may
// have landed, so it must not be treated as though it definitely did not.
func nonProgress(result string) bool {
	return result == ResultRefused || result == ResultNoop || result == ResultUnwritten
}

// breakerIgnores reports whether a result is invisible to the breaker's consecutive-run
// walk — neither counted toward the trip NOR treated as the progress that resets it.
//
// This is checked BEFORE nonProgress in checkBreaker, so it is the single guard keeping
// the breaker off its own output — nonProgress's exclusion of the same results is
// belt-and-braces and is not independently reachable. Mutation-checked: dropping
// ResultRateLimited here (and admitting it in nonProgress) reproduces the livelock in the
// second meter — a 60-attempt retry storm pushes a 15m cooldown back out to 14m50s
// remaining instead of counting down to 5m.
//
// ResultDryRun is here rather than in nonProgress for #214: five rehearsals of a release
// must not open a 15-minute breaker against the real one. INVISIBLE is the whole
// requirement, and it is the fail-CLOSED half that is easy to miss — treating a dry run
// as progress instead would let a spinning caller interleave `--dry-run` between its
// refusals and reset the breaker's run to zero every time, disarming the loop stop with
// the very flag that was supposed to be free. Ignored, it can neither trip the breaker
// nor rescue a caller from it.
func breakerIgnores(result string) bool {
	return result == ResultRateLimited || result == ResultDisabled || result == ResultDryRun
}

// AllowWrite enforces the outward-write gate as of now. See AllowWriteAt.
// repo is the "owner/name" repo string; pr is the issue/PR number.
//
// NO CALL SITE CAN OPT OUT OF A BUDGET (#439 review). An earlier draft
// of the two-tier design SKIPPED the per-PR tier when pr was 0 and skipped BOTH tiers
// when repo was empty, "for tools without PR context". That is fail-open, and it fired:
// deskevidence and deskpr's create path pass pr=0 legitimately (a PR number does not
// exist before the PR does), and the skip silently moved them from the base cap of 10/hr
// to the per-repo 100/hr — a 10× loosening on live write paths, one of them the exact
// verb behind the PR-flood risk. deskrelease passed an empty repo and so
// skipped both tiers, leaving release writes bounded only by the breaker, which counts
// CONSECUTIVE non-progress and therefore never trips on a run of successful releases:
// unbounded.
//
// So a missing scope now narrows the budget instead of removing it:
//
//   - pr == 0 with a repo → the repo's UNNUMBERED bucket, capped at
//     RateLimitPerPRPerHour. Writes that carry no PR number share one bucket per repo,
//     which is the same blast-radius cap the base single tier enforced for them.
//   - repo == "" → a TOOL-WIDE budget at RateLimitPerPRPerHour, which is exactly the
//     retired RateLimitPerHour behaviour. A call site with no repo context is a call
//     site whose blast radius cannot be reasoned about, so it gets the tightest scope,
//     not the loosest.
//
// The direction is the fail-closed one: an unclassified scope fails CLOSED. Widening a specific
// call site is then a deliberate edit with an argument attached, which is what the two
// cap constants above already require of themselves.
//
// THE BUCKET A GATE READS MUST BE THE BUCKET ITS WRITES LAND IN (#439, third
// review). Narrowing a scope is not enough on its own — a gate can be aimed at a bucket
// the call site never fills, which reads like a tight cap and enforces nothing. That is
// how `deskpr create` stayed at 100/hr through a fix that was supposed to hold it at 10:
// it gated on the repo's unnumbered bucket while auditing each create with the REAL number
// of the PR it had just made, so no successful create could ever land in the bucket its own
// gate counted. The meter was also inverted — only FAILED creates (which record no number)
// accumulated, so ten failures locked the tool out for an hour while ninety-nine successes
// did not move it. Use AllowWriteRepoWide for a call site whose writes carry a number it
// cannot know in advance; see that function.
func AllowWrite(tool, repo string, pr int) error {
	return AllowWriteAt(tool, repo, pr, time.Now())
}

// AllowWriteRepoWide gates a call site whose writes CANNOT be attributed to a bucket the
// gate could name in advance, and holds it at RateLimitPerPRPerHour over every charged
// write the tool made on `repo` in the window.
//
// `deskpr create` is the motivating case and currently the only caller. A create's audit
// line records the number of the PR it just created — a different, previously unseen number
// every time — so there is no per-PR bucket that can accumulate creates and no unnumbered
// bucket they land in either. Every scope AllowWrite can express is therefore empty for
// this call site by construction, and the only bucket its writes reliably fall into is
// "this tool, this repo". Counting that at the PER-PR cap keeps PR creation bounded by
// that cap, as it was before the tiers existed, on the verb behind the PR-flood risk,
// and — unlike recording the PR as nil to force the buckets to line up — it leaves the
// created number in the audit trail, which is the one field that makes a create traceable.
//
// It is deliberately STRICTER than the per-repo tier it subsumes (20 vs 100), so callers
// cannot reach for it as a way to buy headroom.
func AllowWriteRepoWide(tool, repo string) error {
	return AllowWriteRepoWideAt(tool, repo, time.Now())
}

// VerdictIssueTool is the audit `tool` name a verdict-issue filing records under — and so
// the NAME OF ITS RATE BUCKET. The verdict-by-issue lane (R-6) flushes one signed batch
// issue every ~5 minutes; that CADENCE is the throttle, so the old self-imposed daily cap —
// deskfile's 3 `new` issues per repo per rolling 24h (deskfile.go) — deliberately does NOT
// bind this lane. It also structurally cannot: that budget keys on tool=="deskfile", and a
// verdict filing is a DISTINCT tool, so filing under this name is exactly what keeps the
// lane off the daily cap while still recording every filing on the audit line. What remains
// is that audit line plus the rolling-hour meters below as a runaway backstop; GitHub's own
// hard limits are the final backstop.
const VerdictIssueTool = "verifyloop-verdict"

// AllowVerdictIssueWrite gates one verdict-issue filing on `repo`. Each filing creates a
// FRESH issue whose number the filer cannot know in advance — the same shape as `deskpr
// create` — so it is metered REPO-WIDE (see AllowWriteRepoWide): every charged verdict write
// on the repo in the rolling hour counts against RateLimitPerPRPerHour, which bounds a
// runaway filer. There is deliberately NO 24h/daily window here — see VerdictIssueTool.
func AllowVerdictIssueWrite(repo string) error {
	return AllowWriteRepoWideAt(VerdictIssueTool, repo, time.Now())
}

// AllowVerdictIssueWriteAt is AllowVerdictIssueWrite with an injectable clock (test seam).
func AllowVerdictIssueWriteAt(repo string, now time.Time) error {
	return AllowWriteRepoWideAt(VerdictIssueTool, repo, now)
}

// AllowWriteRepoWideAt is AllowWriteRepoWide with an injectable clock.
func AllowWriteRepoWideAt(tool, repo string, now time.Time) error {
	mine, err := pointsFor(tool)
	if err != nil {
		return err
	}
	if repo == "" {
		// Same fail-closed fallback as AllowWriteAt: no repo, no reasoning about blast
		// radius, so the tightest scope applies.
		if err := checkToolBudget(tool, now, mine); err != nil {
			return err
		}
		return checkBreaker(tool, "", 0, now, mine)
	}
	if err := checkRepoWideBudget(tool, repo, now, mine); err != nil {
		return err
	}
	// The per-repo tier is strictly looser than the one above and so can never be the
	// binding constraint here. It runs anyway: the tiers are conjunctive by design, and an
	// exception carved for "this one is dominated" is exactly the reasoning that produced
	// the skips this file spent two reviews removing.
	if err := checkRepoBudget(tool, repo, now, mine); err != nil {
		return err
	}
	if err := checkBreakerRepo(tool, repo, now, mine); err != nil {
		return err
	}
	return checkBreakerBackstop(tool, now, mine)
}

// AllowWriteAt is AllowWrite with an injectable clock — the testable core. It applies
// the meters described above to `tool`'s audit history, scoped by `repo` and `pr`,
// and returns:
//   - RateLimited (exit 4) when a per-PR, per-repo, or circuit-breaker limit is hit.
//     Carries a RetryAfter (RetryAfterOf) stating the exact free-at instant;
//   - Unverifiable (exit 6) when the audit file is unreadable, a line is malformed, or a
//     timestamp is unparseable — never a silent "assume under budget";
//   - nil when one more write is within all budgets and the breaker is closed.
func AllowWriteAt(tool, repo string, pr int, now time.Time) error {
	mine, err := pointsFor(tool)
	if err != nil {
		return err
	}

	// No branch here leaves a caller ungated — see AllowWrite's contract. An empty repo
	// falls back to the tool-wide budget (the retired single tier); a zero PR is a
	// narrower scope within the repo, not an exemption from one.
	if repo == "" {
		if err := checkToolBudget(tool, now, mine); err != nil {
			return err
		}
	} else {
		if err := checkPRBudget(tool, repo, pr, now, mine); err != nil {
			return err
		}
		if err := checkRepoBudget(tool, repo, now, mine); err != nil {
			return err
		}
	}
	if err := checkBreaker(tool, repo, pr, now, mine); err != nil {
		return err
	}
	return checkBreakerBackstop(tool, now, mine)
}

// pointsFor loads `tool`'s audit lines, reduced to what the meters need and ordered
// oldest-first. Shared by every AllowWrite* entry point so they cannot disagree about
// which lines the meters see or about the fail-closed treatment of an unparseable timestamp
// (Unverifiable — never a silent "assume under budget").
func pointsFor(tool string) ([]auditPoint, error) {
	entries, err := LoadEntries()
	if err != nil {
		return nil, err // already an Unverifiable *DeskError
	}
	var mine []auditPoint
	for _, e := range entries {
		if e.Tool != tool {
			continue
		}
		ts, perr := time.Parse(time.RFC3339, e.TS)
		if perr != nil {
			return nil, Unverifiable(
				fmt.Sprintf("audit entry for %q has an unparseable ts %q — move file aside to audit.jsonl.corrupt-<ts>", tool, e.TS),
				perr)
		}
		mine = append(mine, auditPoint{ts: ts, result: e.Result, repo: e.Repo, pr: e.PR})
	}
	// Stable so entries sharing a whole-second RFC3339 timestamp keep append order —
	// the breaker's "consecutive" walk depends on it.
	sort.SliceStable(mine, func(i, j int) bool { return mine[i].ts.Before(mine[j].ts) })
	return mine, nil
}

// auditPoint is one of `tool`'s audit lines reduced to what the meters need.
type auditPoint struct {
	ts     time.Time
	result string
	repo   string
	pr     *int
}

// chargedInWindow counts `tool`'s charged writes inside the rolling window that match
// `scope`, and — when the count has reached `limit` — returns the instant the budget
// frees and the rounded wait to advertise.
//
// Extracted so every tier computes the window, the free-at instant and the retry-after
// from ONE implementation. The three tiers previously carried byte-identical copies of
// this arithmetic, which is a divergence risk on the one calculation a caller sleeps on:
// a fix applied to one copy and missed on another produces a tier that advertises a
// deadline the caller wakes up short of, and the "attempt ONCE" instruction turns into a
// retry loop. `over` is reported separately from the count so a caller under budget can
// still see its own usage.
func chargedInWindow(now time.Time, mine []auditPoint, limit int, scope func(auditPoint) bool) (count int, over bool, freeAt time.Time, retryAfter time.Duration) {
	cutoff := now.Add(-rateWindow)
	var charged []time.Time
	for _, e := range mine {
		if e.ts.Before(cutoff) || !chargesBudget(e.result) {
			continue
		}
		if !scope(e) {
			continue
		}
		charged = append(charged, e.ts)
	}
	count = len(charged)
	if count < limit {
		return count, false, time.Time{}, 0
	}
	// The limit-th write back from the end is the one whose expiry frees a slot; +1s so a
	// caller waking exactly on the boundary is certainly past it.
	freeAt = charged[count-limit].Add(rateWindow).Add(time.Second)
	return count, true, freeAt, roundUpToSecond(freeAt.Sub(now))
}

// budgetRefusal renders the shared refusal text for a tier. `target` names the scope that
// is exhausted and `shared` says who else draws on it — the two halves that differ between
// tiers; every other word, including the DO-NOT-retry-loop instruction, is deliberately
// identical across them so a caller reads the same protocol whichever tier refuses.
func budgetRefusal(tool, tier, target string, count, limit int, freeAt time.Time, retryAfter time.Duration, shared string) error {
	return RateLimitedAfter(fmt.Sprintf(
		"refused: %s hit its %s write budget (%d charged writes on %s in the last hour; max %d) — "+
			"retry-after: %ds (free at %s). DO NOT retry-loop: sleep the full retry-after and "+
			"attempt ONCE, hand the artifact back to the desk, or run the raw command manually. %s",
		tool, tier, count, target, limit,
		int(retryAfter/time.Second), freeAt.UTC().Format(time.RFC3339), shared),
		retryAfter)
}

// checkPRBudget applies the rolling-hour per-PR write budget.
//
// pr == 0 is the repo's UNNUMBERED bucket rather than a skip (see AllowWrite): it matches
// this repo's entries that recorded no PR number, so deskevidence, deskpr's create path
// and deskrelease are held to RateLimitPerPRPerHour instead of falling through to the 5×
// wider per-repo tier. Entries are matched on `pr == nil || *pr == 0` because the two
// encodings mean the same thing and different tools write different ones — deskevidence
// and deskrelease omit the field (nil), deskpost always writes a number and so records 0.
// Matching only one of them would leave the bucket half-blind, which is the same
// fail-open shape one level down.
func checkPRBudget(tool, repo string, pr int, now time.Time, mine []auditPoint) error {
	// The unnumbered (pr=0) bucket may carry a per-tool override (unnumberedBucketCap); a
	// numbered PR always uses the base cap. Either way the tier stays the per-PR tier, well
	// under the per-repo 100 — an override RAISES a specific tool's unnumbered ceiling, it
	// never lets the write fall through to a wider tier (the #439 fail-open shape).
	limit := RateLimitPerPRPerHour
	if pr == 0 {
		limit = unnumberedCapFor(tool)
	}
	count, over, freeAt, retryAfter := chargedInWindow(now, mine, limit, func(e auditPoint) bool {
		if e.repo != repo {
			return false
		}
		if pr == 0 {
			return e.pr == nil || *e.pr == 0
		}
		return e.pr != nil && *e.pr == pr
	})
	if !over {
		return nil
	}
	target := fmt.Sprintf("%s#%d", repo, pr)
	shared := "Other writers on this PR may share this budget."
	if pr == 0 {
		target = repo + " (writes carrying no PR/issue number)"
		shared = "Every write on this repo that carries no PR number shares this one bucket."
	}
	return budgetRefusal(tool, "per-PR", target, count, limit, freeAt, retryAfter, shared)
}

// checkRepoBudget applies the rolling-hour per-repo write budget.
func checkRepoBudget(tool, repo string, now time.Time, mine []auditPoint) error {
	count, over, freeAt, retryAfter := chargedInWindow(now, mine, RateLimitPerRepoPerHour, func(e auditPoint) bool {
		return e.repo == repo
	})
	if !over {
		return nil
	}
	return budgetRefusal(tool, "per-repo", repo, count, RateLimitPerRepoPerHour, freeAt, retryAfter,
		"Other writers on this repo may share this budget.")
}

// checkRepoWideBudget counts EVERY charged write the tool made on `repo` in the window —
// numbered, unnumbered, all of it — against RateLimitPerPRPerHour.
//
// It is the tier for a call site whose writes carry a PR number it cannot know in advance
// (see AllowWriteRepoWide). The defining property, and the one the tests pin, is that this
// scope is a SUPERSET of every bucket the call site's writes could land in: whatever number
// a create records, that line is on this repo and is therefore counted. No write the gate
// admits can escape the bucket the gate reads, which is precisely what the per-PR and
// unnumbered scopes could not promise for this call site.
func checkRepoWideBudget(tool, repo string, now time.Time, mine []auditPoint) error {
	count, over, freeAt, retryAfter := chargedInWindow(now, mine, RateLimitPerPRPerHour, func(e auditPoint) bool {
		return e.repo == repo
	})
	if !over {
		return nil
	}
	return budgetRefusal(tool, "repo-wide", repo+" (all writes by this tool, numbered or not)",
		count, RateLimitPerPRPerHour, freeAt, retryAfter,
		"This verb creates the target it writes to, so it is held at the per-PR cap across the whole repo.")
}

// checkToolBudget is the fallback tier for a call site with NO repo context. It counts
// every charged write by the tool in the window against RateLimitPerPRPerHour — which is
// precisely the retired single-tier RateLimitPerHour behaviour, so a scope-less caller is
// held to the base cap rather than escaping the meter (see AllowWrite).
func checkToolBudget(tool string, now time.Time, mine []auditPoint) error {
	count, over, freeAt, retryAfter := chargedInWindow(now, mine, RateLimitPerPRPerHour, func(auditPoint) bool { return true })
	if !over {
		return nil
	}
	return budgetRefusal(tool, "tool-wide", tool+" (no repo scope)", count, RateLimitPerPRPerHour, freeAt, retryAfter,
		"This call site passes no repo, so it draws on one budget for the whole tool.")
}

// breakerRun walks `mine` newest-first over the entries `scope` admits and returns the
// trailing consecutive-non-progress run and the timestamp of its newest member. Shared
// by the per-target breaker and the tool-wide backstop so they cannot disagree about
// what "consecutive" means; breakerIgnores stays the single guard keeping both meters
// off the stops' own output.
func breakerRun(mine []auditPoint, scope func(auditPoint) bool) (run int, last time.Time, members []auditPoint) {
	for i := len(mine) - 1; i >= 0; i-- {
		e := mine[i]
		if !scope(e) || breakerIgnores(e.result) {
			continue // out-of-scope and the stops' own output are invisible — see nonProgress
		}
		if !nonProgress(e.result) {
			break // progress resets the run
		}
		if run == 0 {
			last = e.ts
		}
		run++
		members = append(members, e)
	}
	return run, last, members
}

// breakerOpen renders the shared open-breaker refusal. `target` names the tripped scope
// and `advice` is the half that differs between the per-target breaker (whose reader's
// own input IS the problem) and the backstop (whose reader is usually blameless — #447's
// misdirected-advice finding).
func breakerOpen(tool string, run, trip int, last, now time.Time, target, advice string) error {
	freeAt := last.Add(BreakerCooldown)
	if !now.Before(freeAt) {
		// Cooled down: let exactly one attempt through. If it also comes back
		// non-progress it becomes the new `last` and the breaker re-opens for another
		// BreakerCooldown, so a genuinely spinning caller is throttled to one attempt
		// per cooldown rather than stopped forever.
		return nil
	}
	retryAfter := roundUpToSecond(freeAt.Sub(now))
	return RateLimitedAfter(fmt.Sprintf(
		"refused: %s has made %d consecutive attempts that changed nothing (refused/noop) on %s — "+
			"circuit breaker open (trips at %d) — retry-after: %ds (free at %s). %s "+
			"Retrying does not extend this wait, but it does not shorten it either.",
		tool, run, target, trip,
		int(retryAfter/time.Second), freeAt.UTC().Format(time.RFC3339), advice),
		retryAfter)
}

// checkBreaker applies the consecutive-non-progress circuit breaker, scoped to the
// (repo, pr) bucket the write targets — the SAME bucket as checkPRBudget, for the same
// reason #439 scoped the budget: a refusal loop is one caller spinning on one target,
// and its stop must not starve every other target on the machine (#447). An empty repo falls back to the
// tool-wide walk — the fail-closed narrowing AllowWrite already applies to scope-less
// callers everywhere else.
//
// The per-target walk opens one gap the global walk covered: a storm spread thin across
// targets. checkBreakerBackstop below closes it; AllowWrite* call sites run BOTH, and
// the tiers are conjunctive like every other meter in this file.
func checkBreaker(tool, repo string, pr int, now time.Time, mine []auditPoint) error {
	scope := func(auditPoint) bool { return true }
	target := tool + " (no repo scope)"
	if repo != "" {
		if pr == 0 {
			scope = func(e auditPoint) bool {
				return e.repo == repo && (e.pr == nil || *e.pr == 0)
			}
			target = repo + " (writes carrying no PR/issue number)"
		} else {
			scope = func(e auditPoint) bool {
				return e.repo == repo && e.pr != nil && *e.pr == pr
			}
			target = fmt.Sprintf("%s#%d", repo, pr)
		}
	}
	run, last, _ := breakerRun(mine, scope)
	if run < BreakerTrip {
		return nil
	}
	return breakerOpen(tool, run, BreakerTrip, last, now, target,
		"This is a LOOP, not a queue: fix the input (the refusal reason is in the audit "+
			"detail) before attempting again, or hand the artifact back to the desk.")
}

// checkBreakerRepo is checkBreaker's scope for a repo-wide call site (AllowWriteRepoWide):
// the write cannot name a PR bucket in advance, so its breaker run is the whole repo.
func checkBreakerRepo(tool, repo string, now time.Time, mine []auditPoint) error {
	run, last, _ := breakerRun(mine, func(e auditPoint) bool { return e.repo == repo })
	if run < BreakerTrip {
		return nil
	}
	return breakerOpen(tool, run, BreakerTrip, last, now, repo+" (all writes by this tool)",
		"This is a LOOP, not a queue: fix the input (the refusal reason is in the audit "+
			"detail) before attempting again, or hand the artifact back to the desk.")
}

// checkBreakerBackstop is the tool-wide consecutive-non-progress stop. It exists for the
// storm the per-target breaker cannot see: non-progress attempts spread across many
// targets, none forming a per-target run. Its refusal NAMES the run's members — the
// blocked caller's own input is usually fine, so telling it to "fix the input" would be
// the misdirected advice #447 documented; attribution is the only actionable thing this
// message can offer.
func checkBreakerBackstop(tool string, now time.Time, mine []auditPoint) error {
	run, last, members := breakerRun(mine, func(auditPoint) bool { return true })
	if run < BreakerBackstopTrip {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, e := range members {
		n := e.repo
		if n == "" {
			n = "(no repo)"
		}
		if e.pr != nil {
			n = fmt.Sprintf("%s#%d", n, *e.pr)
		}
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return breakerOpen(tool, run, BreakerBackstopTrip, last, now,
		fmt.Sprintf("%d targets across the watched set (%s)", len(names), strings.Join(names, ", ")),
		"This is a fleet-wide refusal storm, not necessarily YOUR loop: the run's members "+
			"are named above so the stall can be attributed — check the audit detail for "+
			"those targets before assuming your own input is at fault.")
}

// roundUpToSecond renders a wait as whole seconds, never rounding DOWN and never
// returning zero. Rounding down would advertise a deadline the caller wakes up just
// short of — one refusal, one more audit line, one more round. Callers sleep this value
// exactly once.
func roundUpToSecond(d time.Duration) time.Duration {
	if d <= 0 {
		return time.Second
	}
	if d%time.Second != 0 {
		d += time.Second - d%time.Second
	}
	return d
}
