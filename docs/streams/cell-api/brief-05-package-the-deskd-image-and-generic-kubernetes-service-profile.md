---
brief: assay:assay:cell-api:05
title: Package the deskd image and generic Kubernetes service profile
why: "A desk workload image and an operator image do not package the service. A separate public deskd image and generic Kubernetes profile make its read, health and network boundaries inspectable before any deployment."
wave: 14
depends: ["cell-api/03", "cell-api/08", "desk-containers/27", "desk-containers/29"]
unblocks: ["cell-api/06"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the service deployment profile, admitted read capability, effective network policy and health/recovery behavior. Operator-selected workload and recovery targets must exist before real-profile exposure."
issues: []
schema: brief-v2
version: 3
id: 682f451b-986d-442c-97d7-acf2f8f4e70b
authored: 2026-10-20 by cell-api authoring
decision-trigger: spec
exec-tier: strong
exec-tier-why: Cross-component compatibility, privacy and authority boundaries require design judgment and independent negative controls.
domain: complicated
outcome: none
sources:
  - docs/streams/cell-api/README.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md
  - docs/proposals/non-ci-architecture/21-cell-operator-surface.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md
  - https://github.com/medici-finance/assay/pull/2890
  - https://github.com/medici-finance/assay/issues/3669
  - freshness-checked 2026-10-20 @ 15b60414f3ec134942a0ec3162b550cd95cbbc8e
  - docs/streams/cell-api/brief-08-build-the-deskd-binary-and-image-from-public-source-through-the-makefile.md
  - freshness-checked 2026-10-27 @ dda6c8c67d3bbdb185c16428609e6e11678ca63a
consumers:
  - "images/deskd/: follow-up cell-api/08"
  - "containers/k8s/components/cell-api/: follow-up cell-api/05"
  - "containers/k8s/components/postgres/: follow-up cell-api/05"
  - "containers/k8s/components/postgres-networkpolicy/: follow-up cell-api/05"
  - "containers/k8s/qualify.sh: follow-up cell-api/05"
  - "tools/desk/internal/cellapi/: follow-up cell-api/05"
  - "tools/desk/testdata/cellapi/: follow-up cell-api/05"
  - "docs/cell-api-kubernetes.md: follow-up cell-api/05"
  - "docs/postgres.md: follow-up cell-api/05"
  - "changelog/cell-api-image.md: follow-up cell-api/05"
  - "Makefile: follow-up cell-api/05"
  - "scripts/build-windows.ps1: follow-up cell-api/05"
  - "tools/desk/internal/deskkit/citrigger_test.go: follow-up cell-api/05"
  - "legacy source consumers: out-of-scope (private source owner retains their exact crosswalk; this brief records public dispositions and does not edit them)"
---

# Brief 05 — Package the deskd image and generic Kubernetes service profile

## Context

files: `images/deskd/` (planned by cell-api/08; consumed here), `Makefile`, `scripts/build-windows.ps1`, `tools/desk/internal/deskkit/citrigger_test.go`, `containers/k8s/components/cell-api/` (planned), `containers/k8s/components/postgres/` (planned), `containers/k8s/components/postgres-networkpolicy/` (planned), `containers/k8s/qualify.sh` (planned), `tools/desk/internal/cellapi/` (planned), `tools/desk/testdata/cellapi/` (planned), `docs/cell-api-kubernetes.md` (planned), `docs/postgres.md` (planned), `changelog/cell-api-image.md` (planned)

facts:
- pin: public main 15b60414f, inspected 2026-10-20; images/deskd and service component are planned.
- separation: the service, dedicated Linux operator and five role Pods have distinct capabilities and lifecycle.
- storage: desk-containers/27 owns the CNPG profile and its per-cell owner/login role. That existing role is not a service read identity; this brief owns the proposed additive read-role/profile amendment, under the same database owner.
- build: cell-api/08 owns the deskd binary and local image build (`images/deskd/Dockerfile`, `make deskd-build`, `make deskd-image`). Inspected again at public main dda6c8c67 on 2026-10-27, the root `Makefile` documents `desk-install` as HUMAN-ONLY, and `tools/winparity` holds its `.PHONY` set equal to `scripts/build-windows.ps1`'s parity block.
- limits: no production latency, density, RTO or RPO target is established by authoring.

layering: A service host is justified by read-side lifecycle and custody separation; it composes 03 and existing R6, never agent/operator machinery. Verify 1–3 and 6 inspect rendered and local disposable fixtures; Verify 7 separately qualifies actual policy in an explicitly authorized disposable profile.

single-point-of-failure: rendered network policy is the reachability control; CNI enforcement must be measured in Verify 7. If it is bypassed, the service read-only database identity limits SQL access (Verify 6), while 03 store admission and R4 executor checks remain separate. Neither a manifest nor a read-only role prevents private read disclosure by itself; both effective policy and mounted authorization are required.

design-fit:
  owner: R7 service host packaging; desk-containers owns the operator/desk and database profiles
  contract: none — R7's transport responsibility is registered by cell-api/01; existing semantic owners and contracts remain controlling
  retires: []
  weight: verbs +0; flags +0; refusals +0; rule-text +0 (no new operator vocabulary proposed; record any actual implementation delta for review)
  why-add: Reuse the existing owners and one client seam; remove duplicate service composition only after the last-consumer proof. Package/process additions are bounded by the declared files, not permission to add another controller.

All planned paths and named tests below are implementation deliverables. The source pin
proves the starting point, not that these deliverables exist or were tested. The
[stream refactor map](README.md#bounded-refactor-map) defines the public transfer boundary.

## Read first

Read the [R7 ownership and admission record](README.md#authority-and-admission),
[cell operator contract](../../proposals/non-ci-architecture/21-cell-operator-surface.md),
[forge-service boundary](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md),
[interactive Kubernetes direction](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md),
`spec/cell-op-v1.md`, `docs/contracts.md` and `docs/go-engineering.md`.
Resolve every declared dependency's current contract and independent Evidence before
pickup. The package mapping is a reviewable proposal; a conflicting contract is
NEEDS_CONTEXT, never permission to select new custody or duplicate its owner.

## Human decision

Confirm the service deployment profile, admitted read capability, effective network policy and health/recovery behavior. Operator-selected workload and recovery targets must exist before real-profile exposure.

Options:
1. **Accept the demonstrated bounded design** — accept the evidenced compatibility and containment for its declared scope; provisioning, deployment and source retirement retain their separate operator acts.
2. **Hold for correction** — name the missing proof or incompatible behavior; retain the current supported path until corrected.

Default if no answer: none — blocks until answered

The decision becomes concrete from the implementation design and evidence. No answer
keeps acceptance pending; it grants no time-based default or activation authority.

## Ground rules

- Work in an owned branch; no merge, ready flip, release, installation or live infrastructure contact.
- Public source and fixtures must be self-contained and synthetic. Never copy private locators, configuration, credentials, incidents or history into Assay.
- Preserve the single R2 PostgreSQL runtime, R3 isolation, R4 role authority and R6 forge mirror. A service, terminal, restored file or cached result grants no authority.
- Keep this stream parked until admitted. Stop at implemented; independent verification, human acceptance and operational cutover remain distinct.

## Task

1. Consume cell-api/08's deskd image; do not fork, re-implement or widen its build. Declare the artifact pin: the image digest bound to the source-SHA tag that 08's `make deskd-image` produced. Also declare the supported service/client matrix. Every profile, receipt and `deskd-deploy` reference names that digest, never a mutable tag. A change the profile needs in the image goes through 08's contract and its Verify rows, not a second Dockerfile here. Publishing an image remains a separate release act.

2. Create the generic offline-renderable Kubernetes service component with explicit cell/source bindings and default-deny ingress and egress. Allow only named qualified clients to its read API and the service to the existing projection endpoint and required DNS; namespace and Pod selectors must intersect, never accidentally broaden into alternative peers. The initial profile denies agent-Pod ingress to the service and agent access to SQL/publishers. No privileged host mounts, host network/ports, Docker socket, wildcard RBAC, ambient kubeconfig or agent/operator credential mounts. Render a ClusterIP Service only; no Ingress, NodePort, LoadBalancer or external IP. A policy removed, widened or not enforced is a failed exposure gate.

3. Wire only the existing R6 reader and approved R2 projection/storage access. Propose one separate service read-only role per cell in the existing database, with CONNECT, required schema USAGE and SELECT only on an explicit allowlist of approved projection views. It owns no database/schema/table and has no owner-role membership or SET ROLE path, write/DDL/TEMPORARY grant, privilege-administration, replication, BYPASSRLS or mutating-function execution. Test inherited/PUBLIC/default grants too; a `default_transaction_read_only` setting alone is not a privilege boundary. No second database, mirror or adopted legacy SQLite coordination.

   This brief amends the artifacts delivered by desk-containers/27: an additive read-role/grants template in the existing postgres component, the service's narrow peer rule in the postgres-networkpolicy component, and the provisioning/rotation/revocation procedure in `docs/postgres.md` (planned). The existing CNPG/database owner reviews and provisions those grants and the role Secret out of band; deskd never provisions itself. No existing owner role is widened or mounted in the service, and there is no new broker. Public templates carry only Secret references: the service mounts exactly its scoped read-role Secret and required non-secret TLS trust, never cell-owner, database-admin, human, backup or publisher credentials. Service-account Secret reads and administrative Pod subresources remain forbidden under the existing R3 boundary. Missing approved grants/identity or unavailable projection access holds readiness; there is no owner-credential fallback.

4. Distinguish process live, dependencies ready, projection freshness and acceptance: stale/unavailable authorization or data reports degraded readiness/coverage, with no private counts in probes. Qualify restart, graceful termination, SSE resync and dependency outage with local disposable processes and offline render checks. A Pod restart or reconnect cannot launch agents, renew authority or replay effects. Scope retry/load behavior to inherited forge budget and perf-v3 observations; add no invented production budget.

5. Own the service-specific qualification here. Deliver `containers/k8s/components/cell-api/qualify.sh` (planned) for a separately authorized, isolated disposable profile: require explicit opt-in, profile, kubeconfig and context; print the target before contact. Reuse desk-containers/29's qualification harness and receipt-bound target guard: resolve endpoint/TLS identity offline, compare it to the run-owned receipt and pin it on connection before any API call. Refuse substituted files/endpoints, ambient, shared or production contexts. Exercise actual CNI reachability with real rendered resources and the scoped service identity; removing policy or widening selectors must fail the expected matrix. Its receipt binds image digest, rendered policy/profile hash, database grants/identity, target and test results. Missing tooling, skipped probes, unreadable results or mismatched receipts are could-not-check and nonzero, never success. This implementation does not contact a live cluster; an authorized operator executes Verify 7 separately before exposure. desk-containers/29's existing suite does not cover this new service. Brief 07 must dereference this receipt as a prerequisite, not invent or substitute one.

6. Document image/profile pins, secret-reference custody, the network/health matrix and rollback; regenerate examples offline. Admission and human acceptance of the proposed read-role/profile amendment precede provisioning or exposure. Existing R2/R3/R4 owners retain authority, and separate operational acts remain required.

7. Add a HUMAN-ONLY `deskd-deploy` target to the root `Makefile`, documented in its header beside `desk-install`. It applies this brief's rendered profile, pinned to Task 1's image digest, to one explicitly named target:
   - It requires explicit `DESKD_PROFILE`, `DESKD_KUBECONFIG` and `DESKD_CONTEXT` arguments. It passes `--kubeconfig` and `--context` on every kubectl call. It ignores and refuses an ambient `KUBECONFIG` or the default kubeconfig file, and a missing argument is a named refusal.
   - Before any cluster contact it prints the context, API endpoint, namespace, image digest and rendered-profile hash.
   - It reuses desk-containers/29's receipt-bound target guard, as Task 5 does. It refuses ambient, shared, production or receipt-unbound contexts and substituted kubeconfig files or endpoints, with zero cluster calls.
   - It deploys only the digest-pinned image, never a mutable tag.
   - It resolves `kubectl` through PATH, so the existing cluster-guard shim and its explicit operator opt-in still apply. It adds no bypass of that shim.
   - A human operator runs it, as with `desk-install`. No CI workflow, plugin, desk tool or agent dispatch invokes it.
   - Add it to `.PHONY` and the `scripts/build-windows.ps1` parity block. Its Windows handler is a named non-zero refusal, and `tools/winparity` stays green.
   - Deliver `TestDeskdDeployRefusesUnboundTargets` (planned; Verify 8). It runs the real recipe extracted from the `Makefile` under `/bin/sh`, because CI has no make. A temporary HOME and a recording fake `kubectl` on PATH stand in for the cluster. Register the test in `ciCrossModuleRegistry`.

## Interface and acceptance contract

Consume cell-api/01's versioned HTTP/JSON and SSE seam and the existing cell-op-v1
result/plan meanings. Authentication, current authorization, provider custody and
acceptance are separate. Required coverage/freshness, exact subject and unknown outcomes
survive every adapter. No agent-facing legacy human-answer or adopt tools; no generic role,
URL, SQL-write, token or apply escape hatch. Human named actions remain closed and
separately qualified. Initial service exposure is read/plan-only; approval-based apply
stays absent until amendment 21's five R7/R3 properties are independently qualified.

Update the declared documentation and generated schema/client/profile examples affected
by the Task. Preserve prior Evidence and source history; unresolved consumer or contract
coverage is a hold. Do not invent deployment performance/recovery thresholds: reuse the
inherited workload and require the operator to select missing targets before exposure.

## Verify

These exact named tests must be delivered and run, with a recorded failing control
before the restored pass. Each uses real production composition and an independently
specified expected result; zero selected tests or a skipped required fixture is failure.
Only explicit disposable fixtures may open local listeners/databases. No production or
ambient kubeconfig is permitted. Real-profile qualification remains a separate operator
act; offline passes do not establish it.

For row 4, the independent verifier obtains the actual implementation delivery from its
merged PR and records its full immutable SHA in `ASSAY_DELIVERY_SHA` and its actual
first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither defaults to HEAD, origin/main or
this authoring PR. Both commits and the fetched mainline must exist locally; the command
requires the delivery to have landed on that mainline before checking its immutable
snapshot. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed. Record
exact corroborated/disproved/unchecked counts; unchecked required consumers remain held
and omitted consumers remain a review obligation. These are future delivery inputs.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +dereference +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestDeskdProfileRender$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestDeskdProfileRender ' "$o")` | exit 0; named PASS. Actual image and rendered components contain no credential values and reference only the scoped read-role Secret plus TLS trust; planted owner/admin/human/publisher Secret, wildcard privilege or second SQLite runtime fails. Missing default-deny ingress/egress, widened peer selectors, agent peer, host network/port, Ingress, NodePort, LoadBalancer or external IP each fails a named negative fixture. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestDeskdHealthStates$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestDeskdHealthStates ' "$o")` | exit 0; named PASS. Real service health distinguishes alive, ready and stale/unavailable projection; private counts are absent and injected false-ready behavior fails. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestDeskdRestart$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestDeskdRestart ' "$o")` | exit 0; named PASS. Real disposable service restart/disconnect/resync under dependency failure creates zero dispatch/effect/lease renewal; old cursor and inaccessible cache fail safely. |
| 4 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/05 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers require review. |
| 5 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |
| 6 | check:ci +flow +mutation +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestDeskdDatabaseScope$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestDeskdDatabaseScope ' "$o")` | exit 0; named PASS with a real disposable PostgreSQL fixture and production grant templates. Bypass HTTP and connect with the service role: approved projection SELECT succeeds; DML/DDL/TEMP, owner SET ROLE, mutating functions, non-allowlisted data and foreign-cell database access fail. Planted owner-role Secret or inherited/PUBLIC/default write grant fails the oracle; absent database fixture fails, never skips. |
| 7 | gate:human +flow +mutation +neighbour | `test -n "$ASSAY_CELL_API_PROFILE" && test -n "$ASSAY_CELL_API_KUBECONFIG" && test -n "$ASSAY_CELL_API_CONTEXT" && test -n "$ASSAY_CELL_API_RECEIPT" && sh containers/k8s/components/cell-api/qualify.sh --allow-cluster-queries --profile "$ASSAY_CELL_API_PROFILE" --kubeconfig "$ASSAY_CELL_API_KUBECONFIG" --context "$ASSAY_CELL_API_CONTEXT" --receipt "$ASSAY_CELL_API_RECEIPT"` | exit 0 only with human authorization and receipt-bound API/TLS identity in the owned disposable profile. Actual CNI allows qualified client-to-service and service-to-projection reads; agent/foreign-namespace ingress, agent-to-database/publisher and unauthorized service egress fail with application ingress bypassed. Missing/widened policy mutations fail; record exact image/profile/grants-bound receipt and controls. An offline render pass, absent receipt or unavailable probe cannot satisfy this row. |
| 8 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestDeskdDeployRefusesUnboundTargets$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestDeskdDeployRefusesUnboundTargets ' "$o")` | exit 0; named PASS over the real `deskd-deploy` recipe from the root `Makefile`, run under `/bin/sh` with a temporary HOME and a recording fake `kubectl`. Each of these exits non-zero with a named reason and records zero kubectl calls: missing profile, missing kubeconfig, missing context, ambient `KUBECONFIG` only, default kubeconfig file only, shared or production context, endpoint or TLS identity not matching the receipt, substituted kubeconfig, and mutable image tag. The bound disposable fixture prints context, endpoint, namespace, digest and profile hash before its first kubectl call, and every call carries explicit `--kubeconfig` and `--context`. A static scan of `.github/workflows/`, `plugins/` and `tools/desk/cmd/` finds no `deskd-deploy` invocation. Deleting any one refusal clause from a scratch copy of the recipe fails the test. |
| 9 | check:ci +neighbour | `(cd tools/winparity && GOWORK=off go run . --root ../..)` | exit 0; prints `winparity: OK`. The Makefile `.PHONY` set and the Windows parity block are equal and include `deskd-deploy`, and the Windows handler is a named refusal, never a silent success. |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| Service image/profile gains operator powers or private build residue | row 1 |
| Service is reachable from agent Pods or outside its admitted profile, or forwards unauthorized access to database/publisher paths | rows 1 and 7; 03's mounted authorization remains independently required |
| Service receives the cell owner/login role or inherits write/DDL/foreign-cell access despite a read-only claim | rows 1 and 6; direct database checks bypass HTTP |
| Healthy process is reported as healthy data/acceptance | row 2 |
| Restart/reconnect becomes new runtime ownership or repeats work | row 3 |
| Offline policy render or an unrelated qualification receipt is mistaken for effective service containment | row 7; missing or mismatched evidence holds exposure |
| `deskd-deploy` reaches an ambient, shared, production or substituted cluster, or deploys a mutable tag | row 8 (zero kubectl calls on every refusal); the cluster-guard shim's operator opt-in stays a separate layer |
| `deskd-deploy` is run by CI or an agent instead of a human operator | row 8's static no-invocation scan; review-only beyond it |
| The image deployed differs from the one cell-api/08 built and checked | Task 1's digest pin, which row 8 and row 7's receipt both bind |
| The Windows script drifts from the Makefile | row 9 |
| A fixture is mistaken for deployment authority or a private inventory is called complete without its owner | No row; review-only: inspect decision provenance, source-owner receipt and explicit operational limits before acceptance. |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or deployment result is asserted. -->

## Review

Gate: human. Review the actual mounted/effect boundary and retained ownership, the
compatibility inventory and the independent negative controls. Confirm that Task 1 pins
cell-api/08's image rather than re-building it, and that `deskd-deploy` stays a
human-only act behind explicit, receipt-bound target arguments. Test success grants
neither deployment nor credential provisioning, source closure or a human decision.
