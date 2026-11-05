---
brief: assay:assay:graph-execution:43
title: Expose caller-scoped launch control and qualify the shared journey
why: "A bounded authenticated entrypoint makes the shared supervisor usable while proving that callers cannot administer profiles, change roles or bypass shared limits."
wave: 7
depends: ["graph-execution/42"]
unblocks: []
split-from: graph-execution/26
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Adds the authenticated runtime launch endpoint. Human acceptance confirms caller isolation, fixed profile/role/limit binding, no operator administration or effect credentials, and exact joined-subject evidence before any activation."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 366cd30e-3d42-443f-92df-8afef2988770
authored: 2026-11-05 by statusgen newbrief
sources:
- docs/streams/graph-execution/brief-26-shared-loop-admin.md
- docs/streams/graph-execution/task-workflow-program.md
- docs/proposals/non-ci-architecture/20-amendment-manifest.md
- docs/go-engineering.md
- "freshness-checked 2026-11-05 @ 7f61cff94b188d46b4e38e6259798d85bb5d4510"
- "PR #4157: delivered current-owner startup preparation; reused, not reimplemented"
exec-tier: strong
exec-tier-why: "Cross-module custody, durable failure recovery and independent lower-boundary authority checks require design judgment."
domain: complicated
outcome: none
consumers:
- "loopadmin/control/: follow-up graph-execution/43 (this child; only implementation may change to fixed-here)"
- "loopadmin/cmd/loop-admin/: follow-up graph-execution/43 (this child; only implementation may change to fixed-here)"
- "loopadmin/go.mod: follow-up graph-execution/43 (this child; only implementation may change to fixed-here)"
- "loopadmin/go.sum: follow-up graph-execution/43 (this child; only implementation may change to fixed-here)"
- "loopadmin/README.md: follow-up graph-execution/43 (this child; only implementation may change to fixed-here)"
- "docs/loop-admin.md: follow-up graph-execution/43 (this child; only implementation may change to fixed-here)"
- "ci/staged-workflows/loopadmin.yml: follow-up graph-execution/43 (this child; implementation corroborates the actual path)"
- "tools/desk/internal/deskkit/citrigger_test.go: follow-up graph-execution/43 (this child; implementation corroborates the actual path)"
---

# Brief 43 — Expose caller-scoped launch control and qualify the shared journey

## Context

files:
- `loopadmin/control/`
- `loopadmin/cmd/loop-admin/`
- `loopadmin/testdata/supervisor/`
- `loopadmin/go.mod`
- `loopadmin/go.sum`
- `loopadmin/README.md`
- `docs/loop-admin.md` (planned)
- `changelog/graph-execution-43-control.md` (planned)
- `ci/staged-workflows/loopadmin.yml`
- `tools/desk/internal/deskkit/citrigger_test.go`

facts: No control endpoint, loop-admin command or active dedicated loopadmin workflow exists at pickup. The runner module has a staged workflow. New command entrypoints must use the shared Cobra/Viper foundation; a standalone module cannot import tools/desk/internal/cli. Catalogue selection of Cobra/Viper does not admit an exact dependency version. /22 owns real provider adapters and /27 owns standing-role migration; /21 owns workflow scheduling. This brief activates none of them.

layering: A thin bounded authenticated control adapter calls the single supervisor with a caller capability and operator-approved configuration. The caller-facing DTO/SDK closure contains no signer, credential minting, profile/install administration or arbitrary argv. Keep independently enforced lower-boundary profile/role/generation/capacity checks even when the endpoint authenticates the request.

design-fit:
- owner: loopadmin/control for caller-scoped transport; supervisor for runtime lifecycle; operator configuration remains outside runtime authority
- contract: spec/loop-admin-runner-v1.md and spec/loop-admin-execution-profile-v1.md; preserve canonical identity and unknown-outcome meanings
- retires: no earlier delivered control; this is the remaining parent scope
- weight: no new command/flag/refusal counts are ratified by authoring; the implementation must measure its exact delta, keep additions in the canonical owner and explain each positive dimension. No new scheduler, work store, alternate engine or credential broker is permitted.
- why-add: A bounded authenticated entrypoint makes the shared supervisor usable while proving that callers cannot administer profiles, change roles or bypass shared limits.

