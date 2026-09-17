---
stream: desk-supervision
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: [351]
board: generated
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

## The worker-operations delta (briefs 13-15) — a second, orthogonal plane

Briefs 01-09 are the **DERIVED plane**: liveness *reclaimed from artifacts* (branch-SHA, PR
updates, audit) the worker cannot fake, acting on a **dead or stalled** worker. Briefs 13-15
add the **WORKER-OPERATIONS plane**: *self-reported vitals* (context-%, tokens, session age,
subagents, model) that only the session itself can know, acting on a **healthy-but-full**
worker — to recycle it gracefully *before* it dies. The two do not conflict: different subject
(dead vs full), different source (derived vs self-report), different trigger (silence vs
budget). The house "never self-report" invariant is a *work-plane / derived-liveness* rule — a
session reporting its own context % makes no claim about the work, so there is no collision, and
because a `could-not-check` vital yields no recycle signal at all, nothing on this plane can
suppress a reclaim on the other. The full framing is at the top of `desk-supervision/13`.

- **13** fills the reserved `tokens` stub in `desksupervise-status-v1` with a self-reported
  `resource` block (three-state: measured / could-not-check / null, never a fabricated 0),
  piggybacked on the per-tick roster beacon write. This is the ONLY strictly-new collection.
- **14** adds a NEW recycle trigger on brief 04's lifecycle hooks: a *graceful* recycle of a
  healthy worker past its context / age budget (hand off to durable state, exit, respawn), with
  a hard-recycle backstop reusing brief 02's per-run stop for a worker that will not cooperate.
- **15** stands a LOCAL supervisor host on the operator's own (non-k8s) cell via `cellctl`, so
  supervision + recycle reach the operator's own desks, and aggregates per-cell vitals into a
  fleet ops view — separate from the statusgen work board.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Observable probes + the `desksupervise` observer — liveness that bites](brief-01-observable-probes-and-observer.md) | 0 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #411 @ ee156917abb50f02d5a92649a8bccf563d22474f) |
