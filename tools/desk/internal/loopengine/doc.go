// Package loopengine is the deterministic drain engine for the desk's archetype-A
// loops (docs/loop-engine-architecture.md §4 contract, §7 skeleton; brief
// loop-engine/01). It moves the OUTER control loop — read the queue, hold a pool of
// N in-flight workers, claim, dispatch, land each result as it returns, refill the
// freed slot, idle-poll — out of the operator model's attention and into code, on
// top of the deterministic instruments tools/desk already provides (deskkit.Guard,
// audit, the shared claims dir).
//
// # Dedupe-at-start — the STATED guarantee (loop-engine/10)
//
// Claim is the SINGLE MANDATORY ENTRY POINT for dispatch. The guarantee, stated so
// consumers can be held to it:
//
//	An item ID is dispatched at most once concurrently, across ALL engine consumers
//	sharing the claims dir. This holds only for dispatchers that route through
//	Claim(); a dispatcher that bypasses Claim() is outside the guarantee and is a
//	bug, not a policy choice.
//
// Dedupe-at-start is worthless as a convention and only real as a semantic guarantee
// behind one entry point: the same brief was implemented twice, 81 minutes apart
// (PRs #411/#413, 2026-08-03), because the protocol existed as prose in two places and
// bound nobody. Restating the mechanism elsewhere does not extend the guarantee —
// routing through Claim is what extends it.
//
// # What "acquired" does and does not assert (the evidence bound)
//
// A claim records that THIS dispatcher is working on the item. On its own it says
// nothing about whether the work is already done or in flight somewhere else: the
// claims dir is one source of evidence, not all of it. Measured 2026-08-13 across the
// stream boards in this repo, 24 of 147 rows reading "todo" already had a MERGED PR
// (~16%) — so a dispatcher that consults only the board and the claim file is wrong
// about one item in six, in the direction that causes duplicate work.
//
// So Claim's return is bounded, and the bound is stated in Claim's own output rather
// than left implicit:
//
//   - (true, nil) — acquired. Asserts mutual exclusion among claims-dir participants,
//     PLUS whatever Config.WorkEvidence was able to prove. With WorkEvidence unset it
//     asserts ONLY the former, and Claim SAYS SO on Config.Progress every time. There
//     are no silent caps: if Claim bounds what it consulted, it announces the bound.
//   - (false, nil) — do not dispatch. Either a live claim is held elsewhere, or
//     WorkEvidence reported the item already taken; the reason is printed as a
//     "DEDUP <id> — <why>" line naming what it deduplicated against.
//   - (false, err) — COULD-NOT-CHECK (docs/three-state-instrument-rule.md). The lock
//     could not be held, a claim could not be read/written, or WorkEvidence could not
//     reach its source. This is deskkit.Unverifiable (exit 6) and it is NEVER
//     "assume free" (the #146 lesson). A dedupe that fails open silently is the
//     defect, not the fix.
//
// # Boundary conditions
//
//   - Sanitized-ID collision. The claim path is the item ID with "/", the OS path
//     separator and ".." each replaced by "_", so "loop-engine/01" cannot escape the
//     claims dir. Two DISTINCT IDs that sanitize to the same segment therefore share
//     one claim and collide with each other. That is deliberate over-locking (safe
//     direction), not a hash: prefix item IDs by repo when two repos own a stream of
//     the same name.
//   - Kind is attribution, not a correctness axis. Two claims on the same Item collide
//     regardless of deskkit Kind, because the on-disk path is keyed on Item alone.
//   - Stale-reclaim has a SINGLE winner. A claim older than Config.StaleClaim with no
//     provably-live branch may be reclaimed; the whole create-or-reclaim decision runs
//     under a directory-wide flock and rewrites the file IN PLACE (never
//     Remove-then-create, which would open a window for a second fresh claimant). A
//     claim whose recorded branch cannot be PROVEN inactive (Config.BranchActive nil)
//     is conservatively not reclaimed.
//   - ReleaseClaim on dispatch failure. A caller that claims and then fails to dispatch
//     MUST ReleaseClaim, so the slot is handed back rather than parked behind a claim
//     that only time can clear. Release of a missing claim is a no-op.
//   - Probe-then-lock is not one atomic step. WorkEvidence runs before the flock (a
//     network call must not be made while holding a directory-wide lock), so evidence
//     appearing between the probe and the create is not caught by this call. The lock
//     closes the local race; the probe closes the "work already exists elsewhere"
//     hole. Neither closes the other, and neither is a substitute for the branch/PR
//     check the dispatching consumer runs.
//   - Cross-machine claims are NOT this package's job. The claims dir is machine-local:
//     it serialises dispatchers sharing a machine and does nothing for two desks on
//     different machines. That case is methodology/42's GitHub-durable
//     refs/dispatch/<id> claim (tools/dispatch-claim.sh), which is a separate home and
//     is not re-implemented here.
//   - Enforcement is not complete yet. This package states the contract; every
//     dispatcher actually routing through Claim arrives with the consumer migrations
//     (loop-engine/02, /03). Until then the statement above is the bar those
//     migrations are held to, not a description of the whole tree.
//
// # Retry is declared data, and its taxonomy has THREE outcomes (loop-engine/08)
//
// Dispatch runs under a declared RetryPolicy (Config.Retry, overridable per Item). A
// dispatch failure is classified by Classify into one of five classes — transient,
// retry-after, refusal, deterministic, unclassified — of which only the first two are
// ever retried. The full table, every bound, and the measurements behind each class are
// in retry.go and rendered into README.md from Taxonomy(), which is the one declared
// source; the doc copy is CI-diffed, not hand-maintained.
//
// Three properties are worth stating here because they are contracts, not details:
//
//   - COULD-NOT-CHECK IS FIRST-CLASS. ClassUnclassified is not a synonym for
//     non-retryable. An unrecognised failure is neither retried (an infinite loop) nor
//     landed as a decision nobody made (a silent work loss): it lands as
//     VerdictNeedsContext. The shapes that land there are published by
//     UnclassifiedShapes — a taxonomy that hides its blind spots is a silent cap.
//   - EXIT 5 IS NEVER A RETRY TRIGGER, AND NEVER A FALLBACK TRIGGER. It is a deliberate
//     safety stop; retrying it identically loops forever, and answering it by reaching
//     for a different tool is the same write with the guard removed. The retryable base
//     set is CLOSED and RetryPolicy.NonRetryable can only narrow it, so "retry on 5" is
//     not expressible in this package's configuration at all.
//   - A RETRY RE-DISPATCHES THE SAME CLAIM. dispatchWithRetry never releases and never
//     re-acquires between attempts: exactly one Claim per item per drain pass (no
//     double-acquire) and exactly one ReleaseClaim after the terminal outcome is landed
//     (no orphan). Because a held claim ages, Run refuses a config whose
//     RetryPolicy.MaxElapsed could reach Config.StaleClaim — otherwise the retry policy
//     itself could let a second dispatcher reclaim a live claim mid-retry.
//
// Timeouts and heartbeat liveness are loop-engine/09's, not this classifier's, and
// "context deadline exceeded" is deliberately left unclassified so the decision has one
// home. Each retry decision is journalled to the audit stream (Tool "loopengine", Verb
// "retry") as the groundwork loop-engine/11's replay reads.
//
// # What the engine buys (honest bound — arch doc §1.1)
//
// The win is ATTRIBUTION, AUDIT, and STRUCTURAL SEPARATION — NOT cryptographic
// un-forgeability. Concretely:
//
//   - The scheduler leaves the model's attention entirely. Pool occupancy, the claims
//     ledger, the refill trigger and the push-race retry become code with zero
//     attention cost, so the loop cannot collapse to single-threaded "from confusion"
//     under load (the observed verify-desk failure). This holds regardless of dispatch
//     mode (see below) — the whole win is that the outer loop is code, not prose.
//   - No inline-verify path exists: the ONLY way an item leaves the queue is Dispatch.
//     The #541 failure mode (a full afternoon of inline verification, zero durable
//     artifacts) is not discouraged, it is unrepresentable.
//   - author != runner is a typed structural guard (CheckAuthorRunner), a comparison
//     against the brief's implementer identity, not an honor-system prose rule.
//   - Land consumes only the structured Result (command -> exit -> key output row,
//     runner attributed to the identity the engine dispatched) — no free-text verdict
//     reaches a board cell.
//
// A session holding an App signing key can still mint a token and write what it likes;
// the engine narrows the forgery surface from "any free-text cell" to "an actor with
// signing material". That is a real improvement, not a close. The true close is
// author != approver between DISTINCT Apps plus branch protection (the same caveat
// pr-review-desk carries for its reviewer-App gate). This package's claims stop there.
//
// # Dispatch mode (Open Question 9.1, resolved for brief-01)
//
// A Go conductor CANNOT call the harness Agent tool — there is no Go binding to it. So
// Run ships in the INTERIM mode: the Go engine owns ALL scheduler state deterministically
// (pool occupancy, claims, refill, land, is_done, Guard) and drives dispatch through the
// Loop.Dispatch hook. In interim mode the adapter's Dispatch EMITS an exact dispatch
// instruction (the fully-rendered prompt + tier) which the operator model executes
// verbatim as an Agent call, then feeds a structured Result back through the returned
// Handle. The model carries no scheduler state — one loop iteration is "the engine says
// exactly which dispatch to make; make it; feed the Result back". This is the same
// relationship the loops already have with deskboard ("the board computes ACTION, the
// model executes it"), extended to the whole loop.
//
// The eventual NATIVE-PRIMITIVE upgrade — a future Go->Agent binding, or running Run as
// a Workflow so dispatched workers are real child processes — is a drop-in swap of the
// Loop.Dispatch implementation: the engine loop, the contract, and every consumer are
// unchanged. Run is written so that swap touches only Dispatch. See README.md.
package loopengine
