---
brief: assay:assay:desk-containers:24
title: "`cellctl gc <cell>` — one housekeeping verb, plan-first: cache, scratch, worktrees, stale claims, orphaned processes, expired plans"
why: >-
  A cell accumulates residue: a go cache that only shrinks under pressure, scratch task
  directories, worktrees of finished work, dispatch claims whose holder died, background loops
  that outlived their session, and old operation plans. Each has its own cleanup command today,
  and the operator has to know all of them. `gc` measures all of it, shows one plan with the
  bytes and items it would reclaim, and runs only what the operator confirms at the terminal.
wave: 13
depends: ["desk-containers/15", "desk-containers/21"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) — classifies this verb as an interim bridge and names its target owner"
  - "Incident 2026-10-14: 70.5 GB of go cache, ten orphaned background loops (some three days old), and held claims waiting out their TTL, each cleaned by a different hand-run command"
  - "existing per-item instruments: `cellctl cache <cell> clean`; `cellctl scratch <cell> sweep`; `deskwt prune`; `desksupervise tick` (reclaim-eligible dispatch claims); #2880's marker-only orphan stop; desk-containers/15's plan store"
  - "#2879 — build-cache age trim and per-record recovery; gc picks them up through `cache clean` when they land"
  - "desk-containers/21 — doctor's classification, which gc reuses so 'safe to reclaim' means the same thing in both verbs"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no gc verb"
exec-tier: strong
exec-tier-why: (c) gc releases claims and stops processes; a misclassified live item is the error that survives fixture tests
domain: complicated
consumers:
  - "target owner — continuing operations — the workspace-lifecycle-completion slice and the single maintenance owner (retirement, maintenance cadence): out-of-scope (this verb is an interim bridge on today's tools, per docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
  - "tools/desk/cmd/cellctl (new gc verb): follow-up desk-containers/24 (this brief)"
  - "docs/cellctl.md: follow-up desk-containers/24 (this brief)"
  - "the console cockpit (a housekeeping panel over `gc --plan`): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 24 — `cellctl gc <cell>`

## Context
files:
- `tools/desk/cmd/cellctl/gc.go` (planned), `gc_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `gc`
- `docs/cellctl.md`
- `changelog/desk-containers-24-gc.md` (planned)

facts:
- **Interim bridge.** gc is an interim bridge composing today's per-item instruments. In the target, resource retirement and storage reclamation belong to the workspace-lifecycle slice of continuing operations, with one maintenance owner per common Git store and resumable retirement. gc must not become a second maintenance owner: it stays a plan-first operator trigger over whatever that owner exposes.
- Plain `gc <cell>` is the plan only (dry-run): per category, the items, the bytes, and the instrument that would run. Categories:
  - go cache, via `cache clean`;
  - scratch, via `scratch sweep`;
  - worktrees of merged or closed work, via `deskwt prune`;
  - dispatch claims that `desksupervise` classes reclaim-eligible;
  - orphaned marked processes, via #2880;
  - expired or consumed desk-containers/15 plans.
- Applying: `gc <cell> --run` shows the plan. Applying is an interactive-terminal confirmation only (desk-containers/15): the verb shows the plan and the operator types its confirm code. That is a convenience confirm, not an authorization control. Off a TTY, gc is plan-only: `gc --plan` writes the plan, and `gc --run` and `gc --apply` (state-changing) are refused `not-interactive` (desk-containers/15 row 8). `--only <category>` (repeatable) narrows the run.
- Every item is re-classified at apply; anything now alive or could-not-check is skipped and reported. gc never deletes a worktree with uncommitted changes or an unmerged, unpushed branch (the `deskwt` rule), and never releases a claim whose session is alive.
- The result reports bytes reclaimed (logical, as `cache` reports them) and items per category, plus skipped items with reasons.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- gc calls the existing instruments; it adds no deletion logic of its own beyond plan pruning.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement plan collection over the instruments' dry-run modes and `cellop.Inventory`.
2. Implement `--run` / plan-apply with per-item re-classification.
3. Docs; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestGCPlan' -count=1 -v` | `--- PASS` — plain `gc` lists all six categories and leaves the file tree hash unchanged |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestGCSkipsAliveAtApply' -count=1 -v` | `--- PASS` — a claim whose session comes alive between plan and apply is skipped and reported, not released |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestGCKeepsDirtyWorktree' -count=1 -v` | `--- PASS` — a worktree with an uncommitted file survives `gc --run` |
| 4 | gate:model +flow +dereference | On a disposable house cell seeded with an over-budget cache, a dead claim, a merged-branch worktree and a marked orphan: `cellctl gc <cell>`, then `cellctl gc <cell> --run` on a TTY, type the plan's confirm code | the plan lists every seeded item; after the run, `cellctl doctor <cell>` reports none of them; record bytes reclaimed |
| 3b | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestGCRunRefusesOffTTY' -count=1 -v` | `--- PASS` — `gc <cell> --run` with stdin or stdout not a TTY is refused `not-interactive`, and nothing is reclaimed |
| 5j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestGcEmitsCellOpV1' -count=1 -v` | `--- PASS` — `gc --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 5 | check | `statusgen --root . --lint` | exit 0 |
| 5a | check | `statusgen --root . --consumers --brief desk-containers/24` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: every action is an existing instrument's, re-verified per item at apply, and work in flight or uncommitted is never touched. The reviewer checks rows 2 and 3.
