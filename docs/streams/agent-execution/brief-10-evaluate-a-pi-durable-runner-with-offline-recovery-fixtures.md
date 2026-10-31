---
brief: assay:assay:agent-execution:10
title: Evaluate a Pi Durable runner with offline recovery fixtures
why: Pi Durable may remove conversation and tool-recovery machinery we would otherwise maintain. A bounded experiment must show which code can be omitted without weakening invocation ownership, uncertain-effect handling or independent acceptance.
wave: 1
depends: ["graph-execution/20"]
unblocks: []
effort: M
gate: model
risk: {regulatory: "no", customer: "no", irreversible: "no", sensitive-data: "no"}
issues: []
schema: brief-v2
version: 1
id: f3d1b020-c31b-4e6c-ac2c-d13bd404ff60
authored: 2026-10-22 by runtime pilot authoring session
sources:
- docs/proposals/non-ci-architecture/27-agent-execution-environments.md
- docs/proposals/non-ci-architecture/11-target-architecture.md
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md
- spec/loop-admin-runner-v1.md
- freshness-checked 2026-10-22 @ ecc895ce2dd5989ff2119fb7aeeed5e1e0b8761e (origin/main)
- "Pi source 8c03fef4d3b7ecf7a9e8cc5b1312ff60ec2be149: packages/durable/src/harness/tool.ts, README.md and src/testing/storage-conformance.ts"
- "Vercel durable-harness source fb056a3c08549fdad9b87ed8358a27e2097ded11: packages/pi-world, packages/durable-harness/src/pi-durable.ts and test/pi-durable-sigkill.test.ts"
exec-tier: strong
exec-tier-why: Cross-process recovery, capability honesty, late ownership and effect reconciliation require concurrency and boundary reasoning.
domain: complicated
outcome: none
consumers:
- "loopadmin/experiments/pirunner/: follow-up agent-execution/10 (this brief; experimental adapter and recovery fixtures)"
- "loopadmin/testdata/pirunner/: follow-up agent-execution/10 (this brief; source, result and fault manifests)"
- "docs/reports/agent-execution/pi-runner-pilot.md: follow-up agent-execution/10 (this brief; decision and deletion map)"
- "loopadmin/README.md: follow-up agent-execution/10 (this brief; explicit offline invocation only)"
- "ci/staged-workflows/loopadmin.yml: follow-up agent-execution/10 (this brief; offline experiment test coverage)"
- "loopadmin/runner/: out-of-scope (consume its existing API and conformance kit unchanged)"
- "loopadmin/supervisor and loopadmin/adapters/localharness: out-of-scope (graph-execution/26 and /22 retain implementation; this optional result does not block them)"
- "managed persistence, controller and role executors: out-of-scope (existing R2/R4 and agent-execution/03-04 retain adoption and integrated qualification)"
---

# Brief 10 — Evaluate a Pi Durable runner with offline recovery fixtures

## Context

files: `loopadmin/experiments/pirunner/adapter.go` (planned), `loopadmin/experiments/pirunner/pilot_test.go` (planned), `loopadmin/experiments/pirunner/node/{package.json,package-lock.json,tsconfig.json,src/}` (planned), `loopadmin/testdata/pirunner/` (planned), `loopadmin/README.md`, `ci/staged-workflows/loopadmin.yml`, `docs/reports/agent-execution/pi-runner-pilot.md` (planned), `changelog/agent-execution-10-pi-runner-pilot.md` (planned). New experiment/report/fixture paths are planned deliverables, not present behavior. No live workflow file, Go core API, production profile or default adapter is changed.

single-point-of-failure: trusting a resumed tool's original authorization — backed by an independent synthetic effect executor's current-owner check; bypass the upper adapter check to prove that lower boundary refuses. These fixtures qualify an experimental integration, not production custody.

