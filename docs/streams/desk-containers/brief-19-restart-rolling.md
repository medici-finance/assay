---
brief: assay:assay:desk-containers:19
title: "`cellctl restart <cell> [role...]` — CLI adapter onto the runner-lifecycle restart sequence (drain → stop → start per role, rolling), so the pipeline never fully stops"
why: >-
  Restarting desks is routine: a new tool release, a changed model pin, a session that has
  run out of context. Today it is a full stop of every window followed by a hand-run recovery.
  A rolling restart cycles one desk at a time. The others keep reviewing, verifying and
  dispatching, and each restarted desk starts from the handoff record its predecessor wrote.
  Restart is a sequence of runner-lifecycle transitions, not a mechanism of its own, so the
  verb is a thin adapter: it composes `drain` and `up` and inherits whatever implements
  those transitions, interim today and the lifecycle contract later.
wave: 10
depends: ["desk-containers/17", "desk-containers/18"]
unblocks: ["desk-containers/23"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-14: a `restart <cell>` that cycles the desks; the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge): restart is a runner-lifecycle sequence and cellctl is an adapter"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) — restart = drain → stop → start per role, defined there as a lifecycle sequence"
  - "desk-containers/17 — the converging `up --role`; desk-containers/18 — `drain <cell> [role...]`, `--now`, and the handoff record"
  - "freshness-checked 2026-10-14 @ caca38a58 (origin/main) — no restart verb; the nearest is `reset`, which also removes worktrees and caches (#2670)"
exec-tier: any
domain: clear
consumers:
  - "the runner lifecycle and ownership contract (target owner of the restart sequence): out-of-scope (graph-execution R2/R3; this verb is an adapter and needs no change when the mechanism moves)"
  - "tools/desk/cmd/cellctl (new restart verb): follow-up desk-containers/19 (this brief)"
  - "docs/cellctl.md: follow-up desk-containers/19 (this brief)"
  - "the console cockpit (renders per-role and whole-cell restart results and plans only; never applies): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 19 — `cellctl restart`, rolling

## Context
files:
- `tools/desk/cmd/cellctl/restart.go` (planned), `restart_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `restart`
- `docs/cellctl.md`
- `changelog/desk-containers-19-restart.md` (planned)

facts:
- `restart <cell> [role...]` runs, for each targeted role in `ROLES` order with the-desk LAST: `drain <cell> <r>` (18's positional role), then `up <cell> --role <r>` (17), and waits for that role's `up` to report `up` before moving on.
- `--all-at-once` drains every targeted role together and then runs one `up`, for when the pipeline should stop.
- **`restart --now` plan shape: one batch plan over per-role plans.** `restart --now --plan` writes one `cell-op-plan` per role, each with that role's own precondition evidence (its in-flight claims, which `drain --now` would abandon), plus a batch plan listing the ordered role plan ids and digests.
  - Applying is one interactive confirm of the batch's code. The batch plan is consumed atomically **before** the first role runs, so a concurrent second apply of the same batch is refused `already-consumed`.
  - At each step, immediately before that role's `drain --now`, its role plan is re-measured and consumed. A drifted role is refused `plan-stale`, and a role plan found already consumed stops the sequence; the sequence never skips ahead.
  - The batch expiry bounds the **start** of each step. A step not started within 10 minutes is not run; the result is `partial`, and `next[]` names a fresh `restart --now --plan` for the remaining roles.
- If a role fails to come back, restart stops there. It does not restart further roles, it reports the failed role with `up`'s error, and it exits 1. The roles not yet touched keep running.
- **Off a TTY, `restart` in every form except `--plan` is refused (plan-only):** `restart` and `restart --now` are refused `not-interactive` (desk-containers/15 row 8), and `restart --now --plan` works anywhere.
- **Per-role stop.** Each step drains one role with brief 18's `drain <cell> <role>`, which ends in the per-role `down <cell> <role>`, then starts it with `up --role`. The per-role stop re-verifies the window's identity before closing it and refuses on a mismatch or stale receipt, so siblings' windows, cadences and work are not touched. That is what lets row 3's "at least four of five live on every poll" hold. An interactive-mode role (the default) measures harness idleness as could-not-check, so `restart` stops it only through the operator's interactive confirm, which states that. The cell-wide `down <cell>` is never used.
- It is a composition of lifecycle transitions through `drain` and `up` and adds no state or mechanism of its own; the interim mechanism belongs to desk-containers/18. Output is a desk-containers/15 result with one `roles[]` entry per role, carrying both phases. A `--now` restart is applied only by interactive confirm; off a TTY it is plan-only.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- Compose `drain` and `up`; do not re-implement either.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `restart` as above; register it; docs; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestRestartRolling' -count=1 -v` | `--- PASS` — the recorded call order is drain r1, up r1, drain r2, up r2, …, the-desk last; at no point are two roles drained together |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestRestartStopsOnFailure' -count=1 -v` | `--- PASS` — when role 2's `up` fails, roles 3+ are never drained and the result names role 2's error |
| 2b | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestRestartNowBatchPlan' -count=1 -v` | `--- PASS` — `--now --plan` writes per-role plans plus a batch plan binding their digests; two concurrent applies of the batch: exactly one proceeds; a drifted role is refused `plan-stale` and later roles are untouched; a consumed role plan stops the sequence (never skip-and-continue); a step not started before expiry is not run (`partial`); a mutant that skips consumed roles fails |
| 2c | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestRestartRefusesOffTTY' -count=1 -v` | `--- PASS` — `restart <cell>` and `restart <cell> --now`, each with stdin or stdout not a TTY, are refused `not-interactive`, and no role is drained |
| 3 | gate:model +flow +dereference | On a disposable house cell with stub desks (cadence and interactive mode): `cellctl restart <cell>` on a TTY, confirming, while polling every 2 s each role's liveness (the harness pid and start time from its surface receipt, alive) | every poll shows at least four of five roles live; no poll shows a role other than the one being restarted down; the run ends with all five live and a fresh handoff record read by each. `running=true` is not used as liveness, because it marks a cadence pass in flight and holds for an interactive role's whole life |
| 4j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestRestartEmitsCellOpV1' -count=1 -v` | `--- PASS` — `restart --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 4 | check | `statusgen --root . --lint` | exit 0 |
| 4a | check | `statusgen --root . --consumers --brief desk-containers/19` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no. The reviewer checks that row 3 shows the pipeline never fully stopped.
