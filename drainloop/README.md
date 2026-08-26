# drainloop — a framework-agnostic drain harness

A small, self-contained drain engine you can `go get` or clone and run: **read a queue → claim
an item → dispatch it and await its result → land it → release the claim → move to the next →
idle**. Items are drained one at a time — at most one is in flight — matching the current
emit-one-instruction form, where the engine names exactly which dispatch to make and the result
is fed back before the next item is considered. The scheduler is deterministic code, so it never
has to live in an operator model's attention. This module is the generic, importable takeaway of a companion article on
why agent loops stall and how moving the scheduler out of the model fixes it; the article is
one directory up at [`../drain-engine.md`](../drain-engine.md).

Nothing here is tied to any particular queue, worker runtime, or infrastructure. The engine
knows only the six-method contract below; the queue, the claim store, and the dispatcher are
adapters you replace. The public core is deliberately **deskkit-free** — it imports no house
package (asserted by `TestNoDeskkitImports`), so it stays cloneable and importable on its own.

## Install

```sh
go get github.com/medici-finance/assay/drainloop
```

Or clone the containing repo and copy this directory. It is a standalone Go module
(`github.com/medici-finance/assay/drainloop`), so it builds and imports without the rest of
the tree.

## Quickstart

Requires Go 1.25+. From this directory:

```sh
go test ./...        # run the engine and its negative tests
go run ./cmd/demo    # drain five fake items with the stand-in adapters
```

The demo drains five items one at a time: it claims each item before dispatching it, awaits and
lands each echoed result, releases the claim, and stops when the queue is empty. You will see the scheduling
decisions (`CLAIM` / `DISPATCH` / `LAND` / `RELEASE` / `IDLE`) printed as they happen.

> This module lives inside the larger `medici-finance/assay` tree, which is a collection of
> standalone modules (no `go.work`). Prefix commands with `GOWORK=off` if your environment
> ever introduces a workspace, so the module builds on its own — exactly as this repo's CI
> runs it. A standalone clone needs no such flag.

## The contract (six methods)

An adapter implements the `Loop` interface in [`engine.go`](./engine.go). The engine owns the
pool arithmetic; the adapter owns everything role-specific, and role-specific behaviour is
confined to exactly two of the six methods.

| Method | Signature | Owns |
|--------|-----------|------|
| `Name` | `Name() string` | the loop's identity — logging + claim namespace |
| `SelectQueue` | `SelectQueue() ([]Item, error)` | where items come from; the engine does not know or care |
| `TierPolicy` | `TierPolicy(Item) (Tier, error)` | routing — return `TierHuman` to send an item to a human instead of a worker; the `error` distinguishes "no tier" from could-not-check |
| `Dispatch` | `Dispatch(Item, Tier) (Handle, error)` | carrying out one dispatch — the resolved `Tier` routes it to the right runner class; the only method that differs between "emit an instruction" and "spawn a process" |
| `Land` | `Land(Result) error` | recording one result (the `Item` is folded into `Result.Item`) — write evidence, release what the item held; **all other heterogeneity lives here** |
| `OnIdle` | `OnIdle() error` | what "empty" means — the engine's `Config.StopWhenIdle` decides stop vs poll |

Claiming is a separate small interface, `Claimer` (`Claim` / `Release`), because the dedupe
guarantee rides entirely on it: **an item ID is in flight at most once, for dispatchers that
route through `Claim`.** A dispatcher that skips the claim is outside the guarantee by
construction, not by policy.

`Handle` is an **interface** (`Done() <-chan Result`, `Item() Item`), so the same engine loop
drives synchronous dispatch (an already-resolved handle from `Resolved`) and asynchronous
dispatch (a handle backed by a real child process) without changing. That seam is why "emit an
instruction today, spawn a process later" swaps one method and touches nothing else.

## Why the contract stays small

Everything that varies between roles is pushed into `TierPolicy` and `Land`. Everything else —
the claim-before-dispatch chokepoint, the land-and-release cycle, the drain-continues-on-failure
discipline — is the engine's, written once and identical for every consumer. The moment the contract grows a
seventh method to accommodate one role, that method propagates to every consumer and the
engine stops being a scheduler and becomes a drawer of per-role policy. Keeping it at six is
the property that lets a second and third role adopt the engine without the first role's
decisions leaking into theirs. Treat a proposed new method as a design decision, not a patch.

Two disciplines the engine enforces rather than requests:

