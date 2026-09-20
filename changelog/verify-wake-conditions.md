### Added
- Verification **wake receipts** (`verify-wake-v1`): a failed or blocked verifier run records a
  checkable wake condition — the inputs it observed, the blocker class, and what must change
  before re-running is worth a slot. An unchanged receipt keeps the failure visible as a `wait`
  row (naming its blocker and next actor) but no longer consumes a verifier dispatch every pass;
  a changed relevant input, tool version, Verify definition, or completed action wakes it, while
  an unrelated change does not.

### Changed
- `verifyloop plan` classifies a failed/blocked brief with an unchanged wake receipt into a new
  `wait` bucket instead of re-dispatching it. Unreadable declared inputs stay visibly
  could-not-check (never rounded up to unchanged), and legacy or incomplete receipts stay
  eligible for one classification pass. A partial hold lets a newly-runnable row dispatch while
  the held rows are recorded as explicitly unrun — no partial result closes the whole brief.
