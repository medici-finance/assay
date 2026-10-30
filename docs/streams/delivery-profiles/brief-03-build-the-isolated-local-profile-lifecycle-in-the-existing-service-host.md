---
brief: assay:assay:delivery-profiles:03
title: "Build the isolated local profile lifecycle in the existing service host"
why: "An early delivery must start and clean up predictably without reading operational credentials or leaving processes behind."
wave: 14
depends: ["delivery-profiles/01", "cell-api/02", "graph-execution/26", "cell-api/03"]
unblocks: ["delivery-profiles/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes composition, process/session containment or how a simulated capability is represented at an execution/acceptance boundary; driver reviews preservation of existing authority and exact-subject scope."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 2c185ceb-f871-4075-ad3f-329634a9a759
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
  - "tools/desk/cmd/deskd/: fixed-here after implementation"
  - "tools/desk/internal/cellapi/deliveryprofile/: fixed-here after implementation"
  - "service bridge: follow-up delivery-profiles/04"
  - "qualification runner: follow-up delivery-profiles/06"
  - "cell-api/02 service placement: out-of-scope (producer retains migration ownership; checkpoint actual host paths before adding lifecycle glue)"
---

# Brief 03 — Build the isolated local profile lifecycle in the existing service host

## Context

files: `tools/desk/internal/cellapi/deliveryprofile/` (planned), `tools/desk/cmd/deskd/` (planned), `tools/desk/testdata/cellapi/deliveryprofiles/` (planned), `tools/desk/go.mod`, `tools/desk/go.sum`, `docs/delivery-profiles.md` (planned), `changelog/delivery-profiles-03.md` (planned)

facts:
- Source baseline: public main `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, inspected 30 October 2026; [spec source inventory](spec.md#sources-and-observed-baseline--30-october-2026) distinguishes shipped code from planned owners.
- `loopadmin/runner/fake` and `runner/conformance` exist. `loopadmin/supervisor` is graph-execution/26's unfinished producer. Public `deskd/` is a compatibility source placement; cell-api/01–04 own R7 delivery. Their Task/Verify and completed Evidence are unchanged by this plan.
- Paths and tests introduced here are planned implementation deliverables. At pickup recheck actual published APIs/module imports, source/contract/guideline revisions, claims/overlap and the host/client's delivered paths; record continue/adapt/hold and owner acknowledgement for any changed seam.

layering: existing public service profile host owns this slice. Pure contract functions accept values; host adapters own transport/files/processes. Reuse existing service/supervisor/runner and preserve independent authority checks. No new service, execution engine, claim store, credential broker or acceptance authority.

design-fit:
  owner: "existing public service profile host"
  contract: "spec/delivery-profile-v1.md planned by /01; spec/loop-admin-runner-v1.md and spec milestone supported-scope contract remain authoritative."
  retires: ["duplicate selected-journey setup or scope inference replaced in this delivery; enumerate actual replaced sites at pickup"]
  weight: "No new binary, service, framework or durable store; bounded library/host/test changes. Measure actual complexity at implementation, no reduction claimed now."
  why-add: "Existing owners lack this explicit profile composition/journey; another orchestration or mock-service layer would duplicate their semantics."

single-point-of-failure: the profile host misinterprets a synthetic principal or run subject. The cell-api/03 issuer and ingress admission bind the scoped principal; independent lower store, claim and effect admission reject non-profile subjects even when ingress is bypassed (Verify rows 5–6). Planted-fault consumer tests exercise those separate boundaries; common schema defects still require review against the source contract.

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

1. Add an explicit disposable profile lifecycle to the existing public service host. Reuse /01 validation before effects. Wait for delivered cell-api/02, cell-api/03 and graph-execution/26. Extend the R7-selected service host and delegate child lifetime to the public process/supervisor owner. The current standalone deskd module is compatibility evidence only; do not add another profile host there or copy it across module boundaries.
2. Obtain synthetic caller credentials only from the existing issuer/admission owner selected and approved by cell-api/03; record issuer/configuration identity and bind profile-only audience, exact run namespace and disposable principal. The profile host issues/brokers no token. If unsupported, hold for that owner's scoped amendment. Refuse profile routes and these credentials in ordinary mode; independently refuse a synthetic principal reading/acting on non-profile cells, claims or stores below ingress. Implement start, reset, status and idempotent teardown with owned temporary root, scoped synthetic session, ephemeral loopback listeners, reviewed recipe references and allowlisted child environment. Never select a provider or arbitrary executable from user-controlled profile text; ordinary startup retains existing semantics.
3. Reject external targets, ambient provider credentials, unowned/symlink-escaped roots and arbitrary delete/kill targets. Actual process/transport boundaries independently enforce policy with upper validation bypassed. Print selected profile/modes/targets before any contact.
4. Record partial startup and cleanup failures; cleanup only resources this run owns. Interrupt/restart finds its own residue without reviving a stale subject. Read-only evidence retention contains no credential. Do not introduce a durable coordination store or a synthetic production claim.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileLifecycle$' ./internal/cellapi/deliveryprofile/... > "$out" && grep -F -- '--- PASS: TestProfileLifecycle ' "$out")` | exit 0 and named PASS. Real host lifecycle starts/resets/stops disposable fixtures; partial-start and interrupt cases leave no owned running resource. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileContain$' ./internal/cellapi/deliveryprofile/... > "$out" && grep -F -- '--- PASS: TestProfileContain ' "$out")` | exit 0 and named PASS. With outer validator bypassed, remote dial/ambient credential attempt is refused at actual I/O boundary; sentinel files survive. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileHost$' ./cmd/deskd/... > "$out" && grep -F -- '--- PASS: TestProfileHost ' "$out")` | exit 0 and named PASS. Actual host entrypoint requires opt-in; ordinary startup contract unchanged; repeated teardown and unowned-process negative controls pass. |
| 4 | check:ci +dereference | `statusgen --root . --consumers --brief delivery-profiles/03` | pre-merge only; exit 0 at implementation diff; actual edited consumers corroborate routing. Out-of-scope rationale remains a reviewer judgment. |
| 5 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileOrdinaryDeny$' ./internal/cellapi/... > "$out" && grep -F -- '--- PASS: TestProfileOrdinaryDeny ' "$out")` | exit 0 and named PASS. Ordinary-mode production mux refuses profile routes and profile-audience credential, with a positive opt-in-profile control; planted ordinary-mode acceptance fails. |
| 6 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfilePrincipalFence$' ./internal/cellapi/... > "$out" && grep -F -- '--- PASS: TestProfilePrincipalFence ' "$out")` | exit 0 and named PASS. Bypass ingress and directly exercise lower store/claim/effect admission: synthetic principal cannot read or mutate non-profile cells/claims/stores; counters show zero access and scoped disposable positive control succeeds. |

Coordination: /03 owns `tools/desk/internal/cellapi/deliveryprofile/` lifecycle setup; /04 owns the host-to-supervisor bridge. Checkpoint the common package/interface with the other worker before editing; do not dispatch both into shared package changes without an agreed serialization.

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
