<!-- GENERATED from plugins/assay/references/desk-common.md by deskinstall --harness cursor. Do not edit. -->
# Desk-common — the procedure every desk role shares

<!-- assay:harnesslint non-matrix-reference — harness-neutral procedure shared by the desk-role skills, not a per-harness capability binding; the capability-to-mechanism matrix and the per-skill degradation cells live in claude-code.md, codex.md and cursor.md -->

The five desk-role skills (`the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk`,
`verify-desk`) each carried the same procedure blocks, word for word. This file states each such
block ONCE. Where a desk body used to carry the block it now carries a pointer here, under the same
heading, so an in-body `§` cross-reference still lands; a sentence that belongs to one desk alone
stays in that desk's body, directly under the pointer.

**Procedure only — never a hard gate.** A reference file is read only when a session chooses to
read it, so a gate placed here is a gate that can be skipped. The hard gates — the trust gate, the
desk's output floor, the git push policy — are therefore never moved here: they stay resident in
each desk body (the push policy in the always-injected resident rules as well). The shared rule
blocks a loop must hold in hand while it runs (git push policy, no-attribution, escalation labels,
insight-routing, the reversibility test, tick mode, the cross-desk lane verbs) are likewise NOT
here: the desk bodies carry them as copies generated from one declared source and byte-checked, so
they stay resident without drifting.

**House values never appear in this file.** It states the procedure and its shape; the concrete
values it depends on (roster, config home, stream roots, the driver's name) resolve from the
consuming project's own house-rules doc, exactly as for
[`desk-shell.md`](./desk-shell.md).

## Liveness contract

The desk bodies' `## Liveness contract (binding)` sections point here. A body may add a sentence
that is its own — how that desk creates its fixed-cadence sweep, for example — directly under its
pointer.

A standing liveness contract binds this window from boot: start the standing
self-scheduled loop (`capability:durable-monitor` — best-effort, never the sole
wake signal; the fixed-cadence board sweep is the real liveness backstop and the
always-on observability service its durable home) BEFORE the first sweep and keep
it ticking for the life of the window; every tick re-sweeps this desk's own queue fresh; every relay (a
cross-session hand-over, on the lane) is acknowledged — `deskcomms ack` — or filed, never
assumed delivered.
The desk runs **default-forward** — never ask the driver what to work on next:
a driver scope instruction narrows preference, not a cage — when the scoped
batch drains, note the transition in the hand-off note and widen back to the
standing queue. Checkpoints state their default and continue; standing down
requires an empty standing queue after a fresh sweep PLUS a hand-off artifact
on the driver surface, and a manual human kick that moves queued work is an
incident to file on the project's methodology tracker. Hard gates (human-gated
decisions, budgets, breakers, explicit stop-orders) are unchanged.

## Supervision and context

The supervisor reacts to events; it is not a second worker or a model-powered timer.
After dispatch, retain the child/command handles and the next required sweep/stop-check
deadline. Return to model work only for a child result or actionable message, human
steering, a failed watcher, or a due role check. On a completion, consume that result,
perform the required independent verdict readback, and refill the available slot.
A notification is a cue, never evidence of a posted verdict or a drained queue.

When nothing actionable changed, block on the existing event/command handle until the
next required deadline, using the longest wait the exposed tool and harness instructions
permit. Do not invent a shorter polling interval. On an early timeout, re-arm the wait
with the remaining time; do not inventory agents/processes, reread logs or manuals,
refresh the whole board, send progress pings, or print an unchanged status summary.
Run the required stop/liveness and fresh queue checks when due, then return to waiting.
A failed read remains could-not-check. Completion events refill slots without waiting
for the next queue sweep; the role's sweep cadence and stop obligations remain intact.

Keep one owner for each routine CI/log watch. Use a non-model watcher with the existing
cadence and retry-after/backoff, reporting terminal status, actionable failure, watcher
failure or the role's required liveness signal. Do not create an agent whose only job
is to repeatedly ask whether another agent or CI job has finished. The supervisor does
not duplicate the item's routine watch, but still performs every required independent
verdict readback and final ready/claim check. Save full logs to files.

If only a future cadence is due, wait interruptibly in the current turn. Never end a
turn just to have a standing objective immediately restart the same unchanged checks.
If the harness cannot wait efficiently or cannot wake after the turn ends, state that
capability gap and use the existing operator-selected outer cadence mechanism; do not
imitate a durable monitor with repeated model turns or a new polling daemon. Switching
launch modes requires the existing clean handoff; do not abandon children, claims or
processes, change the driver's Goal, or claim idle without the required fresh sweep.

Read the role and applicable references at boot. While their content remains available
in context, continue from it; a new turn does not require rereading unchanged manuals.
After compaction or a source change, recover the missing or changed instructions before
acting. Use the existing [standing note](./standing-note.md), not a second checkpoint
schema: keep item/repository, lane and exact head, worktree, child and command handles,
unresolved findings/holds and evidence paths under In-flight; keep last sweep and next
due check under Board/Monitors. This remains a re-probe index, never authority, fresh
queue evidence or a verdict.
Save full logs to files and bring back status, relevant failure excerpts and paths.

