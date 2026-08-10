---
brief: loop-engine/07
title: Interim — arm a persistent Monitor in verify-desk so the drain self-sustains before the Go conductor lands
wave: 0
depends: []
unblocks: []
effort: S
gate: human
gate-why: the deliverable edits a standing desk operating skill (verify-desk/SKILL.md), which is the-desk's/human:<name>'s remit — role-windows run skills, never edit them (verify-desk-does-not-edit-skills). The interim mechanism must not silently outlive brief-01's conductor, so both landing it and removing it at cutover are human:<name>'s to sequence.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-20 by the-desk (human:<name>'s "fix the verify loop" direction; three-agent root-cause investigation this session)
sources: ["docs/loop-engine-architecture.md (§4 drain contract, §6 per-loop irreducibles, OQ 9.2 tier rung — freshness-checked 2026-07-20)", ".claude/skills/verify-desk/SKILL.md (the loop with no durable liveness mechanism — boot sequence arms no Monitor; line 274-275 admits the only signal is a manual re-poll)", ".claude/skills/pr-review-desk/SKILL.md (steps 3-4: the two persistent:true Monitors + anti-idle hard gate this brief ports)", ".claude/skills/issue-loop/SKILL.md (step 3: persistent Monitor precedent)", "#79 (false-idle incident)", "F-verify-self-attest"]
exec-tier: strong
exec-tier-why: edits the live verify-desk operating skill; a wrong Monitor cadence or a loose anti-idle wording re-introduces the false-idle collapse (#79) the change exists to cure. The wording must match the pr-review-desk hard gate exactly.
why: >-
  The verify loop boots but does not sustain. Its skill body is emphatic that it must DRAIN
  the Awaiting queue to empty ("DRAIN, don't wait… Work it to empty"; "CONTINUOUS drain until
  the Awaiting queue is empty"), but its boot sequence arms no durable liveness mechanism —
  no harness Monitor, no cadence sweep, no wakeup — and its own text admits the only signal is
  "the Awaiting queue, polled each boot / after each merge wave", i.e. a manual re-poll. So once
  the model finishes what it can see in one turn and emits its incremental report, nothing wakes
  it and control falls back to human:<name>. Its two sibling loops do NOT have this problem: pr-review-desk
  arms two persistent:true Monitors (a head-sha event monitor + a ~5-min liveness backstop) plus
  a "never claim idle without a fresh board sweep" hard gate, and issue-loop arms one. verify-desk
  arms none. The proper fix is brief-01's deterministic Go conductor, but that is L-effort, its
  cutover is human-gated, and it is still an open draft PR (#867). This brief is the small interim
  bridge: give verify-desk the same durable-Monitor + anti-idle liveness its siblings already have,
  so the drain self-sustains today, and remove it when the conductor lands.
---

# Brief 07 — Interim durable Monitor for verify-desk

- **Stream**: loop-engine
- **Wave**: 0 (independent; no code dependency — pure skill-body edit)
- **Depends-on**: nothing
- **Relationship to brief-01**: **interim stopgap, superseded by 01.** Brief-01 moves the
  outer loop into a Go conductor. This brief keeps the loop *in the model* but makes it
  self-sustaining via the harness `Monitor` primitive its siblings already use. When 01's
  conductor becomes verify-desk's driver, this Monitor boot step is removed in the same cutover.
- **Effort**: S (skill-body edit + a mirrored anti-idle gate; no Go)

## Problem (root-caused this session)

Three independent investigation agents converged on one cause: **verify-desk's liveness is
prose, not mechanism.** The skill tells the operator to loop until the queue is empty, but
nothing re-invokes the session across turns. Contrast the sibling loops:

| Loop | Durable liveness at boot | Anti-idle guard |
|---|---|---|
| pr-review-desk | **two** `persistent: true` Monitors (head-sha event + ~5-min cadence backstop) | "never claim idle without a fresh board sweep" hard gate |
| issue-loop | one `persistent: true` Monitor (~60s) | — |
| **verify-desk** | **none** | **none** |

Result: verify-desk stops and hands back to human:<name> after each incremental report — the exact
behaviour human:<name> flagged. This is a defect against the skill's own "DRAIN, don't wait" mandate,
not expected behaviour.

## Deliverables

1. **Arm a durable Monitor at boot.** Add a boot step to `../oit/.claude/skills/verify-desk/SKILL.md`
   that arms a `persistent: true` harness `Monitor` whose signal is the Awaiting-verification/
   review queue depth (regenerate STATUS.md / read the Awaiting section), on a fixed cadence
   backstop (~5 min, matching pr-review-desk's liveness Monitor) so the window re-invokes itself
   whenever the queue is non-empty — regardless of whether a merge-wave event fired. Mirror the
   pr-review-desk step 3/4 structure; do NOT invent a new mechanism.
2. **Port the anti-idle hard gate.** Add the pr-review-desk "blind, not idle" gate to verify-desk:
   the window may never report "queue drained" without a *fresh* Awaiting-queue sweep in the same
   turn — a stale board reads as blind, forcing a re-poll, so a quiet Monitor can never produce a
   silent all-clear.
3. **Encode the CURRENT tier collapse, keep the rung a one-liner.** Per loop-engine OQ 9.2 / F-16,
   the Monitor-driven drain still routes risk-flagged briefs to human/strong TODAY. The tier floor
   that would let it auto-drain the cheap set is human:<name>'s decision (filed as the F-16 ratification
   issue). Encode the current conservative routing and keep the rung flip a one-line change, so
   ratifying F-16 does not require re-touching this brief's mechanism.
4. **Removal note.** Add a comment at the Monitor boot step marking it as the brief-07 interim
   bridge, removed when brief-01's conductor becomes the driver — so it does not silently outlive
   its purpose (contract-erosion guard, loop-engine §8 spirit).

## Definition of Done

- verify-desk's boot sequence arms a `persistent: true` Monitor on the Awaiting queue with a
  fixed-cadence liveness backstop, and carries the "no idle claim without a fresh sweep" gate.
- The change is a skill-body edit only — no engine code, no change to the drain *content* (tier
  rules, irreversible carve-out, Evidence/status-to-main behaviour are untouched).
- A dry re-read confirms the loop, on a non-empty queue, re-invokes itself rather than returning
  to human:<name>; the removal note ties it to brief-01's cutover.

## Verify (executable)

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ciE 'persistent.*true' .claude/skills/verify-desk/SKILL.md` | ≥1 (a `persistent: true` Monitor boot step now exists, mirroring pr-review-desk) |
| 2 | `grep -ciE -e 'fresh .*sweep' -e 'blind, not idle' -e 'never claim.*drained' .claude/skills/verify-desk/SKILL.md` | ≥1 (the anti-idle gate is present) |
| 3 | `grep -ciE -e 'brief-07' -e 'interim' .claude/skills/verify-desk/SKILL.md` | ≥1 (removal note ties the Monitor step to brief-01's cutover) |
| 4 | `git diff origin/main -- .claude/skills/verify-desk/SKILL.md \| grep -cE '^\+' \| xargs -I{} test {} -gt 0 && echo ok` | `ok` (the skill edit landed; diff is additive boot/liveness only — reviewer confirms tier/irreversible/Evidence content unchanged) |
| 5 | `statusgen --lint; echo $?` | `0` |

## Notes

- **This is not brief-01.** Brief-01 is the durable, model-out-of-the-loop Go conductor and stays
  the real fix. This brief buys correct liveness *now* with a one-step skill edit, and is designed
  to be deleted at 01's cutover.
- **Skill-edit remit**: the deliverable edits a standing desk skill, so implementation is the-desk's
  or human:<name>'s — not a generic fanout worker (hence `gate: human`).
