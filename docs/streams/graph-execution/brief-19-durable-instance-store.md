---
brief: assay:assay:graph-execution:19
title: Durable instance store and production coverage binding
why: A task must survive its parent process without losing intent, artifacts or the evidence obligations that make
  it acceptable.
wave: 6
depends: ["graph-execution/09", "graph-execution/03", "statusgen/15", "graph-execution/28", "graph-execution/29", "graph-execution/04", "graph-execution/16", "graph-execution/30"]
unblocks:
- graph-execution/23
- graph-execution/21
- "forge-neutral/37"
- "forge-neutral/38"
- "forge-neutral/41"
effort: L
gate: model
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'no'
  sensitive-data: 'no'
issues: []
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- docs/streams/graph-execution/task-workflow-program.md
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/20-amendment-manifest.md — R2 owner row, R2 amendment scope, storage-routing and claim-store conflicts
- docs/proposals/non-ci-architecture/11-target-architecture.md — lines 92, 96, 98, 100 (PostgreSQL operational store, per-cell database/role)
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md — lines 7, 13, 24, 34, 46, 50-56, 76 (one runtime implementation, transactional intent, reconstruction, authority incarnation)
- docs/proposals/non-ci-architecture/15-regression-from-day-one.md — lines 13, 34, 51, 54, 56, 72-75 (claim-test intent, real PostgreSQL layer, new obligations)
- docs/proposals/non-ci-architecture/16-delivery-sequence.md — lines 56, 69, 104 (R2 deliverables, operating obligations, no silent store fallback)
- docs/proposals/non-ci-architecture/19-progressive-adoption-and-oversight.md — line 49 (no SQLite runtime)
- "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
- freshness-checked 2026-10-13 @ 4696d598e58820806d0bcf8c9b2dd87f29115400
- docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md — 2026-10-14 consumer amendment
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'workflow/controller: follow-up graph-execution/21'
- 'statusgen/coverage.go: follow-up graph-execution/19 (this brief; flips to fixed-here when the implementation edits the path)'
- 'workflow/workspace: follow-up graph-execution/23'
- 'workflow/store PostgreSQL coordination schema (spec/coordination-store-v1.md): follow-up graph-execution/19 (this brief; flips to fixed-here when the implementation edits the path)'
- 'drainloop transactional intent/outbox handoff seam: follow-up graph-execution/04 (19 implements it in PostgreSQL)'
- 'drainloop operator incarnation and ownership-generation seam (spec/operator-incarnation-v1.md): follow-up graph-execution/16 (19 stores the activation record it checks)'
- 'loopadmin execution journal: follow-up graph-execution/26 (same engine, own schema; no Go dependency on workflow)'
- 'tools/desk/cmd/deskclaim-ref claim tests: out-of-scope (their intent is carried here as TestPGClaimHolderSemantics per graph-execution/29 inventory; the command is unchanged)'
- 'forge-neutral file/served claim store (forge-neutral/22-32): out-of-scope (supported legacy desk-tools claim path; applicability note in the forge-neutral README; retirement is the non-CI R8 decision)'
version: 5
id: ead4e747-4d61-4ec6-abaf-332873c79b21
---

# Brief 19 — Durable instance store and production coverage binding

## Context

files: `workflow/go.mod` (planned), `workflow/README.md` (planned), `docs/lifecycle.md`, `workflow/store/` (planned), `workflow/bindings/` (planned), `workflow/testdata/store/` (planned), `statusgen/coverage.go`, `statusgen/instance.go` (planned), `changelog/graph-execution-19-durable-instance-store.md` (planned). Added by the R2 amendment (2026-10-13): `spec/coordination-store-v1.md` (planned), `workflow/store/migrations/` (planned), `workflow/store/reconstruct/` (planned), `workflow/testdata/reconstruct/` (planned), `.github/workflows/assay-workflow.yml` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the store's expected-version/generation predicate on every transition write — behind it, the existing ownership authority's fence, which refuses a restored or stale writer even when the store is bypassed.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Library-first amendment — 2026-10-08

Read `docs/library-first.md`. statusgen/15 owns the canonical eligibility/coverage API;
this brief owns durable bindings and their observations. Pin that package, preserve its
unknown/held outcomes and test the real binding-to-evaluator flow. Do not create a second
`graphcontract` package or move the evaluator into workflow. Store acceptance, admission,
process lifecycle and effect authority remain distinct.

