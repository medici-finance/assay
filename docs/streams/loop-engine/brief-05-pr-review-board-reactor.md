---
brief: loop-engine/05
title: pr-review-desk board-reactor — formalize the event-reactive driver; do NOT drain-ify
wave: 1
depends: ["loop-engine/01"]
unblocks: []
effort: M
gate: human
gate-why: the reactor drives the ready-flip verb (the desk's highest-authority outward write) and its cutover onto a standing review window is human:<name>'s call (desk-tools C-1 pattern); implementation dispatches normally, the gate binds the cutover + sign-off.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-19 by Fable design session (human:<name>'s fix-the-verify-loop direction)
sources: ["docs/loop-engine-architecture.md (§3 archetype B, §8 step 5 — freshness-checked 2026-07-19)", ".claude/skills/pr-review-desk/SKILL.md (the 9-action machine {NEEDS-REVIEW, RE-REVIEW, BLOCKED, CHECK, WAIT-CI, CI-RED, MERGE-CURR, FLIP, READY}; the #79 idle-gate incident)", "tools/desk/cmd/deskboard (already computes the transitions)", "loop-engine/01 (Dispatch/Handle/Result primitive)"]
exec-tier: strong
exec-tier-why: reactor semantics (event coalescing, re-arm on head/CI/review deltas, never-idle gate) are the subtle half; a wrong is-done assumption rebuilds #79.
why: >-
  pr-review-desk is NOT a drain: its items are long-lived PRs that re-arm on every head-SHA,
  CI, or review event; "queue empty" never holds, and treating it as a drain rebuilds the
  #79 incident (19 actionable PRs behind a false all-clear). deskboard already computes the
  9-action state machine deterministically; what runs in the model's attention is the
  reactor — watch deltas, dispatch per action, sweep on cadence. This brief gives archetype
  B its own small deterministic driver that reuses 01's dispatch+isolation primitive while
  keeping event-reactive control flow.
---

# Brief 05 — pr-review-desk board-reactor

## Context
files: `../assay-toolkit/tools/desk/internal/loopengine/` (reuse Dispatch/Handle/Result ONLY — the Loop
drain interface is explicitly NOT implemented here), `../assay-toolkit/tools/desk/cmd/reviewloop/` (new:
reactor driver), `../oit/.claude/skills/pr-review-desk/SKILL.md` (staged repoint; irreducibles stay)
facts:
- Reactor loop: `deskboard` sweep on cadence (~60-90s) + durable event triggers → for each
  PR with an actionable ACTION, dispatch/act per the machine: NEEDS-REVIEW/RE-REVIEW →
  dispatch reviewer at PR-appropriate tier (RE-REVIEW resumes the original reviewer where
  its session survives); MERGE-CURR → no-op; BLOCKED/CHECK/WAIT-CI/READY → no dispatch
  (waiting states, but VISIBLE — never dropped from the board); CI-RED → surface (red is the
  worker's, the desk routes); FLIP → the desk's flip verb via deskpost, gated on the
  dual-verdict-at-head rule for risk-classed PRs.
- The idle gate is the #79 lesson made structural: the reactor may report idle ONLY when
  deskboard prints zero NEEDS-REVIEW and zero RE-REVIEW — the driver enforces this; a model
  cannot declare all-clear.
- Idempotency: `deskkit.AlreadyDone(repo, pr, head, verb)` guards every outward verb —
  event coalescing (N deltas on one PR between sweeps) must collapse to at most one action
  per (pr, head, verb).
- Irreducibles that stay in the skill/desk: App-approval semantics, dual-verdict-at-head,
  MERGE-CURR vs RE-REVIEW judgment on conflict edits (deskboard classifies; the desk can
  overrule upward only), flip = desk-only, drain-before-pull user-delta policy.
- issue-loop's board-reactor pieces (blocked-placeholder unblock scanner, needs-decision
  watch) are follow-up consumers of this driver — note the seam in the PR body; wiring them
  is a follow-up brief, not this one.
- Reviewer dispatch wording stays neutral (dispatch-neutral-wording rule) — the prompt
  template carries the PR facts, never a security frame.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Branch + draft PR only.
- Stop at `implemented`; cutover of the standing review window is human:<name>'s act — stage the
  skill diff, report BLOCKED-ON-IAN at the stop-point.
- Do NOT implement the drain Loop interface for this desk; do NOT add reactor hooks to the
  drain contract. Shared surface = Dispatch/Handle/Result only.
- The flip verb keeps every existing deskpost constraint (App identity, dual-verdict gate);
  this brief adds a caller, never a bypass.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `cmd/reviewloop` reactor driver: cadence sweep off deskboard, action table per facts,
   AlreadyDone-guarded verbs, the zero-NEEDS-REVIEW/zero-RE-REVIEW idle gate in code,
   Guard() at every iteration boundary.
2. Fixture drill: a PR whose fixture head advances mid-cycle re-arms (MERGE-CURR vs
   RE-REVIEW classified by deskboard, honored by the driver); coalescing collapses repeat
   deltas to one dispatch; idle is refused while any NEEDS-REVIEW exists.
3. Stage the pr-review-desk SKILL.md repoint (reactor bookkeeping → driver; judgment
   irreducibles stay) in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0 |
| 2 | `go test ./tools/desk/cmd/reviewloop/... -run 'IdleGate' -count=1` | exit 0 (idle refused with a NEEDS-REVIEW present — the #79 test) |
| 3 | `go test ./tools/desk/cmd/reviewloop/... -run 'Coalesce' -count=1 -v 2>&1 \| grep -c 'dispatched=1'` | ≥1 (N deltas on one (pr,head) → one action) |
| 4 | `grep -rn 'Loop interface\|loopengine.Loop' tools/desk/cmd/reviewloop/ \| wc -l` | 0 (reactor does not implement the drain contract) |
| 5 | PR body contains the staged skill diff + the issue-loop reactor-seam note | present |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at verification time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (standing-window cutover; the flip verb is in scope). Reviewer confirms (a) the
idle gate is code, not prose (row 2), (b) no drain-contract implementation or contamination
(row 4), (c) every outward verb rides existing deskpost/deskkit constraints — no new bypass
around the App-identity or dual-verdict gates, (d) waiting states stay visible on the board
rather than being dropped, (e) dispatch wording in the reviewer template is neutral.
