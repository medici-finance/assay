---
brief: desk-supervision/03
title: Eligibility reconciliation — stop a run whose item became ineligible
why: >-
  "Merged or closed PR = done, stop, never push its branch again" is prose. A worker whose
  PR a human merged mid-run keeps working, and the recorded failures are re-pushes to merged
  branches and resumes of already-merged PRs read as drafts. Symphony refreshes tracker
  state for every running issue each tick and terminates the ineligible ones; the observer
  now has the tick and the stop primitive, so the rule can become a mechanical check that
  fires within one observer interval instead of a sentence a model has to remember.
wave: 2
depends: ["desk-supervision/01", "desk-supervision/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §8.5 part B (tracker state refresh: terminal ⇒ terminate + clean; active-but-not-routable ⇒ terminate without cleanup; refresh failure ⇒ keep running, retry next tick) — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "tools/desk/internal/loopengine/engine.go — measured 2026-09-02: no reconcile step; a running handle is only ever landed by its own result, the liveness timers, or the 120m StaleClaim backstop"
  - "tools/desk/internal/deskkit/prstate.go and disposition.go — the existing PR-state and disposition reads (merged / closed / SUPERSEDED / RESOLVED-ELSEWHERE) the check reuses"
  - "freshness-checked 2026-09-02 @ 30c9934"
exec-tier: strong
exec-tier-why: >-
  (b) and (c): the eligibility verdict joins forge PR state, the board row on origin/main
  and the claim record; a false "ineligible" stops live work and a false "eligible" is the
  status quo. Could-not-check must stay a no-op.
consumers:
  - "tools/desk/cmd/desksupervise/main.go tick: fixed-here (reconciliation is a step of the existing tick, not a second loop)"
  - "plugins/assay/skills/worker-desk/SKILL.md 'merged or closed PR = DONE' invariant: fixed-here (one sentence noting the mechanical backstop; the rule text itself stays)"
  - "tools/desk/cmd/deskwt (workspace cleanup on terminal items): out-of-scope (deskwt prune already removes fully-merged, clean worktrees on its own interval; reconciliation releases the claim and stops the run, it never deletes a worktree)"
---

# Brief 03 — Eligibility reconciliation

## Context

files:
- `tools/desk/internal/loopengine/reconcile.go` (new) — `Eligibility(claim) (Verdict,
  error)` joining three reads; pure over injected readers so it tests offline.
- `tools/desk/internal/loopengine/reconcile_test.go` (new).
- `tools/desk/cmd/desksupervise/main.go` (planned) — `tick` runs reconciliation for every
  `state=dispatched` claim before the liveness evaluation; new output class
  `INELIGIBLE(<reason>)` with `action=STOP+RELEASE`.
- `tools/desk/cmd/desksupervise/testdata/` — fixtures: merged PR, closed PR, brief row
  no longer `in-progress`/`todo` on origin/main, claim released underneath, forge
  unreachable.

single-point-of-failure: the forge PR-state read — behind it, the board row read from
`origin/main` (a brief that flipped to `implemented`/`verified`/`done` is ineligible even
when the PR read fails) and the claim record itself (a claim someone released or stole is
ineligible for THIS holder regardless of the PR).

facts:
- Ineligible, terminal (stop + release + journal `SUPERSEDED`): the recorded PR is
  `MERGED` or `CLOSED`; the item's board row on `refs/remotes/origin/main` is not in an
  active status (`todo`, `in-progress`); the claim ref no longer names this holder.
- Ineligible, non-terminal (stop, do NOT release — a human or another desk owns the next
  move): the PR carries a disposition of `SUPERSEDED` / `RESOLVED-ELSEWHERE`, or a
  `question` / `needs-decision` label was added after dispatch.
- Could-not-check on ANY read ⇒ keep the run, log `BLIND(<source>)`, retry next tick —
  Symphony's "if state refresh fails, keep workers running" rule, which is also this
  house's three-state rule.
- The observer never deletes a worktree; cleanup stays with `deskwt prune`'s own rules
  (tracked-clean AND fully merged), which a merged PR's worktree satisfies on its next
  sweep.
- Stop and release use brief 02's `ArmRunStop` and brief 01's release path; this brief
  adds no new write primitive.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `reconcile.go`: `type EligibilityReaders struct { PR func(repo string, n int)
   (PRState, error); BoardRow func(root, item string) (Status, error); Claim func(key
   string) (ClaimRecord, error) }` and `Eligibility(c ClaimRecord, r EligibilityReaders)
   (EligibilityVerdict, error)` returning one of `Eligible`, `IneligibleTerminal(reason)`,
   `IneligibleHeld(reason)`, or an error for could-not-check. Order the reads cheapest
   first (claim, board row, PR) and short-circuit on the first ineligible reading.
2. `desksupervise tick`: reconciliation runs first; an `INELIGIBLE` claim is stopped and
   (terminal only) released, journalled as `SUPERSEDED reason=<...>`, and skipped by the
   liveness step. `--dry-run` prints without acting.
3. Fixtures + tests for each reason and for each could-not-check source.
4. Worker-desk skill: one sentence under the invariants noting the mechanical backstop.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/loopengine/ -run 'Eligib\|Reconcile' -count=1` | exit 0; output contains `ok` |
| 2 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/pr-merged.json --observations-fixture cmd/desksupervise/testdata/alive-worker-obs.json` | exit 0; output contains `INELIGIBLE(pr-merged)` and `action=STOP+RELEASE` |
| 3 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/pr-closed.json --observations-fixture cmd/desksupervise/testdata/alive-worker-obs.json` | exit 0; output contains `INELIGIBLE(pr-closed)` |
| 4 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/brief-flipped.json --observations-fixture cmd/desksupervise/testdata/alive-worker-obs.json` | exit 0; output contains `INELIGIBLE(board-row-` |
| 5 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/needs-decision.json --observations-fixture cmd/desksupervise/testdata/alive-worker-obs.json` | exit 0; output contains `INELIGIBLE(needs-decision)` and `action=STOP`; output does not contain `RELEASE` |
| 6 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/forge-unreachable.json --observations-fixture cmd/desksupervise/testdata/alive-worker-obs.json; echo rc=$?` | output contains `BLIND(pr)` and `rc=6`; output does not contain `INELIGIBLE` and does not contain `STOP` |
| 7 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/alive-worker.json --observations-fixture cmd/desksupervise/testdata/alive-worker-obs.json` | exit 0; output contains `ALIVE`; output does not contain `INELIGIBLE` (positive control: an eligible live run is untouched) |
| 8 | `grep -c 'desksupervise' plugins/assay/skills/worker-desk/SKILL.md` | output is `1` or more |
| 9 | `statusgen --root . --consumers --brief desk-supervision/03` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "stops a live run because the forge blinked" → row 6; "releases a
claim on a human-held PR so a fresh worker re-dispatches into a needs-decision" → row 5;
"a brief flipped by the verifier is still worked" → row 4; "reconciliation runs but the
happy path regresses" → row 7. Review-only: the exact label set that counts as held.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
