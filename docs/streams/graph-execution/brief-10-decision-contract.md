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

### Non-implementer verifier run — VERIFY: BLOCKED — 0/3 pass, 3 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|-------------|---|
| 1 | cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestDecisionAssessment' ./... | exit 0; PASS for the base decision-assessment test, no "[no tests to run]" for the owning package | COULD-NOT-CHECK — hermetic witness owed (darwin): the sanctioned check:ci network-off sandbox needs Linux unshare --net; host is darwin (re-executed network-off by design). Direct non-hermetic run: exit 0; `--- PASS: TestDecisionAssessment` (the unanchored prefix also matches, and the direct run also passed, the two sibling decision-assessment tests rows 2 and 3 name); owning deskkit package "ok ... 2.9s" with no "[no tests to run]". | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestDecisionAssessment.*MalformedDefaults' ./... | exit 0; PASS for the malformed-defaults negative test, no "[no tests to run]" for the owning package | COULD-NOT-CHECK — hermetic witness owed (darwin): check:ci network-off sandbox needs Linux unshare --net; host is darwin. Direct non-hermetic run: exit 0; the named --- PASS line for this row's test, with all 13 negative subtests passing (unknown label, unknown shadow label, NaN, infinite, out-of-range, invalid normalization, mismatched subject, stale input, mismatched schema, uncalibrated-carries-probability, abstained-carries-probabilities, budget overrun, inapplicable calibration). (`go test -list '^TestDecisionAssessment.*MalformedDefaults'` selects exactly the one test this row names — the malformed-defaults negative test.) | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestDecisionAssessment.*DecideJournal' ./... | exit 0; PASS for the decide-journal test, no "[no tests to run]" for the owning package | COULD-NOT-CHECK — hermetic witness owed (darwin): check:ci network-off sandbox needs Linux unshare --net; host is darwin. Direct non-hermetic run: exit 0; the named --- PASS line for this row's test, with all 4 flow subtests passing; the test calls real Question.Decide through the prediction advisor with a recording journal (well-formed advised+journalled; malformed -> default + error outcome; abstention -> default via vocabulary check; unjournallable advice discarded -> default). (`go test -list '^TestDecisionAssessment.*DecideJournal'` selects exactly the one test this row names — the decide-journal test.) | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE enumeration (kit §4 — enumerate, rank by irreversibility, derive top-ranked). Literals introduced/changed by the diff, each a literal at file:line:

- the prediction-normalization tolerance constant = 1e-6 @ tools/desk/internal/deskkit/decisionassessment.go:68 (the sum-to-1 rounding tolerance)
- probability domain bounds 0 and 1 @ tools/desk/internal/deskkit/decisionassessment.go:165 (prob < 0 || prob > 1)
- justification truncation length = 280 @ tools/desk/internal/deskkit/decisionassessment.go:258

Rank: none is irreversible (all reversible by edit + redeploy; item risk metadata is all "no", irreversible: no). Top-ranked by consequence is the normalization tolerance (the one genuinely tuned threshold); the probability bounds are definitional; the truncation length is a reversible display knob (ranks last, no derivation needed).

- RISK-VALUE: DERIVED — the prediction-normalization tolerance constant = 1e-6 @ tools/desk/internal/deskkit/decisionassessment.go:68 — this bounds |sum(calibrated probs) - 1|. Accumulated float64 rounding over a handful of O(1) terms is on the order of machine epsilon (~2.2e-16) times the term count, i.e. ~1e-15, so 1e-6 absorbs legitimate rounding by ~9 orders of magnitude while still rejecting a genuinely unnormalized distribution (the negative row uses sum 0.7). Fail-safe either way: too tight rejects valid predictions into the conservative default; too loose admits only negligibly-off distributions; reversible edit + redeploy.
- RISK-VALUE: DERIVED — probability domain bounds 0 and 1 @ tools/desk/internal/deskkit/decisionassessment.go:165 — a probability is definitionally within [0,1]; the guard rejects any value outside that closed interval. First principles, no tuning.
- RISK-VALUE: N/A for the truncation length 280 — a reversible display-only justification cap, an operational knob out of scope by design.


## Review

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
