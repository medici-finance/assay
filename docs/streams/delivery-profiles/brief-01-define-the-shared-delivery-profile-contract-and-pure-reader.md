---
brief: assay:assay:delivery-profiles:01
title: "Define the shared delivery-profile contract and pure reader"
why: "A declared fake can otherwise be mistaken for a real capability. One shared composition contract lets tools and evidence agree on exactly what was exercised."
wave: 0
depends: []
unblocks: ["delivery-profiles/02", "delivery-profiles/03"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes composition, process/session containment or how a simulated capability is represented at an execution/acceptance boundary; driver reviews preservation of existing authority and exact-subject scope."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 8f6e5d2e-10ff-4840-b7a2-292721131cd2
authored: 2026-10-30 by assay-worker
exec-tier: strong
exec-tier-why: Cross-module runtime identity, isolation, lifecycle and qualification claims require consistent boundary reasoning.
domain: complicated
outcome: none
sources:
  - docs/streams/delivery-profiles/spec.md
  - spec/milestone-v1.md
  - spec/loop-admin-runner-v1.md
  - docs/library-first.md
  - docs/go-engineering.md
  - "freshness-checked 2026-10-30 at public main 15ab949bd6158c2a9ef469adf362f3fb5d2fca5a"
consumers:
  - "loopadmin/deliveryprofile/: fixed-here after implementation; planned pure owner"
  - "loopadmin/runner/fake/: follow-up delivery-profiles/02"
  - "tools/desk/internal/cellapi/deliveryprofile/: follow-up delivery-profiles/03 and /04"
  - "tools/desk/internal/cellapi/: follow-up delivery-profiles/04 if selected by the delivered R7 owner"
  - "public thin cockpit: follow-up delivery-profiles/05"
  - "milestone acceptance adapters: out-of-scope (milestone-support/06 owns acceptance; profile decoding grants no authority and does not modify that adapter)"
---

# Brief 01 — Define the shared delivery-profile contract and pure reader

## Context

files: `spec/delivery-profile-v1.md` (planned), `schemas/delivery-profile-v1.json` (planned), `loopadmin/deliveryprofile/` (planned), `loopadmin/testdata/deliveryprofiles/` (planned), `loopadmin/go.mod`, `docs/contracts.md`, `docs/library-first.md`, `docs/delivery-profiles.md` (planned), `changelog/delivery-profiles-01.md` (planned)

facts:
- Source baseline: public main `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, inspected 30 October 2026; [spec source inventory](spec.md#sources-and-observed-baseline--30-october-2026) distinguishes shipped code from planned owners.
- `loopadmin/runner/fake` and `runner/conformance` exist. `loopadmin/supervisor` is graph-execution/26's unfinished producer. Public `deskd/` is a compatibility source placement; cell-api/01–04 own R7 delivery. Their Task/Verify and completed Evidence are unchanged by this plan.
- Paths and tests introduced here are planned implementation deliverables. At pickup recheck actual published APIs/module imports, source/contract/guideline revisions, claims/overlap and the host/client's delivered paths; record continue/adapt/hold and owner acknowledgement for any changed seam.

layering: loopadmin/deliveryprofile owns this slice. Pure contract functions accept values; host adapters own transport/files/processes. Reuse existing service/supervisor/runner and preserve independent authority checks. No new service, execution engine, claim store, credential broker or acceptance authority.

design-fit:
  owner: "loopadmin/deliveryprofile"
  contract: "spec/delivery-profile-v1.md planned by /01; spec/loop-admin-runner-v1.md and spec milestone supported-scope contract remain authoritative."
  retires: ["duplicate selected-journey setup or scope inference replaced in this delivery; enumerate actual replaced sites at pickup"]
  weight: "No new binary, service, framework or durable store; bounded library/host/test changes. Measure actual complexity at implementation, no reduction claimed now."
  why-add: "Existing owners lack this explicit profile composition/journey; another orchestration or mock-service layer would duplicate their semantics."

single-point-of-failure: profile or runner subject misinterpreted by a shared adapter — independent consuming-boundary authorization and exact-subject checks plus planted-fault consumer tests back it; common schema defects still require review against the source contract.

## Read first

- [Controlling draft spec and criterion coverage](spec.md), [plan and real blocking heads](README.md).
- `spec/loop-admin-runner-v1.md`, `spec/milestone-v1.md`, `docs/library-first.md`, `docs/go-engineering.md`.
- `docs/streams/cell-api/README.md`, `docs/streams/graph-execution/README.md`; read the named producer's actual implementation/qualified API before reuse.

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

1. Publish the versioned spec/schema and one strict pure Decode/Validate/Canonical/Digest API in the existing loopadmin module. Inputs are supplied bytes/values; no filesystem, environment, clock, process, credential, network or acceptance authority. Do not reuse runner.ExecutionProfile for composition.
2. Validate exact identities, component modes/artifact versions, operation-scoped interface contracts, connection references, deterministic scenarios, supported scope/limitations and lifecycle/evidence references. Reject unknown schema, fields, modes, absent required metadata and incompatible connections. Recipe references select reviewed host code, never arbitrary shell commands.
3. Provide valid/invalid conformance fixtures and external-consumer proof under GOWORK=off, including the actual service consumer module. Preserve module publication gaps: a relative replace or placeholder module version proves source builds only. Update the canonical semantic-owner and documentation rows, not a second registry.
4. Bind evidence to profile, component/scenario/artifact digests and actual environment; preserve absent observations. Define compatibility and new-subject requirements when fake becomes real. No milestone field, lifecycle state or receipt acceptance is invented.

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
| 1 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileDecode$' ./deliveryprofile/... > "$out" && grep -F -- '--- PASS: TestProfileDecode ' "$out")` | exit 0 and named PASS. Strict production decoding rejects an unknown mode and missing component; planted ignore-mode mutation fails. |
| 2 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileContract$' ./deliveryprofile/... > "$out" && grep -F -- '--- PASS: TestProfileContract ' "$out")` | exit 0 and named PASS. Connections and evidence bind one profile/scenario subject; changed digest or execution-profile confusion fails. |
| 3 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileImport$' ./deliveryprofile/... > "$out" && grep -F -- '--- PASS: TestProfileImport ' "$out")` | exit 0 and named PASS. Temporary external consumer plus actual delivered service import use the public package, with dependency closure and no private/internal import. |

| 4 | check:ci +dereference | `statusgen --root . --consumers --brief delivery-profiles/01` | exit 0 at implementation diff; actual edited consumers corroborate routing. Out-of-scope rationale remains a reviewer judgment. |

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
