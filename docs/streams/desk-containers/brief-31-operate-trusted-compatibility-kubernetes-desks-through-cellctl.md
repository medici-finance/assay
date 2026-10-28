---
brief: assay:assay:desk-containers:31
title: Operate trusted compatibility Kubernetes desks through cellctl
why: Operators need interactive agents with retained files in Kubernetes without first replacing desk scheduling, verification or the whole managed runtime. The existing package opens shells; this bounded compatibility path starts the actual configured agents and makes reconnect and controlled stop explicit.
wave: 8
depends: ["desk-containers/06", "desk-containers/15"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: Terminal access to a compatibility desk includes custody of its role PEM, model credential and session transcript. The human confirms the concrete operator, image, boot and stop design preserves those existing boundaries and does not claim managed candidate isolation.
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
authored: 2026-10-28 by compatibility milestone authoring session
sources:
  - "maintainer request, 2026-10-28: smaller interactive Kubernetes compatibility milestone"
  - "docs/streams/desk-containers/README.md#small-interactive-kubernetes-compatibility-milestone"
  - "docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md (compatibility and managed profiles remain distinct)"
  - "freshness-checked 2026-10-28 @ 0a4c08b83b1c3a743d6bf1ce7b1f41b331d17125: worker-authenticated fetch; actual cellctl routing, entrypoint, manifests and module imports inspected"
exec-tier: strong
exec-tier-why: Cross-process execution identity, credential custody, terminal streams and stop uncertainty need design judgment.
domain: complicated
consumers:
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/31"
  - "tools/desk/internal/kubecompat/: follow-up desk-containers/31"
  - "containers/k8s/compatibility/: follow-up desk-containers/31"
  - "containers/k8s/components/compatibility-cellctl/: follow-up desk-containers/31"
  - "containers/k8s/check.py: follow-up desk-containers/31"
  - "containers/k8s/testdata/: follow-up desk-containers/31"
  - "containers/base/Dockerfile: follow-up desk-containers/31"
  - "docs/cellctl.md: follow-up desk-containers/31"
  - "docs/docker.md: follow-up desk-containers/31"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desk-containers/31"
  - "managed operator and supervisor contracts: out-of-scope (existing desk-containers/28–29 and graph-execution owners retain their stronger promises)"
id: d75d55ff-3b37-4b25-827c-91a6b2f85139
---

# Brief 31 — Trusted compatibility Kubernetes desks through cellctl

## Context

files:
- tools/desk/internal/kubecompat/ (planned config, identity, lifecycle, native API/stream adapters and fake-transport fixtures)
- tools/desk/cmd/cellctl/kube_compat.go (planned routing and attach-only adapter)
- tools/desk/cmd/cellctl/kube_compat_test.go (planned compiled-CLI flow tests)
- tools/desk/cmd/cellctl/cobra.go (existing command registration)
- tools/desk/cmd/cellctl/envkeys.go (existing configuration registry)
- tools/desk/cmd/cellctl/cell.go (existing configuration loading)
- tools/desk/cmd/cellctl/desk.go (existing role launch routing)
- tools/desk/cmd/cellctl/up.go (existing cell-op plan/result adapter)
- tools/desk/cmd/cellctl/status.go (existing status routing)
- tools/desk/cmd/cellctl/down.go (existing stop controls)
- tools/desk/cmd/cellctl/help/ (existing operator help)
- tools/desk/cmd/cellctl/testdata/cli-nonhelp-golden.json (existing command inventory)
- tools/desk/go.mod and tools/desk/go.sum (native Kubernetes SDK only if needed after the existing dependency assessment)
- containers/k8s/compatibility/desk-session.sh (planned bounded PID-1 agent launcher)
- containers/k8s/compatibility/desk-session.test.sh (planned launcher negative controls)
- containers/k8s/compatibility/cellctl-config.schema.json (planned profile contract)
- containers/k8s/components/compatibility-cellctl/ (planned opt-in paused workloads, launch wiring and custodian-only RBAC)
- containers/base/Dockerfile (package the compatibility launcher in a newly identified candidate image)
- containers/k8s/check.py and containers/k8s/testdata/ (extend existing offline profile checking and hostile fixtures)
- docs/cellctl.md and docs/docker.md (actual lifecycle, custody and manual recovery limits)
- docs/streams/desktools-v2/cli-migration.json (route the new CLI surface through the existing owner registry)
- changelog/desk-containers-31-kubernetes-compatibility.md (planned)

facts:
- At source 0a4c08b83, the compatibility base has five StatefulSets/PVCs, stdin/tty and digest-pinned v1.0.52 role images. Those digests boot the existing shell command; they do not prove the new agent launcher exists.
- The shared containers/entrypoint.sh checks runtime PEM/model custody and then executes its supplied command. Preserve that preflight; the new opt-in command must invoke the approved pinned agent and role instructions, rather than count an idle shell as a running agent.
- Existing k8s-kind launch.go takes the local harness path. Existing native container routing in container_native.go is a routing example, not a Kubernetes implementation to clone.
- Current up.go already uses internal/cellop and cell-op-v1 for plan, interactive confirm, re-measure and JSON. Brief 15 is implemented with recorded PASS; its human acceptance remains separate from implementation eligibility.
- The existing image preflight is the current Claude-compatible credential profile; a claimed additional harness needs its own explicit compatible credential/argv qualification, rather than inferred parity. The current desk module has no Kubernetes client library or Pod attach adapter. A native SDK choice and compatible pin are implementation pickup work; no executable API from the future loopadmin supervisor is assumed.

layering: Kubernetes remains the workload controller; existing cellctl and role policies own the operator and work meanings. A module-local kubecompat adapter separates pure config/identity/transition decisions from typed native API and terminal effects. The bounded PID-1 launcher starts one role execution and preserves stop/dedup facts; it is not a second general scheduler, workflow engine or effect authority. Tasks 1–4 and Verify rows 1–5 exercise the boundaries.

single-point-of-failure: a client can mistake a different or restarted execution for its retained desk; exact Kubernetes execution identity and the independent boot-layer launch record refuse that mistake even with CLI bookkeeping bypassed. Neither record grants effect authority or managed isolation.

design-fit:
  owner: existing cellctl operator lifecycle; Kubernetes owns workload reconciliation and existing desks own task policy
  contract: none — add one explicitly selected compatibility profile to the existing operator boundary, retaining cell-op-v1
  retires: []
  weight: verbs +1 attach-only, flags +0 ordinary lifecycle, configuration keys +1, bounded refusal cases +6 (target, identity, custody, lineage, quiescence, stop-unknown)
  why-add: Existing k8s-kind routing starts a local harness and cannot address Pod streams. An opt-in backend and attach-only command avoid repointing legacy registrations or constructing the future general runtime. Manual attach was considered, but cannot make cellctl start/status/stop address the same execution.

## Read first

- [Controlling compatibility scope and criteria](README.md#small-interactive-kubernetes-compatibility-milestone).
- [Existing credential contract](../../../containers/secrets.md), [compatibility package](../../../containers/k8s/README.md) and [cell-operation contract](../../../spec/cell-op-v1.md).
- [Current architecture and distinct managed profile](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md).
- [Library boundaries](../../library-first.md), [Go engineering](../../go-engineering.md), [semantic contracts](../../contracts.md) and the installed implementation-pickup procedure.

## Human decision

Decision-trigger: spec. At implementation pickup, record the concrete trusted-custodian profile, launch record and controlled-stop design before asking for acceptance. The executor authors a self-contained decision section from that design. No response authorizes no deployment, credential grant or profile activation; offline design and fixtures can proceed.

## Ground rules

- Authoring and implementation are offline against real infrastructure. No cluster, production endpoint, live model, deployment or workflow contact. Fake adapters and synthetic credentials only.
- Preserve compatibility key custody and the current credential preflight. This profile is for trusted operators and agents; it does not qualify arbitrary candidate code or the managed operator component.
- Do not change existing managed briefs, their approvals or Evidence, and do not make workflow storage/controller, CNPG, queue improvements or a general supervisor prerequisites to this transition.
- Preserve existing STOP, role claim and in-flight work controls. A new transport is not permission to terminate a busy role, reuse an unknown execution or bypass an existing refusal.
- Keep publication in a branch and draft PR. Stop at implemented; independent verification and existing human acceptance retain their owners.

## Task

1. At pickup record the source revision, current producer/image, inherited credential and cell-op contracts, module boundary, current claims/PRs and a continue/adapt/hold assessment. Recheck current context before publication. Define the opt-in cell-kube-compat-v1 configuration, selected by CELL_KUBERNETES_CONFIG on k8s-kind cells; absent selection preserves the legacy local path. Bind explicit kubeconfig file and context, expected API endpoint and TLS server/CA identity, cell/namespace, role-to-StatefulSet/Pod/container mapping, approved launch argv and image digest. Never discover an ambient target, execute an arbitrary credential plugin, accept an insecure TLS flag or print credential values. Malformed/missing/mismatched inputs make zero operation calls. Review/pin a native SDK if required; do not shell to kubectl or add a CLI-over-CLI integration.
2. Add an opt-in compatibility component built from the existing five-role package. Installation pre-creates exactly named workload and custody-scoped access; workloads begin paused at replicas zero. Ordinary cellctl can read/scale only those named workloads and attach only to their configured containers; it cannot create arbitrary workloads, mount credentials, mutate RBAC, exec/port-forward/debug into another role, or reach database/executor custody. Preserve agent token automount disabled, restricted egress, separate role Secrets and terminal/log/node/downstream-log custody. This trusted operator is a key-custody principal, not the credential-free managed operator. Supply an explicit API target and print its non-secret identity before first contact. Offline dry run/plan/check must not contact the API.
3. Package a bounded PID-1 launcher in a newly built, digest-bound compatibility candidate image and select it only in this opt-in component; retain the old v1.0.52 package as historical compatibility output. Reuse existing entrypoint/init/process-group handling where applicable. Invoke exactly the pinned agent and configured role instructions after the unchanged credential preflight. Before launch, durably record intent on the retained role volume. Positively initialize first boot as an operator-owned preparation act with no prior execution, rather than interpreting absent history as permission. Admit a later launch only after a recorded controlled clean stop for a distinct Pod UID. A same-UID container restart, dirty/erased/corrupt history or unknown predecessor starts held with no agent/model invocation. The local stamp is crash/dedup bookkeeping within trusted custody, not an authorization boundary or a replacement for role claims. No new database, general scheduler or automatic crash/partition recovery is introduced.
4. Route up, desk, status and down through the selected backend. up/desk reuse existing role selection and cell-op plan/confirm/re-measure/result semantics; no bare --yes or new approval meaning. Starting an already current role is a no-op or connection to that execution, never another model request. Add cellctl attach <cell> <role> as an attach-only operation: no scale, process creation or model request when disconnected or absent. TTY input/output/resize and detach/disconnect affect presentation only; no signal-proxy shortcut can stop a role when its terminal closes. Keep launch/status/stop observations separate from work acceptance. Refuse host-cockpit/model overrides the pinned profile cannot honor.
5. Retain run/kubecompat/<role>.json under the selected cell for operator observations, binding config digest, StatefulSet UID, Pod UID, container ID and restart count, image, role and observation state. Never treat the writable receipt as authorization. Validate the live tuple before streams and effects; same-Pod new-container identity invalidates an old stream/receipt. A controlled stop requires existing role quiescence/claim and effect reconciliation, or an explicitly disclosed custodian-confirmed interactive idle condition when no machine signal exists. If the role remains busy or cannot be measured, hold and name the manual stop procedure. The launcher signals and reaps the complete execution group, records clean stop only after it has stopped, and exits. Observe exact container termination before recording stop success or admitting a replacement. Scale/delete accepted, timeout, Ready/NotReady or an absent Pod is not termination proof. Lost acknowledgement, changed/lost UID, node partition and uncertain old background execution remain held; no force deletion, forced claim release, automatic retry or history reset.
6. Update existing CLI/help/config/command-inventory routes, the profile-aware offline checker and operator docs. Keep non-selected registrations unchanged. Document all manual preparation and custody restrictions, including preinstalled pinned agent/toolchain/work inputs, explicitly qualified harness/model profile and role policies, cold initialization, install versus up, detach versus stop, and the held recovery procedure. Report filesystem retention, conversation continuation and process continuation separately; a PVC is not a promise of restored conversation or live process. Supply the package/image identity and config/receipt fixtures for the separate small-profile qualification brief. No runtime benefit, latency or recovery target is invented.

## Interface contract

- Profile: cell-kube-compat-v1, selected only by CELL_KUBERNETES_CONFIG; its schema and examples live under containers/k8s/compatibility/. Missing selection does not alter existing kinds.
- Current identity: cell, role, profile/config digest, namespace, StatefulSet UID, Pod UID, container name, container ID/restart count and image digest. A new container under the same Pod is a new execution.
- Lifecycle: existing up/desk/status/down adapters; attach is presentation-only. The only supported recovery is a controlled, reconciled clean stop followed by a new Pod execution. Unknown old execution remains held for a custodian.
- Observations: the existing cell-op-v1 result/plan plus operator receipts at run/kubecompat/<role>.json. The qualified profile publishes no new work/claim authority.
- Qualification handoff: actual candidate image digest, rendered component, explicit config, initialized-volume provenance, fake transport cases and exact cold/start/attach/detach/status/stop/new-start CLI sequence. All named new paths and tests below are deliverables, not already-run evidence.

## Verify

Every named test must execute and emit its named PASS; a go test invocation matching no test is not evidence. The delivery inputs in row 7 come from the actual merged implementation PR and its actual first parent, never a moving main or this authoring change.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -count=1 -v ./cmd/cellctl -run '^TestKubeCompatCellctlLifecycle$' > "$o" && grep -F -- '--- PASS: TestKubeCompatCellctlLifecycle ' "$o")` | exit 0, named PASS; the compiled CLI drives the production adapter through fake API/streams for all five roles, actual agent launch, retained files, resize, detach/reconnect and controlled stop/new start; repeated up/attach creates zero additional agent/model invocations |
| 2 | check:ci +mutation +flow | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -count=1 -v ./internal/kubecompat/... -run '^TestKubeCompatIdentity$' > "$o" && grep -F -- '--- PASS: TestKubeCompatIdentity ' "$o")` | exit 0, named PASS; omitted opt-in/target, wrong TLS identity, foreign namespace/role, stale Pod UID and same-UID new container/restart count refuse; planted ambient fallback or omitted tuple check fails the test |
| 3 | check:ci +mutation +flow | `sh containers/k8s/compatibility/desk-session.test.sh --self-test` | exit 0; output contains "checked-clean: compatibility launch and execution-group stop"; invoking the boot layer without cellctl still refuses same-Pod restart, erased/corrupt history and unconfirmed predecessor; first boot requires positive initialization; controlled stop waits for planted ordinary and detached/reparented descendants (including setsid) and an unclean exit never grants another launch |
| 4 | check:ci +mutation +flow | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -count=1 -v ./cmd/cellctl -run '^TestKubeCompatStopUncertainty$' > "$o" && grep -F -- '--- PASS: TestKubeCompatStopUncertainty ' "$o")` | exit 0, named PASS; busy/unknown role, missing termination acknowledgement, absent Pod, accepted scale/delete, lost response, partition, surviving execution group and changed identity remain held with zero replacement launch; removing one hold fails the test |
| 5 | check:ci +mutation +dereference | `sh containers/k8s/check.sh --compatibility-cellctl --self-test` | exit 0; output contains "checked-clean: compatibility cellctl profile"; actual rendered component and candidate-image binding pass; hostile live replicas at installation, old shell-only digest, wildcard scope, foreign-role attach/log/debug, broad credential mounts, missing API-token isolation and missing network/terminal/log custody declarations are rejected offline; no discovery/API call. Effective RBAC/network/log custody remains separately human-qualified |
| 6 | check:ci +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -count=1 -v ./cmd/cellctl -run '^TestKubeCompatParity$' > "$o" && grep -F -- '--- PASS: TestKubeCompatParity ' "$o")` | exit 0, named PASS; non-selected k8s/house/container/scrubbed routing and current cell-op JSON/plan/confirmation contract remain unchanged; docs examples replay against the fake production transport |
| 7 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/31 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the actual immutable implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved, and resolve unchecked required consumers rather than treating them as PASS |

