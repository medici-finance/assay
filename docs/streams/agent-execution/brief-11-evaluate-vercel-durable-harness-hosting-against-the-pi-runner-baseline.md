---
brief: assay:assay:agent-execution:11
title: Evaluate Vercel durable-harness hosting against the Pi runner baseline
why: Durable hosting may let us omit session wake, continuation and competing-writer machinery. Compare Vercel's implementation with the same Pi workload before adding another store, queue or driver to the planned runtime.
wave: 2
depends: ["agent-execution/10"]
unblocks: []
effort: M
gate: model
risk: {regulatory: "no", customer: "no", irreversible: "no", sensitive-data: "no"}
issues: []
schema: brief-v2
version: 1
id: b781b25e-95bd-48c5-b16b-4c47fce865d5
authored: 2026-10-22 by runtime evaluation authoring session
sources:
- docs/proposals/non-ci-architecture/27-agent-execution-environments.md
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md
- docs/proposals/non-ci-architecture/11-target-architecture.md
- docs/streams/agent-execution/brief-10-evaluate-a-pi-durable-runner-with-offline-recovery-fixtures.md
- spec/loop-admin-runner-v1.md
- "freshness-checked 2026-10-22 @ 1cce37af3 (origin/main); authoring stacked on Pi brief PR #4138 @ 4b428532e7c00ef348aff953a386786db0e6d1f3"
- "Vercel durable-harness freshly fetched main fb056a3c08549fdad9b87ed8358a27e2097ded11: public Pi adapter, World storage/anchors, recovery tests and queue-handover design"
exec-tier: strong
exec-tier-why: Cross-process ownership, queue redelivery, independent external-effect fencing and comparative evidence require concurrency and boundary reasoning.
domain: complicated
outcome: none
consumers:
- "loopadmin/experiments/vercelharness/: follow-up agent-execution/11 (this brief; offline World-backed runner experiment)"
- "loopadmin/testdata/vercelharness/: follow-up agent-execution/11 (this brief; pinned sources, cases, controls and comparison evidence)"
- "docs/reports/agent-execution/vercel-durable-harness-evaluation.md: follow-up agent-execution/11 (this brief; decision and deletion map)"
- "loopadmin/README.md: follow-up agent-execution/11 (this brief; opt-in offline invocation)"
- "ci/staged-workflows/loopadmin.yml: follow-up agent-execution/11 (this brief; staged network-off test coverage)"
- "loopadmin/experiments/pirunner/: out-of-scope (consume agent-execution/10's comparator and fixtures unchanged; any missing reusable seam is reported, not replaced with a second Pi baseline)"
- "loopadmin/runner, supervisor and localharness: out-of-scope (existing graph-execution/20, /26 and /22 retain contracts and production launch ownership)"
- "managed PostgreSQL, provider executors and Cell API: out-of-scope (R2/R4/R7 and agent-execution/03-04 retain integration and qualification)"
---

# Brief 11 — Evaluate Vercel durable-harness hosting against the Pi runner baseline

## Context

files: `loopadmin/experiments/vercelharness/adapter.go` (planned), `loopadmin/experiments/vercelharness/evaluation_test.go` (planned), `loopadmin/experiments/vercelharness/node/` (planned), `loopadmin/testdata/vercelharness/` (planned), `docs/reports/agent-execution/vercel-durable-harness-evaluation.md` (planned), `loopadmin/README.md`, `ci/staged-workflows/loopadmin.yml`, `changelog/agent-execution-11-vercel-evaluation.md` (planned). These are future implementation deliverables. No live workflow, production default, core runner schema, deployment profile or credential configuration changes here.

single-point-of-failure: assuming World session-writer fencing authorizes every resumed operation — backed by a separately controlled synthetic executor that independently rejects stale effect authority. Exercise it with the upper session/adapter check bypassed. No production trust boundary is qualified by these fixtures.

