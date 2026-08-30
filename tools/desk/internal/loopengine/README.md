# loopengine — deterministic drain engine

The outer control loop of the desk's archetype-A drains — **read the queue → hold a pool of N
in-flight workers → claim → dispatch → land each result as it returns → refill → idle-poll** —
moved out of the operator model's attention and into code, on top of the deterministic
instruments `tools/desk` already provides (`deskkit.Guard()`, audit, the shared claims dir).

This package is the real thing built from the architecture design (§4 contract, §7 skeleton,
§9 open questions).

## Dedupe-at-start is a STATED guarantee, and `Claim()` is its only entry point

**An item ID is dispatched at most once concurrently, across ALL engine consumers sharing the
claims dir. This holds only for dispatchers that route through `Claim()`; a dispatcher that
bypasses `Claim()` is outside the guarantee and is a bug, not a policy choice.** Restating the
protocol somewhere else does not extend it — routing through `Claim()` does. (The same brief was
implemented twice, 81 minutes apart, when the protocol lived as prose in two places and bound
nobody.)

What `acquired` asserts is **bounded, and the bound is announced**. A claim file records that a
dispatcher is working on an item; it is not evidence the work is untaken — measured 2026-08-13,
24 of 147 board rows reading `todo` in this repo already had a **merged** PR. So `Claim()` first
consults `Config.WorkEvidence`, the per-consumer probe for evidence outside the claims dir (open
and merged PRs naming the item, the board row), and reports which checks it ran:
`(true, nil)` acquired, bounded by whatever actually ran; `(false, nil)` do not dispatch, with a
`DEDUP <id> — <why>` line naming the evidence; `(false, err)` **could-not-check**
(`deskkit.Unverifiable`, exit 6) — never "assume free". With no probe configured `Claim()` says so
on every item (`BOUND <id> — evidence consulted is the claims dir ONLY`): no silent caps.
Boundary conditions — sanitized-ID over-locking, single-winner stale reclaim, `ReleaseClaim` on
dispatch failure, probe-then-lock not being one atomic step — are stated in
[`doc.go`](./doc.go). Cross-machine claiming is **not** this package's job; that is the
GitHub-durable `refs/dispatch/<id>` claim, out of scope here. Full enforcement (every dispatcher
routing through `Claim()`) arrives with the consumer migrations.

## Retry is DECLARED DATA, and the taxonomy has THREE outcomes

