---
brief: desk-hardening/06
title: Durable desk watchdog + autonomous drive + file-at-discovery
why: >
  The desks' continuous-watch mechanism is a local background bash loop shelling gh; on machine
  idle macOS App Nap suppresses it, so the monitor goes silently blind while foreground gh works.
  It went blind live: the pr-review-desk reported "nothing in flight" while 19 actionable PRs
  piled up. Underneath sit two behavioural gaps: the standing desks run reactively (act-on-prompt
  then idle, the human becomes the scheduler), and they narrate filable discoveries in chat and
  wait for approval instead of filing at discovery time. A durable watchdog belongs in an
  always-on committed service, not a laptop loop — and a desk must never claim "idle" without a
  fresh sweep.
wave: 0
depends: []
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [86, 79, 70, 71]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#86 (background gh monitor goes blind on idle — durable watchdog belongs in the observability service)"
  - "assay-toolkit#79 (URGENT: desk went silently blind, falsely reported idle while 19 PRs piled up)"
  - "assay-toolkit#70 (standing desk roles run reactively instead of autonomously looping — REVIEW/VERIFY/COORDINATE)"
  - "assay-toolkit#71 (desk narrates filable discoveries and waits for approval instead of filing at discovery)"
  - "EXTERNAL HEAD: oit #627/#651 (observability watchdog exporter — the durable service home)"
  - "EXTERNAL GATE: oit #556 (App issues:write — the filing identity #71 needs), #120 (worker App workflows perm)"
exec-tier: strong
exec-tier-why: "liveness/heartbeat design where a dead monitor must be LOUD not silent (a false all-clear is the failure); spans a service + four desk skills."
consumers:
  - "[oit-obs] the observability watchdog service (oit #627/#651): follow-up (the durable liveness half lands there, not here)"
  - "[oit] .claude/skills/{pr-review-desk,verify-desk,batch-fanout,the-desk}/SKILL.md: fixed-here (autonomous drive + no-idle-without-sweep + file-at-discovery)"
---

# Brief 06 — Durable desk watchdog + autonomous drive + file-at-discovery

## Context
files:
- `[oit-obs]` the always-on watchdog service under oit `docs/streams/observability/` (#627/#651) — the durable-liveness deliverable's true home
- `[oit]` `../oit/.claude/skills/pr-review-desk/SKILL.md`, `../oit/.claude/skills/verify-desk/SKILL.md`, `../oit/.claude/skills/the-desk/SKILL.md`, `../oit/.claude/skills/batch-fanout/SKILL.md` (autonomous drive; no-idle-without-sweep; file-at-discovery)
out-of-repo files: none
facts:
- root cause of the blindness (#86, diagnosed 2026-07-17): background-execution suppression on idle (App Nap) — NOT auth, NOT rate limit, NOT keychain lock (a file-token test disproved the keychain theory; record so it is not re-tried)
- what works today: reviewer task-completion notifications (harness events, independent of background gh) + on-demand foreground `deskboard.go` sweeps
- #79: the desk conflated "my dispatched reviewers are done" with "the queue is empty" — different facts; a board sweep must be a hard precondition of any idle claim
- #86 coverage note: any watch mechanism MUST cover the full board-bearing repo set (the lean monitor had dropped repos that had open PRs; the board still covered them, but a monitor watching fewer repos than the board is a latent blind spot)
- #70: the skills describe the per-item loop but not an autonomous DRIVE ("loop until the queue is empty, then watch; refill empty slots before yielding"); the only automatic re-invocation is on subagent completion — no re-invocation on external state change
- #71: file at discovery is the DEFAULT (oit#474); the chat relay is a *notification* of the filed issue, never a request for permission; the desk added the per-item human-approval gate itself; the actor-path fix is oit#556 (App issues:write)
- this is the same defect class as desk-hardening/01 turned on the desk itself: an instrument (the board's "idle") reporting a state it never checked

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Durable liveness — the contract, here; the service, external.** This brief owns the
   *requirement + interface*: the watchdog runs in the always-on committed service (oit
   `#627/#651`), not a local bash loop; it emits a **heartbeat** so a dead monitor is LOUD
   (detectable), never silently absent; it covers the full board-bearing repo set. **The service
   build is the external head** — do NOT reimplement it here; land the contract + the desk-side
   fallback and pair with the observability stream. **Candidate approaches:** (a) the
   watchdog-exporter + Pushgateway (oit#627/#651); (b) a GitHub-webhook receiver; both run
   regardless of any operator's machine state.
2. **No idle claim without a fresh sweep (#79).** Codify in every standing-desk skill: the desk
   MUST run a fresh `deskboard.go` sweep and confirm zero NEEDS-REVIEW / RE-REVIEW before it ever
   says "idle" / "caught up" / "nothing in flight". "My reviewers finished" is not evidence the
   queue is empty. Make the sweep a hard precondition.
3. **Autonomous drive (#70), all three standing roles.** After ANY event (subagent completion,
   human message, timer), sweep the queue and advance every actionable item / refill empty slots
   before yielding. Idle only when the queue is genuinely empty. Make "reactive idle with
   actionable items" a visible defect, not a resting state (the desk can answer "what am I
   waiting on and why" at any moment).
4. **File-at-discovery default (#71).** The chat/PR relay is a notification of the *filed* issue
   ("filed as <repo>#<N>"), never a request for permission. Reserve "ask first" for the narrow
   carve-outs (PII/secrets/exploit → the human directly; a real decision fork → `needs-decision`).
   Treat "I'll hold this observation rather than file it" as a smell. Note the actor-path
   dependency (oit#556 / #120) so filing has a clean non-the-org identity.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci 'fresh sweep\|no idle\|precondition\|caught up' <pr-review-desk SKILL.md>` | exit 0; ≥ 1 (no-idle-without-sweep codified) |
| 2 | `grep -ci 'heartbeat\|liveness\|loud\|dead monitor' <the brief's contract doc / skill>` | exit 0; ≥ 1 (a dead monitor is detectable) |
| 3 | `grep -ci 'refill\|advance every\|autonomous\|loop until' <verify-desk SKILL.md the-desk SKILL.md>` | exit 0; ≥ 1 (autonomous drive present, all roles) |
| 4 | `grep -ci 'filed as\|file at discovery\|not.*permission' <pr-review-desk SKILL.md>` | exit 0; ≥ 1 (#71 default present) |
| 5 | confirm the durable watchdog is referenced to oit #627/#651, NOT reimplemented in a local bash loop | grep shows a pointer to the observability service, no `& disown` bash-loop primitive added |

## Verify note
Prose/skill deliverables — these are PRESENCE gates; the review gate owns whether the drive
actually loops. The durable-liveness *service* is verified in the observability stream, not here.

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date. MUST confirm the brief did NOT re-home the durable
watchdog into another laptop bash loop (the exact thing #86 rejects) — the liveness half belongs
to the always-on service.
