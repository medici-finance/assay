---
brief: assay:assay:performance-collection:02
title: "Producer adapters for workflow, usage and local performance"
why: "A logger with only synthetic input cannot explain desk delays or rework. Bounded adapters should turn existing performance, dispatch, historian and runner observations into locally useful measurements without waiting for the full cell runtime."
wave: 4
depends: ["performance-collection/01", "desktools-v2/63", "desktools-v2/64", "desktools-v2/65"]
unblocks: ["performance-collection/06", "performance-collection/07"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Correlated operational measurements and their access/retention boundary require review of the implemented field policy and isolation; tests do not authorize live collection."
decision-trigger: spec
issues: []
schema: brief-v2
version: 6
id: 20aa552e-a2cf-4e0d-959c-0b47c9091d0a
authored: 2026-10-13 by performance collection authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "assay:assay:desktools-v2:62 — consumer routing for the 2026-10-16 performance amendment"
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
  - "medici-finance/assay#3353 — cell startup scratch scan; measurements on v1.0.48"
  - "medici-finance/assay#3099 — remaining parent launch telemetry coverage"
  - "freshness-checked 2026-10-20 @ eb82d5b62 (merged main, installed v1.0.51); the 2026-10-18 v1.0.49 install recorded source 3cb57291e"
  - docs/streams/performance-collection/README.md
  - docs/telemetry.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/continuing-operations/spec.md
  - freshness-checked 2026-10-13 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
  - "docs/desk-workflow-efficiency-spec.md"
  - "freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d; approved spec and consolidation amendment"
exec-tier: strong
exec-tier-why: Cross-component privacy, identity, concurrency and loss semantics require design judgment and negative controls.
domain: complicated
outcome: none
consumers:
  - "perfrec/testdata/: follow-up performance-collection/02 (read all three instrument versions; no second writer)"
  - "statusgen/perfrecord.go: out-of-scope (statusgen/26 owns producer integration; this brief imports selected records)"
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
  - "perfdata/adapters/phasecost_test.go: follow-up performance-collection/02"
---

# Brief 02 — Producer adapters for workflow, usage and local performance

## Context

files: `perfdata/adapters/` (planned), `perfdata/cmd/perfdata/` (planned), `perfdata/testdata/sources/` (planned), `loopadmin/runner/client.go` (existing), `loopadmin/runner/telemetry.go` (planned), `loopadmin/runner/telemetry_test.go` (planned), `loopadmin/go.mod` (existing), `loopadmin/go.sum` (planned), `tools/desk/internal/deskkit/dispatchrecord.go` (existing), `tools/desk/internal/deskkit/perfexport.go` (planned), `tools/desk/internal/deskkit/perfexport_test.go` (planned), `tools/desk/go.mod` (existing), `tools/desk/go.sum` (existing), `docs/performance-collection.md` (planned), `docs/contracts.md` (existing), `docs/streams/graph-execution/brief-26-shared-loop-admin.md` (existing), `changelog/performance-collection-02.md` (planned), `perfdata/adapters/phasecost_test.go` (planned); additional startup scope: `perfdata/adapters/cellstart_test.go` (planned).

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

1. Add version-selected read-only adapters for perf-v1/perf-v2/perf-v3 files, dispatch-record-v1, historian records and an explicit manually supplied observation file. Source readers take exact configured paths and scopes, never search home directories/transcripts. Re-import keeps stable source-event identity and an explicit cutoff; truncation, rotation, duplicate rows, schema changes and source loss retain coverage. Preserve source timestamps and their known resolution.

2. At the existing dispatch-record seam, emit an optional bounded observation with the same canonical attempt identity through the alias mapper. Keep the canonical dispatch/audit records and their current semantics unchanged. Domain success never depends on optional emission; old tools are supported by file import. Consume the perf-v1 hooks and files present on the inspected main; do not modify its seven-day privacy contract. <!-- graph: not-a-gate — desk-tools/23 is an optional input adapter, not an implementation prerequisite; fixture import works without its merge. -->

3. Add an optional observer to the existing runner Client with an unchanged default constructor path. Emit launch request/observed running/terminal/unknown/cancel and usage snapshots from actual protocol observations; distinguish requested start from confirmed execution. Preserve current authority, model, budget and cancellation rules. The sink never calls back into admission or blocks under the client lock; dropped notifications are explicit coverage. Do not implement a second launcher, sampler, process journal or supervisor.

4. Attribute events to aliased work/candidate/attempt and source epoch where available. Standing sessions with several subjects remain shared/unattributed unless a source supplies a valid split. Keep cumulative usage as snapshots and deduplicate polls; unknown token or cost fields remain unknown. Model/profile attribution is normalized and bounded, never copied from unvalidated arbitrary strings.

5. Provide a strict offline import for declared human effort, review/evidence applicability, CI and outcome observations: record method, source reference, occurrence/observation times, candidate/contract, units and coverage. Human waiting duration is not hands-on time; sampled/inferred events are labeled. This is not a live forge/check-run poller or automatic active-time measurement.

6. Document a producer coverage matrix: which stage/review/CI/resource facts are measured, proxies, manually supplied or unavailable. Emit gap summaries alongside local reports. Add a dated compatibility amendment to graph-execution/26 so the future supervisor carries the optional sink over its own canonical process facts; preserve its existing Task/Evidence and do not require that journal for these adapters.

7. Validate module packaging and the production-to-local-report flow with actual runner and dispatch seams, preserving all source authority tests. Build source adapters without DB/server packages. Provide local rollout and revert instructions without enabling collection on the authoring machine.

8. Supply saved inputs for all three reports: qualified completion/stage transitions and current stream snapshots, explicit hold-reason/owner/source references, and version-qualified perf-v1/perf-v2/perf-v3 invocation/cpu/child/forge samples. Preserve first-sighting opening balances, duplicate/reopen identity and known observation precision. Import does not guess stall causes, descendant CPU, bypassed HTTP calls or tokens. Export a coverage manifest for sources/versions/epochs and explicit unknown historical intervals. Add TestOperatingReportSourceCoverage for duplicated imports, opening balances, unknown hold reasons and unsupported counters; no forge polling is added here.


### Approved workflow-efficiency amendment — 17 October 2026

Use the /01 selected phase/coverage fields at existing runner, dispatch-record and deskkit optional
observer seams. Carry source/tool/adapter versions, bounded command/kit/caller/phase labels and
parent-child correlation only through the permitted alias profile. Capture actual
attempts/retries/304 and provider-reported points where instrumented; subprocess bypasses and
inclusive CPU limitations remain explicit unknown coverage. Deduplicate repeated usage snapshots and
parent summaries. Existing perf-v1 import cannot invent missing phases/points; correct legacy
attribution belongs to desk-tools/38, whose absence must not prevent fixture imports. Keep recording
outside audit/budget/admission locks with bounded loss and sink failure behavior. No new launcher,
spool, sampler or live collector. Additional integrations beyond these seams require focused follow-
ons rather than expanding this L.

This is an amendment to unfinished implementation acceptance. Preserve every prior Task, Verify
obligation, risk decision and Evidence entry. All tests named here are planned deliverables; this
authoring change claims no implementation or operational adoption.

## Interface contract

Inputs and outputs use performance-event-v1 and performance-batch-v1 from 01, with
source/profile/policy revision, immutable identity, provenance and coverage. No extension
may reinterpret old fields silently. The local producer owns event identity; authenticated
collector policy owns admission scope; the store owns committed receipts. Performance
observations confer no work, acceptance or effect authority. Unsupported mandatory schema
or unavailable measurements must remain visible through every downstream reader.

### Cell startup source coverage — 2026-10-18

Extend the existing phase importer/observer fixtures to cover /26's optional launch observations:
cell preparation, lock wait, scratch admission, final cache enrollment and confirmed child start,
with parent cell invocation and role-attempt correlation only through the permitted alias profile.
Treat `up` request, printed launch line, process-start receipt and later deskboot as different facts.
Reuse the /01 phase contract; source version/coverage identifies absent fields. Importing old rows
cannot manufacture a launch timestamp. Shared preparation is a parent cost, not one cost per desk;
concurrent role intervals overlap. Refused/failed/unknown starts stay in cohort accounting.

The producer repair (effective parent opt-in and complete phases) belongs to graph-execution/26
and source issue #3099. This L brief consumes its saved records at the existing seams only: no
cellctl policy edit, new collector, journal or reverse dependency. Fixtures work without that
repair, and label missing coverage explicitly. Record installed version separately from measured
version: #3353's 173.16-second sample is v1.0.48; later installed v1.0.49 is not a new sample.
The existing privacy, consent, loss and no-blocking-under-lock rules apply unchanged.

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
| 6 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v -run '^TestOperatingReportSourceCoverage$' ./... > "$o" && grep -F -- '--- PASS: TestOperatingReportSourceCoverage ' "$o")` | exit 0; named test runs and passes, including the negative cases in Task 8. |
| 7 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPhaseProducerFlow$' ./... > "$o" && grep -F -- '--- PASS: TestPhaseProducerFlow ' "$o")` | exit 0; named PASS. Real adapter fixtures reconstruct nested work without double count; old perf-v1 yields unknown fields. Consent-off, drop/slow sink and missing children preserve domain results and expose coverage. |
| 8 | check:ci +flow +mutation | `(cd perfdata && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceInstrumentVersionImport$' ./adapters/... > "$out" && grep -F -- '--- PASS: TestPerformanceInstrumentVersionImport ' "$out")` | exit 0; named PASS; legacy and successor desk/statusgen fixtures preserve boundary, nullable coverage and stable import identity; mixed-version naive totals and absent-to-zero plants fail |
| 9 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellStartSourceCoverage$' ./... > "$o" && grep -F -- '--- PASS: TestCellStartSourceCoverage ' "$o")` | exit 0; named PASS. Versioned up/role/start fixtures preserve one shared cost, overlapping intervals, failure/unknown coverage and distinct measured/installed versions. Mutation: use launch-request time as confirmed start or fill missing phases with zero; test fails. |

