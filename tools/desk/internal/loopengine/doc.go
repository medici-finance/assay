// Package loopengine is the deterministic drain engine for the desk's archetype-A
// loops (docs/loop-engine-architecture.md §4 contract, §7 skeleton; brief
// loop-engine/01). It moves the OUTER control loop — read the queue, hold a pool of
// N in-flight workers, claim, dispatch, land each result as it returns, refill the
// freed slot, idle-poll — out of the operator model's attention and into code, on
// top of the deterministic instruments tools/desk already provides (deskkit.Guard,
// audit, the shared claims dir).
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
