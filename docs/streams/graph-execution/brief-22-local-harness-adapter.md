---
brief: assay:assay:graph-execution:22
title: Shared local harness adapter with Codex and Claude qualification
why: A fake runner proves contracts but cannot execute useful model work; a narrow adapter qualifies Codex and Claude profiles
  without multiplying orchestration implementations.
wave: 7
depends:
- graph-execution/20
- graph-execution/26
unblocks:
- graph-execution/27
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
- 'standing desk bindings: follow-up graph-execution/27'
- 'workflow/controller: follow-up graph-execution/21'
- 'tools/desk/cmd/cellctl: follow-up graph-execution/27 (supplies its resolved Codex or Claude launch profile)'
- 'adopting cohorts: out-of-scope (live qualification and model choice require owner authorization)'
version: 2
id: 97c5ef2b-7f1c-4d7b-9484-dcfc042926d1
---

# Brief 22 — Shared local harness adapter with Codex and Claude qualification

## Context

files: `loopadmin/adapters/localharness/` (planned), `loopadmin/testdata/localharness/` (planned), `loopadmin/README.md` (planned), `docs/workflow-runner-qualification.md` (planned), `changelog/graph-execution-22-local-harness-adapter.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The shared loopadmin module is bootstrapped by /20 and must remain usable without the instance store (19). This brief depends on /26: the supervisor and the module's one process runner (moved from `tools/desk/internal/cellprocess` by /26) must already exist, because Go's `internal` visibility rule keeps `loopadmin/` from importing the old package and a copy of the runner or a direct exec site here would sit outside the process-launch audit that /26 extends to `loopadmin/`. At main 307fe1699 cellctl already builds and launches Claude, Codex and Cursor harness argv (`tools/desk/cmd/cellctl/launch.go`, `cadence.go`, `launch_tick.go`, `cursor.go`) through `tools/desk/internal/cellprocess`. This adapter wraps that: its input is the resolved, operator-approved Codex or Claude profile cellctl already builds, and it adds no second argv builder or process runner. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

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

## Task

1. Implement one local-harness adapter over the resolved, operator-approved profiles cellctl already produces (`deskLaunch`, `prepareTickLaunch`). Qualify pinned bounded Codex invocations first and pinned Claude invocations through the same adapter; cellctl supplies profiles in 27, and fixtures supply them here. Do not add a second argv builder or provider-specific process runner. Each profile remains legacy until qualified and explicitly enrolled; Cursor remains legacy pending its own qualification. Record exact executable/version, request/session ID, model actually used, tool restrictions and serialization version. Determine supported invocation/output from the pinned installed client at implementation pickup; commit scrubbed fixture transcripts and the capability manifest before claiming support. No hard-coded commercial price or assumed cancellation guarantee.
2. Expose the adapter through the shared loop-admin supervisor /26; do not add a caller-specific process launcher. Use an isolated supervised process with an operator-approved argv/profile, sanitized environment and constrained workspace/tool access. Never execute an argv supplied by task text. The controller store and role credentials are outside the child domain. Inherit existing credential custody; do not implement another vault.
3. Map start/observe/cancel/reconcile to protocol states. When the harness cannot prove an unknown request stopped, retain unknown and require reconciliation/operator resolution. Do not simulate resume by launching a second unknown request. Charge all reported usage to the original attempt/work; missing usage remains unknown.
4. Run the 20 conformance suite against a process fixture plus captured protocol outputs. Publish an offline qualification report; live model calls and adapters beyond these Codex/Claude profiles require an adopting cohort decision. Record control gaps rather than weakening them. Update docs and changelog.

## Bounded-profile acceptance amendment — 2026-10-18

Use the [single delivery contract](task-workflow-program.md#standing-desk-efficiency-delivery--2026-10-18).
Add profile-specific fixtures under the existing `loopadmin/testdata/localharness/`
for Codex and Claude; no new module or scheduling API. Reuse the Codex bounded-tick
argument builder and its Goals-disabled behavior. Do not simulate waiting by ending
and restarting a standing Goal, invent durable child resume, or silently select another
harness. Validate actual profile/executable/version and sanitize reported usage/output;
missing session continuity or model/usage evidence remains unavailable.
Qualify cancellation/reconciliation and bounded completion for each profile separately,
including unsupported resume and process exit without an accepted result. A failing
Codex profile cannot be declared supported because the Claude fixture passes.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessConformance$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessConformance " "$routing_out")` | exit 0; named PASS; one fixture process executes a scoped attempt and preserves actual runner/model identity; mutation: record the requested model instead of the actual runner/model identity — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessUnknownCancel$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessUnknownCancel " "$routing_out")` | exit 0; named PASS; lost acknowledgment or unconfirmed cancellation holds instead of relaunching; mutation: treat a cancel request as a confirmed stop — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessUntrustedArgvAndSecrets$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessUntrustedArgvAndSecrets " "$routing_out")` | exit 0; named PASS; task-supplied argv and secret leakage are refused at the launcher boundary, which launches only the supplied approved profile argv; mutation: append task-supplied argv to the approved profile argv — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessBothCallers$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessBothCallers " "$routing_out")` | exit 0; named PASS; desk and workflow clients use the same adapter and pinned model controls; session continuity cannot cross roles and unsupported resume holds in both modes; mutation: key session continuity by caller only, not by role — the named test must fail |

| 5 | check:ci +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLocalHarnessCodexAndClaudeProfiles$' ./... > "$o" && grep -F -- '--- PASS: TestLocalHarnessCodexAndClaudeProfiles ' "$o")` | exit 0; named PASS. The actual adapter consumes both approved builders and frozen protocol fixtures; neither profile can fall back to the other, and unsupported capabilities hold. Mutation: omit the Codex fixture/profile branch while leaving Claude green; the named test fails. |
| 6 | check:ci +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLocalHarnessCodexBoundedNoGoalLoop$' ./... > "$o" && grep -F -- '--- PASS: TestLocalHarnessCodexBoundedNoGoalLoop ' "$o")` | exit 0; named PASS. Codex fixture argv retain Goals-disabled bounded execution and completion receipts; repeated idle observation starts no process. Mutation: restore automatic Goal continuation for the bounded profile; the named test fails. |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
