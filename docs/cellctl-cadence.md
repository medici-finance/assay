# Host desk cadence

Command-line forms (flags, precedence, exit codes) are listed in [the `cellctl` command reference](cellctl-cli-reference.md); what changed from the earlier hand-written parser is in [the compatibility table](cellctl-cli-compat.md).

`cellctl` can own the clock for a host desk whose harness cannot wake itself after
its current turn. The cockpit runs one foreground Go supervisor; that supervisor
starts a bounded role pass, waits for completion, then waits the configured interval.
Herdr and Orca supply the terminal. Codex and Cursor supply the agent pass.

```sh
cellctl up house --cockpit herdr --harness codex --cadence 5m --tick-budget 20m
cellctl up example --cockpit orca --harness cursor --cadence 30m --tick-budget 20m
```

Configure each role's model first. Codex keeps its existing model-policy resolution.
Cursor uses `CURSOR_MODEL_<role>` (role hyphens become underscores), then
`CURSOR_MODEL_default`, then `TIER_MODEL_<TIER>_CURSOR`; there is no inferred model.
Cursor uses the `agent` CLI and its own authentication and installed skills. Cursor
currently refuses model-policy files, Claude providers/config directories, and
non-house cell kinds. A host cell's name is operator-selected; `house` is the kind.

For one role:

```sh
cellctl desk house pr-review-desk --cadence 30m --tick-budget 20m
cellctl set house CELL_CADENCE=30m CELL_TICK_BUDGET=20m
cellctl cadence house status
```

Codex desks default to an interactive terminal session. This applies after harness
policy resolution to direct `desk` launches and each role opened by `up`.
Before invoking the role, the bootstrap requests native `get_goal`, creates the
standing objective only when no Goal exists, and reads it back. Only a matching,
active readback admits the standing role. Existing objectives, usage, budgets and
paused state are preserved; a mismatch or unavailable Goal tools leaves the session
waiting for user direction with a could-not-check report. No token budget is added.

Native Goals continue authorized work after ordinary final responses and completed
batches while the terminal remains interactive. An empty queue does not complete the
standing objective. `/goal` inspects it; `/goal pause` and `/goal clear` retain user
control. Blocking conditions, interruption and native continuation limits can stop
progress: this is not a durable polling clock or a permanent-service guarantee.

If a user says "correct and resume" while the Goal is inactive, the bootstrap
instructs the desk to complete the authorized correction under its existing
safeguards and then read native state again. If the state is still inactive and
the exposed tools offer no resume operation, it gives the exact `/goal resume`
control instead of claiming success or asking again for correction permission.
Matching active readback remains required for standing continuation. The launcher
does not own or mutate native Goal state.

Explicit `--cadence` / `CELL_CADENCE` durations select the existing headless supervisor;
`--cadence off` or saved `CELL_CADENCE=off` explicitly selects interactive mode.
A tick budget requires an explicit cadence; `off` ignores a saved budget. Codex tick
launches disable native Goals and never receive the standing bootstrap. Claude and
Cursor keep their existing behavior. Defaults do not rewrite cell.env or change
already-running sessions.

The same bootstrap covers host and scrubbed Codex desk argv. Native container cells
run an independently pinned image entrypoint with its own paused/handoff contract;
cellctl does not construct that image's Codex prompt, so this change does not establish
a Goal inside those images. Smoke checks and non-desk commands remain unchanged.

A budget includes the role's exit reserve;
its minimum is 61 seconds and default is 20 minutes. Choose a budget that also fits
the actual work; a review may need substantially more time. Every pass receives
`ASSAY_TICK=1` and `ASSAY_TICK_DEADLINE` in seconds, so it performs one sweep and exits.
The supervisor sends no periodic keystrokes or prompts into a busy interactive turn.

## Switching an existing interactive house

Finish or hand off active work, then exit each Codex desk cleanly so its ownership
checkpoint records completion. Close the old cockpit with `cellctl down house --cockpit herdr`. After the roles have stopped, clear local stop requests and launch:

```sh
cellctl cadence house resume
cellctl up house --cockpit herdr --harness codex --cadence 5m --tick-budget 20m
cellctl cadence house status
```

Keep the explicit cadence flag when choosing repeated headless passes. Omitting it
selects the interactive default unless cell.env saves a duration.
`supervisor=active` identifies a cadence owner; `supervisor=interactive` does not.
If status instead reports `unfinished`, follow the recovery procedure below only after
confirming the old harness and all children have stopped. Do not delete locks or
checkpoints to force a second owner. A launch never clears a stop flag itself.

## Configuration and ownership

The cadence uses the same resolved model, policy arguments, role, repository roots,
command environment, cockpit and worktree as a normal desk launch. A cockpit command explicitly
pins the cell registry using `--cells-root`, so a cockpit server with a different
inherited environment cannot select a different cell. Provider credentials stay in
memory/environment; the checkpoint contains schedule and completion metadata only.

A per-role operating-system file lock excludes both another supervisor and an
interactive replacement. Every completed pass schedules its successor relative to
completion, so slow passes do not overlap and missed intervals do not build a queue.
A saved next-due time survives supervisor restart. Downtime coalesces into one pass.

