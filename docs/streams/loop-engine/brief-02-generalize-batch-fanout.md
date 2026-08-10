---
brief: loop-engine/02
title: Generalize — batch-fanout as the second drain-engine consumer (contract validation)
wave: 1
depends: ["loop-engine/01"]
unblocks: []
effort: M
gate: human
gate-why: cutover of the standing fanout window to an engine-driven pool that dispatches worker agents autonomously is human:<name>'s call (desk-tools C-1 pattern); implementation dispatches normally, the gate binds the cutover + sign-off.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [221]
schema: brief-v1
authored: 2026-07-19 by Fable design session (human:<name>'s fix-the-verify-loop direction)
sources: ["docs/loop-engine-architecture.md (§3 archetype A, §6 batch irreducibles, §8 step 2 — freshness-checked 2026-07-19)", ".claude/skills/batch-fanout/SKILL.md (pool-of-8, refill-on-completion, orphan-resume priority, claims protocol, F-35)", "loop-engine/01 (the contract being validated)", "issue #221 (out-of-repo serialization)"]
exec-tier: any
exec-tier-why: the contract and reference consumer exist after 01; this is an adapter implementation against a frozen interface.
why: >-
  A contract with one consumer is an implementation detail. batch-fanout is the second and
  hardest-fitting drain consumer — Land() is a near-no-op (the worker's draft PR is the
  artifact), the pool is a standing N=8 with orphan-resume priority, and dispatch tiering is
  effort × exec-tier instead of a flat floor. If §4 survives this consumer unchanged, it is
  real; if it needs a new hook, that is a design finding to file, not a hook to add.
---

# Brief 02 — batch-fanout as second engine consumer

## Context
files: `../assay-toolkit/tools/desk/cmd/fanoutloop/` (new adapter), `../assay-toolkit/tools/desk/internal/loopengine/`
(consume only — contract changes are out of scope), `../oit/.claude/skills/batch-fanout/SKILL.md`
(staged repoint, cutover human like 01's)
facts:
- SelectQueue = STATUS.md Next-up regenerated in the loop's own worktree (fresh
  `origin/main` fetch each cycle); excludes dep-incomplete briefs and `issue-loop/issue-*`
  placeholder rows (issue-loop/12 — those are brief-03's consumer); the 4-per-stream cap and
  staleness exclusion are already applied by statusgen — the adapter consumes, never
  re-implements.
- Pool = standing N=8, refill on completion, orphan-resume takes priority over fresh
  dispatch (mm/10): the orphan sweep (open PRs owing worker action >4h, no live claim) is
  part of OnIdle AND the per-cycle scan.
- Land() ≈ no-op by design: the worker's draft PR is the durable artifact; Land only clears
  the claim once branch-as-claim takes over (worker's first push) and records the handle in
  the dispatch log.
- TierPolicy = effort × exec-tier (S session-tier, M/L cheap-tier; `exec-tier: strong` →
  session-tier only, prompt carries the cheap-pickup STOP text). gate:human briefs dispatch
  normally (the gate binds approval, not implementation).
- Out-of-repo serialization (#221): a brief whose Context declares `out-of-repo files:` is
  engine-serialized — max ONE in flight across all streams; the engine checks in-flight
  claims/PRs for overlapping declarations (typed check, was prose).
- Worker-prompt essentials stay per-loop prose in the dispatch template (merge-never-rebase,
  one brief = one branch = one PR, Monitor-own-PR, F-35 no-shared-paths).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Branch + draft PR only.
- Stop at `implemented`; cutover of the standing fanout window is human:<name>'s act — stage the
  skill diff, report BLOCKED-ON-IAN at the stop-point.
- Contract freeze binds hardest here: any needed engine change → STOP, file the design
  issue, report NEEDS_CONTEXT.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `cmd/fanoutloop` adapter implementing the six hooks per facts; dispatch prompt template
   carrying the batch worker essentials verbatim from the current skill.
2. Fixture drill: pool fills to 8, refills on completion, orphan-resume preempts fresh
   dispatch, `issue-loop/issue-*` rows skipped, out-of-repo serialization refuses a second
   in-flight declaration.
3. Negative tests: claim collision with a concurrent (fixture) issue-loop claimant via the
   shared claims dir; cap-starved pool fills remaining slots with orphan resumes before
   idling a slot.
4. Stage the batch-fanout SKILL.md repoint diff (irreducibles stay) in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0 |
| 2 | `go test ./tools/desk/cmd/fanoutloop/... -run 'Pool' -count=1 -v 2>&1 \| grep -cE -e 'refill' -e 'resume-priority'` | ≥2 (standing-pool + orphan-priority both exercised) |
| 3 | `go test ./tools/desk/cmd/fanoutloop/... -run 'Serial' -count=1` | exit 0 (out-of-repo second-in-flight refused) |
| 4 | `git diff --stat origin/main -- tools/desk/internal/loopengine/ \| tail -1` | empty (contract untouched — the validation claim) |
| 5 | PR body contains the staged batch-fanout SKILL.md diff + cutover stop-point | present |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at verification time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (cutover of a standing desk window). Reviewer confirms (a) the engine contract
is byte-identical to 01's (Verify row 4 — a changed contract means this brief failed its
purpose and files the design issue instead), (b) statusgen-applied policy (caps, staleness)
is consumed, never re-implemented, (c) orphan-resume priority and out-of-repo serialization
are typed checks with negative tests, (d) gate:human briefs dispatch normally with the
BLOCKED-ON-IAN protocol in the prompt template.