## Definition of Done

The opt-in profile runs the actual configured agents through one cellctl backend, preserves current custody and cell-op/stop controls, and passes every offline row with discriminating negative controls. The candidate image and profile are identified for independent qualification. This brief does not establish effective cluster isolation, deployment, final human acceptance or unattended recovery support.

## Pre-mortem

| Failure mode | Caught by |
|---|---|
| Five Running shells are reported as five agents | rows 1 and 5; actual launch plus candidate image binding |
| Reconnect or same-UID restart creates a second invocation | rows 1–3; exact execution tuple and independent boot layer |
| A busy or partitioned old role is replaced on a scale acknowledgement | row 4; no termination proof means held |
| The main child exits but dispatched/background work survives | rows 3–4; complete execution group and reconciliation |
| A new backend widens custody or silently changes legacy registrations | rows 2, 5 and 6; effective live boundaries are separately qualified |
| A plausible but wrong example or consumer route passes a presence check | rows 1, 6 and 7; actual flow and immutable delivery dereference |

## Evidence

<!-- Independent verifier records source, image/profile, actual commands and limitations. Authoring is not implementation or acceptance evidence. -->

## Review

Gate: human. Confirm the actual trusted-custodian operator and boot/stop design, with lower-layer negative controls; retained compatible agents do not become a managed isolation profile. Preserve independent verification and the separate operator-authorized qualification/adoption gate.
