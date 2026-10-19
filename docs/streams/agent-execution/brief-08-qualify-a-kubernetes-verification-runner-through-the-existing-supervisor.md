---
brief: assay:assay:agent-execution:08
title: Qualify a Kubernetes verification runner through the existing supervisor
why: "A Kubernetes runner must preserve the existing attempt, evidence and recovery contract so moving execution off a desk does not create another scheduler or publisher."
wave: 12
depends: ["agent-execution/03", "graph-execution/21", "graph-execution/25"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "A Kubernetes runner must preserve the existing attempt, evidence and recovery contract so moving execution off a desk does not create another scheduler or publisher."
issues: []
schema: brief-v2
version: 1
authored: 2026-10-19 by statusgen newbrief
sources:
  - docs/streams/graph-execution/verifier-migration.md
  - docs/proposals/non-ci-architecture/27-agent-execution-environments.md
  - freshness-checked 2026-10-19 @ 2b2383f8e (origin/main)
exec-tier: strong
exec-tier-why: "Cross-process ownership, unknown API outcomes and containment must agree."
domain: complicated
decision-trigger: spec
id: c387db36-88f5-484d-b2ac-3f9930b59734
consumers:
  - "loopadmin/supervisor: fixed-here"
  - "workflow/controller: fixed-here"
  - "operator deployment: out-of-scope (separate profile activation)"
---

# Brief 08 — Qualify a Kubernetes verification runner through the existing supervisor

## Context

files: `loopadmin/adapters/kubernetes/` (planned), `loopadmin/adapters/kubernetes/testdata/` (planned), `loopadmin/supervisor/` (planned), `workflow/controller/` (planned), `docs/kubernetes-verifier-profile.md` (planned), `changelog/kubernetes-verifier-profile.md` (planned).

facts: `loopadmin/runner` and its conformance kit exist at main `2b2383f8e`; the production
supervisor/controller prerequisites are not qualified by this brief. The execution stream
is parked pending its controlling amendment. Implementation dispatch waits for approval/admission of the controlling execution stream; authoring and review of this plan may proceed while it is parked. Before implementation, /03 must provide qualified-profile/readiness enforcement, /21 the durable controller and /25 the current-authority publication boundary. These outputs are required, not merely file names.

This new profile does not replace /22's local
harness adapter, create another process runner or require Kubernetes for interim verification.

single-point-of-failure: result-to-attempt binding at the adapter; independent controller
current-generation checks and separately enforcing publisher credentials prevent a stale
or forged runner result from authorizing publication even when that adapter is bypassed.

## Read first

- [Execution environment scope](../../proposals/non-ci-architecture/27-agent-execution-environments.md).
- [Verifier state, queue and authority contract](../graph-execution/verifier-migration.md).
- [Runner protocol](../../../spec/loop-admin-runner-v1.md).
- [Local readiness enforcement](brief-03-local-readiness-enforcement.md).

## Human decision

Decision-trigger: spec. Recommended: one namespace-scoped verification executor profile,
digest-pinned images and credential-free candidate Jobs, with only the existing supervisor
holding the narrow Kubernetes API capability. Alternative: retain the qualified local runner
until an adequate isolation/recovery profile is available. Before activation, the operator
selects concrete cluster/namespace, API permissions, service accounts, effective network and
runtime isolation, storage retention and recovery objectives, after reviewing the disposable
qualification evidence. No response means no activation. No permission is inferred from a
manifest, API success or a published verdict. This brief does not choose a production target.

## Task

1. Implement a runner-protocol adapter through the existing supervisor. Use immutable
   invocation/attempt/generation and profile inputs; fixed digest, namespace, mounts and
   capability allowlist; no arbitrary caller-selected service account or command profile.
   Candidate Jobs receive no operator, forge-publisher or default service-account credentials.
   Fail before creation on unsupported required isolation, unavailable profile or stale receipt.
2. Use /19's existing launch/effect records via the supervisor, with stable Job identity.
   On create timeout reconcile the admitted Job before another create; distinguish definite
   failure from unknown. Consume completion/result only at the current attempt/generation.
   Duplicate completion is harmless; a replacement Pod cannot create another logical verdict.
3. Route stop, deadline, drain and cancellation through the existing control contract. A delete
   request is not proof a Job stopped; retain an unknown/reconciling state until observation
   establishes it. Keep evidence retention and resource retirement separate from acceptance.
   Never put the acceptance policy, controller state or publication capability in a Job.
4. Connect the production adapter to the production supervisor/controller using an injected
   fake Kubernetes API and hostile fixtures. No live cluster is contacted. Run runner
   conformance and the named tests below. Document the later disposable-cluster qualification
   procedure: actual containment, duplicate execution, node loss, API outage, credential
   denial, output retention and safe cancellation. Offline results alone do not qualify
   Kubernetes isolation or permit migration; real-profile evidence is a separate adoption gate.

## Interface contract

Existing runner v1 invocation enters; attributed result, status and recovery observations
leave. PostgreSQL retains coordination; Kubernetes Job status never grants acceptance.
The workflow controller owns admission, stage progression and acceptance; /19 owns PostgreSQL transactions. The existing supervisor owns only process/activity execution, not work coordination or acceptance. Update the qualification documentation and changelog.
No new scheduler, queue store, public CLI or credential-minting service.

## Verify

These named tests are implementation deliverables, not executed authoring evidence. Each
must call the production adapter; expected results are fixed independently of its output.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeRunnerContract$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeRunnerContract " "$out")` | exit 0; named PASS; production adapter passes v1 conformance; unsupported mandatory profile refuses; mutation: ignore required isolation — test fails |
| 2 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeLostCreate$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeLostCreate " "$out")` | exit 0; named PASS; lost create acknowledgment reconciles one logical Job; duplicate and stale completion cannot advance; mutation: mint a new Job on timeout — test fails |
| 3 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeCredentialFence$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeCredentialFence " "$out")` | exit 0; named PASS; no publisher/operator or default API credentials in candidate Jobs, including direct adapter invocation; mutation: accept caller service account — test fails |
| 4 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeCancelRecovery$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeCancelRecovery " "$out")` | exit 0; named PASS; crash, cancellation and pending delete preserve reconciliation/evidence/retirement; mutation: treat delete request as completed retirement — test fails |
| 5 | check:ci +flow +mutation | `(cd workflow && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeControllerSeam$" ./controller/ > "$out" && grep -Fq -- "--- PASS: TestKubeControllerSeam " "$out")` | exit 0; named PASS; production supervisor/controller consumes real adapter observations through the injected API; process success without acceptance remains unaccepted; mutation: advance on Job success — test fails |

## Pre-mortem

A fake API cannot prove network isolation, scheduler semantics or credential custody in a
real cluster. Those remain explicit unqualified adoption obligations. The executable rows
cover production adapter decisions; independent review assesses containment design and the
operator owns the real-profile evidence. Keep actual API outcomes distinct from model advice.

## Evidence

<!-- Non-implementer records exact commands, revision, environment, results and limits. -->

## Review

Human gate. Confirm profile boundary, lower-layer refusals and explicit deployment exclusion.
