---
brief: assay:assay:graph-execution:17
title: Graph-linked release and outcome records without new authority
why: A completed brief does not tell an operator whether its artifact was released or whether the intended result occurred. Shared links preserve those distinctions without inventing deployment permission.
wave: 4
depends:
- graph-execution/06
unblocks:
- graph-execution/18
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
- 'delivery adapters: out-of-scope (implementations consume this public contract in their own rollout)'
- 'operator clients: out-of-scope (render this projection without granting actions)'
version: 1
id: 56e6e98b-65fb-4418-bd8e-6775da5177c5
---

# Brief 17 — Graph-linked release and outcome records without new authority

## Context

files: `spec/lifecycle-links-v1.md` (planned), `schemas/lifecycle-links-v1.json` (planned), `statusgen/lifecyclelinks.go` (planned), `statusgen/lifecyclelinks_test.go` (planned), `statusgen/testdata/lifecyclelinks/` (planned), `docs/lifecycle.md`, `changelog/graph-execution-17-lifecycle-links.md` (planned)

facts: Current workflow schema has no cross-pattern composition and forge-oriented effects only. This brief links records; it does not add deploy/rollback actions or a new runtime.

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

1. Define objective/change, instance/attempt, release/artifact/environment and observation references, baseline/window and interpretation owner. Preserve existing work IDs and exact subject identities.
2. Validate linked records and emit one read-only projection with freshness, pending owner and provenance. Distinguish verified-unreleased, released-unobserved, observed-inconclusive and observed-outcome; do not infer business value from activity.
3. Provide adapter conformance fixtures using recorded external receipts; actual forge/delivery state stays authoritative at its provider. Define additive compatibility with 06 records. No mandatory graph database or single combined storage artifact.

## Interface contract

Run → evidence → recorded release → observation projects exact identities and refuses wrong-artifact joins.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestLifecycleLinks" ./...` | exit 0; output includes PASS for TestLifecycleLinks, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestLifecycleLinksWrongArtifactDenied" ./...` | exit 0; output includes PASS for TestLifecycleLinksWrongArtifactDenied, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestLifecycleLinksRunToOutcome" ./...` | exit 0; output includes PASS for TestLifecycleLinksRunToOutcome, with no [no tests to run] for its owning package |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
