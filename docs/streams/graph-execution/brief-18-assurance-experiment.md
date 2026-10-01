---
brief: assay:assay:graph-execution:18
title: Offline graph, advice and assurance integration proof
why: Independent component tests cannot show that a confident model, a restarted worker or an incomplete audit packet is handled correctly across the whole path. One reproducible fixture suite makes the integration claim reviewable.
wave: 9
depends:
- graph-execution/05
- graph-execution/07
- graph-execution/12
- graph-execution/14
- graph-execution/15
- graph-execution/17
- graph-execution/25
unblocks: []
effort: M
gate: model
risk:
  regulatory: no
  customer: no
  irreversible: no
  sensitive-data: no
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/work-input-amendment.md
- freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3 (pending scope, not implementation)
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
- docs/streams/graph-execution/task-workflow-program.md — execution routing amendment 2026-10-02
- freshness-checked 2026-10-02 @ a944ad1103aadaba919c11fe425089057f5c2f4e
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'statusgen: fixed-here'
- 'drainloop: out-of-scope (consumed through completed contracts, no duplicate executor)'
version: 3
id: 677d16bd-8d1f-4c11-a92d-4815b1f11991
---

# Brief 18 — Offline graph, advice and assurance integration proof

## Context

files: `statusgen/assuranceexperiment.go` (planned), `statusgen/assuranceexperiment_test.go` (planned), `statusgen/testdata/graph-execution/assurance/` (planned), `docs/streams/graph-execution/assurance-experiment-report.md` (planned), `changelog/graph-execution-18-assurance-experiment.md` (planned)

facts: The base eight-case experiment remains runnable without Laya. This extension uses recorded/fake advice; real model metrics must come from 12 and be labeled separately.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Work-input amendment — 2026-09-30

Add WI-5 to this brief's existing connected offline experiment and report. Reuse run/flow
records; do not retrofit the implemented flow brief or create another event store. Exercise
duplicate event, failed-launch retry, unrelated/relevant edit, external policy/API change,
head movement during work, crash after write, overlapping reservations and restored stop/
spend state. Pin expected affected claims before the run and include incomplete manifests.

Export per-work timing, preparation/failed-attempt cost, accepted-outcome denominator,
rediscovery classification provenance and recovery/staleness counters. A deliberately
omitted failed attempt must change the accounting verdict. Counterbalance fixture order
and distinguish replay/fake timings from actual model performance. Include a matched pilot
protocol (same runner/profile first; profile × model second) in the existing report, but
claim no measured quota saving from offline fixtures. No new model benchmark or real
provider execution is part of this brief.

## Task workflow amendment — 2026-10-02

Extend the existing integration suite with production controller, internal review and publication adapters. Include desk/program ownership races, late results, rollback with unknown launch/effect, internal review costs, frozen-candidate mismatch and report-only publication. Retain existing advice/control cases and dependencies; declare each fixture oracle before execution.

Implement the named failure/flow case below in the declared test surface. This amendment does not record implementation evidence or authorize live activation.

## Task

1. Implement GEA-18 integration cases listed in the amendment using real evaluator/coverage/record APIs and a fake external authority. Include confidence-versus-hard-gate, expired calibration, wrong subject, lost ack, stale owner, restore pause, quota and missing control population.
2. Write a reproducible matrix mapping each GEA requirement to fixture/evidence or explicitly deferred capability. Score outcome and permitted method independently. An adapter bypass must fail even if the UI or upper policy is disabled.
3. Generate the report from actual results and retain failed/could-not-check rows. Baseline graph tests still pass. No real deployment, benchmark, certification or pilot-success claims may be derived from this fixture exercise.

## Interface contract

Admission → instance → execution/evidence → lifecycle projection → control export is exercised as a connected offline path.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceExperiment" ./...` | exit 0; output includes PASS for TestAssuranceExperiment, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceExperimentUpperLayerBypass" ./...` | exit 0; output includes PASS for TestAssuranceExperimentUpperLayerBypass, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceExperimentEndToEnd" ./...` | exit 0; output includes PASS for TestAssuranceExperimentEndToEnd, with no [no tests to run] for its owning package |
| 4 | check:ci +flow | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestAssuranceExperimentInterveningChange.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAssuranceExperimentInterveningChange$" ./... > "$wi_out" && grep -q -- "--- PASS: TestAssuranceExperimentInterveningChange " "$wi_out")` | exit 0; named PASS; dispatch → intervening change → result acceptance holds stale evidence and retains artifacts |
| 5 | check:ci +mutation | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestAssuranceExperimentCountsFailedWork.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAssuranceExperimentCountsFailedWork$" ./... > "$wi_out" && grep -q -- "--- PASS: TestAssuranceExperimentCountsFailedWork " "$wi_out")` | exit 0; named PASS; omitted preparation or failed-attempt spend is detected; accepted-work denominator cannot be inflated; mutation: drop failed-attempt spend from the report total — the named test must fail |
| 6 | check:ci +flow +mutation | `(cd statusgen && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAssuranceControllerToPublication$" ./... > "$routing_out" && grep -q -- "--- PASS: TestAssuranceControllerToPublication " "$routing_out")` | exit 0; named PASS; injected stale review or omitted failed cost changes integration verdict |


The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
