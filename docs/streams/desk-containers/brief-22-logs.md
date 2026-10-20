---
brief: assay:assay:desk-containers:22
title: "`cellctl logs <cell> [role] [--lines N]` — read a desk window's recent output from whichever cockpit holds it"
why: >-
  When a desk fails at launch, its error is printed in its own window. In a multi-tab cockpit
  that means opening each tab to find the line. On 2026-10-14, four desks failed with the same
  lock error, and each one had to be read from its own pane. `logs` reads a role's window
  through the cockpit's own API and prints it, for one role or all of them, so the console
  cockpit can show it too.
wave: 10
depends: ["desk-containers/15", "desk-containers/18"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) — classifies this verb as an interim bridge and names its target owner"
  - "Incident 2026-10-14: four role panes each held `cellctl: interactive launch: deskkit: advisory lock is held by another process`, found by reading every pane through the cockpit's pane-read command"
  - "tools/desk/cmd/cellctl/herdr_workspace.go — each cell owns a herdr workspace receipt; role tabs are labelled by role; tmux windows are named by role in session `<cell>-cell`"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no logs verb"
exec-tier: any
domain: clear
consumers:
  - "target owner — projection and interaction (terminals), served through the R7 Cell API: out-of-scope (this verb is an interim bridge on today's tools, per docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
  - "tools/desk/cmd/cellctl (new logs verb): follow-up desk-containers/22 (this brief)"
  - "docs/cellctl.md: follow-up desk-containers/22 (this brief)"
  - "the console cockpit (the side panel's log view): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 22 — `cellctl logs`

## Context
files:
- `tools/desk/cmd/cellctl/logs.go` (planned), `logs_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `logs`
- `docs/cellctl.md`
- `changelog/desk-containers-22-logs.md` (planned)

facts:
- **Interim bridge.** logs is an interim bridge: it reads today's cockpit panes. In the target, terminal attachment and output are projection-and-interaction concerns served through the Cell API; 'notifications and terminal attachment are not durable ownership'.
- tmux: `tmux capture-pane -p -t <window_id> -S -<N>`, using the window id verified below (role windows are named `<cell>-<role>`, desk-containers/18). herdr: read the pane of the role's tab in the cell's recorded workspace, using herdr's pane-read command (re-check its exact flags against the installed herdr before writing; do not use memory). orca: the terminal's scrollback via orca's read command where one exists, otherwise could-not-check naming the gap.
- Only panes in the cell's own recorded session or workspace are read, never another cell's. Before reading, `logs` re-verifies the same identity binding as desk-containers/18's per-role stop: session, cell-qualified `<cell>-<role>` name, and the harness pid and start time. Any mismatch or stale receipt refuses `surface-identity-mismatch` / `surface-stale`, and nothing is read.
- No role means every role in `ROLES` plus comms, each under a header. `--lines` defaults to 50.
- `--json` returns the desk-containers/15 result with each role's lines in `evidence.lines[]`. An absent window is `state: absent`, not an error.
- Pane text can carry secrets a desk echoed. `logs` prints what is already on the operator's screen and stores nothing.

## Ground rules
- CLI built on the desktools-v2 Cobra/Viper foundation (desktools-v2 cli-contract.md §7): every new command, verb or launcher this brief adds uses that shared foundation (added 2026-10-18).
- NEVER git push / trigger workflows. Stop at `implemented`.
- Read-only: never send keys or input to a pane.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `logs` for the three cockpits with the ownership check; register; docs; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestLogs' -count=1 -v` | `--- PASS` — stub tmux/herdr binaries record the exact read invocation per cockpit |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestLogsOwnCellOnly' -count=1 -v` | `--- PASS` — a role tab in another cell's workspace is never read; the call is refused |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestLogsNeverWrites' -count=1 -v` | `--- PASS` — no send-keys/input call is ever made |
| 4 | gate:model +flow +dereference | On a live herdr cell: `cellctl logs <cell> worker-desk --lines 20` | output matches what the worker-desk tab shows on screen; record both |
| 5j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestLogsEmitsCellOpV1' -count=1 -v` | `--- PASS` — `logs --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 5 | check | `statusgen --root . --lint` | exit 0 |
| 5a | check | `statusgen --root . --consumers --brief desk-containers/22` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: read-only, and only for panes the cell itself owns. The reviewer checks row 2.