State is stored under `<cell>/run/cadence/<role>/checkpoint.json`. A zero agent exit
is insufficient: the last output line must satisfy the existing `desktick` grammar
and name this role. Codex's final-message output file supplies that evidence; progress
logs do not. Missing, invalid or wrong-role summaries and execution errors are
`could-not-check`, never `noop`. `status` distinguishes the current role owner from
its last reported outcome and shows the last supervisor heartbeat and next due time.
The heartbeat proves the supervisor is responding, not that a queue sweep succeeded.

## Stop and recovery

```sh
cellctl cadence house stop pr-review-desk
cellctl cadence house status pr-review-desk
cellctl cadence house resume pr-review-desk
cellctl desk house pr-review-desk --cadence 30m
```

A stop request cancels an active pass on the next supervisor heartbeat (normally five
seconds), then reaps its process tree. `cellctl down` requests this cancellation
before closing cockpit windows. `resume` clears only the local stop request; it does
not launch a model, clear the cell's STOP/DISABLED flags, or discard a checkpoint.

A supervisor crash during a pass or interactive session is ambiguous: the agent might still be running.
An unfinished checkpoint therefore refuses all replacement launches, even when the
supervisor lock is free. Inspect and stop the prior harness and its descendants first.
Only after confirming that they have stopped, explicitly reset that role:

```sh
cellctl cadence house recover pr-review-desk --confirm-stopped
```

The confirmation is an operator assertion, not a PID-based proof. Recovery does not
kill an arbitrary recorded PID. Ordinary resume never clears this condition.

### Reconcile preflight

`cellctl up <cell>`, and `cellctl desk <cell> <role>` for its one role, gather that
evidence before launching anything, so a cell whose windows were closed by hand needs
one answer instead of one recovery per role. The preflight:

1. **Inventories** every role's checkpoint and, for `up`, the comms checkpoint and its
   launch receipt. It lists each unfinished (`running`) record with its pid, recorded start
   time, last heartbeat and recorded surface. A finished record with a failed outcome was
   written by a supervisor that reaped its own child; it is not stale and blocks nothing, as
   before.
2. **Classifies** each record by pid *and* start time, never by pid alone. The start times
   come from the role lease's owner record, which a crashed owner leaves behind (the
   preflight reads it without taking the lease, so the evidence survives the read).
   - `owned`: a live process holds the role lease. That is a running window; it is listed
     and never touched.
   - `dead`: the lease is free, and the recorded supervisor and role child have each
     exited or now belong to a process with a different start time (pid reuse), and no
     process is left in the dead role child's process group. A reused pid is never
     signalled.
   - `alive`: a recorded process still matches its pid and start time; or a process
     carrying the role's marker is still running and is not a signal candidate (shielded, or
     not re-parented to pid 1); or a process without
     a readable marker is still in the dead role child's process group. Such a process
     descends from the role but is not proven ours: it is named and never signalled.
   - `could-not-check`: something could not be measured, including a running pid with no
     recorded start time. It is never treated as dead.
3. **Finds orphans the cell owns.** Every child cellctl launches for a role carries
   `ASSAY_CELL=<cell>`, `ASSAY_CELL_DIR=<cell directory>` and `ASSAY_CELL_ROLE=<role>`
   (`comms` for the service), and descendants inherit them. A process is an orphan only
   if it carries this cell's marker for a role whose lease is free, and it was re-parented
   to pid 1 (or descends, through marked processes, from one that was). Nothing related to
   anything that may be live and not this cell's is ever a candidate. The shield roots are
   the invoking process, this cell's live lease holders, every process of another user, every
   `cellctl` (any cell's supervisor), and every process whose environment was read and does
   not carry this cell's marker (another cell's role child, an unrelated program). Shielded
   are the roots, their descendants, their ancestors (a terminal-multiplexer server hosting a
   live window) and anything else those ancestors host, walked across processes of every
   user. A parent missing from the process listing is could-not-check. A process whose
   environment is withheld is not a root: every live window has a readable `cellctl`
   supervisor above it, which is. Any live marked process of a role that is not a signal
   candidate (shielded, under a live parent, or under a subreaper) keeps that role's record
   `alive`: a record is `dead` only when no marked process of the role is left running. cellctl drops an inherited
   marker from its own environment at start, so the cockpit processes it starts (a tmux
   server or window, a herdr or orca surface) never carry one. Only the role child and the
   comms service are stamped. Paths and command lines are never ownership proof.
   Where the OS withholds a process's environment (macOS does for its own platform
   binaries, such as a bare `sleep`), the marker cannot be read and the process is never
   signalled. One that stayed in the dead role child's process group keeps the record
   `alive`. One that left that group (its own process group or session) is not seen at all,
   and neither is anything descended from a session with no recorded role child; the record
   can then be cleared while it runs. On a Linux host where orphans re-parent to a child
   subreaper rather than pid 1, a real orphan is shielded (the subreaper is an ancestor of the
   invoking process) and is never signalled, but it is still listed: the record is `alive`
   and the preflight refuses, the safe direction.
