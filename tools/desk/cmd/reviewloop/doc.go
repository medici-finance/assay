// Command reviewloop is pr-review-desk's BOARD-REACTOR driver (loop-engine/05).
//
// # It is not a drain, and it is deliberately not built like one
//
// The archetype-A loops (verify-desk, batch-fanout, the issue-loop dispatch lane) are
// DRAINS: a queue that empties, whose termination condition is "nothing left". They are
// driven by tools/desk/internal/loopengine, whose Run() exits only on a stop flag and
// whose is_done() is "no in-flight work".
//
// pr-review-desk is archetype B. Its items are LONG-LIVED PRs that re-arm on every
// head-SHA, CI, or review event; "queue empty" never holds for longer than the interval
// between two events, and a driver that treats an empty read as a terminal state rebuilds
// incident #79 — 19 actionable PRs (12 NEEDS-REVIEW + 7 RE-REVIEW) sitting behind a false
// all-clear that came from a dead monitor, not from an empty board.
//
// So this package implements NONE of the drain engine's control flow. It does not
// implement loopengine.Loop; it adds no hook to that frozen contract; there is no
// SelectQueue / OnIdle / is_done here. The one thing it takes from the drain side is the
// idea that scheduling facts belong in code rather than in the operator model's attention.
//
// It goes one step further than loop-engine/05's brief asked, and the deviation is stated
// rather than left to be discovered: the brief permitted reusing loopengine's
// Dispatch/Handle/Result types, and this package imports NOTHING from loopengine at all.
// The reason is measured, not stylistic — loopengine.Item is brief-shaped (BriefPath,
// RiskFlags parsed from brief-v1 frontmatter, Implementer for the author!=runner guard,
// ExecTier). A board row is a long-lived PR with none of those fields, so wrapping one in
// an Item would mean constructing a mostly-empty brief and inviting the next reader to
// assume the drain's scheduling semantics come with it. The compile-time distance is the
// cheapest guarantee that a later change cannot drift this reactor into a drain.
//
// # What is formalized
//
// Three facts that were prose in .claude/skills/pr-review-desk/SKILL.md become code:
//
//  1. The ACTION table (ActionTable). The skill's boot step 2 names NINE actions; the
//     board computes EIGHTEEN (tools/desk/cmd/deskboard/board.go:655-698). A reactor
//     written against the nine would silently drop the other nine states off the board.
//     ActionTable is required by test to be exhaustive over deskboard's own constants,
//     and an action it does not know is COULD-NOT-CHECK (exit 6), never "nothing to do".
//
//  2. The idle gate (IdleVerdict). The skill's HARD GATE says the desk may report idle
//     only when a fresh sweep shows zero NEEDS-REVIEW and zero RE-REVIEW. Here that is a
//     function whose THIRD state is CouldNotCheck, and every way of failing to read the
//     board — decode error, empty scope, truncated PR population, a stale sweep, an
//     unknown action — lands there rather than in Idle. A model cannot declare all-clear;
//     it can only report what IdleVerdict returns.
//
//  3. Event coalescing (Coalesce). N deltas on one (repo, PR, head) between sweeps
//     collapse to at most one outward action per (repo, pr, head, verb), guarded by
//     deskkit.AlreadyDone — the same audit-keyed idempotency every deskpost verb uses.
//
// # Stop point
//
// Only the `plan` subcommand ships. Like verifyloop, this tool emits the deterministic
// reactor output — the disposition of every board row, the coalesced outward verbs, and
// the idle verdict — and spawns nothing. Cutover of the standing review window onto this
// driver is gate: human (the reactor drives the ready-flip verb, the desk's
// highest-authority outward write). That is BLOCKED-ON-HUMAN by design, not an omission.
//
// The reactor NEVER merges. Merge is always the human's; the desk's most it may do is
// flip a MERGE-NOW draft to ready, and this package only ever EMITS that verb for a
// caller that carries deskpost's existing App-identity and dual-verdict gates.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused · 6 unverifiable.
package main
