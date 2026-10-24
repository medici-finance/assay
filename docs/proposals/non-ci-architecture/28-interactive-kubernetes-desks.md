# Interactive Kubernetes desks and an in-cluster operator

**Status:** draft — implementation direction requested 20 October 2026; authoring amendment,
not evidence of deployment or approval of a particular custody profile. The requester
explicitly permits `cellctl` inside Kubernetes and removes native Windows from the
required path. Earlier revision-bound approvals and activation gates retain their scope.

Request tracked by [Assay #3669](https://github.com/medici-finance/assay/issues/3669).

Implementation admission update, 24 October: the maintainer approved amendment 27
and requested agent-execution activation in
[the approval and reconciliation change](https://github.com/medici-finance/assay/pull/4349).
That approval removes the stream-level hold. This chapter's concrete custody/profile
decisions, effective qualification and deployment gates retain their independent scope.

## Outcome and checked starting point

An operator enters a dedicated Linux operator container and uses `cellctl` to start,
inspect, attach to, detach from and recover a cell. Each configured desk has its **own
interactive container in its own Pod**, with an independently retained writable workspace.
The five current role bindings remain coordinator, intake, worker, review and verify.
Consume the configured role registry: the separately planned review-role rename must not
introduce a sixth owner or break existing image names.

The interactive connection is optional presentation. Disconnecting it must not stop a
desk, dispatch a new model request, transfer a claim or create another role owner.
A healthy idle desk can remain available without an active model invocation.

Freshness: Assay main `d8687cab6`, fetched 20 October 2026. Inspected source:
`containers/base/Dockerfile`, all five desk Dockerfiles, `containers/entrypoint.sh`,
`tools/desk/cmd/cellctl/{cell.go,desk.go,launch.go,up.go}` and `loopadmin/runner/`.
The images and runner protocol exist. `containers/k8s/`, `loopadmin/supervisor/` and
`loopadmin/adapters/kubernetes/` do not exist at this revision. The current `k8s` cell
kind still launches its harness through the local launch path; the name is not evidence
of per-desk Kubernetes execution. The native container path also limits ordinary `up`
to the coordinator. Merely putting that binary in a Pod does not close these gaps.

Open PRs were read on 20 October 2026: public #3001 owns the local PostgreSQL profile;
#3350 proposes a replacement image scan; #3584 owns issue-projection/perf integration;
#3583 owns own-request forge measurement. Preserve those active contracts and coordinate
at pickup. Neither their draft state nor a historical board cell proves absence of code.
Dispatch refs were not enumerated; dispatch still requires the normal fresh claim check.
No cluster, production endpoint or model was contacted by this authoring work.

## Target and ownership

| Surface | Owner | Required boundary |
|---|---|---|
| Operator container and its CLI wiring | desk-containers/28; existing `cellctl` and cell-operation contract | Human operator only; no model conductor, forge signing key or publisher credential |
| One interactive desk workload per role | desk-containers/06; agent-execution/09 managed profile | Per-role Pod/PVC, pinned image and role binding; a terminal is not a scheduler |
| Launch, lease, stop and recovery | graph-execution/26–27, /16; agent-execution/03 and /09 | One supervisor and one current owner per role/generation; no launch on unknown stop |
| Work selection and judgment | Existing desk policies; desk-tools/39 and /42; graph-execution/27 | Deterministic observation/bookkeeping; bounded model work for actual judgment |
| Effects, keys and acceptance | Existing R1/R4 evaluators and role executors | Candidate/agent code cannot reach operator or publisher custody; process exit is not acceptance |
| Runtime state | Cell PostgreSQL; desk-containers/27 CNPG profile | One coordination store, separate journal schema; no SQLite runtime or workflow-store prerequisite for standing mode |
| Attach, logs and status | Existing cell-operation/supervisor projection and Kubernetes transport | Bind cell, namespace, Pod UID, container and generation; reconnect never starts work |
| Qualification and adoption | desk-containers/29 and existing R8 migration owner | Reconcile old writers/effects, prove rollback, then an operator-approved scoped cutover |

The operator Pod is separate because it holds operator capabilities; each desk is separate
because its role, workspace, model profile and lifecycle are independently governed.
This does not create another controller, API, database or credential broker. Reuse existing
Go libraries within an authority boundary and typed clients across it. No Docker daemon,
Docker-in-Docker, host socket, privileged Pod or native Windows prerequisite is introduced.
A desktop needs only an existing approved way to enter the operator terminal.

## Two explicit profiles

**Compatibility interactive profile.** /06 packages the current images and historical
runtime-injected credential contract. It is an explicit transition with the same custody
limitations as today; it is not a qualified untrusted-candidate execution boundary. No
agent is admitted merely because the Pod is Running. Current credential mounts must not
be relabelled as isolated future custody. /02's unfinished independent acceptance remains.

**Managed standing-desk profile.** /09 consumes /26–27 and the qualified-profile/readiness
contract. Agent/candidate containers hold neither a cluster credential nor operator or
forge-publisher credentials. The existing executor/model-custody owner supplies only the
capabilities qualified for that profile. Revise the image entrypoint through this named
profile: the legacy PEM-required preflight remains for compatibility, while a managed
boot validates its approved executor/profile binding and never silently falls back to a
PEM or ambient credential. Shared images alone do not prove equivalent isolation.

No /28 operator component is installed in the compatibility profile. Compatibility role
Secrets remain role-specific; sharing a signing key or model credential across independent
roles is not qualified by this plan. Check all committed examples as well as rendered output.

The Kubernetes verifier profile in agent-execution/08 remains a separate, bounded Job
profile with its existing controller/publication dependencies and stricter network floor.
/09 first extracts reusable Kubernetes transport/identity/reconciliation mechanics;
/08 depends on it and consumes those mechanics without acquiring interactive privileges.
Standing desks do **not** depend on the workflow controller, candidate publication bridge
or workflow instance tables. Workflow adoption can follow separately.

## Interaction and containment

Use existing lifecycle meanings: start/up, status, attach, detach, pause/drain, stop,
resume and fenced restore. Closing a stream, successful delete submission, elapsed lease
or a missing Pod in an incomplete observation is not confirmation that execution stopped.
A rescheduled Pod is a new execution identity even if the StatefulSet name and PVC remain.
Preserve conversation/files/process semantics separately; PVC recovery does not imply
conversation or process continuation. Confirm the old execution group stopped before a
replacement can act. A partitioned node with an old process is a held case.

Installation/GitOps pre-creates approved workloads and RBAC; ordinary operator `up`
selects a pinned binding rather than submitting arbitrary Pod specs. The /28 operator component is **managed-overlay only** and refuses compatibility bindings.
Its desk capability is `pods/attach` (`get`/`create` as needed by the pinned transport) on
exact named managed desk Pods, with current UID/container/generation validation. Deny
`pods/exec`, `pods/portforward` and `pods/ephemeralcontainers` for **all verbs**, including
`get`, `create`, `patch` and `update`; a transport upgrade must not recover those powers.
No operator attach/exec/port-forward/debug access reaches compatibility desks, PostgreSQL,
executors or other cells. Any terminal access to a compatibility desk is transitive custody
of its App PEM and model credentials; the table's credential-free operator boundary is a
managed-profile property only. Compatibility TTY output is also a session transcript
in container logs, including echoed input. Restrict compatibility `pods/log` alongside
attach/exec/ephemeral-debug to the relevant key-custody principals. Node log files and
collectors are part of that boundary: exclude compatibility namespaces from node log
shipping, or require downstream stores, exports/backups and readers to meet the same
custody and retention/access controls. Generic shared logging must not widen credential
custody. /29 qualifies both API log access and the actual collection path with synthetic
transcripts; the managed operator must not gain compatibility log access. Never infer
custody isolation from absent Secret-read permission.
Explicitly map transport verbs and named resources. Do not grant wildcard RBAC, Secrets
reads, impersonation or RBAC mutation.
If a later design needs workload mutation, author its admission restrictions and prove it
cannot select a more privileged service account or mount; no generic create privilege is
inferred here. Keep the CNPG restore/admin identities separate from desk/operator identities.

Kubernetes [service-account documentation](https://kubernetes.io/docs/concepts/security/service-accounts/)
states that Pod credentials are normally injected, so agent Pods explicitly disable
automounting. The trusted operator/transport uses an explicit projected credential and
pinned namespace/target, never ambient discovery as fallback. The
[RBAC contract](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) permits named
subresource restrictions, while top-level create cannot be restricted by resourceNames.
The [attach interface](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_attach/)
selects Pod and container. Those are design inputs, not proof of effective cluster policy.
Desk and operator Pods have default-deny ingress, no Service/Ingress exposing their
interactive containers, and explicit profile-owned egress allowlists. The managed agent
allowlist excludes the Kubernetes API, node/cloud metadata, CNPG and direct forge access;
only declared model and narrow executor endpoints plus required DNS are admitted. The
trusted operator/transport has only the declared API/control and DNS destinations; it is
not a network route to database or publisher custody. Compatibility egress remains explicitly
limited to its qualified forge/model endpoints and DNS. Effective policy is separately proven
by /29, with denied endpoints tested from hostile workloads.

The real-profile harness takes an explicit kubeconfig path, context and an ownership receipt
binding the expected API endpoint and TLS trust/server identity. Resolve and compare those
values locally before any network contact; do not use ambient KUBECONFIG or a same-named
context from another file. The authenticated connection must validate that identity before
an operation, and a mismatch makes zero operation calls.

Offline render/schema checks never contact an API server; do not use `kubectl apply
--dry-run=client` as a promise of offline validation.

## Existing optimization work is part of the profile

| Obligation | Existing owner | Kubernetes acceptance |
|---|---|---|
| No model polling for an unchanged idle/blocked queue | graph-execution/27, desk-tools/39 and /42 | Five role Pods, zero additional model launches on idle or reconnect; judgments still run when required |
| Bounded startup and cleanup | graph-execution/26–27 | Cold/warm and 1x/10x retained history; no recursive admission/finish sweep; one compatible shared preparation, distinct role leases |
| Correct observation reuse and complete review cost | desk-tools/37–44 | Same exact subject/coverage and independent verdicts; qualified versions and rollback before retirement |
| Custody/discovery hot paths | desktools-v2/58–60 and /62 | Reuse their libraries after qualification; no second token cache or component-discovery engine in the operator image |
| Parent/child/CLI attribution | desktools-v2/63–65; performance-attribution spec | Preserve perf-v3 context through operator, supervisor, Pod and child; unknown usage remains unknown and each invocation is counted once |
| Native improvement evidence | desk-improvement/06–08 and /10 | Actual producer/profile provenance; optional collector/DB unavailability is a disclosed gap, not fabricated zero cost |
| Verifier efficiency | desk-tools/45–51 | Reuse current queue/accepted inputs and environment evidence; containers do not authorize duplicate PASS work or change landing authority |

Offline tests establish deterministic counts and failure behavior. Any live benefit claim
needs a matched workload, versions/profile/image, cache state, sample counts and full costs
(including preparation, retries, failed attempts, storage and human work). No latency, RTO,
RPO, density or monetary target is invented. The adopting operator must set the workload,
capacity and recovery targets before a real-profile qualification run.

## Public ownership and migration

All reusable code, specifications, briefs, images, generic Kustomize/RBAC profiles,
operator packaging, checks and retirement contracts for this effort are owned by
`medici-finance/assay`. This amendment is the current public plan; do not copy an older
parallel runtime programme into a new stream. Adopt generic obligations into existing
owners and keep their historical implementation/Evidence intact.

| Remaining obligation from earlier plans | Public destination |
|---|---|
| Pure policy, shared facts, coverage and shadow comparison | deskcore, statusgen/15; graph-execution/03 and /28–31; forge-neutral/36 |
| Scoped executors, credential admin and permanent outbound enforcement | forge-neutral/39–41; desktools-v2/06, /10–11, /58 and /66; R4 custody boundary |
| Claims, leases, durable effects, restart and recovery | graph-execution/04, /16, /19 and /26; one PostgreSQL authority |
| Typed human acts and exact candidate publication | Existing decision contracts, graph-execution/25 and /31; existing human-gate owners |
| Isolated witnesses and verifier migration | graph-execution verifier-migration contract, /22–25, /28–29; agent-execution/08 |
| Cell operator, shared modes and environment qualification | desk-containers/06, /18–24, /27–29; graph-execution/26–27; agent-execution/01–05 and /09 |
| Per-role adoption, whole-repository writer handover and rollback | graph-execution/27, verifier-migration; composability/03 and R8 qualification |
| Install profiles and last-consumer retirement | composability/03; desktools-v2 custody contracts; existing retirement records |
| Baselines, close-out and improvement | desk-tools/37–44, desk-improvement, perf-v3; measured matched outcomes rather than copied historical totals |
| Product-specific tool overlays | Reusable recipe/image interface in Assay; a consumer supplies its own toolchain/configuration |

A source-repository handoff must inventory every unfinished task, dependency, active PR,
claim and verification obligation, pair it to a public successor, and reconcile the
source's dispatch state. Completed source evidence and historical decisions remain
provenance; they do not become Kubernetes evidence. Old source references are retired or
redirected only after the replacement and its obligations are accounted for. No issue,
PR, dependency or active claim is silently closed. Private credentials, incident details
and deployment values are not publication inputs; the public implementation must be
independently executable with synthetic fixtures. Actual source-board retirement is a
separate, explicitly tracked migration deliverable, not a claim made by this authoring PR.

## Delivery and critical path

1. Existing /06 supplies the offline-renderable interactive workload package. Existing
   image/credential verification still applies. Docker launch /04, compose /05, native
   Windows, CronJobs /12, GitOps examples /13–14 and AX exploration are not prerequisites.
2. Build the existing shared runner/standing-desk/profile prerequisites; new
   **agent-execution/09** adds the Kubernetes standing profile without a workflow controller.
3. New **desk-containers/28** packages the operator image and wires `cellctl` to that profile.
   Existing /07 becomes an optional Linux operator-terminal layout over that single path.
4. New **desk-containers/29** qualifies all five roles, effective boundaries, optimization
   counts and recovery against the /27 PostgreSQL profile. Existing R8 owns live cutover.

The managed branch is `/26 + /22 -> /27`, with `agent-execution/01–03`, then
`agent-execution/09 -> desk-containers/28 -> /29`. `/06` and `/27` (database) join it;
/29 also checks /18 lifecycle and /20 inventory. This is the true implementation head,
not a Windows port or putting an unmodified `cellctl` binary in an image. The execution
stream remains parked until its controlling contract is admitted. This requested
amendment does not silently unpark it or claim any production profile qualified.