## Pre-mortem and detection

Drop the candidate binding, add cumulative samples, or allow sink errors to replace runner results; source-flow, attribution and authority cases must fail. Missing coverage is not a successful zero-data test. The end-to-end
row must traverse actual public producer/reader or served interfaces. Scope/retention
adequacy and deployment workload selection are review judgments; their measured evidence
is required before activation, not invented from component-test success.

## Performance reuse amendment — 2026-10-16

Import both the existing perf-v1 and the deliberately versioned successor from
desktools-v2/61 through read-only adapters. Preserve their instrument versions, timing
boundaries, measurement availability and phase/child inclusion. Keep samples separated or
explicitly normalize only where supported; never infer zero from an old absent counter.
Actual board/dispatch/token records from /62 and statusgen/26 are consumer fixtures; they
do not require an active service or add producer hooks here. Retain legacy import behavior
when newer producers are unavailable. Do not add raw command text, repository identifiers,
credential metadata or local paths to exported payloads. Follow the existing profile,
pseudonymization, retention and remote-consent contract.

The dependency on /61 is for the concrete versioned adapter fixtures; the hotpath local
logging and credential work do not wait on this importer, perfdata or its database.


## Perf-v3 delivery amendment — 19 October 2026

Apply the [perf-v3 contract](../../performance-attribution-spec.md). Existing Tasks/Verify/Evidence and consent/retention controls remain owed.

