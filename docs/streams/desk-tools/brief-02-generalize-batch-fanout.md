---
brief: assay:assay:desk-tools:02
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
schema: brief-v2
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
version: 1
id: edc0d376-9699-4fa4-aa64-d10c8006f318
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
| 1 | `cd tools/desk && GOWORK=off go test ./... -count=1` | exit 0 |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/... -run 'Pool' -count=1 -v 2>&1 \| grep -cE -e 'refill' -e 'resume-priority'` | ≥2 (standing-pool + orphan-priority both exercised) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/... -run 'Serial' -count=1` | exit 0 (out-of-repo second-in-flight refused) |
| 4 | `git diff --stat origin/main -- tools/desk/internal/loopengine/ \| tail -1` | empty (contract untouched — the validation claim) |
| 5 | PR body contains the staged batch-fanout skill diff + cutover stop-point | present |
| 6 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

## Evidence
<!-- appended at verification time by a NON-implementer: one row per Verify item. -->

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian))

VERIFY: BLOCKED — 4/6 pass, 1 could-not-check (environment), 1 could-not-check (check-definition), 0 implementation fail.
Non-implementer run from a worktree cut detached at merged main 893cd6114b0382a1f71e6ef763c6601a5e19d270,
darwin/arm64, offline (KUBECONFIG=/dev/null). Execution witness below is the verbatim output of
`statusgen verifyrun --brief` (statusgen built from this tree); the hand-written table after it records the
direct runs and the reading of each row. gate: human — Evidence only; the status stays `implemented` and the
sign-off belongs to the human gate.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./... -count=1` | could-not-run exit=- — timed out after 10m0s — no verdict was produced | sha256:e1a0c49f19ad | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/... -run 'Pool' -count=1 -v 2>&1 \| grep -cE -e 'refill' -e 'resume-priority'` | pass exit=0 | sha256:06e9d52c1720 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/... -run 'Serial' -count=1` | pass exit=0 | sha256:116175433f83 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git diff --stat origin/main -- tools/desk/internal/loopengine/ \| tail -1` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 5 | `PR body contains the staged batch-fanout skill diff + cutover stop-point` | fail exit=1 | sha256:41dd5011f49f | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 | sha256:9b9e71858974 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |

Hand-written rows (direct, non-hermetic runs at the same tree):

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./... -count=1` | exit 0 | COULD-NOT-CHECK (environment) — the witness row timed out at its 10m limit on a host at load average ~50 (many concurrent test runs). Direct run: exit 1 after 8m40s — 83 packages ok, 2 FAIL, both 5-second wedge-deadline assertions: loopengine TestDrain ("engine did not stop within deadline") and commsloop TestRunDoesNotBusySpinOnEmptyQueue ("loopengine.Run did not stop within deadline"). Re-running exactly those two packages in isolation: exit 0 (loopengine ok 13.5s, commsloop ok 9.6s). Matches the tracked load-induced timing flake (#612, #1232); no fanoutloop failure | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/... -run 'Pool' -count=1 -v 2>&1 \| grep -cE -e 'refill' -e 'resume-priority'` | ≥2 | PASS — exit 0, count 6 (≥2). Key lines: "refill: a standing pool of 2 drained 5 items by refill-on-completion"; "resume-priority: the orphan resume preempted the fresh brief (order=[resume:pr-1234 fresh/01])"; the fills-to-standing-pool-of-8 subtest holds exactly 8 in flight under 12 eligible | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/... -run 'Serial' -count=1` | exit 0 | PASS — exit 0, "--- PASS: TestSerial" (second out-of-repo in-flight declaration refused) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `git diff --stat origin/main -- tools/desk/internal/loopengine/ \| tail -1` | empty | PASS as written — exit 0, empty output. Caveat for the gate: post-merge this compares merged main against itself, so it is vacuous here. The claim it stands for (the adapter added no engine hook) holds on reading: the out-of-repo serialization rides the engine's existing WorkEvidence config seam (adapter.go workEvidence) and the adapter implements only the existing Loop methods. The loopengine package has since changed through later briefs (2817 insertions since the home-handoff import 8a36708d3); those are not attributable to this brief | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | PR body contains the staged batch-fanout skill diff + cutover stop-point | present | COULD-NOT-CHECK (check-definition) — the row is prose, not a command; the witness executed it as a shell line and recorded fail exit=1, which is not an observation about the PR body. The delivering PR predates this repository's public tree (the code arrived with the home-handoff import 8a36708d3), so its body is not readable from this repository. Suggested amendment: rewrite row 5 as a check that runs in this tree, or mark it a human-read row naming where the body lives | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | PASS — exit 0, no PROBLEM line | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Risk-bearing values (enumerated over the fanoutloop adapter: adapter.go, tier.go, board.go, plus the worker-desk width policy it reads; risk metadata: irreversible no, all flags no — every entry below is a reversible operational knob, fixable by an edit and a redeploy):

- RISK-VALUE: DERIVED — out-of-repo in-flight cap = 1 (any other in-flight out-of-repo id makes the item taken) @ tools/desk/cmd/fanoutloop/adapter.go:454-455 — the brief's facts pin "max ONE in flight across all streams"; the loop returns taken on the first other id, which is exactly a cap of one. Wrong-value effect: concurrent writers to the same out-of-repo files; reversible.
- RISK-VALUE: DERIVED — TierPolicy effort S = TierSession @ tools/desk/cmd/fanoutloop/tier.go:42-43; effort M/L/unspecified = TierCheap @ tier.go:44-45; exec-tier strong = TierSession @ tier.go:35-36 — the brief's facts pin "S session-tier, M/L cheap-tier; exec-tier strong session-tier only". Unspecified-to-cheap is the adapter's own fail-safe (never silently promote unknown work). Wrong-value effect: cost leak or under-powered pickup; reversible.
- RISK-VALUE: DERIVED — orphan resume = TierCheap @ tools/desk/cmd/fanoutloop/tier.go:38-39 — not in the brief's facts; the adapter's stated reason is that a resume is bounded rework against stated findings. Reversible.
- RISK-VALUE: DERIVED — worker-desk Default width = 8 @ tools/desk/internal/deskkit/width.go:211 — the brief's facts pin "Pool = standing N=8"; the width policy now lives in the roster table (moved there after this brief), bounded by the per-PR write budget. Related entries not introduced by this brief: DeclaredMax = 12 @ width.go:212 and DefaultReserve resume = 2 @ width.go:217 (added by a later brief). Reversible.
- RISK-VALUE: NAMED, NOT DERIVED — orphan age threshold = >4h @ tools/desk/cmd/fanoutloop/adapter.go:50 (and board.go:129) — OPEN QUESTION for the gate: the value exists only in comments and in the brief's facts; the code does not enforce it, because the orphan source returns nil in the offline reference build (adapter.go:481, "wired at cutover"). No source says why 4 hours is right, and there is no literal in code to derive against until the cutover wires the orphan sweep.

## Review
Gate: human (cutover of a standing desk window). Reviewer confirms (a) the engine contract is
byte-identical to the reference consumer's (Verify row 4 — a changed contract means this brief
failed its purpose and files the design issue instead), (b) upstream-applied policy (caps,
staleness) is consumed, never re-implemented, (c) orphan-resume priority and out-of-repo
serialization are typed checks with negative tests, (d) gate:human briefs dispatch normally with
the cutover stop-point protocol in the prompt template.
