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
design: DR-process-extract
decision-issue: 6583
ruling: "https://github.com/medici-finance/assay/issues/6583#issuecomment-6905190371"
issues: [5492]
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
- "loopadmin/process/run_test.go: fixed-here (residue R1: TestProcessNilEnvIsEmpty pins a nil environment as empty; the entries for paths #5903 already moved are trimmed per the driver's decomposition acceptance, https://github.com/medici-finance/assay/issues/5492#issuecomment-6902528386)"
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
- `tools/desk/internal/cellcadence/mutations.json`
- `tools/desk/internal/cellscratch/mutations.json`
- `ci/staged-workflows/cellcache-mutate.yml`
- `.github/scripts/go-test-shard.py`
- `tools/go-analysis/baseline.json`

facts: Delivery record: the runner move this child specifies landed as increment 1 of #5492 in merged PR #5903 (merge aae4fbf71; the move itself is 957f953b7), after this split was authored. `tools/desk/internal/cellprocess` is retired and `loopadmin/process` exists on main. This child's scope and gate are unchanged, it is not retired, and independent verification and the human gate still own acceptance. Measured against main at 876ca4bca, the delivery satisfies rows 1-6 and the delivered half of row 8 (the move, importer migration, packaging, audit re-key, contract rows and custody guards); those rows now name tests that exist there, under their delivered names. Residue, owned by this child's implementer once #5492 releases the gate: R1, the literal nil-environment test that Task 2 names, which #5903 did not write (row 9); R2, row 7's delivery input, because #5903 cannot be it (see the row 7 note under Verify). The README row for this child is generated and is not edited here; `todo` stays accurate while R1 and R2 are open. The rest of this paragraph describes the pickup head, before that delivery. At the pickup head, `loopadmin/process` is absent. The sole raw harness runner is `tools/desk/internal/cellprocess/run.go::runObserved`; its public wrappers call `withCache`, which imports desk-internal `cellcache` and `deskkit`. A literal file move cannot compile outside that module and omitting those imports removes custody/telemetry. Production importers are cellctl cadence/cache/scratch/comms, cellreader and gitmaintenance. The cache class guard (now `TestLaunchCustodyClassGuard`), `TestCacheObservedRoutes`, cachequiet and the forge register inspect this route. Merged PR #4157 delivered bounded startup and cadence cache maintenance; consume that implementation and its committed controls, do not repair the same class again. Residue delivery: R1 adds `TestProcessNilEnvIsEmpty` in `loopadmin/process/run_test.go`; for R2, `consumers:` is trimmed to that one path, the only consumer path the residue delivery edits, citing the driver's acceptance of this decomposition (https://github.com/medici-finance/assay/issues/5492#issuecomment-6902528386). The `gates:` edge was removed by a separate change on main.

layering: Extract process-tree execution once into loopadmin, with a narrow mandatory host-custody port supplied by the desk owner. Keep storage policy, credential resolution and the recorder in their current owners; process code cannot import tools/desk internal packages. Retire the old process package and migrate every importer in the same source increment. A missing mandatory custody adapter refuses instead of silently selecting an unguarded process route.

design-fit:
- owner: loopadmin/process for process-tree containment; cellcache for storage custody; deskkit/perfrec for the existing telemetry meaning
- contract: R-observed-launch-route, R-cache-consumer-custody and R-cache-launch-custody in docs/contracts.md
- retires: the old cellprocess implementation path; no duplicate runner or control is retained
- weight: no new command/flag/refusal counts are ratified by authoring; the implementation must measure its exact delta, keep additions in the canonical owner and explain each positive dimension. The existing runner retires in this child.
- why-add: Moving the existing process owner first makes shared execution reusable while preserving cache custody, bounded preparation and the audit that sees every launch.

single-point-of-failure: the shared execution boundary; independent cache-custody and audit/source guards must remain effective when an upper caller is bypassed.

## Human decision

The question is whether to accept the delivered design that moves the desk's single process runner into the shared loop-admin module. An earlier merged change made the move. This change adds what was left: a test pinning that a launch with a nil environment starts the child with an empty environment, and a trim of the consumer list to the one path this change edits.

