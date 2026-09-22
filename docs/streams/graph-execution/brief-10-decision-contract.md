---
brief: assay:assay:graph-execution:10
title: Typed advice and separate deterministic policy records
why: Today bounded advice has no durable description of probability, calibration or applicability. Consumers need to distinguish an uncertain suggestion from an authorized policy result without breaking existing Decide callers.
wave: 0
depends: []
unblocks:
- graph-execution/11
- graph-execution/13
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
- 'tools/desk/internal/deskkit/decide.go: fixed-here'
- 'tools/desk/internal/runnertable/decider.go: out-of-scope (tier separation remains unchanged)'
- 'tools/desk/internal/deskkit/admission.go: follow-up graph-execution/13'
version: 1
id: 2a0db879-6a28-4530-8f8a-c1b5c54ca540
---

# Brief 10 — Typed advice and separate deterministic policy records

## Context

files: `spec/decision-assessment-v1.md` (planned), `schemas/decision-assessment-v1.json` (planned), `tools/desk/internal/deskkit/decide.go`, `tools/desk/internal/deskkit/decide_test.go`, `tools/desk/internal/deskkit/decisionassessment.go` (planned), `tools/desk/internal/deskkit/decisionassessment_test.go` (planned), `tools/desk/internal/deskkit/decide.md`, `changelog/graph-execution-10-decision-contract.md` (planned)

facts: Advice has Answer and Justification; Decide already has conservative defaults, kill switch and non-default journal enforcement. The decider is not a worker tier.

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

1. Define request, prediction and separate policy-result envelopes per GEA-04,09–11. Include subject, input/schema digests, label distribution, explicit abstention, provider/calibrator versions, actual backend, budget and evidence references. Preserve Advice callers through an explicit projection.
2. Reject unknown labels, NaN/infinity, invalid normalization with declared rounding tolerance, mismatched subject, stale input and inapplicable calibration. Uncalibrated labels may be shadow-only; do not synthesize confidence.
3. Keep no-advisor, disabled, timeout, malformed, out-of-budget and journal-failure paths conservative. Evidence explanations are untrusted; no tools or write credentials enter the provider contract.

## Interface contract

Provider envelope → Decide → journal/default behaves correctly with malformed and unjournalled advice.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessment" ./...` | exit 0; output includes PASS for TestDecisionAssessment, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentMalformedDefaults" ./...` | exit 0; output includes PASS for TestDecisionAssessmentMalformedDefaults, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentDecideJournal" ./...` | exit 0; output includes PASS for TestDecisionAssessmentDecideJournal, with no [no tests to run] for its owning package |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
