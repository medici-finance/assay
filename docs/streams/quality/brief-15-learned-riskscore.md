---
brief: assay:assay:quality:15
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
schema: brief-v2
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §9.1 — PR riskscore feed; Evolution: graduate to a JIT model trained on the repo's own traced defects, heuristic features remain fallback + explanation"
  - "docs/streams/quality/spec.md §5 (M2 SZZ) — the traced-defect corpus the model trains on"
  - "docs/streams/quality/spec.md §12 — Kamei et al., JIT quality-assurance feature family"
  - "docs/streams/quality/spec.md §3.2 — three-state invariant; §10 honest-claims"
version: 1
id: fbaac7e4-e22d-48d8-bc73-01c5da3277e8
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
### Non-implementer verifier run — VERIFY: PASS — 2026-09-15 claude-sonnet-5-verifier (verify-desk dispatch), merged main e4ad0a07

Runner != implementer. Own temporary worktree off origin/main, offline (KUBECONFIG=/dev/null). gate: model, risk {all no}, irreversible: no. Files present at the merged SHA: qualgen/riskscore/heuristic.go, features.go, learned.go + their _test.go files (all landed via PR #309).

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | cd qualgen && go build ./... && go vet ./riskscore/ | 0 | clean build + vet, no output | 2026-09-15 | claude-sonnet-5-verifier |
| 2 | cd qualgen && go test ./riskscore/ | 0 | ok github.com/medici-finance/assay/qualgen/riskscore 0.267s — all 10 subtests pass (heuristic, feature-extractor, learned-model) | 2026-09-15 | claude-sonnet-5-verifier |
| 3 | cd qualgen && go test ./riskscore/ -run TestLearned_BeatsHeuristicOnHeldOut -v | 0 | PASS — "held-out evaluation: learned beats heuristic: held-out AUC 0.776 vs 0.612 over 300 later changes, at corpus size 300 / trace-rate 72%" — temporal train/test split, learned strictly beats heuristic on held-out later changes | 2026-09-15 | claude-sonnet-5-verifier |
| 4 | cd qualgen && go test ./riskscore/ -run TestLearned_NoFutureLeakInTrainingSet -v | 0 | PASS — a fixture defect with a future label time is excluded from the training set built for an earlier as-of cutoff | 2026-09-15 | claude-sonnet-5-verifier |
| 5 | cd qualgen && go test ./riskscore/ -run TestLearned_ScoreCarriesHeuristicFeatures -v && go test ./riskscore/ -run TestLearned_UnderCorpusFallsBackToHeuristic -v | 0 | both PASS — every learned score carries its heuristic decomposition; below MinCorpus the result is could-not-measure/could-not-learn with no fabricated learned zero | 2026-09-15 | claude-sonnet-5-verifier |

**VERIFY: PASS** — all 5 Verify rows offline-clean; none unrun.

**Risk-bearing value enumeration** (qualgen/riskscore/heuristic.go, learned.go — every literal constant this PR introduces):
- w1 = HotspotPercentile weight 0.40 @ heuristic.go:115
- w2 = TracedDefectDensity weight 0.30 @ heuristic.go:116
- w3 = TopIdentityOwnershipShare weight 0.15 @ heuristic.go:117
- w4 = MissingCouplingPartners weight 0.15 @ heuristic.go:118
- kDensity = 5.0 (defect-density saturation constant, normDensity) @ heuristic.go:203
- kCoupling = 3.0 (missing-coupling-partners saturation constant, saturate) @ heuristic.go:214
- MinCorpus = 40 (three-state under-corpus training threshold) @ learned.go:47
- Epochs = 400 (gradient-descent epochs) @ learned.go:49
- LearningRate = 0.1 @ learned.go:50
- L2 = 1e-3 (L2 regularization strength) @ learned.go:51

Ranked by irreversibility: none of the ten are irreversible — every one is a code default a
follow-up PR can edit and redeploy, and per spec §9.1 "consumers weight them; thresholds
live in the consumer's config, not here" — this tool is advisory-only (§9.2: "Advisory
NOTICE first; hard gating is a later, separate decision"), so a wrong value mis-prioritizes
an advisory PR flag rather than blocking or corrupting anything. Within that reversible set,
MinCorpus ranks highest because it is the on/off switch for the honest-claims/three-state
invariant itself (spec §3.2, §10): too low, and the model "graduates" past heuristic on too
little data, producing a real (not fabricated) but statistically unreliable learned number
that could still be quoted via Comparison.Claim() as if backed by real signal. The four
heuristic weights and the two saturation constants rank next (they are what actually ships
whenever the corpus is under MinCorpus, which is most of the time pre-graduation); Epochs/
LearningRate/L2 rank last as pure gradient-descent tuning knobs.

**RISK-VALUE: NAMED, NOT DERIVED** — MinCorpus=40 @ qualgen/riskscore/learned.go:47 — the
model trains a 15-dimensional Kamei feature vector (FeatureNames(), features.go:162-167).
Standard logistic-regression guidance (events-per-variable, Peduzzi et al. 1996, cited via
Kamei et al.'s own JIT-QA lineage at spec.md §12) wants on the order of 10 defect-positive
events per predictor before treating coefficients as stable — for 15 predictors that is
~150 positive events, i.e. more than 40 TOTAL examples even before accounting for class
balance. Neither the brief's own precondition ("a SEASONED M2 corpus large enough to train",
spec §11) nor spec §9.1's "once the M2 corpus is large enough" name a number, so nothing in
the repo's stated constraints licenses 40 specifically — it is the implementer's own choice,
undocumented as a derivation. The green Verify table does not settle this: row 5's
under-corpus test only exercises the below-threshold PATH (any threshold would pass it), and
row 3's held-out win is measured on a fixture corpus of 300 examples, well clear of the
40-example boundary, so it says nothing about whether 40 itself is sound. Recommend a
follow-up brief/comment deriving MinCorpus from the actual feature count (or citing a
specific EPV target) rather than the current unexplained 40 — flagging for reviewer
attention, not blocking this PASS since the value is reversible, advisory-only, and the
item's own risk block is all-no / gate:model.

- w1..w4, kDensity, kCoupling: reversible heuristic tuning knobs, unchanged in kind from the
  existing §9.1 hand-weighted design (spec §9.1 explicitly reserves the tuning judgment to
  "consumers"); no further derivation attempted — out of scope by the same reversibility
  reasoning above.
- Epochs/LearningRate/L2: standard gradient-descent hyperparameters; convergence is
  indirectly checked by Verify row 3 (the model must beat the heuristic baseline on held-out
  data), which would fail on a badly-diverged fit — reversible, lowest-ranked, no further
  derivation attempted.

## Review
Gate: model (all four risk answers no — repo-agnostic OSS Go training on the repo's own
traced-defect corpus and scoring changes; no regulated, customer, irreversible, or
sensitive-data surface). This is `exec-tier: strong` work: the reviewer specifically
confirms the temporal split admits no future leak (Verify row 4) and that the learned
score never ships without its heuristic explanation/fallback (Verify row 5), then records
verdict + date in the stream README table.
