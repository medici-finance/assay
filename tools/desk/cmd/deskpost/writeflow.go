package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// toolName is the fixed audit/rate-limit key for this binary. Using a constant (rather
// than os.Args[0]) keeps AllowWrite's per-tool counting and every audit line aligned to
// the same key regardless of how the binary was invoked.
const toolName = "deskpost"

// postOpts are the cross-verb modifiers every mutating verb accepts. Both are OFF by
// default: the tool's behaviour with no flags is byte-identical to before.
//
//   - dryRun  — run every check, stop immediately before the one mutating call, exit 0,
//     audit `dryrun` (#355). It is a REHEARSAL, never a waiver: no check is
//     skipped, softened, or reordered, and there is no flag that posts an unvalidated
//     body. The verb is the same code path up to the write.
//   - wait    — how long a rate-limit refusal (exit 4) may be waited out in-tool instead
//     of being handed to the caller, who has historically handed it to a raw `gh`
//     command with no --head pin (#197). Zero = the old behaviour: refuse
//     immediately.
type postOpts struct {
	dryRun bool
	wait   time.Duration
}

// writeResult is what a verb's plan returns: the outcome to audit and the exit status.
// err is nil for ok/noop and a *deskkit.DeskError otherwise (its code drives the exit).
type writeResult struct {
	verb    string // discriminated verb for idempotency + audit ("review:security:approve", "comment:<digest>", "ready")
	repo    string
	pr      int
	head    string // "" until known (early refusals)
	bodyDig string // review/comment only
	result  string // deskkit.Result* value
	detail  string
	err     error
}

// --- writeResult constructors ---

func refused(verb, repo string, pr int, head, detail string) writeResult {
	return writeResult{verb: verb, repo: repo, pr: pr, head: head, result: deskkit.ResultRefused, detail: detail, err: deskkit.Refused("refused: " + detail)}
}

func unverifiable(verb, repo string, pr int, head, detail string, cause error) writeResult {
	return writeResult{verb: verb, repo: repo, pr: pr, head: head, result: deskkit.ResultUnverifiable, detail: detail, err: deskkit.Unverifiable(detail, cause)}
}

// unverifiableNoWrite is unverifiable's PRE-WRITE sibling (#448): the check
// ran and could not positively verify a precondition — same exit code (6), same fail-
// closed refusal — but no outward write was ever attempted, because the failure is a
// local determination made from data already read (a CI rollup that is pending, or an
// empty rollup on a CI-required repo, or a diff whose read came back short). Audited as
// deskkit.ResultUnwritten so it does not charge the outward-write budget the way
// deskkit.ResultUnverifiable does. Contrast unverifiable, reserved for a write that WAS
// sent with an outcome that could not be confirmed.
func unverifiableNoWrite(verb, repo string, pr int, head, detail string, cause error) writeResult {
	return writeResult{verb: verb, repo: repo, pr: pr, head: head, result: deskkit.ResultUnwritten, detail: detail, err: deskkit.Unverifiable(detail, cause)}
}

func noop(verb, repo string, pr int, head, detail string) writeResult {
	return writeResult{verb: verb, repo: repo, pr: pr, head: head, result: deskkit.ResultNoop, detail: detail}
}

// dryRun is the --dry-run terminus: every check passed and the verb STOPPED immediately
// before its one mutating call. Exit 0, audited `dryrun` — a class NEITHER meter counts
// (deskkit/ratelimit.go: chargesBudget and breakerIgnores both exclude it), so rehearsing
// a body costs nothing. Distinct from noop, which means "the write was already done".
//
// #355: today the only way to discover that a body will be refused is to
// spend a charged attempt, one offender at a time — the reported ratchet was 49 → 37 → 33
// chars across three charged attempts on ONE body. That pressure is what produced the
// issue's worst outcome: a reviewer obfuscated the PR's own head SHA to get past the
// scanner. Free rehearsal removes the pressure without touching a single check.
func dryRun(verb, repo string, pr int, head, detail string) writeResult {
	return writeResult{verb: verb, repo: repo, pr: pr, head: head, result: deskkit.ResultDryRun, detail: detail}
}

func done(verb, repo string, pr int, head, bodyDig, detail string) writeResult {
	return writeResult{verb: verb, repo: repo, pr: pr, head: head, bodyDig: bodyDig, result: deskkit.ResultOK, detail: detail}
}

// deskDir delegates to the exported canonical resolver deskkit.StateDir() — the state
// directory (~/.config/assay) where the outward-write flock file is placed. Tests redirect
// it (and deskkit's audit log) by setting $HOME.
func deskDir() (string, error) {
	return deskkit.StateDir()
}

// auditLock is the flock held across LoadEntries→remote-call→audit-append,
// so two parallel desk sessions cannot double-post or double-count. A lock held >60s is
// treated as stale → Unverifiable (exit 6).
type auditLock struct{ f *os.File }

