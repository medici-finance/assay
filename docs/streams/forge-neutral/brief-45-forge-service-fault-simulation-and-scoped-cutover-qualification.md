---
brief: assay:assay:forge-neutral:45
title: "Forge service fault simulation and scoped cutover qualification"
why: "The service crosses caller, executor, provider, database and cache boundaries. A complete disposable journey with lost replies, revocation and reconstruction is needed before routing real desk writes through it or retiring any direct publisher."
wave: 9
depends: ["forge-neutral/38", "forge-neutral/41", "forge-neutral/43", "forge-neutral/44", "desk-tools/43", "desk-tools/44"]
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm one scoped service cutover only after the served journey, credential inventory, recovery and access-retention evidence are complete. Read release can be accepted independently; unknown effects, missing R3/R4 isolation or surviving direct publisher credentials hold write activation."
decision-trigger: spec
issues: []
outcome: none
schema: brief-v2
version: 2
id: 5306ce94-4e21-44af-82e4-d552eaca0ff5
authored: 2026-10-14 by forge-service authoring
sources:
  - https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/forge-neutral/README.md
  - freshness-checked 2026-10-14 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
  - R6 proposal inspected at c5e8a3f8da3c715b72efa8d8d6b5e74e659ebd52; main merged into this authoring branch without editing historical Evidence
  - "docs/desk-workflow-efficiency-spec.md"
  - "freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d; approved spec and consolidation amendment"
exec-tier: strong
exec-tier-why: Cross-component identity, privacy, concurrency and failure semantics require design judgment and negative controls.
domain: complicated
consumers:
  - "tools/desk/internal/forgeservice/integration/: follow-up forge-neutral/45"
  - "workflow/store/forgejourney_test.go: follow-up forge-neutral/45"
  - "tools/desk/internal/deskkit/citrigger_test.go: follow-up forge-neutral/45"
  - "tools/desk/internal/deskkit/forge_surface_test.go: follow-up forge-neutral/45"
  - "ci/staged-workflows/forge-service.yml: follow-up forge-neutral/45"
  - "docs/forge-service.md: follow-up forge-neutral/45"
  - "docs/enforcement-model.md: follow-up forge-neutral/45"
  - "docs/forge-service-cutover.md: follow-up forge-neutral/45"
  - "docs/proposals/non-ci-architecture/20-amendment-manifest.md: follow-up forge-neutral/45"
  - "changelog/forge-service-qualification.md: follow-up forge-neutral/45"
  - "statusgen/evaluation/: out-of-scope (existing domain evaluation meaning preserved)"
  - "plugins/assay/skills/: out-of-scope (no caller wired to an unshipped service)"
  - "tools/desk/internal/forgeservice/integration/reviewjourney_test.go: follow-up forge-neutral/45"
---

# Brief 45 — Forge service fault simulation and scoped cutover qualification

## Context

files: `tools/desk/internal/forgeservice/integration/` (planned), `workflow/store/forgejourney_test.go` (planned), `tools/desk/internal/deskkit/citrigger_test.go`, `tools/desk/internal/deskkit/forge_surface_test.go`, `ci/staged-workflows/forge-service.yml` (planned), `docs/forge-service.md` (planned), `docs/enforcement-model.md`, `docs/forge-service-cutover.md` (planned), `docs/proposals/non-ci-architecture/20-amendment-manifest.md`, `changelog/forge-service-qualification.md` (planned), `tools/desk/internal/forgeservice/integration/reviewjourney_test.go` (planned). Paths not present at the source pin are planned deliverables; existing
files are amended only at the named seam. No user-home or out-of-repo configuration is edited.

facts:
- 37/38 remain mirror/event owners. 40 supplies authenticated local serving; 41 owns the write-through adapters; 42/43 supply exact-source and derived-cache behavior.
- R2/R3/R4 own store reconstruction, current authority and role publication. A static manifest or fake provider pass does not certify provider permission scopes or a managed deployment.
- This repository is offline against live infrastructure. Operator cutover/live authorization proofs are supplied as a runsheet, not executed by a worker.

layering: forge-service integration and the existing R2/R3/R4 recovery/cutover owners. Pure typed requests and source facts stay independent of
transport/storage/custody; effectful composition remains inside the desk module and R2 store.
Task and served/negative Verify cases below enforce that split.

