---
brief: assay:assay:graph-execution:12
title: Reproducible decision evaluation and calibration manifests
why: Fast labels are useful only when error, abstention and escalation cost are known on relevant work. A repeatable comparison prevents adopting an impressive confidence number that fails outside the model’s training domain.
wave: 2
depends:
- graph-execution/11
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
- 'evaluation/decisions: fixed-here'
- 'tools/desk/internal/deskkit/admission.go: follow-up graph-execution/13'
version: 1
id: d77c1094-2fd6-46eb-a453-2dafd933ebf8
---

# Brief 12 — Reproducible decision evaluation and calibration manifests

## Context

files: `evaluation/decisions/evaluate.py` (planned), `evaluation/decisions/test_evaluate.py` (planned), `evaluation/decisions/fixtures/` (planned), `evaluation/decisions/README.md` (planned), `schemas/calibration-manifest-v1.json` (planned), `changelog/graph-execution-12-decision-evaluation.md` (planned)

facts: Provider 11 supplies exact backend identity; no upstream benchmark establishes Assay performance. Core graph fixtures must not require model weights or GPU.

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

1. Implement a replayable evaluator for rules, a classical classifier and Laya records. Dataset IDs, label adjudication, related-entity/repository/time splits and separate calibration/test populations are mandatory; reject overlap.
2. Emit per-class errors, Brier/log loss, calibration/reliability, risk–coverage with uncertainty, abstention and downstream escalation/cost. Report candidate ordering without prescribing a universal threshold. Track model/schema/domain/backend/precision applicability.
3. Measure real-request cold/warm p50/p95, throughput at declared concurrency, peak host RAM/VRAM and failure/fallback counts. Separate measured, replayed and unavailable configurations. Preserve fixed holdouts; failing slices prevent advisory promotion even if aggregate accuracy passes.
4. Produce a signed-off-by-owner-ready manifest/report template; this public brief uses synthetic or licensed public data only. All factual metrics in a report must regenerate from its input records.

## Interface contract

Prediction files → split validation → calibration manifest/report reproduces known fixture metrics and rejects leaked holdouts.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `python3 -m unittest discover -s evaluation/decisions -p test_evaluate.py -v` | exit 0; named suite tests execute |
| 2 | check:ci +mutation | `python3 -m unittest discover -s evaluation/decisions -p test_evaluate.py -v` | exit 0; TestSplitLeakageRejected executes and rejects the negative fixture |
| 3 | check:ci +dereference | `python3 -m unittest discover -s evaluation/decisions -p test_evaluate.py -v` | exit 0; TestMetricsRegenerateFromRecords reproduces metrics from fixture input records |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
