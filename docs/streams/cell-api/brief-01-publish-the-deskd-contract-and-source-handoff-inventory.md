---
brief: assay:assay:cell-api:01
title: Publish the deskd contract and source handoff inventory
why: "A service move is unsafe while clients, wire behavior and private residues are implicit. A public contract and source handoff inventory make migration independently executable without silently stranding consumers."
wave: 8
depends: ["forge-neutral/39", "desk-containers/15"]
unblocks: ["cell-api/02"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the supported compatibility window, sanitized source and consumer inventory, and public protocol fields before consumers depend on them. Publication of private source or credential material is excluded."
issues: []
schema: brief-v2
version: 2
id: 3dd33603-816a-4f85-9960-0ea8d18d1245
authored: 2026-10-20 by cell-api authoring
decision-trigger: spec
exec-tier: strong
exec-tier-why: Cross-component compatibility, privacy and authority boundaries require design judgment and independent negative controls.
domain: complicated
outcome: none
sources:
  - docs/streams/cell-api/README.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md
  - docs/proposals/non-ci-architecture/21-cell-operator-surface.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md
  - https://github.com/medici-finance/assay/pull/2890
  - https://github.com/medici-finance/assay/issues/3669
  - freshness-checked 2026-10-20 @ 15b60414f3ec134942a0ec3162b550cd95cbbc8e
consumers:
  - "tools/desk/cellapi/: follow-up cell-api/01"
  - "spec/cell-api-v1.md: follow-up cell-api/01"
  - "schemas/cell-api-v1.json: follow-up cell-api/01"
  - "tools/desk/testdata/cellapi/: follow-up cell-api/01"
  - "docs/cell-api-handoff.md: follow-up cell-api/01"
  - "docs/contracts.md: follow-up cell-api/01"
  - "docs/library-first.md: follow-up cell-api/01"
  - "changelog/cell-api-contract.md: follow-up cell-api/01"
  - "legacy source consumers: out-of-scope (private source owner retains their exact crosswalk; this brief records public dispositions and does not edit them)"
---

# Brief 01 — Publish the deskd contract and source handoff inventory

## Context

files: `tools/desk/cellapi/` (planned), `spec/cell-api-v1.md` (planned), `schemas/cell-api-v1.json` (planned), `tools/desk/testdata/cellapi/` (planned), `docs/cell-api-handoff.md` (planned), `docs/contracts.md`, `docs/library-first.md`, `changelog/cell-api-contract.md` (planned)

facts:
- pin: public main 15b60414f, inspected 2026-10-20; tools/desk/cellapi is absent.
- reuse: cell-op-v1 exists; tools/desk/internal/cellop and deskkit cannot be imported by outside clients.
- source: typed v1 HTTP/JSON, ETags, SSE replay/resync, Go and generated TypeScript clients exist in the legacy inventory; they do not prove public network qualification.
- boundary: forge-neutral/39 owns forge envelopes; this brief does not redefine them.

layering: Pure wire types and deterministic mappings, a separate explicit HTTP client and desk-module composition for internal cellop. Task 1–3 and Verify 1–2 pin legal imports and effects.

single-point-of-failure: the wire mapping can misstate cell identity, coverage or resync even when schema-valid. Verify 1 checks independent expected fixtures and Verify 2 checks an external import closure; these are detection oracles, not a second runtime authorization layer. This contract-only brief mounts no network service; 03 owns mounted authorization.

design-fit:
  owner: R7 transport contract in tools/desk/cellapi; cell-op and forge meanings retain their owners
  contract: none — R7's transport responsibility is registered by cell-api/01; existing semantic owners and contracts remain controlling
  retires: []
  weight: verbs +0; flags +0; refusals +0; rule-text +0 (no new operator vocabulary proposed; record any actual implementation delta for review)
  why-add: Reuse the existing owners and one client seam; remove duplicate service composition only after the last-consumer proof. Package/process additions are bounded by the declared files, not permission to add another controller.

All planned paths and named tests below are implementation deliverables. The source pin
proves the starting point, not that these deliverables exist or were tested. The
[stream refactor map](README.md#bounded-refactor-map) defines the public transfer boundary.

## Read first

Read the [R7 ownership and admission record](README.md#authority-and-admission),
[cell operator contract](../../proposals/non-ci-architecture/21-cell-operator-surface.md),
[forge-service boundary](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md),
[interactive Kubernetes direction](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md),
`spec/cell-op-v1.md`, `docs/contracts.md` and `docs/go-engineering.md`.
Resolve every declared dependency's current contract and independent Evidence before
pickup. The package mapping is a reviewable proposal; a conflicting contract is
NEEDS_CONTEXT, never permission to select new custody or duplicate its owner.

## Human decision

Confirm the supported compatibility window, sanitized source and consumer inventory, and public protocol fields before consumers depend on them. Publication of private source or credential material is excluded.

Options:
1. **Accept the demonstrated bounded design** — accept the evidenced compatibility and containment for its declared scope; provisioning, deployment and source retirement retain their separate operator acts.
2. **Hold for correction** — name the missing proof or incompatible behavior; retain the current supported path until corrected.

Default if no answer: none — blocks until answered

The decision becomes concrete from the implementation design and evidence. No answer
keeps acceptance pending; it grants no time-based default or activation authority.

## Ground rules

- Work in an owned branch; no merge, ready flip, release, installation or live infrastructure contact.
- Public source and fixtures must be self-contained and synthetic. Never copy private locators, configuration, credentials, incidents or history into Assay.
- Preserve the single R2 PostgreSQL runtime, R3 isolation, R4 role authority and R6 forge mirror. A service, terminal, restored file or cached result grants no authority.
- Keep this stream parked until admitted. Stop at implemented; independent verification, human acceptance and operational cutover remain distinct.

## Task

1. Publish the versioned cell API contract, independent literal fixtures and supported old/new client matrix. Inventory reads, ETags, null/unknown coverage, errors, SSE event kinds/cursors/resync, terminal output, local presentation markers, human named actions and agent tool discovery. Every legacy surface gets one disposition: preserve, translate, explicitly unsupported while legacy retained, or separately gated. Unknown mandatory schemas fail explicitly.

2. Create a narrow public Go package and explicit endpoint/transport HTTP client inside the existing desk module. Separate pure envelope mapping from HTTP effects. Prove an outside module can import it with GOWORK=off. Keep deskkit, token minting, plan stores/apply and raw role/URL overrides out of its closure. Map existing cell-op-v1 at a legal internal composition seam and reuse forge-neutral/39.

3. Publish a sanitized handoff inventory with opaque component/consumer IDs: source code/imports/build inputs, CLI/MCP/web/cockpit and generated clients, images/installers/pins, config and credential references, and unfinished source tasks/dependencies/open PRs/claims/verification obligations. Each entry names source owner, revision-bound private receipt, public destination, disposition, acceptance owner and retirement condition. The private crosswalk retains exact source locators; unavailable source access means incomplete inventory, never zero consumers.

4. Apply the README refactor map: take service/contracts/adapters, reshape storage and authority wiring, retain unsupported callers. Public code and fixtures build without the private tree. No visibility/license change, private history copy or source-work closure; all reusable implementation lands in Assay.

5. Register only the new transport responsibility in docs/contracts.md and docs/library-first.md. Publish the handoff/compatibility guide and regenerate contract fixtures. Keep admission and deployment gates intact.

## Interface and acceptance contract

Consume cell-api/01's versioned HTTP/JSON and SSE seam and the existing cell-op-v1
result/plan meanings. Authentication, current authorization, provider custody and
acceptance are separate. Required coverage/freshness, exact subject and unknown outcomes
survive every adapter. No agent-facing legacy human-answer or adopt tools; no generic role,
URL, SQL-write, token or apply escape hatch. Human named actions remain closed and
separately qualified. Initial service exposure is read/plan-only; approval-based apply
stays absent until amendment 21's five R7/R3 properties are independently qualified.

Update the declared documentation and generated schema/client/profile examples affected
by the Task. Preserve prior Evidence and source history; unresolved consumer or contract
coverage is a hold. Do not invent deployment performance/recovery thresholds: reuse the
inherited workload and require the operator to select missing targets before exposure.

## Verify

These exact named tests must be delivered and run, with a recorded failing control
before the restored pass. Each uses real production composition and an independently
specified expected result; zero selected tests or a skipped required fixture is failure.
Only explicit disposable fixtures may open local listeners/databases. No production or
ambient kubeconfig is permitted. Real-profile qualification remains a separate operator
act; offline passes do not establish it.

For row 4, the independent verifier obtains the actual implementation delivery from its
merged PR and records its full immutable SHA in `ASSAY_DELIVERY_SHA` and its actual
first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither defaults to HEAD, origin/main or
this authoring PR. Both commits and the fetched mainline must exist locally; the command
requires the delivery to have landed on that mainline before checking its immutable
snapshot. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed. Record
exact corroborated/disproved/unchecked counts; unchecked required consumers remain held
and omitted consumers remain a review obligation. These are future delivery inputs.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellWireCompat$' ./cellapi/... > "$o" && grep -F -- '--- PASS: TestCellWireCompat ' "$o")` | exit 0; named PASS. Legacy/new fixtures cross real codecs/client; schema-valid wrong cell, false completeness and wrong resync fail the oracle. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellClientBoundary$' ./cellapi/... > "$o" && grep -F -- '--- PASS: TestCellClientBoundary ' "$o")` | exit 0; named PASS. An external consumer builds; planted internal/credential import or ambient endpoint discovery fails. Explicit-input mapping has no hidden I/O. |
| 3 | check:ci +dereference +neighbour +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestHandoffCoverage$' ./cellapi/... > "$o" && grep -F -- '--- PASS: TestHandoffCoverage ' "$o")` | exit 0; named PASS. Published dispositions resolve to public contracts plus source-owner receipt metadata; a planted second consumer or live source obligation prevents complete/retirable output. |
| 4 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/01 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers require review. |
| 5 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| A compatible-looking wire change loses uncertainty or leaks another cell | row 1 |
| The client requires private/internal code or acquires credentials | row 2 |
| A source consumer or live obligation disappears | row 3 |
| A fixture is mistaken for deployment authority or a private inventory is called complete without its owner | No row; review-only: inspect decision provenance, source-owner receipt and explicit operational limits before acceptance. |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or deployment result is asserted. -->

## Review

Gate: human. Review the actual mounted/effect boundary and retained ownership, the
compatibility inventory and the independent negative controls. Test success grants
neither deployment nor credential provisioning, source closure or a human decision.
