---
brief: loop-engine/03
title: issue-loop dispatch lane as third engine consumer — intake triage stays prose
wave: 1
depends: ["loop-engine/01"]
unblocks: []
effort: M
gate: human
gate-why: cutover of the standing issue window's dispatch lane to an engine driver is human:<name>'s call (desk-tools C-1 pattern); implementation dispatches normally, the gate binds the cutover + sign-off.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-19 by Fable design session (human:<name>'s fix-the-verify-loop direction)
sources: ["docs/loop-engine-architecture.md (§3 archetype split inside issue-loop, §8 step 3 — freshness-checked 2026-07-19)", ".claude/skills/intake-desk/SKILL.md (issue lane vs intake lane; claims-locked placeholder fan-out; desk-automation marker protocol)", "tools/statusgen --scan-issues + tools/desk/cmd/issueboard (the deterministic selects)", "loop-engine/01"]
exec-tier: any
exec-tier-why: adapter against a frozen contract with two existing consumers as reference.
why: >-
  issue-loop is two lanes in one skill: the issue-DISPATCH lane is a textbook drain (fresh
  placeholders → claims-locked fan-out → placeholder retire) and belongs on the engine; the
  intake-TRIAGE lane is heterogeneous judgment (four exits, reasons required) and must NOT
  be mechanized. A third consumer with a different SelectQueue (issueboard, not statusgen)
  and a different Land (placeholder lifecycle, not Evidence) completes the contract's
  validation across every drain-shaped loop.
---

# Brief 03 — issue-loop dispatch lane on the engine

## Context
files: `../assay-toolkit/tools/desk/cmd/issueloop/` (new adapter for the DISPATCH lane only),
`../oit/.claude/skills/intake-desk/SKILL.md` (staged repoint of the dispatch lane; intake lane
untouched; also the in-repo home for the stop-flag block — see facts)
facts:
- SelectQueue = issueboard fresh-placeholder ACTIONs (`statusgen --scan-issues` /
  `issueboard` output — one computed ACTION per open issue; the adapter consumes ACTIONs,
  never re-derives them from `gh`).
- Claim namespace: `issue-loop--issue-<NN>.claim` in the shared claims dir — the same dir
  batch-fanout checks, so the engine-owned claim is what makes the issue-loop/12
  double-dispatch guard structural for both desks.
- Land() = placeholder lifecycle: worker opened a fix PR → placeholder rides until the
  bugs/<N>.md close-PR flow retires it; worker posted a blocking question → placeholder gets
  `blocked: awaiting-issue-response` + `blockedAt` (the `<!-- desk-automation -->` marker
  protocol stays in the dispatch prompt template).
- The blocked-placeholder UNBLOCK scanner and needs-decision watch are archetype B
  (board-reactor) — out of scope here; they stay on their current path until brief-05's
  reactor driver exists to host them.
- Intake triage (four exits, reasons, needs-decision authoring) is archetype C: stays prose,
  explicitly untouched by this brief.
- Drift repair carried by this brief (from the guardrail audit, arch doc §5): the in-repo
  issue-loop SKILL.md lacks the stop-flag boot/iteration block the other four skills carry
  (it lives only in a user-level delta today). The engine gives the lane Guard() for free;
  the staged skill edit adds the canonical in-repo block for the prose remainder. If
  brief-04 has already landed its single-home pointer, point instead of restating.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Branch + draft PR only.
- Stop at `implemented`; cutover of the standing issue window is human:<name>'s act — stage the skill
  diff, report BLOCKED-ON-IAN at the stop-point.
- Contract freeze: engine changes are out of scope — STOP and file a design issue if the
  lane doesn't fit.
- Intake lane is out of bounds: no triage mechanization, no shared-engine hooks for it.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `cmd/issueloop` adapter: SelectQueue off issueboard ACTIONs; Claim in the
   `issue-loop--issue-<NN>` namespace; TierPolicy = triage-adjacent dispatch smart-only;
   Dispatch prompt = current placeholder-worker prompt (marker protocol, blocked-state
   frontmatter steps) as the template; Land per facts.
2. Fixture drill: fresh placeholders drain concurrently; a blocked-question result lands as
   frontmatter mutation not a retire; a batch-fanout fixture claimant loses the claim race
   cleanly.
3. Stage the SKILL.md edit: dispatch lane repoint + the in-repo stop-flag block (or pointer,
   per facts); intake lane diff must be EMPTY.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0 |
| 2 | `go test ./tools/desk/cmd/issueloop/... -run 'Claim' -count=1` | exit 0 (cross-desk claim race: exactly one winner, loser skips cleanly) |
| 3 | `go test ./tools/desk/cmd/issueloop/... -run 'Blocked' -count=1 -v 2>&1 \| grep -c 'awaiting-issue-response'` | ≥1 (blocked landing is a frontmatter mutation, not a retire) |
| 4 | `git diff --stat origin/main -- tools/desk/internal/loopengine/ \| tail -1` | empty (contract untouched) |
| 5 | Staged SKILL.md diff shows dispatch-lane + stop-flag changes ONLY (intake-lane hunks: none) | present in PR body |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at verification time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (standing-window cutover). Reviewer confirms (a) contract untouched (row 4),
(b) intake triage is verifiably out of the diff (row 5), (c) the claim namespace matches
what batch-fanout's skip/claim logic expects so the issue-loop/12 guard is now structural,
(d) the unblock scanner was NOT drain-ified (it waits for brief-05's reactor), (e) the
stop-flag drift repair lands in the in-repo single home.
