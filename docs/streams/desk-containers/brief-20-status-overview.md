---
brief: assay:assay:desk-containers:20
title: "`cellctl status <cell>` as a cell overview — per-role liveness, model, heartbeat, claims, stop state, plus comms, cache, locks and tool version"
why: >-
  Answering "what is my cell doing?" today takes six commands and a process listing: cadence
  status, comms checkpoints, cache status, lock holders, the dispatch claims, and which
  binary is installed. One `status` that answers all of it, and is also the inventory that
  `doctor`, `gc` and the console cockpit read, removes most of the manual digging from the
  2026-10-14 recovery.
wave: 8
depends: ["desk-containers/15"]
unblocks: ["desk-containers/21", "desk-containers/25"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-14: cell-level verbs so an operator does not run things per desk"
  - "Incident 2026-10-14: the recovery needed `cadence status`, the comms checkpoint, `cache status`, `lsof` on the cell's lock files, the dispatch-claim list and `ps` to find two `cellctl` processes holding `.reset.lock` and a scratch task lock"
  - "tools/desk/cmd/cellctl/status.go — house cell: one cadence line per role; scrubbed cell: running/stopped/stale-lock"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — status reports no claims, comms, cache, lock holders or version"
exec-tier: any
domain: complicated
consumers:
  - "tools/desk/cmd/cellctl/status.go: follow-up desk-containers/20 (this brief; the existing per-role cadence line stays as the first column set, so current readers keep working)"
  - "tools/desk/internal/cellop (planned) inventory reader: follow-up desk-containers/20 (this brief; doctor and gc reuse it)"
  - "docs/cellctl.md: follow-up desk-containers/20 (this brief)"
  - "the console cockpit (the desk rail and side panel render this document): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 20 — `cellctl status <cell>` as a cell overview

## Context
files:
- `tools/desk/cmd/cellctl/status.go`, `status_test.go`
- `tools/desk/internal/cellop/inventory.go` (planned) — one reader for every probe below, shared with brief 21 and brief 24
- `docs/cellctl.md`
- `changelog/desk-containers-20-status-overview.md` (planned)

facts:
- Per role: window present in the recorded cockpit (yes/no/could-not-check); harness pid + process start time; model and harness from the launch record; cadence `running`, `last`, heartbeat age; dispatch claims held by the role's session (count, oldest age); stop state (machine-wide, cell, role, from desk-containers/16).
- Cell rows: comms (checkpoint outcome, gateway pid alive, queued/held counts); go cache (logical bytes vs budget, available vs floor, active consumer records); lock holders (every lock under the cell's `run/` plus `.reset.lock`, with holder pid, command and start time, or "free"); tool version (`cellctl version` vs the pin the cell was booted with, flagging an older binary).
- Read-only. Every probe that cannot measure reports `null` + could-not-check, never a default. Exit 0 unless the cell fails to load (unchanged).
- Text output keeps today's per-role cadence line first, unchanged, then the new columns. `--json` is the desk-containers/15 result with `evidence{}` carrying every field.
- The probe set is one function, `cellop.Inventory(cell)`, so `doctor` and `gc` never re-measure differently.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- No writes, no lock acquisition. Lock holders are found by inspection (`lsof`-equivalent per platform), never by try-locking.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `cellop.Inventory` with the probes above, each independently could-not-check.
2. Rebuild `status` text and `--json` on it, keeping the existing line as the prefix.
3. Docs; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ ./internal/cellop/ -run 'TestStatusOverview|TestInventory' -count=1` | exit 0 |
| 2 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestInventoryNeverTryLocks' -count=1` | exit 0 — with a fixture process holding a cell lock, inventory names the holder and never acquires or contends for the lock |
| 3 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestInventoryNullNotZero' -count=1` | exit 0 — an unreadable cache root yields `null` and could-not-check, not 0 bytes |
| 4 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestStatusLegacyPrefix' -count=1` | exit 0 — the first line per role is byte-identical to the pre-change format |
| 5 | gate:model +flow +dereference | On a live cell: `cellctl status <cell> --json`, then check three of its claims by hand (one harness pid with `ps -o lstart=`, one lock holder with `lsof`, the cache bytes against `cellctl cache <cell> status`) | all three match. Record the commands and outputs. |
| 6 | check | `statusgen --root . --lint` | exit 0 |
| 6a | check | `statusgen --root . --consumers --brief desk-containers/20` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: read-only. The reviewer checks row 5.
