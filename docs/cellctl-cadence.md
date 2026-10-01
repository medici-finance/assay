# Host desk cadence

`cellctl` can own the clock for a host desk whose harness cannot wake itself after
its current turn. The cockpit runs one foreground Go supervisor; that supervisor
starts a bounded role pass, waits for completion, then waits the configured interval.
Herdr and Orca supply the terminal. Codex and Cursor supply the agent pass.

```sh
cellctl up house --cockpit herdr --harness codex --cadence 30m --tick-budget 20m
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

The cadence is opt-in. Existing interactive launches retain their execution mode.
`--cadence off` overrides a saved cadence. A budget includes the role's exit reserve;
its minimum is 61 seconds and default is 20 minutes. Choose a budget that also fits
the actual work; a review may need substantially more time. Every pass receives
`ASSAY_TICK=1` and `ASSAY_TICK_DEADLINE` in seconds, so it performs one sweep and exits.
The supervisor sends no periodic keystrokes or prompts into a busy interactive turn.

## Configuration and ownership

The cadence uses the same resolved model, policy arguments, role, repository roots,
shim path, cockpit and worktree as a normal desk launch. A cockpit command explicitly
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

A supervisor crash during a pass is ambiguous: the agent might still be running.
An unfinished checkpoint therefore refuses all replacement launches, even when the
supervisor lock is free. Inspect and stop the prior harness and its descendants first.
Only after confirming that they have stopped, explicitly reset that role:

```sh
cellctl cadence house recover pr-review-desk --confirm-stopped
```

The confirmation is an operator assertion, not a PID-based proof. Recovery does not
kill an arbitrary recorded PID. Ordinary resume never clears this condition.

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
