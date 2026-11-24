### Fixed
- The `go-lint` ratchet passes on main again: four baseline allowances for a QF1001 finding in the review-observation schema test, resolved by an earlier change but still listed (one per scan configuration), are pruned. The gate and the ratchet are unchanged and nothing is added to the baseline.
