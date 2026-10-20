---
brief: assay:assay:performance-collection:01
title: "Local performance event contract, spool and offline report"
why: "Operators need useful local measurements before a collector or database exists. A versioned local record and reader establish the consent, identity and coverage seam that later storage and transport can consume without changing old telemetry promises."
wave: 0
depends: []
unblocks: ["performance-collection/02", "performance-collection/03", "performance-collection/05"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Correlated operational measurements and their access/retention boundary require review of the implemented field policy and isolation; tests do not authorize live collection."
decision-trigger: spec
issues: []
schema: brief-v2
version: 5
id: 1f12e2ac-a0b9-4881-a452-4be522f432ad
authored: 2026-10-13 by performance collection authoring
sources:
  - "assay:assay:desktools-v2:63 — perf-v3 contract fixture and importer compatibility"
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
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
  - "perfrec/: out-of-scope (desktools-v2/61 owns the existing local recorder; this brief defines the separate correlated collection contract)"
  - "perfdata/go.mod: follow-up performance-collection/01"
  - "perfdata/event.go: follow-up performance-collection/01"
  - "perfdata/event_test.go: follow-up performance-collection/01"
  - "perfdata/consent.go: follow-up performance-collection/01"
  - "perfdata/local/: follow-up performance-collection/01"
  - "perfdata/report/: follow-up performance-collection/01"
  - "perfdata/cmd/perfdata/: follow-up performance-collection/01"
  - "perfdata/testdata/: follow-up performance-collection/01"
  - "spec/performance-event-v1.md: follow-up performance-collection/01"
  - "schemas/performance-event-v1.json: follow-up performance-collection/01"
  - "schemas/performance-batch-v1.json: follow-up performance-collection/01"
  - "docs/telemetry.md: follow-up performance-collection/01"
  - "docs/performance-collection.md: follow-up performance-collection/01"
  - "docs/contracts.md: follow-up performance-collection/01"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md: follow-up performance-collection/01"
  - "docs/proposals/non-ci-architecture/14-storage-forge-graph.md: follow-up performance-collection/01"
  - "docs/proposals/non-ci-architecture/16-delivery-sequence.md: follow-up performance-collection/01"
  - "docs/streams/continuing-operations/proposed-briefs.md: follow-up performance-collection/01"
  - "statusgen/telemetry.go: out-of-scope (legacy counts-only endpoint and schema remain unchanged)"
  - "loopadmin/runner/client.go: follow-up performance-collection/02"
  - "perfdata/postgres/: follow-up performance-collection/03"
  - "perfdata/ingest/: follow-up performance-collection/04"
  - "perfdata/upload/: follow-up performance-collection/05"
  - "perfdata/phasecost_test.go: follow-up performance-collection/01"
---

# Brief 01 — Local performance event contract, spool and offline report

## Context

files: `perfdata/go.mod` (planned), `perfdata/event.go` (planned), `perfdata/event_test.go` (planned), `perfdata/consent.go` (planned), `perfdata/local/` (planned), `perfdata/report/` (planned), `perfdata/cmd/perfdata/` (planned), `perfdata/testdata/` (planned), `spec/performance-event-v1.md` (planned), `schemas/performance-event-v1.json` (planned), `schemas/performance-batch-v1.json` (planned), `docs/telemetry.md` (existing), `docs/performance-collection.md` (planned), `docs/contracts.md` (existing), `docs/proposals/non-ci-architecture/11-target-architecture.md` (existing), `docs/proposals/non-ci-architecture/14-storage-forge-graph.md` (existing), `docs/proposals/non-ci-architecture/16-delivery-sequence.md` (existing), `docs/streams/continuing-operations/proposed-briefs.md` (existing), `changelog/performance-collection-01.md` (planned), `perfdata/phasecost_test.go` (planned)

facts (source pin in frontmatter; planned paths/tests are deliverables, not implemented claims):
- The fresh main has no perfdata module; loopadmin/runner/{request,result,usage}.go already defines invocation, work and nullable usage types. No online supervisor is needed to validate and store bounded observations.
- statusgen/telemetry.go emits telemetry-v1 counts only behind two switches and an empty endpoint. The inspected main now includes perf-v1 hooks and deskperf (desk-tools/23 remains implemented, not independently verified); both old schemas and switches stay unchanged.
- The new stream README is the bounded design source. Continuing-operations O02 owns measurement meanings, and graph-execution/06 owns run records; performance events carry selected observations and references only.

layering: perfdata contract and local storage; existing domain owners retain the meanings of work, usage and acceptance. A pure typed contract owns validation/aggregation; filesystem, HTTP and database adapters own effects. Task boundaries and the flow/negative Verify cases below enforce this separation. No database or network dependency enters the local writer.

single-point-of-failure: the collection consent/identity boundary; independent typed payload validation and source-scoped storage checks constrain disclosure, while missing optional data cannot grant authority or reset required accounting. Verify bypasses upper adapters and exercises lower boundaries.

design-fit:
  owner: perfdata contract and local storage; existing domain owners retain the meanings of work, usage and acceptance
  contract: none — register the bounded collection owner under docs/contracts.md in 01; retain existing domain/measurement owners
  retires: []
  weight: verbs +1 (perfdata with record/report); flags +5 (input, dir, profile, policy, json); refusals +4 bounded validation classes; rule-text +0
  why-add: Bounded optional collection has no complete-path owner today. Reusing the load-bearing audit/journal as a prunable transport was considered and rejected; use existing producer seams and one shared contract instead. These are scope estimates; measure the implemented delta.

## Read first

Read [the stream design](README.md), [telemetry promise](../../telemetry.md), and
[library-first boundaries](../../library-first.md). All load-bearing scope and acceptance
requirements for this brief are restated below; sibling briefs describe providers, not
additional hidden acceptance criteria. Refresh active claims and source paths before edits.

## Human decision

Confirm the exact correlated field allowlist, local loss envelope and measured writer overhead from the implementation evidence. Local-only collection and the separate optional database direction are already requested; this sign-off does not grant remote transmission.

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

1. Create one stdlib-only root contract package and local/report packages. Freeze performance-event-v1 and batch/receipt schemas with strict field and enum validation, bounded typed payloads, stable source/epoch/sequence/event identity, occurred/recorded times, instrument and collection-policy versions, nullable measurements and explicit measured/proxy/estimated basis. Define usage delta versus cumulative snapshots, corrections and unsupported-version behavior before downstream implementation.

2. Implement explicit tool/workflow profiles under ASSAY_TELEMETRY=1, with the additional profile selection required by the README. Off creates nothing. The workflow profile uses random persisted aliases for qualified domain IDs, scoped by source epoch; private alias mappings stay local and are excluded from preview/upload. No argv, paths, titles, bodies, raw errors, credentials or transcripts in event payloads. Unknown enum input becomes an explicit unavailable observation or a refused record, never copied text.

3. Implement bounded, segmented append-only local storage under a caller-resolved state directory in performance-events/, distinct from audit.jsonl, dispatch-records.jsonl and perf/. Store schema manifests, source epoch, alias map and coverage separately with restrictive permissions and atomic metadata replacement. Readers tolerate an incomplete tail and distinguish corruption, missing files and a complete empty window. Multiple producer processes cannot interleave records or reuse identities; crash/restart cannot silently reset a source sequence.

4. Keep recording best-effort through a bounded sink contract: full queue, disk-full and slow writer do not change the caller result or wait indefinitely. Document the flush budget and crash-loss envelope, record drops/gaps where possible, and expose uncertain coverage after unclean termination. Seven UTC days is the initial age cap; require an explicit byte cap in the selected policy. Retire unsent expired data with explicit coverage loss; no upload or socket in this brief.

5. Deliver perfdata record and report as thin local commands over the libraries, with exact payload preview and JSON output. Report per-source coverage, unfinished attempts, measured/proxy durations, nullable usage and distribution counts. Do not add nested durations or cumulative usage snapshots into false totals. A local synthetic producer-to-reader example must run with no cell, DB, credentials or API installed.

6. Publish the API input and acknowledgment contract as fixtures now so database, server and sender can develop independently. Include partial results, committed ACK semantics, identical duplicate and conflicting-identity responses, bounded replay horizon, consent revision and authenticated source binding requirements; no server is implemented here.

7. Update the architecture storage table and delivery mapping to the separate optional database; add a semantic-owner entry for collection only. Reconcile O02/O04 transport consumption without claiming their metric-definition program is implemented. Publish consent/retention limitations and local build/use commands; state plainly in docs/telemetry.md that ASSAY_TELEMETRY=1 together with an explicit profile now also gates a pseudonymous local record class, and scope the existing anonymized, counts-only promise to telemetry-v1 only; the local executable must build independently before any PostgreSQL/HTTP adapter exists. Leave remote activation and existing telemetry endpoints untouched.

8. Freeze the three-report input snapshot and per-source coverage manifest described in the README. Preserve legacy raw retention while supporting explicitly configured 90-day distribution retention; count/sum plus retained samples or versioned mergeable histograms must support pooled Avg/P95. Report collection-on/off cost, drops and retention loss. No composed three-panel rendering or network read is required by this collection brief; the low-level offline reader in Task 5 remains in scope. Add TestReportDistributionRetention proving two unequal daily sample sets pool correctly and missing days stay unknown.


### Approved workflow-efficiency amendment — 17 October 2026

Extend the existing bounded event schema and local reducer with selected phases (boot/auth/audit,
collect/page/enrich/classify, plan, workspace/source/packet, claim/recheck, child execution),
parent-child correlation and explicit own/descendant/inclusive CPU coverage. Distinguish actual HTTP
attempts by bounded endpoint class, retries/304s and actual/estimated/unknown GraphQL points. Do not
serialize raw URLs/argv/identities or expand profiles implicitly. Aliased correlation follows the
existing selected profile; tool-only profiles cannot smuggle workflow identity. Prevent parent-
summary plus child-event double counting and never sum overlapping wall intervals as elapsed time.
Preserve perf-v1/telemetry-v1 and existing privacy, seven-day policy, optional-sink isolation and
schema/version rules. This remains one event/spool/report owner, not a parallel profiler. No DB/API
is needed for local tests and the stream remains parked. Bound implementation to this schema/reducer
contract; defer extra producer integrations to /02.

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

## Verify

All named tests below are **implementation deliverables**, not tests already executed.
Each command requires the named PASS line, so a missing test cannot pass through Go's
"no tests to run" behavior. Each test also includes the planted failing control stated
below, exercising production code rather than an independently reimplemented checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestEventPrivacy$' ./... > "$o" && grep -F -- '--- PASS: TestEventPrivacy ' "$o")` | exit 0; named PASS. Strict schema and profile allowlist reject planted identifiers/free text; workflow mapping is excluded from emitted bytes and old payload fixtures stay unchanged. |
| 2 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestSpoolRecovery$' ./local/... > "$o" && grep -F -- '--- PASS: TestSpoolRecovery ' "$o")` | exit 0; named PASS. Concurrent append, partial tail, process crash, full disk and expired-unsent fixtures expose gaps without changing the wrapped result; source IDs survive restart. |
| 3 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLocalReportFlow$' ./... > "$o" && grep -F -- '--- PASS: TestLocalReportFlow ' "$o")` | exit 0; named PASS. Real local record command to report command works without DB/server; zero transport calls; repeated cumulative usage and nested time are not double-counted; missing is not zero. |
| 4 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLocalCostAndBounds$' ./local/... > "$o" && grep -F -- '--- PASS: TestLocalCostAndBounds ' "$o")` | exit 0; named PASS. Bounded queue and flush under slow I/O; matched small-event append median below inherited 1 ms target on declared reference fixture, with p95/max and on/off overhead reported. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/01` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |
| 6 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v -run '^TestReportDistributionRetention$' ./... > "$o" && grep -F -- '--- PASS: TestReportDistributionRetention ' "$o")` | exit 0; named test runs and passes, including the negative cases in Task 8. |
| 7 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPhaseCostSchema$' ./... > "$o" && grep -F -- '--- PASS: TestPhaseCostSchema ' "$o")` | exit 0; named PASS. A nested/overlapping fixture reconstructs CPU coverage, endpoint attempts and unknown quota without double counting; planted free text, profile escalation or unknown-as-zero is refused. |

