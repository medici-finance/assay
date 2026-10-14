---
brief: assay:assay:performance-collection:02
title: "Producer adapters for workflow, usage and local performance"
why: "A logger with only synthetic input cannot explain desk delays or rework. Bounded adapters should turn existing performance, dispatch, historian and runner observations into locally useful measurements without waiting for the full cell runtime."
wave: 1
depends: ["assay:assay:performance-collection:01"]
unblocks: ["assay:assay:performance-collection:06"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Correlated operational measurements and their access/retention boundary require review of the implemented field policy and isolation; tests do not authorize live collection."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 20aa552e-a2cf-4e0d-959c-0b47c9091d0a
authored: 2026-10-13 by performance collection authoring
sources:
  - docs/streams/performance-collection/README.md
  - docs/telemetry.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/continuing-operations/spec.md
  - freshness-checked 2026-10-13 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
exec-tier: strong
exec-tier-why: Cross-component privacy, identity, concurrency and loss semantics require design judgment and negative controls.
domain: complicated
outcome: none
consumers:
  - "perfdata/adapters/: follow-up performance-collection/02"
  - "perfdata/cmd/perfdata/: follow-up performance-collection/02"
  - "perfdata/testdata/sources/: follow-up performance-collection/02"
  - "loopadmin/runner/client.go: follow-up performance-collection/02"
  - "loopadmin/runner/telemetry.go: follow-up performance-collection/02"
  - "loopadmin/runner/telemetry_test.go: follow-up performance-collection/02"
  - "loopadmin/go.mod: follow-up performance-collection/02"
  - "loopadmin/go.sum: follow-up performance-collection/02"
  - "tools/desk/internal/deskkit/dispatchrecord.go: follow-up performance-collection/02"
  - "tools/desk/internal/deskkit/perfexport.go: follow-up performance-collection/02"
  - "tools/desk/internal/deskkit/perfexport_test.go: follow-up performance-collection/02"
  - "tools/desk/go.mod: follow-up performance-collection/02"
  - "tools/desk/go.sum: follow-up performance-collection/02"
  - "docs/performance-collection.md: follow-up performance-collection/02"
  - "docs/contracts.md: follow-up performance-collection/02"
  - "docs/streams/graph-execution/brief-26-shared-loop-admin.md: follow-up performance-collection/02"
  - "tools/desk/internal/deskkit/audit.go: out-of-scope (required audit and budget authority unchanged)"
---

# Brief 02 — Producer adapters for workflow, usage and local performance

## Context

files: `perfdata/adapters/` (planned), `perfdata/cmd/perfdata/` (planned), `perfdata/testdata/sources/` (planned), `loopadmin/runner/client.go` (existing), `loopadmin/runner/telemetry.go` (planned), `loopadmin/runner/telemetry_test.go` (planned), `loopadmin/go.mod` (existing), `loopadmin/go.sum` (planned), `tools/desk/internal/deskkit/dispatchrecord.go` (existing), `tools/desk/internal/deskkit/perfexport.go` (planned), `tools/desk/internal/deskkit/perfexport_test.go` (planned), `tools/desk/go.mod` (existing), `tools/desk/go.sum` (existing), `docs/performance-collection.md` (planned), `docs/contracts.md` (existing), `docs/streams/graph-execution/brief-26-shared-loop-admin.md` (existing), `changelog/performance-collection-02.md` (planned)

facts (source pin in frontmatter; planned paths/tests are deliverables, not implemented claims):
- DispatchRecordSchema in tools/desk/internal/deskkit/dispatchrecord.go is dispatch-record-v1. dispatch_ref already distinguishes attempts; its raw value and repo/item fields are private local data, never an upload-safe ID.
- statusgen/history.go loads lifecycle transitions sampled by regeneration. They prove observed stage dwell, not actual start time, service time or a complete human-gate history.
- loopadmin/runner/client.go is an in-memory authority-aware adapter client. Start/Observe/Reconcile/Cancel expose real state and Usage; its future supervisor owns durable journaling. An optional observer must not hold the client mutex during I/O or alter fencing.
- perf-v1 inputs come from desk-tools/23 when available. Missing perf-v1 support is reported as source-unavailable; no duplicate hooks or cherry-pick of the in-flight PR.

layering: domain producers retain authority; perfdata adapters translate selected observations. A pure typed contract owns validation/aggregation; filesystem, HTTP and database adapters own effects. Task boundaries and the flow/negative Verify cases below enforce this separation. No database or network dependency enters the local writer.

single-point-of-failure: the collection consent/identity boundary; independent typed payload validation and source-scoped storage checks constrain disclosure, while missing optional data cannot grant authority or reset required accounting. Verify bypasses upper adapters and exercises lower boundaries.

design-fit:
  owner: domain producers retain authority; perfdata adapters translate selected observations
  contract: none — register the bounded collection owner under docs/contracts.md in 01; retain existing domain/measurement owners
  retires: []
  weight: verbs +0; flags +4 (import-source, input, profile, policy on the existing perfdata adapter); refusals +3 source/schema/coverage classes; rule-text +0
  why-add: Bounded optional collection has no complete-path owner today. Reusing the load-bearing audit/journal as a prunable transport was considered and rejected; use existing producer seams and one shared contract instead. These are scope estimates; measure the implemented delta.

## Read first

Read [the stream design](README.md), [telemetry promise](../../telemetry.md), and
[library-first boundaries](../../library-first.md). All load-bearing scope and acceptance
requirements for this brief are restated below; sibling briefs describe providers, not
additional hidden acceptance criteria. Refresh active claims and source paths before edits.

## Human decision

Confirm that the implemented source adapters and workflow profile disclose only the selected fields and preserve runner authority. Review the measured versus proxy coverage matrix before using these reports for capacity or human-effort claims.

Options:
1. **Accept the bounded implementation** — accept only the demonstrated field, failure and isolation contract; keep live activation separately controlled.
2. **Hold for correction** — name the unsupported claim or boundary and revise the implementation before acceptance.

Default if no answer: none — acceptance remains pending. The decision is presented after implementation evidence is concrete; it does not block authoring or offline implementation.

## Ground rules

- Isolate changes; no push to main, merge, workflow dispatch or live infrastructure contact.
- Synthetic fixtures and explicitly disposable local database only; no production credentials/data.
- Preserve operational audit, budget, idempotency and authority when optional collection is off.
- Stop at implemented; independent verification and human acceptance remain separate.
- Unsupported/missing observations are explicit; never convert them into zero or success.

## Task

1. Add version-selected read-only adapters for perf-v1 files, dispatch-record-v1, historian records and an explicit manually supplied observation file. Source readers take exact configured paths and scopes, never search home directories/transcripts. Re-import keeps stable source-event identity and an explicit cutoff; truncation, rotation, duplicate rows, schema changes and source loss retain coverage. Preserve source timestamps and their known resolution.

2. At the existing dispatch-record seam, emit an optional bounded observation with the same canonical attempt identity through the alias mapper. Keep the canonical dispatch/audit records and their current semantics unchanged. Domain success never depends on optional emission; old tools are supported by file import. When desk-tools/23 is available, consume its files; do not modify its seven-day privacy contract.

3. Add an optional observer to the existing runner Client with an unchanged default constructor path. Emit launch request/observed running/terminal/unknown/cancel and usage snapshots from actual protocol observations; distinguish requested start from confirmed execution. Preserve current authority, model, budget and cancellation rules. The sink never calls back into admission or blocks under the client lock; dropped notifications are explicit coverage. Do not implement a second launcher, sampler, process journal or supervisor.

4. Attribute events to aliased work/candidate/attempt and source epoch where available. Standing sessions with several subjects remain shared/unattributed unless a source supplies a valid split. Keep cumulative usage as snapshots and deduplicate polls; unknown token or cost fields remain unknown. Model/profile attribution is normalized and bounded, never copied from unvalidated arbitrary strings.

5. Provide a strict offline import for declared human effort, review/evidence applicability, CI and outcome observations: record method, source reference, occurrence/observation times, candidate/contract, units and coverage. Human waiting duration is not hands-on time; sampled/inferred events are labeled. This is not a live forge/check-run poller or automatic active-time measurement.

6. Document a producer coverage matrix: which stage/review/CI/resource facts are measured, proxies, manually supplied or unavailable. Emit gap summaries alongside local reports. Add a dated compatibility amendment to graph-execution/26 so the future supervisor carries the optional sink over its own canonical process facts; preserve its existing Task/Evidence and do not require that journal for these adapters.

7. Validate module packaging and the production-to-local-report flow with actual runner and dispatch seams, preserving all source authority tests. Build source adapters without DB/server packages. Provide local rollout and revert instructions without enabling collection on the authoring machine.

<!-- graph: desk-tools/23 is an optional input adapter, not an implementation prerequisite; fixture import works without its merge. -->

## Interface contract

Inputs and outputs use performance-event-v1 and performance-batch-v1 from 01, with
source/profile/policy revision, immutable identity, provenance and coverage. No extension
may reinterpret old fields silently. The local producer owns event identity; authenticated
collector policy owns admission scope; the store owns committed receipts. Performance
observations confer no work, acceptance or effect authority. Unsupported mandatory schema
or unavailable measurements must remain visible through every downstream reader.

## Verify

All named tests below are **implementation deliverables**, not tests already executed.
Each command requires the named PASS line, so a missing test cannot pass through Go's
"no tests to run" behavior. Each test also includes the planted failing control stated
below, exercising production code rather than an independently reimplemented checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestProducerLocalFlow$' ./... > "$o" && grep -F -- '--- PASS: TestProducerLocalFlow ' "$o")` | exit 0; named PASS. Dispatch, runner usage and imported historian data reach the same local report with DB/API absent; consent-off leaves optional storage untouched. |
| 2 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestUsageAndIdentity$' ./adapters/... > "$o" && grep -F -- '--- PASS: TestUsageAndIdentity ' "$o")` | exit 0; named PASS. Redispatch has a new attempt; repeated cumulative polls count once; shared sessions, unknown costs, clock skew and mapping-epoch changes cannot create false joins. |
| 3 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestImportsAndCoverage$' ./adapters/... > "$o" && grep -F -- '--- PASS: TestImportsAndCoverage ' "$o")` | exit 0; named PASS. Legacy perf-v1 fixture, incomplete history, manual effort, late CI and evidence invalidation import preserve basis and cutoff; missing source is visibly unavailable. |
| 4 | check:ci +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestObserverAuthority$' ./runner/... > "$o" && grep -F -- '--- PASS: TestObserverAuthority ' "$o")` | exit 0; named PASS. Runner sink failures and callbacks cannot alter fencing, cancellation, limits or acceptance; callbacks cannot block under the client lock. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/02` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |

## Pre-mortem and detection

Drop the candidate binding, add cumulative samples, or allow sink errors to replace runner results; source-flow, attribution and authority cases must fail. Missing coverage is not a successful zero-data test. The end-to-end
row must traverse actual public producer/reader or served interfaces. Scope/retention
adequacy and deployment workload selection are review judgments; their measured evidence
is required before activation, not invented from component-test success.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. -->

## Review

Gate: human. Review actual disclosure/identity boundaries, loss visibility and shared-value
flow evidence. Required operational records must still function with optional collection
disabled. Human acceptance changes no live destination, credential, collection switch or
retention policy.
