---
brief: assay:assay:performance-collection:04
title: "Authenticated performance ingestion API"
why: "Remote producers need an authenticated, bounded collector that acknowledges durable storage. Without a served-boundary contract, retries can inflate reports or one source can write another source's history."
wave: 2
depends: ["performance-collection/03"]
unblocks: ["performance-collection/06"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Correlated operational measurements and their access/retention boundary require review of the implemented field policy and isolation; tests do not authorize live collection."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: bedf2fbf-a7a1-4032-8a16-d8bebdd1e7bc
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
  - "perfdata/ingest/: follow-up performance-collection/04"
  - "perfdata/cmd/perf-ingest/: follow-up performance-collection/04"
  - "perfdata/ingest/testdata/: follow-up performance-collection/04"
  - "perfdata/deploy/: follow-up performance-collection/04"
  - "docs/performance-api.md: follow-up performance-collection/04"
  - "docs/performance-collection.md: follow-up performance-collection/04"
  - "tools/desk/internal/deskkit/audit.go: out-of-scope (required audit and budget authority unchanged)"
---

# Brief 04 — Authenticated performance ingestion API

## Context

files: `perfdata/ingest/` (planned), `perfdata/cmd/perf-ingest/` (planned), `perfdata/ingest/testdata/` (planned), `perfdata/deploy/` (planned), `docs/performance-api.md` (planned), `docs/performance-collection.md` (planned), `changelog/performance-collection-04.md` (planned)

facts (source pin in frontmatter; planned paths/tests are deliverables, not implemented claims):
- performance-collection/01 freezes the wire batch and per-event committed-acknowledgment contract; /03 provides transactional storage and a dedicated database.
- This is a new inbound collection service, not the cell API or a forge webhook. It must have no forge token, cell credential, launch endpoint or operational authority.
- An event ID or caller-provided cell/source field is not authentication. Enrollment and credentials belong to the operator; payload claims never select the authorized namespace.

layering: perfdata ingestion use case and HTTP adapter; no cell execution authority. A pure typed contract owns validation/aggregation; filesystem, HTTP and database adapters own effects. Task boundaries and the flow/negative Verify cases below enforce this separation. No database or network dependency enters the local writer.

single-point-of-failure: the collection consent/identity boundary; independent typed payload validation and source-scoped storage checks constrain disclosure, while missing optional data cannot grant authority or reset required accounting. Verify bypasses upper adapters and exercises lower boundaries.

design-fit:
  owner: perfdata ingestion use case and HTTP adapter; no cell execution authority
  contract: none — register the bounded collection owner under docs/contracts.md in 01; retain existing domain/measurement owners
  retires: []
  weight: verbs +1 (perf-ingest); flags +1 (config); refusals +6 auth/scope/schema/size/rate/store classes; rule-text +0
  why-add: Bounded optional collection has no complete-path owner today. Reusing the load-bearing audit/journal as a prunable transport was considered and rejected; use existing producer seams and one shared contract instead. These are scope estimates; measure the implemented delta.

## Read first

Read [the stream design](README.md), [telemetry promise](../../telemetry.md), and
[library-first boundaries](../../library-first.md). All load-bearing scope and acceptance
requirements for this brief are restated below; sibling briefs describe providers, not
additional hidden acceptance criteria. Refresh active claims and source paths before edits.

## Human decision

Confirm the collector's authenticated source boundary, resource limits and durable acknowledgment behavior against the served tests. Enabling an actual endpoint and enrolling producers remain separate operator actions.

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

1. Implement POST /v1/performance/batches and a bounded health/readiness surface over the existing validation/storage interfaces. Separate pure validation/dedup meaning from HTTP and PostgreSQL effects. Do not introduce another event schema or duplicate serializer; share fixtures with the local writer and sender.

2. Authenticate each request with an operator-provisioned source-scoped credential and server-side source/profile policy. Use TLS for non-loopback traffic, explicit endpoint configuration, credential expiry/revocation and rotation; never accept a namespace solely from a payload/header. A unauthenticated caller cannot enroll or expand permissions. No credential material, raw body or private mapping in server/error/access logs.

3. Apply byte, event-count, decompression, concurrency, timeout and per-source admission limits before costly decoding or database work. Reject unknown schema/fields and invalid policy/profile bindings with bounded fixed error codes. A valid event cannot smuggle free text through arbitrary metadata; the DB writer independently enforces source scope and validated typed records.

4. Acknowledge only committed events with per-event identity and disposition. Return repeatable accepted/duplicate/conflict/rejected outcomes; describe partial batches precisely. A disconnect after commit is recoverable by identical resend. Retryable storage unavailability/rate pressure never returns a durable ACK. Fail readiness when storage is unavailable without affecting cell APIs.

5. Provide operator-only configuration/enrollment documentation, payload preview examples and health counters for accepted, duplicate, conflicting, rejected and unavailable outcomes. Use bounded dimensions in metrics, with no work IDs as monitoring labels. Do not add a general read API, queue scheduler, hosted registration UI or deployment automation.

6. Run actual served HTTP tests with synthetic credentials, TLS and the real database. Bypass client validation to exercise server and storage boundaries independently. Pin all fixtures; no test contacts a live endpoint. Publish the versioned API and supported client/schema matrix.

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

Database rows require ASSAY_TEST_PERF_DSN pointing to an explicitly disposable local
Timescale database provisioned from the pinned profile. Missing DSN fails qualification;
tests must never skip to green or fall back to a live/default database. Isolation rows also
provision a separate synthetic cell database and actual restricted roles on that instance.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestServedIngest$' ./ingest/... > "$o" && grep -F -- '--- PASS: TestServedIngest ' "$o")` | exit 0; named PASS. Real HTTP/TLS to real disposable database persists a valid local-writer batch and returns only committed identities; lost response followed by replay yields one event. |
| 2 | check:ci +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestServedIsolation$' ./ingest/... > "$o" && grep -F -- '--- PASS: TestServedIsolation ' "$o")` | exit 0; named PASS. Forged source, missing/expired/revoked credentials, disallowed profile and unknown fields cannot insert; direct storage scope checks remain independent. |
| 3 | check:ci +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestServedBounds$' ./ingest/... > "$o" && grep -F -- '--- PASS: TestServedBounds ' "$o")` | exit 0; named PASS. Oversized/compressed/slow requests, source rate pressure and concurrent batches are bounded with stable errors; no secret or event body in captured logs. |
| 4 | check:ci +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestStoreDownNoAck$' ./ingest/... > "$o" && grep -F -- '--- PASS: TestStoreDownNoAck ' "$o")` | exit 0; named PASS. Database outage and commit failure yield retryable non-ACK outcomes; restart/redelivery preserves commit meaning and cells remain independent. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/04` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |

## Pre-mortem and detection

Trust the body source instead of authenticated scope, ACK before commit, or remove a request limit; served isolation, commit and bounds tests must fail. Missing coverage is not a successful zero-data test. The end-to-end
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
