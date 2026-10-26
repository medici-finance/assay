---
brief: assay:assay:agent-execution:08
title: Qualify a Kubernetes verification runner through the existing supervisor
why: "A Kubernetes runner must preserve the existing attempt, evidence and recovery contract so moving execution off a desk does not create another scheduler or publisher."
wave: 12
depends: ["agent-execution/03", "graph-execution/21", "graph-execution/25", "agent-execution/09"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "A wrong containment or credential profile can expose cluster authority and leave external execution unresolved; the human gate approves the boundary, and activation needs separate real-profile evidence."
issues: []
schema: brief-v2
outcome: "none"
version: 3
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
supervisor/controller prerequisites are not qualified by this brief. At that authoring revision, the execution stream
was parked. Admission of amendment 27 and the stream for implementation is proposed by
[PR #4349](https://github.com/medici-finance/assay/pull/4349) and takes effect on the
maintainer's merge of it; this supplies no profile activation or deployment approval. Before implementation, /03 must provide qualified-profile/readiness enforcement, /21 the durable controller and /25 the current-authority publication boundary. These outputs are required, not merely file names.

This new profile does not replace /22's local
harness adapter, create another process runner or require Kubernetes for interim verification.

single-point-of-failure: result-to-attempt binding at the adapter; independent controller
current-generation checks and separately enforcing publisher credentials prevent a stale
or forged runner result from authorizing publication even when that adapter is bypassed.

layering: Task 1–4 keep runner semantics in the existing supervisor/runner contract and isolate Kubernetes effects in one adapter; Verify 1–8 exercise that seam and its independent credential/containment refusals. Controller acceptance and publication remain separate owners.

design-fit:
  owner: loopadmin (existing runner/supervisor execution contract; the adapter owns only Kubernetes translation)
  contract: none — docs/contracts.md has no semantic-owner row for the planned Kubernetes execution adapter; spec/loop-admin-runner-v1.md and graph-execution/26 remain the inherited execution contract
  retires: []
  weight: verbs +0, flags +0, refusal classes +6 (profile, image, pod floor, network floor, credential source, stale attempt), rule-text lines +80 budget; implementation records the actual delta
  why-add: Kubernetes API effects cannot live in the pure runner protocol or the workflow acceptance owner. Extend the existing supervisor with one adapter; retaining only the local adapter was considered but cannot execute the requested Kubernetes profile. No replacement scheduler or publisher is added.

## Read first

- [Execution environment scope](../../proposals/non-ci-architecture/27-agent-execution-environments.md).
- [Verifier state, queue and authority contract](../graph-execution/verifier-migration.md).
- [Runner protocol](../../../spec/loop-admin-runner-v1.md).
- [Local readiness enforcement](brief-03-local-readiness-enforcement.md).

## Human decision

Decision-trigger: spec

<!-- Pickup-time guidance; the executor authors the decision after the design step.
Recommended: one namespace-scoped verification executor profile,
digest-pinned images and credential-free candidate Jobs, with only the existing supervisor
holding the narrow Kubernetes API capability. Alternative: retain the qualified local runner
until an adequate isolation/recovery profile is available. Before activation, the operator
selects concrete cluster/namespace, API permissions, service accounts, effective network and
runtime isolation, storage retention and recovery objectives, after reviewing the disposable
qualification evidence. No response means no activation. No permission is inferred from a
manifest, API success or a published verdict. This brief does not choose a production target.
-->

## Task

1. Implement a runner-protocol adapter through the existing supervisor. Use immutable
   invocation/attempt/generation and profile inputs; fixed digest, namespace, mounts and
   capability allowlist; no arbitrary caller-selected service account or command profile.
   Candidate Jobs receive no operator, forge-publisher or default service-account credentials.
   Fail before creation on unsupported required isolation, unavailable profile or stale receipt.
   The submitted Pod spec has a non-negotiable floor: privileged=false,
   allowPrivilegeEscalation=false, runAsNonRoot=true, all Linux capabilities dropped with
   none added, seccomp RuntimeDefault (or a stricter qualified profile), readOnlyRootFilesystem=true,
   hostNetwork=false, hostPID=false and hostIPC=false, no hostPath and no default service-account token.
   Apply the floor to every regular, init and any permitted ephemeral container; reject
   unrecognized injected container/mount/profile fields. Writable scratch/output uses only
   declared bounded volumes, never a writable root or node mount. Unsupported workloads hold.
   Require a digest-qualified image; a mutable tag or malformed digest refuses before creation.
   Require a profile-bound default-deny ingress/egress policy reference and an explicit allowlist.
   The allowlist excludes node/cloud metadata, Kubernetes API, coordination database and forge;
   candidate network access is off by default. Missing policy, stale qualification or declared
   unavailable enforcement refuses before creation. An offline spec check is not proof that
   the cluster enforces the policy; real containment remains the adoption gate below.
   The supervisor receives only an explicit profile-bound credential reference and target;
   never load ambient kubeconfig or discover in-cluster credentials as a fallback. No explicit
   credential, target or opt-in means no API call. Print the non-secret target before first
   contact. Namespace-scoped permissions allow jobs create/get/list/watch/delete and pods
   get/list/watch plus pods/log get only; no secrets, pods/exec, RBAC, cluster-wide or wildcard
   permissions. Credential material stays in the supervisor's execution boundary and never
   enters Job specs, artifacts or logs. Broader capability requires a separate reviewed profile,
   not a caller-selected override of this one.
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
| 1 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeRunnerContract$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeRunnerContract " "$out")` | exit 0; named PASS; production adapter passes v1 conformance; unsupported mandatory profile and tag-only/malformed image references refuse before any create; mutations: ignore required isolation or accept a mutable image tag — test fails |
| 2 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeLostCreate$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeLostCreate " "$out")` | exit 0; named PASS; lost create acknowledgment reconciles one logical Job; duplicate and stale completion cannot advance; mutation: mint a new Job on timeout — test fails |
| 3 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeCredentialFence$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeCredentialFence " "$out")` | exit 0; named PASS; no publisher/operator or default API credentials in candidate Jobs, including direct adapter invocation; mutation: accept caller service account — test fails |
| 4 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeCancelRecovery$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeCancelRecovery " "$out")` | exit 0; named PASS; crash, cancellation and pending delete preserve reconciliation/evidence/retirement; mutation: treat delete request as completed retirement — test fails |
| 5 | check:ci +flow +mutation | `(cd workflow && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeControllerSeam$" ./controller/ > "$out" && grep -Fq -- "--- PASS: TestKubeControllerSeam " "$out")` | exit 0; named PASS; production supervisor/controller consumes real adapter observations through the injected API; process success without acceptance remains unaccepted; mutation: advance on Job success — test fails |
| 6 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubePodSecurityFloor$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubePodSecurityFloor " "$out")` | exit 0; named PASS; inspect every submitted container and volume in the fake API; privileged/escalation/root/host namespace/hostPath/capability-add/writable-root cases refuse; mutation: accept a hostPath or privileged container — test fails |
| 7 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeNetworkFloor$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeNetworkFloor " "$out")` | exit 0; named PASS; missing/default-allow/stale/unavailable network profile or forbidden metadata/API/database/forge destinations refuse before create; mutation: allow a profile with no default-deny reference — test fails |
| 8 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestKubeCredentialSource$" ./adapters/kubernetes/ > "$out" && grep -Fq -- "--- PASS: TestKubeCredentialSource " "$out")` | exit 0; named PASS; absent explicit credential/target/opt-in refuses with zero API calls even with ambient kubeconfig/in-cluster settings present; validates narrow profile permissions, prints sanitized target before contact, exposes no credential; mutation: fall back to ambient credentials — test fails |
| 9 | check:ci | `statusgen --consumers --brief agent-execution/08 --base $(git merge-base origin/main HEAD)` | exit 0; declared consumer sites corroborate against the implementation diff; out-of-scope claims remain reviewer-owned, not a proof of completeness |

## Pre-mortem

A fake API cannot prove network isolation, scheduler semantics or credential custody in a
real cluster. The adapter must still prove the submitted spec and refusal floor offline. Real qualification must prove effective pod admission, network enforcement (including metadata/API/database/forge denial), credential/RBAC scope and containment against hostile candidate code. Those remain explicit unqualified adoption obligations. The executable rows
cover production adapter decisions; independent review assesses containment design and the
operator owns the real-profile evidence. Keep actual API outcomes distinct from model advice.

## Evidence

<!-- Non-implementer records exact commands, revision, environment, results and limits. -->

## Review

Human gate. Confirm profile boundary, lower-layer refusals and explicit deployment exclusion.

## Shared Kubernetes transport — 20 October 2026

Consume agent-execution/09's Kubernetes identity, explicit-target transport and unknown-outcome
reconciliation primitives. That brief owns the first shared adapter package and standing
profile; this brief adds only its bounded verifier Job profile after it. Keep the stricter
candidate network/credential floor, controller/publication prerequisites and every current
Verify obligation. Interactive attach capability is never inherited by verifier Jobs.
No second Kubernetes runner, ambient kubeconfig discovery or operator credential in a Job.
The managed standing-desk route does not wait for this verifier profile.
