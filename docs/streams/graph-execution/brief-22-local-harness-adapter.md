---
brief: assay:assay:graph-execution:22
title: First pinned local harness adapter and qualification fixtures
why: A fake runner proves contracts but cannot execute useful model work; a narrow adapter connects one harness
  without multiplying orchestration implementations.
wave: 7
depends:
- graph-execution/20
- graph-execution/26
- desktools-v2/66
unblocks:
- graph-execution/27
- agent-execution/03
effort: L
gate: human
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'yes'
  sensitive-data: 'no'
gate-why: Launches harness processes beside inference credentials; the owner confirms the launcher accepts only approved profile argv with a sanitized environment and that credentials stay confined to the adapter boundary.
issues: []
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- docs/streams/graph-execution/task-workflow-program.md
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'deskcore/writepolicy and writeguard hook profile: follow-up graph-execution/22'
- 'standing desk bindings: follow-up graph-execution/27'
- 'workflow/controller: follow-up graph-execution/21'
- 'tools/desk/cmd/cellctl: follow-up graph-execution/27 (supplies its resolved Claude launch profile)'
- 'adopting cohorts: out-of-scope (live qualification and model choice require owner authorization)'
version: 3
id: 97c5ef2b-7f1c-4d7b-9484-dcfc042926d1
---

# Brief 22 — First pinned local harness adapter and qualification fixtures

## Context