Under the design there is one runner. Every launch passes through a mandatory custody port that the desk supplies. A runner built without a custodian refuses to launch, and so does a custodian that admits a launch without a way to finish it. Cache storage policy, credentials and telemetry stay with their existing desk owners. The process-launch audit now covers the new module, and the old runner package is deleted rather than kept as a second copy. Two limits remain open: the import guard checks the desk module only, and native Windows runtime containment has not been qualified.

A human has to decide because this design moves the audited process-launch site and its cache-custody guards. If the boundary is wrong, a caller can start a process outside cache custody or outside the audit, and a passing test run would not show it.

Options:
1. **Accept the design as delivered.** The design record is approved as written. Independent verification then runs this item's checks on merged main, and final acceptance stays a separate human step.
2. **Accept with changes.** Name the boundary change you want, for example a different custody port shape or an extra guard. A follow-up change is opened for it, and verification waits until it lands.
3. **Hold.** The item stays at its current status, the design record stays unapproved, and verification does not start.

Default if no answer: none — blocks until answered

With no answer, nothing moves: the design record stays a draft, the item stays at its current status, and verification does not start.

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

The parent-row number in Expect identifies the unchanged obligation this child implements. These tests are prospective deliverables, not observed results; after the #5903 delivery, rows 1-6 and 8 name tests that exist on main and row 9 and row 7's input are the residue named in the delivery record. Preserve the original named selectors even where their existing contract names exceed the worker naming preference. Use targeted bounded package runs in the worker; CI owns the full matrix.

