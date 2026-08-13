# loopengine — deterministic drain engine (loop-engine/01)

The outer control loop of the desk's archetype-A drains — **read the queue → hold a pool of N
in-flight workers → claim → dispatch → land each result as it returns → refill → idle-poll** —
moved out of the operator model's attention and into code, on top of the deterministic
instruments `tools/desk` already provides (`deskkit.Guard()`, audit, the shared claims dir).

Design doc: [`docs/loop-engine-architecture.md`](../../../../docs/loop-engine-architecture.md)
(§4 contract, §7 skeleton, §9 open questions). This package is the real thing built from §7.

## The contract is frozen (arch doc §8 "contract erosion")

The `Loop` interface — `Name / SelectQueue / TierPolicy / Dispatch / Land / OnIdle` — is
deliberately small. Heterogeneity lives in `Land` and `TierPolicy`. **Anything that does not
fit those two hooks does NOT go in the engine**: a proposed new hook is a design review (file
a contract-erosion issue), not a patch. Four downstream briefs (02–05) consume this contract;
an economy-tier hook added here propagates to every consumer.

## Dispatch mode — interim now, native primitive later (Open Question 9.1, resolved)

A Go conductor **cannot call the harness `Agent` tool** — there is no Go binding to it. So
`Run()` ships in the **interim mode**: the Go engine owns ALL scheduler state deterministically
(pool occupancy, claims, refill, land, `is_done`, `Guard`) and drives dispatch through
`Loop.Dispatch`, which **emits an exact dispatch instruction** the operator model executes
verbatim as an `Agent` call, then feeds a structured `Result` back through the returned
`Handle`. Zero scheduler state lives in the model's attention — one iteration is *"the engine
says exactly which dispatch to make; make it; feed the Result back"*.

**The whole win — the outer loop leaving the model's attention — is achieved by the Go engine
regardless of dispatch mode.** Interim mode is not a lesser version of the win; it is the win,
with a human-in-the-loop dispatch step instead of a native spawn.

**Eventual upgrade (native primitive):** a future Go→`Agent` binding, or running `Run()` as a
Workflow so dispatched workers are real child processes, is a **drop-in swap of the
`Loop.Dispatch` implementation only** — the engine loop, the `Loop`/`Handle`/`Result`
contract, and every consumer are unchanged. `Run()` is written so that swap touches nothing
else.

## Honest bound (arch doc §1.1) — do not overclaim

What the engine buys is **attribution, audit, and structural separation**, NOT cryptographic
un-forgeability. No inline-verify path exists (the only way an item leaves the queue is
`Dispatch` — the #541 class is unrepresentable), `author != runner` is a typed structural
guard (`CheckAuthorRunner`, distinct exit code 7), and Evidence is rendered from the dispatched
runner's structured `Result` (command → exit → key output), not free text. But a session
holding App signing material can still forge; the true close is `author != approver` between
distinct Apps + branch protection, which is out of scope here.

## Consumers

- `tools/desk/cmd/verifyloop` — the verify-desk reference consumer (brief-01). Proves the
  contract end-to-end; the autonomous cutover to the standing verify window is **gate: human**.
