# Host desk cadence

`cellctl` can own the clock for a host desk whose harness cannot wake itself after
its current turn. The cockpit runs one foreground Go supervisor; that supervisor
starts a bounded role pass, waits for completion, then waits the configured interval.
Herdr and Orca supply the terminal. Codex and Cursor supply the agent pass.

```sh
cellctl up house --cockpit herdr --harness codex
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

House desks whose resolved harness is Codex default to a five-minute interval and the
existing 20-minute pass budget. This applies to `desk` and each role opened by `up`,
including harnesses selected by model policy. Claude, Cursor and non-house cells keep
their existing defaults. Explicit `--cadence` / `CELL_CADENCE` values take precedence;
`--cadence off` or saved `CELL_CADENCE=off` selects an interactive Codex session.
`--tick-budget` / `CELL_TICK_BUDGET` overrides the budget independently for a defaulted
Codex cadence. Defaults are resolved at launch and are not written to cell.env.
Already-running interactive sessions retain their execution mode until restarted.

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

The explicit cadence also works on older installations where Codex cadence is opt-in.
After installing this default, the `--cadence` and `--tick-budget` flags may be omitted.
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
