---
brief: assay:assay:desk-containers:15
title: "The cell-operation machine seam — one `--json` result contract and a plan-id → apply confirmation that a program can drive and a human still clicks"
why: >-
  The cell-level verbs this stream adds (up, drain, restart, status, doctor, logs, gc,
  upgrade) are also going to be driven by an operator window, not only typed in a terminal.
  If each verb invents its own output and its own "are you sure?", that window has to
  screen-scrape eight formats and fake a TTY to answer prompts, which removes the human from
  the confirmation. This brief fixes the seam once. Every verb gets a versioned JSON result,
  and every confirmation becomes a two-step plan → apply: the program shows the plan, a human
  approves that exact plan id, and the tool re-checks state before acting.
wave: 7
depends: []
unblocks: ["desk-containers/17", "desk-containers/18", "desk-containers/20", "desk-containers/21", "desk-containers/22", "desk-containers/24", "desk-containers/25"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver direction 2026-10-14 (relayed by the coordinator): every new cell verb must be driveable from the console cockpit, with a stable `--json` contract and a non-TTY two-step plan-id → apply form that is still a human click, re-verifies at apply, and never offers a blanket `--yes`"
  - "#2880 — the reconcile preflight's TTY-only prompt and its 'no --yes, ever' boundary; this brief gives that boundary a non-TTY form without weakening it"
  - "docs/cellctl-go-cache.md — `cellctl cache <cell> status` already emits a versioned JSON document (`schema: cell-go-cache-v1`, before/after measurements, typed outcome, could-not-check as null): the precedent this contract generalises"
  - "tools/desk/cmd/cellctl/cache.go and status.go — the existing JSON and line outputs this brief does not break"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no cross-verb result schema, no plan file, no `--plan`/`--apply` flag exists in `tools/desk/cmd/cellctl`"
exec-tier: strong
exec-tier-why: (a) the schema and plan lifecycle are design decisions six later briefs build on; (c) apply-time re-verification is safety plumbing where a stale plan acting on changed state survives happy-path tests
domain: complicated
consumers:
  - "tools/desk/internal/cellop (planned; result types, plan store, apply re-verification): follow-up desk-containers/15 (this brief; new package)"
  - "docs/cellctl-machine-seam.md (planned): follow-up desk-containers/15 (this brief)"
  - "tools/desk/cmd/cellctl verbs up/drain/restart/status/doctor/logs/gc/upgrade: follow-up desk-containers/17, desk-containers/18, desk-containers/19, desk-containers/20, desk-containers/21, desk-containers/22, desk-containers/23, desk-containers/24 (each adopts the contract in its own brief)"
  - "the console cockpit (an operator window per cell that renders results and posts plan approvals): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 15 — The cell-operation machine seam

## Context
files:
- `tools/desk/internal/cellop/` (planned) — `result.go` (the `cell-op-result` types), `plan.go` (plan store + lifecycle), `apply.go` (re-verify-then-act harness), tests
- `tools/desk/internal/cellop/testdata/cell-op-result-v1.schema.json` (planned) — JSON Schema for the result document
- `tools/desk/internal/cellop/testdata/cell-op-plan-v1.schema.json` (planned) — JSON Schema for the plan document
- `docs/cellctl-machine-seam.md` (planned) — the contract, written for a program author
- `tools/desk/cmd/cellctl/consent_flag_test.go` (planned) — the no-blanket-consent flag walk (row 6), which every later verb brief inherits
- `tools/desk/cmd/cellctl/plan_roundtrip_test.go` (planned) — the end-to-end plan → apply flow through the built binary (row 5a)
- `changelog/desk-containers-15-machine-seam.md` (planned)

facts:
- **Result document** (`schema: cell-op-result-v1`), emitted on stdout by every cell verb given `--json`, and nothing else on stdout: `verb`, `cell`, `started`/`finished` (UTC RFC 3339), `outcome` (`ok` | `partial` | `refused` | `failed` | `could-not-check`), `roles[]` (`role`, `state`, `detail`, `evidence{}`), `actions[]` (what was done, each with its own outcome), `plan_id` (when a plan was produced), and `next[]` (exact follow-up commands). Human-readable text goes to stderr. An unmeasured value is JSON `null` with a could-not-check outcome, never zero (the go-cache precedent).
- **Exit codes** are unchanged by `--json`: 0 ok, 1 failed/partial, 5 refused, 6 could-not-check.
- **Plan → apply.** Any verb whose interactive form asks a confirmation question gains `--plan`. On a TTY, `--plan` is optional and the existing prompt stands. Off a TTY, the confirmation can only be given this way:
  1. `<verb> <cell> --plan [--json]` measures and writes `run/plans/<plan-id>.json` (owner-only, mode 0600, under the cell's `run/`). It contains the verb, the exact action list, each action's **precondition evidence** (pid + process start time, checkpoint sequence, lock holder, byte counts), creation time, and a TTL of 10 minutes. It prints the id. It changes nothing else.
  2. `<verb> <cell> --apply <plan-id> --approved-by <who>` loads the plan and re-measures every precondition. If any one differs, it refuses `plan-stale` (exit 5), naming the action, and acts on nothing. Otherwise it performs exactly the planned actions, records the `--approved-by` value and the result into the plan file, and marks the plan consumed. A plan applies at most once.
- **No blanket consent.** There is no `--yes`, no `--force` that skips re-verification, and no apply without an id. `--approved-by` is recorded, not authenticated. The human click lives in the driving program, and the contract states that a driver must not call `--apply` without an explicit per-plan human action.
- Expired, consumed, malformed or foreign-cell plans are refused (exit 5) with the reason. Plans are pruned by `gc` (desk-containers/24).

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- This brief ships the package, schemas and doc. Adopting the contract in each verb belongs to that verb's brief. Do not edit existing verb output here.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the result and plan types with JSON Schemas, and a writer that guarantees stdout carries only the document.
2. Write the plan store (atomic create, 0600, TTL, single-consume) and the apply harness (re-measure through caller-supplied probes, refuse stale, then run).
3. Write `docs/cellctl-machine-seam.md` (planned): both documents, the exit codes, the plan lifecycle, the rule against apply without a human action, and a worked plan → apply example.
4. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./internal/cellop/ -count=1` | exit 0 |
| 2 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyRefusesStale' -count=1` | exit 0 — a plan whose recorded pid + start time no longer matches is refused `plan-stale` and none of its actions run, including actions whose preconditions still hold |
| 3 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestPlanSingleUseAndTTL' -count=1` | exit 0 — a second apply of the same id, an apply after TTL, and an apply with another cell's id are each refused exit 5 |
| 4 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestStdoutOnlyDocument' -count=1` | exit 0 — a verb harness that logs progress still leaves stdout parseable as exactly one schema-valid document |
| 5 | check +dereference | `go -C tools/desk test ./internal/cellop/ -run 'TestDocExampleValidates' -count=1` | exit 0 — the worked example JSON in `docs/cellctl-machine-seam.md` (planned) is extracted and validated against the shipped schemas, so the doc cannot drift from the code |
| 5a | check +flow | `go -C tools/desk test ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1` | exit 0 — a fixture verb run through the built `cellctl` binary writes a plan with `--plan --json`, a second process applies it with `--apply <id>`, and a third apply of the same id is refused: CLI → plan store → apply harness → result document end to end |
| 6 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestNoBlanketConsentFlag' -count=1` | exit 0 — walks every registered cellctl command and fails if any defines a `yes`, `y`, `assume-yes` or `skip-verify` flag |
| 7 | check | `statusgen --root . --lint` | exit 0 |
| 7a | check | `statusgen --root . --consumers --brief desk-containers/15` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: the package acts on nothing by itself. The reviewer checks that rows 2 and 3 fail against a mutant that skips re-measurement.
