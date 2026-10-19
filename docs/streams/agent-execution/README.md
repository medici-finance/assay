---
stream: agent-execution
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: [2500]
board: generated
spec: docs/proposals/non-ci-architecture/27-agent-execution-environments.md
---

# Agent execution environments

Deliver portable, qualified execution through the existing runner, workspace, coordination
and effect owners. The [future-state amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md)
is the controlling specification. These are authored implementation obligations; every
brief begins todo and no runtime is activated. The stream is parked: its controlling
amendment is a requested proposal, not an approved spec, so admission to dispatch is
pending. Activation needs that amendment's own approval provenance recorded on its spec;
the earlier revision-bound architecture decision does not supply it. Later adoption and
activation gates stay separate, and existing human and deployment gates still bind consumers.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Qualified harness, model and backend capability contract](brief-01-qualified-execution-profile.md) | 1 | M | todo | — | — |
| 02 | [Prepared-environment receipts and reusable workspace port](brief-02-prepared-environment-receipts.md) | 2 | M | todo | — | — |
| 03 | [Enforce qualified profiles and workspace readiness in local execution](brief-03-local-readiness-enforcement.md) | 8 | M | todo | — | — |
| 04 | [Qualify restored execution against ownership and external effects](brief-04-recovery-effect-qualification.md) | 12 | M | todo | — | — |
| 05 | [Expose preparation, execution, recovery and acceptance separately](brief-05-lifecycle-projection.md) | 13 | M | todo | — | — |
| 06 | [Investigate AX compatibility through offline contract fixtures](brief-06-ax-offline-compatibility.md) | 3 | M | todo | — | — |
| 07 | [Evaluate governed harness refinement with isolated offline fixtures](brief-07-harness-refinement-evaluation.md) | 3 | M | todo | — | — |
| 08 | [Qualify a Kubernetes verification runner through the existing supervisor](brief-08-qualify-a-kubernetes-verification-runner-through-the-existing-supervisor.md) | 12 | M | todo | — | — |
<!-- statusgen:briefs:end -->

The generator owns the table. All eight briefs are M-sized, strong-tier work. New tests and
paths named by their Verify tables are future deliverables, not authoring-time evidence.

## Critical path and checked head

Source baseline: main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`, 16 October 2026.
`loopadmin/runner` and its conformance kit are implemented; the existing suite passed at
authoring. The new extension and readiness receipt do not exist. `workflow/`, the shared
supervisor and first real adapter remain planned. The open-PR inventory was checked on
16 October: no listed PR targets this new stream or its new extension/receipt paths;
graph-execution/20 has an independent Evidence PR, which is left untouched. This is a
bounded inventory, not a claim that every related worker is idle.

Smallest unblocking move: /01 extends the existing v1 negotiation and conformance kit;
it needs no graph store, daemon, new model or cluster. Existing /20's independent
verification/release obligations remain prerequisites to adopting its output.

```text
graph-execution/20 -> 01 -> 02 ---------> 06 AX offline feasibility (independent branch)
                         +------------> 07 Harness² refinement protocol (independent branch)
                         |
graph-execution/26 -> 22 + 02 -> 03 local readiness/capability enforcement
graph-execution/21 + 25 + 03 -> 04 integrated recovery -> 05 truthful lifecycle
graph-execution/27 -------------------------------> 05
graph-execution/21 + 25 + 03 -> 08 Kubernetes profile (parked, separately gated)
```

The inherited integration head still includes graph-execution/02 independent verification
before /28–29 and R2/R3 consumers. Follow graph-execution's current routing rather than
assuming its historic SQLite or file-claim paths apply. /04 deliberately waits for the
real controller/publication implementations: a hand-built fake workflow is not its proof.
Its fake provider is injected behind those production owners. /06 cannot shorten this path.

## Dependency waves

Global waves include the existing graph-execution prerequisites:

| Wave | New work | Existing prerequisite |
|---|---|---|
| 1 | 01 qualified execution profile contract | graph-execution/20, wave 0 |
| 2 | 02 prepared-environment receipt and workspace port | 01 |
| 3 | 06 AX compatibility and 07 Harness² refinement investigations | 01–02; independent of core integration |
| 8 | 03 local supervisor/adapter readiness enforcement | 02 and graph-execution/22 (7), /26 (6) |
| 12 | 04 recovery and external-effect qualification; 08 Kubernetes executor profile | 03 and graph-execution/21 (7), /25 (11) |
| 13 | 05 lifecycle projection and operator status | 03–04 and graph-execution/27 (8) |

## Existing owners and completion boundary

| Existing work | Disposition |
|---|---|
| graph-execution/20 runner v1 and existing Evidence | Consume unchanged; /01 is a new extension obligation |
| graph-execution/22, /26–27 local adapter, supervisor, standing callers | Original deliverables first; /03 and /05 extend them without a second launcher |
| graph-execution/04, /16, /19, /21, /25 | Retain PostgreSQL, ownership, recovery, controller and publication authority; /04 qualifies their assembled behavior |
| graph-execution/23, /28 | Retain immutable candidate and acceptance records; environment receipt grants neither |
| R7 Cell API and celld S1–S6 | Reuse ingress/lifecycle and preserve separately scoped experiments; /05 supplies their shared snapshot |
| Existing Git maintenance and retirement | Reuse through the workspace port; no new pruner or TTL deletion rule |

Core completion is independently verified /01–05 with compatibility and negative cases.
/06 completes with an evidence-backed go/no-go/unknown report, not a running AX cluster.
Implementation is not release, installation, adoption or activation. Live profile approval
and whole-system R1–R8 qualification retain their existing owners and decision records.

## Authoring validation boundary

On the pinned main source, `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance`
returned exit 0 and `ok github.com/medici-finance/assay/loopadmin/runner/conformance`
on 16 October 2026. This is the existing production-client/fake-adapter suite, not the new
briefs' tests. Open-PR file enumeration returned 21 open PRs with no pagination gaps and no
new extension/receipt-path owner. The architecture-review proposal also edits the target
chapter; this change adds a separate linked section without adopting its proposal. The
pattern-schema authoring and runner Evidence PRs retain their existing scope.

## Harness² follow-through

The independently runnable /07 investigation evaluates propose–probe–compose as a source of
reviewed profile proposals. It is not an AX adapter or a production optimization loop.
It qualifies immutable bundle identity, independent holdouts, grading-data isolation and
whole-experiment accounting through deterministic fixtures. Existing improvement-proposal,
review and profile-adoption owners retain promotion; no installed guardrail or live attempt
can be modified by a generated composition. Core /01–05 remain independent of /06–07.

## Kubernetes verifier follow-on — 19 October 2026

[Brief 08](brief-08-qualify-a-kubernetes-verification-runner-through-the-existing-supervisor.md)
adds one executor profile through the existing supervisor after /03 and graph-execution/21,
/25 (wave 12). It does not block the interim local evidence pilot. Offline adapter proof
and later disposable-cluster qualification are distinct. The stream remains parked; neither
the new brief nor the user's implementation-planning request activates a cluster profile.