facts:
- Fresh public main `1cce37af3`, inspected 2026-10-22, retains the existing `loopadmin/runner` contract/conformance kit and no implemented `loopadmin/supervisor/` or `workflow/`. The target source/runner paths do not differ from the Pi authoring base; /10 is pending in PR #4138 rather than merged. This authoring branch is stacked on that public PR; it does not implement or activate either experiment.
- Pi pilot /10 consumes graph-execution/20, making /11 wave 2. /10 owns the plain-Pi adapter, real runner conformance, scripted models and fault workload. Its optional Vercel evidence, if already produced, is reusable input rather than work to repeat. A completed no-go/unknown report is not an available successful baseline by implication.
- Freshly fetched Vercel main `fb056a3c08549fdad9b87ed8358a27e2097ded11` declares durable-harness 0.0.1-beta.6, experimental. Public `durable-harness/pi-durable` exposes `openWorldStorage`, `resumeWith`, `openWorldReader` and `acquireHarness`; `pi-world`/`harness-world` are private workspace implementation packages, not supported standalone imports.
- The inspected Pi World backend commits to a World log, reconstructs session state from that history, and uses queue/deadline anchors plus storage-writer fencing. Its notes describe a round trip per commit and full-log reopen without built snapshots. Local World recovers after a host restart; this does not prove hosted redelivery latency, idle compute release or PostgreSQL availability.
- The public package advertises a Postgres World, but a shared database engine does not create shared transactions with Assay's admission/effect ledger. Public Claude/Codex adapters require Vercel Sandbox at the pinned revision; local/Postgres World support does not imply those adapters run in our Kubernetes Pods. The queue design names send-and-hold as in progress, not a shipped assumption.

layering: Reuse /10's runner-client/corpus boundary and add one experiment-only adapter for the public Vercel Pi entry point. One World host owns inner-session continuation; the test caller owns admission and synthetic authority, and its separate executor owns synthetic effects. Node is justified by the upstream dependency and process-kill lifecycle; Go remains the existing protocol client. No production supervisor, generic service or new core protocol is introduced. Task 2 and Verify 2/4 prove separation rather than copy authority into the World log.

design-fit:
  owner: existing loopadmin/runner owns invocation semantics; this experiment owns only World-host integration and comparative observations
  contract: none — consume spec/loop-admin-runner-v1.md unchanged, with existing R2/R4 authority contracts; no new semantic-owner row is needed for an isolated investigation
  retires: []
  weight: "verbs +0, flags +0, production refusals +0, production rule-text lines +0; one opt-in experiment with a lockfile and decision report"
  why-add: n/a for those dimensions; the bounded package compares a reusable upstream owner instead of building parallel session-hosting machinery

## Read first