**This section is the single home for the retry rules the desk skills currently carry as
prose.** Five skills each state their own version ("rerun once for a known flake", "retry the
push", "never retry `deskpost` exit 4"); those passages are noted, not edited, here — they are
replaced at their next consumer migration. Until then this table
is what the engine actually does, and the prose is what a human is asked to do.

The table below is **generated from `Taxonomy()` in [`retry.go`](./retry.go)** and diffed by
`TestRetryTaxonomyDocIsDerivedNotDuplicated`: `retry.go` is the one declared source, this is a
regenerated copy, and drift between them is a red test rather than a stale paragraph.

<!-- taxonomy:begin -->
| Class | Engine action | Bound | Decided by (measured evidence) |
|-------|---------------|-------|-------------------------------|
| `transient` | retry, exponential backoff + jitter | `MaxAttempts` total attempts; each wait clamped to `MaxInterval`; all waiting summed under `MaxElapsed` | measured 2026-08-13: `API Error: 529 Overloaded` killed two worker sessions minutes apart (both resumed and completed); GitHub secondary rate limit 403 at ~16 concurrent ops on one token with core budget remaining; gh/GitHub 5xx |
| `retry-after` | wait the STATED instant, then attempt once | `RetryAfterMaxAttempts` (tighter than `MaxAttempts`); a stated wait longer than `MaxElapsed` is NOT slept — the item is landed blocked with the free-at instant | deskkit exit 4 carrying `RetryAfterOf`; measured 2026-08-13, `deskfile` rc=4 with its filing budget exhausted until a named wall-clock time. Retrying early re-charges the budget that refused us |
| `refusal` | NEVER retry; land blocked | exactly one attempt — and `NonRetryable` can only narrow the retryable set, so `retry on 5` is not expressible in this package's config | deskkit exit 3 / 5 / 6 and loopengine exit 7. Measured 2026-08-13: `deskpr create` rc=5 on a writeguard false positive, rc=5 on two bodycheck refusals, a guard-blocked `git push`. A refusal is a decision, not a failure — the fix is upstream (reword, or fix the guard), and reaching for another tool to make the same write is the worse sibling of retrying |
| `deterministic` | NEVER retry; land blocked | exactly one attempt; reachable only via the typed `Deterministic()` wrapper — the engine CANNOT infer it from an exit code | measured 2026-08-13: `deskpr create` rc=6 from the `origin/remotes/origin/main` spelling (a known ref-spelling bug, hit by every worktree in this repo, every time) and `deskclaim release --kind` rc=5, a plain unknown-flag usage error. Both arrive as a bare 5 or 6, so the classifier files them under refusal; same action, different remediation |
| `unclassified` | could-not-check: NOT retried, NOT silently dropped — landed `NEEDS_CONTEXT` | zero retries and zero waiting; every shape that lands here is published by `UnclassifiedShapes()` | the third state of the instrument invariant. Motivating measurement 2026-08-13: a PR with ZERO CI check-runs, attributed to an App-authored anti-recursion rule — an attribution that is measurably FALSE (an App-authored PR carries 19 check-runs; three PRs sharing PR author and commit author carry 21 / 8 / 0). `wait and retry` versus `escalate` is not derivable from that symptom, and a two-state taxonomy would have guessed |
<!-- taxonomy:end -->

### Why there are three outcomes and not two

A retryable/non-retryable pair has to **guess** on a failure it does not recognise, and both
guesses are defects — guess retryable and an unrecognised refusal loops forever; guess
non-retryable and a transient failure drops the work silently. The engine therefore reports in
the same three states every other desk instrument does
([`docs/three-state-instrument-rule.md`](../../../../docs/three-state-instrument-rule.md)):
checked-clean (dispatch succeeded), checked-failed (a class was decided), **could-not-check**
(`unclassified`). `unclassified` is landed as `NEEDS_CONTEXT`, which routes to a human — it is
neither retried nor recorded as a decision nobody made.

The measurement that forced it: a PR showing **zero CI check-runs**, attributed to an
"App-authored PRs get no CI" anti-recursion rule. That attribution is measurably **false** — an
App-authored PR in the same repo carries 19 check-runs, and three PRs sharing both PR author and
commit author carry 21 / 8 / 0. "Wait and retry" versus "escalate" is not derivable from the
symptom, so the taxonomy is allowed to say so.

### `exit 5` is never a retry trigger, and never a fallback trigger

Exit 5 is a deliberate safety stop. Retrying it identically loops forever (the fix is upstream —
reword the body, fix the guard), and *falling back to another tool* is the worse sibling: a
guard-blocked `git push` answered by reaching for the API is the same write with the guard
removed. This is enforced structurally, not by convention: the retryable base set is **closed**
to `transient` and `retry-after`, and `RetryPolicy.NonRetryable` can only **narrow** it. "Retry
on 5" is not expressible in this package's configuration surface.

Note that 5 is **already overloaded** — `deskclaim release --kind` returns 5 for a plain
unknown-flag usage error, a deterministic bug rather than a safety refusal. This package does
not add a third meaning to 5; it records that the engine cannot tell the two apart from the code
alone, which is why `deterministic` is reachable only through the typed `Deterministic()`
wrapper.

### Every bound is stated — no silent caps

<!-- bounds:begin -->
| Bound | Default | What it caps | At the bound |
|-------|---------|--------------|--------------|
| `MaxAttempts` | `3` | total attempts for one item, first attempt included | land blocked (`attempts-exhausted`), release the claim, drain continues |
| `InitialInterval` | `2s` | the wait before attempt 2 | — |
| `BackoffCoefficient` | `2` | growth per attempt | — |
| `MaxInterval` | `1m0s` | backoff CEILING — no single wait exceeds it | waits stop growing; attempts continue to `MaxAttempts` |
| `MaxElapsed` | `5m0s` | wall-clock cap on all waiting for one item | land blocked (`wall-clock-cap`) WITHOUT waiting; must stay under `Config.StaleClaim` or `Run` refuses to start |
| `Jitter` | `0.2` | +/- randomisation of each computed backoff | — |
| `RetryAfterMaxAttempts` | `2` | attempts for a `retry-after` failure specifically | land blocked; deskkit's own text says sleep the full wait and attempt ONCE |
<!-- bounds:end -->

### What the taxonomy does NOT classify

These shapes are published by `UnclassifiedShapes()` and land in `unclassified`. A taxonomy that
does not name its own blind spots is a silent cap.

<!-- unclassified:begin -->
- a bare exit 4 carrying NO retry-after — the wait is unknown, and guessing it short is exactly the retry livelock; deskkit.RateLimited (no retry-after) produces this shape
- `context deadline exceeded` and every other timeout — the timeout taxonomy and heartbeat liveness live in liveness.go; classifying timeouts here would fork that decision across two homes
- `no such host` / DNS resolution failure — indistinguishable from a permanently wrong hostname without a second observation the engine does not have
- any error carrying no deskkit exit code and matching no declared transient signature — the default, and deliberately so
- a PR showing ZERO CI check-runs — not a dispatch error at all: it surfaces inside a worker's Result, its cause is could-not-check (the App-authored anti-recursion attribution is measurably false), and 'wait' vs 'escalate' is not derivable from the symptom
- a load-induced test flake inside a dispatched worker (a 5s fail-closed callout timeout reddening 3 tests under parallel `go test`) — the dispatch SUCCEEDED, so this is a Result verdict for Land, not a dispatch failure for this classifier
<!-- unclassified:end -->

### How a retry interacts with the claim

A retry **re-dispatches the same claim**. `dispatchWithRetry` runs entirely inside the claim the
drain acquired before the first attempt: it never releases and never re-acquires between
attempts, so a retry can neither **double-acquire** (there is exactly one `Claim()` per item per
drain pass) nor **orphan** the claim (it is released exactly once, after the terminal outcome is
landed). A release-then-reclaim between attempts would reopen the double-dispatch window
`claim.go`'s flock dance exists to close, because the gap between release and acquire is
unlocked.

A held claim also **ages**. That is why `MaxElapsed` exists and why `Run` refuses a config whose
`MaxElapsed` could reach `Config.StaleClaim`: a dispatcher that waited past the stale threshold
could have its own live claim reclaimed under it by a second dispatcher that correctly judged it
abandoned — a double dispatch produced by the retry policy itself. Cross-machine claiming is
still the GitHub-durable `refs/dispatch/<id>` claim, unchanged by this.

### Retry re-dispatches; whether the worker RESUMES is the adapter's business

Measured 2026-08-13: `529 Overloaded` killed two worker sessions minutes apart, both had their
uncommitted work intact in their worktrees, and both completed after being resumed from state —
restarting from scratch would have burned that work. The `Loop` interface is frozen, so there is
no `Dispatch` parameter for "this is attempt 2"; the signal rides the existing `Payload`
template-variable channel instead, as `retry_attempt` / `retry_of`, set only from attempt 2
onward (a first dispatch's `Payload` is unchanged). The engine cannot observe whether an adapter
honours it — that bound is stated, not enforced.

### Adjacent, cited but not absorbed

- The liveness path owns **timeouts** and heartbeat liveness. An HTTP transport timeout on the
  dispatch *call* is transport-class here; a dispatched *worker* going quiet is the liveness
  path's, and `context deadline exceeded` is deliberately left unclassified.
- The three-state `WorkEvidence` probe on `Claim()` landed earlier; this reuses that
  shape rather than inventing a second one.
- **Crash recovery** is journal replay. Every scheduling decision — claim,
  dispatch, retry, land, reclaim, release, refuse — is journalled to the audit stream
  (`Tool: loopengine`), each line carrying a per-`Run()` `runID` lineage token. At boot
  `Recover` (`recover.go`) replays the journal scoped to the loop name and classifies the
  claim files a prior crashed run left on disk: a completed leftover is **released**; an
  in-flight claim of a prior run of THIS conductor is freed for **immediate reclaim** (no
  120m staleness wait); another consumer's claim is **untouched**. A corrupt/unreadable
  journal fails closed to the existing conservative heuristics — never a guess from a partial
  replay (including a torn final line, the crash mid-append this feature recovers from: that
  makes recovery unavailable, not a partial replay). The reclaim itself still routes through
  `claim.go`'s lock-guarded single winner. Four enforced guards bound the reclaim path so it
  never re-executes an outward intent unsafely: recovery runs only while `Run()` holds the
  **per-lineage boot flock** (`lineagelock.go` — a live sibling of the same lineage still
  holds it, so its claims are never mistaken for a crashed run's); `deskkit.Guard()` is
  consulted **before** `Recover`, so an armed kill switch stops boot-time claim mutation; an
  in-flight claim is **not auto-reclaimed without a `WorkEvidence` probe** (re-dispatch would
  consult only the claims dir); and an item already reclaimed `MaxRecoveryReclaims` times is
  **not reclaimed again** (the only cross-boot bound on a crash-looping item). Removal is a
  **compare-and-delete** under the claims-dir flock (`deskkit.ReleaseMatching`) — a claim
  reclaimed in place underneath recovery is left alone, not deleted from its new live owner.

## The contract is frozen (arch doc §8 "contract erosion")

The `Loop` interface — `Name / SelectQueue / TierPolicy / Dispatch / Land / OnIdle` — is
deliberately small. Heterogeneity lives in `Land` and `TierPolicy`. **Anything that does not
fit those two hooks does NOT go in the engine**: a proposed new hook is a design review (file
a contract-erosion issue), not a patch. Four downstream briefs (02–05) consume this contract;
an economy-tier hook added here propagates to every consumer.

## Advisory write-scopes — a coordination hint, never a lock

Parallel workers isolated in worktrees still collide at **merge time** on a shared file. A
brief's **write-scope set** (the path prefixes it expects to touch) makes that foreseeable
collision visible at dispatch, before the work starts. The set is
**derived** by default from the brief's Context `files:` list — each entry normalized to a
repo-relative path prefix, a `../<repo>/…` entry to that sibling repo plus prefix, a glob
trimmed back to its directory prefix — and **replaced** by an optional `write-scopes:`
frontmatter override. `writescope.go` carries the pure derivation + overlap logic;
`writescope_io.go` the offline readers; `Item.WriteScopes` carries the set as **data**.

It is a **coordination HINT, not a lock** (verbatim from the donor mechanism: "coordination
hints, not locks"). Nothing here blocks a dispatch, gates a claim, serializes execution, or
authorizes/denies a filesystem write — the sole product is a warning line a human reads
(`WRITE-OVERLAP: <candidate> ~ <in-flight> on <prefix>`), emitted by the plan/dispatch surfaces
**after** every eligible item is already scheduled. Proceeding over a warning is correct when
the overlap is intended (an agreed merge-serialization). This adds **no new `Loop` hook** — it
is plan-output data plus a dispatch-time read, riding `Item` data, so the frozen contract is
untouched. Three-state honest: a brief whose scope set cannot be derived is reported
`could-not-derive`, never rounded down to "no overlaps".

## Dispatch mode — interim now, native primitive later (Open Question 9.1, resolved)

A Go conductor **cannot call the harness `Agent` tool** — there is no Go binding to it. So
`Run()` ships in the **interim mode**: the Go engine owns ALL scheduler state deterministically
(pool occupancy, claims, refill, land, `is_done`, `Guard`) and drives dispatch through
`Loop.Dispatch`, which **emits an exact dispatch instruction** the operator model executes
verbatim as an `Agent` call, then feeds a structured `Result` back through the returned
`Handle`. Zero scheduler state lives in the model's attention — one iteration is *"the engine
says exactly which dispatch to make; make it; feed the Result back"*.

**The whole win — the outer loop leaving the model's attention — is achieved by the Go engine
regardless of dispatch mode.** Interim mode is not a lesser version of the win; it is the win,
with a human-in-the-loop dispatch step instead of a native spawn.

**Native-primitive upgrade (landed):** the native-primitive dispatch has since landed —
`cmd/verifyloop/dispatch_native.go` spawns each dispatched worker as a real ACP child process,
selected by `adapter.go`'s `Dispatch` when `Native` is set, in place of the interim
emit-and-await. It was a **drop-in swap of the `Loop.Dispatch` implementation only** — the
engine loop, the `Loop`/`Handle`/`Result` contract, and every consumer were unchanged; `Run()`
was written so that swap touched nothing else. Interim mode remains the default, so it is
current behaviour, not history.

## Honest bound (arch doc §1.1) — do not overclaim

What the engine buys is **attribution, audit, and structural separation**, NOT cryptographic
un-forgeability. No inline-verify path exists (the only way an item leaves the queue is
`Dispatch` — the inline-verify class is unrepresentable), `author != runner` is a typed structural
guard (`CheckAuthorRunner`, distinct exit code 7), and Evidence is rendered from the dispatched
runner's structured `Result` (command → exit → key output), not free text. But a session
holding App signing material can still forge; the true close is `author != approver` between
distinct Apps + branch protection, which is out of scope here.

## Consumers

- `tools/desk/cmd/verifyloop` — the verify-desk reference consumer. Proves the
  contract end-to-end; the autonomous cutover to the standing verify window is **gate: human**.
