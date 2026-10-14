---
brief: assay:assay:desk-containers:18
title: "`cellctl drain|pause|resume <cell> [role...]` — CLI adapter onto the runner-lifecycle drain/pause/resume transitions and the handoff record, implemented on today's stop flags and cadence as the interim mechanism"
why: >-
  Closing desk windows by hand kills whatever the desks were doing. Agents die mid-task, their
  claims sit held until they time out, the cadence checkpoints are left "unfinished", and the
  next session starts with no idea what the last one was in the middle of. In the target
  architecture, draining, pausing, resuming and handing off are states and transitions of the
  one runner lifecycle contract. This brief gives the operator those transitions now, as
  `cellctl` verbs that are adapters onto the transition vocabulary, running on today's stop
  flags and cadence supervisor as an explicitly interim implementation. The verbs keep their
  meaning when the lifecycle contract replaces the mechanism underneath.
wave: 8
depends: ["desk-containers/15", "desk-containers/16"]
unblocks: ["desk-containers/19", "desk-containers/21"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-14: a `drain <cell>` that drains the loops first and then shuts down, with a handoff note; the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge): these are runner-lifecycle states/transitions; cellctl verbs are adapters; today's STOP-flag/cadence mechanism is the interim first implementation, not the design"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) §Lifecycle states and transitions — the state set, transitions, invariants and the interim mapping this brief implements"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Runtime durability — 'a terminal or agent chat must not be its memory'; 'A lease timeout does not cancel an external request already in flight'; a failed acknowledgment is 'an unknown outcome to reconcile'"
  - "Incident 2026-10-14: five hand-closed desk windows left five unfinished cadence checkpoints, a failed comms checkpoint, held dispatch claims waiting out their TTL, and ten orphaned background loops"
  - "desk-containers/16 — the interim cell-scoped stop flag behind `pause`"
  - "tools/desk/README.md — `desksupervise status --json` joins dispatch claims with liveness; claims carry the owning session"
  - "tools/desk/cmd/cellctl/down.go — `down` requests cancellation through each supervisor, waits for its lease and clean checkpoint, then closes the recorded surface"
  - "freshness-checked 2026-10-14 @ caca38a58 (origin/main) — no drain, pause or resume verb; `cadence <cell> stop|resume` controls the cadence supervisor, not the loops' own stop check"
exec-tier: strong
exec-tier-why: (b) spans the stop flags, dispatch claims, cadence supervisors and the skill bodies against a transition vocabulary another owner will implement; (c) wait/timeout/shutdown ordering is concurrency
domain: complicated
consumers:
  - ".cursor/skills/** (generated mirror of the skill bodies): follow-up desk-containers/18 (this brief; regenerated with `deskinstall --harness cursor`)"
  - "tools/desk/cmd/cellctl (drain/pause/resume adapters): follow-up desk-containers/18 (this brief)"
  - "plugins/assay/skills/{the-desk,pr-review-desk,verify-desk,worker-desk,intake-desk}/SKILL.md (write the handoff record on a cell drain; read it at boot): follow-up desk-containers/18 (this brief)"
  - "plugins/assay/references/handoff-note.md (planned): follow-up desk-containers/18 (this brief)"
  - "docs/cellctl.md: follow-up desk-containers/18 (this brief)"
  - "the runner lifecycle and ownership contract (target owner of these transitions and of the durable handoff record): out-of-scope (graph-execution R2/R3 packages; this brief's mechanism is interim and is replaced there, per docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
  - "the console cockpit (renders drain/pause/resume results and plans only via the shared cell-op contract; never applies): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 18 — `drain`, `pause`, `resume` adapters and the handoff record

