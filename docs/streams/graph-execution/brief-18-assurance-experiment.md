---
brief: assay:assay:graph-execution:18
title: Offline graph, advice and assurance integration proof
why: Independent component tests cannot show that a confident model, a restarted worker or an incomplete audit packet is handled correctly across the whole path. One reproducible fixture suite makes the integration claim reviewable.
wave: 5
depends:
- graph-execution/05
- graph-execution/07
- graph-execution/12
- graph-execution/14
- graph-execution/15
- graph-execution/17
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
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'statusgen: fixed-here'
- 'drainloop: out-of-scope (consumed through completed contracts, no duplicate executor)'
version: 1
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

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
