---
brief: assay:assay:graph-execution:40
title: Extract the shared process runner with launch custody intact
why: "Moving the existing process owner first makes shared execution reusable while preserving cache custody, bounded preparation and the audit that sees every launch."
wave: 1
depends: ["graph-execution/20"]
unblocks: ["graph-execution/41"]
split-from: graph-execution/26
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Relocates the audited process-launch site and its cache-custody guards. Human acceptance confirms every caller still crosses the same custody and process-containment boundary."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 5377cd38-a556-4367-b01b-dbc4b1390d3b
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
- "loopadmin/process/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/cmd/cellctl/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/internal/cellreader/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/internal/gitmaintenance/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/internal/cellcache/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/internal/forgeban/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/internal/deskkit/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/internal/cachequiet/: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "tools/desk/go.mod: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "docs/contracts.md: follow-up graph-execution/40 (this child; only implementation may change to fixed-here)"
- "Dockerfile: follow-up graph-execution/40 (this child; implementation corroborates the actual path)"
- "images/assay-harness/Dockerfile: follow-up graph-execution/40 (this child; implementation corroborates the actual path)"
- "images/verify-runner/Dockerfile: follow-up graph-execution/40 (this child; implementation corroborates the actual path)"
---

# Brief 40 — Extract the shared process runner with launch custody intact

## Context

files:
- `loopadmin/process/`
- `loopadmin/go.mod`
- `loopadmin/go.sum`
- `tools/desk/internal/cellprocess/` (retired)
- `tools/desk/internal/cellcache/`
- `tools/desk/internal/deskkit/{perf.go,forge_surface_test.go,citrigger_test.go}`
- `tools/desk/internal/forgeban/allowlist.go`
- `tools/desk/internal/cachequiet/`
- `tools/desk/cmd/cellctl/`
- `tools/desk/internal/{cellreader,gitmaintenance}/`
- `tools/desk/go.mod`
- `tools/desk/go.sum`
- `docs/{cellctl-cadence.md,cellctl-scratch.md,cellctl-go-cache.md,contracts.md}`
- `tools/desk/README.md`
- `changelog/graph-execution-40-process-extraction.md` (planned)
- `Dockerfile`
- `images/assay-harness/Dockerfile`
- `images/verify-runner/Dockerfile`

facts: At the pickup head, `loopadmin/process` is absent. The sole raw harness runner is `tools/desk/internal/cellprocess/run.go::runObserved`; its public wrappers call `withCache`, which imports desk-internal `cellcache` and `deskkit`. A literal file move cannot compile outside that module and omitting those imports removes custody/telemetry. Production importers are cellctl cadence/cache/scratch/comms, cellreader and gitmaintenance. `TestCacheClassGuard`, `TestCacheObservedRoutes`, cachequiet and the forge register inspect this route. Merged PR #4157 delivered bounded startup and cadence cache maintenance; consume that implementation and its committed controls, do not repair the same class again.

layering: Extract process-tree execution once into loopadmin, with a narrow mandatory host-custody port supplied by the desk owner. Keep storage policy, credential resolution and the recorder in their current owners; process code cannot import tools/desk internal packages. Retire the old process package and migrate every importer in the same source increment. A missing mandatory custody adapter refuses instead of silently selecting an unguarded process route.

design-fit:
- owner: loopadmin/process for process-tree containment; cellcache for storage custody; deskkit/perfrec for the existing telemetry meaning
- contract: R-observed-launch-route, R-cache-consumer-custody and R-cache-launch-custody in docs/contracts.md
- retires: the old cellprocess implementation path; no duplicate runner or control is retained
- weight: no new command/flag/refusal counts are ratified by authoring; the implementation must measure its exact delta, keep additions in the canonical owner and explain each positive dimension. The existing runner retires in this child.
- why-add: Moving the existing process owner first makes shared execution reusable while preserving cache custody, bounded preparation and the audit that sees every launch.

single-point-of-failure: the shared execution boundary; independent cache-custody and audit/source guards must remain effective when an upper caller is bypassed.

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

