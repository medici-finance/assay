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

## Worktree hygiene

Worktree sprawl is owned by `deskwt prune` — it runs at boot and under its own interval
supervisor; no loop carries an hourly prune tick and nobody hand-deletes worktrees (the
ENFILE incident, 2026-07-23: sprawl exhausted the system open-file table).

## Driver-act runsheet entry

- **An escalation that is an ACT only the driver can perform** (not a decision) also gets a
  `RUNSHEET.md` entry per the `human-runsheet` skill — the filed issue stays the escalation, the
  runsheet is the exact command the driver runs.

## Boundary — what is not here yet

Only blocks the desk bodies stated IDENTICALLY moved here; moving text is not the place to choose
between two readings of a rule. Blocks the bodies still state in their own, differing words — the
stop-flag check, the fresh-sweep hard gate, refresh-don't-remember, file-and-exit, the receipt
line, the output contract, the subagent-verdict re-probe, the changelog-fragment clause — stay in
each body until one wording is settled for each; each then moves here in the same way, or, if it is
a hard gate, into the resident rules.
