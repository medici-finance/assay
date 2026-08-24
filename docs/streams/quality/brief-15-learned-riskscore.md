---
brief: quality/15
title: learned riskscore graduation — JIT defect-prediction model with heuristic fallback
wave: 3
why: >-
  The §9.1 hand-weighted risk features are a fixed guess at what predicts defects. Once the
  repo has traced enough of its OWN defects, a just-in-time defect-prediction model can
  learn the weights from that history and predict better on held-out defects. The
  hand-weighted features must remain the fallback and the explanation layer — a learned
  score that cannot show its reasoning does not get to replace one that can.
depends: ["quality/07"]
unblocks: []
effort: M
gate: model
exec-tier: strong
exec-tier-why: >-
  model-design + data-leakage-avoidance reasoning (open questions a and c) — a temporal
  split done wrong silently trains on a defect's own future and inflates every metric.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §9.1 — PR riskscore feed; Evolution: graduate to a JIT model trained on the repo's own traced defects, heuristic features remain fallback + explanation"
  - "docs/streams/quality/spec.md §5 (M2 SZZ) — the traced-defect corpus the model trains on"
  - "docs/streams/quality/spec.md §12 — Kamei et al., JIT quality-assurance feature family"
  - "docs/streams/quality/spec.md §3.2 — three-state invariant; §10 honest-claims"
---

# Brief 15 — learned riskscore graduation (JIT defect-prediction model)

## Context

files:
- NEW `qualgen/riskscore/features.go` (planned) (+ `features_test.go`) — the Kamei-style
  JIT feature extractor over a change: diffusion (files/subsystems/entropy of the change),
  size (lines added/deleted), history (prior changes/defects to the touched files, recent
  churn), and author-class features (human/agent/automation identity, per §4.2/§4.4).
- NEW `qualgen/riskscore/learned.go` (planned) (+ `learned_test.go`) — trains a JIT
  defect-prediction model on the repo's OWN traced defects (M2 corpus from quality/07) and
  scores a change; every learned score is emitted ALONGSIDE the §9.1 heuristic features it
  is built from, so the score is always explainable.
- NEW `qualgen/riskscore/heuristic.go` (planned) (+ `heuristic_test.go`) — the existing
  hand-weighted §9.1 features as the FALLBACK + explanation layer: used verbatim when the
  corpus is too small to train, when the model cannot produce its features, or as the
  human-readable decomposition beside any learned score.
- CONSUMES M2 traces (defect lineage + per-file defect density) from quality/07 — the
  labeled corpus the model trains and is evaluated on.

facts:
- **Learned score never stands alone** (spec §9.1): the hand-weighted heuristic features
  remain the fallback and the explanation layer. A learned score is emitted WITH its
  underlying features; if the model cannot show its features, the heuristic score is what
  ships. This is a design invariant, not a nice-to-have.
- **Training-data leakage is the primary hazard (exec-tier: strong).** The model MUST use a
  TEMPORAL split: a defect labeled at time T may never appear in the training set used to
  predict at or before T. Blame-derived labels look backward, so a naive split leaks a
  defect's own future into its features. The split is temporal by commit/merge time, and
  the evaluation is on held-out LATER defects only.
- **Three-state under-corpus** (spec §3.2): below the minimum corpus size the model does
  not train — the score is emitted as heuristic-only with a could-not-learn status, never a
  fabricated learned zero.
- **Honest-claims** (spec §10): any "learned beats heuristic" claim ships with its
  held-out evaluation metric and the corpus size/trace-rate it was measured at — never a
  bare "more accurate."
- **Preconditions.** Wave 3: needs a SEASONED M2 corpus large enough to train (spec §11);
  below that threshold the tool runs heuristic-only. The code is testable against a fixture
  defect corpus without a live seasoned corpus.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- The learned score MUST always carry its heuristic-feature decomposition; training MUST be
  temporally split so no change is scored using knowledge from its own future. If anything
  is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `heuristic.go`: the §9.1 hand-weighted features (hotspot percentile, traced
   defect density, top-identity ownership share, missing coupling partners) with a
   three-state `measured` flag — the fallback + explanation layer, usable standalone.
2. Implement `features.go`: the Kamei-style JIT feature vector for a change — diffusion,
   size, history, author-class — computed from the M2 corpus + change metadata.
3. Implement `learned.go`: train a JIT defect-prediction model on the repo's own traced
   defects using a TEMPORAL train/test split (split by commit/merge time; a defect is only
   ever in the training set for predictions AFTER its label time); score a change and emit
   the learned score TOGETHER with its §9.1 heuristic decomposition. Below the minimum
   corpus size, fall back to heuristic-only with a could-not-learn status.
4. Emit any learned-vs-heuristic comparison with its held-out metric + corpus size/
   trace-rate attached (honest-claims §10).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./riskscore/` | exit 0 |
| 2 | `cd qualgen && go test ./riskscore/` | exit 0; heuristic, feature-extractor, and learned-model tests pass |
| 3 (DEREFERENCE) | `cd qualgen && go test ./riskscore/ -run TestLearned_BeatsHeuristicOnHeldOut -v` | exit 0 — fixture defect corpus with a TEMPORAL train/test split; the learned model's held-out accuracy/AUC on LATER defects is asserted strictly greater than the heuristic baseline's on the same held-out set (proves the model actually learned signal, not merely that it produces a number) |
| 4 (leakage/negative) | `cd qualgen && go test ./riskscore/ -run TestLearned_NoFutureLeakInTrainingSet -v` | exit 0 — for a defect labeled at time T, the training set used to score any change at/before T contains NO record derived from that defect's own future; a fixture that injects a future-leaked label is rejected/excluded |
| 5 (explainability/fallback) | `cd qualgen && go test ./riskscore/ -run TestLearned_ScoreCarriesHeuristicFeatures -v && go test ./riskscore/ -run TestLearned_UnderCorpusFallsBackToHeuristic -v` | exit 0 — every learned score is emitted with its §9.1 heuristic feature decomposition; below the minimum corpus the output is heuristic-only with a could-not-learn status, never a fabricated learned zero |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — repo-agnostic OSS Go training on the repo's own
traced-defect corpus and scoring changes; no regulated, customer, irreversible, or
sensitive-data surface). This is `exec-tier: strong` work: the reviewer specifically
confirms the temporal split admits no future leak (Verify row 4) and that the learned
score never ships without its heuristic explanation/fallback (Verify row 5), then records
verdict + date in the stream README table.
