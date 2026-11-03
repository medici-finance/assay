---
brief: assay:assay:delivery-profiles:05
title: "Expose the real profile journey and simulation scope in the cockpit"
why: "Operators need a working integrated journey and a clear statement that the runner is simulated, so an attractive demo cannot be mistaken for a deployed capability."
wave: 15
depends: ["delivery-profiles/03", "delivery-profiles/04", "cell-api/03", "cell-api/04"]
unblocks: ["delivery-profiles/06"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes composition, process/session containment or how a simulated capability is represented at an execution/acceptance boundary; driver reviews preservation of existing authority and exact-subject scope."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 0550105c-e0b3-44ed-84dd-1db0935165d3
authored: 2026-10-30 by assay-worker
exec-tier: strong
exec-tier-why: Cross-module runtime identity, isolation, lifecycle and qualification claims require consistent boundary reasoning.
domain: complicated
outcome: none
sources:
  - docs/streams/delivery-profiles/brief-01-define-the-shared-delivery-profile-contract-and-pure-reader.md
  - docs/streams/delivery-profiles/spec.md
  - spec/milestone-v1.md
  - spec/loop-admin-runner-v1.md
  - docs/library-first.md
  - docs/go-engineering.md
  - "freshness-checked 2026-10-30 at public main 15ab949bd6158c2a9ef469adf362f3fb5d2fca5a"
consumers:
  - "public thin cockpit from cell-api/04: fixed-here after implementation"
  - "public typed service client: fixed-here after implementation"
  - "evidence export: follow-up delivery-profiles/06"
  - "desktop Electron migration: out-of-scope (first profile uses cell-api/04 public thin cockpit; desktop packaging is a separate product migration)"
---

# Brief 05 — Expose the real profile journey and simulation scope in the cockpit

## Context

files: `examples/cell-api-client/` (planned), `tools/desk/cellapi/` (planned), `tools/desk/internal/cellapi/integration/` (planned), `tools/desk/testdata/cellapi/deliveryprofiles/` (planned), `docs/cell-api-clients.md` (planned), `docs/delivery-profiles.md` (planned), `changelog/delivery-profiles-05.md` (planned)

facts:
- Source baseline: public main `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, inspected 30 October 2026; [spec source inventory](spec.md#sources-and-observed-baseline--30-october-2026) distinguishes shipped code from planned owners.
- `loopadmin/runner/fake` and `runner/conformance` exist. `loopadmin/supervisor` is graph-execution/26's unfinished producer. Public `deskd/` is a compatibility source placement; cell-api/01–04 own R7 delivery. Their Task/Verify and completed Evidence are unchanged by this plan.
- Paths and tests introduced here are planned implementation deliverables. At pickup recheck actual published APIs/module imports, source/contract/guideline revisions, claims/overlap and the host/client's delivered paths; record continue/adapt/hold and owner acknowledgement for any changed seam.

layering: cell-api public thin cockpit owns this slice. Pure contract functions accept values; host adapters own transport/files/processes. Reuse existing service/supervisor/runner and preserve independent authority checks. No new service, execution engine, claim store, credential broker or acceptance authority.

design-fit:
  owner: "cell-api public thin cockpit"
  contract: "spec/delivery-profile-v1.md planned by /01; spec/loop-admin-runner-v1.md and spec milestone supported-scope contract remain authoritative."
  retires: ["duplicate selected-journey setup or scope inference replaced in this delivery; enumerate actual replaced sites at pickup"]
  weight: "No new binary, service, framework or durable store; bounded library/host/test changes. Measure actual complexity at implementation, no reduction claimed now."
  why-add: "Existing owners lack this explicit profile composition/journey; another orchestration or mock-service layer would duplicate their semantics."

single-point-of-failure: profile or runner subject misinterpreted by a shared adapter — independent consuming-boundary authorization and exact-subject checks plus planted-fault consumer tests back it; common schema defects still require review against the source contract.

## Read first

- [Controlling draft spec and criterion coverage](spec.md), [plan and real blocking heads](README.md).
- `spec/loop-admin-runner-v1.md`, `spec/milestone-v1.md`, `docs/library-first.md`, `docs/go-engineering.md`.
- `docs/streams/cell-api/README.md`, `docs/streams/graph-execution/README.md`; read the named producer's actual implementation/qualified API before reuse.

Routed consumer obligations: `delivery-profiles/01`. Consume their declared interfaces; these references do not invent additional start dependencies.

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

1. Consume the actual public thin cockpit from cell-api/04 and mounted authorization from /03. Reconcile its selected UI source path with examples/cell-api-client at pickup; if /04 supplies no runnable operator surface, hold and get its owner-scoped amendment rather than relabel a generated declaration as a cockpit.
2. Expose profile selection, launch, observe, cancel and reconcile through the existing generated/typed client and real /04 host bridge. Profile metadata comes from the bound backend subject. Missing/unknown metadata disables effectful profile actions and shows unqualified scope; it never defaults to real.
3. Provide a persistent plain-language simulated-runner indicator, active scenario, limitations and run outcome. Distinguish cancellation requested, confirmed stop, definite failure, unknown and scripted completion. Changing profile/reset clears stale observations; no UI-only completion fixture supplies acceptance.
4. Preserve ordinary client auth/privacy behaviour. Regenerate the owning API client from its supported contract generator and record the exact command from cell-api/04. Update user docs for the explicit disposable flow; no product-wide debug panel or desktop rehome.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileClient$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileClient ' "$out")` | exit 0 and named PASS. Actual public thin client -> mounted service -> real supervisor -> fake runner; counters and trace identify each hop, no synthetic front-end shortcut. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileDisplay$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileDisplay ' "$out")` | exit 0 and named PASS. Rendered scope and actions reflect backend profile; missing metadata, swapped fake-as-real and stale reset each fail independent controls. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileClientAuth$' ./internal/cellapi/integration/... > "$out" && grep -F -- '--- PASS: TestProfileClientAuth ' "$out")` | exit 0 and named PASS. Unauthorized/wrong-cell request never reaches runner, even with UI gating bypassed. Supported UI interaction test and captured render accompany the Go flow. |
| 4 | check:ci +dereference | `statusgen --root . --consumers --brief delivery-profiles/05` | pre-merge only; exit 0 at implementation diff; actual edited consumers corroborate routing. Out-of-scope rationale remains a reviewer judgment. |

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