| 02 | [Per-run stop signal — `STOP.run.<key>` flag + desk-window stop on observer signal](brief-02-run-stop-signal.md) | 1 | M | done | 2026-09-06 opus-4.8[1m]-verifier | 2026-09-07 assay-reviewer-app[bot] (approved PR #561 @ 311bb7ccad3e6e59c92f40556fae3b3bb915ec8a) |
| 03 | [Eligibility reconciliation — stop a run whose item became ineligible](brief-03-eligibility-reconcile.md) | 2 | M | done | 2026-09-11 sonnet-5-verifier | 2026-09-12 assay-reviewer-app[bot] (approved PR #910 @ 53b198c801cc5f21d6644edd39c7e63f16dad727) |
| 04 | [Lifecycle hooks — after-create / before-run / after-run / before-remove from config home](brief-04-lifecycle-hooks.md) | 1 | M | implemented | — | — |
| 05 | [Per-class concurrency reservation — fresh / resume / rework caps in the planner](brief-05-per-class-caps.md) | 0 | S | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #412 @ 4db07e12b5f6821104244d0386f062851144b7bb) |
| 06 | [Workpad — one upserted progress comment per PR](brief-06-workpad.md) | 0 | M | done | 2026-09-11 sonnet-5-verifier | 2026-09-12 assay-reviewer-app[bot] (approved PR #913 @ 01843e69cf9de3799d79ecf3182e03323dc8f0e6) |
| 07 | [Runtime snapshot — `desksupervise status` for operators and the console](brief-07-runtime-snapshot.md) | 1 | M | done | 2026-09-06 opus-4.8[1m]-verifier | 2026-09-07 assay-reviewer-app[bot] (approved PR #352 @ 496796982b573be17a032163cd3f6423e58be239) |
| 08 | [Objectives over transitions — measure an objective-style worker kit with skillbench](brief-08-objectives-over-transitions.md) | 1 | M | todo | — | — |
| 09 | [Per-push CI fan-out — trigger selection so a docs-only push stops paying for a Go build](brief-09-ci-fanout-per-push.md) | 0 | S | implemented | — | — |
| 10 | [Confirm or repair the workflow App wiring — one identity holding workflows:write, installed and scope-proven](brief-10-workflow-app-wiring.md) | 0 | M | blocked | — | — |
| 11 | [The single-workflow-only-PR contract, and the verb by which the workflow App writes and lands it](brief-11-workflow-only-pr-contract.md) | 1 | M | blocked | — | — |
| 12 | [Retire the staged-copy hand-landing once the workflow App PR path is proven](brief-12-retire-staged-copy-landing.md) | 2 | M | blocked | — | — |
| 13 | [Worker-operations vitals — the self-report resource block](brief-13-worker-operations-vitals.md) | 2 | M | todo | — | — |
| 14 | [Budget-driven recycle — retire a healthy worker before it degrades](brief-14-budget-driven-recycle.md) | 3 | M | todo | — | — |
| 15 | [Local supervisor host + multi-cell vitals aggregation](brief-15-local-supervisor-host-and-aggregation.md) | 4 | M | todo | — | — |
<!-- statusgen:briefs:end -->

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

Smallest unblocking move: land 01. Briefs 05, 06 and 09 are independent of it and can run in
the same wave; 08 waits on 06 because an objective-style prompt leans on the workpad for
continuity across attempts.

**Brief 09 sits in this stream, and off the critical path, on purpose.** It is the same
concern as 05 one layer down: 05 caps how many workers the desk runs at once, and 09 caps
what each of their pushes then costs on a runner pool two runners wide. Neither blocks the
other — 09 touches no engine code and no desk verb, only which CI workflows a given diff
shape asks a question of — so it is wave 0 with no `depends:` and no `unblocks:`.

**Briefs 10 → 11 → 12 are a second, self-contained chain — the workflow-landing lane.** They
fix the reason 09 cannot reach `verified` today: 09's workflow edits are delivered as a staged
copy a human must hand-land, and #1185 is that hand-copy still not landed. The chain wires the
**workflow App** (the single `workflows: write` identity) to author a workflow-only PR, so a
workflow change lands on its own reviewable PR instead of a staged copy that stalls and drifts.
All three cite [`DR-workflow-app-landing`](../decisions/DR-workflow-app-landing.md) and are
`gate: human`: the DR proposes the rule and each brief's human gate confirms a piece of it.
The chain touches no engine code and is independent of the `01 → 02 → 03` supervision path —
its head, brief 10, is the App's ground truth (installed? correctly scoped?), which is a
provisioning question a human may have to answer before 11 and 12 can proceed.

**The vitals delta rides a third, independent chain off 07.** Briefs 13-15 do not change the
original head — they hang off the built machinery, and they are unrelated to the workflow-landing
lane above (no shared files, no shared gate). `13` needs the snapshot + schema (`07`), `14` needs
the vitals (`13`) plus the lifecycle hooks (`04`), and `15` needs both the vitals (`13`) and
the recycle (`14`). So the longest chain in the stream is now the delta's:
`01 → 07 → 13 → 14 → 15`. Its real head is still `01` (no probe, no observer, nothing to
snapshot), which is already `done` — so the delta's smallest unblocking move is `13`, gated
only by `07` landing (done). `14` and `15` are human-gated (a new autonomous stop of healthy
work; a persistent local host under operator credentials), so each also waits on its decision
issue, not just its `depends:`.

## Dependency waves

```
Wave 0: [01 probes+observer]  [05 per-class caps]  [06 workpad]  [09 CI fan-out]  [10 workflow-App wiring]
Wave 1: [02 run-stop] ← 01    [04 hooks] ← 01    [07 snapshot] ← 01    [08 objectives A/B] ← 06    [11 workflow-only PR] ← 10
Wave 2: [03 reconcile] ← 01, 02    [12 retire staging] ← 11    [13 vitals resource block] ← 07
Wave 3: [14 budget-driven recycle] ← 04, 13
Wave 4: [15 local host + fleet aggregate] ← 13, 14
```

Critical paths: `01 → 02 → 03` (supervision), `10 → 11 → 12` (workflow landing), and
`01 → 07 → 13 → 14 → 15` (vitals delta) — three independent chains.

## Shared conventions

- Every new verb follows the deskkit contract: kill switch first, one audit line per
  invocation, exit 0 ok · 3 disabled · 5 refused · 6 unverifiable, fail closed. A
  could-not-check probe is never read as "no life" and never as "eligible".
- The observer is **read-mostly**: its only writes are a claim release through
  `deskkit.ReleaseMatching`, a stop-flag file in the state directory, and a filed issue —
  never a PR write, never a worktree delete.
- Nothing in this stream weakens a guard. A stop flag can only halt; it cannot authorise.
- Public-tree self-containment: briefs here name no private repo, machine path or session.