1. Inventory all process importers and raw process primitives at the current head. Move the existing process-tree implementation, including Unix groups/interactive terminal handling and Windows suspended job enrollment, into `loopadmin/process`; delete the old package, without copying or retaining a second runner.
2. Preserve the public invocation semantics, including nil environment meaning empty, cancellation/deadline propagation, output bounds, observation after custody, uncertain cleanup and retained active-cache records. Design the narrow host-custody integration so no caller can bypass required enrollment by choosing an alternate observed entrypoint or an optional callback. Keep policy/environment interpretation and telemetry with existing desk owners; qualify the boundary with direct calls and a planted second raw caller.
3. Migrate every current importer, including non-cadence consumers. Keep existing cadence tests unchanged. Link loopadmin through a versioned requirement plus relative replace, carry its sibling replacements through GOWORK=off tools/desk/release builds, and update module-copy packaging inputs. Admit any new dependency version through the existing risk-summary procedure before changing manifests; no competing implementation to avoid admission.
4. Extend the forge/process-launch audit to scan loopadmin and re-key its one moved exec site. Retain the unresolved-argv verdict as could-not-check, not a forge permit. Carry cachequiet, raw-run and observed-route class guards and their planted controls through the extraction; update their canonical contract rows and cross-module trigger registry. Prepare workflow changes as staged source if the role cannot publish them; do not silently leave tests unenforced.
5. Consume #4157's preparation, maintenance, storage and opt-in parent-recorder implementation. Carry the original parent rows 10–12 through real up/direct/cadence/runner/storage seams after extraction: 1/5 roles, cold/warm, 1x/10x history, concurrent up/direct, changed configuration/generation, maintenance held mid-scan, stop/authority refusals and clean/interrupted down. Preserve protected hashes, one role owner, zero admission/finish historical walks and sample counts/build/workload identities. No new latency SLO or consent widening.
6. Update process-owner and current cadence/storage/build documentation, the consumer/control register and changelog. Preserve old independently recorded evidence as historical; new source needs fresh joined evidence.

## Interface contract

Extract process-tree execution once into loopadmin, with a narrow mandatory host-custody port supplied by the desk owner. Keep storage policy, credential resolution and the recorder in their current owners; process code cannot import tools/desk internal packages. Retire the old process package and migrate every importer in the same source increment. A missing mandatory custody adapter refuses instead of silently selecting an unguarded process route. Caller identity, invocation and current generations remain attached to every typed state, receipt and evidence item. Unknown stays unknown; process success is never work acceptance.

## Verify

The parent-row number in Expect identifies the unchanged obligation this child implements. These tests are prospective deliverables, not observed results. Preserve the original named selectors even where their existing contract names exceed the worker naming preference. Use targeted bounded package runs in the worker; CI owns the full matrix.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestProcessLaunchAuditCoversLoopAdmin$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestProcessLaunchAuditCoversLoopAdmin ' "$o")` | Parent row 4: exit 0; named PASS. Audit sees the moved production runner and both roots; planted unregistered loopadmin exec fails; dropping the loopadmin root fails. |
| 2 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellPrepBounded$' ./cmd/cellctl/ ./internal/cellcache/ > "$o" && grep -F -- '--- PASS: TestCellPrepBounded ' "$o")` | Parent row 10: exit 0; named PASS. Production up/direct/cadence and storage seams retain the full parent workload matrix; begin/finish sweeps and walking cache enrollment each fail the test. |
| 3 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellPrepFences$' ./cmd/cellctl/ > "$o" && grep -F -- '--- PASS: TestCellPrepFences ' "$o")` | Parent row 11: exit 0; named PASS. Concurrent starts, stale receipts/profile/generation, absent accounting and interrupted down retain per-role leases, stop/authority checks and protected hashes; stale-lease and unacknowledged-reclaim mutations fail. |
| 4 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellLaunchTelemetry$' ./cmd/cellctl/ ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestCellLaunchTelemetry ' "$o")` | Parent row 12: exit 0; named PASS. Resolved config-only opt-in records shared preparation once through actual confirmed starts; unknown/refused starts stay incomplete; off/slow/failed sink semantics remain; ambient-only consent and missing-enrollment mutations fail. |
| 5 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestProcessCustodyFlow$' ./internal/cellcache/ > "$o" && grep -F -- '--- PASS: TestProcessCustodyFlow ' "$o" && GOWORK=off go build ./cmd/cellctl/)` | Exit 0; named production process→host-custody integration proves final enrollment, current free-space refusal, definite completion release and uncertain completion retention through the moved owner. Carry existing custody, observed-route and containment fixtures to their new owning packages without removing assertions. A planted second raw caller and omitted enrollment each fail; original cadence tests stay unchanged. Full platform/release builds remain separate required qualification. |

## Pre-mortem and dispatch checks

A process move drops cache enrollment or its telemetry: rows 2–5 and the carried custody mutations. Audit scans the empty old directory: row 1 must observe the actual moved exec site and the planted site. Release packaging omits sibling modules: Task 3 and the GOWORK=off consumer build, plus independent release-build review. Platform containment drift: preserve native fixture tests and report unavailable Windows runtime qualification honestly.

Rows discriminate through the named production mutations; every claimed fact is source-pinned above. Final design adequacy, the mandatory custody API and source/fixture fidelity are independent review obligations, not presence checks. All four parent risk flags and its human gate are conserved. This split authors no runtime profile or operational permission.

## Evidence

<!-- A non-implementer records actual command, exit, named result, immutable subject/environment and date. No prospective row is claimed passed by authoring. -->

## Review

Gate: human. Parent decision issue #5492 remains the decision surface for the complete /26 promise. No child approval is inferred from an empty issue or from authoring this split. At design pickup, present the concrete child acceptance candidate under the existing decision process before activation or final acceptance. The proposed decomposition must be reviewed before dispatch.
