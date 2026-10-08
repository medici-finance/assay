# Library-first component contracts

Status: proposed implementation direction, 2026-10-08; adoption is reviewed with this plan.
This document describes upcoming work, not a claim that the SDKs or extractions exist.

Shared behavior has one canonical implementation exposed through versioned libraries or
SDKs. Consumers link it directly where compatible with their authority and deployment.
CLIs are adapters, not mandatory cross-module integration boundaries. A process boundary
remains where it provides independent authority, isolation or deployment separation; its
client SDK carries typed requests, never the server's credentials. Imports alone are not
runtime containment. Taxonomy membership describes components and grants no authority.

## Ownership and allowed access

| Meaning | Canonical implementation / planned interface | Consumers and restrictions |
|---|---|---|
| Forge facts and completeness | Existing forge adapters; narrow `forgeread` API, forge-neutral/36 | `deskread`, statusgen's reader adapter. No generic query, raw token getter or write API on the consumer interface. |
| Eligibility and evidence coverage | `statusgen/evaluation`, statusgen/15 | statusgen commands, workflow bindings, projection producers. Explicit observations in; deterministic verdicts out. No admission, identity, capacity or claim authority. |
| Stream-view wire schema | Existing `statusgen/streamview` | Producers and authorized readers pin the contract; no policy or serving behavior in this package. |
| Workflow progression | `workflow` store/controller, graph-execution/19 and /21 | Calls evaluation and loop-admin APIs; process exit is not work acceptance. |
| Process supervision | `loopadmin`, graph-execution/26 | Standing desks and workflow stages; no dependency on workflow store, no effect credentials. |
| Operator lifecycle | cellctl's operator libraries and typed service clients | Operator provisioning stays outside working sessions; library reuse does not merge authorities. |

The existing semantic-owner index in `docs/contracts.md` remains the ownership register.
Implementations update its rows and existing architecture checks; this table is a migration
map, not a competing register. Module placement is not a new semantic owner.

## Authority and compatibility

- Offline/frozen inputs require no credentials and remain the default. Missing, partial,
  stale and unsupported inputs retain distinct unavailable/held outcomes.
- Caller-facing SDK packages must not transitively import custody, credential minting,
  role executors or write adapters. Authenticated read adapters are effectful packages,
  composed only where the existing identity policy permits that process to hold access.
- A role-owned credential is not passed into statusgen to avoid a subprocess. Retain the
  current deskread bridge as an explicit compatibility adapter until the approved isolated
  reader client is available. No new daemon, token service or credential profile is implied.
- The CI workflow-token profile stays owned by forge-neutral/34. Extracting its constructor
  later must preserve opt-in, repository/host binding, read-kind limits and refusals. The SDK
  does not grant a new CI identity policy or widen token scopes.
- Versioned JSON remains supported for shell/non-Go consumers. The Go API and wire contract
  have distinct compatibility obligations. Preserve command flags, exit codes and envelopes.
- Same input means the same snapshot, evaluator version and policy digest. Library reuse
  alone does not guarantee identical answers from different observations or dependency pins.
- Pin module versions, keep release/build resolution working with `GOWORK=off`, and exercise
  an external consumer from a local module proxy with no workspace or relative replace.
  This is packaging qualification, not permission to cut a release or contact providers.

## Routing and sequence

| Work | Owner | Dependency / retirement |
|---|---|---|
| First shared fact-read slice and SDK | forge-neutral/36 (new) | Existing OpenIssues implementations; no dependency on /18, /34 or /35. Preserves their identity work. |
| Remaining statusgen read migrations | forge-neutral/18 | /36 plus existing /33 and /34; preserve /35's separately reviewed ruling-control moves. |
| Ruling and sign-off reads | forge-neutral/35 | /18 and its existing controls; use the shared reader, no independent credentials. |
| Canonical evaluation extraction | statusgen/15 (new) | Offline extraction from existing code; no dependency on fact-reader migration or workflow store. |
| Durable bindings | graph-execution/19 | Consumes statusgen/15; no second extraction or copied evaluator. |
| Hold no-forge-CLI / no-ambient behavior | desktools-v2/08 | After /18 AND /35; permit narrow SDK imports, prohibit capabilities, not every library. |
| CLI parsing | desktools-v2/16, /18 and remaining CLI briefs | Continue independently; no change to semantic or authority contracts. |

Wave 0: review this contract and the named consumer amendments. Wave 1: /36 and
statusgen/15 independently. Wave 2: their named consumers, subject to existing dependencies.
Retirement follows consumer qualification, never merely library availability. Existing
implemented briefs keep their evidence and scope; changes land as follow-ups.

The first blockers are concrete: statusgen's implementation is `package main`, its evaluator
loads files, and the forge implementation is behind `tools/desk/internal`. Extract those
specific seams; do not export all of deskkit or wait for a cell taxonomy registry.
