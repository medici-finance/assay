---
brief: assay:assay:cell-api:06
title: Bind the in-cluster cellctl operator to the public service
why: "The dedicated in-cluster operator needs the public service while preserving the existing shared lifecycle path. Binding cellctl to that API prevents a service move from introducing a competing launcher or privileged terminal bypass."
wave: 15
depends: ["cell-api/05", "desk-containers/28"]
unblocks: ["cell-api/07"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the demonstrated in-cluster operator binding and exact managed workload access. A working terminal or API plan must not grant apply authority, compatibility credential access or permission to replace an unknown running owner."
issues: []
schema: brief-v2
version: 3
id: a97eaf30-7338-45eb-8f3a-832a07f4de84
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
consumers:
  - "tools/desk/cmd/cellctl/: follow-up cell-api/06"
  - "tools/desk/internal/cellapi/: follow-up cell-api/06"
  - "tools/desk/testdata/cellapi/: follow-up cell-api/06"
  - "containers/k8s/components/operator/: follow-up cell-api/06"
  - "docs/cell-api-operator.md: follow-up cell-api/06"
  - "changelog/cell-api-operator.md: follow-up cell-api/06"
  - "legacy source consumers: out-of-scope (private source owner retains their exact crosswalk; this brief records public dispositions and does not edit them)"
---

# Brief 06 — Bind the in-cluster cellctl operator to the public service

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `tools/desk/cmd/cellctl/`, `tools/desk/internal/cellapi/` (planned), `tools/desk/testdata/cellapi/` (planned), `containers/k8s/components/operator/` (planned), `docs/cell-api-operator.md` (planned), `changelog/cell-api-operator.md` (planned)

facts:
- pin: public main 15b60414f, inspected 2026-10-20; cellctl and internal/cellop exist.
- prerequisite: desk-containers/28 supplies the dedicated Linux managed-overlay operator, consuming agent-execution/09 and shared runner owners.
- authorization: amendment 21 defers API apply; amendment 28 forbids operator access to compatibility desk custody.
- identity: attach binds cell, namespace, Pod UID, container and generation, never name alone.

layering: Flat operator adapter uses cell-op-v1 and 01 client; trusted runner transport remains outside the service. Task 1–4 and Verify 1–3 prohibit lifecycle and custody shortcuts.

single-point-of-failure: the trusted transport binding of role, cell, Pod UID and generation can authorize the wrong target. Verify 2 bypasses the CLI and tests the transport itself; R3 runtime identity/isolation and R4 effect admission remain lower authorities. Verify 3 holds on ambiguous old-stop instead of assuming replacement is safe.

design-fit:
  owner: Cell-operation CLI adapter; R7 serves projections and R2/R3 execute the existing qualified lifecycle
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

Confirm the demonstrated in-cluster operator binding and exact managed workload access. A working terminal or API plan must not grant apply authority, compatibility credential access or permission to replace an unknown running owner.

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

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner\'s delta; unchanged obligations remain.

## Task

1. Bind existing cellctl status/plan/diagnostic views to the public service/client and existing cell-op-v1 results. Endpoint, trust, namespace and profile come only from operator-owned configuration. Use the component path desk-containers/28 actually delivers; if its contract changes, reconcile this declared path before implementation. Do not create a second operator image or runtime.

2. Preserve start/up, pause/drain/stop/resume and recovery through the existing shared supervisor/runner transport. The service returns projections/plans and does not become the lifecycle executor. In-cluster interaction stays in the dedicated Linux operator container, separate from the service and five role Pods; no native Windows prerequisite.

3. Reuse the managed attach contract with current Pod UID/container/generation validation. Deny exec, portforward and ephemeral debug for every verb; deny compatibility desk/log custody, SQL/admin and publisher access. Reconnecting a terminal only changes presentation. Unknown old-stop observation holds replacement; a new Pod with an old name does not inherit execution authority.

4. Keep API apply absent and R2/R3 recorded-transition allowlist intact while approvals are deferred. Terminal confirmation remains the existing operator convenience, never a new security authorization. Wrong cell, stale plan/generation, operator credential presented by an agent and unavailable service must fail without ambient local/legacy fallback.

5. Preserve perf-v3 invocation/caller context with explicit protocol consent and unknown coverage; optional telemetry cannot change control. Document operator/service/runner flow and recovery, regenerate examples and retain full legacy bindings until 07.

- **Instance amendment:** Bind in-cluster operator projections and attach plans to declared instance/generation and exact managed Pod UID. Preserve cell.env desired population, paused peers and predecessor/successor display; route process lifecycle through the existing runner. Distinct replacement overlap cannot become same-instance unknown-stop relaunch or arbitrary Pod creation, API apply or compatibility credential access.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellOperatorFlow$' ./cmd/cellctl/... > "$o" && grep -F -- '--- PASS: TestCellOperatorFlow ' "$o")` | exit 0; named PASS. Real operator CLI crosses mounted service and shared transport fixture; independently expected cell-op result/holds agree, no second supervisor is started. |
| 2 | check:ci +flow +mutation +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellOperatorBounds$' ./cmd/cellctl/... > "$o" && grep -F -- '--- PASS: TestCellOperatorBounds ' "$o")` | exit 0; named PASS. Bypassed CLI meets existing transport admission; stale UID/generation, foreign cell, agent-held operator intent and prohibited subresources are refused. A planted sibling fallback is detected. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellOperatorRecovery$' ./cmd/cellctl/... > "$o" && grep -F -- '--- PASS: TestCellOperatorRecovery ' "$o")` | exit 0; named PASS. Disconnect, unavailable service and ambiguous old-stop produce zero replacement/launch and no ambient credential fallback; confirmed recovery uses the existing owner once. |
| 4 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/06 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers require review. |
| 5 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./internal/cellapi/integration/... -run '^TestDeskInstanceOperator$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceOperator ' "$o")` | Named test executes (zero matching tests is not a pass); Operator/service/runner fake-API flow selects the correct peer and preserves paused/draining state; same-named changed UID or stale plan refuses. Mutation: attach by role/name alone makes the test fail. |
## Threat model and detection

| Failure mode | Caught by |
|---|---|
| API adapter becomes another launcher or invents lifecycle meanings | row 1 |
| Operator identity reaches a foreign/compatibility workload or stale Pod | row 2 |
| Unknown stop or reconnect creates a second role owner | row 3 |
| A fixture is mistaken for deployment authority or a private inventory is called complete without its owner | No row; review-only: inspect decision provenance, source-owner receipt and explicit operational limits before acceptance. |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or deployment result is asserted. -->

## Review

Gate: human. Review the actual mounted/effect boundary and retained ownership, the
compatibility inventory and the independent negative controls. Test success grants
neither deployment nor credential provisioning, source closure or a human decision.
