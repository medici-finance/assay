# Library-first component contracts

Status: implementation direction, 2026-10-08, ruled for offline and frozen inputs only. The
driver ruled on #2395 for option 1, recorded in
[`DR-forge-neutral-36`](streams/decisions/DR-forge-neutral-36.md): statusgen may link a narrow
module's offline and frozen packages; every online forge read stays on the read verb, `deskkit`
stays internal, and no credential moves into a process that has none today. The forge-seam
direction of 2026-09-14 (statusgen reaches the seam by running the read verb, never by linking
a package; see the desktools-v2 spec header) is amended in that part only. forge-neutral/36
becomes eligible for pick-up and keeps its own human gate; the ruling is not its sign-off. This
document describes upcoming work, not a claim that the SDKs or extractions exist.

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
- A role-owned credential is not passed into statusgen to avoid a subprocess. **Every online
  forge read statusgen makes stays on the deskread read-verb adapter**, a process boundary,
  under the identity the verb resolves. In-process SDK use inside statusgen is limited to
  offline and frozen inputs, which need no credential. `deskkit` stays internal: statusgen's
  module manifest names nothing under `tools/desk`. No brief in this plan retires the
  deskread bridge for any online read. Retiring it would need its own human-gated brief and a
  driver ruling, and none is authored. No new daemon, token service or credential profile is
  implied.
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
| Remaining statusgen read migrations | forge-neutral/18 | Existing /08, /33 and /34; no dependency on /36. Online reads stay on deskread; if /36 has landed, uses its OpenIssues types, and moves no adapter into the SDK; preserve /35's separately reviewed ruling-control moves. |
| Ruling and sign-off reads | forge-neutral/35 | /18 and its existing controls; reads go through deskread, no independent credentials. |
| Scan and CI path coverage of the extracted module | forge-neutral/36 | Lands with the extraction: forge-CLI scan, ambient-token rule and workflow path filter cover `forgeread/`. |
| Further read kinds in the SDK | not yet briefed | Each authenticated adapter moved into the SDK is a /36-class extraction under a human gate. |
| Canonical evaluation extraction | statusgen/15 (new) | Offline extraction from existing code; no dependency on fact-reader migration or workflow store. |
| Durable bindings | graph-execution/19 | Consumes statusgen/15; no second extraction or copied evaluator. |
| Hold no-forge-CLI / no-ambient behavior | desktools-v2/08 | After /18 AND /35; permit the narrow SDK's offline and frozen packages only; keep the manifest check that statusgen names nothing under `tools/desk`. |
| CLI parsing | desktools-v2/16, /18 and remaining CLI briefs | Continue independently; no change to semantic or authority contracts. |

Wave 0: review this contract and the named consumer amendments; the driver's ruling on the
forge-read direction is recorded (#2395, `DR-forge-neutral-36`). Wave 1: /36, under its own
human gate, and statusgen/15 independently. Wave 2: their named consumers, subject to existing dependencies; the forge
read chain (/18, /35, desktools-v2/08) keeps its own sequence and does not wait on /36.
Retirement follows consumer qualification, never merely library availability. Existing
implemented briefs keep their evidence and scope; changes land as follow-ups.

The first blockers are concrete: statusgen's implementation is `package main`, its evaluator
loads files, and the forge implementation is behind `tools/desk/internal`. Extract those
specific seams; `deskkit` stays internal, and nothing waits for a cell taxonomy registry.
