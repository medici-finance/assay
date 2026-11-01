---
brief: assay:assay:delivery-profiles:04
title: "Connect the real orchestration supervisor to the selected runner"
why: "The delivered journey must exercise the real orchestration owner. A demo-only launch controller would hide the integration work instead of proving it."
wave: 10
depends: ["delivery-profiles/02", "graph-execution/26", "cell-api/02"]
unblocks: ["delivery-profiles/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes composition, process/session containment or how a simulated capability is represented at an execution/acceptance boundary; driver reviews preservation of existing authority and exact-subject scope."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: fe6489b7-5f78-43ab-b996-07fd207357f0
authored: 2026-10-30 by assay-worker
exec-tier: strong
exec-tier-why: Cross-module runtime identity, isolation, lifecycle and qualification claims require consistent boundary reasoning.
domain: complicated
outcome: none
sources:
  - docs/streams/delivery-profiles/brief-03-build-the-isolated-local-profile-lifecycle-in-the-existing-service-host.md
  - docs/streams/delivery-profiles/brief-01-define-the-shared-delivery-profile-contract-and-pure-reader.md
  - docs/streams/delivery-profiles/spec.md
  - spec/milestone-v1.md
  - spec/loop-admin-runner-v1.md
  - docs/library-first.md
  - docs/go-engineering.md
  - "freshness-checked 2026-10-30 at public main 15ab949bd6158c2a9ef469adf362f3fb5d2fca5a"
consumers:
  - "public cell-api service handlers: fixed-here after implementation"
  - "loopadmin/supervisor/: out-of-scope (graph-execution/26 owns real supervision; this bridge consumes its supported public API without rewriting it)"
  - "loopadmin/runner.Client: fixed-here (bridge calls existing public semantics; no runner state machine duplicated)"
  - "public thin cockpit: follow-up delivery-profiles/05"
---

# Brief 04 — Connect the real orchestration supervisor to the selected runner

## Context

files: `tools/desk/internal/cellapi/`, `tools/desk/cellapi/` (planned), `tools/desk/testdata/cellapi/deliveryprofiles/` (planned), `tools/desk/go.mod`, `tools/desk/go.sum`, `tools/desk/internal/cellapi/deliveryprofile/` (planned), `docs/delivery-profiles.md` (planned), `changelog/delivery-profiles-04.md` (planned)

facts:
- Source baseline: public main `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, inspected 30 October 2026; [spec source inventory](spec.md#sources-and-observed-baseline--30-october-2026) distinguishes shipped code from planned owners.
- `loopadmin/runner/fake` and `runner/conformance` exist. `loopadmin/supervisor` is graph-execution/26's unfinished producer. Public `deskd/` is a compatibility source placement; cell-api/01–04 own R7 delivery. Their Task/Verify and completed Evidence are unchanged by this plan.
- Paths and tests introduced here are planned implementation deliverables. At pickup recheck actual published APIs/module imports, source/contract/guideline revisions, claims/overlap and the host/client's delivered paths; record continue/adapt/hold and owner acknowledgement for any changed seam.

layering: R7 service adapter over loopadmin supervisor owns this slice. Pure contract functions accept values; host adapters own transport/files/processes. Reuse existing service/supervisor/runner and preserve independent authority checks. No new service, execution engine, claim store, credential broker or acceptance authority.

design-fit:
  owner: "R7 service adapter over loopadmin supervisor"
  contract: "spec/delivery-profile-v1.md planned by /01; spec/loop-admin-runner-v1.md and spec milestone supported-scope contract remain authoritative."
  retires: ["duplicate selected-journey setup or scope inference replaced in this delivery; enumerate actual replaced sites at pickup"]
  weight: "No new binary, service, framework or durable store; bounded library/host/test changes. Measure actual complexity at implementation, no reduction claimed now."
  why-add: "Existing owners lack this explicit profile composition/journey; another orchestration or mock-service layer would duplicate their semantics."

single-point-of-failure: profile or runner subject misinterpreted by a shared adapter — independent consuming-boundary authorization and exact-subject checks plus planted-fault consumer tests back it; common schema defects still require review against the source contract.

## Read first

- [Controlling draft spec and criterion coverage](spec.md), [plan and real blocking heads](README.md).
- `spec/loop-admin-runner-v1.md`, `spec/milestone-v1.md`, `docs/library-first.md`, `docs/go-engineering.md`.
- `docs/streams/cell-api/README.md`, `docs/streams/graph-execution/README.md`; read the named producer's actual implementation/qualified API before reuse.

Routed consumer obligations: `delivery-profiles/01`, `delivery-profiles/03`. Consume their declared interfaces; these references do not invent additional start dependencies.

## Ground rules

- Parked draft: no dispatch or runtime activation until source approval and stream admission; own isolated branch and draft PR only. No ready flip, merge, tag, pin, cluster/production contact or release.
- Preserve gates, current-subject/actor/fence checks and private custody. Synthetic data only unless a separate bounded local invocation is explicitly authorized for /07.
- Consume shared public APIs in-process; no subprocess RPC to a CLI, copied state machine or imported foreign internal package. Local replace is source qualification only, not published compatibility.
- Never fill Evidence or mark verified/done as implementer. Missing required runtime/database/support is explicit could-not-check, never a passing skip.

## Human decision

Driver reviews this bounded boundary and its inherited authority checks. Recommended:
approve the scoped design; alternatives request corrections or hold. Source approval
alone does not supply a missing per-brief ruling, runtime permission or acceptance.
Default if no answer: none — blocks until answered.

## Task

1. Wait for actual delivered cell-api/02 and graph-execution/26 APIs. Establish module/import and version closure from the real service. Reconcile planned tools/desk/internal/cellapi versus compatibility deskd placement with R7 owner; exact selected paths are recorded before effects. Missing supported supervisor is HOLD, never an invented substitute.
2. Add a thin profile adapter routing launch/observe/cancel/reconcile through the real supervisor and runner.Client to /02 fake.Adapter. Preserve caller identity, existing assignment/fence/dedupe/cancellation rules, operation-scoped authorization and error states. Do not implement workflow progression, grant model qualification or create another journal.
3. Bind profile, scenario, synthetic session and run identity through actual request serialization and handler dispatch. No renderer side execution shortcut. If /26 requires a real disposable PostgreSQL journal, use its existing harness and retain unknown on unavailable database; do not replace that journal with an in-memory fake for a real-supervisor claim.
4. Keep fault controls confined to the explicit disposable adapter; they cannot select a production provider. Exercise unauthorized/wrong session, stale generation, lost acknowledgement, lower-boundary bypass and late-result cases with counted downstream effects. Update service contract/generated clients via their owning generator.

## Deliverables and DoD

The Task's production paths, canonical docs, shared consumer routes and named positive /
negative tests are delivered together. Update owning generated API/docs via that owner's
actual generator (record command at pickup), not hand-edited generated output; the public
Go library docs are source Markdown and need no invented site build. Preserve existing
normal startup/identity behavior. Include the declared per-PR changelog fragment. Review
must examine actual consumer flow and limitations, not only imports or file existence.

## Verify

Every test below is a FUTURE deliverable, not an authoring-time result. Commands run from
public repository root. Each requires a named PASS to reject vacuous selectors. The test
must exercise the production seam and fail under the specified wrong-but-plausible
mutation. Implement executable mutation recipes under the selected module's testdata,
record actual commands/digests and restore each mutation before the next check.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileBridge$' ./internal/cellapi/... > "$out" && grep -F -- '--- PASS: TestProfileBridge ' "$out")` | exit 0 and named PASS. Real service adapter invokes delivered supervisor and runner.Client with fake adapter; launch/observe/cancel/reconcile share exact subject. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileBridgeFence$' ./internal/cellapi/... > "$out" && grep -F -- '--- PASS: TestProfileBridgeFence ' "$out")` | exit 0 and named PASS. Wrong session/generation/profile blocked independently below outer check, no launch; unqualified supervisor or database cannot silently skip. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileUnknown$' ./internal/cellapi/... > "$out" && grep -F -- '--- PASS: TestProfileUnknown ' "$out")` | exit 0 and named PASS. Lost acknowledgement reconciles one launch; deliberate replay-on-unknown and wrong-subject mutations turn test red. |
| 4 | check:ci +dereference | `statusgen --root . --consumers --brief delivery-profiles/04` | pre-merge only; exit 0 at implementation diff; actual edited consumers corroborate routing. Out-of-scope rationale remains a reviewer judgment. |

Coordination: /03 owns lifecycle setup in the shared deliveryprofile package; this brief owns its supervisor bridge. Checkpoint the interface and serialize common-package edits; wave assignment does not authorize concurrent edits there.

## Pre-mortem

A structurally correct profile with a confident false scope is the primary failure.
Verify rows deliberately corrupt that interpretation and exercise the adjacent consumer.
Additional negative controls are the specific mutation in each Expect cell. Review owns
scope adequacy, approved permission and whether the captured UI depicts the promised
operator journey; code/test agreement alone cannot settle those judgments.

## Evidence

<!-- Independent verifier records exact source/artifact/profile/scenario/environment,
commands/exits, named PASS and failing control, coverage/unknowns and evidence location.
No implementation or acceptance evidence exists in this authoring change. -->

## Review

Gate: human. Preserve the controlling risks, exact-subject boundary and independent checks. A draft authoring PR completes no brief and accepts no milestone.