## Context
files:
- `tools/desk/cmd/cellctl/lifecycle.go` (planned) — the transition vocabulary and the interim mechanism behind it, in one place
- `tools/desk/cmd/cellctl/drain.go` (planned), `drain_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `drain`, `pause`, `resume`
- `plugins/assay/skills/{the-desk,pr-review-desk,verify-desk,worker-desk,intake-desk}/SKILL.md` — handoff step and boot read
- `plugins/assay/references/handoff-note.md` (planned)
- `docs/cellctl.md`
- `changelog/desk-containers-18-drain.md` (planned)

facts:
- **The vocabulary is the design; the mechanism is interim.** The states, transitions and invariants are those in amendment 21's §Lifecycle states and transitions: role states `running`, `pausing`, `paused`, `draining`, `drained`, `stopped`; transitions `pause`, `resume`, `drain(timeout)`, `stop`, `start`. They are implemented behind one internal interface in `lifecycle.go`. The interim mechanism, named as such in code comments and in `docs/cellctl.md`, is:
  - `pause`: desk-containers/16's cell flag;
  - `drain`: pause, then wait for the cadence checkpoint `running=false` and zero in-flight dispatch claims owned by the cell's sessions;
  - `stop`: `down`;
  - the handoff record: a file under the cell.

  When the runner lifecycle contract ships, `lifecycle.go` is re-pointed at it and the verbs do not change.
- **Off a TTY, every form of `pause`, `resume` and `drain` except `--plan` is refused `not-interactive`** (they are state-changing, desk-containers/15 row 8); `--plan` works anywhere.
- **A recorded request is never authority.** The transition interface executes `resume` and `start` only when an operator invokes them interactively. A request read from a file under the cell (which any same-account process can write) is executed only if it is on the allowlist of stop / no-new-work transitions: pause, a timed drain, stop. Everything else, including resume, start, restart, `drain --now` and a foreign-stop lift, is never taken from a record, matching amendment 21's rule for R3.
- `pause <cell> [role...]` moves the targeted roles to `paused` once their loops reach the next iteration boundary. It creates a cell flag only if that flag does not already exist, and it records each flag it created (path, inode, creation time) in `run/lifecycle/pause-record.json` under the cell.
- **`resume` lifts only its own stops.** It removes exactly the flags listed in that record whose identity still matches, then moves those roles back to `running`. It never removes `DISABLED`, `STOP`, `STOP.<loop>`, a cell flag that existed before the pause (for example one an incident responder set), or another cell's flag. Lifting a stop that `pause` did not set is never implicit: `resume --foreign <flag>` writes a desk-containers/15 plan naming that exact flag and applies it only by interactive confirm. It is a deliberate operator act, not an authorization control. Both verbs are idempotent.
- `drain <cell> [role...] [--timeout 20m]` runs `pause`, then waits (polling at most every 10s, bounded by the timeout) for each targeted role to be idle with no in-flight work, collects the handoff record, then runs `stop`. **Invariant from the architecture:** draining never cancels an external effect already in flight. Work still running at timeout is listed, and the role stays up. The command exits 1 with `next[]` naming `drain --now` and `resume`.
- `drain --now` stops after a 60s handoff bound. It always goes through a desk-containers/15 plan, which lists every in-flight claim that will be abandoned as an unknown outcome to reconcile, and is applied only by the interactive confirm. Applying is an interactive-terminal confirmation only (desk-containers/15): the verb shows the plan and the operator types its confirm code. That is a convenience confirm, not an authorization control. Off a TTY the verb is plan-only.
- **Handoff record** (interim form: `run/handoff/<role>.md` under the cell). It holds the time; the claims the desk held and their state; open PRs it was driving; the decision it was waiting on; and one paragraph on what it was doing. A desk reads its record at boot before its first sweep, then marks it read. The shape is stated once in `plugins/assay/references/handoff-note.md` (planned). In the target, the durable resume state lives in the runtime coordination store, because "a terminal or agent chat must not be its memory". This file is the interim carrier, and amendment 21 records the migration.
- Abandonment is never silent: every claim still held at stop appears in the result's `actions[]` and in the handoff record.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- No process killing here; that is `doctor`/`gc` and #2880.
- Do not let the verbs' observable meaning depend on the interim mechanism. Tests assert transitions and states, not flag-file paths, except in the one `lifecycle.go` test that pins the interim mapping.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the transition interface and the interim mechanism in `lifecycle.go`.
2. Implement `pause`/`resume`/`drain` as adapters over it, adopting the desk-containers/15 result and plan contract.
3. Add the handoff step and boot read through the one reference.
4. Docs, labelling the mechanism interim and pointing to amendment 21; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestLifecycleTransitions' -count=1 -v` | `--- PASS` — role states move running → pausing → paused → running and running → draining → drained → stopped as amendment 21 defines; an illegal transition (resume from stopped) is refused |
| 1b | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestResumeLeavesForeignStops' -count=1 -v` | `--- PASS` — with `DISABLED`, `STOP`, `STOP.worker-desk`, a pre-existing `STOP.cell.<cell>` and `STOP.cell.<other>` present, `pause` then `resume` leaves every one of them in place and removes only the flag `pause` created; `resume --foreign` off a TTY is refused `not-interactive`; a mutant that removes all cell flags fails |
| 1c | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestPauseResumeDrainRefuseOffTTY' -count=1 -v` | `--- PASS` — `pause`, `resume`, `resume --foreign`, `drain` and `drain --now`, each with stdin or stdout not a TTY, are refused `not-interactive` and change no flag |
| 1d | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestRecordedResumeNotExecuted' -count=1 -v` | `--- PASS` — request files under the cell asking for `resume`, `start`, `restart`, `drain --now` or a foreign-stop lift, written as a desk could write them, are not executed by the transition interface; recorded `pause`, timed `drain` and `stop` are (an allowlist). A denylist mutant that blocks only resume, start and restart fails on the `drain --now` and foreign-stop cases |
| 2 | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainWaitsForInFlight' -count=1 -v` | `--- PASS` — with a fixture claim held by the cell's session, `stop` is not invoked until the claim releases; a mutant that stops right after pause fails |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainTimeoutDoesNotStop' -count=1 -v` | `--- PASS` — on timeout the roles stay up, outcome is `partial`, and `next[]` names `drain --now` and `resume` |
| 4 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainNowInteractiveOnly' -count=1 -v` | `--- PASS` — `drain --now` off a TTY is refused `not-interactive`; on a pseudo-terminal with the right confirm code, the result lists every abandoned claim as an unknown outcome |
| 5 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestInterimMappingPinned' -count=1 -v` | `--- PASS` — the single test that names the interim mechanism (cell flag, cadence checkpoint, handoff file) passes, and no other test references the flag path |
| 6 | check +dereference | `grep -l 'references/handoff-note.md' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/intake-desk/SKILL.md \| wc -l` | 5 |
| 7 | gate:model +flow +dereference | On a disposable house cell with stub desks holding one fixture claim: `cellctl drain <cell> --timeout 2m` while the claim releases after 30s | the claim releases; a handoff record exists per role; the cell is stopped; `cellctl cadence <cell> status` shows no `unfinished`; a following `cellctl up <cell>` exits 0 with every role `up` and each desk reports reading its record |
| 8j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainEmitsCellOpV1' -count=1 -v` | `--- PASS` — `drain --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 8 | check | `statusgen --root . --lint` | exit 0 |
| 8a | check | `statusgen --root . --consumers --brief desk-containers/18` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: the transitions only add stops and call the existing clean `down`; abandonment needs an interactive confirm on a plan that lists every abandoned item as an unknown outcome. The reviewer checks rows 2, 4 and 5, the last one being the guarantee that the interim mechanism stays replaceable.