## Pre-mortem and detection

Bypass the profile/field filter, make the sink propagate disk-full into the caller result, and sum repeated cumulative snapshots; the corresponding privacy, recovery and flow cases must fail. Missing coverage is not a successful zero-data test. The end-to-end
row must traverse actual public producer/reader or served interfaces. Scope/retention
adequacy and deployment workload selection are review judgments; their measured evidence
is required before activation, not invented from component-test success.

## Performance reuse amendment — 2026-10-16

Fresh main already carries perfrec, shared by desk-tools and statusgen/perfrecord.go. Keep
that local-only recorder and its existing consent/history as an independent instrument;
desktools-v2/61 owns its schema/boundary evolution, not this correlated-spool contract.
Define timing basis, inclusive/exclusive semantics, availability and instrument/schema
version explicitly on imported observations. A legacy perf-v1 total and a new entrypoint
measurement are not interchangeable samples. Nested phase, child and parent durations
cannot be added into causal elapsed-time savings. The contract accepts bounded typed
observations without requiring a database, network collector or runtime service.

Preserve the historical sources and the merged reporting amendment as dated history;
the current implementation state is the shared perfrec module at eba941721. This amendment
does not replace its writer or retrospectively widen old payload/consent promises.


## Perf-v3 delivery amendment — 19 October 2026

Apply the [perf-v3 contract](../../performance-attribution-spec.md). Existing Tasks/Verify/Evidence and consent/retention controls remain owed.

Freeze the collection representation of the perf-v3 contract, not a second local instrument.
Local workflow v3 may contain typed qualified work locators; the existing source/epoch alias
mapper must translate them before this spool, preview or upload. Keep the explicit selected
workflow profile and never forward native harness IDs/raw qualified locators. Preserve agent,
session, fresh invocation/parent identities with scoped aliases, desk versus assigned role,
request-duration distributions and measured/estimated/unknown GraphQL units/sample coverage.
Child observations are own-process; deduplicate identity and never sum inclusive timing.
Extend TestPhaseCostSchema and TestEventPrivacy to reject planted raw locators, inherited
identity in tool profile, invalid context, false-zero points and parent-summary duplication.
No dependency on v3 implementation is required to freeze synthetic collection fixtures.

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
