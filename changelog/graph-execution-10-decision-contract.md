### Added
- `deskkit`: a typed decision-assessment envelope (`AssessmentRequest`/`Prediction`/
  `PolicyResult`, `spec/decision-assessment-v1.md`, `schemas/decision-assessment-v1.json`)
  layered on top of the existing `Decide`/`Advice` consult, keeping calibrated
  probabilistic advice strictly separate from a deterministic policy record.
  `ValidatePrediction` rejects unknown labels, NaN/Inf or out-of-range probabilities,
  invalid normalization, mismatched subject/digests, an uncalibrated label carrying a
  synthesized probability, and inapplicable calibration. `PredictionAdvisor` projects a
  validated `Prediction` into the existing `Advice` contract with zero change to
  `Decide`'s fail-closed default, budget, timeout, journal or reserved-verb rules
  (graph-execution/10).
