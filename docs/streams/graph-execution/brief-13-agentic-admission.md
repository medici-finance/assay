---
brief: assay:assay:graph-execution:13
title: Deterministic admission over facts and bounded probabilistic advice
why: A task may look easy to a model while lacking a verifier, permission or safe recovery path. Admission must explain these differences and prevent confidence from bypassing a mandatory control.
wave: 1
depends:
- graph-execution/10
unblocks:
- graph-execution/14
effort: M
gate: human
risk:
  regulatory: yes
  customer: no
  irreversible: no
  sensitive-data: no
gate-why: The owner confirms the proposed admission policy preserves mandatory controls and existing human authority.
decision-trigger: spec
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
- 'tools/desk/internal/deskkit: fixed-here'
- 'tools/desk/internal/loopengine: follow-up graph-execution/14'
- 'statusgen/assayscore.go: out-of-scope (formula and history unchanged)'
version: 1
id: 31e146c5-2294-437b-9314-55d1dbef8558
---

# Brief 13 — Deterministic admission over facts and bounded probabilistic advice

## Context

files: `spec/agentic-admission-v1.md` (planned), `schemas/agentic-assessment-v1.json` (planned), `tools/desk/internal/deskkit/admission.go` (planned), `tools/desk/internal/deskkit/admission_test.go` (planned), `tools/desk/internal/deskkit/testdata/admission/` (planned), `docs/enforcement-model.md`, `changelog/graph-execution-13-agentic-admission.md` (planned)

facts: Pattern risk-input already maps low/standard/elevated/human to mandatory gates. Suitability dispositions are a different dimension; AssayScore keeps its existing formula.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Human decision

Decision-trigger: spec. At implementation pickup, prepare concrete policy choices and negative-path evidence, then file a self-contained decision issue. No response permits no activation; schema/test work may proceed within the declared scope.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Task

1. Implement pure policy evaluation with GEA-09/10 hard predicates, advisory dimensions, freshness and explicit reasons. Define allowed dispositions and an explicit mapping to risk-input preserving all mandatory gates. No model call is needed by the evaluator.
2. Unknown hard readiness holds implementation; discovery requires its own scope. Advice can only restrict an already owner-admitted category. Preserve human merge/release floors, stop flags, data policy and defaults when advice is absent or uncalibrated.
3. Add fixtures for high-confidence unsafe advice, missing verifier, revoked category, schema/environment change, expired exception, no data authority and all-hard-pass but policy-disallowed work. Document applicability and exception owner/expiry; never produce a single safety/compliance score.

## Interface contract

Facts + recorded advice → deterministic policy → mandatory graph gates; a high score cannot delete a required node.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmission" ./...` | exit 0; output includes PASS for TestAgenticAdmission, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionConfidenceCannotAuthorize" ./...` | exit 0; output includes PASS for TestAgenticAdmissionConfidenceCannotAuthorize, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionRiskInputUnion" ./...` | exit 0; output includes PASS for TestAgenticAdmissionRiskInputUnion, with no [no tests to run] for its owning package |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
