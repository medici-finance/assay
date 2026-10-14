---
brief: assay:assay:desk-containers:18
title: "`cellctl drain|pause|resume <cell> [role...]` — stop new work, let in-flight work finish, write a handoff note, then shut down"
why: >-
  Closing desk windows by hand kills whatever the desks were doing. Agents die mid-task, their
  claims sit held until they time out, the cadence checkpoints are left "unfinished", and the
  next session starts with no idea what the last one was in the middle of. A drain does the
  same shutdown in order: stop taking new work, wait a bounded time for running work to
  finish, have each desk write down where it was, then stop cleanly. Pause is the first half
  on its own.
wave: 8
depends: ["desk-containers/15", "desk-containers/16"]
unblocks: ["desk-containers/19"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-14: a `drain <cell>` that drains the loops first and then shuts down; scope confirmed to include the handoff note"
  - "Incident 2026-10-14: five hand-closed desk windows left five unfinished cadence checkpoints, a failed comms checkpoint, held dispatch claims waiting out their TTL, and ten orphaned background loops"
  - "desk-containers/16 — the cell-scoped stop flags this verb sets and clears"
  - "tools/desk/README.md — `desksupervise status --json` joins dispatch claims with liveness; dispatch claims carry the owning session (`DESK_SESSION`), so a cell's in-flight work is the set of `state=dispatched` claims owned by that cell's sessions"
  - "tools/desk/cmd/cellctl/down.go — `down` requests cancellation through each supervisor, waits for its lease and clean checkpoint, and closes the recorded surface; drain ends by calling it"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no drain, pause or resume verb; `cadence <cell> stop|resume` exists per role, but it controls the cadence supervisor, not the loops' own stop check"
exec-tier: strong
exec-tier-why: (b) spans the stop flags, dispatch claims, cadence supervisors and the skill bodies; (c) the wait/timeout/shutdown ordering is concurrency, where an early shutdown survives happy-path tests
domain: complicated
consumers:
  - "tools/desk/cmd/cellctl (new drain/pause/resume verbs): follow-up desk-containers/18 (this brief)"
  - "plugins/assay/skills/{the-desk,pr-review-desk,verify-desk,worker-desk,intake-desk}/SKILL.md (the handoff-note step on a cell stop, and reading the note at boot): follow-up desk-containers/18 (this brief)"
  - "docs/cellctl.md: follow-up desk-containers/18 (this brief)"
  - "the console cockpit (drain/pause/resume buttons over `--json` and plan/apply): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 18 — `drain`, `pause`, `resume`, and the handoff note

## Context
files:
- `tools/desk/cmd/cellctl/drain.go` (planned), `drain_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `drain`, `pause`, `resume`
- `plugins/assay/skills/{the-desk,pr-review-desk,verify-desk,worker-desk,intake-desk}/SKILL.md` — handoff step + boot read
- `plugins/assay/references/handoff-note.md` (planned) — the note's shape, stated once
- `docs/cellctl.md`
- `changelog/desk-containers-18-drain.md` (planned)

facts:
- `pause <cell> [role...]` touches `STOP.cell.<cell>` (no roles) or `STOP.cell.<cell>.<role>` (desk-containers/16) and returns. Loops stop at their next iteration boundary, and the Guard refuses new outward verbs. `resume` removes exactly the flags `pause` created. Both are idempotent and report the flag set.
- `drain <cell> [role...] [--timeout 20m]`:
  1. pause;
  2. wait until every targeted loop reports idle (cadence checkpoint `running=false`, or the loop has written its stop acknowledgement) AND the cell's in-flight dispatch claims reach zero, polling at most every 10s and bounded by `--timeout`;
  3. each targeted desk writes its **handoff note** (below) on seeing the cell stop flag;
  4. `down` for the targeted roles.
  On timeout, it does not shut down. It reports the claims and roles still busy and exits 1 with `next[]` naming `drain --now` (shut down anyway) and `resume`.
- `drain --now` skips the wait but still pauses first and still asks for handoff notes with a 60s bound. Off a TTY it is plan → apply (desk-containers/15): the plan lists the in-flight claims that will be abandoned.
- **Handoff note**: `run/handoff/<role>.md` under the cell, one per role, overwritten each drain. It holds the time; the claims the desk held and their state; open PRs it was driving; the last decision it was waiting on; and one paragraph on "what I was doing". On boot a desk reads its note, if one exists, before its first sweep, then renames it `<role>.<timestamp>.read.md`. The shape is stated once in `plugins/assay/references/handoff-note.md` (planned), and the five skill bodies point at it.
- Abandoning work is never silent: every claim still held at shutdown is listed in the result's `actions[]` and in the role's handoff note.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- Do not kill processes in this verb. Process-level cleanup belongs to `doctor`/`gc` and #2880's reconcile.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `pause`/`resume` over the desk-containers/16 flags.
2. Implement `drain` with the wait, timeout and handoff ordering above, adopting the desk-containers/15 result and plan/apply contract.
3. Add the handoff step and the boot read to the five skill bodies through the one reference.
4. Docs and changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestPauseResume' -count=1` | exit 0 — pause creates exactly the cell-scoped flags; resume removes only those and leaves a pre-existing machine-wide `STOP` in place |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainWaitsForClaims' -count=1` | exit 0 — with a fixture claim held by the cell's session, drain does not call `down` until the claim releases; a mutant that calls `down` right after pause fails |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainTimeoutDoesNotShutDown' -count=1` | exit 0 — on timeout the roles are still up, outcome is `partial`, and `next[]` names `drain --now` and `resume` |
| 4 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainNowPlanApply' -count=1` | exit 0 — off a TTY `drain --now` needs `--plan` then `--apply <id>`; the plan lists the abandoned claims |
| 5 | check +dereference | `grep -l 'references/handoff-note.md' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/intake-desk/SKILL.md \| wc -l` | 5 |
| 6 | gate:model +flow | On a disposable house cell with stub desks that hold one fixture claim: `cellctl drain <cell> --timeout 2m` while the claim releases after 30s | the claim releases, a handoff note exists for every role, the cell is down, the cadence status has no `unfinished`, and a following `cellctl up <cell>` boots with no reconcile prompt. Record the result JSON. |
| 7 | check | `statusgen --root . --lint` | exit 0 |
| 7a | check | `statusgen --root . --consumers --brief desk-containers/18` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: drain only adds stop flags and calls the existing clean `down`. Abandoning work (`--now`) is listed, never silent, and off a TTY it needs a human-approved plan. The reviewer checks rows 2, 3 and 6.