func acquireAuditLock() (*auditLock, error) {
	dir, err := deskDir()
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, deskkit.Unverifiable("cannot create desk-tools dir", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot open audit lock", err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		lerr := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if lerr == nil {
			return &auditLock{f: f}, nil
		}
		if lerr != syscall.EWOULDBLOCK {
			f.Close()
			return nil, deskkit.Unverifiable("cannot acquire audit lock", lerr)
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, deskkit.Unverifiable(
				"audit lock held >60s (stale lock?) — another desk write may be stuck; retry, or a human clears it", nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *auditLock) release() {
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}

// runOutward is the single orchestration path for every mutating verb (review/comment/
// ready). Guard() has already run in main. It holds the flock across:
//
//	LoadEntries → AllowWrite → plan(checks + remote call) → audit append
//
// `repo` and `pr` are the budget SCOPE, and they are passed in rather than read back out
// of plan's result on purpose (#439 review). Both are known from argv
// before any network call — every caller computes repo as owner+"/"+name and already has
// the number — so scoping the gate costs nothing and, crucially, requires nothing to have
// run first. An earlier draft ran plan BEFORE the gate to harvest the scope from its
// result; but plan is not a planning step, it is the closure that performs the outward
// write (comment.go's client.postComment, review.go's postReview, ready.go's flip). With
// the budget spent, that ordering POSTED the comment and then refused: the caller got
// exit 4 and "rate-limited" while the comment was already on GitHub, and because
// ResultRateLimited does not charge the budget (deskkit chargesBudget), the write that
// landed was invisible to the meter that exists to count it. The gate must be upstream of
// the network call it bounds; anything that has to run first does not belong in it.
//
// so the rate-limit read, the idempotency read, the remote call, and the audit write are
// one critical section. Every exit path writes exactly one audit line — including
// refusals, which is what lets the gates see a loop at all.
//
// WHICH gate a refusal feeds changed in #209 and the distinction is
// load-bearing: a refusal is audited `refused`, which does NOT charge the outward-write
// budget (nothing reached GitHub, so there was no remote amplification to cap) and DOES
// count toward the non-progress circuit breaker (BreakerTrip consecutive → open for
// BreakerCooldown). So the requirement that "a refusal loop must be stopped" still holds — the breaker
// stops it, sooner than the old shared counter did — but a refusal loop no longer
// exhausts the hourly budget and strands every other writer of this tool.
func runOutward(args []string, opts postOpts, repo string, pr int, plan func(entries []deskkit.Entry, opts postOpts) writeResult) int {
	deadline := timeNow().Add(opts.wait)
	for attempt := 1; ; attempt++ {
		code, err := attemptOutward(args, opts, repo, pr, plan)
		if opts.wait <= 0 || !deskkit.IsRateLimited(err) {
			return code
		}
		// --wait (#197): the budget refusal degrades to WAITING rather than
		// to the documented raw-`gh` fallback, which has no --head flag and so attaches a
		// verdict to whatever the head happens to be when it runs. Under a saturated
		// limiter that fallback is the COMMON path, so the head pin — the whole reason a
		// verdict is verifiable — is off more often than on.
		//
		// Looping here is safe by construction rather than by luck: a `ratelimited` audit
		// line neither charges the budget (deskkit chargesBudget) nor feeds the
		// non-progress breaker (breakerIgnores), so waiting cannot extend its own wait.
		// That is the #209 livelock, and it is closed in the limiter, not
		// here.
		ra := deskkit.RetryAfterOf(err)
		if ra <= 0 {
			// A rate-limit refusal with no retry-after cannot be waited out — waiting
			// would be a guess. Surface it as the plain exit 4 it is.
			return code
		}
		if timeNow().Add(ra).After(deadline) {
			fmt.Fprintf(stderr, "deskpost: rate-limited; the budget frees in %s, which is past the "+
				"--wait budget of %s — not waiting. Re-run with a longer --wait rather than "+
				"falling back to a raw `gh pr review` (it has no --head pin).\n", ra, opts.wait)
			return code
		}
		fmt.Fprintf(stderr, "deskpost: rate-limited (attempt %d); --wait is set, sleeping %s for the "+
			"budget to free rather than dropping to an unpinned fallback\n", attempt, ra)
		sleepFor(ra + time.Second) // +1s so the rolling window has certainly moved
	}
}

// sleepFor / timeNow are the clock seams a --wait test replaces, so the retry loop can be
// exercised end to end without a test that actually sleeps for an hour. They are the ONLY
// clock this file reads, and the limiter is driven from the same one (AllowWriteAt below)
// — a test whose fake sleep advanced the loop's clock but not the limiter's would prove
// the loop retries and prove nothing about whether the retry can ever succeed.
var (
	sleepFor = time.Sleep
	timeNow  = time.Now
)

// attemptOutward is ONE pass of the flow above: it holds the flock across
//
//	LoadEntries → AllowWrite → plan(checks + remote call) → audit append
//
// and returns the exit code plus the error that produced it (nil for ok/noop/dryrun), so
// runOutward can decide whether a retry is warranted. The gate runs BEFORE plan because
// plan performs the write — see runOutward. The lock is released BEFORE any --wait sleep:
// holding it across an hour-long wait would block every other desk writer on this machine,
// converting one session's exhausted budget into a global stall.
func attemptOutward(args []string, opts postOpts, repo string, pr int, plan func(entries []deskkit.Entry, opts postOpts) writeResult) (int, error) {
	// Both early-exit lines below carry repo/pr for ATTRIBUTION (#439, third
	// review, finding 2): every repo-scoped tier matches on `e.repo == repo`, so an audit
	// line written with an empty repo is invisible to every tier's own bookkeeping. Both
	// fields are parameters of this function; omitting them was free before the scoping
	// landed and would leave these lines untraceable to the target they failed against.
	//
	// Both failures are also pre-AllowWrite: no lock means no critical section was ever
	// entered, and an unreadable audit file means the gate was never even consulted, let
	// alone a write attempted. Audited as ResultUnwritten, not ResultUnverifiable
	// (#448) — charging the write budget for a call that could not even
	// START is the same mischarge ready.go's CI-read failures made, one layer up.
	lock, err := acquireAuditLock()
	if err != nil {
		return finishAudit(args, writeResult{verb: toolName, repo: repo, pr: pr, result: deskkit.ResultUnwritten, detail: err.Error(), err: err}), err
	}
	defer lock.release()

	entries, lerr := deskkit.LoadEntries()
	if lerr != nil {
		// Corrupt/unreadable audit file — exit 6. (The audit append below targets the
		// same file; if it is truly unreadable the append warning surfaces too.)
		return finishAudit(args, writeResult{verb: toolName, repo: repo, pr: pr, result: deskkit.ResultUnwritten, detail: lerr.Error(), err: lerr}), lerr
	}

	// The budget is checked on a --dry-run too: a rehearsal makes no remote WRITE but it
	// does make remote READS, and deskrelease sets the precedent that a dry run is still
	// SUBJECT to both meters while feeding neither (#214). What it must never do is COST
	// a charged attempt — that is the property #355 needs, and it comes from the audited
	// result being `dryrun`, below.
	//
	// The refusal carries the scope it was gated on, so the audit line records WHICH PR's
	// budget refused rather than an empty target.
	if aerr := deskkit.AllowWriteAt(toolName, repo, pr, timeNow()); aerr != nil {
		return finishAudit(args, writeResult{
			verb: toolName, repo: repo, pr: pr,
			result: resultForErr(aerr), detail: aerr.Error(), err: aerr,
		}), aerr
	}

	wr := plan(entries, opts)
	return finishAudit(args, wr), wr.err
}

// finishAudit writes the audit line and returns the process exit code. The token/key/
// body MATERIAL never reaches this record — only the discriminated verb, a bodyDigest,
// SHAs, and counts.
func finishAudit(args []string, wr writeResult) int {
	prPtr := wr.pr
	var headPtr *string
	if wr.head != "" {
		h := wr.head
		headPtr = &h
	}
	e := deskkit.Entry{
		Tool:       toolName,
		Verb:       wr.verb,
		ArgsDigest: deskkit.ArgsDigest(args),
		BodyDigest: wr.bodyDig,
		Repo:       wr.repo,
		PR:         &prPtr,
		HeadSHA:    headPtr,
		Result:     wr.result,
		Detail:     wr.detail,
	}
	if err := deskkit.Log(e); err != nil {
		fmt.Fprintf(stderr, "deskpost: WARNING: could not write audit line: %v\n", err)
	}

	// Output goes through the package stdout/stderr vars (helpers.go), which production
	// binds to the process streams and tests capture. Writing to os.Stderr directly here
	// meant the ONE line a caller actually reads — the refusal detail — was the one line
	// no test could assert on, which is how a refusal message can drift from what it needs
	// to say without anything going red.
	if wr.err != nil {
		fmt.Fprintln(stderr, "deskpost: "+wr.detail)
		return deskkit.ExitCodeOf(wr.err)
	}
	if wr.detail != "" {
		fmt.Fprintln(stdout, "deskpost: "+wr.detail)
	}
	return deskkit.ExitOK
}

func resultForErr(err error) string {
	switch {
	case err == nil:
		return deskkit.ResultOK
	case deskkit.IsRateLimited(err):
		return deskkit.ResultRateLimited
	case deskkit.IsRefused(err):
		return deskkit.ResultRefused
	case deskkit.IsDisabled(err):
		return deskkit.ResultDisabled
	default:
		return deskkit.ResultUnverifiable
	}
}