files: `loopadmin/adapters/localharness/` (planned), `loopadmin/testdata/localharness/` (planned), `loopadmin/README.md` (planned), `docs/workflow-runner-qualification.md` (planned), `changelog/graph-execution-22-local-harness-adapter.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The shared loopadmin module is bootstrapped by /20 and must remain usable without the instance store (19). This brief depends on /26: the supervisor and the module's one process runner (moved from `tools/desk/internal/cellprocess` by /26) must already exist, because Go's `internal` visibility rule keeps `loopadmin/` from importing the old package and a copy of the runner or a direct exec site here would sit outside the process-launch audit that /26 extends to `loopadmin/`. At main 307fe1699 cellctl already builds and launches Claude, Codex and Cursor harness argv (`tools/desk/cmd/cellctl/launch.go`, `cadence.go`, `launch_tick.go`, `cursor.go`) through `tools/desk/internal/cellprocess`. This adapter wraps that: its input is the resolved, operator-approved Claude profile cellctl already builds, and it adds no second argv builder or process runner. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the launcher argv/environment boundary — the adapter launches only the supplied operator-approved profile argv with a sanitized environment; behind it, 26's caller-scoped launch endpoint refuses a profile the caller is not bound to, and role credentials are absent from the child domain.

risk-answers: `irreversible: yes`, so `gate: human`, as for 14 and 16 in this stream. The brief is offline-fixture-only — no provider call, no credential held (existing custody is inherited), committed transcripts are scrubbed fixtures, and live qualification is an adopter decision — but the adapter is a process launcher next to inference credentials: once it is implemented and verified, a human check skipped at verification cannot be recovered by a revert. Lowering the gate is a maintainer ruling, not an authoring choice.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Shared write-policy amendment — 18 October 2026

Perform brief-v1 §4.1.1 pickup against the approved target, `docs/go-engineering.md`
and the [write-policy contract](../../write-policy.md). Consume desktools-v2/66's
shared evaluator/profile in the local adapter; keep hook execution as an adapter
where the selected host requires it. This is unaffected by the withdrawn
old CLI-parser migration work (desktools-v2/31). Reuse validated workspace/context
ports; do not duplicate allocation, claim, custody or write-policy meanings.

Include `loopadmin/adapters/localharness/writepolicy_test.go` (planned) and its
module/dependency wiring in files. At pickup settle the available workspace-port
version and supported profile; missing required allocation evidence holds that
profile, not the independent evaluator extraction. Preserve prior source accounts.

## Task

1. Implement the first adapter for a pinned Claude Code harness profile whose input is a resolved, operator-approved argv/env profile of the shape cellctl's existing Claude launch builder produces (`deskLaunch`, `prepareTickLaunch`); cellctl supplies it in 27, and fixtures supply it here. Do not add a second argv builder. Codex and Cursor stay on cellctl's legacy launch path until each has its own adapter qualification. Record exact executable/version, request/session ID, model actually used, tool restrictions and serialization version. Determine supported invocation/output from the pinned installed client at implementation pickup; commit scrubbed fixture transcripts and the capability manifest before claiming support. No hard-coded commercial price or assumed cancellation guarantee.
2. Expose the adapter through the shared loop-admin supervisor /26; do not add a caller-specific process launcher. Use an isolated supervised process with an operator-approved argv/profile, sanitized environment and constrained workspace/tool access. Never execute an argv supplied by task text. The controller store and role credentials are outside the child domain. Inherit existing credential custody; do not implement another vault.
3. Map start/observe/cancel/reconcile to protocol states. When the harness cannot prove an unknown request stopped, retain unknown and require reconciliation/operator resolution. Do not simulate resume by launching a second unknown request. Charge all reported usage to the original attempt/work; missing usage remains unknown.
4. Run the 20 conformance suite against a process fixture plus captured protocol outputs. Publish an offline qualification report; live model calls and additional adapters require an adopting cohort decision. Record control gaps rather than weakening them. Update docs and changelog.

5. Map the selected profile's validated repository/allocation/generation/session and
   resolved path observations into /66's evaluator at the local tool/effect boundary.
   Use typed writes directly where known; preserve pure command classification for
   hook-shaped requests. Refuse absent/stale/foreign mandatory context; no permissive
   cwd/prefix inference or legacy fallback. Keep writeguard as a thin command hook
   for command-hook hosts, never a mandatory subprocess between compatible Go code.
6. Qualify allowed owned file/Git work and refused protected/foreign writes through
   the actual adapter. Then bypass the hook and prove the supported sandbox still
   confines those effects. Preserve existing authorized Git administration effects;
   no blanket denial of all Git common-directory writes and no broad write-root
   widening. Unknown confinement support is could-not-check and blocks that profile.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessConformance$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessConformance " "$routing_out")` | exit 0; named PASS; one fixture process executes a scoped attempt and preserves actual runner/model identity; mutation: record the requested model instead of the actual runner/model identity — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessUnknownCancel$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessUnknownCancel " "$routing_out")` | exit 0; named PASS; lost acknowledgment or unconfirmed cancellation holds instead of relaunching; mutation: treat a cancel request as a confirmed stop — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessUntrustedArgvAndSecrets$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessUntrustedArgvAndSecrets " "$routing_out")` | exit 0; named PASS; task-supplied argv and secret leakage are refused at the launcher boundary, which launches only the supplied approved profile argv; mutation: append task-supplied argv to the approved profile argv — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessBothCallers$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessBothCallers " "$routing_out")` | exit 0; named PASS; desk and workflow clients use the same adapter and pinned model controls; session continuity cannot cross roles and unsupported resume holds in both modes; mutation: key session continuity by caller only, not by role — the named test must fail |
| 5 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestLocalWritePolicy$' ./adapters/localharness > "$out" && grep -q '^--- PASS: TestLocalWritePolicy ' "$out")` | exit 0; actual adapter and shared evaluator agree on independently expected owned/protected writes, owned staged commit and foreign-ref/config refusal; missing/stale/foreign bindings launch no effect; supported real sandbox also refuses with the hook bypassed; unavailable support remains could-not-check. Production mutation skipping the evaluator/binding or broadening write roots must fail. |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.

## Execution-environment follow-on routing — 2026-10-16

Reciprocal unblocks only: agent-execution/03. The [execution amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md) adds a follow-on obligation over this brief's existing output. Its Task, Verify, gate, version and historical Evidence are unchanged; this note grants no delivery, reassignment or activation.
