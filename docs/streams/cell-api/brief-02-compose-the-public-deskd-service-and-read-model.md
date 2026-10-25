---
brief: assay:assay:cell-api:02
title: Compose the public deskd service and read model
why: "Moving only a binary leaves duplicate indexes and embedded lifecycle behavior. Composing the public service over existing readers gives every client one truthful read model without another scheduler or authority store."
wave: 9
depends: ["cell-api/01", "forge-neutral/37", "graph-execution/21"]
unblocks: ["cell-api/03"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm service data minimization and legacy state classification, including presentation markers and derived indexes. Fixture success does not approve a network listener or new credential custody."
issues: []
schema: brief-v2
version: 3
id: 45642e26-2004-4ea3-be9d-3b62fb269259
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
  - "tools/desk/cmd/deskd/: follow-up cell-api/02"
  - "tools/desk/internal/cellapi/: follow-up cell-api/02"
  - "tools/desk/testdata/cellapi/: follow-up cell-api/02"
  - "docs/cell-api-service.md: follow-up cell-api/02"
  - "changelog/cell-api-service.md: follow-up cell-api/02"
  - "legacy source consumers: out-of-scope (private source owner retains their exact crosswalk; this brief records public dispositions and does not edit them)"
---

# Brief 02 — Compose the public deskd service and read model

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `tools/desk/cmd/deskd/` (planned), `tools/desk/internal/cellapi/` (planned), `tools/desk/testdata/cellapi/` (planned), `docs/cell-api-service.md` (planned), `changelog/cell-api-service.md` (planned)

facts:
- pin: public main 15b60414f, inspected 2026-10-20; cmd/deskd and internal/cellapi are absent.
- reuse: forgeread root/offline and loopadmin/runner exist; planned mirror/controller packages are dependencies, not permission to duplicate them.
- authority: R2 PostgreSQL owns coordination; R6 owns one mirror; Git/provider records retain their authority.
- legacy: the earlier service combines an index, presentation markers, routes and process adapters; that composition is not the target ownership.

layering: Thin command and effectful adapters around deterministic projection mapping. Task 1–3 and Verify 1–3 forbid hidden polling, SQL authority and lifecycle forks.

single-point-of-failure: the read-model adapter can turn stale or partial source state into a false authoritative result. R1/R2 provenance and the existing R6 reader remain the source of truth; Verify 1–3 compare their independent fixtures and detect hidden effects or legacy authority adoption. No ingress bypass is claimed here: Task 3 keeps serving local synthetic inputs until 03.

design-fit:
  owner: R7 service composition; R1/R2/R6 retain evaluation, coordination and forge ownership
  contract: none — R7's transport responsibility is registered by cell-api/01; existing semantic owners and contracts remain controlling
  retires: []
  weight: verbs +1 public command; flags +0 new vocabulary; refusals +0 new policy; rule-text +0 (rehome the existing service or client command; record the actual implementation delta)
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

Confirm service data minimization and legacy state classification, including presentation markers and derived indexes. Fixture success does not approve a network listener or new credential custody.

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

1. Land reusable deskd service code in the public desk module, with a thin command and internal composition. Implement selected inbox/cell/work/status/hold projections through R1 domain results, R2/controller observations and R6 authorized mirror, using 01’s contract and synthetic fixtures. Preserve exact subjects and per-input coverage/freshness; no second acceptance evaluator.

2. Replace legacy direct forge polling with R6’s admitted reader. Add no poll scheduler, cache database, effect ledger or token cache. Classify every legacy SQLite field as rebuildable projection, presentation marker, retained history or unsupported input. Necessary durable markers use the existing approved storage owner; no legacy claim/lease becomes current runtime authority. Unknown mappings hold migration.

3. Start only against explicitly supplied local synthetic inputs until 03 qualifies ingress. Startup and reconnect cannot launch desks, acquire work or replay effects. Dependency loss reports stale/partial/could-not-check, not an empty healthy queue. Bounded duplicate subscriptions reuse owner observations rather than refreshing upstream per client. Keep the legacy service until 07.

4. Preserve perf-v3 logical-call, handler and actual upstream counts with unknown coverage. Never attribute process-wide concurrent CPU to every handler. Optional diagnostics cannot change required audit, budget or authority. Document state mappings and restore compatibility and regenerate affected fixtures.

- **Instance amendment:** Project instance and aggregate runtime state from existing R2/R3 observations, including desired revisions and partial replacement; keep per-instance provenance/freshness and unknown occupancy. No local authoritative population/lease store or server-side scheduler. Reconnect/resync neither creates instances nor resumes paused desks.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellReadModel$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestCellReadModel ' "$o")` | exit 0; named PASS. Real composition projects independent mirror/controller fixtures; revoked admission, partial scan, wrong candidate and false accepted/fresh output fail. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellNoRuntimeFork$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestCellNoRuntimeFork ' "$o")` | exit 0; named PASS. Real startup/reconnect has no extra scheduler, coordination store, direct forge poll or token mint; a planted hidden launch/refresh fails the effect oracle. |
| 3 | check:ci +dereference +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellStateMapping$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestCellStateMapping ' "$o")` | exit 0; named PASS. Legacy fields have evidenced dispositions; a SQLite lease promoted to live authority or unmapped presentation marker holds handover. |
| 4 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/02 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers require review. |
| 5 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./internal/cellapi/... -run '^TestDeskInstanceReadModel$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceReadModel ' "$o")` | Named test executes (zero matching tests is not a pass); Mounted service keeps two workers/three reviewers plus paused/draining state distinct across reconnect, dependency loss and revision change. Mutation: collapse by role or turn missing occupancy into zero makes the test fail. |
## Threat model and detection

| Failure mode | Caught by |
|---|---|
| Read model rewrites uncertainty or acceptance | row 1 |
| Service quietly becomes a poller or launcher | row 2 |
| Legacy database state becomes fresh authority | row 3 |
| A fixture is mistaken for deployment authority or a private inventory is called complete without its owner | No row; review-only: inspect decision provenance, source-owner receipt and explicit operational limits before acceptance. |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or deployment result is asserted. -->

## Review

Gate: human. Review the actual mounted/effect boundary and retained ownership, the
compatibility inventory and the independent negative controls. Test success grants
neither deployment nor credential provisioning, source closure or a human decision.