## Non-CI R2 amendment — 2026-10-13 (PostgreSQL coordination and reconstruction)

The maintainer ratified the non-CI target (medici-finance/assay#2500, revision 23f655f5d);
approval activates nothing. This section supersedes the **engine choice** in Task 1 and the
2026-10-02 routing that permitted an embedded SQLite store; every other obligation above
stands. Owner row: package R2 in the [amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md).

- **One runtime store: PostgreSQL.** The store is the cell's PostgreSQL coordination
  database (target 11:92, 14:7, 14:24): attempts, leases and claim generations, schedules,
  effect intents and receipts, pending decisions and feedback, and retirement progress. One
  database and restricted application role per cell (11:100, 14:13). There is no SQLite
  runtime and no second interchangeable online backend (14:7, 19:49); SQLite may later be an
  offline read-only projection format, which this brief does not build. A missing or
  unreachable database refuses; it never falls back to another store (16:104).
- **Contract `coordination-store-v1`.** `spec/coordination-store-v1.md` (planned) names the
  tables this brief owns, the versioned migration ledger, the activation record and the
  reconstruction inputs. Migrations are versioned and forward-only; an unsupported mandatory
  schema version refuses.
- **Transactional handoff.** Admission, claim generation and the effect intent commit in one
  PostgreSQL transaction before any executor is invoked; delivery uses an outbox with stable
  operation identities (14:34). This is the PostgreSQL implementation of the store-neutral
  handoff seam graph-execution/04 defines; a crash with the database intact resumes from the
  outbox, and stale outbox entries are never replayed blindly (13:71).
- **Incarnation binding.** The store keeps an activation record naming the operator
  incarnation it was activated under; the incarnation itself lives outside PostgreSQL per
  graph-execution/16's `spec/operator-incarnation-v1.md` (planned) (14:50). A mismatched or
  missing incarnation starts the store paused; a restored SQL generation never revives an old
  capability (14:56, 15:51).
- **Reconstruction.** After whole-database deletion, rebuild canonical identities and
  outstanding obligations from Git definitions, published evidence and forge reads supplied as
  fixtures; mark lost attempts unknown; inventory unknown resources and hold them, never delete
  them as orphans (14:46, 14:76, 15:54). Admission starts paused. Recovery time and loss
  classes are written into the contract as declared limits, not promised zero loss.
- **Carried claim-test intent.** The claim semantics graph-execution/29 routes here (15:13):
  live-holder refusal, malformed claims, transport uncertainty and acknowledgment, plus
  stale-generation release and publication (15:44).
- **Operating obligations (16:69).** Pending feedback survives every crash boundary and is
  cleared only for its own generation (15:72); retirement progress persists each completed step
  so it resumes without guessing (15:74).
- **Tests use a real disposable PostgreSQL** (15:34). The rows below require
  `ASSAY_TEST_POSTGRES_DSN` pointing at a disposable server the test may create and drop
  databases on; an unset DSN fails the row and a test must never skip itself into a PASS.
  The module's CI job provides that service. No production credentials. The planned workflow file is a test-only job with a disposable service container and no secrets, so the four risk answers stand; workflow-file changes may need a maintainer push under the repository's server-side controls.
- **Single point of failure (R2).** The transition write's expected-version/generation
  predicate inside the current incarnation. Behind it: the effect boundary's incarnation and
  generation check from graph-execution/16, which refuses a stale capability even when the
  store is bypassed (graph-execution/16's row 7).
- Risk answers re-checked: unchanged (disposable databases and fixtures only; no live
  contact). Effort rises to L, the ceiling; anything beyond the rows below is a new brief.

## R3 and later follow-ups — not in this amendment

Exclusive repository assignment (R3) and publication/effect identity (R4) amend other briefs;
nothing here pre-empts them.

## Public runtime context amendment — 2026-10-14

Persist /30's versioned context, dependency/coverage fingerprint and validity bounds beside
existing candidate/acceptance bindings; no new coordination ledger or operational backend.
Restoring an old valid-looking result cannot renew its time/access/ownership validity.
Return source cutoff and incomplete context explicitly through the binding adapter; /14
still performs current authority checks. Add `TestPGContextRestore` to the existing real
PostgreSQL suite: store a context, restore/reload it after its lease/access bound, and prove
its binding-to-evaluator flow cannot admit the action. A valid positive case must remain.
No mandatory Git journal or signature infrastructure is introduced. Existing Evidence remains.

## Operation-count accounting amendment — 2026-10-15

The store consumes the target's [brake and pacing contract](../../proposals/non-ci-architecture/11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed).
**Brake** counts (write ceilings, daily and new-issue caps, the circuit breaker, child-launch
and retry budgets, cumulative attempt budgets) keep transactional records and fail closed:
unknown brake usage after restart or reconstruction holds the governed effect until it is
reconstructed from retained evidence or an explicitly authorized reset is recorded.

**Pacing** count updates are not a mandatory part of admission's intent/outbox transaction
and need no global lock across network work or exact budget-slot reservation. The store
seeds a lost pacing estimate from the last retained count and from the window's effect
intents and receipts it already holds; that is a bounded window query, not an audit-history
replay. After whole-database loss, when neither survives, it records at most one reset per
window and limiter scope and treats a second unseeded reset as an exhausted window. Unknown
usage never becomes a fresh allowance, and a missing estimate is reported, not zero.

Required intent, effect identity, approval, incarnation and unknown-outcome records remain
transactional and fail closed as already specified. Qualify against target doc 15's
§Operation-count qualification cases; row 13 below carries that obligation and is red until
the named test exists. Existing Task/Verify rows and Evidence are preserved.

## Task

1. Create the workflow Go module and a single-controller SQLite store (engine superseded 2026-10-13: PostgreSQL per the R2 amendment above) with transactions, versioned migrations and content-addressed artifact references. Select and pin a maintained driver; document supported filesystem/durability assumptions. Persist canonical instance/node/attempt IDs, revisions, wait/stop state and artifact manifests. No second work identity or claim authority.
2. Implement expected-version/generation transitions, complete export/import and restore validation. Reject missing artifacts, unsupported mandatory schema and stale ownership; never acknowledge a transition before durability. Restore is passive until the existing ownership authority fences the previous owner.
3. Connect real instance-to-pattern-node bindings to coverage. Consume `statusgen/evaluation` from statusgen/15; that brief alone owns extraction. Update binding adapters without changing evaluator semantics. Do not copy coverage logic into workflow. No binding found for a declared instance means held, not an unpatterned success.
4. Add restart, version-conflict, missing-artifact and stale-acceptance flow fixtures, and one fixture that bypasses the store's expected-version predicate to show the ownership fence still refuses a stale writer. Wire cross-module CI paths for statusgen and workflow so consumer tests run on changes in either. Update lifecycle/storage documentation and the changelog.
5. (R2, 2026-10-13) Write `spec/coordination-store-v1.md` (planned) and implement the store on PostgreSQL with versioned migrations, one transaction for admission, claim generation and effect intent, an outbox with stable operation identities that implements graph-execution/04's handoff seam, and the activation record graph-execution/16's incarnation check reads. Add a static guard that the runtime store imports no SQLite or other embedded engine. Provide the disposable PostgreSQL service in the module's CI job. Add a test that resolves the contract's named tables and columns against the migrations in both directions.
6. (R2) Implement whole-database reconstruction over supplied Git-definition, published-evidence and forge-read fixtures, and backup rollback that starts paused under a mismatched incarnation. Mark lost attempts unknown, hold unknown resources and never replay stale outbox entries blindly.
7. (R2) Carry the claim-test intent (holder/role/lane/attempt/generation compare, malformed claims, transport uncertainty, stale-generation release/publication) and persist pending feedback and decisions per generation plus retirement progress per completed step.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestWorkflowStoreRestartBinding$" ./... > "$routing_out" && grep -q -- "--- PASS: TestWorkflowStoreRestartBinding " "$routing_out")` | exit 0; named PASS; restart preserves work and all pattern obligations; missing binding holds; mutation: reload instances on restart without their pattern-obligation binding — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestWorkflowStoreIncompleteRestoreDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestWorkflowStoreIncompleteRestoreDenied " "$routing_out")` | exit 0; named PASS; missing artifact or tampered acceptance refuses restore; mutation: skip manifest validation on restore — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestWorkflowStoreStaleTransitionDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestWorkflowStoreStaleTransitionDenied " "$routing_out")` | exit 0; named PASS; two expected-version transitions cannot both advance; mutation: drop the expected-version predicate from the transition write — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStoreBypassFenced$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStoreBypassFenced " "$routing_out")` | exit 0; named PASS; with the store's expected-version predicate bypassed, a stale owner generation's transition is still refused by the existing ownership authority's fence and no effect is recorded; mutation: accept a transition whose owner generation the ownership authority has fenced — the named test must fail |
| 5 | check:ci +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGIntentClaimOneTxn$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPGIntentClaimOneTxn " "$routing_out")` | exit 0; named PASS against a real disposable PostgreSQL; admission, claim generation and effect intent commit in one transaction before the executor is invoked, and a crash after commit resumes from the outbox exactly once; mutation: commit the effect intent in a separate transaction after invoking the executor — the named test must fail |
| 6 | check:ci +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGDeleteReconstructs$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPGDeleteReconstructs " "$routing_out")` | exit 0; named PASS; after the whole database is dropped, reconstruction from Git-definition, published-evidence and fake forge-read fixtures recovers canonical identities and obligations, marks lost attempts unknown, holds unknown resources and starts with admission paused; mutation: treat a missing attempt row as never started instead of unknown — the named test must fail |
| 7 | check:ci +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGRollbackStaleRefused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPGRollbackStaleRefused " "$routing_out")` | exit 0; named PASS; restoring an earlier backup while an old capability survives starts the store paused, the old capability cannot admit an effect even when its SQL generation reappears, and a stale outbox entry is reconciled or held, never replayed; mutation: accept the restored activation record as current without checking the operator incarnation — the named test must fail |
| 8 | check:ci +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGClaimHolderSemantics$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPGClaimHolderSemantics " "$routing_out")` | exit 0; named PASS; a live holder refuses a second claim, a malformed claim is refused, a non-holder release leaves the claim intact, a stale-generation release or publication is refused and an unknown transport outcome holds; mutation: release on holder-name match without comparing role, lane, attempt and generation — the named test must fail |
| 9 | check:ci +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGPendingFeedbackDurable$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPGPendingFeedbackDurable " "$routing_out")` | exit 0; named PASS; feedback racing attempt completion survives each crash boundary and is not cleared with the previous generation, and retirement resumes from its last recorded step; mutation: clear pending feedback by work ID instead of by generation — the named test must fail |
| 10 | check:ci +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestNoSQLiteRuntimeStore$" ./... > "$routing_out" && grep -q -- "--- PASS: TestNoSQLiteRuntimeStore " "$routing_out")` | exit 0; named PASS; no package under `workflow/store` imports a SQLite or other embedded database driver, and an unreachable database refuses rather than falling back; mutation: add an embedded-driver import under `workflow/store` — the named test must fail |
| 11 | check:ci +dereference +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoordinationSpecMatchesMigrations$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCoordinationSpecMatchesMigrations " "$routing_out")` | exit 0; named PASS; every table and column `spec/coordination-store-v1.md` (planned) names exists in `workflow/store/migrations/` (planned), and every migrated table is named in the contract; mutation: drop one column from the contract text — the named test must fail |
| 12 | check:ci +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGContextRestore$" ./store/ > "$out" && grep -F -- "--- PASS: TestPGContextRestore " "$out")` | exit 0; named PASS; real PostgreSQL reload/restore retains old cutoff and expiry; binding flow holds stale context and accepts current context; mutation: replace expiry with restore time |
| 13 | check:ci +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGOperationCountSeedAndBrake$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPGOperationCountSeedAndBrake " "$routing_out")` | exit 0; named PASS against a real disposable PostgreSQL; after the pacing counter rows are dropped mid-window, the estimate is seeded from the window's effect intents and receipts and restarts admit no more than cap + C; after whole-database loss one reset is recorded and a second treats the window as exhausted; an unknown brake count holds; mutation: seed the lost estimate at zero — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only. R2 amendment facts, declared files, edges, consumers and sizing were re-checked against main at 4696d598e on 2026-10-13; rows 5–11 were run at that head and are red because the workflow module and the named tests do not exist yet.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
