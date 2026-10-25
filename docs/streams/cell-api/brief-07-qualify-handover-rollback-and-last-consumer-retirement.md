---
brief: assay:assay:cell-api:07
title: Qualify handover rollback and last-consumer retirement
why: "Public code and green fixtures do not retire existing service consumers or writers. A complete inventory and reversible handover proof prevent orphaned clients, lost history and simultaneous authority during the move."
wave: 16
depends: ["cell-api/04", "cell-api/06", "desk-containers/29", "forge-neutral/45", "composability/03"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
gate-why: "Confirm one scoped handover and its independently demonstrated rollback, including complete consumer and credential inventory and preserved evidence. Actual source closure, capability retirement and durable-data disposition require the responsible human owner; no inventory entry authorizes deletion by itself."
issues: []
schema: brief-v2
version: 3
id: f53b3589-27a7-4af6-ba9b-953e01597197
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
  - "tools/desk/cmd/deskmigrate/: follow-up cell-api/07"
  - "tools/desk/internal/cellapi/: follow-up cell-api/07"
  - "tools/desk/testdata/cellapi/: follow-up cell-api/07"
  - "migrations/: follow-up cell-api/07"
  - "docs/cell-api-handoff.md: follow-up cell-api/07"
  - "docs/cell-api-retirement.md: follow-up cell-api/07"
  - "changelog/cell-api-handover.md: follow-up cell-api/07"
  - "legacy source consumers: out-of-scope (private source owner retains their exact crosswalk; this brief records public dispositions and does not edit them)"
---

# Brief 07 — Qualify handover rollback and last-consumer retirement

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `tools/desk/cmd/deskmigrate/`, `tools/desk/internal/cellapi/` (planned), `tools/desk/testdata/cellapi/` (planned), `migrations/`, `docs/cell-api-handoff.md` (planned), `docs/cell-api-retirement.md` (planned), `changelog/cell-api-handover.md` (planned)

facts:
- pin: public main 15b60414f, inspected 2026-10-20; R7 migration is not delivered.
- gates: cell-api/04 and /06 close selected clients/operator; desk-containers/29 and forge-neutral/45 provide their separate qualification; composability/03 owns R8 tooling.
- inventory: 01 records every source/consumer and unresolved PR/claim/verification obligation; unknown coverage forbids last-consumer retirement.
- authority: one mutating cell per external repository; restore never renews old capabilities or converts an unknown effect into a retry.

layering: Extend the existing migration tool and receipt model; no new handover store or migration daemon. Task 1–4 and Verify 1–3 test real handover plans and lower authority fences.

single-point-of-failure: old-writer fence/revocation is the cutover control; restoring a service or database must not restore that authority. Verify 2 exercises the fenced incarnation against R2 incarnation and R4 role/epoch admission, with unknown effects held. Inventory completeness is a separate source-owner judgment checked by Verify 1 and 3, never inferred from a green fence test.

design-fit:
  owner: R8 migration/retirement in composability/03, consuming R7 service and existing R2/R3/R4/R6 qualification
  contract: none — R7's transport responsibility is registered by cell-api/01; existing semantic owners and contracts remain controlling
  retires: ["legacy service and adapter path only after qualified handover and explicit last-consumer act"]
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

Confirm one scoped handover and its independently demonstrated rollback, including complete consumer and credential inventory and preserved evidence. Actual source closure, capability retirement and durable-data disposition require the responsible human owner; no inventory entry authorizes deletion by itself.

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

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner\'s delta; unchanged obligations remain.

## Task

1. Extend composability/03 migration planning and receipts with the deskd service/client/profile tuple and 01 inventory. Reconcile every source module, build/image pin, CLI/MCP/cockpit consumer, deployment/credential binding and unfinished task/dependency/open PR/claim/evidence obligation. Publish sanitized dispositions and owner receipts only; the source owner updates its private crosswalk/board separately. This public brief cannot close private work or silently adopt an active claim.

2. Shadow authorized old/new reads using matched subjects, source frontiers, admission, workload and versions. Classify differences in coverage, state, freshness, acceptance and performance; do not hide an inaccessible source as empty. Selected client flows must use the same qualified API; unsupported clients retain a supported legacy path and block any claim that the old service has no consumers.

3. Rehearse scoped handover in disposable fixtures: drain old work, reconcile pending/unknown external effects, fence/revoke old writer capability, record current incarnation/assignment and qualify the new service/clients/operator before switching. Dereference 05's service-specific network and database-identity receipt against the exact candidate image/profile/grants; desk-containers/29's desk/operator qualification cannot substitute for it. Missing, stale or mismatched evidence holds handover. Preserve evidence, human decisions and history. SQL mirror or cache loss cannot mint authority. No actual live cutover, install, credential revoke or source deletion is performed by this implementation.

4. Rehearse rollback for failed cutover and later service failure. Stop/fence the new owner, reconcile possibly submitted effects, restore the recorded supported service/client/profile binding and obtain current operator authority before any writer returns. An old database/volume/backup does not restore credentials or unspent budgets. Where safe rollback cannot be established, hold; never run both writers.

5. Emit last-consumer retirement eligibility only when inventory is complete, every required source-owner receipt and acceptance is current, no active consumer/claim/effect is unresolved, retained history is readable and rollback is proven. Separate eligibility from the human retirement act. Publish removal conditions and test-ledger departure report; no blanket deletion and no weakened security gate. Source owner performs its explicitly authorized reconciliation later.

6. Update migration notices and handoff/retirement docs with exact qualification revisions, limits and operator acts. Keep all independent Evidence and historical rulings; plans are not deployment or retirement receipts.

- **Instance amendment:** Inventory every managed/legacy instance, paused/draining predecessor and shared reservation during cutover/rollback. Retire only after all applicable owners/effects and required instance-capability consumers reconcile. Preserve one mutating cell per repo; within-cell replacement overlap is not permission for simultaneous legacy/new writers outside shared admission.

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
| 1 | check:ci +flow +mutation +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellHandover$' ./cmd/deskmigrate/... > "$o" && grep -F -- '--- PASS: TestCellHandover ' "$o")` | exit 0; named PASS. Real migration plan/receipt flow uses scoped old/new snapshots and owner receipts; an unlisted consumer, active claim or missing independent acceptance blocks retirement. Missing/stale/mismatched 05 image/profile/grants-bound service qualification blocks handover, including substitution of the desk-only qualification receipt. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellRollback$' ./cmd/deskmigrate/... > "$o" && grep -F -- '--- PASS: TestCellRollback ' "$o")` | exit 0; named PASS. Disposable old/new executor fixture tests crashes before/after provider submission, unknown effects and restore; fenced old incarnation cannot write and rollback never produces dual authority. |
| 3 | check:ci +dereference +mutation +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellRetirement$' ./cmd/deskmigrate/... > "$o" && grep -F -- '--- PASS: TestCellRetirement ' "$o")` | exit 0; named PASS. Last-consumer decision dereferences every inventory disposition and retained-history receipt. A planted second image pin/consumer and stale credential receipt hold; eligibility cannot execute deletion. |
| 4 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/07 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers require review. |
| 5 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./internal/cellapi/integration/... -run '^TestDeskInstanceCutover$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceCutover ' "$o")` | Named test executes (zero matching tests is not a pass); Paused/draining/unknown legacy instance or unsupported required consumer holds retirement; rollback preserves reservations and operator incarnation. Mutation: inventory only one member per role makes the test fail. |
## Threat model and detection

| Failure mode | Caught by |
|---|---|
| Source obligations or a second consumer vanish from the migration report | row 1 |
| Failed cutover or old restore creates duplicate writer authority | row 2 |
| Retirement drops readable history or treats incomplete inventory as complete | row 3 |
| A fixture is mistaken for deployment authority or a private inventory is called complete without its owner | No row; review-only: inspect decision provenance, source-owner receipt and explicit operational limits before acceptance. |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or deployment result is asserted. -->

## Review

Gate: human. Review the actual mounted/effect boundary and retained ownership, the
compatibility inventory and the independent negative controls. Test success grants
neither deployment nor credential provisioning, source closure or a human decision.
