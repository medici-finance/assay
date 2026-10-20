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
wave: 9
depends: ["desk-containers/15", "desk-containers/16", "desk-containers/17"]
unblocks: ["desk-containers/19", "desk-containers/21", "desk-containers/22"]
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
  - "tools/desk/cmd/cellctl/down.go and cadence.go (`stopCadences`) — `down` writes each cadence-mode role's STOP file, waits up to 20 s for its lease and refuses on an unfinished pass; interactive-mode role sessions are hard-killed with the cockpit session; then it closes the recorded surface"
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
  - "tools/desk/cmd/cellctl/cobra.go (`down` Use becomes `down <cell> [role...]`), tools/cellctl/testdata/cellctl-shell-oracle.sh (cell-qualified `<cell>-<role>` window names and the per-role `down`), and tools/desk/cmd/cellctl/testdata/cli-nonhelp-golden.json plus the usage text `TestUsageMatchesOracle` compares: follow-up desk-containers/18 (this brief; oracle and goldens move together as deliberate diffs)"
  - "the console cockpit (renders drain/pause/resume results and plans only via the shared cell-op contract; never applies): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 18 — `drain`, `pause`, `resume` adapters and the handoff record

## Context
files:
- `tools/desk/cmd/cellctl/lifecycle.go` (planned) — the transition vocabulary and the interim mechanism behind it, in one place
- `tools/desk/cmd/cellctl/drain.go` (planned), `drain_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `drain`, `pause`, `resume`; role arguments on `down`
- `tools/desk/cmd/cellctl/down.go` — `down <cell> <role...>`, the per-role interim stop
- `tools/desk/cmd/cellctl/role_surface.go` (planned) — per-role surface receipt written at launch (`run/cadence/<role>/surface.json`) and its single-surface close
- `tools/desk/cmd/cellctl/up.go` — record each role's surface receipt when its window opens
- `plugins/assay/skills/{the-desk,pr-review-desk,verify-desk,worker-desk,intake-desk}/SKILL.md` — handoff step and boot read
- `plugins/assay/references/handoff-note.md` (planned)
- `docs/cellctl.md`
- `changelog/desk-containers-18-drain.md` (planned)

facts:
- **The vocabulary is the design; the mechanism is interim.** The states, transitions and invariants are those in amendment 21's §Lifecycle states and transitions: role states `running`, `pausing`, `paused`, `draining`, `drained`, `stopped`; transitions `pause`, `resume`, `drain(timeout)`, `stop`, `start`. They are implemented behind one internal interface in `lifecycle.go`. The interim mechanism, named as such in code comments and in `docs/cellctl.md`, is:
  - `pause`: desk-containers/16's cell flag;
  - `drain`: pause, then wait until each role is idle per its mode, as defined under **Idle signal, per mode** below (cadence: between passes; interactive: no claim and no extra harness child, with harness idleness could-not-check, so the interactive role is stopped only after the operator's TTY confirm);
  - `stop`: **`down <cell> <role...>`**, the per-role interim stop below (never the cell-wide `down <cell>`);
  - the handoff record: a file under the cell.

  When the runner lifecycle contract ships, `lifecycle.go` is re-pointed at it and the verbs do not change.
- **Off a TTY, every form of `pause`, `resume` and `drain` except `--plan` is refused `not-interactive`** (they are state-changing, desk-containers/15 row 8); `--plan` works anywhere.
- **A recorded request is never authority.** A request read from a file under the cell (which any same-account process can write) is executed only if it is on the allowlist of stop / no-new-work transitions: pause, a timed drain, and a per-role stop **only for a role already paused or drained**, executed as pause → re-measure the idle signal → stop (below); a busy role's recorded stop is refused. Everything else, including resume, start, restart, `drain --now` and a foreign-stop lift, is never taken from a record, matching amendment 21's rule for R3.
- `pause <cell> [role...]` moves the targeted roles to `paused` once their loops reach the next iteration boundary. It creates a cell flag only if that flag does not already exist, and it records each flag it created (path, inode, creation time) in `run/lifecycle/pause-record.json` under the cell.
- **`resume` lifts only its own stops.** It removes exactly the flags listed in that record whose identity still matches, then moves those roles back to `running`. It never removes `DISABLED`, `STOP`, `STOP.<loop>`, a cell flag that existed before the pause (for example one an incident responder set), or another cell's flag. Lifting a stop that `pause` did not set is never implicit: `resume --foreign <flag>` writes a desk-containers/15 plan naming that exact flag and applies it only by interactive confirm. It is a deliberate operator act, not an authorization control. Both verbs are idempotent.
- `drain <cell> [role...] [--timeout 20m]` runs `pause`, then waits (polling at most every 10s, bounded by the timeout) for each targeted role to be idle with no in-flight work, collects the handoff record, then runs the per-role stop for each targeted role. **Invariant from the architecture:** draining never cancels an external effect already in flight. Work still running at timeout is listed, and the role stays up. The command exits 1 with `next[]` naming `drain --now` and `resume`.
- **Per-role interim stop: `down <cell> <role...>`.** With role arguments, `down` stops exactly the named roles and never a sibling or another cell's role. Without role arguments it is today's cell-wide `down`: unchanged, never used by these transitions, and it now also removes every role's surface receipt. For each named role, in order:
  1. stop that role's cadence alone: the per-role part of `stopCadences` (the role's STOP file, a 20 s lease wait, and a refusal on an unfinished pass);
  2. **re-measure the idle signal again**, after that wait and immediately before the close (below);
  3. **re-verify the surface identity** against the live cockpit (below);
  4. close exactly that one surface with the single-surface calls cellctl already makes in `closeCommsSurface` (`tools/desk/cmd/cellctl/comms_startup.go`): tmux `kill-window -t <window_id>`; herdr `tab close <tab_id>` (present in herdr 0.9.4, where `herdr tab --help` lists `close`; the build is probed the same way as orca's flag, and an unsupported build refuses); orca `terminal close --terminal <handle>`, only where the orca build advertises `--terminal`;
  5. remove the receipt.

  It never falls back to closing the whole session.
- **Surface receipt, bound to the live surface (a work-safety check, not authorization).**
  - **What is recorded:** at every launch, the role's receipt `run/cadence/<role>/surface.json` records the cockpit, this cell's session, workspace or runtime id, the window, tab or terminal id, the **cell-qualified window name `<cell>-<role>`**, and the role harness's pid and process start time.
    - Window names become cell-qualified at launch, so names cannot collide across cells on the shared tmux server. A colon is avoided because tmux target syntax uses it.
  - **How it is written:** 0600, created with `O_CREAT|O_EXCL|O_NOFOLLOW` in a temporary name, then renamed atomically into the cell-owned 0700 directory. It replaces any previous receipt at every launch, and it is removed on a per-role close and on cell-wide `down`.
  - **Identity check before any close.** Every value must match the live surface:
    - **tmux:** the window's `session_name` equals this cell's session; its `window_name` equals `<cell>-<role>`; and the pane's process tree contains the recorded harness pid, with a start time equal to the recorded one.
    - **herdr:** the tab is in this cell's recorded workspace and carries the `<cell>-<role>` label; and `herdr pane process-info --pane <pane>` (which reports `shell_pid` and the foreground processes) shows the recorded harness pid with its recorded start time.
    - **orca:** the runtime id and terminal handle match, the terminal title is `<cell>-<role>`, and the recorded harness pid and start time are in the terminal's process where the orca build exposes it; a build that does not expose it refuses `surface-identity-mismatch`.
  - **Refusals:** any mismatch, or any value that cannot be read, refuses `surface-identity-mismatch` and closes nothing. A **stale** receipt (the recorded harness pid is gone, or its start time differs) refuses `surface-stale`, is reported, and closes nothing; the operator re-launches or runs cell-wide `down`.
  - **What this protects against:** these checks keep a stale receipt from closing another cell's or a sibling's window. A forger under the same account could close any window directly, so this is not an authorization control.
- **Idle signal, per mode (the work-safety precondition, not a security boundary).** A per-role stop runs only for a role already paused or drained, and executes as pause → re-measure → stop. "Idle" is defined per mode:
  - **cadence-mode role:** between passes, meaning its cadence checkpoint reads `running=false`.
  - **interactive-mode role** (the default; its checkpoint holds `Running: true` until the harness exits, `tools/desk/internal/cellcadence/interactive.go`, so `running=false` never holds):
    - no dispatch claim held by that role's session;
    - no process under the harness pid other than the harness itself and the long-lived helpers recorded at launch;
    - the harness itself idle, with no tool call in flight. **This last part cannot be measured from outside the harness today; it is reported as could-not-check.**

  Consequences:
  - A recorded stop for an interactive role is therefore always refused `role-busy` (could-not-check is never idle).
  - An interactive role is stopped only through the operator's terminal confirm, on a plan whose text states that harness idleness was not measured.
  - A busy result at any re-measure refuses `role-busy`, whatever the request claimed.
  - The residual window is now only the interval between the final re-measure (step 2, after the 20 s wait) and the close (step 4): a tool call that starts inside an already-running harness turn in that instant. Because the role is paused, no new pass or dispatch can begin.
- `drain --now` stops after a 60s handoff bound. It always goes through a desk-containers/15 plan, which lists every in-flight claim that will be abandoned as an unknown outcome to reconcile, and is applied only by the interactive confirm. Applying is an interactive-terminal confirmation only (desk-containers/15): the verb shows the plan and the operator types its confirm code. That is a convenience confirm, not an authorization control. Off a TTY the verb is plan-only.
- **Handoff record** (interim form: `run/handoff/<role>.md` under the cell). It holds the time; the claims the desk held and their state; open PRs it was driving; the decision it was waiting on; and one paragraph on what it was doing. A desk reads its record at boot before its first sweep, then marks it read. The shape is stated once in `plugins/assay/references/handoff-note.md` (planned). In the target, the durable resume state lives in the runtime coordination store, because "a terminal or agent chat must not be its memory". This file is the interim carrier, and amendment 21 records the migration.
- Abandonment is never silent: every claim still held at stop appears in the result's `actions[]` and in the handoff record.

## Ground rules
- CLI built on the desktools-v2 Cobra/Viper foundation (desktools-v2 cli-contract.md §7): every new command, verb or launcher this brief adds uses that shared foundation (added 2026-10-18).
- NEVER git push / trigger workflows. Stop at `implemented`.
- The only process this brief may stop is the named role's own window and harness, through the identity-verified single-surface close of a role that passed the idle re-measure. It never signals any other process: no siblings, no other cell's roles, no orphans (those are `doctor`/`gc` and #2880).
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
| 1d | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestRecordedResumeNotExecuted' -count=1 -v` | `--- PASS` — request files under the cell asking for `resume`, `start`, `restart`, `drain --now` or a foreign-stop lift, written as a desk could write them, are not executed by the transition interface; recorded `pause` and timed `drain` are, and a recorded per-role `stop` is executed only against a role already paused or drained that re-measures idle, while a recorded `stop` against a BUSY role (an in-flight claim) is refused (an allowlist with a work-safety precondition). A denylist mutant that blocks only resume, start and restart fails on the `drain --now` and foreign-stop cases, and a mutant that executes a recorded stop without the idle/drained check fails on the busy-role case. Two further refusal cases, each with its own mutant: (a) a recorded per-role stop for idle role A leaves a busy sibling B (not named by the record) running, with its window, cadence and in-flight claim intact; a mutant that stops the whole cell fails. (b) A record that claims role A is idle while A holds a claim or a live harness child is refused `role-busy` at the re-measure; a mutant that trusts the record's claim fails. |
| 1e | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestPerRoleDownClosesOneSurface' -count=1 -v` | `--- PASS` — for stub tmux, herdr and orca cockpits: `down <cell> worker-desk` re-verifies identity and issues exactly one single-surface close (`kill-window -t <id>`, `tab close <tab>`, `terminal close --terminal <h>`) for that role and removes its receipt. Each of these refuses and closes nothing: no receipt or an unsupported close (`per-role-close-unsupported`, including a herdr build without `tab close` and an orca build without `--terminal`); a receipt whose id now names **another cell's** same-role window, or a **sibling's** window in this cell (`surface-identity-mismatch`); a **stale** receipt whose recorded harness pid is gone or has a different start time (`surface-stale`). A probe that cannot determine identity (a failing herdr `process-info`, an unreadable start time) refuses `surface-identity-mismatch`. Mutants that must each fail: one that treats an unreadable probe as a match, one that skips the identity check, one that closes the whole session, one that closes without a receipt |
| 1f | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestIdleSignalPerMode' -count=1 -v` | `--- PASS` — a cadence role between passes (`running=false`, no claim) measures idle; an interactive role (checkpoint `Running: true`) with no claim and no extra child measures harness-idle `could-not-check`, so a recorded stop for it is refused `role-busy` and the interactive drain plan states the could-not-check; the idle re-measure runs after the 20 s cadence wait (a role that takes a claim during the wait is refused); a mutant that reads interactive `Running: true` as busy-forever or as idle fails |
| 2 | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainWaitsForInFlight' -count=1 -v` | `--- PASS` — with a fixture claim held by the cell's session, `stop` is not invoked until the claim releases; a mutant that stops right after pause fails |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainTimeoutDoesNotStop' -count=1 -v` | `--- PASS` — on timeout the roles stay up, outcome is `partial`, and `next[]` names `drain --now` and `resume` |
| 4 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainNowInteractiveOnly' -count=1 -v` | `--- PASS` — `drain --now` off a TTY is refused `not-interactive`; on a pseudo-terminal with the right confirm code, the result lists every abandoned claim as an unknown outcome |
| 5 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestInterimMappingPinned' -count=1 -v` | `--- PASS` — the single test that names the interim mechanism (cell flag, cadence checkpoint, handoff file) passes, and no other test references the flag path |
| 6 | check +dereference | `grep -l 'references/handoff-note.md' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/intake-desk/SKILL.md \| wc -l` | 5 |
| 7 | gate:model +flow +dereference | On a disposable house cell with stub desks holding one fixture claim: `cellctl drain <cell> --timeout 2m` while the claim releases after 30s | the claim releases; a handoff record exists per role; the cell is stopped (every targeted role's surface is closed and its receipt removed, no role harness pid from the receipts is alive, and `cellctl cadence <cell> status` reports no `unfinished`); `cellctl cadence <cell> status` shows no `unfinished`; a following `cellctl up <cell>` exits 0 with every role `up` and each desk reports reading its record |
| 8j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDrainEmitsCellOpV1' -count=1 -v` | `--- PASS` — `drain --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 8 | check | `statusgen --root . --lint` | exit 0 |
| 8a | check | `statusgen --root . --consumers --brief desk-containers/18` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: the transitions only add stops and stop only the named roles through the per-role `down <cell> <role...>`, after the drain has waited them idle and re-measured the idle signal per mode, and only after re-verifying the window's live identity against its receipt (a mismatch or stale receipt refuses), so a sibling's or another cell's window is not closed; the cell-wide `down` is never used here; abandonment needs an interactive confirm on a plan that lists every abandoned item as an unknown outcome. The reviewer checks rows 2, 4 and 5, the last one being the guarantee that the interim mechanism stays replaceable.
