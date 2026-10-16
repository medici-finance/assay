---
brief: assay:assay:performance-collection:09
title: "Bounded performance issue and repair-PR loop"
why: "Persistent measured regressions should enter one accountable investigation and repair proposal instead of repeated alerts or speculative automatic changes."
wave: 7
depends: ["performance-collection/08", "graph-execution/04", "graph-execution/09", "graph-execution/14", "graph-execution/16", "graph-execution/26"]
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Report collection, exposure and recurring effect boundaries need acceptance of the concrete implementation; no live permission is granted by this draft."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: cba621ef-40f1-44de-ac2f-0ed90323628a
authored: "2026-10-15 by reporting authoring session"
sources:
  - docs/streams/performance-collection/README.md
  - docs/streams/continuing-operations/spec.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - "freshness-checked 2026-10-15 at public main d430e8f18"
exec-tier: strong
exec-tier-why: "Cross-source identity, coverage, authority and migration must agree."
domain: complicated
outcome: none
consumers:
  - "perfdata/assessment/: follow-up performance-collection/09"
  - "perfdata/proposals/: follow-up performance-collection/09"
  - "perfdata/testdata/performance-repair/: follow-up performance-collection/09"
  - "plugins/assay/skills/operating-report/SKILL.md: follow-up performance-collection/09"
  - "docs/streams/continuing-operations/spec.md: follow-up performance-collection/09"
  - "tools/desk/internal/deskkit/autolane.go: follow-up performance-collection/09"
  - "tools/desk/internal/deskkit/autolane_test.go: follow-up performance-collection/09"
  - "tools/desk/cmd/deskautolane/lane.go: follow-up performance-collection/09"
  - "docs/performance-collection.md: follow-up performance-collection/09"
  - "docs/contracts.md: follow-up performance-collection/09"
  - "changelog/performance-repair-proposals.md: follow-up performance-collection/09"
---

# Brief 09 — Bounded performance issue and repair-PR loop

## Context

files: `perfdata/assessment/` (planned), `perfdata/proposals/` (planned), `perfdata/testdata/performance-repair/` (planned), `plugins/assay/skills/operating-report/SKILL.md` (planned), `docs/streams/continuing-operations/spec.md`, `docs/performance-collection.md` (planned), `docs/contracts.md`, `changelog/performance-repair-proposals.md` (planned), `tools/desk/internal/deskkit/autolane.go`, `tools/desk/cmd/deskautolane/lane.go`

facts:
- This is authored todo scope, not deployed capability. Collection briefs 01/02 produce the bounded source contract; no database or remote endpoint is needed by local reports.
- Existing work/acceptance, runner usage and operational audit retain their owners. `statusgen/flow.go` is package main, not an importable library; consume its versioned saved export, not an invented cross-module import.
- perf-v1 has seven-day raw retention, own-process CPU and partial forge/child counters. Preserve source limits; absent history stays unknown.
- Source pin is in frontmatter; refresh actual source, claims and prerequisites at pickup.

layering: pure validation/aggregation and typed snapshot consumers; adapters own I/O and effects. Future cell coordination and effect execution remain with R2/R3/R4; optional reporting cannot grant authority.

design-fit:
  owner: perfdata report/proposal consumer over existing source semantics
  contract: performance-event-v1 and bounded snapshots from 01/02; register the consumer in docs/contracts.md
  retires: [temporary local perf-v1 triage reader after released parity qualification]
  weight: "Measure added verbs, flags, refusals and cost at implementation; do not invent savings."
  why-add: "Existing source owners lack this composed operator journey. Reuse their outputs rather than a second historian, scheduler or acceptance evaluator."

single-point-of-failure: source coverage and publication scope — independent expected fixtures and lower-boundary authorization tests reject unsupported complete/authorized claims.

## Read first

Read the stream README, docs/library-first.md, docs/report-packs.md, continuing-operations spec sections 5/9, and runtime-contract-alignment. All required outcomes are restated in Tasks and Verify.

## Human decision

Choose the pilot subject/workload, versioned regression and coverage policy, occurrence budget and permitted issue-only or draft-PR proposal mode. Demonstrate recovery and lower executor limits first. No autonomous merge, policy rewrite or production intervention is requested.

Options:
1. Accept the demonstrated scope, source coverage and access/effect boundaries; select the concrete operating profile before activation.
2. Hold the unsupported boundary and continue offline fixture work only.

Default if no answer: none — acceptance/activation remains held; offline implementation can proceed.

## Ground rules

- Own isolated branch and draft PR; no main push, merge, live deployment or workflow dispatch.
- Synthetic sources and fake effect boundaries until the applicable operating policy is recorded.
- Stop at implemented; independent verifier fills Evidence.
- Missing coverage, authority or effect outcome never becomes zero, success or permission to retry.

## Task

