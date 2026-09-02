### Added
- `qualgen/riskscore` graduates a learned JIT defect-prediction model (Kamei-style
  diffusion/size/history/author features, temporal-split logistic regression)
  that always carries the §9.1 hand-weighted heuristic decomposition alongside
  it as fallback and explanation — a future-leak-proof training split, a
  could-not-learn fallback under a thin corpus (never a fabricated learned
  zero), and an honest learned-vs-heuristic comparison on held-out AUC
  (quality/15).
