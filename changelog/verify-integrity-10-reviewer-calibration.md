### Added
- `deskcalibrate`: a monthly reviewer-calibration verb. `deskcalibrate sample` draws a
  reproducible, seed-recorded sample of the PRs the reviewer App APPROVED in the prior
  month and REFUSES a re-review whose model vendor equals the reviewer role's own
  (`ASSAY_REVIEWER_VENDOR`) — a same-vendor re-review measures two instances of one model,
  not an independent judge. `deskcalibrate report` renders the agreement metric as an
  explicit numerator/denominator FRACTION, never a bare percentage (verify-integrity/10).
- `statusgen`: per-finding-class **reversal-rate** mining joined to the gate-yield
  accounting, with the two-month demotion rule — a class whose reversal rate exceeds 50%
  for two consecutive months is marked advisory; a later month under 50% restores it.

### Changed
- `pr-review-desk` skill: a finding-class register with a `blocking`/`advisory` status and
  the reversal-rate demotion rule wired to the monthly calibration report.
- Roster schema: `ASSAY_REVIEWER_VENDOR` / `ASSAY_VERIFIER_VENDOR` are recognised keys in
  both `statusgen` and the desk tools, so a roster carrying them no longer collapses the
  configuration on the unknown-key refusal.