single-point-of-failure: a green fake/component suite mistaken for active credential containment; actual served client/executor processes plus real SQL faults and explicit operator credential-inventory evidence constrain the claim.

design-fit:
  owner: forge-service integration and the existing R2/R3/R4 recovery/cutover owners
  contract: none — extend existing read/effect owners and register only the new transport or derived-index responsibility in the named docs
  retires: []
  weight: verbs 0; flags 0; refusals 0; rule-text +0; scoped cutover runsheet
  why-add: Reuse existing mirror, effect, Git and custody boundaries instead of adding another authority. The named capability fills a concrete missing consumer seam; measure the implemented delta and do not widen it speculatively.

## Read first

Read [the bounded architecture amendment](https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md), the stream README's dated forge-service
section, and each dependency brief's current contract. All load-bearing requirements for this
brief are restated here. The architecture draft is not activation approval; refresh its revision,
source paths, active claims and existing decisions before dispatch. Missing required contracts
mean NEEDS_CONTEXT, not an invented substitute or a narrowed acceptance test.

## Human decision

Confirm one scoped service cutover only after the served journey, credential inventory, recovery and access-retention evidence are complete. Read release can be accepted independently; unknown effects, missing R3/R4 isolation or surviving direct publisher credentials hold write activation.

Options:
1. **Accept the demonstrated bounded design** — accept only the proven scope and failure behavior; keep live credential provisioning and activation as separate operator acts.
2. **Hold for correction** — identify the unproven boundary or budget and revise before acceptance.

Default if no answer: none — acceptance remains pending. This implementation-evidence decision
does not prevent authoring or independent offline fixture work. Existing mirror retention and
webhook payload decisions are not answered by this section.

## Ground rules

- Isolate changes; never push main, merge, ready-flip, release, dispatch workflows or contact live infrastructure.
- Use synthetic data, disposable processes/repositories and explicit local database fixtures; no production tokens or data.
- Preserve role, candidate, unknown-outcome, required audit/budget and canonical acceptance meanings.
- Stop at implemented; independent verification and acceptance remain separate.
- Do not widen an unsupported provider capability or duplicate another store/evaluator to make a test pass.

## Task

1. Build a complete disposable integration harness: real selected desk client -> authenticated served endpoint -> separate role executor -> fake GitHub/GitLab -> real PostgreSQL intent/receipt/mirror -> client query at observation token. Include exact-SHA source reads and incremental derived queries. Independent literal expectations cover both providers and distinguish intentional unsupported cases.

2. Simulate lost replies, process death at durable transitions, conflicting payloads, SQL outage after provider success, duplicate/reordered/dropped webhook hints, external edits and recovery. Assert no automatic replay of an unknown potentially submitted effect and no older answer masquerading as a minimum-token read. Burst 50 equal authorized reads with no invalidation into one scripted inventory walk, while distinct admissions remain isolated.

3. Bypass ingress and exercise the executor/store/source boundaries: reviewer-role escalation, wrong host/cell/repo/audience/candidate, expiry/revocation, stale incarnation after SQL restore, cross-admission private bodies/counts/errors and shared-object membership. Supply permitted positive controls and deliberately faulty variants so a missing gate cannot pass the fixture.

4. Delete and restore the disposable database under R2 recovery; preserve canonical obligations, unknown effect/usage history and teardown/credential inventory. Show derived indexes are rebuildable and permissions remain current. Compare served read semantics with supported legacy reads at pinned revisions; compare selected guarded write refusals without making real duplicate provider effects.

5. Register cross-module readers and stage the needed CI integration with trigger coverage; do not activate workflows here. Keep fixtures and runshell instructions free of production credentials/private source content. Existing test runners/conformance fixtures are reused, not replaced by another testing framework.

6. Publish a bounded report and operator runsheet for shadow reads, one repository assignment, current R3/R4 containment proof, current provider permissions/entitlement and retention decisions, credential revocation, write-profile enablement and rollback. No operator act is inferred from a green fake. Read-only release is independently qualified; write activation fails if an old direct publisher remains. Do not retire unrelated current commands or change merge/ready/human authority.


### Approved workflow-efficiency amendment — 17 October 2026