For row 7, the independent verifier obtains the actual implementation delivery from its
merged PR and records its full immutable SHA in `ASSAY_DELIVERY_SHA` and its actual
first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither defaults to HEAD, origin/main or
this authoring PR. Both commits and the fetched mainline must exist locally; the command
requires the delivery to have landed on that mainline before checking its immutable
snapshot. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed. These are
future delivery inputs.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestProcessLaunchAuditCoversLoopAdmin$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestProcessLaunchAuditCoversLoopAdmin ' "$o")` | Parent row 4: exit 0; named PASS. Audit sees the moved production runner and both roots; planted unregistered loopadmin exec fails; dropping the loopadmin root fails. |
| 2 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellPrepBounded$' ./cmd/cellctl/ ./internal/cellcache/ > "$o" && grep -F -- '--- PASS: TestCellPrepBounded ' "$o")` | Parent row 10: exit 0; named PASS. Production up/direct/cadence and storage seams retain the full parent workload matrix; begin/finish sweeps and walking cache enrollment each fail the test. |
| 3 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellPrepFences$' ./cmd/cellctl/ > "$o" && grep -F -- '--- PASS: TestCellPrepFences ' "$o")` | Parent row 11: exit 0; named PASS. Concurrent starts, stale receipts/profile/generation, absent accounting and interrupted down retain per-role leases, stop/authority checks and protected hashes; stale-lease and unacknowledged-reclaim mutations fail. |
| 4 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellLaunchTelemetry$' ./cmd/cellctl/ ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestCellLaunchTelemetry ' "$o")` | Parent row 12: exit 0; named PASS. Resolved config-only opt-in records shared preparation once through actual confirmed starts; unknown/refused starts stay incomplete; off/slow/failed sink semantics remain; ambient-only consent and missing-enrollment mutations fail. |
| 5 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && p=$(mktemp) && trap 'rm -f "$o" "$p"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCacheProcessCustody$' ./internal/cellrunner/ > "$o" && grep -F -- '--- PASS: TestCacheProcessCustody ' "$o" && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLaunchCustodyClassGuard$' ./internal/cellrunner/ > "$o" && grep -F -- '--- PASS: TestLaunchCustodyClassGuard ' "$o" && GOWORK=off go build -o /dev/null ./cmd/cellctl/ && cd ../../loopadmin && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestRunnerCustodyBoundary$' ./process/ > "$p" && grep -F -- '--- PASS: TestRunnerCustodyBoundary ' "$p" && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCacheObservedRoutes$' ./process/ > "$p" && grep -F -- '--- PASS: TestCacheObservedRoutes ' "$p")` | Exit 0; four named PASS lines, delivered by #5903 under these names. `TestCacheProcessCustody` (tools/desk/internal/cellrunner) is the production process-to-host-custody integration: the desk custodian holds the cache lease while a launch is admitted, definite completion releases it, uncertain completion retains it until confirmed recovery, and a storage-pressure floor refuses the launch before any child starts. `TestLaunchCustodyClassGuard` plants a second importer of the runner outside `internal/cellrunner` and must report it. In `loopadmin/process`, `TestRunnerCustodyBoundary` covers a missing custodian, refused custody, an admission-less custodian and uncertain cleanup reported unclean, and `TestCacheObservedRoutes` plants second raw observed launch sites that must each be reported. Omitted enrollment fails `TestCacheProcessCustody`. Original cadence tests stay unchanged. Full platform/release builds remain separate required qualification. |
| 6 | check +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && test ! -e tools/desk/internal/cellprocess && { git grep -n -e 'internal/cellprocess' -- . ':(exclude)CHANGELOG.md' ':(exclude)changelog/' ':(exclude)docs/streams/' ':(exclude)statusgen/strength_test.go' > "$o"; rc=$?; if test "$rc" -gt 1; then exit 1; fi; rest=$(grep -v -E -e '^loopadmin/README\.md:[0-9]+:.*The one process runner, moved here from .tools/desk/internal/cellprocess. \(graph-execution/26\)' -e '^tools/desk/internal/deskkit/forge_surface_test\.go:[0-9]+:// \(loopadmin/process, moved from internal/cellprocess, #5492\)\.$' "$o"); st=$?; if test "$st" -gt 1; then exit 1; fi; echo "$rest"; test -z "$rest"; })` | Exit 0: the old package directory is gone and no live source, mutation manifest, staged workflow, shard table, analysis baseline or current doc still names `internal/cellprocess` (history, stream records and the statusgen fixture string are excluded by path; the two provenance notes that name the former location, one line of `loopadmin/README.md` and one comment line in `tools/desk/internal/deskkit/forge_surface_test.go`, are filtered by their exact line content, so any other `internal/cellprocess` reference in either file still fails the row). A git error also fails (only grep's no-match exit 1 passes). Mutation: leave one `internal/cellprocess` path in `tools/desk/internal/cellscratch/mutations.json`; the row fails. |
| 7 | check:ci | `(test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --root . --consumers --brief graph-execution/40 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA" > "$consumer_out" && cat "$consumer_out" && grep -Eqx -- 'summary: [1-9][0-9]* corroborated, 0 disproved, 0 unchecked, 0 brief\(s\) claiming nothing' "$consumer_out")` | Exit 0 only for the named landed implementation delivery and its first parent: each `consumers:` entry flips to fixed-here where the diff edits it and corroborates; record exact corroborated/disproved/unchecked counts, none disproved or unchecked. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed. A routing claim the diff contradicts fails. Input rule after the #5903 delivery: #5903 (merge aae4fbf71, first parent 50cd771d4) cannot be the input. This child's brief is not in its snapshot, and statusgen v1.0.84 reports `COULD-NOT-CHECK: brief must resolve uniquely in the delivered snapshot`. The input is the landed delivery that carries this child's residue (R1 and R2), after this brief is on main. That diff will not edit the consumer paths #5903 already moved, so the residue implementer trims `consumers:` to the paths the residue delivery edits in the same change, citing the driver's decomposition acceptance on #5492; until then this row fails closed. |
| 8 | check +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v ./process/... > "$o" && grep -F -- '--- PASS: TestRunPreservesLaunchAndExit ' "$o")` | Task 2, delivered half: exit 0; the moved package's own tests all pass, including the named PASS (delivered by #5903 as `TestRunPreservesLaunchAndExit`). The child receives exactly the arguments, working directory and supplied environment, sees a parent-only variable as absent, gets EOF on stdin and returns its exit code. Mutation: have the child inherit the parent environment; the named test fails on the leaked variable. The literal nil-environment case is row 9. |
| 9 | check +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v ./process/... > "$o" && grep -F -- '--- PASS: TestProcessNilEnvIsEmpty ' "$o")` | Residue R1 (Task 2), not delivered by #5903: exit 0; the package's tests all pass, including the named PASS. A call with a nil environment (not a non-nil one that omits a variable, which row 8's test covers) starts the child with an empty environment and no inherited secrets. The test does not exist on main, so this row fails until the residue lands. Mutation: treat a nil environment as inherit-the-parent; the named test fails. |

