---
stream: cell-api
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: [3669]
board: generated
spec: docs/proposals/non-ci-architecture/21-cell-operator-surface.md
---

# Cell API and deskd migration

Move the reusable deskd service, read model and client seam into public Assay, then
package it for the interactive Kubernetes cell. This is R7's existing owner, not a
second controller or forge-service programme. All seven briefs are authored work,
not delivered code or deployment qualification. **The stream is active for implementation.**

## Authority and admission

The [whole target](../../proposals/non-ci-architecture/11-target-architecture.md)
places the service and cockpit in Assay. The maintainer's merge of
[PR #2890](https://github.com/medici-finance/assay/pull/2890) on
2026-10-14 at 20:35:08 UTC (merge `716a74574ffe5d404f727dcac7c98a1e6b1bd5b3`)
is the ratification venue explicitly named by amendment 21. It selects its recommended
option: R7 belongs to a new **parked** `cell-api` stream; R2 records lifecycle requests,
R3 executes the allowed transitions, and approval-based apply stays deferred.
The earlier “proposed / pending merge” wording in amendments 20/21 is a historical
pre-merge account, not evidence that the merge is still outstanding. This scaffold
records the completed ownership decision without treating it as permission to activate.

[Delivery sequence](../../proposals/non-ci-architecture/16-delivery-sequence.md),
[manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md),
[cell operation](../../proposals/non-ci-architecture/21-cell-operator-surface.md),
[forge service](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md),
[runtime alignment](../../proposals/non-ci-architecture/23-runtime-contract-alignment.md)
and [interactive Kubernetes direction](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md)
remain controlling inputs, with each document's own approval scope. Issue
[#3669](https://github.com/medici-finance/assay/issues/3669) requests all reusable work
here. It does not ratify every mechanism in later draft amendments. The file/package
mapping below is the bounded implementation proposal for review; no new custody profile
or authentication mechanism is selected by authoring it.

Unparking requires the existing admission process to record the applicable controlling
contract and decision provenance. Both are recorded here. Decision provenance:
the maintainer's own unpark request (20 October 2026): https://github.com/medici-finance/assay/issues/3669#issuecomment-6302690781
Controlling contract: amendment 21 as ratified by the merge of medici-finance/assay#2890;
its header now records that approved status. The merge of the change that flipped this
README to `active` is the scheduling record. Pure interface/fixture work can precede
R5/R6 runtime qualification only through that process: ordinary brief admission, under
which each brief retains its declared dependencies and human gates.
Service activation still requires R5/R6,
R3/R4 authority and the consuming profile's human acceptance. Agent-execution stays
parked on its own terms. A recorded dependency or passed unit test cannot lift those gates.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Publish the deskd contract and source handoff inventory](brief-01-publish-the-deskd-contract-and-source-handoff-inventory.md) | 8 | M | todo | — | — |
| 02 | [Compose the public deskd service and read model](brief-02-compose-the-public-deskd-service-and-read-model.md) | 9 | M | todo | — | — |
| 03 | [Qualify mounted cell API authorization and privacy](brief-03-qualify-mounted-cell-api-authorization-and-privacy.md) | 13 | M | todo | — | — |
| 04 | [Move thin CLI MCP and cockpit clients onto the public API](brief-04-move-thin-cli-mcp-and-cockpit-clients-onto-the-public-api.md) | 14 | M | todo | — | — |
| 05 | [Package the deskd image and generic Kubernetes service profile](brief-05-package-the-deskd-image-and-generic-kubernetes-service-profile.md) | 14 | M | todo | — | — |
| 06 | [Bind the in-cluster cellctl operator to the public service](brief-06-bind-the-in-cluster-cellctl-operator-to-the-public-service.md) | 15 | M | todo | — | — |
| 07 | [Qualify handover rollback and last-consumer retirement](brief-07-qualify-handover-rollback-and-last-consumer-retirement.md) | 16 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Bounded refactor map

Source inspection on 2026-10-20 distinguishes reusable legacy behavior from a supported
public implementation. The legacy package names, exact revision and consumer locators
remain in the source owner's private crosswalk. Public acceptance uses synthetic
fixtures and a published contract, never access to that private tree. Brief 01 must
produce the revision-bound sanitized handoff inventory before code transfer; this
paragraph is not a claim that every source consumer has already been accounted for.

| Surface inspected | Take, reshape or retain | Public owner and destination proposal |
|---|---|---|
| Typed HTTP/JSON v1 reads, ETags, errors; SSE events, cursor/resync; Go client and generated TypeScript client | Preserve useful wire behavior through an explicit compatibility matrix. Bound bodies/cursors and keep unknown/freshness visible; do not inherit arbitrary route access or machine-trust shortcuts. | 01: `tools/desk/cellapi/`, `spec/cell-api-v1.md` (planned), `schemas/cell-api-v1.json` (planned); pure public client package, no internal imports by consumers |
| Service composition, derived inbox/cell/roadmap projections, source readers and local presentation markers | Move the service code and read-model adapters; share the existing R6 mirror and R1 projections. Classify local markers separately from work/effect authority. The legacy SQLite index is not the target coordination store. | 02: `tools/desk/cmd/deskd/`, `tools/desk/internal/cellapi/`; R2 PostgreSQL and existing forge mirror remain sole owners |
| Mounted mux, capability guard, caller identity, per-repository filtering, named human actions and MCP authority assertions | Reuse the proven assertions, then exercise the actual public mounted handlers. Legacy bearer/loopback, terminal-input or human-write behavior is compatibility evidence, not authority for a public network listener. | 03: same composition plus existing R3/R4 and forge-neutral/40 custody; default read/plan-only ingress |
| CLI, MCP, generated client and cockpit connection adapter | Move thin adapters onto one versioned API. Cockpit follows the service; preserve only the selected read/status/reconnect workflow in this set. Feature/packaging redesign is outside it. | 04: public CLI/MCP adapters and a small cockpit client example; legacy consumers remain until explicitly migrated or retained |
| Existing standalone service image with a derived-index volume | Rebuild a publication-clean service image from public source. Keep service host separate from the in-cluster operator and five agent Pods. Replace legacy index/storage wiring only through 02's classification. | 05: `images/deskd/` and generic `containers/k8s/components/cell-api/` |
| Terminal backends and local process/session controls | Retain legacy adapters until qualified replacement; route managed lifecycle and Kubernetes attach through the shared runner and existing cellctl profile. No copied spawn/kill loop inside deskd. | 06 consumes desk-containers/28, graph-execution/26–27 and agent-execution/09 |
| Source configuration, credentials, documentation, builds, pins and outstanding work | Publish generic schemas/examples only. Keep adopter roster/values, issuer/operator configuration, private crosswalk, secrets and historical incidents at their original custody. Inventory last consumers and qualification obligations before retirement. | 01 inventory → 07 migration/rollback under composability/03 (R8); source owner reconciles private work separately |

The contract package can be imported by outside clients; `tools/desk/internal/cellop`,
`deskkit` and the planned `forgeservice` cannot. Keep their effectful composition within
the desk module. Map the existing `cell-op-v1` DTO at that seam rather than exporting
its plan store/apply machinery or creating another lifecycle evaluator. Reuse
forge-neutral/39's typed contract instead of defining a second forge API. Any legal
extraction needed for reuse is a deliverable of 01, with the importing consumer tested.

## Implemented foundations versus planned delivery

Public main `15b60414f3ec134942a0ec3162b550cd95cbbc8e`, inspected 2026-10-20,
contains `tools/desk/internal/cellop/`, `spec/cell-op-v1.md`, `cellconfig/`,
`forgeread/reader.go`, `forgeread/offline.go`, `loopadmin/runner/` and the five desk
images. At that revision, `tools/desk/cmd/deskd/`, `tools/desk/cellapi/`,
`tools/desk/internal/cellapi/` and `tools/desk/internal/forgeservice/` are absent.
The R6/service briefs on main are obligations, not proof those packages shipped.
The Kubernetes workload package is a separate delivery; it does not move deskd.
No source-code test, cluster deployment, performance benefit or full consumer inventory
is claimed by this authoring change. Existing implementation and independent Evidence
stay intact, including verified work awaiting generated-board reconciliation.

## Code, configuration and credential boundaries

- Public Assay owns reusable service/client code, schemas, images, generic profiles,
  tests and migration tooling. Source owners retain historical decisions and active
  work/claim obligations until their crosswalk is reconciled; no blanket source closure.
- Operator-controlled configuration selects cell, namespaces, endpoints, trusted issuer,
  audience, source admission and role bindings. It is outside evaluated workspaces.
  A request, header, environment field or a broader ingestion identity grants no role.
- Query/service processes have only their admitted read capabilities. Brief 05 proposes
  a separate per-cell read-only database identity, provisioned by the existing database
  owner; the service never holds the cell owner/login role or an administrative credential.
  Its grants, secret references and effective policy require independent proof and human
  acceptance before exposure. R4 executors keep
  separate per-role publisher credentials; R3 keeps operator and candidate environments
  apart. No all-role broker, operator key on the service, or human credential at rest.
- Initial network serving is read/plan-only. Human named actions remain a closed set
  behind their own evidence/decision contract and are not activated by these briefs.
  The legacy human-answer and adopt tools remain absent from agent-facing discovery and dispatch.
  An API plan is never approval, and an approval record is never an unqualified apply route.
- CLI, MCP and later cockpit use the same authorized results. Filter before private
  joins, counts, errors, pagination, ETags and SSE events. Reauthorize cached reads and
  resumed streams; unknown coverage remains unknown. Reconnect cannot launch work.

## Dependency waves and critical path

The generator-derived wave values include external prerequisites. Within this set:
`01 -> 02 -> 03 -> 05 -> 06 -> 07` and `03 -> 04 -> 07`; 07 joins 04 and 06.

- 01 consumes forge-neutral/39 and desk-containers/15: compatibility/source inventory
  and pure protocol work, with no running database prerequisite.
- 02 joins the existing R6 mirror (forge-neutral/37) and controller projection owner
  (graph-execution/21), without recreating either.
- 03 requires R5's complete verifier proof (graph-execution/18) and the qualified
  R3/R4 serving boundary (forge-neutral/40). This preserves the R5/R6 activation path.
- 05 consumes the CNPG profile and desk-containers/29's existing qualification harness;
  06 consumes the dedicated in-cluster operator profile.
  05 owns the additive read-role/profile amendment and the service-specific disposable
  network qualification; desk-containers/29 continues to qualify the desk/operator profile.
  04 is the bounded client migration, including the later thin cockpit connection.
- 07 joins the Kubernetes cell qualification, forge-service qualification and R8
  migration tooling before any legacy last-consumer retirement.

**Actual runtime head:** admitted R1 contracts → existing R2/R3 and R4 qualification →
R5/R6 → 03 → 05 → 06 → 07. Local 01 is the smallest interface step after admission;
it is not an available deployment shortcut. The earlier R7 estimate of two or three
briefs grouped the whole API/cockpit; this seven-brief decomposition separates source
compatibility, service, trust boundary, clients, packaging, operator binding and retirement
so each has one reviewable obligation. It adds no runtime owner.

**Tempting dead end:** copying the legacy service and SQLite index into a Pod or letting
cellctl act as another supervisor. Neither proves scoped service access, preserves the
single runtime authority nor accounts for old publishers and readers.

## Proof and open decisions

Every brief names exact planned tests, an independent oracle and a negative control.
Tests must run the real mounted handlers/import closure/rendered profile; a route-name
list or a schema-valid response alone is insufficient. Public synthetic fixtures cover
foreign cells/repos/counts, role spoofing, revoked access during SSE, stale generation,
unknown effects, health without dependencies and reconnect without dispatch. An adopter
qualification separately tests effective policy and custody in an explicitly owned
environment; offline plans cannot certify it.

No new RTO, RPO, latency, capacity, credential lease or calendar deadline is invented.
Use inherited workload/measurement contracts; before exposure the human owner selects
any missing deployment targets and the supported client-version window. Readiness,
recovery, effects and acceptance remain separate facts. New human decisions default to
`none`; the private source's former destination choice is already answered by #3669.

## Desk-instance amendment — 25 October 2026

The [requested instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) amends unfinished owner briefs without changing lifecycle status or completed Evidence. Shared R2/R3 instance ownership/capacity → standing clients (graph-execution/27) → desired-population adapter (desk-containers/31) → lifecycle/Kubernetes consumers. API contract work can proceed against the declared versioned shape; runtime activation still requires its actual dependencies and independent qualification. Paused instances and draining predecessors remain part of pickup/retirement inventory.