Add actual review queue -> offline plan -> preparation -> watcher/report and worker/intake
projection journeys, using desk-tools/43 compatible caller and /44 cost/retirement qualification as
producer evidence. Compare direct and served paths against the same oracle and cost cohort under
scope/permission/head/schema/event-loss faults. No duplicated full sweep for shadow reporting and no
silent loss of wake/monitor health. Distinguish source read qualification, installed plugin/tool/pin
adoption, query-service activation and publisher switch. Existing R3/R4 containment, effect
recovery, zero old publishers and human activation remain required; early direct-reader delivery
does not wait for this service cutover.

This is an amendment to unfinished implementation acceptance. Preserve every prior Task, Verify
obligation, risk decision and Evidence entry. All tests named here are planned deliverables; this
authoring change claims no implementation or operational adoption.

## Interface contract

Use forge-service-v1 from 39, mirror admission/freshness from 37, event hints/demand from 38,
R2/R4 logical effect identity and current R2/R3 authority. Cache/observation tokens never grant
permission. Preserve requester, represented actor, provider executor and exact source inputs.
Provider outcome and local projection availability are separate. Unsupported, stale, incomplete
and permission-denied are explicit; loss does not become success, an empty queue or fresh authority.

## Verify

Named tests below are implementation deliverables, not tests already run. Each command requires
the named PASS line, so a missing test or skipped database case fails. Database tests must use
the inherited graph-execution/19 explicitly disposable PostgreSQL harness and fail when it is
unavailable; no live context or silent skip. Faulty controls exercise the actual production
boundary and independent expectation, not another implementation of the same checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestServedForgeJourney$' ./internal/forgeservice/integration/... > "$r" && grep -F -- '--- PASS: TestServedForgeJourney ' "$r")` | exit 0; named PASS. Actual local clients, served handlers, role-executor processes and real disposable PostgreSQL complete both-provider issue/comment/read/source/derived journeys. Named positive/negative results bind the implementation and independent fixture. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestServedForgeFaultMatrix$' ./internal/forgeservice/integration/... > "$r" && grep -F -- '--- PASS: TestServedForgeFaultMatrix ' "$r")` | exit 0; named PASS. Lost reply, death after send, failed local receipt commit, delayed refresh, reordered/dropped hint and partial provider enumeration never create blind duplicate writes or false complete results. Configured upstream backoff/request budget prevents client fan-out. |
| 3 | check:ci +flow +mutation | `(cd workflow && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeServiceReconstruction$' ./store/... > "$r" && grep -F -- '--- PASS: TestForgeServiceReconstruction ' "$r")` | exit 0; named PASS. Delete/restore disposable SQL: old incarnation is denied, unknown submitted effects remain held, authorized reconstruction restores useful scoped state and derived queries. Missing history never becomes zero budget or renewed permission. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeServiceCutoverInterlock$' ./internal/forgeservice/integration/... > "$r" && grep -F -- '--- PASS: TestForgeServiceCutoverInterlock ' "$r")` | exit 0; named PASS. Disposable inventory proves query/agent processes cannot reach publication credentials. Surviving direct publisher, unavailable R3/R4 evidence, unselected access lease/retention policy or missing provider capability holds write activation; permitted read-only mode remains usable. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/45` | exit 0; implementation diff corroborates routing claims. Out-of-scope claims remain reviewer judgments, not gate-certified passes. |
| 6 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestServedReviewJourney$' ./internal/forgeservice/integration/... > "$o" && grep -F -- '--- PASS: TestServedReviewJourney ' "$o")` | exit 0; named PASS. Production supported clients traverse served queue/plan/context and role issue projections; scope loss/stale head/schema mismatch cannot idle or bypass effects, and duplicate enumeration is counted. |

## Pre-mortem and detection

For each critical Verify case, retain the valid control and a deliberately wrong subject
variant: removed permission/generation check, unknown converted to success, wrong source key,
omitted invalidation or unsafe replay, as applicable. The independent expected result must
reject that variant; record the failing control with the restored production pass. Do not
satisfy a flow row with a route registry, a prose legend or zero selected tests. Test fixtures
qualify their named local boundary; provider permission inventories, deployment workload and
operator cutover remain explicit evidence/acceptance obligations.

## Evidence

<!-- Independent verifier records source/subject revision, environment, literal command,
exit, named result, positive and failing control, and date. No result is asserted at authoring. -->

## Review

Gate: human. Inspect actual dependency/import and credential reachability, private data and
count isolation, current authority and unknown outcomes. Tests cannot authorize provisioning
credentials, switching a managed writer, answering a human decision or changing a merge policy.