- **A failed dispatch does not abort the drain.** It is landed as `VerdictError` and its claim
  released, so one bad item cannot freeze the pool. See `TestDispatchFailureLandsAndContinues`.
- **Claiming is the single dedupe chokepoint.** A claim held elsewhere is a skip, never a
  second dispatch. See `TestClaimCollisionSkipsItem`.

## Optional layers (opt-in, deskkit-free)

Each of these is off at its zero value, so a `Config` that sets only `Loop`/`Claimer`/`PoolSize`
runs the plain six-method core. They are deskkit-free by design — the generic shape lives here;
a house wires its own implementation behind the interface.

| Layer | Where | What it does |
|-------|-------|--------------|
| `WorkEvidence` | `Config.Evidence` | probe before claim — if the work is already done elsewhere, land it without dispatch; a could-not-check probe SKIPS rather than duplicates |
| author-not-runner guard | `Config.RunnerID` + `CheckAuthorRunner` | an item whose `Implementer` equals the runner is routed to `Land` as held, never dispatched to its own author |
| `Journal` | `Config.Journal` | a structured `Event` per scheduling decision — attribution derived from the run, for replay |
| `RetryPolicy` | [`retry.go`](./retry.go) | the three-state retry taxonomy (transient / deterministic / **could-not-classify → route to a human**), with bounded attempts and backoff; a facility your `Land`/re-selection consults, kept off the six-method contract on purpose |
| `Timeouts` | [`journal.go`](./journal.go) | the schedule-to-start / start-to-close / heartbeat liveness taxonomy; the type lives here, enforcement (which reads live infra) is yours |

## Files

| File | What it is |
|------|-----------|
| [`item.go`](./item.go) | `Item`, `Tier`, `Verdict`, `EvidenceRow`, `Result`, `Handle` — the typed vocabulary |
| [`engine.go`](./engine.go) | the `Loop` / `Claimer` contract, `Config`, and the `Run` drain loop |
| [`claim.go`](./claim.go) | `FileClaim`, a stand-in claim store using `O_EXCL` for single-winner claims |
| [`adapters.go`](./adapters.go) | `MemoryQueue` + an echoing dispatcher — the stand-in adapters |
| [`evidence.go`](./evidence.go) | `WorkEvidence`, `CheckAuthorRunner` — the pre-dispatch gates |
| [`retry.go`](./retry.go) | `RetryPolicy` and the three-state `Class` taxonomy |
| [`journal.go`](./journal.go) | `Journal`/`Event` sink and the `Timeouts` liveness taxonomy |
| [`cmd/demo`](./cmd/demo) | a runnable demonstration |
| `*_test.go` | the negative tests: claim collision, dispatch failure, held item, could-not-check tier, the opt-in layers, and the deskkit-free assertion |

## Adapting it to your stack

Replace the three stand-ins, in order of how much of your infrastructure they touch:

1. **`SelectQueue` + `Land`** — point them at your real backlog (a tracker, a table, a board)
   and your real evidence sink. Derive the `Result` you land from structured `Rows` (command,
   exit code, key output), not a prose blob.
2. **`Dispatch`** — call your real worker: a subprocess, an HTTP endpoint, an agent spawn.
   Return an already-resolved `Handle` (`Resolved`) for synchronous work, or back a `Handle`
   with a channel for asynchronous work. The engine loop does not change either way.
3. **`Claim` / `Release`** — the `FileClaim` here is deliberately minimal. A production claim
   store adds stale-claim reclaim and a durable cross-machine lock; the "has this been done
   elsewhere?" probe is offered separately as the `WorkEvidence` hook, so you can wire it
   without replacing `FileClaim`.

The engine loop in the middle — the part that used to live in a model's attention — you keep
unchanged.

## What this does and does not give you

It gives you **attribution** (every scheduling decision is journalled, not narrated
afterward), **evidence** (a result derived from structured output), and **structural
separation** (the thing that schedules cannot be the thing that judges). It does **not** make a
result impossible to fake: a caller holding the right credentials can still forge one. That
close — the author of a change being unable to also approve it — belongs to distinct signing
identities and branch protection, outside this harness.

## Versioning

drainloop is homed inside `medici-finance/assay` and released under that repo's umbrella
`assay/vX.Y.Z` tag — it is **source** you clone or import at a tagged commit, not a per-tool
binary release, so a consumer pins the umbrella version (never a `drainloop/vX`). If independent
versioning or heavy external adoption ever appears, the promotion trigger is to move it to its
own repository with its own release tags — a later call recorded in the convergence plan.