4. **Asks once, on a terminal:** it prints the plan and asks
   `Clear stale state and stop N orphaned processes? [y/N]`. On `y` it sends SIGTERM to
   each orphan, waits a bounded five seconds, sends SIGKILL to survivors, and re-checks
   each by pid and start time. Before signalling it re-checks that each orphan's role
   lease is still free, and every signal is preceded by a fresh check that the pid still
   has the same start time and marker, then by a check of the kernel's own start time
   recorded at listing (an unreadable one is could-not-check, and nothing is signalled).
   Immediately before every signal the whole shield is also recomputed from a fresh
   listing, because a window can open while the prompt waits: a target no longer listed as
   the same orphan stops the run with nothing further signalled. It
   then closes a recorded comms surface whose identity still matches, runs the same code
   as `cadence recover --confirm-stopped` and `comms recover --confirm-stopped`, re-reads
   the state and continues the launch.
5. **Otherwise refuses as before.** That covers a non-terminal stdin or stdout, an answer
   other than `y`, and any `alive` or `could-not-check` record. The refusal prints the
   full inventory and the exact commands to run by hand, not just the first refusal it
   hits. Orphans with no stale record never blocked a launch, so declining to stop them
   lets the launch continue as it always has.

There is deliberately no `--yes` flag: clearing stays a human act, as `--confirm-stopped`
intends. `cellctl up <cell> --dry-run` (or `DRY_RUN=1`) prints the inventory and the plan
and changes nothing. The preflight never touches queued, held or acknowledged comms
messages, keys, configuration, worktrees, branches or the Go cache. When its plan stops or
clears anything, it points at `cellctl cache <cell> status` and
`cellctl cache <cell> recover --confirm-stopped` for a cache consumer record left by a process
it proved dead.

## Codex command environment

Host Codex launches use explicit command-environment settings instead of generated
Bash desk-tool wrappers. The Codex process retains its operator login and plugin
home. Its command subprocesses receive the cell's `HOME` and `USERPROFILE`, native
desk-tools PATH, cell credential directory, role and repository roots. GitHub CLI
configuration retains its original `GH_CONFIG_DIR`, XDG, or native platform location.
No credential value is copied onto launch arguments. Claude and Cursor keep their
existing environment adapters.

On macOS and Linux, the role prompt requires an explicit Bash `shell` argument and
`login=false` on command tools and propagates that requirement to subagents.
The current Codex CLI chooses its default shell from the account database and ignores
`SHELL`; this is an instruction to use the supported per-command override, not a claim
that cellctl changes that CLI default. The launch separately disables shell snapshots,
login-shell use and profile loading, sets `ZDOTDIR` to the cell home, and clears
`BASH_ENV` and `ENV`. The operator's shell configuration is never edited.
Windows retains its native shell and does not acquire a Bash or WSL dependency.
These settings are invocation-local; existing Codex sessions need restarting.
An operator's Codex environment include filters can still remove explicit values.
Such filters must admit the cell home, PATH, role/root and tick variables above;
cellctl does not silently broaden an operator's allowlist.

Codex's concurrent child-thread limit is resolved from this cell's effective roster
width. It excludes the primary thread. Each cadence pass resolves it again so a width
change or expiry takes effect. An interactive launch samples it at startup. Feature
checks ask the installed CLI for its effective state instead of requiring a literal
`multi_agent=true` entry; this does not prove account capacity or override managed
limits. See the [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).

## Cursor workspace admission

Install the Cursor bindings into each actual role workspace, for example:

```sh
deskinstall --harness cursor --repo /absolute/cell/worktrees/worker-desk
```

A local installation in the source checkout alone is insufficient: uncommitted skills
and configuration do not follow a Git worktree. Launch checks the actual workspace's
role skill, references and bindings. Cadence also refuses a missing or different
source `.cursor/cli.json`, so source permission denials are not silently lost.
Headless passes use `--print --force`; the CLI must advertise that explicit denials
remain enforced. No trust flag, sandbox override or permission-file rewrite is added.

## Lifetime and validation limits

The Go supervisor survives the end of each model turn while its cockpit process
remains running. Closing the cockpit or rebooting stops that process; no OS service
or startup registration is installed by this change. Start the same command again
to resume from the checkpoint. This is fixed-cadence supervision; event-triggered
acceleration through `deskmonitor` is not enabled by this mode.

Execution tests use compiled local fixtures, without model calls or live cockpit
mutation. They exercise multiple passes, deadlines, process-tree cleanup, checkpoint
restart/refusal, missing summaries, and concurrent ownership. Native Windows runtime
execution and complete Windows cockpit lifecycle are separate validation obligations;
Windows compilation alone does not establish those behaviors.

The offline Windows runtime checks can be run from `tools/desk` in PowerShell:

```powershell
go test ./internal/cellcadence ./internal/cellprocess
go test ./cmd/cellctl -run 'TestCodex|TestWindowsCommandEnvironment|TestInteractiveOwner'
```

These checks exercise compiled local fixtures and require no provider login. They do
not certify a complete Windows cell installation or an interactive cockpit launch.