Read perf-v1, perf-v2 and perf-v3 through version-selected adapters. /63–65 supply v3
schema/propagation/request fixtures; old rows remain unattributed with historical timing and
counter gaps, never backfilled by branch/argv inference. Actual producers remain their own
owners. Explicit workflow import aliases every qualified local work and correlation reference
under this source epoch before persistence/preview/export; tool-only import excludes them.
Preserve deskRole versus assigned role, points basis/unit/sample coverage and request timing.
Extend TestOperatingReportSourceCoverage and TestPhaseProducerFlow (the existing phase
adapter obligation) with two same-number issues in different scopes, two agent roles, child
causal links, re-import duplicates, mixed versions, missing cost and an uninstrumented child.
Require the existing exact PASS lines; all earlier Verify rows remain obligations.

## Query/cache measurement amendment — 19 October 2026

Preserve the query/cache boundary contract from docs/performance-attribution-spec.md:
client logical service calls and transport attempts, handler timing/cache/coalescing outcomes,
and actual direct/passthrough/refresh/poll/repair provider sends/cost remain different kinds.
Import correlation only under the selected aliased workflow policy; never forward raw local
locators or infer missing server coverage as zero. Shared refresh/background cost remains
shared/unallocated and one provider observation cannot be copied to all waiting work items.
Do not sum client wait, handler/provider spans or concurrent process CPU as elapsed savings.
Extend the existing local/served/replay/report Verify flow for this owner with the shared
50-query/three-page fixture, warm hit, passthrough retry, background poll and absent server
data. Wrongly combining the two sides of one service call or multiplying upstream points
must fail the independent expected result; existing selectors and all Evidence remain intact.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. -->

## Review

Gate: human. Review actual disclosure/identity boundaries, loss visibility and shared-value
flow evidence. Required operational records must still function with optional collection
disabled. Human acceptance changes no live destination, credential, collection switch or
retention policy.

## Reporting amendment provenance

2026-10-15: three operating reports requested; collection precedes on-demand and daily generation. Source inspection: public main d430e8f18. Existing status and independent Evidence are unchanged.