single-point-of-failure: the journal-admitted invocation boundary; protected operator incarnation and the adapter/caller authority fence remain independent and are exercised with direct bypass fixtures.

## Read first

- [Parent contract](brief-26-shared-loop-admin.md), version 10, including all amendments and its unchanged acceptance table.
- [Task workflow program](task-workflow-program.md), [structured input](work-input-amendment.md) and [stream routing](README.md).
- [Library ownership](../../library-first.md), [Go engineering](../../go-engineering.md), [semantic owners and controls](../../contracts.md).
- [R2 target and amendment ownership](../../proposals/non-ci-architecture/20-amendment-manifest.md), [operator incarnation](../../../spec/operator-incarnation-v1.md).
- [Implementation pickup](../../../plugins/assay/references/implementation-pickup.md); re-establish current source and active claims before effects.

## Ground rules

- This is a proposed split of graph-execution/26. Its parent remains the complete acceptance contract; this authoring does not deliver it or change its prerequisites, gate, Task, Verify or Evidence.
- Work offline in an owned branch. No operational database, provider/model call, image launch, service activation, live desk migration, release, workflow dispatch, merge or ready flip.
- Real PostgreSQL checks use only an explicitly owned disposable fixture. Missing fixture configuration fails its row; no self-skip, ambient database discovery or local embedded substitute.
- Preserve one canonical work identity and claim authority. No effect credential or operator administration in launch code. No credentials in packets, errors, artifacts or logs.
- Retain every existing independent guard and mutation. A control relocation must carry its consumer flow and negative control; any security weakening stops for the human.
- Stop at implemented behind a draft PR; independent verification and the human gate own acceptance. Record before/after weight and fail-first evidence as required by the worker kit.

## Task

1. Admit exact required Cobra/Viper versions through risk records before manifests. Implement the command on that foundation, with explicit typed configuration and validated source precedence. Default execution uses offline fixtures; live activation is a separate trusted operator act. No custom flag parser, ambient profile search or secret lookup in the caller SDK.
2. Expose bounded authenticated start/observe/cancel/reconcile/status and receipt access scoped to the capability's caller namespace and immutable operator-approved role/profile/generation/limits. Validate assignment/reservation references and reject caller-supplied argv, arbitrary role/profile overrides, limit raises, recursive unconstrained creation, unknown mandatory fields and unrecognized callers. Enforce body/result bounds and cancellation without exposing credential material.
3. Independently refuse a direct supervisor/adapter bypass that changes bindings, reuses another namespace receipt or exceeds journal-shared capacity. A runtime client can never reach profile installation, credential administration, key rotation or operational backup/restore; inference credentials, if later qualified, stay inside their approved adapter boundary and never in status/packets/artifacts/logs. This source contribution uses fake providers only.
4. Wire sanitized status with mode/caller/optional workflow references; viewer attach/detach never starts or resumes. Document process-vs-work ownership, passive journal backup/restore, fail-closed configuration and unknown launch/cancel recovery, fixture commands, manual approvals and unsupported capabilities. No production availability claim from a fake journey.
5. Qualify the joined actual CLI/control → supervisor → PostgreSQL journal → fake adapter journey in standing and workflow modes, with workflow storage absent in standing composition. Re-run all thirteen unchanged parent acceptance rows at the integrated exact source, carrying /40 startup/telemetry controls and /41 real database failure controls. The final acceptance map owns rerunning, not reimplementing, earlier child behavior. A child component pass cannot stand in for parent integrated acceptance.
6. Prepare the dedicated module CI and all cross-module trigger changes using existing job/fixture owners. Preserve the staged-to-live human workflow promotion gate, no dispatch/release or workflow-token scope raise. The dedicated workflow must execute the actual row selectors and mutation maps against an explicit disposable PostgreSQL fixture; until promoted, report that coverage as could-not-check. Publish one scrubbed joined evidence bundle and human gate review candidate; /26 remains unimplemented until full scope is delivered and independent acceptance advances it.

## Interface contract

