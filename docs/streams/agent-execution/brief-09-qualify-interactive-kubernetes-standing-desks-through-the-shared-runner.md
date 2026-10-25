---
brief: assay:assay:agent-execution:09
title: Qualify interactive Kubernetes standing desks through the shared runner
why: Operators need separately interactive Kubernetes desks without a second scheduler or moving privileged credentials into agent workloads.
wave: 9
depends: ["agent-execution/03", "graph-execution/27"]
unblocks: ["desk-containers/28", "agent-execution/08"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
issues: []
schema: brief-v2
version: 4
authored: 2026-10-20 by Assay architecture authoring session
sources:
  - https://github.com/medici-finance/assay/issues/3669
  - docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md
  - freshness-checked 2026-10-20 @ d8687cab6 (fetched origin/main; code and open PRs inspected)

exec-tier: strong
exec-tier-why: Cross-process ownership, custody and recovery require design judgment.
domain: complicated
decision-trigger: spec
gate-why: Operator and workload authority cross a process boundary; the human confirms effective credential isolation, current-owner fencing and the independently witnessed profile before adoption.
id: cb59f755-2881-4a8f-8354-ee422db4f3ea
consumers:
  - "loopadmin/adapters/kubernetes/: follow-up agent-execution/09"
  - "loopadmin/supervisor/: follow-up agent-execution/09"
  - "containers/entrypoint.sh: follow-up agent-execution/09"
  - "containers/secrets.md: follow-up agent-execution/09"
  - "loopadmin/adapters/kubernetes verifier profile: follow-up agent-execution/08"
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/28"

outcome: none

---

# Brief 09 — Interactive Kubernetes standing desks through the shared runner

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `loopadmin/adapters/kubernetes/` (planned shared API transport and standing profile), `loopadmin/testdata/kubernetes/` (planned), `loopadmin/supervisor/` (profile binding), `containers/entrypoint.sh`, `containers/secrets.md`, `docs/kubernetes-desk-profile.md` (planned), `changelog/kubernetes-standing-desks.md` (planned).

facts: Inspected Assay main d8687cab6, 20 October 2026. The runner protocol exists;
the Kubernetes adapter, shared supervisor and manifests remain planned. The current image
preflight requires an App PEM and model credentials. /08 is a distinct candidate Job
profile requiring the workflow controller/publication bridge. Standing roles must not
inherit that requirement or its execution identity. Consume /03 readiness/profile and
/27 standing-desk outputs as actual importable APIs, never cmd/main or tools/desk/internal
from this external module. The execution stream remains parked pending admission.

single-point-of-failure: adapter maps the wrong Pod or credential profile; independent
supervisor generation/stop checks and the existing executor's current-authority checks
reject effects even when transport is bypassed. Effective Pod/network/RBAC isolation is
separately qualified before activation.

layering: One loopadmin Kubernetes API-effect adapter implements runner translation;
shared pure identity/profile checks and API transport are reused by /08. Desk policy,
PostgreSQL journal, effect custody and acceptance retain their existing owners. No
second lifecycle controller, queue, credential broker or scheduler. Verify 1–4 exercise the seam.

design-fit:
  owner: loopadmin runner/supervisor; Kubernetes adapter owns transport translation only
  contract: none — new profile extends spec/loop-admin-runner-v1.md through its existing negotiation
  retires: []
  weight: verbs +0, flags +0 public verbs, refusal predicates +4 (binding, credential profile, old owner, unknown transport); actual text delta recorded at implementation
  why-add: A host-process adapter cannot address a Kubernetes Pod identity. One adapter shares transport with the planned Job profile rather than adding a parallel remote runner.

## Read first

- [Interactive target](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md).
- [Runner contract](../../../spec/loop-admin-runner-v1.md).
- [Standing clients](../graph-execution/brief-27-standing-desk-loop-admin.md).
- [Readiness owner](brief-03-local-readiness-enforcement.md).
- [Distinct verifier profile](brief-08-qualify-a-kubernetes-verification-runner-through-the-existing-supervisor.md).

## Human decision

At pickup, present the concrete credential, transport and network boundary with its
negative-path evidence. Terminal access to a credential-bearing compatibility desk gives
transitive access to that desk's signing and model credentials. The managed profile must
keep those privileges outside the agent workload; this choice does not select a cluster.

Options:
1. **Managed standing desks (recommended)** — qualify separate preinstalled role Pods,
   an attach-only trusted operator and credential-free agent/candidate workloads before adoption.
2. **Retain compatibility** — keep the existing trusted credential-bearing desk profile;
   do not enable the managed operator component or claim managed custody isolation.

Default if no answer: none — blocks until answered

## Ground rules

Isolated branch and draft PR only. No deployment, workflow dispatch, live cluster or model
contact. Fake Kubernetes API/streams and disposable local storage only. Preserve the parked
stream's admission gate. Independent verification owns Evidence. Existing budgets, effect
and human gates remain in force. Refresh claims/active PRs before implementation.

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner\'s delta; unchanged obligations remain.

## Task

1. Add the standing profile to the existing runner/supervisor API. Bind cell, role, namespace,
   Pod UID, container, owner incarnation/generation, qualified image digest, profile and
   preparation receipt. Installation pre-creates the declared workloads; normal launch
   cannot submit arbitrary Pod/service-account/mount specifications. Inject an explicit
   scoped API credential into the trusted adapter only; missing target/opt-in/credential
   causes zero API calls. Print the non-secret target before first contact. No ambient
   kubeconfig or in-cluster credential discovery fallback.
2. Implement start/reconcile, inspect, attach/reconnect and control using the existing
   shared journal and role leases. Typed terminal streams preserve input/output, resize,
   detach and bounded reconnect. Attachment never starts the model or becomes a lease.
   Repeated up, concurrent operators and a timed-out request reconcile the same role
   generation. Pod replacement and partitioned-node old execution hold until the old
   execution group is confirmed stopped/fenced; DELETE acceptance or TTL alone is insufficient.
   Resume does not renew an exhausted budget or clear an unrelated stop/decision.
3. Add a **named managed boot profile** to the image preflight, consuming /03's validated
   binding. Retain legacy PEM-required behavior for compatibility consumers. Managed agent
   Pods receive no operator/cluster or forge-publisher credentials, disable service-account
   automount, and cannot obtain the adapter's credential or call operator lifecycle APIs.
   Bind model authentication through the already qualified custody owner; if unavailable,
   hold rather than invent a broker or mount an App PEM. Missing/unknown profile refuses.
   Operator terminal access is attach-only to named managed desk Pods. Deny exec,
   port-forward and ephemeral/debug subresources for every verb, including
   get/create/patch/update. Refuse a compatibility binding: its terminal is transitive
   signing/model-key custody. Require default-deny ingress and a qualified managed-agent
   egress allowlist excluding API, metadata, CNPG and direct forge; allow only declared
   model/executor endpoints and DNS.
   Document actual enforcement and residual trust; never infer isolation from containers.
4. Factor Kubernetes identity, API observation and unknown-outcome transport primitives so
   /08 consumes them from this package. Keep Job and standing profile allowlists distinct:
   a verifier cannot acquire attach/exec or a standing credential, and a standing role
   cannot acquire publication authority or arbitrary Job admission. Preserve the stricter
   /08 credential/network floor and its controller dependencies.
5. Exercise real production adapter and supervisor code against hostile fake API/stream
   fixtures: lost reply, duplicate delivery, API unavailable, stale UID/generation, wrong
   role, concurrent reconnect, unknown cancellation and permission denial. Carry current
   perf-v3 parent/child identities and preserve unknown usage. Idle and reconnect cause zero
   extra model calls. Regenerate the owning profile documentation and image compatibility
   matrix. Add path triggers for every module/container file the checks read.

- **Instance amendment:** Change the managed standing Kubernetes binding from one Pod/workspace/lease per role to one per configured instance/generation for worker/review. Reuse role images/authority, separate instance workspaces and validate cell/role/instance labels plus namespace/Pod UID/container/current generation. Use operator pre-provisioned desired workloads; normal desk launch never gains arbitrary Pod creation. Distinct replacement instances may overlap after the admission barrier; same-instance unknown-stop/partition remains held. Add instances to all hostile fake API/profile paths.

## Interface contract

Standing mode carries a desk binding, not fabricated workflow/node fields. Immutable
invocation and execution identity enter; sanitized lifecycle/stream receipts leave. Transport
liveness, process failure, retained files, session continuity and accepted work are separate.
Unknown observations hold. Existing conformance and extension negotiation remain mandatory.

## Verify

Tests below are implementation deliverables, not existing PASS claims. Each named test must
run (zero skips) and fail under its stated mutation. Run from repository root.

For the consumer row, the independent verifier obtains the actual implementation delivery
from the merged PR, records its full immutable SHA in `ASSAY_DELIVERY_SHA`, and records that
commit's actual first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither value defaults to
HEAD, origin/main or this authoring PR. Both commits must be available in the verification
checkout; absent/unknown provenance fails closed. Retain exact consumer result counts and
resolve unchecked required consumers separately; zero disproofs alone is not complete flow
coverage. These inputs are future implementation-verification evidence, not claims made here.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && GOWORK=off go test -count=1 -v ./adapters/kubernetes/... -run '^TestStandingIdentity$' > "$o" && grep -F -- '--- PASS: TestStandingIdentity ' "$o")` | exit 0, named PASS; actual supervisor/adapter maps all five roles; duplicate up/attach makes one owner and zero extra model calls; accepting another Pod UID makes it fail |
| 2 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && GOWORK=off go test -count=1 -v ./adapters/kubernetes/... -run '^TestStandingRecovery$' > "$o" && grep -F -- '--- PASS: TestStandingRecovery ' "$o")` | exit 0, named PASS; lost reply/partition/unknown stop holds replacement; paused state, effects and budgets survive transport/operator restart; treating delete submission as stopped fails |
| 3 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && GOWORK=off go test -count=1 -v ./adapters/kubernetes/... -run '^TestKubeProfileIsolation$' > "$o" && grep -F -- '--- PASS: TestKubeProfileIsolation ' "$o")` | exit 0, named PASS; wrong credential/profile and missing opt-in make zero calls, agent cannot reach operator/executor custody with upper caller bypassed, verifier never gains terminal authority; operator rejects compatibility bindings and all exec/port-forward/debug verbs (including get/create/patch/update); forbidden network endpoint or swapped capability fails |
| 4 | check +flow +neighbour +dereference | `(cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/... ./adapters/kubernetes/...)` | exit 0, no skipped required profile cases; shared runner negotiation, existing local behavior and legacy image preflight preserved; actual image compatibility checks included by adapter tests |
| 5 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief agent-execution/09 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers remain a review obligation |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && cd loopadmin && GOWORK=off go test -count=1 -v ./adapters/kubernetes/... -run '^TestKubeDeskInstances$' > "$o" 2>&1 && grep -F -- '--- PASS: TestKubeDeskInstances ' "$o")` | Named test executes (zero matching tests is not a pass); Two mixed-profile worker Pods and three reviewer Pods have separate workspaces; concurrent up/attach and same-named changed UID cannot duplicate an instance. A successor starts while its predecessor drains without inheriting claims; mutation: key by role, share PVC or accept name without UID makes the test fail. |
## Pre-mortem

Wrong Pod or duplicate role owner: row 1. Lost stop mistaken for cancellation: row 2.
Cross-profile authority or ambient credential fallback: row 3. Compatibility regression:
row 4. Effective cluster containment is review plus /29's separate operator-run qualification;
fake API success alone does not establish it.

## Evidence

<!-- Independent verifier records commands, revision, profile and explicit limits. -->

## Review

Human gate. Review the independent controls, import boundaries and actual credential/model
profile. Offline fixture success does not authorize deployment or unpark the stream.
