---
brief: assay:assay:delivery-profiles:07
title: "Qualify a real-runner replacement without inheriting simulated acceptance"
why: "Replacing a fake with a real runner changes what has been demonstrated. A new qualification must expose contract drift and cannot reuse simulated acceptance."
wave: 17
depends: ["delivery-profiles/06", "graph-execution/22"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes composition, process/session containment or how a simulated capability is represented at an execution/acceptance boundary; driver reviews preservation of existing authority and exact-subject scope."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 20d73492-ff4f-4113-b9f7-5d42e404bce3
authored: 2026-10-30 by assay-worker
exec-tier: strong
exec-tier-why: Cross-module runtime identity, isolation, lifecycle and qualification claims require consistent boundary reasoning.
domain: complicated
outcome: none
sources:
  - docs/streams/delivery-profiles/brief-02-qualify-deterministic-runner-scenarios-against-the-shared-contract.md
  - docs/streams/delivery-profiles/spec.md
  - spec/milestone-v1.md
  - spec/loop-admin-runner-v1.md
  - docs/library-first.md
  - docs/go-engineering.md
  - "freshness-checked 2026-10-30 at public main 15ab949bd6158c2a9ef469adf362f3fb5d2fca5a"
consumers:
  - "selected real adapter from graph-execution/22: out-of-scope (producer owns harness integration; profile qualification consumes its existing public factory/API)"
  - "loopadmin/runner/conformance/: fixed-here (qualification invokes existing shared cases; any semantic change requires owner-acknowledged amendment)"
  - "public client/service: fixed-here (real-runner qualification traverses the same public clients and handlers from delivery-profiles/05 and /06)"
  - "milestone-support acceptance: out-of-scope (existing acceptance adapter and driver own acceptance; a real-runner test exports evidence without advancing it)"
---

# Brief 07 — Qualify a real-runner replacement without inheriting simulated acceptance

## Context

files: `tools/desk/internal/cellapi/integration/` (planned), `tools/desk/testdata/cellapi/deliveryprofiles/` (planned), `examples/delivery-profiles/` (planned), `docs/delivery-profiles.md` (planned), `changelog/delivery-profiles-07.md` (planned)

facts:
- Source baseline: public main `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, inspected 30 October 2026; [spec source inventory](spec.md#sources-and-observed-baseline--30-october-2026) distinguishes shipped code from planned owners.
- `loopadmin/runner/fake` and `runner/conformance` exist. `loopadmin/supervisor` is graph-execution/26's unfinished producer. Public `deskd/` is a compatibility source placement; cell-api/01–04 own R7 delivery. Their Task/Verify and completed Evidence are unchanged by this plan.
- Paths and tests introduced here are planned implementation deliverables. At pickup recheck actual published APIs/module imports, source/contract/guideline revisions, claims/overlap and the host/client's delivered paths; record continue/adapt/hold and owner acknowledgement for any changed seam.

layering: existing public integration harness and real runner adapter owns this slice. Pure contract functions accept values; host adapters own transport/files/processes. Reuse existing service/supervisor/runner and preserve independent authority checks. No new service, execution engine, claim store, credential broker or acceptance authority.

design-fit:
  owner: "existing public integration harness and real runner adapter"
  contract: "spec/delivery-profile-v1.md planned by /01; spec/loop-admin-runner-v1.md and spec milestone supported-scope contract remain authoritative."
  retires: ["duplicate selected-journey setup or scope inference replaced in this delivery; enumerate actual replaced sites at pickup"]
  weight: "No new binary, service, framework or durable store; bounded library/host/test changes. Measure actual complexity at implementation, no reduction claimed now."
  why-add: "Existing owners lack this explicit profile composition/journey; another orchestration or mock-service layer would duplicate their semantics."

single-point-of-failure: profile or runner subject misinterpreted by a shared adapter — independent consuming-boundary authorization and exact-subject checks plus planted-fault consumer tests back it; common schema defects still require review against the source contract.

## Read first

- [Controlling draft spec and criterion coverage](spec.md), [plan and real blocking heads](README.md).
- `spec/loop-admin-runner-v1.md`, `spec/milestone-v1.md`, `docs/library-first.md`, `docs/go-engineering.md`.
- `docs/streams/cell-api/README.md`, `docs/streams/graph-execution/README.md`; read the named producer's actual implementation/qualified API before reuse.

Routed consumer obligations: `delivery-profiles/02`. Consume their declared interfaces; these references do not invent additional start dependencies.

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

1. Wait for compatible delivered graph-execution/22 adapter and explicit driver selection of supported disposable local host/invocation scope. Record actual model/harness/artifact, capabilities and required credentials scope; no provider/cluster permission is inferred from profile selection. Unsupported or unavailable real adapter stays unqualified.
2. Run shared /02 contract cases against both fake and selected real adapter at pinned revisions, then rerun the same real client/service/supervisor journey using the actual real adapter. Capability equivalence covers declared lifecycle/interfaces only, never model output quality or unmeasured performance.
3. Use a new profile subject/version as /01 requires, retain fake-profile evidence unchanged, and export fresh criteria and cleanup evidence. Prove fake or stale receipts cannot satisfy real-required acceptance; an explicitly simulated consumer may still use the fake profile.
4. Keep deterministic fake scenarios for regression, document differences/unsupported observations and local rollback to the previous profile selection. No production adoption, tag, pin, workflow, live cluster contact or automatic acceptance. Scope extension beyond approved local invocation is a new driver decision.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileReplacement$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileReplacement ' "$out")` | exit 0 and named PASS. Actual selected real adapter and fake run shared cases; deliberate contract mismatch fails, unavailable real runtime is not skipped. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileRealJourney$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileRealJourney ' "$out")` | exit 0 and named PASS. Real client/host/supervisor/adapter execute the separately authorized local workload and cleanup; exact actual subject is recorded. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileNoInheritance$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileNoInheritance ' "$out")` | exit 0 and named PASS. Fake, stale or wrong-environment evidence cannot satisfy real-required qualification; historical fake result remains readable. |

| 4 | check:ci +dereference | `statusgen --root . --consumers --brief delivery-profiles/07` | exit 0 at implementation diff; actual edited consumers corroborate routing. Out-of-scope rationale remains a reviewer judgment. |

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