- [Pi pilot /10](brief-10-evaluate-a-pi-durable-runner-with-offline-recovery-fixtures.md), [stream approval boundary](README.md), [existing runner specification](../../../spec/loop-admin-runner-v1.md) and `loopadmin/runner/{adapter,client,request,result,usage}.go`.
- [Portable environment target](../../proposals/non-ci-architecture/27-agent-execution-environments.md), [storage and restore](../../proposals/non-ci-architecture/14-storage-forge-graph.md), [whole-system target](../../proposals/non-ci-architecture/11-target-architecture.md) and [runtime alignment](../../proposals/non-ci-architecture/23-runtime-contract-alignment.md).
- [Local adapter /22](../graph-execution/brief-22-local-harness-adapter.md), [shared supervisor /26](../graph-execution/brief-26-shared-loop-admin.md), [profile /01](brief-01-qualified-execution-profile.md), [readiness /03](brief-03-local-readiness-enforcement.md) and [integrated recovery /04](brief-04-recovery-effect-qualification.md).
- [Library-first](../../library-first.md), [Go engineering](../../go-engineering.md) and [regression floor](../../proposals/non-ci-architecture/15-regression-from-day-one.md).
- At Vercel revision `fb056a3c08549fdad9b87ed8358a27e2097ded11`: [public Pi adapter](https://github.com/vercel-labs/durable-harness/blob/fb056a3c08549fdad9b87ed8358a27e2097ded11/packages/durable-harness/src/pi-durable.ts), [World implementation notes](https://github.com/vercel-labs/durable-harness/blob/fb056a3c08549fdad9b87ed8358a27e2097ded11/packages/pi-world/README.md), [queue handover](https://github.com/vercel-labs/durable-harness/blob/fb056a3c08549fdad9b87ed8358a27e2097ded11/docs/queue-send-and-hold.md), [writer fencing tests](https://github.com/vercel-labs/durable-harness/blob/fb056a3c08549fdad9b87ed8358a27e2097ded11/packages/pi-world/src/fence.test.ts), [SIGKILL test](https://github.com/vercel-labs/durable-harness/blob/fb056a3c08549fdad9b87ed8358a27e2097ded11/packages/durable-harness/test/pi-durable-sigkill.test.ts), and the public `src/{claude-code,codex,world,workspace}.ts` modules.

## Scope, prerequisites and limits

- Offline, synthetic, one role/profile; no model/provider credentials, external endpoints, hosted Vercel deployment, Vercel Sandbox, cluster, new database service, real provider writes, managed activation or workflow promotion. An explicitly configured isolated loopback fixture is allowed only if the local World requires it; bind and record its target before contact. No ambient endpoint or credential discovery.
- Pre-stage vetted exact Node/Pi/Vercel/World/compiler versions, integrity-locked dependencies and source fixtures. Build/run the evidence corpus in the approved network-off Linux environment, without secrets, host configuration or Docker socket access. Missing prerequisites stay could-not-check; no test-time install, network fallback or skip-as-pass.
- Five engineering days from implementation pickup. At the cap publish the actual evidence and go/no-go/unknown disposition; do not expand to a Postgres storage implementation, another controller, broad harness benchmark or production integration. M sizing depends on reusing /10 rather than rebuilding it.
- Start the executed comparison only when /10 supplies a source-bound usable baseline and corpus. Completion of /10 is the actual first implementation dependency; absence of the production supervisor is not permission to build one here. If /10 yields no-go/unknown, determine whether it prevents a matched comparison. Source/portability analysis can still proceed, but unavailable baseline behavior cannot be reported as measured.
- Keep the stream parked until its controlling amendment is approved; preserve all later adoption gates. Stop at implemented; verification belongs to a non-implementer. No current production code is deleted in this brief.

## Task

1. **Bind the comparator and dependency tuple.** Reuse /10's accepted source/input manifest, scripted model/tool workload, stable invocation mapping, authority/effect ledger and result vocabulary. If /10's optional Vercel arm already produced applicable evidence, reference its immutable hashes and rerun only uncovered/changed cases. Vercel's development Pi pin is 1.0.4 at the source revision; a newer Pi tuple is a separate compatibility observation. If both arms cannot use the same tuple, first establish a matching plain-Pi baseline under that tuple; otherwise label the comparison inconclusive. No performance claim across simultaneous harness/version/model changes. Record architecture pickup against current source, approval state, supported APIs and active work.
2. **Run the public integration, not a lookalike.** Implement a `runner.Adapter` experiment around `durable-harness/pi-durable` using its public storage/opener/reader APIs and local World. Supply faux models explicitly; never invoke gateway defaults. Reuse /10's bridge and fixtures by their actual supported import/build boundary; if extraction is needed, report the missing seam and seek a bounded amendment rather than duplicate Pi. Exercise the real `runner.Client` and exported conformance kit with fault control driving actual Node/Pi/World behavior. Keep unsupported capabilities explicit. One configured World driver is the sole inner continuation owner; the caller retains start intent, current admission, stop, budget and work identity. No parallel custom heartbeat/retry driver to make World tests pass.
3. **Exercise crash gaps and competing owners.** Bind `(caller, invocation, generation)` to the same World session and Pi submission; new queue/tool IDs cannot create new work/effect IDs. Kill after admission but before the World receipt, after submission, during safe and unsafe tools, and while continuation/background work remains. Redeliver duplicates and reorder fixture notifications; reconcile before relaunch. Freeze A, let the selected backend establish takeover, resume A and observe its actual log writes refused or excluded from accepted replay. Record the backend's fence capability; absent support is a visible limitation, not a fake injected StorageFenced result. Independently refuse stale effects even if session checks are bypassed. Do not infer physical process stop or cancellation from log fencing or queue ownership alone.
4. **Keep authority outside the log.** Check current owner/incarnation, pause and budget before an actual model/tool execution, including recovered safe tools that skip the original beforeTool hook. Resume under a changed policy and unknown authority must hold. Simulate the provider-applied/reply-lost case through the independent synthetic executor: hold/reconcile one logical effect even when a model requests it again under another tool ID. Simulate failure on each side of caller admission/World start to expose the lack of an atomic cross-store transaction. Database or log restoration cannot renew an old approval, effect permission or spending allowance.
5. **Observe without driving.** Use the public read-only reader/snapshots for reconnect and inspector tests. Assert zero new launch/model calls and no owner fencing from observation. Scope synthetic callers to their own session; cross-session reads are refused by the experiment adapter's existing caller boundary. This tests the adapter, not the full Cell API or Vercel's application authentication. Observe cancellation with background children and a silent tool exceeding a configured driver deadline; confirmation requires actual stop evidence. Keep generic chat-route timing separate from Pi storage/backstop timing and test only shipped public controls of the selected path; a planned send-and-hold API cannot satisfy a case.
6. **Compare equivalent work and classify portability.** With the same tuple/script/candidate and 100/1,000 tool-result-entry fixtures from /10, record successful and interrupted attempts, launches, duplicate effects, commit/queue counts, bytes, reopen/replay time, recovery delay, active/wait duration where observable, peak-memory coverage and incomplete usage. Include failed/abandoned work, Node/World startup, artifact copying and test-driver effort. Report active-versus-idle unknown where no reliable source exists; a persisted wait is not proof of zero idle compute. Build a source-backed compatibility map for local World, Postgres World and hosted Vercel, including queue delivery, lock/fence support, process lifetime, schema/migration/retention and restore. Only local World is executed here; all other rows remain source-inspected or unknown. Explicitly retain Assay's PostgreSQL transactions and cross-store reconciliation requirement. Separate Pi virtual files from real nontransactional shell/provider effects, and record the public Claude/Codex Sandbox restriction.
7. **Falsify the integration claims.** Add test-owned negative controls that mutate the actual adapter/bridge and rebuild, reusing /10's control approach and existing mutation facilities where compatible. Removing writer-fence handling, continuation reauthorization, unknown-outcome retention, observer non-driving behavior, stable effect identity or unknown-usage propagation must fail the named behavioral test. Build/import failures, missing dependencies, skip/zero-case selection and test timeout are not kills. Retain a positive baseline and exact failing assertion per control.
8. **Publish a decision and handoff.** Deliver the report plus `source-manifest.json`, `cases.json`, `results.json`, comparison records and negative-control map under the named fixture directory. Bind every measured/classified claim to subject/dependency/source hashes, actual commands/exits, workload and coverage. Provide the deletion/omission map below with exact present/planned paths and ongoing integration/upgrade burdens. Add only opt-in experiment instructions and staged offline CI coverage. A favorable result is optional input to /22, /26, /01 and /03-04, not a new blocker for them. Any managed storage/profile adoption is separately authored through those owners.

## Expected reuse, omissions and retained owners

| Mechanism | Hypothesis to test | Retained obligation |
|---|---|---|
| Session wake/backstop and continuation handling | Delegate to World queue/anchors instead of writing another Pi-session wake loop | Assay admission, role cadence and outer launch journal remain /26-27; no two drivers |
| Competing session-writer detection | Reuse World-backed fencing instead of a separate session-log lock implementation | Existing cell/repository ownership, operator incarnation and independent executor fencing remain |
| Session journal/reopen and execution views | Reuse World log and public read-only reader; omit duplicate session-host journal/inspector paths | Assay work/effect records and authorized Cell API remain independent; full-log replay/retention costs must be recorded |
| Host-specific harness wrappers | Borrow pinned builds, host-held model credentials and dedicated execution environment patterns | No claim of local Kubernetes reuse for public Claude/Codex adapters; no credentials introduced by this experiment |
| Cell controller, PostgreSQL operational ledger, provider reconciliation, independent evidence/acceptance | Keep | No-go if replacing/weakening these meanings is required for the advertised benefit |

No production deletions occur here. Distinguish already-existing code from planned code we could avoid. Identify at least one precise omission/removal for a positive simplification result; count the new World schema/queue/driver/dependency burden alongside it. Copying upstream internals or retaining parallel recovery drivers is not a deletion.

## Mandatory corpus and decision criteria

| Planned group | Required discriminating observation | Wrong implementation to reject |
|---|---|---|
| `TestVercelEvaluationContract` | Real Client/conformance flow, correct role/profile/capabilities and stable session/submission mapping | Kit runs against a fake instead of the selected public integration |
| `TestVercelEvaluationRecovery` | Real process kill/restart, lost start receipt, duplicate/redelivered wakes, background continuation, safe/unsafe tool recovery | Fresh invocation/effect allowance on redelivery or unsafe blind retry |
| `TestVercelEvaluationFencing` | Competing real writers and frozen holder; stale replay is not accepted; lower executor rejects with upper check bypassed | Session lock treated as global effect authority or artificial success standing in for backend fencing |
| `TestVercelEvaluationAuthorityEffects` | Pause/budget/incarnation change between intent and replay; caller/World crash gaps; provider-applied/lost-reply reconciles one effect | beforeTool-only authorization, fresh authority from restored log, or duplicate effect under new tool ID |
| `TestVercelEvaluationObserverStop` | Read-only attach/reconnect starts nothing; scoped observer; silent tool and ignored cancellation/background work remain correctly held until stopped | Observation opens a driver or timeout/lease loss means physical stop |
| `TestVercelEvaluationComparison` | Matching plain-Pi tuple/inputs, fixed-size replay measurements, retry/child accounting, explicit incomplete cost/idle data | Faster result uses different work, hides failed attempts or reports unknown as zero |
| `TestVercelEvaluationEvidence` | Report verdict dereferences exact source/workload/case hashes and backend capability coverage | Local test promoted to Postgres/hosted/Kubernetes qualification or planned queue feature treated as shipped |
| `TestVercelEvaluationNegativeControls` | Positive baseline plus actual rebuilt mutants with named assertion failures; false kills rejected | Build, missing runtime, skip or timeout counted as proving the control |

- **GO — candidate for a separately scoped integration:** all mandatory local cases have observed expected outcomes, controls discriminate, a matched plain-Pi baseline exists, and at least one specific hosting mechanism can be omitted without a second launch/coordination authority. Record the full added operating surface and an R2-compatible managed-persistence follow-on. This is local feasibility only; not production readiness, cost savings or approval to deploy.
- **NO-GO for this integration/simplification hypothesis:** repeatable duplicate effects or stale-authority execution, unknown stop/outcome presented as success, unsupported mandatory behavior silently downgraded, required private-package fork, dependence on changing R2/R4/acceptance ownership, or no credible code omission once replacement burden is counted. Correct refusal can establish an incompatibility; do not change controls to obtain a favorable verdict.
- **UNKNOWN:** missing compatible tuple/baseline, backend fence evidence, test isolation or case/usage coverage prevents a conclusion within the cap. Missing Postgres/hosted execution is always disclosed, even if local feasibility is GO. Baseline/version confounding cannot support an incremental benefit claim.

Record **local safety/compatibility**, **incremental simplification**, and **managed portability** separately before the overall disposition. A local GO cannot overwrite managed unknown/no-go. The evidence classifier must reject fabricated GO from failed, missing, mismatched or source-only cases. A truthful no-go/unknown report may finish an investigation under independent review; it cannot create a passing runtime witness for an unrun/failing row.

## Verify

All new tests/paths below are implementation deliverables, not evidence from authoring. Run at repository root in the pre-staged network-off Linux environment. Missing build/runtime/test selection must fail; do not skip. Record commands, exits, subject/image/dependency hashes, counts and unknowns. GO requires rows 1-9 and the mandatory corpus to pass; a reported no-go/unknown preserves actual failures/unrun coverage for independent review. Row 11 is an unchanged-neighbor check, not execution of the Pi pilot.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `npm --prefix loopadmin/experiments/vercelharness/node run build` | exit 0; pinned compiler builds the checked public-API bridge; output contains `tsc`; no downloads |
| 2 | check:ci +flow | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationContract$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationContract " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 3 | check:ci +flow | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationRecovery$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationRecovery " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 4 | check:ci +flow +mutation | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationFencing$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationFencing " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 5 | check:ci +flow +mutation | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationAuthorityEffects$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationAuthorityEffects " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 6 | check:ci +flow | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationObserverStop$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationObserverStop " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 7 | check:ci +flow | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationComparison$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationComparison " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 8 | check:ci +flow +dereference | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationEvidence$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationEvidence " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 9 | check:ci +flow +mutation | `(cd loopadmin && vercel_out=$(mktemp) && trap 'rm -f "$vercel_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVercelEvaluationNegativeControls$" ./experiments/vercelharness > "$vercel_out" 2>&1 && grep -F -- "--- PASS: TestVercelEvaluationNegativeControls " "$vercel_out")` | exit 0; exact named PASS; corresponding corpus group exercised the real public integration; no absent/skipped cases |
| 10 | check:ci | `statusgen --consumers --brief agent-execution/11 --base $(git merge-base origin/main HEAD)` | exit 0; implementation routing corroborated; out-of-scope judgments remain reviewer-owned; retain delivery base for post-merge verification |
| 11 | check:ci +neighbour | `(cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance)` | exit 0; output names `github.com/medici-finance/assay/loopadmin/runner/conformance`; existing suite remains green |

## Pre-mortem, dependency routing and implementation pickup

The corpus's plausible wrong deliveries map to Verify 2-9: wrong baseline, duplicate driver/effect, stale writer, observer-induced takeover, hidden waiting cost, fabricated portability and false mutation kills. Review-only: materiality of the claimed code omission, sufficiency of the PostgreSQL follow-on and whether the five-day cap remains M-sized. Record those judgments with reasons rather than a universal percentage target.

The critical path is graph-execution/20 -> agent-execution/10's independently verified reusable baseline -> /11. Its authoring parent PR is not implementation evidence. If the Pi pilot does not land or cannot produce a comparable path, hold the affected executed comparison and report the precise dependency; do not supply a parallel Pi baseline under this brief. Reuse applicable /10 optional-arm results without double counting. The stream's parked approval gate and source freshness remain separate from code prerequisites. /22 and /26 retain their own paths and need not wait for this optional evaluation. Current-source, active-owner and contract pickup/recheck is mandatory; record continue/adapt/hold before implementation and after material changes. Preserve completed Evidence and existing activation gates.

## Evidence

<!-- Independent verifier records actual command, exit, subject/environment/date and output/hash. This authoring change runs no Vercel evaluation and claims no new runtime outcome. -->

## Review

Gate: model (offline synthetic evaluation, no production credential, control or deployment change). Review source-backed portability, independent lower-boundary refusals, matched comparator and honest deletion/decision evidence. Any production trust-boundary change needs a separately derived gate and reviewed adoption scope.
