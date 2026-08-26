---
brief: desk-tools/02
title: Generalize — batch-fanout as the second drain-engine consumer (contract validation)
wave: 1
depends: []
unblocks: []
effort: M
gate: human
gate-why: >-
  Cutover of the standing fanout window to an engine-driven pool that dispatches worker agents
  autonomously is a human's call; implementation dispatches normally, and the gate binds the
  cutover plus sign-off, not the implementation.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-19 by Fable design session; re-homed to the desk-tools board 2026-08-26
sources:
  - "The drain-engine architecture and its batch irreducibles — the six-hook contract this brief validates against a second consumer."
  - "The worker-desk role: a standing pool-of-8, refill-on-completion, orphan-resume priority, and the claims protocol the pool obeys."
exec-tier: any
exec-tier-why: the contract and reference consumer exist already; this is an adapter implementation against a frozen interface.
why: >-
  A contract with one consumer is an implementation detail. batch-fanout is the second and
  hardest-fitting drain consumer — Land() is a near-no-op (the worker's draft PR is the
  artifact), the pool is a standing N=8 with orphan-resume priority, and dispatch tiering is
  effort × exec-tier instead of a flat floor. If the engine contract survives this consumer
  unchanged, it is real; if it needs a new hook, that is a design finding to file, not a hook
  to add.
---

# Brief 02 — batch-fanout as second engine consumer

## Dependencies
The drain engine and its reference consumer this originally depended on have landed outside
this stream (done + reviewed), so no typed `depends:` edge remains. The engine contract is
FROZEN for this brief: any needed engine change is a STOP + design finding, never a hook added
here.

## Context
files: `tools/desk/cmd/fanoutloop/` (new adapter), `tools/desk/internal/loopengine/` (consume
only — contract changes are out of scope), plus the batch-fanout skill (staged repoint, cutover
human).

facts:
- SelectQueue = the Next-up board regenerated in the loop's own worktree (fresh `origin/main`
  fetch each cycle); excludes dep-incomplete briefs and issue-loop placeholder rows; the
  4-per-stream cap and staleness exclusion are already applied upstream — the adapter consumes,
  never re-implements.
- Pool = standing N=8, refill on completion, orphan-resume takes priority over fresh dispatch:
  the orphan sweep (open PRs owing worker action >4h, no live claim) is part of OnIdle AND the
  per-cycle scan.
- Land() ≈ no-op by design: the worker's draft PR is the durable artifact; Land only clears the
  claim once branch-as-claim takes over (the worker's first push) and records the handle in the
  dispatch log.
- TierPolicy = effort × exec-tier (S session-tier, M/L cheap-tier; `exec-tier: strong` →
  session-tier only, prompt carries the cheap-pickup STOP text). gate:human briefs dispatch
  normally (the gate binds approval, not implementation).
- Out-of-repo serialization: a brief whose Context declares `out-of-repo files:` is
  engine-serialized — max ONE in flight across all streams; the engine checks in-flight
  claims/PRs for overlapping declarations (a typed check, not prose).
- Worker-prompt essentials stay per-loop prose in the dispatch template (merge-never-rebase,
  one brief = one branch = one PR, Monitor-own-PR, no-shared-paths).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Branch + draft PR only.
- Stop at `implemented`; cutover of the standing fanout window is the human's act — stage the
  skill diff, report the cutover stop-point.
- Contract freeze binds hardest here: any needed engine change → STOP, file the design issue,
  report NEEDS_CONTEXT.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `cmd/fanoutloop` adapter implementing the six hooks per facts; dispatch prompt template
   carrying the batch worker essentials verbatim from the current skill.
2. Fixture drill: pool fills to 8, refills on completion, orphan-resume preempts fresh dispatch,
   issue-loop placeholder rows skipped, out-of-repo serialization refuses a second in-flight
   declaration.
3. Negative tests: claim collision with a concurrent (fixture) issue-loop claimant via the
   shared claims dir; cap-starved pool fills remaining slots with orphan resumes before idling
   a slot.
4. Stage the batch-fanout skill repoint diff (irreducibles stay) in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0 |
| 2 | `go test ./tools/desk/cmd/fanoutloop/... -run 'Pool' -count=1 -v 2>&1 \| grep -cE -e 'refill' -e 'resume-priority'` | ≥2 (standing-pool + orphan-priority both exercised) |
| 3 | `go test ./tools/desk/cmd/fanoutloop/... -run 'Serial' -count=1` | exit 0 (out-of-repo second-in-flight refused) |
| 4 | `git diff --stat origin/main -- tools/desk/internal/loopengine/ \| tail -1` | empty (contract untouched — the validation claim) |
| 5 | PR body contains the staged batch-fanout skill diff + cutover stop-point | present |
| 6 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

## Evidence
<!-- appended at verification time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (cutover of a standing desk window). Reviewer confirms (a) the engine contract is
byte-identical to the reference consumer's (Verify row 4 — a changed contract means this brief
failed its purpose and files the design issue instead), (b) upstream-applied policy (caps,
staleness) is consumed, never re-implemented, (c) orphan-resume priority and out-of-repo
serialization are typed checks with negative tests, (d) gate:human briefs dispatch normally with
the cutover stop-point protocol in the prompt template.
