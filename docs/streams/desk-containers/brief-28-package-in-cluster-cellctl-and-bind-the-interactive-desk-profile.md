---
brief: assay:assay:desk-containers:28
title: Package in-cluster cellctl and bind the interactive desk profile
why: An in-cluster Linux operator removes the native Windows dependency while giving the whole cell one qualified launch and reconnect path.
wave: 10
depends: ["desk-containers/06", "desk-containers/15", "agent-execution/09"]
unblocks: ["desk-containers/29", "desk-containers/07", "cell-api/06"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
issues: []
schema: brief-v2
version: 3
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
id: 2df00ce2-4720-49a7-89b3-48d229509e01
consumers:
  - "containers/operator/: follow-up desk-containers/28"
  - "containers/k8s/: follow-up desk-containers/28"
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/28"
  - "tools/desk/internal/cellop/: follow-up desk-containers/28"
  - "docs/cellctl.md: follow-up desk-containers/28"
  - "docs/docker.md: follow-up desk-containers/28"

outcome: none

---

# Brief 28 — In-cluster cellctl operator and interactive desk binding

## Context

files: `containers/operator/Dockerfile` (planned), `containers/k8s/components/operator/` (planned), `containers/k8s/overlays/managed-desks/` (planned), `tools/desk/cmd/cellctl/` (existing CLI wiring and profile tests), `tools/desk/internal/cellop/` (existing typed result/plan boundary), `containers/k8s/check.sh` (planned) (operator mode), `containers/operator/check.sh` (planned), `containers/operator/testdata/` (planned image/component hostile fixtures), `docs/cellctl.md`, `docs/docker.md`, `changelog/in-cluster-cellctl.md` (planned).

facts: At Assay main d8687cab6, 20 October 2026, the current `k8s` kind launches locally;
packaging it unchanged does not control five remote role Pods. /06 supplies workload
manifests; agent-execution/09 supplies the qualified managed standing profile and shared
transport. /15 supplies typed cell-operation results/plans, not a grant to apply.
Native Windows, Docker Desktop and the workflow controller are not prerequisites.

single-point-of-failure: the operator CLI could address a foreign binding; independent
namespace/resource-scoped transport authorization and current-owner checks in the existing
supervisor/executor reject foreign or stale operations with the upper CLI bypassed.

layering: A thin CLI calls existing cell-operation/supervisor clients; the Linux image and
Kustomize component package it. All remote execution is agent-execution/09's adapter;
no Kubernetes shell-command runner, second control service or policy engine in cellctl.

design-fit:
  owner: cellctl/cell-operation owns operator requests; loopadmin owns execution
  contract: existing cell-op-v1 and loop-admin-runner-v1; no duplicate owner contract
  retires: []
  weight: verbs +0, flags +1 explicit execution profile at most, refusal predicates +2 (unqualified profile, foreign binding); actual text delta recorded
  why-add: Operator authority needs a container separate from agent workloads. Reuse cellctl and the existing transport rather than implement a browser service or native Windows port for this cohort.

## Read first

- [Target and migration routing](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md).
- [Execution profile](../agent-execution/brief-09-qualify-interactive-kubernetes-standing-desks-through-the-shared-runner.md).
- [Cell-operation contract](brief-15-cell-op-machine-seam.md).
- [Operator lifecycle](../../proposals/non-ci-architecture/21-cell-operator-surface.md).

## Human decision

At pickup, present the exact operator capability and terminal scope with image and
negative-path evidence. The in-cluster operator is for the managed profile only. Attaching
to a compatibility desk exposes its signing and model credentials transitively; absence
of a Secret-read permission does not prevent that access.

Options:
1. **Managed operator Pod (recommended)** — named-desk attach only, without exec,
   port-forward, debug-container, Secret-read, publisher, database or workload-creation authority.
   Enable it only after independent managed-profile qualification.
2. **External Linux terminal** — retain the existing operator entry path until the managed
   boundary is qualified; do not deploy the operator component against compatibility desks.

Default if no answer: none — blocks until answered

## Ground rules

No cluster contact, deployment, workflow dispatch, ready flip or merge. Work in an isolated
branch and draft PR. Fake API/streams and offline rendering only. No credential values in
images, fixtures, argv, manifests, plans or logs. Independent verification owns Evidence.
Current parked-profile and existing human activation gates remain.

## Task

1. Build a pinned, non-root Linux operator image with the released cellctl/tools and required
   runtime CA/client libraries. No agent/model conductor, Docker daemon/socket, host mounts
   or model/forge-publisher credentials. Add an optional operator Kustomize component with
   separate ServiceAccount and explicit scoped projected credential. Generic manifests have
   placeholder references only, no Secret resources containing credentials, no wildcard RBAC
   and no database/executor terminal capability. Document digest/release/plugin compatibility.
2. Add an explicit managed Kubernetes profile binding to cellctl's existing new/check/up/desk/
   status/down paths and existing lifecycle contracts. Do not reinterpret every legacy
   `CELL_KIND=k8s` registration as this backend. Migrate config explicitly, preserving rollback.
   Resolve actual configured roles and bind namespace/Pod UID/container/generation through
   the adapter; no task-selected endpoint or shell-interpolated kubectl invocation.
   Missing capability reports held/could-not-check without falling back to host launch.
3. Up converges the five preinstalled role bindings; desk attaches/reconnects through the
   same stream boundary. Terminal close or operator-Pod recreation leaves role ownership
   intact. Status distinguishes preparation, execution, retained state and work acceptance.
   Existing pause/drain/stop/resume flows use the supervisor contract with its current-owner
   checks; do not copy the interim local stop-file mechanism into shared volumes. CLI plan/
   confirmation retains /15's semantics and does not impersonate a human approval.
4. Add a managed overlay consuming /06 workloads and /09's qualified image boot profile.
   Legacy PEM mounts cannot enter this overlay. Ordinary up performs no arbitrary workload,
   service-account, Secret or RBAC creation. Any need for greater Kubernetes privilege holds
   for a separately reviewed admission profile. Keep /27's restore/admin identities separate.
5. Reuse existing preparation, credential, discovery and perf-v3 owners; do not fork their
   caches or rules in the operator container. Carry operator-to-supervisor-to-Pod invocation
   context without adding a second usage record. Write the full install/enter/check/up/attach/
   detach/reconnect/status/stop operator guide, compatibility limitations, and exact profile
   prerequisites in docs/cellctl.md and docs/docker.md. Publish no deployed-support claim.

## Operator image and component acceptance

Task 1 installs the operator component **only in the managed overlay**; compatibility
references refuse during plan/check before any stream is granted. Operator RBAC allows
only named-managed-desk attach (get/create as required by the pinned transport), not exec,
port-forward or ephemeral/debug subresources for any verb (explicitly test get/create/
patch/update). It has no access to compatibility desk terminals, database, executor or
foreign-cell terminals. Default-deny ingress and no desk/operator Service or Ingress keep
terminal transport on the authenticated API path; egress is the explicit control/API/DNS
allowlist, without database/publisher routes.

This brief owns extending /06's `containers/k8s/check.sh` (planned) with `--operator` and
hostile component fixtures, and adding `containers/operator/check.sh` (planned) with
`--image`. The source stage checks the actual Dockerfile and all committed component/
example files, not only rendered output. The built-image stage inspects every layer and
config for a non-root USER, digest-pinned base/release, no conductor/agent or Docker daemon,
no host/socket mounts, and no credential-shaped material. Keep the existing layer-secret
scan; use synthetic positive controls, including an unrendered example credential and a
fixture operator image with a forbidden executable. Every planted property must fail the
specific checker. This defines future checks; no image is built by the authoring change.

## Verify

Named tests and scripts below are deliverables; no required case may silently skip.

For the consumer row, the independent verifier obtains the actual implementation delivery
from the merged PR, records its full immutable SHA in `ASSAY_DELIVERY_SHA`, and records that
commit's actual first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither value defaults to
HEAD, origin/main or this authoring PR. Both commits must be available in the verification
checkout; absent/unknown provenance fails closed. Retain exact consumer result counts and
resolve unchecked required consumers separately; zero disproofs alone is not complete flow
coverage. These inputs are future implementation-verification evidence, not claims made here.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -count=1 -v ./cmd/cellctl -run '^TestKubeCellLifecycle$' > "$o" && grep -F -- '--- PASS: TestKubeCellLifecycle ' "$o")` | exit 0, named PASS; compiled CLI drives actual adapter against fake API for five roles; repeated up, attachment loss and operator restart retain one owner per role without local launches |
| 2 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -count=1 -v ./cmd/cellctl -run '^TestKubeOperatorScope$' > "$o" && grep -F -- '--- PASS: TestKubeOperatorScope ' "$o")` | exit 0, named PASS; missing opt-in/target/profile makes zero contacts; wrong namespace/UID/generation refuses; bypassing CLI still cannot reach database or publisher; widening named scope fails |
| 3 | check +mutation | `sh containers/k8s/check.sh --operator --self-test` | exit 0; offline render and cached schema/structured checks reject planted wildcard RBAC, Secret reads, any exec/port-forward/debug verb on desk Pods (including get/create/patch/update), compatibility attach, database attach, PEM mount, Service/Ingress, missing ingress deny, privileged container and unpinned image; no API/discovery calls |
| 4 | check +flow | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/cellctl/... ./internal/cellop/...)` | exit 0; existing cell-op JSON/plan and host/compatibility registration behavior remain; docs examples are replayed against fake transport by the named lifecycle test |
| 5 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers remain a review obligation |
| 6 | check +flow +mutation | `sh containers/operator/check.sh --dockerfile containers/operator/Dockerfile --component containers/k8s/components/operator --image "$ASSAY_OPERATOR_TEST_IMAGE" --self-test` | exit 0 only with the identified locally built candidate image; actual USER/base/config/layers/executable inventory and component pass, all forbidden-image, mount, credential and RBAC fixtures fail; absent image/tool is could-not-check, never PASS |

## Pre-mortem

Packaging an unchanged host launcher: row 1. Duplicate launch after reconnect: row 1.
Foreign Pod/database access: rows 2–3. Existing registrations broken by a new default:
row 4. Effective projected-token/RBAC and network behavior awaits /29's real-profile gate.

## Evidence

<!-- Independent verifier records revision, image/profile, commands and limits. -->

## Review

Human gate for operator custody and effective scope. All product code and generic deployable
artifacts belong in Assay. No native Windows evidence is required for this Linux cohort.