facts:
- At public main `ecc895ce2dd5989ff2119fb7aeeed5e1e0b8761e` on 2026-10-22, `loopadmin/runner` implements `Adapter` (Start/Observe/Cancel/Reconcile), `Client`, capabilities and the exported `conformance.RunAll`/`Subject`/fault `Control`. `Client` state is in memory; durable launch journaling belongs to graph-execution/26. No `workflow/`, `loopadmin/supervisor/` or real harness adapter exists at this inspection.
- `graph-execution/20` is independently verified in the source board; this brief consumes that contract unchanged. The existing `runner/conformance` package passed a fresh authoring check on 2026-10-22. No Pi adapter or recovery experiment was executed while authoring this brief.
- Pi source `8c03fef4d3b7ecf7a9e8cc5b1312ff60ec2be149` labels Durable experimental and declares version 1.4.0/Node >=22.19. Its initial tool `call` runs `beforeTool`, persists intent, then runs the tool. Recovered `execute` can rerun a safe tool without repeating that initial hook. Stored and current replay policies must both permit safe recovery.
- Pi input `requestId` deduplicates submissions, not Assay launches. Built-in storage has one process owner and no cross-process locking. SQLite is an offline reference backend here, not a second managed coordination store.
- Vercel source `fb056a3c08549fdad9b87ed8358a27e2097ded11` labels durable-harness experimental; its development Pi tuple is 1.0.4, not inspected Pi main 1.4.0. Its World journal/queue and session fencing are useful comparison evidence; compatibility is unproven. Its public Claude/Codex adapters require Vercel Sandbox.

layering: One experiment package in the existing loopadmin module implements `runner.Adapter` over a private bounded JSON-lines bridge to one pinned Node/Pi process. The process boundary isolates the dependency and its termination/restart lifecycle; it creates no supported protocol, listener, service or launcher. Existing `runner.Client` and conformance drive it. A separate fixture-owned effect ledger enforces synthetic current authority independently. Task 2 and Verify 2/5 prove those boundaries; no workflow store or controller is imported.

design-fit:
  owner: loopadmin/runner owns the reused invocation contract; the experiment owns only adapter translation and measurement
  contract: none — no invocation semantic-owner row is declared in docs/contracts.md at the inspected revision; spec/loop-admin-runner-v1.md is the existing contract, unchanged
  retires: []
  weight: "verbs +0, flags +0, production refusals +0, production rule-text lines +0; one experiment package and dependency lockfile behind the `pirunner` build tag, so default `go test ./...` never builds it"
  why-add: n/a for those dimensions; an experiment is needed before delegating runtime recovery, and the report must show concrete omissions/removals rather than add a second implementation permanently

## Read first