## Pre-mortem and dispatch checks

A process move drops cache enrollment or its telemetry: rows 2–5 and the carried custody mutations. Audit scans the empty old directory: row 1 must observe the actual moved exec site and the planted site. Release packaging omits sibling modules: Task 3 and the GOWORK=off consumer build, plus independent release-build review. Platform containment drift: preserve native fixture tests and report unavailable Windows runtime qualification honestly.

Rows discriminate through the named production mutations; every claimed fact is source-pinned above. Final design adequacy, the mandatory custody API and source/fixture fidelity are independent review obligations, not presence checks. All four parent risk flags and its human gate are conserved. This split authors no runtime profile or operational permission.

## Evidence

<!-- A non-implementer records actual command, exit, named result, immutable subject/environment and date. No prospective row is claimed passed by authoring. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestProcessLaunchAuditCoversLoopAdmin$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestProcessLaunchAuditCoversLoopAdmin ' "$o")` | pass exit=0 | sha256:f909bcb1dc1c | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellPrepBounded$' ./cmd/cellctl/ ./internal/cellcache/ > "$o" && grep -F -- '--- PASS: TestCellPrepBounded ' "$o")` | pass exit=0 | sha256:ba5512a71c9d | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellPrepFences$' ./cmd/cellctl/ > "$o" && grep -F -- '--- PASS: TestCellPrepFences ' "$o")` | pass exit=0 | sha256:a286eb5332f6 | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCellLaunchTelemetry$' ./cmd/cellctl/ ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestCellLaunchTelemetry ' "$o")` | pass exit=0 | sha256:26ceced244f0 | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd tools/desk && o=$(mktemp) && p=$(mktemp) && trap 'rm -f "$o" "$p"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCacheProcessCustody$' ./internal/cellrunner/ > "$o" && grep -F -- '--- PASS: TestCacheProcessCustody ' "$o" && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLaunchCustodyClassGuard$' ./internal/cellrunner/ > "$o" && grep -F -- '--- PASS: TestLaunchCustodyClassGuard ' "$o" && GOWORK=off go build -o /dev/null ./cmd/cellctl/ && cd ../../loopadmin && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestRunnerCustodyBoundary$' ./process/ > "$p" && grep -F -- '--- PASS: TestRunnerCustodyBoundary ' "$p" && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestCacheObservedRoutes$' ./process/ > "$p" && grep -F -- '--- PASS: TestCacheObservedRoutes ' "$p")` | pass exit=0 | sha256:cd850eab5d5b | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && test ! -e tools/desk/internal/cellprocess && { git grep -n -e 'internal/cellprocess' -- . ':(exclude)CHANGELOG.md' ':(exclude)changelog/' ':(exclude)docs/streams/' ':(exclude)statusgen/strength_test.go' > "$o"; rc=$?; if test "$rc" -gt 1; then exit 1; fi; rest=$(grep -v -E -e '^loopadmin/README\.md:[0-9]+:.*The one process runner, moved here from .tools/desk/internal/cellprocess. \(graph-execution/26\)' -e '^tools/desk/internal/deskkit/forge_surface_test\.go:[0-9]+:// \(loopadmin/process, moved from internal/cellprocess, #5492\)\.$' "$o"); st=$?; if test "$st" -gt 1; then exit 1; fi; echo "$rest"; test -z "$rest"; })` | pass exit=0 | sha256:01ba4719c80b | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v ./process/... > "$o" && grep -F -- '--- PASS: TestRunPreservesLaunchAndExit ' "$o")` | pass exit=0 | sha256:f2db5c2f8b21 | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v ./process/... > "$o" && grep -F -- '--- PASS: TestProcessNilEnvIsEmpty ' "$o")` | pass exit=0 | sha256:ff3e2f317256 | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6598 run=4ea3c33804a7bc57d00fad13a8a132baf383576502e682088dd95f3ed5f94498 source=476f9f13e2c0b00fc0d5143d5b53597e714b09e5 brief=docs/streams/graph-execution/brief-40-extract-the-shared-process-runner-with-launch-custody-intact.md model=claude-opus-5-5 tier=strong

Row 7 is check:ci; this darwin host has no unshare network sandbox, so it ran in the network-off container lane (verify-runner image, statusgen v1.0.85) inside one admission bracket opened and closed on the host. Delivery input: PR #6568, ASSAY_DELIVERY_SHA = 3698fac71ada2b89bd0ef6b331f205a6116dade6, ASSAY_DELIVERY_BASE_SHA = 50f906fcf8664d0b65aa59a68a498c2e944c0c58 (its first parent); the delivery is an ancestor of origin/main at the verified source.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `(test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --root . --consumers --brief graph-execution/40 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA" > "$consumer_out" && cat "$consumer_out" && grep -Eqx -- 'summary: [1-9][0-9]* corroborated, 0 disproved, 0 unchecked, 0 brief\(s\) claiming nothing' "$consumer_out")` | pass exit=0 sandbox=container-netns | sha256:853956683b8a | 2026-11-16 | assay-verifier-app[bot] @ 476f9f13e2c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:315d28ece52a3d2ecf63784a5bf629a49c2371474c15d569c93bdda12dee67f4
Container-Image: sha256:271d9b171d01592693dc055717f2aa22661bae2ed2e6d59d6e50ecfc0fa5943a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6598 run=4ea3c33804a7bc57d00fad13a8a132baf383576502e682088dd95f3ed5f94498 source=476f9f13e2c0b00fc0d5143d5b53597e714b09e5 brief=docs/streams/graph-execution/brief-40-extract-the-shared-process-runner-with-launch-custody-intact.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=4ea3c33804a7bc57d00fad13a8a132baf383576502e682088dd95f3ed5f94498 source=476f9f13e2c0b00fc0d5143d5b53597e714b09e5 tree=sha256:315d28ece52a3d2ecf63784a5bf629a49c2371474c15d569c93bdda12dee67f4 witness=sha256:57df41c59fd5ecb4dc3f49216e01459c054c06c7ea6ea32da0b80c33d9c083c2 opened=2026-11-16T14:55:35Z closed=2026-11-16T14:56:11Z before=receipt after=check-verifier

### Non-implementer verifier run, 2026-11-16, merged main 476f9f13e2c0

Rows 1-6, 8 and 9 (check class) ran on the darwin host with `statusgen verifyrun --rows 1,2,3,4,5,6,8,9`; row 7 ran in the closed container bracket above (`--bracket-digest` verdict=match, span-lines tree-digest:1, container-image:1, attestation:1). Every row exited 0 and printed its named PASS line; row 7 printed `summary: 1 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`, the one corroborated entry being the loopadmin/process/run_test.go consumer (fixed-here).

Supplemental observations, no acceptance credit (they discharge no original row):

- S1. Probe: host re-read of row 7's consumer report: `statusgen --root . --consumers --brief graph-execution/40 --delivery 3698fac71ada --base 50f906fcf866` (full SHAs as above). Observed: exit 0; `summary: 1 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`; the tool also prints PRODUCER-DELIVERY: COULD-NOT-CHECK, so the producer was established independently: merge 3698fac71 of PR #6568 adds the nil-environment test in loopadmin/process/run_test.go and trims `consumers:` to that path. Date 2026-11-16; runner assay-verifier-app[bot] @ 476f9f13e2c0 (model verifier).
- S2. Probe: row 9 mutation in a scratch copy outside the home: the nil environment inherits the parent (`if env == nil { cmd.Env = os.Environ() }`), then `go test -run '^TestProcessNilEnvIsEmpty$' ./process/`. Observed: exit 1; `--- FAIL: TestProcessNilEnvIsEmpty`, reporting 114 inherited parent variable names. Date 2026-11-16; runner assay-verifier-app[bot] @ 476f9f13e2c0 (model verifier).
- S3. Probe: row 8 mutation in a scratch copy outside the home: the child always inherits (`cmd.Env = append(os.Environ(), env...)`), then `go test -run '^TestRunPreservesLaunchAndExit$' ./process/`. Observed: exit 1; `--- FAIL: TestRunPreservesLaunchAndExit`, the parent-only variable reached the child. Date 2026-11-16; runner assay-verifier-app[bot] @ 476f9f13e2c0 (model verifier).

Risk-bearing values (the item is irreversible). Enumerated over the delivery diffs of PR #5903 (50cd771d4..aae4fbf71, non-test sources) and PR #6568 (50f906fcf..3698fac71) plus the Deliverables: the environment literal, the forge-register key, the audit root, pipeWait, cleanupWait, the two loopadmin dependency pins and the Verify rows' 90s test timeouts.

- RISK-VALUE: DERIVED — cmd.Env = append([]string{}, env...) @ loopadmin/process/run.go:153 — os/exec inherits the parent environment only when Cmd.Env is nil; appending zero elements to the non-nil empty literal yields a non-nil empty slice, so a nil env starts the child with an empty environment and a supplied env passes through exactly. Carried unchanged from the retired runner (line 119 at 50cd771d4); mutation S2 fails the row 9 test.
- RISK-VALUE: DERIVED — Key = "loopadmin/process/run.go::runObserved::<unresolved>" @ tools/desk/internal/forgeban/allowlist.go:201 — the register key is tree prefix + file :: function :: argv0; the one exec site now sits in function runObserved of loopadmin/process/run.go (line 143) with a non-constant argv0 under the "loopadmin/" audit prefix, and it replaces the old internal/cellprocess key one for one, so no second permit survives. Row 1 passes against it.
- RISK-VALUE: DERIVED — loopAdminTreeRoot = "../../../../loopadmin" @ tools/desk/internal/deskkit/forge_surface_test.go:43 — the test package is four directories below the repository root, so the path resolves to the root loopadmin module the runner moved into; dropping that root blinds the audit, which row 1's mutation covers.
- Ranked last, reversible by an edit and redeploy, no derivation needed: pipeWait = 500 * time.Millisecond @ loopadmin/process/run.go:23 and cleanupWait = 2 * time.Second @ loopadmin/process/run.go:24 (moved unchanged from lines 21-22 of the retired runner); golang.org/x/sys v0.49.0 and golang.org/x/term v0.47.0 @ loopadmin/go.mod:9-10 (equal to the tools/desk pins before and after, so no new dependency version was admitted); the Verify rows' -timeout 90s.

Open limits the brief itself records, outside this table: the import guard covers the desk module only, and native Windows runtime containment is not qualified (darwin host, linux container). The host rows' Runner cells note read-set recording as could-not-check on darwin; that annotation concerns read-set capture, not the row results.

**VERIFY: PASS** — 9/9 rows pass at 476f9f13e2c0 (rows 1-6, 8, 9 host; row 7 network-off container bracket). gate: human and irreversible: yes, so this Evidence is routed to the human gate; the status stays where it is.

## Review

Gate: human. Parent decision issue #5492 remains the decision surface for the complete /26 promise. No child approval is inferred from an empty issue or from authoring this split. At design pickup, present the concrete child acceptance candidate under the existing decision process before activation or final acceptance. The proposed decomposition must be reviewed before dispatch. "Decomposition review" is one event: a driver comment on #5492, under the driver's own login, accepting this split and its routing. The driver posted that comment on 2026-11-15 (https://github.com/medici-finance/assay/issues/5492#issuecomment-6902528386), accepting the split into /40, /41, /42 and /43 as merged; the `gates:` edge on #5492 that held this child off Next-up is removed by the follow-up PR that cites it.
