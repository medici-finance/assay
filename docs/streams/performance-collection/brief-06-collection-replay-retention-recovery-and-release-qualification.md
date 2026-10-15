---
brief: assay:assay:performance-collection:06
title: "Collection replay, retention, recovery and release qualification"
why: "Passing logger, database and HTTP tests separately does not prove a usable history. A complete-path replay and recovery exercise must demonstrate truthful totals, bounded loss and independent cell operation before optional remote collection is offered."
wave: 3
depends: ["performance-collection/02", "performance-collection/04", "performance-collection/05"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Correlated operational measurements and their access/retention boundary require review of the implemented field policy and isolation; tests do not authorize live collection."
decision-trigger: spec
issues: []
schema: brief-v2
version: 2
id: 27aaa5a2-0575-4067-8be6-40d929e2dff9
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
  - "perfdata/integration/: follow-up performance-collection/06"
  - "perfdata/testdata/journeys/: follow-up performance-collection/06"
  - "perfdata/scripts/qualify.sh: follow-up performance-collection/06"
  - "perfdata/deploy/cnpg/qualification.md: follow-up performance-collection/06"
  - "perfdata/README.md: follow-up performance-collection/06"
  - "docs/performance-collection.md: follow-up performance-collection/06"
  - "docs/performance-storage.md: follow-up performance-collection/06"
  - "docs/performance-api.md: follow-up performance-collection/06"
  - "docs/distribution.md: follow-up performance-collection/06"
  - "components/perfdata/: follow-up performance-collection/06"
  - "ci/staged-workflows/perfdata.yml: follow-up performance-collection/06"
  - "tools/desk/internal/deskkit/audit.go: out-of-scope (required audit and budget authority unchanged)"
---

# Brief 06 — Collection replay, retention, recovery and release qualification

## Context

files: `perfdata/integration/` (planned), `perfdata/testdata/journeys/` (planned), `perfdata/scripts/qualify.sh` (planned), `perfdata/deploy/cnpg/qualification.md` (planned), `perfdata/README.md` (planned), `docs/performance-collection.md` (planned), `docs/performance-storage.md` (planned), `docs/performance-api.md` (planned), `docs/distribution.md` (existing), `components/perfdata/` (planned), `ci/staged-workflows/perfdata.yml` (planned), `changelog/performance-collection-06.md` (planned)

facts (source pin in frontmatter; planned paths/tests are deliverables, not implemented claims):
- 01/02 provide local recording and adapters; 03 provides storage; 04 serves ingestion; 05 supplies explicit replayable upload. Their component test results alone do not prove the combined system.
- The existing score uses a 90-day baseline and decision-latency proxy; this brief does not replace either. It demonstrates correctly labeled inputs rather than claiming a productivity improvement.
- CNPG failover/recovery requires a separately authorized disposable environment. No live cluster queries or workflow dispatch are permitted by this brief; static manifests are not runtime evidence.

layering: perfdata integration/packaging qualification; semantic owners remain unchanged. A pure typed contract owns validation/aggregation; filesystem, HTTP and database adapters own effects. Task boundaries and the flow/negative Verify cases below enforce this separation. No database or network dependency enters the local writer.

single-point-of-failure: the collection consent/identity boundary; independent typed payload validation and source-scoped storage checks constrain disclosure, while missing optional data cannot grant authority or reset required accounting. Verify bypasses upper adapters and exercises lower boundaries.

design-fit:
  owner: perfdata integration/packaging qualification; semantic owners remain unchanged
  contract: none — register the bounded collection owner under docs/contracts.md in 01; retain existing domain/measurement owners
  retires: []
  weight: verbs +0; flags +0; refusals +0; rule-text +0
  why-add: Bounded optional collection has no complete-path owner today. Reusing the load-bearing audit/journal as a prunable transport was considered and rejected; use existing producer seams and one shared contract instead. These are scope estimates; measure the implemented delta.

## Read first

Read [the stream design](README.md), [telemetry promise](../../telemetry.md), and
[library-first boundaries](../../library-first.md). All load-bearing scope and acceptance
requirements for this brief are restated below; sibling briefs describe providers, not
additional hidden acceptance criteria. Refresh active claims and source paths before edits.

## Human decision

Confirm the complete-path evidence and supported deployment profiles before offering remote collection. A profile whose real restore, failover or upgrade exercise has not run remains unqualified; local-only use can ship independently.

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

1. Build a deterministic journey corpus with known independent expected totals: two sources, repeated attempts, cumulative usage, a review on an old candidate, a held human decision, declared hands-on minutes, cancelled and unfinished work, out-of-order/duplicate delivery, unknown costs and a changed policy version. Include first-party recorded producer fixtures from /02 and separate expected-oracle files; never derive expectations by calling the implementation being tested.

2. Run producers to local spool and local report with no DB/API installed; then explicitly upload the selected window to the actual served API and disposable Timescale database. Compare local and stored/exported counts, durations, costs, identity bindings and coverage after identical filtering. Demonstrate one explainable work journey; do not infer active service time or human effort from elapsed time.

3. Crash at writer/queue/ACK/transaction/checkpoint boundaries; restart and replay. Turn collection off, turn upload off, stop collector and remove the disposable performance DB while a separate disposable cell keeps operating. Required budget/audit records must remain unchanged. Restore the performance backup and show retained history plus explicit missing interval, never reset-to-zero completeness.

4. Qualify retention and aggregate refresh using a simulated 90-day window including late arrivals, corrections and expired replay. Preserve distributions needed for meaningful percentiles; count duplicate and missing events independently. Report collection/transport/storage costs and unknown categories with fixed fixture load, versions and policy. Activation limits remain operator-selected, not inferred from fixture success.

5. Package the standalone local command separately from the collector so a user can install and use local logging without PostgreSQL clients at runtime, credentials or a cell. Follow the existing component/distribution contract; prepare CI coverage under ci/staged-workflows for every new module/read surface without dispatching or promoting workflows. No release tag is cut by this brief.

6. Provide an exact pinned CNPG disposable qualification runsheet for failover, backup restore and extension upgrade with aggregate/retention workers. Record runtime results only if performed by an authorized operator; otherwise explicitly mark the managed-CNPG profile unqualified while preserving qualified local-Docker/local-only use. The operator selects live destination, consent profiles, retention, budgets and RPO/RTO before activation.

7. Publish a capability matrix and handoff: local-only useful after 01/02; source-only reader versus measured producers; Docker-qualified versus CNPG-not-yet-qualified; dashboard and live CI collection gaps; no assertion that metrics improved outcomes or that every requested event is covered.

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

Database rows are classed `check` (env-bound, [brief rule 45](../../brief-rules.md)), not
`check:ci`: they require ASSAY_TEST_PERF_DSN pointing to an explicitly disposable local
Timescale database provisioned from the pinned profile, which CI does not provision, so the
verifier runs them against that database. Missing DSN fails qualification; tests must never
skip to green or fall back to a live/default database. Before any write or destructive step
(crash, restore, role or privilege change, database removal) the harness mechanically refuses
a non-disposable target: it must read back a sentinel the pinned disposable profile writes at
provisioning, and the DSN host must be loopback or the local container host. A mis-set DSN
refuses instead of acting on a real instance. Isolation rows also provision a separate
synthetic cell database and actual restricted roles on that instance.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCollectionJourney$' ./integration/... > "$o" && grep -F -- '--- PASS: TestCollectionJourney ' "$o")` | exit 0; named PASS. Actual producer → local report → sender → served API → real DB/export agrees with independent oracle, with stale evidence, retry and unknown usage visible. |
| 2 | check +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCollectionFaults$' ./integration/... > "$o" && grep -F -- '--- PASS: TestCollectionFaults ' "$o")` | exit 0; named PASS. Crash matrix, consent changes and lost ACKs do not inflate totals; optional outages/deletion do not alter cell authority, budget or required audit state. |
| 3 | check +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestHistoryWindow$' ./integration/... > "$o" && grep -F -- '--- PASS: TestHistoryWindow ' "$o")` | exit 0; named PASS. Simulated 90-day retention, corrections and restore retain valid aggregates and explicit gaps; old replay never resurrects discarded data. |
| 4 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLocalPackageOnly$' ./integration/... > "$o" && grep -F -- '--- PASS: TestLocalPackageOnly ' "$o")` | exit 0; named PASS. Clean external consumer/local command runs with no database, server, cell or network dependency at runtime; unsupported versions refuse explicitly. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/06` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |
| 90 | check:ci +flow | `(cd perfdata && o=$(mktemp) && go test -count=1 -v ./integration/... -run '^TestRemoteReportDistributionParity$' > "$o" && grep -F -- '--- PASS: TestRemoteReportDistributionParity ' "$o")` | exit 0; local and remote export distributions/coverage agree after duplicate delivery and restore; gaps remain visible. |

## Pre-mortem and detection

Remove dedup, erase a loss marker, reinterpret decision wait as effort or make cell operation consult the performance DB; the end-to-end oracle/fault cases must fail. Missing coverage is not a successful zero-data test. The end-to-end
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

## Operating report qualification amendment — 15 October 2026

Include the retained distribution/coverage contract from 01/02 in remote replay/restore
fixtures: source→upload→DB→bounded export must reproduce local counts/sums/Avg/P95 and source
cutoffs without averaging percentiles or multiplying cumulative token snapshots. Preserve
partial-source and expired-window gaps. Local report 07 and daily generation 08 qualify
independently and do not wait for this remote-path brief. Implement TestRemoteReportDistributionParity under perfdata/integration before calling the remote reporting profile qualified. Existing Evidence remains unchanged.