- [Controlling execution-environment amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md), [whole-system target](../../proposals/non-ci-architecture/11-target-architecture.md) and [PostgreSQL/recovery owner](../../proposals/non-ci-architecture/14-storage-forge-graph.md).
- [Existing runner specification](../../../spec/loop-admin-runner-v1.md), `loopadmin/runner/{adapter,client,request,result,usage}.go`, `loopadmin/runner/conformance/{kit,claims,hostile}.go` and [module usage](../../../loopadmin/README.md).
- [Local adapter /22](../graph-execution/brief-22-local-harness-adapter.md), [supervisor /26](../graph-execution/brief-26-shared-loop-admin.md), [profile contract /01](brief-01-qualified-execution-profile.md), [readiness /03](brief-03-local-readiness-enforcement.md) and [integrated recovery /04](brief-04-recovery-effect-qualification.md).
- [Library-first ownership](../../library-first.md), [Go engineering](../../go-engineering.md) and [regression floor](../../proposals/non-ci-architecture/15-regression-from-day-one.md).
- Pinned public upstream [Pi tool recovery](https://github.com/earendil-works/pi/blob/8c03fef4d3b7ecf7a9e8cc5b1312ff60ec2be149/packages/durable/src/harness/tool.ts), [Pi storage conformance](https://github.com/earendil-works/pi/blob/8c03fef4d3b7ecf7a9e8cc5b1312ff60ec2be149/packages/durable/src/testing/storage-conformance.ts), [Vercel SIGKILL test](https://github.com/vercel-labs/durable-harness/blob/fb056a3c08549fdad9b87ed8358a27e2097ded11/packages/durable-harness/test/pi-durable-sigkill.test.ts) and [World integration](https://github.com/vercel-labs/durable-harness/blob/fb056a3c08549fdad9b87ed8358a27e2097ded11/packages/pi-world/README.md).

## Ground rules and bounded scope

- Isolated branch and draft PR; no merge, deployment, workflow dispatch, profile enrollment, credentials or provider/model/cluster calls. All models are faux/scripted and effects use a synthetic executor. Do not run package lifecycle scripts supplied by unreviewed dependencies.
- Pre-stage a vetted exact Node/Pi/compiler tuple, Go dependencies and source fixtures with integrity hashes. Then build/run the evidence corpus in the approved Linux offline verification environment with network denied, no secrets, host configuration or Docker socket exposed. Missing dependencies/isolation are could-not-check; do not weaken isolation to download them. Acquisition is separate from the network-off evidence run.
- One role/profile and one active admitted invocation; a second frozen process is permitted only for the takeover negative test. Exercise standing/workflow request shapes without creating a graph store. Use a child-process fixture, not the unfinished production supervisor.
- Stream admission under amendment 27 activates no runtime or profile. This brief is a planning record and cannot activate anything. Stop implementation at implemented; independent verification owns verified/done.
- **Opt-in, never default CI.** Every Go file in `loopadmin/experiments/pirunner` (tests included) carries `//go:build pirunner`. Default `GOWORK=off go test ./...` in `loopadmin` therefore builds and selects nothing from the package and stays green on a runner with no Node, Pi or npm packages; Verify row 12 proves that. Verify rows 2-9 pass `-tags pirunner`, and fail-not-skip (Task 1) applies once the tag is set: a missing Node/Pi artifact then fails the test. Do not weaken fail-not-skip to make default CI green, and do not commit a vendored `node_modules` tree or prebuilt bundle to make the tagged tests self-contained; dependencies come only from the pre-staged, integrity-pinned set. Acquisition and every later run use `--ignore-scripts` unconditionally (`ignore-scripts=true` in the experiment's npm config), with no registry egress in the evidence run.
- **CI coverage is staged and maintainer-gated.** The staged coverage line is a separate step or job, not an edit to the existing `go test -timeout 120s ./...` line, running `go test -tags pirunner -timeout 20m ./experiments/pirunner` over the pre-staged pinned set only. Adding Node or third-party packages to the self-hosted runner, or ending the staged loopadmin workflow's no-third-party-fetch invariant, is a maintainer ruling at promotion, not an implementer default; the implementer stages the workflow text and does not promote it. If CI is to fetch packages, re-author this brief's risk block and gate first.
- Cap this investigation at five engineering days from implementation pickup. Record effort; at the cap deliver go/no-go/unknown with partial evidence and remaining work instead of expanding into a controller, PostgreSQL backend or managed launcher. The cap is an investigation bound, not a delivery forecast.
- No production deletion in this brief. No generic retry engine, workflow host, transport service, credential broker, production permission hook or forked Pi core. If the work cannot remain M-sized within these exclusions, return the uncovered slice for separate authoring.

## Task

1. **Pin and reproduce.** Record exact package/compiler/Node versions, source revisions, lockfile integrity, license notices and supported OS. Record in `source-manifest.json` who vetted the tuple, the Node runtime checksum and the lockfile integrity before the network-off run, and have row 8 dereference that record. Select one compatible Pi tuple; never assume Pi main works with Vercel's development pin. Retain only minimal attributed source fixtures/hashes needed to dereference the report, not complete upstream trees. No npm/provider downloads occur in tests. Missing runtime/build artifacts fail the test rather than skip it. Record the implementation pickup assessment against the current target/contracts, absent production components and active work: continue/adapt/hold with reasons.
2. **Use the real runner contract.** Implement an experimental Go `runner.Adapter` and `conformance.Control` over the Node harness. Run the exported `conformance.RunAll` against it through the real `runner.Client`. Fault control must drive the Node/Pi adapter, not substitute a second Go fake for the subject. Keep injection controls test-owned, unavailable to managed profiles. Support a tested launch-token budget scope; refuse unsupported scopes/required capabilities. Pin actual model/profile/tools and preserve missing cost as unknown. Do not claim full process preservation or cancel acknowledgment from an abort request alone.
3. **Separate identities and authorities.** Persist the immutable invocation descriptor and stable `(caller, invocation, generation)` to Pi session/submission mapping in the disposable experiment directory before acknowledging start; recovery reconciles that mapping and changed payloads cannot reuse an identity. This is a fixture store with an explicit loss classification, not graph-execution/26's journal. Caller/effect authority stays in the independently controlled synthetic owner ledger. Pi task IDs and retries never mint fresh work IDs, spend allowances or effect IDs. A stale process cannot reopen the same built-in SQLite storage concurrently: the fixture launch owner enforces exclusive storage use, tests that admission directly, and confirms old process-tree stop before reopening it. A frozen old owner can be tested on an isolated copy for the independent effect-fence test; label that distinction, not a Pi multi-writer claim.
4. **Exercise real recovery.** Use `Harness.open`, durable submissions, tool tasks, source-pinned safe/unsafe replay and actual process kill/reopen. Implement execution-time current-owner/pause/budget rechecks on every actual tool execution, including replay that skips `beforeTool`. Route consequential synthetic tools through the separate effect ledger with stable logical effect identity and reconciliation after lost reply. Do not mark a network write safe merely because the tool wrapper is durable. Show tool and session outcomes separately from work acceptance. Preserve pending/unknown effects on cancellation, store loss and restart; do not turn an unknown into failed or zero usage.
5. **Compare and measure.** First use the existing conformance fake as the expected-contract oracle and Pi/SQLite as the real experimental subject; the fake is not a shipped native-supervisor performance comparator. Hold scripted model/tool inputs constant. Record attempts, effect counts, launches, file/conversation/process preservation, usage completeness, child cleanup and commit/reopen measurements. Exercise fixed fixtures of 100 and 1,000 tool-result entries, recording input bytes, log bytes, commit count, replay time and peak-memory coverage. These characterize the experiment, not permanent-desk capacity or provider savings.
6. **Borrow selectively.** Use upstream SIGKILL/frozen-owner tests and read-only observation patterns as test-design inputs. Vercel public `durable-harness/pi-durable` is an optional second arm only if Pi passes the mandatory corpus and time remains: same workload and locked compatible tuple, local World only, separately labeled results. Its private `pi-world` package is not a public import. Do not add a World queue and another driver as competing continuation owners. If not run, retain a source-backed Vercel compatibility disposition; its omission cannot invalidate an otherwise complete Pi result.
7. **Make wrong implementations fail.** Supply a test-owned negative-control runner/map under `loopadmin/testdata/pirunner/` (planned) that uses the existing mutation kit where its copying/build boundary works. Preserve relative Go module dependencies and pinned Node build inputs when materializing test copies; do not silently edit the shared mutation runner to evade a missing dependency. Controls must mutate the actual adapter/bridge execution path and rebuild it. Baseline passes; removing start dedupe, execution-time reauthorization, stable effect identity, unknown-stop hold or unknown-usage propagation must fail its named behavioral assertion. Missing tools, build/import failures, timeouts, skipped or zero-selected tests do not count as killed mutations. The negative-control test must verify these distinctions.
8. **Deliver the decision.** Publish the report and machine-readable `results.json` plus `source-manifest.json`, `cases.json` and the negative-control map in the named fixture directory. Bind every verdict to the locked tuple, source/subject hashes, actual command/exit, case and effect counters. Include the deletion map below with exact inspected paths/symbols and a kept/omit/remove disposition. Wire offline test instructions (naming the `pirunner` tag) and the staged CI coverage line defined in the ground rules, with its explicit timeout; do not promote workflows or install dependencies in normal Go tests. Report which proof remains owed by /01, /03-04, /22 and /26 before managed use. No second live trial is created.

## Expected deletion and retained responsibilities

These are hypotheses to substantiate, not authorization to delete unimplemented plans or existing controls. Distinguish code already present from code we could avoid writing. List the thin adapter's added maintenance surface beside every claimed saving.

| Candidate machinery | Expected outcome if Pi qualifies | Proof and owner that remain |
|---|---|---|
| Custom conversation transcript persistence, follow-up/steer inbox, compaction and stream normalization inside a Pi runner | Omit bespoke implementations; delegate to Pi Harness and snapshots | Report exact source/API and preserved behavior; /22 retains adapter qualification, /05 retains client projection |
| Inner tool intent/result checkpointing, safe replay and nested-task ownership | Omit a parallel inner-session journal/retry tree; use Pi tasks | Real kill/reopen cases, replay-time reauthorization and stable effect dedupe; /26 still owns launch intent/receipt |
| Host-specific observer that reacquires or wakes a session just to display it | Omit that proposed path for Pi; use read-only snapshots | Attach/observe test with zero new launch/model calls; existing Cell API authorization stays |
| Session wake/continuation and writer fencing | Only a potential later omission if a World-backed arm is qualified | Local World is not managed PostgreSQL qualification; no second scheduler or silent store adoption |
| Controller admission, cell/repository ownership and incarnation; outer launch journal; custody/effect ledger; candidate acceptance | Keep all | No-go if the proposal requires weakening, duplicating or relocating these owners to obtain a favorable result |
| Existing `tools/desk/internal/cellprocess` and standing launch consumers | Keep until the existing /26-27 migration proves all consumers moved | This experiment retires neither process containment nor a production launcher |

## Mandatory recovery corpus

Every case has a valid positive control and asserts actual Node/Pi calls, process identities and synthetic effect counts. A Go fake alone cannot satisfy a Pi case.

| Group / planned test | Cases and acceptance observable | Plausible wrong delivery to reject |
|---|---|---|
| `TestPiPilotContract` | Entire exported conformance kit; both modes; unsupported requirements refuse; exact profile/model/tools; malformed/late results denied | Only run a custom happy-path schema test or make the kit exercise a fake instead of the Pi adapter |
| `TestPiPilotRestart` | Lost start ack and duplicate start/submit find one invocation; kill after submit/during stream; reopen same attempt; safe tool may repeat, unsafe tool reports interrupted/partial and never blindly reexecutes | Treat requestId as launch dedupe, or restart as a fresh attempt/allowance |
| `TestPiPilotEffects` | Fake provider applies a write then loses reply; reconcile to one logical effect; model requests same action under a new tool-call ID; unresolved result stays held; failed intent persistence admits no new effect | Duplicate external effect after a model retry or a false definite-failure classification |
| `TestPiPilotAuthority` | Change owner/pause/budget after tool intent and kill; safe replay rechecks actual execution; bypass upper hook to call the synthetic executor directly; stale owner is independently refused; lost fixture store/incarnation stays held | Depend solely on beforeTool or revive authority from a restored session |
| `TestPiPilotStopAndObserve` | Ignored abort and background child remain unconfirmed until stopped; deny concurrent built-in storage open; long silent tool; attach/reconnect/read-only observe issue zero starts/model calls | Timeout means stopped, SQLite provides multi-owner fencing, or observer creates a driver |
| `TestPiPilotAccounting` | Retry/child usage counted once; partial cost remains unknown; restart preserves pause/limits; files-only restore refuses required conversation continuity; changed profile/unknown mandatory capability refuses | Reset spend to zero or claim file restoration preserves a process |
| `TestPiPilotDecisionEvidence` | Report/machine verdict derives from exact cases and source hashes; fixed-size reopen measurements identify actual workload/coverage | A wrong source tuple or omitted case still receives go |
| `TestPiPilotNegativeControls` | Each declared mutation rebuilds and fails its named assertion; unmodified case passes; runner rejects compile/skip/timeout as false kills | Mutations fail setup instead of demonstrating the control |

## Go / no-go / unknown

- **GO — optional adapter candidate only:** all mandatory groups pass on the locked real-Pi path, the negative controls discriminate, and the report identifies at least one precise bespoke implementation that can be omitted/removed without a second authority/store/driver. Added bridge, packaging, containment and upgrade responsibilities are explicit. A future managed-persistence adoption plan must retain R2; GO is not production qualification or approval to activate.
- **NO-GO for this integration:** repeatable duplicate effects, stale authorization, lost/unknown state presented as safe, unsupported mandatory semantics silently downgraded, or a required change of custody/coordination/acceptance owner. A demonstration may record expected refusal; it cannot claim this unsafe integration is an adapter candidate. Also no-go on the simplification hypothesis if every claimed deletion is offset by equivalent replacement machinery and the only benefit is another wrapper.
- **UNKNOWN / inconclusive:** dependency tuple, runtime prerequisites, minimum corpus or attribution cannot be established within the cap. Missing evidence never passes. A properly evidenced no-go or unknown can complete this investigation, but must retain its failed/unrun cases and stop downstream adoption. A red/absent mandatory group cannot be called GO.

The decision-evidence test must reject fabricated GO classifications for every adverse/unknown fixture, while permitting truthful no-go/unknown reports. Behavioral tests may prove correct refusal of an unsupported scenario; an unimplemented mandatory test is still unrun. Do not force a green implementation narrative to finish the investigation. No model-quality, provider-cost or accepted-work throughput claim follows from faux-model timings. Runtime-change adoption and any live cohort require a separately reviewed scoped follow-on under the existing owners.

## Verify

All new commands/tests below are implementation deliverables, **not authoring-time evidence**. Run from repository root in the pre-staged network-off Linux environment. Rows 2-9 run with `-tags pirunner`; with the tag set, missing Node/dependencies or any selected test is a hard verification failure, not a skip, and any `--- SKIP` line fails the row. Retain actual output and subject/image/toolchain hashes. The investigator can publish a truthful no-go/unknown before the behavioral suite is green; such a result requires independent source/case review, not fabricated passing witnesses. GO requires rows 1-9 plus the corpus to pass. Rows 10-12 validate routing, the existing neighbor and the default-CI exclusion, not runtime qualification.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `npm --prefix loopadmin/experiments/pirunner/node run build` | exit 0; pinned TypeScript compiler builds the checked Node bridge; output contains `tsc`; no downloads |
| 2 | check:ci +flow | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotContract$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotContract " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 3 | check:ci +flow | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotRestart$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotRestart " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 4 | check:ci +flow +mutation | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotEffects$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotEffects " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 5 | check:ci +flow +mutation | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotAuthority$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotAuthority " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 6 | check:ci +flow | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotStopAndObserve$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotStopAndObserve " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 7 | check:ci +flow | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotAccounting$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotAccounting " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 8 | check:ci +flow +dereference | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotDecisionEvidence$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotDecisionEvidence " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 9 | check:ci +flow +mutation | `(cd loopadmin && pi_out=$(mktemp) && trap 'rm -f "$pi_out"' 0 && GOWORK=off go test -tags pirunner -count=1 -v -run "^TestPiPilotNegativeControls$" ./experiments/pirunner > "$pi_out" 2>&1 || { cat "$pi_out"; false; } && grep -F -- "--- PASS: TestPiPilotNegativeControls " "$pi_out" && ! grep -F -- "--- SKIP" "$pi_out")` | exit 0; exact named PASS; no `--- SKIP` line; the corresponding corpus group ran against the real experimental path with no absent cases; a failing run prints its retained output |
| 10 | check:ci | `statusgen --consumers --brief agent-execution/10 --base $(git merge-base origin/main HEAD)` | exit 0; implementation routing corroborates; out-of-scope judgments remain reviewer-owned; use retained delivery base after merge |
| 11 | check:ci +neighbour | `(cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance)` | exit 0; output names `github.com/medici-finance/assay/loopadmin/runner/conformance`; existing suite remains green |
| 12 | check:ci +neighbour | `(cd loopadmin && pkgs=$(GOWORK=off go list ./...) && case "$pkgs" in *experiments/pirunner*) false;; *) true;; esac)` | exit 0; the default `loopadmin` package set excludes the tagged experiment, so normal `go test ./...` needs no Node and stays green |

## Pre-mortem and handoff

The failure modes in the corpus map to Verify 2-9; row 8 dereferences pinned evidence and falsifies a false GO, row 9 proves the controls detect plausible wrong code, row 11 protects the unchanged neighbor and row 12 keeps the tagged experiment out of default CI. Review-only judgments: whether an omission materially reduces maintenance, whether the cap remains M-sized and whether the future managed-persistence proposal fits R2. Record their reasons in the report; no fabricated percentage target.

The checked critical-path head is graph-execution/20's existing runner API/kit, not the absent supervisor or graph store. Dependency availability activates nothing. /22 and /26 remain independent; /01 and /03-04 are later qualification consumers rather than prerequisites to this reference experiment. Do not add a hard dependency to those core briefs based on an optional library investigation. At pickup refresh source, open overlaps and exact dependency tuple; record architecture pickup and continue/adapt/hold under the existing implementation-pickup procedure. Keep completed Evidence untouched.

## Evidence

<!-- Appended by an independent verifier: exact command, exit, output/hash, subject, environment and date. No pilot result has been produced by this authoring change. -->

## Review

Gate: model (all risk answers no: offline synthetic investigation, no production control or credential changes). Judge executable recovery evidence, independent lower-boundary refusals, source-backed deletion claims and honest disposition. If implementation would change a production trust boundary, stop and re-author that scope with its derived human gate; this brief grants no such change.
