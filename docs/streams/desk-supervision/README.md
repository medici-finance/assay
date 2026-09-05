---
stream: desk-supervision
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: [351]
---

# desk-supervision Stream

Make the drain engine's designed supervision **bite on live desks**: a running worker that
goes silent is detected and reclaimed by code, a worker whose item became ineligible mid-run
(PR merged or closed, brief flipped, claim released) is stopped by code, the runtime state of
every in-flight dispatch is readable as one snapshot, and the per-run envelope (worktree
hooks, per-class concurrency, the worker's own progress record) is configuration rather than
skill prose.

**Where this comes from.** OpenAI published Symphony (2026-04-27), an Apache-2.0 spec for a
service that turns an issue tracker into a control plane for coding agents:
[the post](https://openai.com/index/open-source-codex-orchestration-symphony/) and
[`SPEC.md`](https://github.com/openai/symphony/blob/main/SPEC.md). Its model of *work* is
weaker than this method's (one role, no independent review or verify, trust delegated to the
tracker's ACL), but its model of the *runner* is stronger: stall detection with kill and
backoff retry (§8.4–8.5), tracker-state reconciliation that terminates ineligible runs
(§8.5 part B), repo-owned lifecycle hooks (§5.3.4), per-state concurrency caps (§5.3.5), a
single persistent "workpad" comment as the agent's progress record (its example
`WORKFLOW.md`), a structured runtime snapshot (§13.3), and the lesson that agents should be
handed objectives, not state-machine transitions. This stream adopts exactly those runner
pieces and nothing of the work model — PRs stay the unit; the desks, gates and identities
stay as they are.

**What is measured, not assumed** (2026-09-02 @ `30c9934`, `origin/main`):

- `tools/desk/internal/loopengine/liveness.go` already carries the full liveness taxonomy —
  `LivenessPolicy{ScheduleToStart, StartToClose (per tier), HeartbeatGap}`, a
  `DefaultLivenessPolicy()` of 10m / 20–90m / 20m, the three-state `ObservableProbe`
  contract, and conservative reclaim through the lock-guarded single winner. `retry.go`
  carries the three-state retry classifier with backoff; `recover.go` replays the journal
  on restart; `journal.go` writes scheduling events to the audit stream.
- **None of it enforces anything today.** `engine.go` (line ~220) says it plainly: `Observe`
  is consumer-supplied and *"with Observe nil, liveness is therefore inert."* A grep of
  `tools/desk/cmd/` for `ObservableProbe` returns **zero** implementations. Every consumer
  (`fanoutloop`, `verifyloop`) exposes only `plan`; the autonomous `run` driver is a
  human-gated cutover because a Go conductor cannot call the harness's agent tool. So the
  policy exists, the probes do not, and no process evaluates the policy against live claims.
- There is **no run-stop primitive**. `tools/desk/internal/deskkit/killswitch.go` knows `DISABLED`, `STOP`,
  `STOP.<loop>` and a `HEARTBEAT` dead-man lease (24h) — all loop-wide. Nothing stops ONE run.
- There is **no mid-run eligibility check**. `engine.go` has no reconcile step; a worker
  whose PR a human merged keeps running until it finishes or its 120-minute stale-claim
  backstop fires. The rule "merged or closed PR = done, stop" is skill prose only.
- Pool width is role-keyed (`tools/desk/internal/deskkit/width.go`, `widthstore.go`): one number per loop, no
  per-class (fresh / resume / rework) reservation. "Resuming started work outranks a fresh
  brief" is prose in `worker-desk`.
- The per-run envelope is prose residue: `KUBECONFIG=/dev/null` lives in
  `tools/desk/cmd/deskdispatch/references/common-clauses.md`; worktree-local credential helper and
  commit identity are `deskwt role-init` and skill text; nothing runs at run-end.
- No verb upserts a single progress comment; `deskreply` posts a new comment each time and
  the outward-write budgets exist to police the resulting sprawl.

## The seam — decided here

**An observer, not a driver.** The engine cannot become the live desk's driver without the
human-gated cutover, and this stream does not wait on it. Instead a standalone, read-mostly
`desksupervise` loop (the shape `deskwt prune --interval` already has) evaluates the
existing `LivenessPolicy` over the live claim set every tick, using house-implemented
`ObservableProbes`, and acts through primitives that already exist or that this stream adds
as narrow verbs: release a claim via the lock-guarded path, arm a per-run stop flag, file a
`could-not-check` issue. When the driver cutover lands, the same probes and the same policy
plug into `Config.Observe` unchanged — the observer is the interim enforcement, not a fork.

**Kill is layered, never single.** A stop must reach a worker through two independent
components: a per-run stop flag the worker's next desk verb refuses on (guard-enforced,
cooperative), and the harness-side stop the desk window issues when the observer signals
(process-level in container mode). Each fails for a different reason in a different place.

**Hooks come from the operator's config home, never from the item's tree.** Symphony reads
hooks from the repository being worked on. Here an item's tree may be an untrusted head, so a
hook file inside it would be arbitrary shell under the desk's credentials. Hooks live in the
desk's state directory only.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Observable probes + the `desksupervise` observer — liveness that bites](brief-01-observable-probes-and-observer.md) | 0 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #411 @ ee156917abb50f02d5a92649a8bccf563d22474f) |
| 02 | [Per-run stop signal — `STOP.run.<key>` flag + desk-window stop on observer signal](brief-02-run-stop-signal.md) | 1 | M | todo | — | — |
| 03 | [Eligibility reconciliation — stop a run whose item became ineligible](brief-03-eligibility-reconcile.md) | 2 | M | todo | — | — |
| 04 | [Lifecycle hooks — after-create / before-run / after-run / before-remove from config home](brief-04-lifecycle-hooks.md) — **human gate** | 1 | M | implemented | — | — |
| 05 | [Per-class concurrency reservation — fresh / resume / rework caps in the planner](brief-05-per-class-caps.md) | 0 | S | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #412 @ 4db07e12b5f6821104244d0386f062851144b7bb) |
| 06 | [Workpad — one upserted progress comment per PR](brief-06-workpad.md) | 0 | M | implemented | — | — |
| 07 | [Runtime snapshot — `desksupervise status` for operators and the console](brief-07-runtime-snapshot.md) | 1 | M | implemented | — | — |
| 08 | [Objectives over transitions — measure an objective-style worker kit with skillbench](brief-08-objectives-over-transitions.md) | 1 | M | todo | — | — |

## Critical path

`desk-supervision/01` (probes + observer) → `desk-supervision/02` (per-run stop) →
`desk-supervision/03` (eligibility reconciliation).

**The head was verified before authoring, and it is not where a reader would guess.** The
tempting first step is "write the stall timer" — but the timer exists (`liveness.go`), and a
second timer against the roster beacons would measure the wrong thing: a beacon is
per-session, self-declared at `deskroster set` time and considered fresh for 60 minutes, so a
wedged worker under a live desk window reads alive indefinitely. The real head is that
**no probe implementation exists and no process evaluates the policy**. Brief 01 supplies
both, read-only, and proves reclaim-eligibility on a fixture with a dead worker. Nothing
downstream can be verified without it: 02's stop needs a run to name, 03's reconciliation
needs the observer tick to hang off, 07's snapshot is the observer's own state rendered.

Smallest unblocking move: land 01. Briefs 05 and 06 are independent of it and can run in the
same wave; 08 waits on 06 because an objective-style prompt leans on the workpad for
continuity across attempts.

## Dependency waves

```
Wave 0: [01 probes+observer]  [05 per-class caps]  [06 workpad]
Wave 1: [02 run-stop] ← 01    [04 hooks] ← 01    [07 snapshot] ← 01    [08 objectives A/B] ← 06
Wave 2: [03 reconcile] ← 01, 02
```

Critical path: `01 → 02 → 03`.

## Shared conventions

- Every new verb follows the deskkit contract: kill switch first, one audit line per
  invocation, exit 0 ok · 3 disabled · 5 refused · 6 unverifiable, fail closed. A
  could-not-check probe is never read as "no life" and never as "eligible".
- The observer is **read-mostly**: its only writes are a claim release through
  `deskkit.ReleaseMatching`, a stop-flag file in the state directory, and a filed issue —
  never a PR write, never a worktree delete.
- Nothing in this stream weakens a guard. A stop flag can only halt; it cannot authorise.
- Public-tree self-containment: briefs here name no private repo, machine path or session.