A thin bounded authenticated control adapter calls the single supervisor with a caller capability and operator-approved configuration. The caller-facing DTO/SDK closure contains no signer, credential minting, profile/install administration or arbitrary argv. Keep independently enforced lower-boundary profile/role/generation/capacity checks even when the endpoint authenticates the request. Caller identity, invocation and current generations remain attached to every typed state, receipt and evidence item. Unknown stays unknown; process success is never work acceptance.

## Verify

The parent-row number in Expect identifies the unchanged obligation this child implements. These tests are prospective deliverables, not observed results. Preserve the original named selectors even where their existing contract names exceed the worker naming preference. Use targeted bounded package runs in the worker; CI owns the full matrix.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLoopAdminCallerIsolationAndDirectBypass$' ./control/... > "$o" && grep -F -- '--- PASS: TestLoopAdminCallerIsolationAndDirectBypass ' "$o")` | Parent row 3: exit 0; named PASS. Authenticated endpoint and independently invoked lower boundary refuse role/profile changes, cross-caller receipt access and shared-capacity overflow; no operator action is reachable. Namespace-lookup bypass fails. |
| 2 | check +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLoopControlJoined$' ./control/... > "$o" && grep -F -- '--- PASS: TestLoopControlJoined ' "$o")` | Exit 0; actual CLI/control, production supervisor and real disposable journal reach one fake provider invocation in either mode. Auth/body-limit refusal, missing current binding, viewer attach/detach and passive restore start no child. Planted endpoint bypass and model-on-attach mutations fail. |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminBothModesNoGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminBothModesNoGraph " "$routing_out")` | Integrated rerun of parent row 1: exit 0; named PASS; both caller modes launch via one fake adapter; standing mode starts with the instance store absent; mutation: import the workflow module from the supervisor — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminUnknownRestartAndAccounting$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminUnknownRestartAndAccounting " "$routing_out")` | Integrated rerun of parent row 2: exit 0; named PASS; crash between caller intent and launch receipt reconciles once; unknown cancel blocks relaunch; usage and limits survive restart; mutation: relaunch on restart when the launch receipt is missing — the named test must fail |
| 5 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminCallerIsolationAndDirectBypass$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminCallerIsolationAndDirectBypass " "$routing_out")` | Integrated rerun of parent row 3: exit 0; named PASS; a direct lower-boundary call cannot change role/profile, reuse another caller receipt or exceed shared capacity; operator actions are unreachable; mutation: skip the caller-namespace check on receipt lookup — the named test must fail |
| 6 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestProcessLaunchAuditCoversLoopAdmin$" ./... > "$routing_out" && grep -q -- "--- PASS: TestProcessLaunchAuditCoversLoopAdmin " "$routing_out")` | Integrated rerun of parent row 4: exit 0; named PASS; the process-launch audit scans `loopadmin/` as well as `tools/desk`, finds the moved runner's exec site in its register, and fails on an unregistered exec site planted in a `loopadmin/` fixture; mutation: drop the `loopadmin/` root from the audit's scan — the named test must fail |
| 7 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminFailClosed$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminFailClosed " "$routing_out")` | Integrated rerun of parent row 5: exit 0; named PASS; an unrecognised caller, an unreadable profile or role-binding configuration and an absent budget each refuse the launch and start no child; mutation: treat an absent budget as unlimited — the named test must fail |
| 8 | check:ci +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestJournalSharedStoreRestart$" ./... > "$routing_out" && grep -q -- "--- PASS: TestJournalSharedStoreRestart " "$routing_out")` | Integrated rerun of parent row 6: exit 0; named PASS against a real disposable PostgreSQL; launch intent and receipt commit in one transaction before the child starts, a restart with the database intact reconciles once, and after the journal database is dropped an unknown launch holds instead of relaunching; mutation: relaunch a child whose launch intent row is missing after database loss — the named test must fail |
| 9 | check:ci +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestJournalIncarnationPaused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestJournalIncarnationPaused " "$routing_out")` | Integrated rerun of parent row 7: exit 0; named PASS; a journal restored from backup under a mismatched or missing operator incarnation starts paused and launches no child until rotation and confirmed group stop; mutation: resume launches when an activation record is present but its incarnation mismatches — the named test must fail |
| 10 | check:ci +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestNoEmbeddedJournalEngine$" ./... > "$routing_out" && grep -q -- "--- PASS: TestNoEmbeddedJournalEngine " "$routing_out")` | Integrated rerun of parent row 8: exit 0; named PASS; no loopadmin package imports a SQLite or other embedded database driver or the `workflow` module; mutation: add an embedded-driver import under `loopadmin/journal` — the named test must fail |
| 11 | check:ci +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminLaunchBudgetBrake$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminLaunchBudgetBrake " "$routing_out")` | Integrated rerun of parent row 9: exit 0; named PASS against a real disposable PostgreSQL journal; with the child-launch usage unreadable the launch is refused and no child starts; repeated supervisor restarts within one window cannot launch beyond the configured budget or regain consumed allowance; a seeded pacing estimate alone does not refuse an authorized launch; mutation: treat an unavailable child-launch count as under budget — the named test must fail |
| 12 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellPrepBounded$' ./... > "$o" && grep -F -- '--- PASS: TestCellPrepBounded ' "$o")` | Integrated rerun of parent row 10: exit 0; named PASS. Production up/direct/cadence fixtures satisfy item 5 of §Cell preparation and startup acceptance at 1x/10x history and with maintenance held mid-scan; zero recursive walks in admission and on finish/`down`, and one compatible shared preparation. Mutation: restore Sweep in beginScratch, restore Sweep in `scratchLaunch.finish`, or a full cache walk at enrollment; each fails the test. |
| 13 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellPrepFences$' ./... > "$o" && grep -F -- '--- PASS: TestCellPrepFences ' "$o")` | Integrated rerun of parent row 11: exit 0; named PASS. Concurrent starts, stale preparation, changed profiles, missing accounting and interrupted down preserve per-role leases, stop/authority gates and protected hashes. Mutation: accept a stale preparation receipt as a current role lease or reclaim unacknowledged work; test fails. |
| 14 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellLaunchTelemetry$' ./... > "$o" && grep -F -- '--- PASS: TestCellLaunchTelemetry ' "$o")` | Integrated rerun of parent row 12: exit 0; named PASS. Config-only opt-in records preparation through confirmed child start; missing/unknown child starts remain incomplete, shared work counted once, no-opt-in writes nothing and slow/failed sink changes no launch result. Mutation: gate parent recording only on ambient environment or omit final enrollment; test fails. |
| 15 | check:ci +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLoopAdminWaitDoesNotInvokeModel$' ./... > "$o" && grep -F -- '--- PASS: TestLoopAdminWaitDoesNotInvokeModel ' "$o")` | Integrated rerun of parent row 13: exit 0; named PASS. Production supervisor timers, unchanged observations and repeated receipts do not call adapter Start; one admitted invocation starts once and completion is delivered without a model waiter. Mutation: start a model on each observe timeout; the named test fails. |

## Pre-mortem and dispatch checks

Authenticated caller overrides its immutable profile at a direct lower boundary: row 1 bypass test. Cross-namespace receipt leaks: row 1 lookup mutation. Fixture default accidentally activates live profile: row 2 endpoint/CLI journey. Viewer attach starts a model: row 2 model-on-attach mutation. Component evidence is sold as the integrated promise: Task 5 and the unchanged parent thirteen-row acceptance table require exact joined-subject independent evidence.

Rows discriminate through the named production mutations; every claimed fact is source-pinned above. Final design adequacy, the mandatory custody API and source/fixture fidelity are independent review obligations, not presence checks. All four parent risk flags and its human gate are conserved. This split authors no runtime profile or operational permission.

## Evidence

<!-- A non-implementer records actual command, exit, named result, immutable subject/environment and date. No prospective row is claimed passed by authoring. -->

## Review

Gate: human. Parent decision issue #5492 remains the decision surface for the complete /26 promise. No child approval is inferred from an empty issue or from authoring this split. At design pickup, present the concrete child acceptance candidate under the existing decision process before activation or final acceptance. The proposed decomposition must be reviewed before dispatch.
