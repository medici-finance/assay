---
brief: assay:assay:performance-collection:05
title: "Explicit resumable upload from the local spool"
why: "Local logs and a collector do not move data by themselves. Explicit, resumable upload lets operators choose what leaves the machine while bounded retry and stable identities prevent silent loss or duplicate accounting."
wave: 1
depends: ["performance-collection/01"]
unblocks: ["performance-collection/06"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Correlated operational measurements and their access/retention boundary require review of the implemented field policy and isolation; tests do not authorize live collection."
decision-trigger: spec
issues: []
schema: brief-v2
version: 2
id: 6d9c9936-0866-403d-945f-1a06329cf352
authored: 2026-10-13 by performance collection authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
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
  - "perfdata/upload/: follow-up performance-collection/05"
  - "perfdata/cmd/perfdata/: follow-up performance-collection/05"
  - "perfdata/testdata/upload/: follow-up performance-collection/05"
  - "docs/performance-api.md: follow-up performance-collection/05"
  - "docs/performance-collection.md: follow-up performance-collection/05"
  - "tools/desk/internal/deskkit/audit.go: out-of-scope (required audit and budget authority unchanged)"
---

# Brief 05 — Explicit resumable upload from the local spool

## Context

files: `perfdata/upload/` (planned), `perfdata/cmd/perfdata/` (planned), `perfdata/testdata/upload/` (planned), `docs/performance-api.md` (planned), `docs/performance-collection.md` (planned), `changelog/performance-collection-05.md` (planned)

facts (source pin in frontmatter; planned paths/tests are deliverables, not implemented claims):
- The batch contract is fixed by /01; use a fake RoundTripper/fixture server during implementation, so this brief need not wait for /03 or /04.
- Local collection opt-in is not transmission opt-in. The existing telemetry promise requires ASSAY_TELEMETRY=1 plus a command-line send action and exact payload preview.
- Source files may rotate, expire or have a partial final line; successful send and committed ACK are different facts. Local age/byte caps remain authoritative even when a destination is offline.

layering: perfdata upload adapter; local record format and retention stay with /01. A pure typed contract owns validation/aggregation; filesystem, HTTP and database adapters own effects. Task boundaries and the flow/negative Verify cases below enforce this separation. No database or network dependency enters the local writer.

single-point-of-failure: the collection consent/identity boundary; independent typed payload validation and source-scoped storage checks constrain disclosure, while missing optional data cannot grant authority or reset required accounting. Verify bypasses upper adapters and exercises lower boundaries.

design-fit:
  owner: perfdata upload adapter; local record format and retention stay with /01
  contract: none — register the bounded collection owner under docs/contracts.md in 01; retain existing domain/measurement owners
  retires: []
  weight: verbs +0 (send subcommand on perfdata); flags +6 (upload, dry-run, destination, since, until, max-batches); refusals +4 consent/destination/profile/checkpoint classes; rule-text +0
  why-add: Bounded optional collection has no complete-path owner today. Reusing the load-bearing audit/journal as a prunable transport was considered and rejected; use existing producer seams and one shared contract instead. These are scope estimates; measure the implemented delta.

## Read first

Read [the stream design](README.md), [telemetry promise](../../telemetry.md), and
[library-first boundaries](../../library-first.md). All load-bearing scope and acceptance
requirements for this brief are restated below; sibling briefs describe providers, not
additional hidden acceptance criteria. Refresh active claims and source paths before edits.

## Human decision

Confirm that the exact preview, independently gated upload, selected history window and replay behavior match the published collection policy. No actual destination is enabled by accepting implementation tests.

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

1. Add perfdata send --upload with explicit bounded source/date window and operator-owned destination/auth profile. Both ASSAY_TELEMETRY=1 and the command-line upload action must be present for every batch. Dry-run validates and prints the exact event payload with zero DNS/socket/HTTP attempts. Before first contact, print the resolved destination origin and auth-profile name (never the credential). Refuse a plain-http destination that is not loopback, so a source credential is never sent unencrypted. Default command behavior never transmits; do not populate statusgen's empty endpoint or turn on an automatic daemon.

2. Use the common event serializer and batch fixtures. Copy records into a stable batch before sending, preview exactly those bytes, and reject unsupported profile/schema. Destination credentials stay in the sender; source paths and alias mappings never enter a batch. Reject cross-origin redirects and credential forwarding; bound HTTPS timeouts and retry-after/backoff/jitter under a per-run work/time limit.

3. Persist per-destination/source-epoch checkpoint and per-event outcomes atomically only after committed ACKs. ACK loss resends the same IDs/bytes. Partial acceptance advances accepted events without skipping retryable ones; permanent rejections are visible with fixed reasons and do not cause an infinite head-of-line block. Different payload under the same ID never mutates or overwrites the old event.

4. Reconcile concurrent sender ownership, process restart, source rotation, truncation, corrupt tails and retention expiry. Preserve a single bounded uploader per selected spool/destination; telemetry locks never block work producers. Queue-full or expired-unsent data is a measured loss where observable, never an unbounded retention override. Report pending bytes/oldest age and unknown gaps.

5. Recheck consent/profile/window before each retry or new batch. Turning upload off prevents future calls and does not erase local data; an already submitted request may commit. Changing destination does not silently replay all history; new destination needs its own explicit window/preview and scoped credential.

6. Document invocation, preview, stop, retry and local-only behavior. The source module stays usable without server/database packages; sender tests against contract fakes are sufficient for this brief, while /06 owns real complete-path qualification.

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
| 1 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestUploadConsent$' ./upload/... > "$o" && grep -F -- '--- PASS: TestUploadConsent ' "$o")` | exit 0; named PASS. Every missing-switch, dry-run, off-between-batches and disallowed-profile case attempts zero new network calls; previews exactly match sent payload bytes. |
| 2 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestUploadResume$' ./upload/... > "$o" && grep -F -- '--- PASS: TestUploadResume ' "$o")` | exit 0; named PASS. Lost ACK, partial ACK, restart and concurrent senders preserve IDs and checkpoint only durable accepts; permanent rejections remain visible. |
| 3 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestUploadRetention$' ./upload/... > "$o" && grep -F -- '--- PASS: TestUploadRetention ' "$o")` | exit 0; named PASS. Rotating/truncated/expired sources and bounded offline backlog expose lost/unknown windows; producers remain independent. |
| 4 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestUploadDestination$' ./upload/... > "$o" && grep -F -- '--- PASS: TestUploadDestination ' "$o")` | exit 0; named PASS. Cross-origin redirects, changed destination and expired source credential cannot forward secrets or silently replay old history; retries/timeouts are bounded. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/05` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |

## Pre-mortem and detection

Drop the upload flag check, advance checkpoint before ACK, or generate new IDs on retry; consent and resume cases must fail. Missing coverage is not a successful zero-data test. The end-to-end
row must traverse actual public producer/reader or served interfaces. Scope/retention
adequacy and deployment workload selection are review judgments; their measured evidence
is required before activation, not invented from component-test success.

## Perf-v3 delivery amendment — 19 October 2026

Apply the [perf-v3 contract](../../performance-attribution-spec.md). Existing Tasks/Verify/Evidence and consent/retention controls remain owed.

Send only profile-selected aliased collection events, never original perf-v3 day files or
raw local work locators. Preserve invocation/event identity, causal links and nullable timing/
GraphQL coverage across resume, lost ACK and consent withdrawal. Extend the existing preview/
replay/opt-out Verify flow with mixed source versions and verify zero sockets in dry-run,
no raw work/native-harness references in bytes, and byte-stable identity on retransmission.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. -->

## Review

Gate: human. Review actual disclosure/identity boundaries, loss visibility and shared-value
flow evidence. Required operational records must still function with optional collection
disabled. Human acceptance changes no live destination, credential, collection switch or
retention policy.
