---
brief: assay:assay:delivery-profiles:02
title: "Qualify deterministic runner scenarios against the shared contract"
why: "A scripted journey needs repeatable failure and recovery behaviour, and a fake that drifts from the runner contract would make an early milestone misleading."
wave: 1
depends: ["delivery-profiles/01"]
unblocks: ["delivery-profiles/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: "Synthetic in-memory runner scenarios carry no operational credentials or authority; preserve existing runner checks."
issues: []
schema: brief-v2
version: 1
id: 74d06a32-8c75-4f97-a9e8-d126475a8ec8
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
  - "loopadmin/runner/conformance/: fixed-here after implementation"
  - "loopadmin/runner/fake/: fixed-here after implementation"
  - "real adapter qualification: follow-up delivery-profiles/07"
  - "service bridge: follow-up delivery-profiles/04"
---

# Brief 02 — Qualify deterministic runner scenarios against the shared contract

## Context

files: `loopadmin/runner/fake/`, `loopadmin/runner/conformance/`, `loopadmin/testdata/deliveryprofiles/` (planned), `loopadmin/README.md`, `changelog/delivery-profiles-02.md` (planned)

facts:
- Source baseline: public main `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, inspected 30 October 2026; [spec source inventory](spec.md#sources-and-observed-baseline--30-october-2026) distinguishes shipped code from planned owners.
- `loopadmin/runner/fake` and `runner/conformance` exist. `loopadmin/supervisor` is graph-execution/26's unfinished producer. Public `deskd/` is a compatibility source placement; cell-api/01–04 own R7 delivery. Their Task/Verify and completed Evidence are unchanged by this plan.
- Paths and tests introduced here are planned implementation deliverables. At pickup recheck actual published APIs/module imports, source/contract/guideline revisions, claims/overlap and the host/client's delivered paths; record continue/adapt/hold and owner acknowledgement for any changed seam.

layering: loopadmin/runner/fake and runner/conformance owns this slice. Pure contract functions accept values; host adapters own transport/files/processes. Reuse existing service/supervisor/runner and preserve independent authority checks. No new service, execution engine, claim store, credential broker or acceptance authority.

design-fit:
  owner: "loopadmin/runner/fake and runner/conformance"
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

## Task

1. Extend existing fake.Adapter and the shared conformance Control/Factory seams, not a second runner state machine. Inventory existing controls before adding any. Define scriptable deterministic success, definite rejection, lost launch acknowledgement, delayed observation, cancel-request/confirmed-stop, ignored cancellation and reset.
2. Use seed/logical steps and stable scenario identities; permit bounded nondeterministic observation timestamps only when separately declared and excluded from deterministic comparison. Reset isolates old run IDs/results. Report synthetic usage explicitly or unknown; never emit fabricated measured provider spend.
3. Run the same runner protocol/lifecycle cases over the fake and a factory for compatible real adapters. Share boundary assertions, retain independent expected outcomes and planted bad adapters. A passing fake does not prove an unavailable real adapter; record that gap.
4. Prove no provider contact/process job, unknown-launch reconciliation without blind replay, terminal-state and claim-generation preservation, and cancel request never treated as confirmation. Document limits, including no model quality, cluster or durable workflow evidence.

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
| 1 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileScenarios$' ./runner/fake/... > "$out" && grep -F -- '--- PASS: TestProfileScenarios ' "$out")` | exit 0 and named PASS. Repeated seed/reset gives identical logical trace; all named positive/fault scenarios are exercised. |
| 2 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileRunnerLaw$' ./runner/conformance/... > "$out" && grep -F -- '--- PASS: TestProfileRunnerLaw ' "$out")` | exit 0 and named PASS. Production runner.Client traverses fake.Adapter with independent expected results; planted duplicate launch and cancel-as-stop fail. |
| 3 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProfileReset$' ./runner/fake/... > "$out" && grep -F -- '--- PASS: TestProfileReset ' "$out")` | exit 0 and named PASS. Old run/event replay after reset is refused; zero provider calls and job processes are counted. |
| 4 | check:ci +dereference | `statusgen --root . --consumers --brief delivery-profiles/02` | pre-merge only; exit 0 at implementation diff; actual edited consumers corroborate routing. Out-of-scope rationale remains a reviewer judgment. |

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

Gate: model. Preserve the controlling risks, exact-subject boundary and independent checks. A draft authoring PR completes no brief and accepts no milestone.
