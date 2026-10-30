---
brief: assay:assay:delivery-profiles:06
title: "Qualify the complete fake-runner journey and export scoped evidence"
why: "Component tests cannot establish that an operator can complete the journey or that its evidence reports the simulated boundary honestly."
wave: 16
depends: ["delivery-profiles/05"]
unblocks: ["delivery-profiles/07"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes composition, process/session containment or how a simulated capability is represented at an execution/acceptance boundary; driver reviews preservation of existing authority and exact-subject scope."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 938b0ff1-330f-45ae-b634-ed36a047e927
authored: 2026-10-30 by assay-worker
exec-tier: strong
exec-tier-why: Cross-module runtime identity, isolation, lifecycle and qualification claims require consistent boundary reasoning.
domain: complicated
outcome: none
sources:
  - docs/streams/delivery-profiles/brief-03-build-the-isolated-local-profile-lifecycle-in-the-existing-service-host.md
  - docs/streams/delivery-profiles/spec.md
  - spec/milestone-v1.md
  - spec/loop-admin-runner-v1.md
  - docs/library-first.md
  - docs/go-engineering.md
  - "freshness-checked 2026-10-30 at public main 15ab949bd6158c2a9ef469adf362f3fb5d2fca5a"
consumers:
  - "public thin cockpit and host: fixed-here after implementation (qualification)"
  - "delivery-profile evidence decoder: fixed-here (export invokes the shared delivery-profiles/01 decoder; no copied parser)"
  - "milestone-support acceptance/report adapters: out-of-scope (their existing briefs own reading scoped evidence and acceptance; this delivery exports evidence only)"
---

# Brief 06 — Qualify the complete fake-runner journey and export scoped evidence

## Context

files: `tools/desk/internal/cellapi/integration/` (planned), `tools/desk/testdata/cellapi/deliveryprofiles/` (planned), `deskd/internal/deliveryprofile/` (planned), `docs/delivery-profiles.md` (planned), `examples/delivery-profiles/` (planned), `changelog/delivery-profiles-06.md` (planned)

facts:
- Source baseline: public main `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, inspected 30 October 2026; [spec source inventory](spec.md#sources-and-observed-baseline--30-october-2026) distinguishes shipped code from planned owners.
- `loopadmin/runner/fake` and `runner/conformance` exist. `loopadmin/supervisor` is graph-execution/26's unfinished producer. Public `deskd/` is a compatibility source placement; cell-api/01–04 own R7 delivery. Their Task/Verify and completed Evidence are unchanged by this plan.
- Paths and tests introduced here are planned implementation deliverables. At pickup recheck actual published APIs/module imports, source/contract/guideline revisions, claims/overlap and the host/client's delivered paths; record continue/adapt/hold and owner acknowledgement for any changed seam.

layering: existing cell-api integration qualification owns this slice. Pure contract functions accept values; host adapters own transport/files/processes. Reuse existing service/supervisor/runner and preserve independent authority checks. No new service, execution engine, claim store, credential broker or acceptance authority.

design-fit:
  owner: "existing cell-api integration qualification"
  contract: "spec/delivery-profile-v1.md planned by /01; spec/loop-admin-runner-v1.md and spec milestone supported-scope contract remain authoritative."
  retires: ["duplicate selected-journey setup or scope inference replaced in this delivery; enumerate actual replaced sites at pickup"]
  weight: "No new binary, service, framework or durable store; bounded library/host/test changes. Measure actual complexity at implementation, no reduction claimed now."
  why-add: "Existing owners lack this explicit profile composition/journey; another orchestration or mock-service layer would duplicate their semantics."

single-point-of-failure: profile or runner subject misinterpreted by a shared adapter — independent consuming-boundary authorization and exact-subject checks plus planted-fault consumer tests back it; common schema defects still require review against the source contract.

## Read first

- [Controlling draft spec and criterion coverage](spec.md), [plan and real blocking heads](README.md).
- `spec/loop-admin-runner-v1.md`, `spec/milestone-v1.md`, `docs/library-first.md`, `docs/go-engineering.md`.
- `docs/streams/cell-api/README.md`, `docs/streams/graph-execution/README.md`; read the named producer's actual implementation/qualified API before reuse.

Routed consumer obligations: `delivery-profiles/03`. Consume their declared interfaces; these references do not invent additional start dependencies.

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

1. Provide one documented runnable journey using built public client, host, real supervisor, fake runner and required real disposable stores. Reuse existing integration/test harnesses. Start from clean owned environment, run all /spec C01–05/C07 cases, reset and teardown; missing component/db/runtime is a failing or could-not-check run, never a skip or fake fallback.
2. Export a bounded scoped evidence bundle through /01 canonical types: revisions/profile/scenario digests, component modes, actual exercised edges, observed trace/results, OS/runtime, verifier/time, cleanup state, unknowns and retrievable artifact references. Redact credentials and synthetic secrets. Digest equality is subject binding, not authority.
3. Exercise complete success, failure, lost ack/reconcile, delayed observation, cancel request/confirmation and refusal; introduce wrong subject/profile, UI-only completed result and missing fake label mutations. Require independently expected traces, not events used as their own oracle.
4. Document setup/cleanup/recovery and operating limits, capture the actual thin UI interaction/render, and enumerate criterion coverage. Export does not set accepted/ready or replace verifyrun. Independent non-implementer runs Verify after merge; driver supplies separate scoped milestone acceptance.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileJourney$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileJourney ' "$out")` | exit 0 and named PASS. Actual complete operator journey and fault matrix at exact profile subject; each hop observed; planted UI-only success and fake-labelled-real fail. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileEvidence$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileEvidence ' "$out")` | exit 0 and named PASS. Production export decoded through /01 preserves mode/scope/unknowns, rejects stale/wrong subject and redacts synthetic secret sentinel. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileJourneyCleanup$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileJourneyCleanup ' "$out")` | exit 0 and named PASS. Partial startup, interrupt, repeat reset/teardown leave no owned resource; unrelated files/processes survive; failed cleanup stays visible. |

| 4 | check:ci +dereference | `statusgen --root . --consumers --brief delivery-profiles/06` | exit 0 at implementation diff; actual edited consumers corroborate routing. Out-of-scope rationale remains a reviewer judgment. |

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