Give each new item a fresh agent context with its admitted assignment and packet paths,
required role instructions and shell/identity constraints. Reuse a child for follow-up
on that same item and lane, not as a permanent slot accumulating unrelated assignments.
Before retiring a child, retain its evidence and resolve live processes/claims through
their existing lifecycle. A fresh context must not bypass a refusal, outstanding finding,
claim or human hold. Re-review handoffs carry prior findings and their evidence; separate
review lanes remain independent and do not inherit the implementer's conclusions.

Reduce duplicate supervision, not review scope: retain the required source/diff reads,
omission follow-up, tests, model admission, independent verdicts and consequential
freshness checks. Use the harness binding for concrete wait and dispatch mechanisms.

## Review observation handoff

This procedure migrates readers; it grants no executor or publication authority.
See `docs/review-efficiency-compatibility.md` in the source distribution for the
producer/plugin/schema matrix and retirement inventory. Qualified producer baseline:
released desk-tools v1.0.59 (`review-queue`, offline selection, advisory preparation).
Use matching source-qualified caller adapters; installed-cell adoption is separate.

The launcher supplies current approved `SelectionPolicy` JSON through
`PR_MONITOR_REVIEW_POLICY` and nonnegative available proposal capacity through
`PR_MONITOR_REVIEW_CAPACITY`. The policy has explicit Owner, Revision,
Evaluator `review-selection-v1`, Unmapped behavior and Priorities (an explicit
empty list is allowed; absent is unknown). Do not substitute the observation’s
embedded policy digest for approval of the priority list.

Select classified mode by setting `PR_MONITOR_REVIEW_OBSERVATION` to a unique,
private caller-minted output path and run the existing `pr-monitor.sh` on the full
configured repo set. That script collects `deskboard review-queue` once, passes
requested scope, current time and a five-minute freshness cutoff to
`reviewloop plan --snapshot FILE --selection JSON --now RFC3339`, and emits the
complete private plan at `FILE.plan`, emitting `REVIEW-PLAN` and
`REVIEW-OBSERVATION` artifact paths rather than full populations on each wake. The script never launches
reviewers. Its event baseline comes from the same population. Keep its due cadence
wake and stop/health checks even when the PR event dimensions did not change.

Treat only a successful current cycle as reusable. On a selected-mode failure,
exit 2 and `MONITOR-DEGRADED` hold the whole cycle and invalidate the output;
prior event baselines remain. A quarantine or failed repo is never removed from
scope. On a public repo, even one outside/unblessed PR holds the entire classified
cycle; continue through the supported legacy recipe on a later cycle until that
quarantine is resolved, or retain the visible hold. This is an operational limitation
of the complete observation, not permission to bypass public-author admission.
If the hold cannot invalidate a prior artifact it says so on a further
`MONITOR-DEGRADED` line; that file is never this cycle's output. Repair/retry, or deliberately choose the supported prior recipe on a later
cycle; never auto-fallback within the failed cycle. Rerun pure selection for a
capacity or priority change using that same supported invocation while applicability
holds; execute candidates in returned policy order. Keep the observation for
`deskdispatch ... --kit review --observation FILE`; fresh effect checks still run.

With matching current scope/policy and a five-minute cutoff, coordinator and inbox
may consume `deskboard throughput --review-observation FILE`. Review depth is
validated from the same complete population and reports its collection cutoff.
Scope/policy/schema/time mismatch blinds review depth without a second review scan;
other pipeline stages remain independently readable. Inbox forwards the selected
`PR_MONITOR_REVIEW_OBSERVATION` to that reader. This adapter requires the source
change shipping with this recipe; v1.0.59 alone does not contain this new flag.

The prior supported recipe remains: run the unselected event monitor, retain the
independent classified `deskboard actions --delta --quiet` cadence, and plan with
`reviewloop plan --actions ACTIONS --prs PRS` using unique scratch files from
`deskboard actions --out ACTIONS` and `deskboard prs --out PRS`. Legacy dispatch
omits `--observation`, and legacy throughput omits `--review-observation`. Run only
one recipe’s effects. Before rollback, retain outstanding findings/handles, invalidate
new-mode artifacts and reconcile completed effects; a mode switch does not cancel a
claim or turn an unknown queue into idle.

## Worktree hygiene

Worktree sprawl is owned by `deskwt prune` — it runs at boot and under its own interval
supervisor; no loop carries an hourly prune tick and nobody hand-deletes worktrees (the
ENFILE incident, 2026-07-23: sprawl exhausted the system open-file table).

## Driver-act runsheet entry

- **An escalation that is an ACT only the driver can perform** (not a decision) also gets a
  `RUNSHEET.md` entry per the `human-runsheet` skill — the filed issue stays the escalation, the
  runsheet is the exact command the driver runs. The issue body carries the same act as an Act
  block (`ask-decision` §"Act — the shape of the fifth part"). `deskfile new` enforces that (exit 5)
  only when the filing is labelled `human-only` or its body's first non-blank line opens with
  `BLOCKED-ON-HUMAN`; filed under `question`, `help wanted` or `needs-decision` alone, the Act block
  rests on this rule, with no tool backstop.

## Boundary — what is not here yet

Only blocks the desk bodies stated IDENTICALLY moved here; moving text is not the place to choose
between two readings of a rule. Blocks the bodies still state in their own, differing words — the
stop-flag check, the fresh-sweep hard gate, refresh-don't-remember, file-and-exit, the receipt
line, the output contract, the subagent-verdict re-probe, the changelog-fragment clause — stay in
each body until one wording is settled for each; each then moves here in the same way, or, if it is
a hard gate, into the resident rules.