1. Implement a pure versioned performance/collection-health assessment profile over 07/08 snapshots. Policy declares compatible workload/build/platform/cohort, baseline, coverage/sample sufficiency, persistence, cooldown and allowed investigation scope. No hard-coded universal CPU/call thresholds. Save no-change/investigate/propose-change/escalate, evidence and uncertainty; incomplete inputs cannot justify speculative product repair.
2. Produce a dry-run proposal packet before effects. Support shadow, issue-only and draft-repair modes through the existing intake/worker interfaces, not a privileged forge-writing reporter. Deduplicate by qualified subject/rule/baseline/episode and record disposition, issue/PR and source snapshot lineage. Missing-data cases route collection health separately. Existing finding/issue owners remain authoritative.
3. Bind managed occurrences to the declared graph runtime prerequisites, current policy and R2 schedule/lease/budget ownership. Use R3 supervision and R4 stable effect intent/receipt reconciliation. Repeated triggers, crash and ambiguous lost ACK cannot mint a second issue or launch. Before qualification, only offline/manual proposal preview is available; do not introduce an interim autonomous scheduler or claim that syntax checks qualify runtime.
4. Enforce configured per-occurrence and cumulative tokens/cost, CPU/wall, provider-call, concurrency and issue/PR limits at the existing admission/executor boundary. Unknown accounting or stale authority holds effects. The assessor cannot self-edit policy, raise its limits, remove a security gate, deploy, merge or change live infrastructure.
5. Route a bounded investigation to an existing worker: reproduce the symptom, identify the owning mechanism and create a candidate/draft PR with comparable before/after performance and the correctness floor. Unreproduced or inconclusive findings remain open with reason; metric rank alone cannot mandate a rewrite. Existing independent review, verification and human merge gates apply. Repair PRs from this loop are never admitted to an automated merge lane (for example the auto-approve lane): a human merges each one, and no lane enactment changes that for this profile. The lane-side consumer is `deskkit.AdmitAutoLane` (`tools/desk/internal/deskkit/autolane.go`): add a category tripwire (for example `performance-repair-pr`) that fires on a 09 repair PR, so the lane refuses it whatever the score or the area opt-in says. A repair PR is identified by two markers the proposal executor writes at creation, either of which alone trips the wire: the label `performance-repair`, and the body line `<!-- assay:performance-repair -->`. The wire also fires when the PR's label timeline holds any application of `performance-repair`, even if the label was later removed: `AdmitAutoLane` gains an `Events` input (and an events-read-error input, which trips `unreadable`) that `facts.admit` in `tools/desk/cmd/deskautolane/lane.go` fills from the label timeline it already reads (`f.events`), so no new forge read is added. A create path that cannot apply either marker refuses the effect instead of opening an unmarked PR. The executor posts as the existing worker App, as Task 2 requires, so authorship cannot be a third signal (the lane already treats any non-worker author as outside it); the residual is stated plainly: an executor that omitted both markers would be caught only by row 3 and by the human opt-in and review upstream. Path and area membership cannot identify a repair PR, since it edits ordinary product code; the markers do, and the existing tripwires continue to judge every PR that carries neither.
6. Keep proposed, merged, awaiting outcome check, improved/inconclusive/adverse states distinct. An independently assessed after-window uses the same declared workload/definition or explains incompatibility; merge alone cannot close the improvement claim. Stop/revert/escalation recommendations route through existing owners. Measure the loop own collection/evaluation cost and missed occurrences. Document managed runtime activation evidence and a bounded pilot disposition.

## Verify

Named commands below are implementation deliverables. Each must run the named test (not succeed with no matching tests).

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./assessment/... -run '^TestPerformanceAssessmentCoverage$' > "$o" && grep -F -- '--- PASS: TestPerformanceAssessmentCoverage ' "$o")` | exit 0; named PASS; compatible persistent fixture regression nominates once; sparse/missing/mismatched cohort returns gap/hold |
| 2 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./proposals/... -run '^TestRepairProposalIdempotency$' > "$o" && grep -F -- '--- PASS: TestRepairProposalIdempotency ' "$o")` | exit 0; named PASS; duplicate triggers, crash and lost ACK reconcile one issue/attempt identity |
| 3 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./proposals/... -run '^TestRepairAuthorityAndBudgets$' > "$o" && grep -F -- '--- PASS: TestRepairAuthorityAndBudgets ' "$o")` | exit 0; named PASS; lower executor refuses stale authority, exceeded/unknown budgets, merge, policy self-modification and the executor never applies the lane admission label to a repair PR, even with the upper filter bypassed; every repair PR the executor creates in draft-repair mode carries both the `performance-repair` label and the `<!-- assay:performance-repair -->` body line, and a create path that cannot apply either refuses the effect rather than opening an unmarked PR |
| 4 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./proposals/... -run '^TestRepairProposalJourney$' > "$o" && grep -F -- '--- PASS: TestRepairProposalJourney ' "$o")` | exit 0; named PASS; dry-run makes no writes; accepted fixture routes one draft PR and independent correctness evidence |
| 5 | check:ci | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./assessment/... -run '^TestRepairOutcomeWindow$' > "$o" && grep -F -- '--- PASS: TestRepairOutcomeWindow ' "$o")` | exit 0; named PASS; merged repair remains awaiting observation; adverse/inconclusive result cannot be labelled improved |
| 6 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./internal/deskkit/ -run '^TestAutoLaneRefusesPerformanceRepairPR$' > "$o" && grep -F -- '--- PASS: TestAutoLaneRefusesPerformanceRepairPR ' "$o")` | exit 0; named PASS; `AdmitAutoLane` refuses a repair PR that is otherwise admissible (worker-App author, valid trailer, in-area path, risk classifier clear) when it carries only the label, and again when it carries only the body marker, with the perfdata executor and its refusals never consulted; the same PR whose label timeline holds a `performance-repair` application is refused although the label was later removed and the body marker is absent (an unreadable timeline trips `unreadable`); the same PR without either marker and with no such timeline entry is judged by the existing tripwires unchanged |
| 7 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/09` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |

## Evidence

<!-- Independent verifier: revision, environment, command, exit, observed evidence/date/runner. -->

## Review

Gate: human. Verify evidence and lower-boundary failure cases; acceptance does not activate a destination, cadence, credential, policy change or merge permission.
